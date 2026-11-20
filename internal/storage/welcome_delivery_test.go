package storage

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage/migrations"
)

func TestWelcomeDeliveryAtomicCreationAndConcurrentWorkers(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	for _, change := range []WelcomeChange{{Action: "save", Content: WelcomeRevision{Subject: "Welcome {{name}}", HTML: "<p>Hello {{name}}: {{email}}</p>", Text: "Hello {{name}}"}}, {Action: "publish", Expected: 1, Revision: 1}} {
		if _, err := f.s.ChangeWelcomeTemplate(ctx, "tenants", f.user.TenantID, f.user.Email, change); err != nil {
			t.Fatal(err)
		}
	}
	makeUser := func() (*User, error) {
		return f.s.CreateUser(ctx, &User{TenantID: f.user.TenantID, DomainID: f.user.DomainID, LocalPart: "ldap-new", Email: "ldap-new@example.com", DisplayName: "LDAP user", AuthSource: "ldap", Enabled: true})
	}
	removeFailure := failCalendarWrite(t, f.s, "calendar_deliveries", "INSERT", "")
	if _, err := makeUser(); err == nil {
		t.Fatal("creation succeeded without durable welcome event")
	}
	if _, err := f.s.GetUserByEmail(ctx, "ldap-new@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatal("user escaped rollback", err)
	}
	removeFailure()
	u, err := makeUser()
	if err != nil {
		t.Fatal(err)
	}
	other, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	// Two processes observe the same queue. Exactly one claims the welcome.
	type result struct {
		job *CalendarDelivery
		err error
	}
	results := make(chan result, 2)
	gate := make(chan struct{})
	var wg sync.WaitGroup
	for _, driver := range []Driver{f.s, other} {
		wg.Add(1)
		go func(d Driver) { defer wg.Done(); <-gate; j, e := d.ClaimCalendarDelivery(ctx); results <- result{j, e} }(driver)
	}
	close(gate)
	wg.Wait()
	close(results)
	var claimed *CalendarDelivery
	claims := 0
	for r := range results {
		if r.err == nil {
			claims++
			claimed = r.job
		} else if !errors.Is(r.err, ErrNotFound) {
			t.Fatal(r.err)
		}
	}
	if claims != 1 || claimed.Kind != "welcome" || claimed.RecipientUserID != u.ID || !strings.Contains(string(claimed.Data), "LDAP user") {
		t.Fatal("claim", claims, claimed)
	}
	if _, err = f.s.db.ExecContext(ctx, f.s.rebind(`UPDATE calendar_deliveries SET lease_until=0 WHERE id=?`), claimed.ID); err != nil {
		t.Fatal(err)
	}
	winner, err := other.ClaimCalendarDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	if err = f.s.CommitWelcomeDelivery(ctx, claimed, func() (*Message, error) { calls.Add(1); return nil, nil }); !errors.Is(err, ErrCalendarConflict) || calls.Load() != 0 {
		t.Fatal("stale worker performed IO", err, calls.Load())
	}
	mb, err := f.s.EnsureMailbox(ctx, u.ID, "INBOX", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = other.CommitWelcomeDelivery(ctx, winner, func() (*Message, error) {
		calls.Add(1)
		return &Message{MailboxID: mb.ID, FilePath: "welcome.eml", Size: int64(len(winner.Data)), InternalDate: time.Now().UTC()}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = other.CompleteCalendarDelivery(ctx, winner); err != nil {
		t.Fatal(err)
	}
	loaded, err := f.s.GetMessageByID(ctx, winner.ID)
	if err != nil || loaded.UID != 1 {
		t.Fatal("receipt missing", loaded, err)
	}
	if err = f.s.UpdateUserEnabled(ctx, u.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = f.s.UpdateUserEnabled(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ClaimCalendarDelivery(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatal("reactivation resent", err)
	}
	if _, err = makeUser(); err == nil {
		t.Fatal("duplicate user creation succeeded")
	}
	var count int
	if err = f.s.db.QueryRowContext(ctx, f.s.rebind(`SELECT COUNT(*) FROM calendar_deliveries WHERE welcome_user_id=?`), u.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate receipt", count, err)
	}
}

func TestWelcomeDeliveryMigrationAndTemplatePrecedence(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	suffix := "sqlite"
	if f.s.dialect == DialectPostgres {
		suffix = "postgres"
	}
	raw, err := migrations.FS.ReadFile("077_welcome_delivery." + suffix + ".sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	tx, err := f.s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = execScript(ctx, tx, parts[1]); err != nil {
		t.Fatal(err)
	}
	if err = execScript(ctx, tx, strings.TrimPrefix(parts[0], "-- +goose Up")); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []struct{ kind, id, subject string }{{"tenants", f.user.TenantID, "Tenant welcome"}, {"domains", f.user.DomainID, "Domain welcome"}} {
		if _, err = f.s.ChangeWelcomeTemplate(ctx, scope.kind, scope.id, f.user.Email, WelcomeChange{Action: "save", Content: WelcomeRevision{Subject: scope.subject, HTML: "<p>Welcome</p>", Text: "Welcome"}}); err != nil {
			t.Fatal(err)
		}
		if _, err = f.s.ChangeWelcomeTemplate(ctx, scope.kind, scope.id, f.user.Email, WelcomeChange{Action: "publish", Expected: 1, Revision: 1}); err != nil {
			t.Fatal(err)
		}
	}
	for i, mode := range []string{"own", "inherit", "off"} {
		if i > 0 {
			if _, err = f.s.ChangeWelcomeTemplate(ctx, "domains", f.user.DomainID, f.user.Email, WelcomeChange{Action: "mode", Expected: int64(i + 1), Mode: mode}); err != nil {
				t.Fatal(err)
			}
		}
		u, err := f.s.CreateUser(ctx, &User{TenantID: f.user.TenantID, DomainID: f.user.DomainID, LocalPart: mode, Email: mode + "@example.com", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		var subject, origin string
		err = f.s.db.QueryRowContext(ctx, f.s.rebind(`SELECT subject,origin_json FROM calendar_deliveries WHERE welcome_user_id=?`), u.ID).Scan(&subject, &origin)
		if mode == "off" {
			if err == nil {
				t.Fatal("off queued mail")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if mode == "own" && subject != "Domain welcome" || mode == "inherit" && subject != "Tenant welcome" {
			t.Fatal("wrong template", mode, subject, origin)
		}
	}
}

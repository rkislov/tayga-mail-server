package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func welcomeDeliveryFixture(t *testing.T) (*fileFixture, *storage.User, string) {
	t.Helper()
	f := newFileFixture(t)
	ctx := context.Background()
	u, err := f.s.store.GetUserByID(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.UpdateUserRoles(ctx, u.ID, "tenant_admin"); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/admin/welcome/tenants/" + u.TenantID
	body, _ := json.Marshal(storage.WelcomeChange{Action: "save", Content: storage.WelcomeRevision{Subject: "Welcome {{name}}", HTML: `<h1>Hello {{name}}</h1><p>{{email}} · {{domain}} · {{tenant}}</p><img alt="Team" src="` + testMailPhoto(t) + `">`}})
	f.call(0, "POST", path, string(body), 200)
	f.call(0, "POST", path, `{"action":"publish","expected":1,"revision":1}`, 200)
	return f, u, path
}

func welcomeMessages(t *testing.T, f *fileFixture, u *storage.User) []*storage.Message {
	t.Helper()
	folders, err := f.s.store.ListMailboxes(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	var out []*storage.Message
	for _, folder := range folders {
		items, err := f.s.store.ListMessages(context.Background(), folder.ID)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, items...)
	}
	return out
}
func createWelcomeUser(t *testing.T, f *fileFixture, owner *storage.User, local string, enabled bool) *storage.User {
	t.Helper()
	u, err := f.s.store.CreateUser(context.Background(), &storage.User{TenantID: owner.TenantID, DomainID: owner.DomainID, Email: local + "@example.com", LocalPart: local, DisplayName: `<Anna & "Team">`, Enabled: enabled})
	if err != nil {
		t.Fatal(err)
	}
	return u
}
func welcomeFiles(t *testing.T, f *fileFixture, u *storage.User) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(f.s.ms.UserRoot(u.Email), "new", "welcome-*:2,"))
	if err != nil {
		t.Fatal(err)
	}
	return len(matches)
}

func TestWelcomeAutomaticAdminCreationFrozenPhotoAndNoResend(t *testing.T) {
	f, owner, path := welcomeDeliveryFixture(t)
	ctx := context.Background()
	// Publishing never backfills the three existing accounts.
	if err := f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range f.users {
		u, _ := f.s.store.GetUserByID(ctx, id)
		if len(welcomeMessages(t, f, u)) != 0 {
			t.Fatal("backfilled existing user")
		}
	}
	body, _ := json.Marshal(map[string]any{"email": "new@example.com", "password": "temporary-secret", "display_name": `<Anna & "Team">`})
	f.call(0, "POST", "/api/v1/admin/users", string(body), 201)
	u, err := f.s.store.GetUserByEmail(ctx, "new@example.com")
	if err != nil {
		t.Fatal(err)
	}
	job, err := f.s.store.ClaimCalendarDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if job.Kind != "welcome" || job.RecipientUserID != u.ID || job.Sender != "postmaster@example.com" || !strings.Contains(job.OriginJSON, `"revision":1`) {
		t.Fatalf("job %+v", job)
	}
	firstMIME := append([]byte{}, job.Data...)
	// Change template and profile after enqueue: the original letter is immutable.
	f.call(0, "POST", path, `{"action":"save","expected":2,"content":{"subject":"New template","html":"<p>New body</p>"}}`, 200)
	f.call(0, "POST", path, `{"action":"publish","expected":3,"revision":2}`, 200)
	if err = f.s.store.UpdateUserProfile(ctx, u.ID, "Changed name"); err != nil {
		t.Fatal(err)
	}
	if err = f.s.deliverCalendarJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	messages := welcomeMessages(t, f, u)
	if len(messages) != 1 {
		t.Fatal("delivery count", len(messages))
	}
	raw, err := f.s.ms.Read(messages[0].FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, firstMIME) {
		t.Fatal("queued MIME changed")
	}
	mail := parseMIMEMessage(raw)
	if !strings.Contains(mail.HTML, "&lt;Anna &amp;") || strings.Contains(mail.HTML, "<Anna") || !strings.Contains(mail.HTML, testMailPhoto(t)) || !strings.Contains(mail.Text, "new@example.com") || strings.Contains(string(raw), "temporary-secret") || strings.Contains(mail.Subject, "New template") {
		t.Fatal("invalid personalization", mail.Subject, mail.HTML)
	}
	if len(mail.Attachments) != 1 || mail.Attachments[0].ContentID == "" {
		t.Fatal("missing inline photo")
	}
	stats, err := f.s.store.WelcomeDeliveryStats(ctx, "tenants", owner.TenantID)
	if err != nil || stats.Delivered != 1 || stats.Pending != 0 {
		t.Fatal("receipt stats", stats, err)
	}
	for range 2 {
		f.s.store.UpdateUserEnabled(ctx, u.ID, false)
		f.s.store.UpdateUserEnabled(ctx, u.ID, true)
		if err = f.s.drainCalendarDeliveries(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(welcomeMessages(t, f, u)) != 1 {
		t.Fatal("reactivation resent welcome")
	}
	if err = f.s.store.DeleteMessage(ctx, messages[0].ID); err != nil {
		t.Fatal(err)
	}
	if err = f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	if len(welcomeMessages(t, f, u)) != 0 {
		t.Fatal("deleted welcome resent")
	}
}

func TestWelcomeDisabledSuppressedResourceDomainOffAndDeletion(t *testing.T) {
	f, owner, _ := welcomeDeliveryFixture(t)
	ctx := context.Background()
	disabled := createWelcomeUser(t, f, owner, "disabled-welcome", false)
	if _, err := f.s.store.ClaimCalendarDelivery(ctx); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal("disabled delivery claimed", err)
	}
	if err := f.s.store.UpdateUserEnabled(ctx, disabled.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	if len(welcomeMessages(t, f, disabled)) != 1 {
		t.Fatal("no letter after activation")
	}
	suppressed := &storage.User{TenantID: owner.TenantID, DomainID: owner.DomainID, Email: "restored@example.com", LocalPart: "restored", Enabled: true}
	if _, err := f.s.store.CreateUser(storage.WithoutWelcomeMail(ctx), suppressed); err != nil {
		t.Fatal(err)
	}
	resource, err := f.s.store.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: owner.DomainID, LocalPart: "meeting-room", DisplayName: "Meeting room", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if resource.UserID == "" {
		t.Fatal("resource missing")
	}
	if _, err = f.s.store.ChangeWelcomeTemplate(ctx, "domains", owner.DomainID, owner.Email, storage.WelcomeChange{Action: "mode", Mode: "off"}); err != nil {
		t.Fatal(err)
	}
	off := createWelcomeUser(t, f, owner, "off-welcome", true)
	if err = f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	for _, u := range []*storage.User{suppressed, off} {
		if len(welcomeMessages(t, f, u)) != 0 {
			t.Fatal("excluded user welcomed", u.Email)
		}
	}
	if _, err = f.s.store.ChangeWelcomeTemplate(ctx, "domains", owner.DomainID, owner.Email, storage.WelcomeChange{Action: "mode", Mode: "inherit", Expected: 1}); err != nil {
		t.Fatal(err)
	}
	deleted := createWelcomeUser(t, f, owner, "deleted-welcome", true)
	if err = f.s.store.DeleteUser(ctx, deleted.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.store.ClaimCalendarDelivery(ctx); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal("deleted account still queued", err)
	}
	// Domain admin stats include only its own domain, never the other tenant's.
	other, err := f.s.store.CreateTenant(ctx, "private-welcome")
	if err != nil {
		t.Fatal(err)
	}
	stats, err := f.s.store.WelcomeDeliveryStats(ctx, "tenants", other.ID)
	if err != nil || stats.Delivered != 0 || stats.Pending != 0 {
		t.Fatal("stats leak", stats, err)
	}
}

func TestWelcomeRetryUsesOneFileAndFencesStaleWorker(t *testing.T) {
	f, owner, _ := welcomeDeliveryFixture(t)
	ctx := context.Background()
	u := createWelcomeUser(t, f, owner, "retry-welcome", true)
	job, err := f.s.store.ClaimCalendarDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	db := f.s.store.(*storage.Store).DB()
	if _, err = db.Exec(`CREATE TRIGGER fail_welcome_message BEFORE INSERT ON messages BEGIN SELECT RAISE(ABORT,'injected welcome failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err = f.s.deliverCalendarJob(ctx, job); err == nil {
		t.Fatal("expected failure after file publication")
	}
	if len(welcomeMessages(t, f, u)) != 0 || welcomeFiles(t, f, u) != 1 {
		t.Fatal("partial receipt or missing recovery file")
	}
	if err = f.s.store.RetryCalendarDelivery(ctx, job, "injected failure"); err != nil {
		t.Fatal(err)
	}
	stats, err := f.s.store.WelcomeDeliveryStats(ctx, "domains", u.DomainID)
	if err != nil || stats.Retrying != 1 || stats.Pending != 1 {
		t.Fatal(stats, err)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_welcome_message`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE calendar_deliveries SET next_attempt=0,lease_until=0 WHERE id=?`, job.ID); err != nil {
		t.Fatal(err)
	}
	winner, err := f.s.store.ClaimCalendarDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(winner.Data, job.Data) {
		t.Fatal("retry changed frozen data")
	}
	// Simulate a restarted server using the same durable queue and mailstore.
	restarted := New(f.s.cfg, f.s.log, f.s.store, f.s.authn, f.s.ms, nil, nil)
	if err = restarted.deliverCalendarJob(ctx, winner); err != nil {
		t.Fatal(err)
	}
	if len(welcomeMessages(t, f, u)) != 1 || welcomeFiles(t, f, u) != 1 {
		t.Fatal("duplicate after retry")
	}
	// The old lease cannot invoke filesystem publication, even after the message
	// has moved away from its original new/ name.
	message := welcomeMessages(t, f, u)[0]
	original := f.s.ms.Abs(message.FilePath)
	if err = os.Rename(original, filepath.Join(filepath.Dir(filepath.Dir(original)), "cur", "moved:2,S")); err != nil {
		t.Fatal(err)
	}
	if err = f.s.deliverCalendarJob(ctx, job); !errors.Is(err, storage.ErrCalendarConflict) {
		t.Fatal("stale worker not fenced", err)
	}
	if welcomeFiles(t, f, u) != 0 || len(welcomeMessages(t, f, u)) != 1 {
		t.Fatal("stale worker recreated file")
	}
	if _, err = f.s.store.ClaimCalendarDelivery(ctx); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal("completed job claimed again", err)
	}
}

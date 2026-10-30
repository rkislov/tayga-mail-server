package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// The same transaction assertions run on SQLite and an isolated PostgreSQL
// schema when TAYGA_TEST_BOOKING_POSTGRES_DSN is supplied.
func TestXMPPDeviceStorageTransactions(t *testing.T) {
	f := newBookingFixture(t)
	s, ctx := f.s, context.Background()
	other := meetingUser(t, f, "other")
	device := &XMPPDevicePassword{ID: NewID(), UserID: f.user.ID, Name: "Phone", SecretHash: "digest", CreatedAt: time.Now().UTC()}
	if err := s.CreateXMPPDevicePassword(ctx, device); err != nil {
		t.Fatal(err)
	}
	if err := s.SetXMPPDevicesOnly(ctx, f.user.ID, true); !errors.Is(err, ErrXMPPDeviceNotTested) {
		t.Fatal(err)
	}
	if err := s.TouchXMPPDevicePassword(ctx, f.user.ID, device.ID); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, owner, credential string }{{"phone", f.user.ID, device.ID}, {"legacy", f.user.ID, ""}, {"foreign", other.ID, ""}} {
		_, err := s.db.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_push_registrations(id,user_id,owner,service,node,credential_id,options_xml,resource) VALUES(?,?,?,?,?,?,'','')`), row.id, row.owner, row.owner+"@example.com", "push.example.com", row.id, row.credential)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.db.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_push_jobs(registration_id) VALUES(?)`), row.id)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RevokeXMPPDevicePassword(ctx, other.ID, device.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	// Fail push deletion after updating the credential: the hash and status must roll back.
	undo := failCalendarWrite(t, s, "xmpp_push_registrations", "DELETE", "")
	if err := s.RevokeXMPPDevicePassword(ctx, f.user.ID, device.ID); err == nil {
		t.Fatal("injected failure ignored")
	}
	kept, err := s.GetXMPPDevicePassword(ctx, f.user.ID, device.ID)
	if err != nil || kept.RevokedAt != nil || kept.SecretHash != "digest" {
		t.Fatal("partial revocation committed", err)
	}
	undo()
	if err = s.SetXMPPDevicesOnly(ctx, f.user.ID, true); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM xmpp_push_registrations`).Scan(&n); err != nil || n != 2 {
		t.Fatal("legacy pruning", n, err)
	}
	if err = s.RevokeXMPPDevicePassword(ctx, f.user.ID, device.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM xmpp_push_jobs`).Scan(&n); err != nil || n != 1 {
		t.Fatal("push cascade / owner isolation", n, err)
	}
	kept, err = s.GetXMPPDevicePassword(ctx, f.user.ID, device.ID)
	if err != nil || kept.RevokedAt == nil || kept.SecretHash != "" {
		t.Fatal("revocation not persisted", err)
	}
	if err = s.TouchXMPPDevicePassword(ctx, f.user.ID, device.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("revoked device touched", err)
	}
	if only, err := s.XMPPDevicesOnly(ctx, f.user.ID); err != nil || !only {
		t.Fatal("policy lost", only, err)
	}
}

func TestXMPPDeviceConcurrentLimit(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	for i := 0; i < 49; i++ {
		if err := f.s.CreateXMPPDevicePassword(ctx, &XMPPDevicePassword{ID: NewID(), UserID: f.user.ID, Name: fmt.Sprint(i), SecretHash: "hash", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	results := make(chan error, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- f.s.CreateXMPPDevicePassword(ctx, &XMPPDevicePassword{ID: NewID(), UserID: f.user.ID, Name: "parallel", SecretHash: "hash", CreatedAt: time.Now().UTC()})
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrXMPPDeviceLimit) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatal("limit raced", successes)
	}
}

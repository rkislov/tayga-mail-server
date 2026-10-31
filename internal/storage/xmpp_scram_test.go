package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestXMPPSCRAMTransactions(t *testing.T) {
	f := newBookingFixture(t)
	s, ctx := f.s, context.Background()
	salt, err := s.EnsureXMPPSCRAMSalt(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.EnsureXMPPSCRAMSalt(ctx, f.user.ID)
	if err != nil || again != salt {
		t.Fatal("salt changed", err)
	}
	k := XMPPSCRAMKey{SourceHash: f.user.PasswordHash, StoredKey: "stored", ServerKey: "server"}
	if err = s.SaveXMPPSCRAMKey(ctx, f.user.ID, k); err != nil {
		t.Fatal(err)
	}
	undo := failCalendarWrite(t, s, "xmpp_scram_keys", "UPDATE", "")
	if err = s.UpdateUserPasswordWithSCRAM(ctx, f.user.ID, "replacement", k); err == nil {
		t.Fatal("injected verifier failure ignored")
	}
	user, _ := s.GetUserByID(ctx, f.user.ID)
	if user.PasswordHash != f.user.PasswordHash {
		t.Fatal("partial password change")
	}
	undo()
	if err = s.UpdateUserPasswordWithSCRAM(ctx, f.user.ID, "replacement", k); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveXMPPSCRAMKey(ctx, f.user.ID, k); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("old login replaced new verifier", err)
	}
	_, keys, err := s.XMPPSCRAMKeys(ctx, f.user.ID)
	if err != nil || len(keys) != 1 || keys[0].SourceHash != "replacement" {
		t.Fatal(keys, err)
	}
	d := &XMPPDevicePassword{ID: NewID(), UserID: f.user.ID, Name: "fixture", SecretHash: "digest", CreatedAt: time.Now().UTC()}
	k.CredentialID = d.ID
	k.SourceHash = d.SecretHash
	d.SCRAM = &k
	if err = s.CreateXMPPDevicePassword(ctx, d); err != nil {
		t.Fatal(err)
	}
	undo = failCalendarWrite(t, s, "xmpp_scram_keys", "DELETE", "")
	if err = s.RevokeXMPPDevicePassword(ctx, f.user.ID, d.ID); err == nil {
		t.Fatal("partial revoke")
	}
	kept, _ := s.GetXMPPDevicePassword(ctx, f.user.ID, d.ID)
	if kept.RevokedAt != nil {
		t.Fatal("partial device revoke")
	}
	undo()
	if err = s.RevokeXMPPDevicePassword(ctx, f.user.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveXMPPSCRAMKey(ctx, f.user.ID, k); !errors.Is(err, ErrNotFound) {
		t.Fatal("re-enrolled revoked credential", err)
	}
	_, keys, err = s.XMPPSCRAMKeys(ctx, f.user.ID)
	if err != nil || len(keys) != 1 {
		t.Fatal("revoked verifier retained", err)
	}
}

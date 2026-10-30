package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestOwnXMPPDevicesAPIAndScope(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	base := "/api/v1/me/xmpp-devices"
	f.call(-1, "GET", base, "", 401)
	f.call(0, "POST", base, `{"name":""}`, 400)
	f.call(0, "POST", base, `{"name":"Phone","user_id":"other"}`, 400)
	w := f.call(0, "POST", base, `{"name":"Phone"}`, 201)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("secret can be cached")
	}
	var created struct {
		Device   storage.XMPPDevicePassword
		Password string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Password) < 60 || created.Device.SecretHash != "" || strings.Contains(w.Body.String(), "secret_hash") {
		t.Fatal("invalid secret response")
	}
	u, credential, err := f.s.authn.AuthenticateXMPP(ctx, "owner@example.com", created.Password)
	if err != nil || credential != created.Device.ID {
		t.Fatal(credential, err)
	}
	if _, _, err = f.s.authn.AuthenticateXMPP(ctx, "reader@example.com", created.Password); err == nil {
		t.Fatal("cross-account credential accepted")
	}
	if _, err = f.s.authn.Authenticate(ctx, u.Email, created.Password); err == nil {
		t.Fatal("general password authentication accepted scoped secret")
	}
	if _, err = f.s.authn.LoginWithPassword(ctx, u.Email, created.Password); err == nil {
		t.Fatal("web login accepted scoped secret")
	}
	stored, err := f.s.store.GetXMPPDevicePassword(ctx, u.ID, credential)
	if err != nil || stored.SecretHash == "" || strings.Contains(stored.SecretHash, created.Password) {
		t.Fatal("secret storage", err)
	}
	f.call(1, "DELETE", base+"/"+credential, "", 404)
	if strings.Contains(f.call(1, "GET", base, "", 200).Body.String(), credential) {
		t.Fatal("other user saw credential")
	}
	f.call(0, "PUT", base+"/policy", `{"devices_only":true}`, 409)
	var closed atomic.Bool
	session, err := f.s.authn.TrackXMPPSession(ctx, u, credential, "127.0.0.1", "XMPP · phone", "device-fixture", func() error { closed.Store(true); return nil })
	if err != nil {
		t.Fatal(err)
	}
	f.call(1, "DELETE", base+"/sessions/"+session, "", 404)
	w = f.call(0, "GET", base, "", 200)
	if !strings.Contains(w.Body.String(), session) || strings.Contains(w.Body.String(), created.Password) || strings.Contains(w.Body.String(), stored.SecretHash) {
		t.Fatal("session listing / secret disclosure")
	}
	f.call(0, "PUT", base+"/policy", `{"devices_only":true}`, 200)
	if closed.Load() {
		t.Fatal("device connection killed by migration")
	}
	f.call(0, "DELETE", base+"/"+credential, "", 200)
	if !closed.Load() {
		t.Fatal("revocation did not close connection")
	}
	f.call(0, "DELETE", base+"/"+credential, "", 200) // Idempotent.
	f.call(0, "GET", base, "", 200)                   // Web remains usable even when last device is revoked.
	if _, _, err = f.s.authn.AuthenticateXMPP(ctx, u.Email, created.Password); err == nil {
		t.Fatal("revoked password authenticated")
	}
	if _, err = f.s.authn.TrackXMPPSession(ctx, u, credential, "", "", "late", nil); err == nil {
		t.Fatal("auth before revoke / registration after revoke accepted")
	}
}

func TestXMPPDeviceConcurrentRegistrationAndRevocation(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	u, err := f.s.store.GetUserByID(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 15; round++ {
		d, _, err := f.s.authn.CreateXMPPDevicePassword(ctx, u.ID, "race")
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		var closed atomic.Bool
		var registerErr, revokeErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, registerErr = f.s.authn.TrackXMPPSession(ctx, u, d.ID, "", "", "race-"+d.ID, func() error { closed.Store(true); return nil })
		}()
		go func() { defer wg.Done(); <-start; revokeErr = f.s.authn.RevokeXMPPDevicePassword(ctx, u.ID, d.ID) }()
		close(start)
		wg.Wait()
		if revokeErr != nil {
			t.Fatal(revokeErr)
		}
		if registerErr == nil && !closed.Load() {
			t.Fatal("registered connection escaped revocation")
		}
		for _, session := range f.s.authn.Sessions() {
			if session.CredentialID == d.ID {
				t.Fatal("revoked connection still registered")
			}
		}
	}
}

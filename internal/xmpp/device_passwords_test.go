package xmpp

import (
	"context"
	"encoding/base64"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
)

func TestXMPPDevicePasswordRevokeAndResume(t *testing.T) {
	srv, store, user := testServer(t)
	srv.authn = auth.NewLayer(store, config.LDAPConfig{}, config.MFAConfig{}, config.OIDCConfig{})
	ctx := context.Background()
	hash, err := srv.authn.Hasher.Hash("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.UpdateUserPassword(ctx, user.ID, hash); err != nil {
		t.Fatal(err)
	}
	device, password, err := srv.authn.CreateXMPPDevicePassword(ctx, user.ID, "Phone")
	if err != nil {
		t.Fatal(err)
	}
	other, otherPassword, err := srv.authn.CreateXMPPDevicePassword(ctx, user.ID, "Desktop")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	phone := newResumeWireClientWithPassword(t, srv, ln, "phone", password)
	phone.send(`<enable xmlns='urn:xmpp:sm:3' resume='true'/>`)
	token := phone.want("enabled", "").ID
	desktop := newResumeWireClientWithPassword(t, srv, ln, "", otherPassword)
	desktop.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='0'/>`)
	if failed := desktop.want("failed", ""); !strings.Contains(failed.Inner, "item-not-found") {
		t.Fatal(failed)
	}
	phone.send(`<r xmlns='urn:xmpp:sm:3'/>`)
	phone.want("a", "") // Wrong credential did not evict the old connection.
	reconnect := newResumeWireClientWithPassword(t, srv, ln, "", password)
	reconnect.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='0'/>`)
	reconnect.want("resumed", "")
	if e := reconnect.next(); e.XMLName.Local != "r" {
		t.Fatal(e)
	}
	select {
	case <-phone.done:
	case <-time.After(4 * time.Second):
		t.Fatal("old connection remained")
	}
	// The native enable request binds its registration to the SASL credential.
	srv.cfg.XMPP.Components = []config.XMPPComponentConfig{{Domain: "push.example.com", Secret: "fixture", Push: true}}
	srv.hub.RegisterComponent(&ComponentConn{Domain: "push.example.com", Send: func([]byte) {}})
	reconnect.send(`<iq type='set' id='push'><enable xmlns='urn:xmpp:push:0' jid='push.example.com' node='phone'/></iq>`)
	reconnect.want("iq", "push")
	var registration, credential string
	if err = store.DB().QueryRow(`SELECT id,credential_id FROM xmpp_push_registrations WHERE node='phone'`).Scan(&registration, &credential); err != nil || credential != device.ID {
		t.Fatal("push not bound to credential", credential, err)
	}
	_, err = store.DB().Exec(`INSERT INTO xmpp_push_jobs(registration_id) VALUES(?)`, registration)
	if err != nil {
		t.Fatal(err)
	}
	if err = srv.authn.RevokeXMPPDevicePassword(ctx, user.ID, device.ID); err != nil {
		t.Fatal(err)
	}
	srv.DropDeviceStreams(user.ID, device.ID)
	select {
	case <-reconnect.done:
	case <-time.After(4 * time.Second):
		t.Fatal("revoked device still connected")
	}
	var count int
	if err = store.DB().QueryRow(`SELECT COUNT(*) FROM xmpp_push_jobs`).Scan(&count); err != nil || count != 0 {
		t.Fatal("push queue survived revocation", count, err)
	}
	if _, _, err = srv.authn.AuthenticateXMPP(ctx, user.Email, password); err == nil {
		t.Fatal("revoked password authenticated")
	}
	if _, _, err = srv.authn.AuthenticateXMPP(ctx, user.Email, otherPassword); err != nil {
		t.Fatal("other device affected", err)
	}
	// A remembered token cannot be resumed by another still-valid device.
	desktop.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='0'/>`)
	desktop.want("failed", "")
	desktop.send(`<iq type='set' id='bind'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'><resource>desktop</resource></bind></iq>`)
	desktop.want("iq", "bind")
	desktop.send(`<enable xmlns='urn:xmpp:sm:3' resume='true'/>`)
	retained := desktop.want("enabled", "").ID
	desktop.conn.Close()
	<-desktop.done
	srv.resumeMu.Lock()
	_, exists := srv.resumable[retained]
	srv.resumeMu.Unlock()
	if !exists {
		t.Fatal("valid stream was not retained")
	}
	if err = srv.authn.RevokeXMPPDevicePassword(ctx, user.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	srv.DropDeviceStreams(user.ID, other.ID)
	srv.resumeMu.Lock()
	_, exists = srv.resumable[retained]
	srv.resumeMu.Unlock()
	if exists {
		t.Fatal("revoked retained stream survived")
	}
	// A delayed push command must not re-create a registration after revocation.
	if err = desktop.session.handlePushIQ("late-push", "set", "", `<enable xmlns='urn:xmpp:push:0' jid='push.example.com' node='late'/>`); err == nil {
		t.Fatal("revoked session registered push")
	}
	// Even a delayed reader finishing after purge may not retain it again.
	desktop.session.retainStream()
	srv.resumeMu.Lock()
	_, exists = srv.resumable[retained]
	srv.resumeMu.Unlock()
	if exists {
		t.Fatal("late retain resurrected revoked stream")
	}
}

func TestXMPPDeviceOnlyMigrationKeepsMailAndWeb(t *testing.T) {
	srv, store, user := testServer(t)
	ctx := context.Background()
	srv.authn = auth.NewLayer(store, config.LDAPConfig{}, config.MFAConfig{}, config.OIDCConfig{})
	hash, _ := srv.authn.Hasher.Hash("fixture-password")
	if err := store.UpdateUserPassword(ctx, user.ID, hash); err != nil {
		t.Fatal(err)
	}
	_, password, err := srv.authn.CreateXMPPDevicePassword(ctx, user.ID, "Tested client")
	if err != nil {
		t.Fatal(err)
	}
	if err = srv.authn.SetXMPPDevicesOnly(ctx, user.ID, true); err == nil {
		t.Fatal("enabled before client was tested")
	}
	plainClient, plainServer := net.Pipe()
	unencrypted := newSession(srv, plainServer, false)
	if err = unencrypted.saslAuth("PLAIN", base64.StdEncoding.EncodeToString([]byte("\x00"+user.Email+"\x00"+password))); err == nil {
		t.Fatal("device password accepted without TLS")
	}
	plainClient.Close()
	plainServer.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	legacy := newResumeWireClient(t, srv, ln, "legacy")
	modern := newResumeWireClientWithPassword(t, srv, ln, "device", password)
	if err = srv.authn.SetXMPPDevicesOnly(ctx, user.ID, true); err != nil {
		t.Fatal(err)
	}
	srv.DropDeviceStreams(user.ID, "")
	select {
	case <-legacy.done:
	case <-time.After(4 * time.Second):
		t.Fatal("legacy client remained connected")
	}
	modern.send(`<iq type='get' id='alive'><ping xmlns='urn:xmpp:ping'/></iq>`)
	modern.want("iq", "alive")
	if _, err = srv.authn.Authenticate(ctx, user.Email, "fixture-password"); err != nil {
		t.Fatal("mail login broken", err)
	}
	if _, err = srv.authn.Authenticate(ctx, user.Email, password); err == nil {
		t.Fatal("device secret escaped XMPP scope")
	}
	login, err := srv.authn.LoginWithPassword(ctx, user.Email, "fixture-password")
	if err != nil {
		t.Fatal("web login broken", err)
	}
	for mech, secret := range map[string]string{"PLAIN": "fixture-password", "X-OAUTH2": login.Tokens.AccessToken} {
		client, server := net.Pipe()
		s := newSession(srv, server, true)
		err = s.saslAuth(mech, base64.StdEncoding.EncodeToString([]byte("\x00"+user.Email+"\x00"+secret)))
		client.Close()
		server.Close()
		if err == nil {
			t.Fatalf("legacy %s bypassed policy", mech)
		}
	}
	if err = srv.authn.SetXMPPDevicesOnly(ctx, user.ID, false); err != nil {
		t.Fatal(err)
	}
	restored := newResumeWireClient(t, srv, ln, "restored")
	restored.send(`<iq type='get' id='alive'><ping xmlns='urn:xmpp:ping'/></iq>`)
	restored.want("iq", "alive")
}

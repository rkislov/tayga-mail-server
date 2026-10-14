package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestMAMPreferencesRoundTripAndArchivePolicy(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()
	var out bytes.Buffer
	s := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&out)}
	if err := s.handleMAMIQ(ctx, "get", "get", `<prefs xmlns='urn:xmpp:mam:2'/>`); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`default="always"`, `<always>`, `<never>`} {
		if !strings.Contains(out.String(), want) {
			t.Fatal("incomplete preferences", out.String())
		}
	}
	out.Reset()
	if err := s.handleMAMIQ(ctx, "set", "set", `<prefs xmlns='urn:xmpp:mam:2' default='never'><always><jid>Bob@EXAMPLE.COM</jid></always><never><jid>bob@example.com/private</jid></never></prefs>`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "type='result'") {
		t.Fatal(out.String())
	}
	for _, tc := range []struct {
		peer string
		want bool
	}{{"bob@example.com/mobile", true}, {"bob@example.com/private", false}, {"carol@example.com", false}} {
		stanza := `<message from='` + tc.peer + `' to='` + u.Email + `'><body>hello</body></message>`
		if err := srv.archiveMAM(ctx, u.Email, ParseJID(tc.peer).Bare(), tc.peer, stanza); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := srv.db.QueryRow("SELECT COUNT(*) FROM xmpp_mam WHERE owner_bare_jid=? AND stanza_id=?", u.Email, tc.peer).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if (count > 0) != tc.want {
			t.Fatalf("%s count=%d", tc.peer, count)
		}
	}
	// Rules do not leak into another owner's archive or retroactively remove history.
	allowed, err := srv.shouldArchive(ctx, "other@example.com", "carol@example.com", `<message/>`)
	if err != nil || !allowed {
		t.Fatal(allowed, err)
	}
	p, err := srv.mamPreferences(ctx, u.Email)
	if err != nil || p.Default != "never" || p.Always[0] != "bob@example.com" {
		t.Fatal(p, err)
	}
}
func TestMAMPreferencesRosterAndValidation(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()
	var out bytes.Buffer
	s := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&out)}
	for _, xml := range []string{`<prefs xmlns='urn:xmpp:mam:2' default='sometimes'/>`, `<prefs xmlns='urn:xmpp:mam:2' default='never'><always><jid>bad jid@example.com</jid></always></prefs>`, `<prefs xmlns='urn:xmpp:mam:2' default='always'><always><jid>bob@example.com</jid></always><never><jid>bob@example.com</jid></never></prefs>`, `<prefs xmlns='wrong' default='never'/>`} {
		out.Reset()
		_ = s.handleMAMIQ(ctx, "bad", "set", xml)
		if !strings.Contains(out.String(), "bad-request") {
			t.Fatal(out.String())
		}
	}
	out.Reset()
	_ = s.handleMAMIQ(ctx, "set", "set", `<prefs xmlns='urn:xmpp:mam:2' default='roster'/>`)
	if err := srv.UpsertChatRoster(ctx, u.ID, "bob@example.com", "Bob"); err != nil {
		t.Fatal(err)
	}
	for _, peer := range []string{"bob@example.com/phone", "stranger@example.com"} {
		allowed, err := srv.shouldArchive(ctx, u.Email, peer, `<message/>`)
		if err != nil || allowed != (peer == "bob@example.com/phone") {
			t.Fatal(peer, allowed, err)
		}
	}
}

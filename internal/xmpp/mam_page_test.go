package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMAMPagesAndOwnerScopedCursor(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()
	stamp := time.Now().UTC()
	for i := 0; i < 7; i++ {
		id := fmt.Sprintf("m%02d", i)
		_, err := srv.db.ExecContext(ctx, `INSERT INTO xmpp_mam(id,owner_bare_jid,with_bare_jid,stanza_id,stanza,created_at) VALUES(?,?,?,?,?,?)`, id, u.Email, "bob@example.com", id, "<message/>", stamp)
		if err != nil {
			t.Fatal(err)
		}
	}
	f := mamFilter{Limit: 3, With: "bob@example.com"}
	rows, total, complete, err := srv.mamPage(ctx, u.Email, f)
	if err != nil || total != 7 || complete || len(rows) != 3 || rows[0].StanzaID != "m00" {
		t.Fatalf("first: %+v %d %t %v", rows, total, complete, err)
	}
	after := rows[2].StanzaID
	f.After = &after
	rows, _, complete, err = srv.mamPage(ctx, u.Email, f)
	if err != nil || complete || len(rows) != 3 || rows[0].StanzaID != "m03" {
		t.Fatalf("second: %+v %t %v", rows, complete, err)
	}
	after = rows[2].StanzaID
	rows, _, complete, err = srv.mamPage(ctx, u.Email, f)
	if err != nil || !complete || len(rows) != 1 || rows[0].StanzaID != "m06" {
		t.Fatalf("last: %+v %t %v", rows, complete, err)
	}
	before := ""
	f.After = nil
	f.Before = &before
	rows, _, complete, err = srv.mamPage(ctx, u.Email, f)
	if err != nil || complete || len(rows) != 3 || rows[0].StanzaID != "m04" {
		t.Fatalf("backward: %+v %t %v", rows, complete, err)
	}
	before = "m00"
	_, _, _, err = srv.mamPage(ctx, "other@example.com", f)
	if err != errMAMCursor {
		t.Fatalf("cross-account cursor: %v", err)
	}
	f.Before = nil
	f.Limit = 0
	rows, total, _, err = srv.mamPage(ctx, u.Email, f)
	if err != nil || len(rows) != 0 || total != 7 {
		t.Fatalf("count only %d %d %v", len(rows), total, err)
	}
	later := stamp.Add(time.Second)
	f.Start = &later
	f.Limit = 3
	rows, total, _, err = srv.mamPage(ctx, u.Email, f)
	if err != nil || len(rows) != 0 || total != 0 {
		t.Fatalf("date filter %d %d %v", len(rows), total, err)
	}
}

func TestMAMNamespacedFormFilters(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()
	for _, peer := range []string{"bob@example.com", "carol@example.com"} {
		if err := srv.archiveMAM(ctx, u.Email, peer, peer, `<message><body>hello</body></message>`); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	s := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&out)}
	// readInnerXML adds namespaces on child nodes, as on a real C2S stream.
	dec := xml.NewDecoder(strings.NewReader(`<iq type='set'><query xmlns='urn:xmpp:mam:2' queryid='q'><x xmlns='jabber:x:data' type='submit'><field var='with'><value>bob@example.com</value></field></x><set xmlns='http://jabber.org/protocol/rsm'><max>1</max></set></query></iq>`))
	tok, _ := dec.Token()
	inner, err := readInnerXML(dec, tok.(xml.StartElement))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.handleMAMIQ(ctx, "iq1", "set", inner); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Count(got, "<result ") != 1 || strings.Contains(got, "carol@example.com") || !strings.Contains(got, "<count>1</count>") || !strings.Contains(got, "complete='true'") {
		t.Fatal(got)
	}
}

func TestMAMQueryFormDiscovery(t *testing.T) {
	var out bytes.Buffer
	s := &Session{JID: ParseJID("alice@example.com/phone"), bw: bufio.NewWriter(&out)}
	if err := s.handleMAMIQ(context.Background(), "discover", "get", `<query xmlns="urn:xmpp:mam:2"/>`); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type='result'", "type='form'", "var='with'", "var='start'", "var='end'"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal(out.String())
		}
	}
}

func TestMAMCursorOutsideQueryFilters(t *testing.T) {
	srv, _, u := testServer(t)
	stamp := time.Now().UTC()
	for i, peer := range []string{"bob@example.com", "carol@example.com", "bob@example.com"} {
		id := fmt.Sprintf("m%d", i)
		if _, err := srv.db.Exec(`INSERT INTO xmpp_mam(id,owner_bare_jid,with_bare_jid,stanza_id,stanza,created_at) VALUES(?,?,?,?,?,?)`, id, u.Email, peer, id, "<message/>", stamp.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	// A client can use a known archive cursor while narrowing the next query
	// to a conversation or time range that no longer includes that cursor.
	cursor := "m1"
	start, end := stamp.Add(2*time.Second), stamp
	for _, tc := range []struct {
		name string
		f    mamFilter
		want string
	}{
		{"peer-after", mamFilter{Limit: 10, With: "bob@example.com", After: &cursor}, "m2"},
		{"peer-before", mamFilter{Limit: 10, With: "bob@example.com", Before: &cursor}, "m0"},
		{"start-after", mamFilter{Limit: 10, Start: &start, After: &cursor}, "m2"},
		{"end-before", mamFilter{Limit: 10, End: &end, Before: &cursor}, "m0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, _, complete, err := srv.mamPage(context.Background(), u.Email, tc.f)
			if err != nil || !complete || len(rows) != 1 || rows[0].StanzaID != tc.want {
				t.Fatalf("filtered cursor: rows=%+v complete=%t err=%v", rows, complete, err)
			}
		})
	}
}

func TestMAMRejectsUnsupportedOrAmbiguousFilters(t *testing.T) {
	srv, _, u := testServer(t)
	if err := srv.archiveMAM(context.Background(), u.Email, "bob@example.com", "private", `<message><body>not requested</body></message>`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, fields, condition string }{
		{"unknown", `<field var='{urn:example}text'><value>match nothing</value></field>`, "feature-not-implemented"},
		{"extension", `<field var='ids'><value>missing</value></field>`, "feature-not-implemented"},
		{"invalid-jid", `<field var='with'><value>not a jid</value></field>`, "bad-request"},
		{"empty-jid", `<field var='with'><value/></field>`, "bad-request"},
		{"duplicate", `<field var='with'><value>bob@example.com</value></field><field var='with'><value>carol@example.com</value></field>`, "bad-request"},
		{"multiple-values", `<field var='with'><value>bob@example.com</value><value>carol@example.com</value></field>`, "bad-request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			s := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&out)}
			query := `<query xmlns='urn:xmpp:mam:2'><x xmlns='jabber:x:data' type='submit'><field var='FORM_TYPE' type='hidden'><value>urn:xmpp:mam:2</value></field>` + tc.fields + `</x></query>`
			if err := s.handleMAMIQ(context.Background(), "filter", "set", query); err != nil {
				t.Fatal(err)
			}
			if got := out.String(); !strings.Contains(got, tc.condition) || strings.Contains(got, "<result ") || strings.Contains(got, "<fin ") {
				t.Fatalf("filter must fail without streaming history: %s", got)
			}
		})
	}
}

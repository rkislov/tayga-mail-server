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

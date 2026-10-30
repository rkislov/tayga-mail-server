package xmpp

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/storage/migrations"
)

func TestRoomArchive(t *testing.T) {
	srv, st, alice := testServer(t)
	testRoomArchive(t, srv, st, alice)
}
func testRoomArchive(t *testing.T, srv *Server, st *storage.Store, alice *storage.User) {
	t.Helper()
	testRoomArchiveMigration(t, srv, alice.Email)
	ctx := context.Background()
	bob, err := st.CreateUser(ctx, &storage.User{TenantID: alice.TenantID, DomainID: alice.DomainID, Email: "bob@example.com", LocalPart: "bob", Enabled: true, AuthSource: "local"})
	if err != nil {
		t.Fatal(err)
	}
	room, err := srv.CreateChatRoom(ctx, alice.Email, "Archive", []string{bob.Email})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := 0; i < 257; i++ {
		ev, e := srv.sendRoomMessage(ctx, alice.Email, room.JID, fmt.Sprintf("message %03d", i))
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, ev.ID)
	}
	// Receive order must survive timestamp ties and clock regressions.
	if _, err = srv.db.ExecContext(ctx, srv.rebind(`UPDATE web_chat_messages SET created_at=? WHERE room_jid=?`), "2026-10-30T08:00:00.000000000Z", room.JID); err != nil {
		t.Fatal(err)
	}
	if _, err = srv.db.ExecContext(ctx, srv.rebind(`UPDATE web_chat_messages SET created_at=? WHERE id=?`), "2026-10-29T08:00:00.000000000Z", ids[256]); err != nil {
		t.Fatal(err)
	}
	cursor := ""
	var got []string
	for i := 0; i < 3; i++ {
		p, e := srv.RoomHistoryPage(ctx, bob.Email, room.JID, cursor, 100)
		if e != nil {
			t.Fatal(e)
		}
		if p.Total != 257 || p.Complete != (i == 2) {
			t.Fatal(p.Total, p.Complete)
		}
		var chunk []string
		for _, m := range p.Messages {
			chunk = append(chunk, m.ID)
		}
		got = append(chunk, got...)
		cursor = p.First
	}
	if strings.Join(got, ",") != strings.Join(ids, ",") {
		t.Fatal("backward traversal lost receive order")
	}
	got = nil
	var after *string
	for i := 0; i < 3; i++ {
		p, e := srv.roomArchivePage(ctx, bob.Email, room.JID, mamFilter{After: after, Limit: 100})
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range p.Messages {
			got = append(got, m.ID)
		}
		last := p.Last
		after = &last
	}
	if strings.Join(got, ",") != strings.Join(ids, ",") {
		t.Fatal("forward traversal lost order")
	}
	p, err := srv.roomArchivePage(ctx, bob.Email, room.JID, mamFilter{Limit: 0})
	if err != nil || len(p.Messages) != 0 || p.Total != 257 || p.Complete {
		t.Fatal(p, err)
	}
	stamp := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	p, err = srv.roomArchivePage(ctx, bob.Email, room.JID, mamFilter{Limit: 100, Start: &stamp, End: &stamp})
	if err != nil || p.Total != 256 {
		t.Fatal(p.Total, err)
	}
	other, err := srv.CreateChatRoom(ctx, alice.Email, "Other", []string{bob.Email})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = srv.RoomHistoryPage(ctx, bob.Email, other.JID, ids[0], 80); err != ErrRoomCursor {
		t.Fatal("foreign cursor accepted", err)
	}
	if _, err = srv.RoomHistoryPage(ctx, bob.Email, room.JID, "missing", 80); err != ErrRoomCursor {
		t.Fatal(err)
	}
	// An older binary can still write default-zero sequence rows after rollback.
	for i, stamp := range []string{"2026-10-30T09:00:00.1Z", "2026-10-30T09:00:00Z"} {
		_, err = srv.db.ExecContext(ctx, srv.rebind(`INSERT INTO web_chat_messages(id,room_jid,sender,body,created_at) VALUES(?,?,?,?,?)`), fmt.Sprintf("legacy-%d", i), room.JID, alice.Email, "legacy", stamp)
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err = srv.RoomHistoryPage(ctx, bob.Email, room.JID, "", 2)
	if err != nil || len(p.Messages) != 2 || p.Messages[0].ID != "legacy-1" || p.Messages[1].ID != "legacy-0" {
		t.Fatal(p, err)
	}
	ev, err := srv.sendRoomMessage(ctx, alice.Email, room.JID, "after rollback")
	if err != nil {
		t.Fatal(err)
	}
	p, err = srv.RoomHistoryPage(ctx, bob.Email, room.JID, "", 1)
	if err != nil || p.Last != ev.ID {
		t.Fatal(p, err)
	}
	// Membership remains valid while disconnected; removal revokes all pages and
	// joining history, and re-adding deliberately restores retained room history.
	if err = srv.UpdateChatRoom(ctx, alice.Email, room.JID, bob.Email, "none", false); err != nil {
		t.Fatal(err)
	}
	if _, err = srv.RoomHistoryPage(ctx, bob.Email, room.JID, "", 80); err != ErrRoomAccess {
		t.Fatal("removed member read archive", err)
	}
	if err = srv.UpdateChatRoom(ctx, alice.Email, room.JID, bob.Email, "member", false); err != nil {
		t.Fatal(err)
	}
	p, err = srv.RoomHistoryPage(ctx, bob.Email, room.JID, "", 80)
	if err != nil || p.Total != 260 {
		t.Fatal(p.Total, err)
	}
	tenant, err := st.CreateTenant(ctx, "foreign")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = srv.db.ExecContext(ctx, srv.rebind(`UPDATE users SET tenant_id=? WHERE id=?`), tenant.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = srv.RoomHistoryPage(ctx, bob.Email, room.JID, "", 80); err != ErrRoomAccess {
		t.Fatal("moved tenant retained access", err)
	}
	rooms, err := srv.ListChatRooms(ctx, bob.Email)
	if err != nil || len(rooms) != 0 {
		t.Fatal("foreign room metadata", rooms, err)
	}
	// A failed archive insert rolls back the sequence allocation too.
	srv.mucMu.Lock()
	_, err = srv.sendRoomMessageAs(ctx, alice.Email, other.JID, "first origin", "Alice", "duplicate-origin", alice.Email+"/test")
	srv.mucMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	var before, afterSeq int64
	if err = srv.db.QueryRowContext(ctx, srv.rebind(`SELECT next_archive_seq FROM web_chat_rooms WHERE jid=?`), other.JID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	srv.mucMu.Lock()
	_, err = srv.sendRoomMessageAs(ctx, alice.Email, other.JID, "duplicate origin", "Alice", "duplicate-origin", alice.Email+"/test")
	srv.mucMu.Unlock()
	if err == nil {
		t.Fatal("duplicate insert succeeded")
	}
	if err = srv.db.QueryRowContext(ctx, srv.rebind(`SELECT next_archive_seq FROM web_chat_rooms WHERE jid=?`), other.JID).Scan(&afterSeq); err != nil || before != afterSeq {
		t.Fatal("counter escaped rollback", before, afterSeq, err)
	}

}

func TestRoomMAMWireAndMembership(t *testing.T) {
	srv, st, alice := testServer(t)
	ctx := context.Background()
	bob, err := st.CreateUser(ctx, &storage.User{TenantID: alice.TenantID, DomainID: alice.DomainID, Email: "bob@example.com", LocalPart: "bob", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	room, err := srv.CreateChatRoom(ctx, alice.Email, "Team", []string{bob.Email})
	if err != nil {
		t.Fatal(err)
	}
	a, ab := mucTestSession(srv, alice, "phone")
	b, bb := mucTestSession(srv, bob, "desktop")
	mucInput(t, a, `<presence to='`+room.JID+`/Alice'><x xmlns='`+mucNS+`'/></presence>`)
	mucInput(t, b, `<presence to='`+room.JID+`/Bob'><x xmlns='`+mucNS+`'/></presence>`)
	ab.Reset()
	bb.Reset()
	message := `<message type='groupchat' to='` + room.JID + `' id='origin'><body>A &amp; B</body><x xmlns='` + mucNS + `#user'><item jid='forged@example.com'/></x></message>`
	mucInput(t, a, message)
	live := bb.String()
	bb.Reset()
	ab.Reset()
	mucInput(t, a, message)
	if bb.Len() != 0 {
		t.Fatal("duplicate broadcast")
	}
	ab.Reset()
	query := func(inner string) {
		bb.Reset()
		mucInput(t, b, `<iq id='page' type='set' to='`+room.JID+`'>`+inner+`</iq>`)
	}
	query(`<query xmlns='urn:xmpp:mam:2' queryid='client-page'><set xmlns='http://jabber.org/protocol/rsm'><max>100</max></set></query>`)
	var result struct {
		From   string `xml:"from,attr"`
		To     string `xml:"to,attr"`
		Result struct {
			ID        string `xml:"id,attr"`
			QueryID   string `xml:"queryid,attr"`
			Forwarded struct {
				Message struct {
					XMLName xml.Name
					From    string `xml:"from,attr"`
					To      string `xml:"to,attr"`
					Body    string `xml:"body"`
					SID     struct {
						ID string `xml:"id,attr"`
						By string `xml:"by,attr"`
					} `xml:"urn:xmpp:sid:0 stanza-id"`
					X struct {
						Item struct {
							JID string `xml:"jid,attr"`
						} `xml:"item"`
					} `xml:"http://jabber.org/protocol/muc#user x"`
				} `xml:"jabber:client message"`
			} `xml:"urn:xmpp:forward:0 forwarded"`
		} `xml:"urn:xmpp:mam:2 result"`
	}
	// Assert using decoded XML, including the namespace inherited by forwarded data.
	var outer roomMAMElement
	dec := xml.NewDecoder(strings.NewReader(bb.String()))
	if err = dec.Decode(&outer); err != nil {
		t.Fatal(err)
	}
	dec = xml.NewDecoder(strings.NewReader(bb.String()))
	if err = dec.Decode(&result); err != nil {
		t.Fatal(err)
	}
	m := result.Result.Forwarded.Message
	if result.From != room.JID || result.To != bob.Email+"/desktop" || result.Result.QueryID != "client-page" || result.Result.ID != m.SID.ID || m.Body != "A & B" || m.From != room.JID+"/Alice" || m.To != "" || m.SID.By != room.JID || m.SID.ID == "" || m.X.Item.JID != alice.Email+"/phone" || !strings.Contains(live, m.SID.ID) {
		t.Fatalf("invalid forwarded message: %+v %s", m, bb.String())
	}
	if ab.Len() != 0 {
		t.Fatal("archive leaked to another resource")
	}
	if !strings.Contains(bb.String(), "complete='true'") || !strings.Contains(bb.String(), "<count>1</count>") {
		t.Fatal(bb.String())
	}
	// Full JID filters distinguish sender resources; bare JID matches all of them.
	for _, with := range []string{alice.Email, alice.Email + "/phone", alice.Email + "/elsewhere"} {
		query(`<query xmlns='urn:xmpp:mam:2'><x xmlns='jabber:x:data' type='submit'><field var='with'><value>` + with + `</value></field></x></query>`)
		count := "1"
		if strings.HasSuffix(with, "/elsewhere") {
			count = "0"
		}
		if !strings.Contains(bb.String(), "<count>"+count+"</count>") {
			t.Fatal(with, bb.String())
		}
	}
	for _, inner := range []string{`<set xmlns='http://jabber.org/protocol/rsm'><after/><before/></set>`, `<set xmlns='http://jabber.org/protocol/rsm'><max>1</max><max>2</max></set>`, `<x xmlns='jabber:x:data' type='submit'><field var='start'><value>no</value></field></x>`, `<x xmlns='jabber:x:data' type='submit'><field var='search'><value>secret</value></field></x>`, `<flip-page/>`} {
		query(`<query xmlns='urn:xmpp:mam:2'>` + inner + `</query>`)
		if !strings.Contains(bb.String(), "type='error'") || strings.Contains(bb.String(), "<result xmlns=") {
			t.Fatal("query widened", inner, bb.String())
		}
	}
	if err = srv.UpdateChatRoom(ctx, alice.Email, room.JID, bob.Email, "none", false); err != nil {
		t.Fatal(err)
	}
	query(`<query xmlns='urn:xmpp:mam:2'/>`)
	if !strings.Contains(bb.String(), "forbidden") {
		t.Fatal(bb.String())
	}
	bb.Reset()
	if _, err = srv.sendRoomMessage(ctx, alice.Email, room.JID, "private after removal"); err != nil {
		t.Fatal(err)
	}
	if bb.Len() != 0 {
		t.Fatal("removed recipient received message")
	}
	mucInput(t, b, `<presence to='`+room.JID+`/Bob'><x xmlns='`+mucNS+`'/></presence>`)
	if strings.Contains(bb.String(), "private after removal") {
		t.Fatal("join history leaked")
	}
}

// Exercise upgrades with real pre-058 rows on SQLite and PostgreSQL, including
// the lexical whole-second/fractional-second boundary that used to sort wrong.
func testRoomArchiveMigration(t *testing.T, srv *Server, owner string) {
	t.Helper()
	db := "sqlite"
	if srv.cfg.Storage.Driver == "postgres" {
		db = "postgres"
	}
	raw, err := migrations.FS.ReadFile("058_room_archive." + db + ".sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if _, err = srv.db.Exec(parts[1]); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	room := "migration@web-groups.example.com"
	if _, err = srv.db.ExecContext(ctx, srv.rebind(`INSERT INTO web_chat_rooms(jid,name,owner_email) VALUES(?,?,?)`), room, "migration", owner); err != nil {
		t.Fatal(err)
	}
	for i, stamp := range []string{"2026-10-30T09:00:00.1Z", "2026-10-30T09:00:00Z"} {
		_, err = srv.db.ExecContext(ctx, srv.rebind(`INSERT INTO web_chat_messages(id,room_jid,sender,body,created_at) VALUES(?,?,?,?,?)`), fmt.Sprint("pre-", i), room, owner, "legacy", stamp)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err = srv.db.Exec(parts[0]); err != nil {
		t.Fatal(err)
	}
	var seq int
	var stamp, sender string
	if err = srv.db.QueryRow(`SELECT archive_seq,created_at,sender_jid FROM web_chat_messages WHERE id='pre-1'`).Scan(&seq, &stamp, &sender); err != nil || seq != 1 || stamp != "2026-10-30T09:00:00.000000000Z" || sender != owner {
		t.Fatal(seq, stamp, sender, err)
	}
	if err = srv.db.QueryRowContext(ctx, srv.rebind(`SELECT next_archive_seq FROM web_chat_rooms WHERE jid=?`), room).Scan(&seq); err != nil || seq != 2 {
		t.Fatal(seq, err)
	}
	if _, err = srv.db.ExecContext(ctx, srv.rebind(`DELETE FROM web_chat_rooms WHERE jid=?`), room); err != nil {
		t.Fatal(err)
	}
}

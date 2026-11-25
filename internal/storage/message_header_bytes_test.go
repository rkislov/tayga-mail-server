package storage

import (
	"testing"
	"unicode/utf8"
)

func TestMessageHeaderBytesPreserveOriginals(t *testing.T) {
	f := newBookingFixture(t)
	ctx := t.Context()
	box, err := f.s.EnsureMailbox(ctx, f.user.ID, "INBOX", "mail/fixture")
	if err != nil {
		t.Fatal(err)
	}
	subject, from, to := "subject\xff", "from\xd0\xee", "to\x00recipient"
	m, err := f.s.InsertMessage(ctx, &Message{MailboxID: box.ID, FilePath: "original.eml", Subject: subject, FromAddr: from, ToAddr: to})
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(m.Subject) || m.Subject != normalizeSearchText(subject) || m.ToAddr != normalizeSearchText(to) {
		t.Fatal("display metadata encoding")
	}
	var a, b, c []byte
	if err = f.s.DB().QueryRowContext(ctx, f.s.rebind(`SELECT subject,from_addr,to_addr FROM message_header_bytes WHERE message_id=?`), m.ID).Scan(&a, &b, &c); err != nil || string(a) != subject || string(b) != from || string(c) != to {
		t.Fatal("original headers not preserved", err)
	}
	if err = f.s.DeleteMessage(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.s.DB().QueryRowContext(ctx, `SELECT count(*) FROM message_header_bytes`).Scan(&count); err != nil || count != 0 {
		t.Fatal("header bytes survived message deletion", err)
	}
}

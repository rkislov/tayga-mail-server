package calutil

import (
	"bytes"
	"mime"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestIMIPSnapshotHeaders(t *testing.T) {
	raw := IMIPMessage("owner@example.com", "Организатор", "reader@example.com", "Встреча\r\nBcc: other@example.com", "Body\nline", "BEGIN:VCALENDAR\r\nMETHOD:REQUEST\r\nEND:VCALENDAR\r\n", "REQUEST", "stable-id", time.Unix(100, 0))
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if m.Header.Get("Bcc") != "" || m.Header.Get("Message-ID") != "<stable-id@tayga>" {
		t.Fatal("injected header or unstable ID")
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil || !strings.HasPrefix(subject, "Встреча") {
		t.Fatal(subject, err)
	}
	from, err := mail.ParseAddress(m.Header.Get("From"))
	if err != nil || from.Name != "Организатор" {
		t.Fatal(from, err)
	}
	if !bytes.Contains(raw, []byte("Body\r\nline")) {
		t.Fatal("noncanonical line endings")
	}
}

package httpapi

import (
	"strings"
	"testing"
)

func TestQuarantineMIMEPreview(t *testing.T) {
	raw := []byte("From: =?UTF-8?B?0KDQvtC80LDQvQ==?= <u@example.org>\r\nSubject: =?UTF-8?B?0KLQtdGB0YI=?=\r\nContent-Type: multipart/alternative; boundary=x\r\n\r\n--x\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n0KLQtdGB0YI=\r\n--x--\r\n")
	var item quarantineItem
	fillQuarantineMeta(&item, raw)
	if item.Subject != "Тест" || !strings.Contains(item.From, "Роман") {
		t.Fatalf("headers: %+v", item)
	}
	if got := messagePreview(raw, 4096); !strings.Contains(got, "Тест") || strings.Contains(got, "--x") {
		t.Fatal("MIME preview", got)
	}
}

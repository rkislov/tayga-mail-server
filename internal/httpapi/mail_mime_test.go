package httpapi

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestMIMENestedAttachments(t *testing.T) {
	raw := "Subject: =?UTF-8?B?0J/RgNC40LLQtdGC?=\r\nContent-Type: multipart/mixed; boundary=outer\r\n\r\n--outer\r\nContent-Type: multipart/alternative; boundary=inner\r\n\r\n--inner\r\nContent-Type: text/plain; charset=windows-1251\r\nContent-Transfer-Encoding: base64\r\n\r\nz/Do4uXy\r\n--inner\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString([]byte("<p>Привет</p>")) + "\r\n--inner--\r\n--outer\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename*=UTF-8''%D0%B1%D0%B8%D0%BB%D0%B5%D1%82.pdf\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi10ZXN0\r\n--outer--\r\n"
	got := parseMIMEMessage([]byte(raw))
	if got.Subject != "Привет" || got.Text != "Привет" || got.HTML != "<p>Привет</p>" {
		t.Fatalf("wrong decoded content: %#v", got)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].Filename != "билет.pdf" || string(got.Attachments[0].Data) != "%PDF-test" {
		t.Fatalf("wrong attachment: %#v", got.Attachments)
	}
}
func TestMIMESingleEncodedHTML(t *testing.T) {
	got := parseMIMEMessage([]byte("Content-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n<p>=D0=9F=D1=80=D0=B8=D0=B2=D0=B5=D1=82</p>"))
	if !strings.Contains(got.HTML, "Привет") {
		t.Fatalf("undecoded HTML %q", got.HTML)
	}
}
func TestMIMEAttachmentNotBody(t *testing.T) {
	got := parseMIMEMessage([]byte("Content-Type: text/html\r\nContent-Disposition: attachment; filename=unsafe.html\r\n\r\n<script>alert(1)</script>"))
	if got.HTML != "" || len(got.Attachments) != 1 {
		t.Fatal("attachment used as body")
	}
}

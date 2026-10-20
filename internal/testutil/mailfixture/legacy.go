// Package mailfixture builds synthetic mail shared by protocol regression tests.
package mailfixture

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

const (
	Subject  = "Проверка кодировки"
	Sender   = "Иван Петров"
	Text     = "Привет, мир! Получен счёт за октябрь."
	HTML     = "<html><head><style>.hidden { color: red; }</style></head><body><p>" + Text + "</p></body></html>"
	Filename = "счёт.pdf"
)

var Attachment = []byte("%PDF-synthetic\x00\x80\xff\r\n")

var Charsets = []struct {
	Label    string
	Encoding encoding.Encoding
}{
	{"koi8-r", charmap.KOI8R},
	{"windows-1251", charmap.Windows1251},
	{"win-1251", charmap.Windows1251},
	{"cp1251", charmap.Windows1251},
}

func Encode(t *testing.T, label, value string) []byte {
	t.Helper()
	for _, cs := range Charsets {
		if cs.Label == label {
			out, err := cs.Encoding.NewEncoder().Bytes([]byte(value))
			if err != nil {
				t.Fatal(err)
			}
			return out
		}
	}
	t.Fatalf("unsupported fixture charset %q", label)
	return nil
}

// Message includes encoded display names, a nested multipart/alternative and
// a binary attachment. No personal or third-party messages are used.
func Message(t *testing.T, label, transfer string, nested bool) []byte {
	t.Helper()
	word := func(s string) string { return mime.BEncoding.Encode(label, string(Encode(t, label, s))) }
	body := func(media, value string) string {
		data := Encode(t, label, value)
		switch transfer {
		case "base64":
			data = []byte(base64.StdEncoding.EncodeToString(data))
		case "quoted-printable":
			var buf bytes.Buffer
			w := quotedprintable.NewWriter(&buf)
			_, _ = w.Write(data)
			_ = w.Close()
			data = buf.Bytes()
		case "8bit":
		default:
			t.Fatalf("unsupported fixture transfer %q", transfer)
		}
		return fmt.Sprintf("Content-Type: %s; charset=%s\r\nContent-Transfer-Encoding: %s\r\n\r\n%s", media, label, transfer, data)
	}
	headers := "From: " + word(Sender) + " <sender@example.test>\r\nTo: recipient@example.test\r\n" +
		"Subject: " + word(Subject) + "\r\nMessage-ID: <" + label + "-" + transfer + "@example.test>\r\n" +
		"Date: Tue, 20 Oct 2026 09:00:00 +0300\r\nMIME-Version: 1.0\r\n"
	if !nested {
		return []byte(headers + body("text/plain", Text))
	}
	return []byte(headers + "Content-Type: multipart/mixed; boundary=outer\r\n\r\n" +
		"--outer\r\nContent-Type: multipart/alternative; boundary=inner\r\n\r\n" +
		"--inner\r\n" + body("text/plain", Text) + "\r\n--inner\r\n" + body("text/html", HTML) +
		"\r\n--inner--\r\n--outer\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=\"" +
		word(Filename) + "\"\r\nContent-Transfer-Encoding: base64\r\n\r\n" +
		base64.StdEncoding.EncodeToString(Attachment) + "\r\n--outer--\r\n")
}

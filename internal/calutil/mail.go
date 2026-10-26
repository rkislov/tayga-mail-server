package calutil

import (
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

// IMIPMessage builds a snapshot once; retries reuse the same bytes and ID.
func IMIPMessage(from, name, to, subject, text, ics, method, id string, date time.Time) []byte {
	clean := func(s string) string { return strings.NewReplacer("\r", "", "\n", "").Replace(s) }
	fromAddress := (&mail.Address{Name: clean(name), Address: clean(from)}).String()
	toAddress := (&mail.Address{Address: clean(to)}).String()
	boundary := "tayga-cal-" + clean(id)
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@tayga>\r\nMIME-Version: 1.0\r\n", fromAddress, toAddress, mime.QEncoding.Encode("UTF-8", clean(subject)), date.UTC().Format(time.RFC1123Z), clean(id))
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", boundary, strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n"))
	if html := renderInvitationHTML(ics, method, subject, text); html != "" {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary)
		encoded := quotedprintable.NewWriter(&b)
		_, _ = encoded.Write([]byte(html))
		_ = encoded.Close()
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/calendar; charset=utf-8; method=%s\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n--%s--\r\n", boundary, clean(method), strings.TrimRight(ics, "\r\n"), boundary)
	return []byte(b.String())
}

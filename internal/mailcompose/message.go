package mailcompose

import (
	"fmt"
	"mime"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

func BuildMessage(fromEmail, fromName string, to, cc, bcc []string, subject, text, html, msgid string) []byte {
	var b strings.Builder
	from := fromEmail
	if fromName != "" {
		from = (&mail.Address{Name: strings.NewReplacer("\r", "", "\n", "").Replace(fromName), Address: fromEmail}).String()
	}
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	if len(cc) > 0 {
		fmt.Fprintf(&b, "Cc: %s\r\n", strings.Join(cc, ", "))
	}
	// Bcc belongs only to the envelope, never to the delivered headers.
	subject = strings.NewReplacer("\r", "", "\n", "").Replace(subject)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: %s\r\n", msgid)
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	if html != "" && text != "" {
		boundary := "tayga-" + strconv.FormatInt(time.Now().UnixNano(), 36)
		fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", boundary, text)
		fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s\r\n", boundary, html)
		fmt.Fprintf(&b, "--%s--\r\n", boundary)
	} else if html != "" {
		b.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
		b.WriteString(html)
	} else {
		b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		b.WriteString(text)
	}
	return []byte(b.String())
}

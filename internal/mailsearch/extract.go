package mailsearch

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"regexp"
	"strings"
	"unicode"
)

// Document holds denormalized headers plus body text for FTS indexing.
type Document struct {
	Subject string
	From    string
	To      string
	Date    string
	Body    string
}

var htmlTagRe = regexp.MustCompile(`(?is)<[^>]+>`)
var htmlEntityRe = regexp.MustCompile(`&(#?\w+);`)

// ParseDocument extracts searchable text from an RFC822 message.
// Prefers text/plain; falls back to HTML with tags stripped.
func ParseDocument(raw []byte) Document {
	doc := Document{}
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		doc.Body = normalizeText(string(raw))
		return doc
	}
	doc.Subject = strings.TrimSpace(m.Header.Get("Subject"))
	doc.From = strings.TrimSpace(m.Header.Get("From"))
	doc.To = strings.TrimSpace(m.Header.Get("To"))
	doc.Date = strings.TrimSpace(m.Header.Get("Date"))

	ct := m.Header.Get("Content-Type")
	media, params, _ := mime.ParseMediaType(ct)
	var plain, html string
	if strings.HasPrefix(media, "multipart/") {
		mr := multipart.NewReader(m.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			body, _ := io.ReadAll(io.LimitReader(p, 2<<20))
			pct, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
			switch {
			case pct == "text/plain" && plain == "":
				plain = string(body)
			case pct == "text/html" && html == "":
				html = string(body)
			}
		}
	} else {
		body, _ := io.ReadAll(io.LimitReader(m.Body, 2<<20))
		if media == "text/html" {
			html = string(body)
		} else {
			plain = string(body)
		}
	}
	if plain != "" {
		doc.Body = normalizeText(plain)
	} else if html != "" {
		doc.Body = normalizeText(stripHTML(html))
	}
	return doc
}

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = htmlEntityRe.ReplaceAllStringFunc(s, func(e string) string {
		switch strings.ToLower(e) {
		case "&nbsp;":
			return " "
		case "&amp;":
			return "&"
		case "&lt;":
			return "<"
		case "&gt;":
			return ">"
		case "&quot;":
			return "\""
		default:
			return " "
		}
	})
	return s
}

func normalizeText(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

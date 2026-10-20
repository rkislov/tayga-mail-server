package mailsearch

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"
	"unicode"

	"github.com/tayga/tms/internal/mailcharset"
	xhtml "golang.org/x/net/html"
)

// Document holds denormalized headers plus body text for FTS indexing.
type Document struct {
	MessageID string
	Subject   string
	From      string
	To        string
	Date      string
	Body      string
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
	doc.MessageID = strings.TrimSpace(m.Header.Get("Message-ID"))
	doc.Subject = DecodeHeader(strings.TrimSpace(m.Header.Get("Subject")))
	doc.From = DecodeHeader(strings.TrimSpace(m.Header.Get("From")))
	doc.To = DecodeHeader(strings.TrimSpace(m.Header.Get("To")))
	doc.Date = strings.TrimSpace(m.Header.Get("Date"))

	var plain, html string
	parts, remaining := 0, int64(2<<20)
	var walk func(textproto.MIMEHeader, io.Reader, int)
	walk = func(h textproto.MIMEHeader, r io.Reader, depth int) {
		if depth > 20 || parts >= 256 || remaining <= 0 || plain != "" {
			return
		}
		parts++
		media, params, _ := mime.ParseMediaType(h.Get("Content-Type"))
		if media == "" {
			media = "text/plain"
		}
		disposition, dp, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
		// A text attachment (or attached multipart message) is not the body.
		if disposition == "attachment" || dp["filename"] != "" || params["name"] != "" {
			return
		}
		if strings.HasPrefix(media, "multipart/") {
			if params["boundary"] == "" {
				return
			}
			mr := multipart.NewReader(r, params["boundary"])
			for parts < 256 && remaining > 0 && plain == "" {
				p, err := mr.NextRawPart()
				if err != nil {
					break
				}
				walk(p.Header, p, depth+1)
			}
			return
		}
		if media != "text/plain" && media != "text/html" || media == "text/html" && html != "" {
			return
		}
		switch strings.ToLower(strings.TrimSpace(h.Get("Content-Transfer-Encoding"))) {
		case "base64":
			r = base64.NewDecoder(base64.StdEncoding, r)
		case "quoted-printable":
			r = quotedprintable.NewReader(r)
		}
		if label := params["charset"]; label != "" {
			if decoded, err := mailcharset.Reader(label, r); err == nil {
				r = decoded
			}
		}
		body, _ := io.ReadAll(io.LimitReader(r, remaining))
		remaining -= int64(len(body))
		if media == "text/html" {
			html = string(body)
		} else {
			plain = string(body)
		}
	}
	walk(textproto.MIMEHeader(m.Header), m.Body, 0)
	if plain != "" {
		doc.Body = normalizeText(plain)
	} else if html != "" {
		doc.Body = normalizeText(ExtractHTMLText(html))
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

// DecodeHeader decodes RFC 2047 words, including legacy mail character sets.
func DecodeHeader(value string) string {
	decoder := mime.WordDecoder{CharsetReader: mailcharset.Reader}
	decoded, err := decoder.DecodeHeader(value)
	if err != nil || strings.Contains(decoded, "=?") {
		// Some older senders add excessive Base64 padding to encoded words.
		words := regexp.MustCompile(`(?i)=\?([^?]+)\?b\?([^?]+)\?=`)
		repaired := words.ReplaceAllStringFunc(value, func(word string) string {
			parts := words.FindStringSubmatch(word)
			raw, e := base64.RawStdEncoding.DecodeString(strings.TrimRight(parts[2], "="))
			if e != nil {
				return word
			}
			return "=?" + parts[1] + "?B?" + base64.StdEncoding.EncodeToString(raw) + "?="
		})
		if decoded, e := decoder.DecodeHeader(repaired); e == nil {
			return decoded
		}
		return value
	}
	return decoded
}

// ExtractHTMLText converts mail markup to text for non-HTML previews.
func ExtractHTMLText(s string) string {
	root, err := xhtml.Parse(strings.NewReader(s))
	if err != nil {
		return stripHTML(s)
	}
	var out strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "head") {
			return
		}
		block := n.Type == xhtml.ElementNode && strings.Contains("|p|div|tr|li|br|h1|h2|h3|table|section|", "|"+n.Data+"|")
		if block {
			out.WriteByte('\n')
		}
		if n.Type == xhtml.TextNode {
			out.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if block {
			out.WriteByte('\n')
		} else if n.Type == xhtml.ElementNode && n.Data == "td" {
			out.WriteByte(' ')
		}
	}
	walk(root)
	return out.String()
}

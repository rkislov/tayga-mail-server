package mailsearch

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/charset"
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

// DecodeHeader decodes RFC 2047 words, including legacy mail character sets.
func DecodeHeader(value string) string {
	decoder := mime.WordDecoder{CharsetReader: func(label string, input io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(label, input)
	}}
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

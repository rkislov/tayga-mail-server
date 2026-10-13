package xmpp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"strings"
)

func attr(se xml.StartElement, name string) string {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func attrIf(name, val string) string {
	if val == "" {
		return ""
	}
	return fmt.Sprintf(` %s='%s'`, name, xmlEscape(val))
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func readElementText(dec *xml.Decoder, start xml.StartElement) (string, error) {
	var b strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return b.String(), nil
			}
		case xml.StartElement:
			_ = dec.Skip()
		}
	}
}

// readInnerXML consumes the rest of start and returns child markup (best-effort reserialize).
func readInnerXML(dec *xml.Decoder, start xml.StartElement) (string, error) {
	var b strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			writeStart(&b, t)
		case xml.EndElement:
			depth--
			if depth > 0 {
				b.WriteString("</")
				b.WriteString(t.Name.Local)
				b.WriteByte('>')
			}
		case xml.CharData:
			b.WriteString(xmlEscape(string(t)))
		}
	}
	_ = start
	return b.String(), nil
}

func writeStart(b *strings.Builder, t xml.StartElement) {
	b.WriteByte('<')
	b.WriteString(t.Name.Local)
	// Decoder resolves prefixes to namespace URIs. Preserve that URI even when
	// its declaration was inherited from the enclosing stanza.
	b.WriteString(" xmlns='")
	b.WriteString(xmlEscape(t.Name.Space))
	b.WriteByte('\'')
	for _, a := range t.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		b.WriteByte(' ')
		if a.Name.Space == "http://www.w3.org/XML/1998/namespace" {
			b.WriteString("xml:")
		}
		b.WriteString(a.Name.Local)
		b.WriteString("='")
		b.WriteString(xmlEscape(a.Value))
		b.WriteByte('\'')
	}
	b.WriteByte('>')
}

func randomID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

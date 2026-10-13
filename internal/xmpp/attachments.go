package xmpp

import (
	"encoding/xml"
	"net/url"
	"path"
	"strings"
)

type ChatAttachment struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

func stanzaAttachment(stanza string) (*ChatAttachment, bool) {
	d := xml.NewDecoder(strings.NewReader(stanza))
	encrypted := false
	link := ""
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local == "encrypted" && (se.Name.Space == "eu.siacs.conversations.axolotl" || se.Name.Space == "urn:xmpp:omemo:2") {
			encrypted = true
		}
		if se.Name.Local == "x" && se.Name.Space == "jabber:x:oob" {
			var x struct {
				URL string `xml:"url"`
			}
			if d.DecodeElement(&x, &se) == nil {
				link = x.URL
			}
		}
	}
	if encrypted {
		return nil, true
	}
	if link == "" {
		link = strings.TrimSpace(extractBody(stanza))
	}
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || strings.ContainsAny(link, "\r\n ") {
		return nil, false
	}
	name := path.Base(u.Path)
	if name == "." || name == "/" || name == "" {
		name = "file"
	}
	return &ChatAttachment{URL: u.String(), Name: name}, false
}

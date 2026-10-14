package xmpp

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// carbonMessage checks direct children, so text or nested payloads cannot toggle policy.
// The expanded carbons:rules:0 feature is not advertised (MUC tracking is separate).
func carbonMessage(raw string) (string, string, bool) {
	dec := xml.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return "", "", false
	}
	root, ok := tok.(xml.StartElement)
	if !ok || root.Name.Local != "message" || (root.Name.Space != "" && root.Name.Space != "jabber:client") {
		return "", "", false
	}
	typ := attr(root, "type")
	if typ != "" && typ != "chat" && typ != "normal" {
		return "", "", false
	}
	eligible := typ == "chat"
	var inner strings.Builder
	for {
		tok, err = dec.Token()
		if err != nil {
			return "", "", false
		}
		switch child := tok.(type) {
		case xml.EndElement:
			if !eligible {
				return "", "", false
			}
			root.Name.Space = "jabber:client"
			var result strings.Builder
			writeStart(&result, root)
			result.WriteString(inner.String())
			result.WriteString("</message>")
			return result.String(), typ, true
		case xml.StartElement:
			if child.Name.Space == "urn:xmpp:carbons:2" && (child.Name.Local == "private" || child.Name.Local == "sent" || child.Name.Local == "received") {
				return "", "", false
			}
			if child.Name.Space == "urn:xmpp:hints" && child.Name.Local == "no-copy" {
				return "", "", false
			}
			if child.Name.Local == "body" && (child.Name.Space == "" || child.Name.Space == "jabber:client") {
				eligible = true
			}
			if child.Name.Space == "urn:xmpp:receipts" || child.Name.Space == "http://jabber.org/protocol/chatstates" || child.Name.Space == "urn:xmpp:chat-markers:0" {
				eligible = true
			}
			content, err := readInnerXML(dec, child)
			if err != nil {
				return "", "", false
			}
			if child.Name.Space == "" {
				child.Name.Space = "jabber:client"
			}
			writeStart(&inner, child)
			inner.WriteString(content)
			inner.WriteString("</" + child.Name.Local + ">")
		case xml.CharData:
			inner.WriteString(xmlEscape(string(child)))
		}
	}
}
func (h *Hub) fanoutCarbon(source JID, kind, raw string) {
	if kind != "sent" && kind != "received" {
		return
	}
	forwarded, typ, ok := carbonMessage(raw)
	if !ok {
		return
	}
	for _, other := range h.Sessions(source.Bare()) {
		if other.JID.Full() == source.Full() {
			continue
		}
		other.mu.Lock()
		enabled := other.carbons
		other.mu.Unlock()
		if !enabled {
			continue
		}
		other.SendRaw([]byte(fmt.Sprintf(`<message xmlns='jabber:client' from='%s' to='%s'%s><%s xmlns='urn:xmpp:carbons:2'><forwarded xmlns='urn:xmpp:forward:0'>%s</forwarded></%s></message>`, xmlEscape(source.Bare()), xmlEscape(other.JID.Full()), attrIf("type", typ), kind, forwarded, kind)))
	}
}

package xmpp

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const roomMAMForm = `<query xmlns='urn:xmpp:mam:2'><x xmlns='jabber:x:data' type='form'><field var='FORM_TYPE' type='hidden'><value>urn:xmpp:mam:2</value></field><field var='with' type='jid-single'/><field var='start' type='text-single'/><field var='end' type='text-single'/></x></query>`

type roomMAMElement struct {
	XMLName  xml.Name
	Type     string           `xml:"type,attr"`
	Var      string           `xml:"var,attr"`
	QueryID  string           `xml:"queryid,attr"`
	Text     string           `xml:",chardata"`
	Children []roomMAMElement `xml:",any"`
}

// Reject unknown filters/extensions instead of silently returning a wider archive.
func parseRoomMAM(inner string) (mamFilter, string, string) {
	f := mamFilter{Limit: 50}
	var q roomMAMElement
	if xml.Unmarshal([]byte(inner), &q) != nil || q.XMLName != (xml.Name{Space: "urn:xmpp:mam:2", Local: "query"}) {
		return f, "", "bad-request"
	}
	seen := map[xml.Name]bool{}
	for _, child := range q.Children {
		if seen[child.XMLName] {
			return f, "", "bad-request"
		}
		seen[child.XMLName] = true
		switch child.XMLName {
		case xml.Name{Space: "jabber:x:data", Local: "x"}:
			if child.Type != "submit" {
				return f, "", "bad-request"
			}
			fields := map[string]bool{}
			for _, field := range child.Children {
				if field.XMLName != (xml.Name{Space: "jabber:x:data", Local: "field"}) || fields[field.Var] {
					return f, "", "bad-request"
				}
				fields[field.Var] = true
				switch field.Var {
				case "FORM_TYPE", "with", "start", "end":
				default:
					return f, "", "feature-not-implemented"
				}
				if len(field.Children) != 1 || field.Children[0].XMLName != (xml.Name{Space: "jabber:x:data", Local: "value"}) || len(field.Children[0].Children) != 0 {
					return f, "", "bad-request"
				}
				value := field.Children[0].Text
				switch field.Var {
				case "FORM_TYPE":
					if value != "urn:xmpp:mam:2" {
						return f, "", "bad-request"
					}
				case "with":
					jid, err := NormalizeJID(value)
					if err != nil {
						return f, "", "bad-request"
					}
					f.With = jid.Full()
				case "start", "end":
					stamp, err := time.Parse(time.RFC3339Nano, value)
					if err != nil {
						return f, "", "bad-request"
					}
					stamp = stamp.UTC()
					if field.Var == "start" {
						f.Start = &stamp
					} else {
						f.End = &stamp
					}
				}
			}
		case xml.Name{Space: "http://jabber.org/protocol/rsm", Local: "set"}:
			fields := map[string]bool{}
			for _, field := range child.Children {
				if field.XMLName.Space != "http://jabber.org/protocol/rsm" || fields[field.XMLName.Local] || len(field.Children) > 0 {
					return f, "", "bad-request"
				}
				fields[field.XMLName.Local] = true
				value := strings.TrimSpace(field.Text)
				switch field.XMLName.Local {
				case "max":
					n, err := strconv.Atoi(value)
					if err != nil || n < 0 {
						return f, "", "bad-request"
					}
					if n > 100 {
						n = 100
					}
					f.Limit = n
				case "before":
					f.Before = &value
				case "after":
					f.After = &value
				default:
					return f, "", "feature-not-implemented"
				}
			}
		default:
			return f, "", "feature-not-implemented"
		}
	}
	if f.After != nil && (f.Before != nil || *f.After == "") {
		return f, "", "bad-request"
	}
	if f.Start != nil && f.End != nil && f.Start.After(*f.End) {
		return f, "", "bad-request"
	}
	return f, q.QueryID, ""
}

// handleMUCIQ holds mucMu until the final IQ, so removing a member cannot
// interleave an archive response or a live room broadcast after the removal.
func (s *Session) handleRoomMAM(ctx context.Context, id, typ, room, inner string) error {
	fail := func(condition string) error { return s.mucError("iq", id, room, condition) }
	if err := s.srv.requireRoomMember(ctx, s.JID.Bare(), room); err != nil {
		return fail("forbidden")
	}
	if typ == "get" {
		return s.mucResult(id, room, roomMAMForm)
	}
	if typ != "set" {
		return fail("bad-request")
	}
	f, queryID, condition := parseRoomMAM(inner)
	if condition != "" {
		return fail(condition)
	}
	p, err := s.srv.roomArchivePage(ctx, s.JID.Bare(), room, f)
	if err == ErrRoomCursor {
		return fail("item-not-found")
	}
	if err != nil {
		return fail("internal-server-error")
	}
	for _, m := range p.Messages {
		// MAM room messages omit 'to' and identify the actual publisher in this
		// non-anonymous room. Never copy client-supplied identity metadata.
		extra := `<x xmlns='` + mucNS + `#user'><item jid='` + xmlEscape(m.SenderJID) + `'/></x>`
		stanza := roomMessageXML(room, m.Nick, "", m.ID, m.Body, m.OriginID, roomCipherExtras(m.OMEMO)+extra)
		s.SendRaw([]byte(fmt.Sprintf(`<message from='%s' to='%s'><result xmlns='urn:xmpp:mam:2' queryid='%s' id='%s'><forwarded xmlns='urn:xmpp:forward:0'><delay xmlns='urn:xmpp:delay' stamp='%s'/>%s</forwarded></result></message>`, xmlEscape(room), xmlEscape(s.JID.Full()), xmlEscape(queryID), xmlEscape(m.ID), m.At.UTC().Format(time.RFC3339Nano), stanza)))
	}
	bounds := ""
	if len(p.Messages) > 0 {
		bounds = `<first>` + xmlEscape(p.First) + `</first><last>` + xmlEscape(p.Last) + `</last>`
	}
	return s.mucResult(id, room, fmt.Sprintf(`<fin xmlns='urn:xmpp:mam:2' complete='%t'><set xmlns='http://jabber.org/protocol/rsm'>%s<count>%d</count></set></fin>`, p.Complete, bounds, p.Total))
}

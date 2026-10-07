package xmpp

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	rePubsubNode   = regexp.MustCompile(`node=['"]([^'"]+)['"]`)
	reItemID       = regexp.MustCompile(`<item[^>]*\bid=['"]([^'"]+)['"]`)
	reWithJID      = regexp.MustCompile(`<jid[^>]*>([^<]+)</jid>`)
	reRosterJID    = regexp.MustCompile(`<item[^>]*\bjid=['"]([^'"]+)['"]`)
	reRosterName   = regexp.MustCompile(`\bname=['"]([^'"]*)['"]`)
	reRosterSub    = regexp.MustCompile(`\bsubscription=['"]([^'"]+)['"]`)
	reRosterAsk    = regexp.MustCompile(`\bask=['"]([^'"]+)['"]`)
	reGroup        = regexp.MustCompile(`<group[^>]*>([^<]*)</group>`)
	reMAMWith      = regexp.MustCompile(`<field[^>]*var=['"]with['"][^>]*>\s*<value>([^<]+)</value>`)
	reMAMMax       = regexp.MustCompile(`<max[^>]*>(\d+)</max>|<field[^>]*var=['"]max['"][^>]*>\s*<value>(\d+)</value>`)
	reDeleteNode   = regexp.MustCompile(`node=['"]([^'"]+)['"]`)
)

func (s *Session) handleIQ(se xml.StartElement) error {
	id := attr(se, "id")
	typ := attr(se, "type")
	to := attr(se, "to")
	inner, err := readInnerXML(s.dec, se)
	if err != nil {
		return err
	}
	ctx := context.Background()

	switch {
	case typ == "set" && strings.Contains(inner, "urn:ietf:params:xml:ns:xmpp-session"):
		return s.writeString(fmt.Sprintf(`<iq type='result' id='%s'/>`, xmlEscape(id)))

	case typ == "get" && strings.Contains(inner, "urn:xmpp:ping"):
		return s.writeString(fmt.Sprintf(
			`<iq type='result' id='%s' from='%s' to='%s'/>`,
			xmlEscape(id), xmlEscape(s.srv.domain), xmlEscape(s.JID.Full()),
		))

	case typ == "get" && strings.Contains(inner, "http://jabber.org/protocol/disco#info"):
		return s.writeDiscoInfo(id, to)

	case strings.Contains(inner, "jabber:iq:roster"):
		return s.handleRosterIQ(ctx, id, typ, inner)

	case strings.Contains(inner, "http://jabber.org/protocol/pubsub"):
		return s.handlePubSubIQ(ctx, id, typ, to, inner)

	case strings.Contains(inner, "urn:xmpp:mam:2"):
		return s.handleMAMIQ(ctx, id, typ, inner)

	case strings.Contains(inner, "urn:xmpp:carbons:2"):
		return s.handleCarbonsIQ(id, typ, inner)

	case typ == "get" || typ == "set":
		return s.writeString(fmt.Sprintf(
			`<iq type='error' id='%s'><error type='cancel'><feature-not-implemented xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`,
			xmlEscape(id),
		))
	}
	return nil
}

func (s *Session) writeDiscoInfo(id, to string) error {
	from := s.srv.domain
	if to != "" && ParseJID(to).Bare() == s.JID.Bare() {
		from = s.JID.Bare()
	}
	return s.writeString(fmt.Sprintf(
		`<iq type='result' id='%s' from='%s' to='%s'><query xmlns='http://jabber.org/protocol/disco#info'>`+
			`<identity category='server' type='im' name='Tayga XMPP'/>`+
			`<identity category='pubsub' type='pep'/>`+
			`<feature var='urn:xmpp:ping'/>`+
			`<feature var='jabber:iq:roster'/>`+
			`<feature var='urn:xmpp:mam:2'/>`+
			`<feature var='urn:xmpp:carbons:2'/>`+
			`<feature var='http://jabber.org/protocol/pubsub'/>`+
			`<feature var='http://jabber.org/protocol/pubsub#publish'/>`+
			`<feature var='urn:xmpp:omemo:2'/>`+
			`</query></iq>`,
		xmlEscape(id), xmlEscape(from), xmlEscape(s.JID.Full()),
	))
}

func (s *Session) handleRosterIQ(ctx context.Context, id, typ, inner string) error {
	if s.user == nil {
		return s.writeString(iqAuthError(id))
	}
	switch typ {
	case "get":
		items, err := s.srv.listRoster(ctx, s.user.ID)
		if err != nil {
			return s.writeString(iqInternalError(id))
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf(`<iq type='result' id='%s'><query xmlns='jabber:iq:roster'>`, xmlEscape(id)))
		for _, it := range items {
			b.WriteString(fmt.Sprintf(`<item jid='%s' name='%s' subscription='%s'`,
				xmlEscape(it.JID), xmlEscape(it.Name), xmlEscape(it.Subscription)))
			if it.Ask != "" {
				b.WriteString(fmt.Sprintf(` ask='%s'`, xmlEscape(it.Ask)))
			}
			b.WriteByte('>')
			var groups []string
			_ = json.Unmarshal([]byte(it.GroupsJSON), &groups)
			for _, g := range groups {
				b.WriteString(`<group>` + xmlEscape(g) + `</group>`)
			}
			b.WriteString(`</item>`)
		}
		b.WriteString(`</query></iq>`)
		return s.writeString(b.String())
	case "set":
		jid := firstSubmatch(reRosterJID, inner)
		if jid == "" {
			return s.writeString(iqBadRequest(id))
		}
		if strings.Contains(inner, `subscription='remove'`) || strings.Contains(inner, `subscription="remove"`) {
			_ = s.srv.deleteRoster(ctx, s.user.ID, ParseJID(jid).Bare())
			_ = s.writeString(fmt.Sprintf(`<iq type='result' id='%s'/>`, xmlEscape(id)))
			return s.pushRosterPush(ParseJID(jid).Bare(), "remove", "", "none", nil)
		}
		name := firstSubmatch(reRosterName, inner)
		sub := firstSubmatch(reRosterSub, inner)
		if sub == "" {
			sub = "none"
		}
		ask := firstSubmatch(reRosterAsk, inner)
		var groups []string
		for _, m := range reGroup.FindAllStringSubmatch(inner, -1) {
			groups = append(groups, m[1])
		}
		gj, _ := json.Marshal(groups)
		it := rosterItem{
			JID: ParseJID(jid).Bare(), Name: name, Subscription: sub,
			GroupsJSON: string(gj), Ask: ask,
		}
		if err := s.srv.upsertRoster(ctx, s.user.ID, it); err != nil {
			return s.writeString(iqInternalError(id))
		}
		_ = s.writeString(fmt.Sprintf(`<iq type='result' id='%s'/>`, xmlEscape(id)))
		return s.pushRosterPush(it.JID, "", it.Name, it.Subscription, groups)
	}
	return nil
}

func (s *Session) pushRosterPush(jid, subscriptionOverride, name, sub string, groups []string) error {
	if subscriptionOverride != "" {
		sub = subscriptionOverride
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		`<iq type='set'><query xmlns='jabber:iq:roster'><item jid='%s' name='%s' subscription='%s'>`,
		xmlEscape(jid), xmlEscape(name), xmlEscape(sub),
	))
	for _, g := range groups {
		b.WriteString(`<group>` + xmlEscape(g) + `</group>`)
	}
	b.WriteString(`</item></query></iq>`)
	raw := []byte(b.String())
	for _, sess := range s.srv.hub.Sessions(s.JID.Bare()) {
		sess.SendRaw(raw)
	}
	return nil
}

func (s *Session) handlePubSubIQ(ctx context.Context, id, typ, to, inner string) error {
	owner := s.JID.Bare()
	if to != "" {
		owner = ParseJID(to).Bare()
	}
	node := firstSubmatch(rePubsubNode, inner)

	switch {
	case typ == "set" && strings.Contains(inner, "<publish"):
		if owner != s.JID.Bare() {
			return s.writeString(iqForbidden(id))
		}
		if node == "" {
			return s.writeString(iqBadRequest(id))
		}
		itemID := firstSubmatch(reItemID, inner)
		if itemID == "" {
			itemID = "current"
		}
		payload := extractItemPayload(inner)
		if err := s.srv.publishPEP(ctx, owner, node, itemID, payload); err != nil {
			s.log.Debug("pep publish", "err", err, "node", node)
			return s.writeString(iqInternalError(id))
		}
		// notify other own resources (PEP event)
		event := fmt.Sprintf(
			`<message from='%s' to='%s'><event xmlns='http://jabber.org/protocol/pubsub#event'>`+
				`<items node='%s'><item id='%s'>%s</item></items></event></message>`,
			xmlEscape(owner), xmlEscape(s.JID.Full()), xmlEscape(node), xmlEscape(itemID), payload,
		)
		for _, sess := range s.srv.hub.Sessions(owner) {
			if sess != s {
				sess.SendRaw([]byte(event))
			}
		}
		return s.writeString(fmt.Sprintf(
			`<iq type='result' id='%s'><pubsub xmlns='http://jabber.org/protocol/pubsub'><publish node='%s'><item id='%s'/></publish></pubsub></iq>`,
			xmlEscape(id), xmlEscape(node), xmlEscape(itemID),
		))

	case typ == "get" && (strings.Contains(inner, "<items") || strings.Contains(inner, "<item")):
		if node == "" {
			return s.writeString(iqBadRequest(id))
		}
		items, err := s.srv.getPEPItems(ctx, owner, node)
		if err != nil {
			return s.writeString(iqInternalError(id))
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf(
			`<iq type='result' id='%s' from='%s' to='%s'><pubsub xmlns='http://jabber.org/protocol/pubsub'><items node='%s'>`,
			xmlEscape(id), xmlEscape(owner), xmlEscape(s.JID.Full()), xmlEscape(node),
		))
		for _, it := range items {
			b.WriteString(fmt.Sprintf(`<item id='%s'>%s</item>`, xmlEscape(it.ItemID), it.Payload))
		}
		b.WriteString(`</items></pubsub></iq>`)
		return s.writeString(b.String())

	case typ == "set" && strings.Contains(inner, "<delete"):
		if owner != s.JID.Bare() {
			return s.writeString(iqForbidden(id))
		}
		node = firstSubmatch(reDeleteNode, inner)
		_ = s.srv.deletePEPNode(ctx, owner, node)
		return s.writeString(fmt.Sprintf(`<iq type='result' id='%s'/>`, xmlEscape(id)))
	}

	return s.writeString(fmt.Sprintf(
		`<iq type='error' id='%s'><error type='cancel'><feature-not-implemented xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`,
		xmlEscape(id),
	))
}

func (s *Session) handleMAMIQ(ctx context.Context, id, typ, inner string) error {
	if typ != "set" {
		return s.writeString(iqBadRequest(id))
	}
	withBare := ""
	if m := reMAMWith.FindStringSubmatch(inner); len(m) > 1 {
		withBare = ParseJID(m[1]).Bare()
	}
	limit := 50
	if m := reMAMMax.FindStringSubmatch(inner); len(m) > 0 {
		for _, g := range m[1:] {
			if g != "" {
				if n, err := strconv.Atoi(g); err == nil {
					limit = n
				}
			}
		}
	}
	rows, err := s.srv.queryMAM(ctx, s.JID.Bare(), withBare, limit)
	if err != nil {
		return s.writeString(iqInternalError(id))
	}
	queryID := randomID()[:12]
	for _, r := range rows {
		stamp := r.CreatedAt.UTC().Format(time.RFC3339)
		msg := fmt.Sprintf(
			`<message to='%s'><result xmlns='urn:xmpp:mam:2' queryid='%s' id='%s'>`+
				`<forwarded xmlns='urn:xmpp:forward:0'><delay xmlns='urn:xmpp:delay' stamp='%s'/>%s</forwarded>`+
				`</result></message>`,
			xmlEscape(s.JID.Full()), xmlEscape(queryID), xmlEscape(r.StanzaID),
			xmlEscape(stamp), r.Stanza,
		)
		s.SendRaw([]byte(msg))
	}
	return s.writeString(fmt.Sprintf(
		`<iq type='result' id='%s'><fin xmlns='urn:xmpp:mam:2' complete='true'>`+
			`<set xmlns='http://jabber.org/protocol/rsm'><count>%d</count></set></fin></iq>`,
		xmlEscape(id), len(rows),
	))
}

func (s *Session) handleCarbonsIQ(id, typ, inner string) error {
	if typ != "set" {
		return s.writeString(iqBadRequest(id))
	}
	s.mu.Lock()
	if strings.Contains(inner, "<enable") {
		s.carbons = true
	} else if strings.Contains(inner, "<disable") {
		s.carbons = false
	}
	s.mu.Unlock()
	return s.writeString(fmt.Sprintf(`<iq type='result' id='%s'/>`, xmlEscape(id)))
}

func extractItemPayload(inner string) string {
	// Find <item ...>PAYLOAD</item> inside <publish>
	idx := strings.Index(inner, "<item")
	if idx < 0 {
		return ""
	}
	gt := strings.IndexByte(inner[idx:], '>')
	if gt < 0 {
		return ""
	}
	start := idx + gt + 1
	end := strings.LastIndex(inner, "</item>")
	if end < start {
		return ""
	}
	return inner[start:end]
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func iqBadRequest(id string) string {
	return fmt.Sprintf(`<iq type='error' id='%s'><error type='modify'><bad-request xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`, xmlEscape(id))
}
func iqForbidden(id string) string {
	return fmt.Sprintf(`<iq type='error' id='%s'><error type='auth'><forbidden xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`, xmlEscape(id))
}
func iqAuthError(id string) string {
	return fmt.Sprintf(`<iq type='error' id='%s'><error type='auth'><not-authorized xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`, xmlEscape(id))
}
func iqInternalError(id string) string {
	return fmt.Sprintf(`<iq type='error' id='%s'><error type='wait'><internal-server-error xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`, xmlEscape(id))
}

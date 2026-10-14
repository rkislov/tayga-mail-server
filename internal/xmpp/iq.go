package xmpp

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/tayga/tms/internal/avatar"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	rePubsubNode = regexp.MustCompile(`node=['"]([^'"]+)['"]`)
	reItemID     = regexp.MustCompile(`<item[^>]*\bid=['"]([^'"]+)['"]`)
	reWithJID    = regexp.MustCompile(`<jid[^>]*>([^<]+)</jid>`)
	reRosterJID  = regexp.MustCompile(`<item[^>]*\bjid=['"]([^'"]+)['"]`)
	reRosterName = regexp.MustCompile(`\bname=['"]([^'"]*)['"]`)
	reRosterSub  = regexp.MustCompile(`\bsubscription=['"]([^'"]+)['"]`)
	reRosterAsk  = regexp.MustCompile(`\bask=['"]([^'"]+)['"]`)
	reGroup      = regexp.MustCompile(`<group[^>]*>([^<]*)</group>`)
	reDeleteNode = regexp.MustCompile(`node=['"]([^'"]+)['"]`)
)

func (s *Session) handleIQ(se xml.StartElement) error {
	id := attr(se, "id")
	typ := attr(se, "type")
	to := attr(se, "to")
	inner, err := readInnerXML(s.dec, se)
	if err != nil {
		return err
	}
	var request struct{ XMLName xml.Name }
	if xml.Unmarshal([]byte(inner), &request) == nil && s.log != nil {
		s.log.Info("xmpp IQ request", "type", typ, "element", request.XMLName.Local, "namespace", request.XMLName.Space, "to", to)
	}
	if handled, err := s.uploadIQ(id, typ, to, inner); handled {
		return err
	}
	ctx := context.Background()

	switch {
	case typ == "get" && strings.Contains(inner, "vcard-temp"):
		user := s.user
		if to != "" {
			email := strings.Split(to, "/")[0]
			other, err := s.srv.store.GetUserByEmail(ctx, email)
			if err != nil || other.TenantID != s.user.TenantID {
				return s.writeString(iqForbidden(id))
			}
			user = other
		}
		value, err := avatar.Get(ctx, s.srv.store, user.ID)
		if err != nil {
			return s.writeString(iqInternalError(id))
		}
		photo := ""
		if value.Data != "" {
			photo = "<PHOTO><TYPE>image/png</TYPE><BINVAL>" + value.Data + "</BINVAL></PHOTO>"
		}
		return s.writeString(fmt.Sprintf(`<iq type='result' id='%s' from='%s' to='%s'><vCard xmlns='vcard-temp'><FN>%s</FN>%s</vCard></iq>`, xmlEscape(id), xmlEscape(user.Email), xmlEscape(s.JID.Full()), xmlEscape(user.DisplayName), photo))
	case typ == "set" && strings.Contains(inner, "urn:ietf:params:xml:ns:xmpp-session"):
		return s.writeString(fmt.Sprintf(`<iq type='result' id='%s'/>`, xmlEscape(id)))

	case typ == "get" && strings.Contains(inner, "urn:xmpp:ping"):
		return s.writeString(fmt.Sprintf(
			`<iq type='result' id='%s' from='%s' to='%s'/>`,
			xmlEscape(id), xmlEscape(s.iqResponder(to)), xmlEscape(s.JID.Full()),
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

// iqResponder keeps domain discovery replies bound to the queried virtual host.
// Account domains may differ from the physical server hostname.
func (s *Session) iqResponder(to string) string {
	if to == s.JID.Domain || to == s.JID.Bare() || to == s.srv.domain {
		return to
	}
	if to == "" && s.JID.Domain != "" {
		return s.JID.Domain
	}
	return s.srv.domain
}

func (s *Session) writeDiscoInfo(id, to string) error {
	from := s.iqResponder(to)
	return s.writeString(fmt.Sprintf(
		`<iq type='result' id='%s' from='%s' to='%s'><query xmlns='http://jabber.org/protocol/disco#info'>`+
			`<identity category='server' type='im' name='Tayga XMPP'/>`+
			`<identity category='pubsub' type='pep'/>`+
			`<feature var='urn:xmpp:ping'/>`+
			`<feature var='jabber:iq:roster'/>`+
			`<feature var='vcard-temp'/>`+
			`<feature var='urn:xmpp:mam:2'/>`+
			`<feature var='urn:xmpp:carbons:2'/>`+
			`<feature var='http://jabber.org/protocol/pubsub'/>`+
			`<feature var='http://jabber.org/protocol/pubsub#publish'/>`+
			`<feature var='http://jabber.org/protocol/pubsub#persistent-items'/>`+
			` <feature var='http://jabber.org/protocol/pubsub#publish-options'/>`+
			`<feature var='http://jabber.org/protocol/pubsub#access-open'/>`+
			`<feature var='http://jabber.org/protocol/pubsub#auto-create'/>`+
			`<feature var='http://jabber.org/protocol/pubsub#retrieve-items'/>`+
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
		items, err := s.srv.effectiveRoster(ctx, s.user.ID)
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
		// Subscription grants are server-owned; roster edits cannot grant presence access.
		sub, ask := "none", ""
		items, err := s.srv.listRoster(ctx, s.user.ID)
		if err != nil {
			return s.writeString(iqInternalError(id))
		}
		for _, old := range items {
			if old.JID == ParseJID(jid).Bare() {
				sub, ask = old.Subscription, old.Ask
			}
		}
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
		`<iq type='set' id='%s'><query xmlns='jabber:iq:roster'><item jid='%s' name='%s' subscription='%s'>`,
		xmlEscape(randomID()), xmlEscape(jid), xmlEscape(name), xmlEscape(sub),
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

func (s *Session) handleMAMIQ(ctx context.Context, id, typ, inner string) error {
	if typ == "get" {
		var root struct{ XMLName xml.Name }
		if xml.Unmarshal([]byte(inner), &root) != nil || root.XMLName.Local != "query" {
			return s.writeString(iqBadRequest(id))
		}
		return s.writeString(fmt.Sprintf(`<iq type='result' id='%s' from='%s' to='%s'><query xmlns='urn:xmpp:mam:2'><x xmlns='jabber:x:data' type='form'><field var='FORM_TYPE' type='hidden'><value>urn:xmpp:mam:2</value></field><field var='with' type='jid-single'/><field var='start' type='text-single'/><field var='end' type='text-single'/></x></query></iq>`, xmlEscape(id), xmlEscape(s.JID.Bare()), xmlEscape(s.JID.Full())))
	}
	if typ != "set" {
		return s.writeString(iqBadRequest(id))
	}
	var query struct {
		XMLName xml.Name
		QueryID string `xml:"queryid,attr"`
		Form    struct {
			Fields []struct {
				Var   string `xml:"var,attr"`
				Value string `xml:"value"`
			} `xml:"field"`
		} `xml:"jabber:x:data x"`
		Set struct {
			Max    *string `xml:"max"`
			After  *string `xml:"after"`
			Before *string `xml:"before"`
		} `xml:"http://jabber.org/protocol/rsm set"`
	}
	if err := xml.Unmarshal([]byte(inner), &query); err != nil || query.XMLName.Local != "query" {
		return s.writeString(iqBadRequest(id))
	}
	f := mamFilter{Limit: 50, After: query.Set.After, Before: query.Set.Before}
	if f.After != nil && (f.Before != nil || *f.After == "") {
		return s.writeString(iqBadRequest(id))
	}
	if query.Set.Max != nil {
		n, err := strconv.Atoi(*query.Set.Max)
		if err != nil || n < 0 {
			return s.writeString(iqBadRequest(id))
		}
		if n > 100 {
			n = 100
		}
		f.Limit = n
	}
	for _, field := range query.Form.Fields {
		switch field.Var {
		case "FORM_TYPE":
			if field.Value != "urn:xmpp:mam:2" {
				return s.writeString(iqBadRequest(id))
			}
		case "with":
			f.With = ParseJID(field.Value).Bare()
		case "start", "end":
			stamp, err := time.Parse(time.RFC3339Nano, field.Value)
			if err != nil {
				return s.writeString(iqBadRequest(id))
			}
			stamp = stamp.UTC()
			if field.Var == "start" {
				f.Start = &stamp
			} else {
				f.End = &stamp
			}
		}
	}
	if f.Start != nil && f.End != nil && f.Start.After(*f.End) {
		return s.writeString(iqBadRequest(id))
	}
	rows, total, complete, err := s.srv.mamPage(ctx, s.JID.Bare(), f)
	if err == errMAMCursor {
		return s.writeString(fmt.Sprintf(`<iq type='error' id='%s'><error type='cancel'><item-not-found xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`, xmlEscape(id)))
	}
	if err != nil {
		return s.writeString(iqInternalError(id))
	}
	queryID := query.QueryID
	for _, r := range rows {
		stamp := r.CreatedAt.UTC().Format(time.RFC3339)
		msg := fmt.Sprintf(
			`<message from='%s' to='%s'><result xmlns='urn:xmpp:mam:2' queryid='%s' id='%s'>`+
				`<forwarded xmlns='urn:xmpp:forward:0'><delay xmlns='urn:xmpp:delay' stamp='%s'/>%s</forwarded>`+
				`</result></message>`,
			xmlEscape(s.JID.Bare()), xmlEscape(s.JID.Full()), xmlEscape(queryID), xmlEscape(r.StanzaID),
			xmlEscape(stamp), r.Stanza,
		)
		s.SendRaw([]byte(msg))
	}
	bounds := ""
	if len(rows) > 0 {
		bounds = fmt.Sprintf("<first>%s</first><last>%s</last>", xmlEscape(rows[0].StanzaID), xmlEscape(rows[len(rows)-1].StanzaID))
	}
	return s.writeString(fmt.Sprintf(
		`<iq type='result' id='%s' from='%s' to='%s'><fin xmlns='urn:xmpp:mam:2' complete='%t'>`+
			`<set xmlns='http://jabber.org/protocol/rsm'>%s<count>%d</count></set></fin></iq>`,
		xmlEscape(id), xmlEscape(s.JID.Bare()), xmlEscape(s.JID.Full()), complete, bounds, total,
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

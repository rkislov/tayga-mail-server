package xmpp

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"strings"
)

type pepForm struct {
	Fields []struct {
		Name   string   `xml:"var,attr"`
		Values []string `xml:"value"`
	} `xml:"field"`
}
type pepOperation struct {
	XMLName xml.Name
	Node    string `xml:"node,attr"`
	Items   []struct {
		ID      string `xml:"id,attr"`
		Payload string `xml:",innerxml"`
	} `xml:"item"`
	Form pepForm `xml:"x"`
}

func omemoNode(node string) bool {
	return node == "eu.siacs.conversations.axolotl.devicelist" || strings.HasPrefix(node, "eu.siacs.conversations.axolotl.bundles:") || node == "urn:xmpp:omemo:2:devices" || node == "urn:xmpp:omemo:2:bundles"
}

func (s *Session) handlePubSubIQ(ctx context.Context, id, typ, to, inner string) error {
	if s.user == nil {
		return s.writeString(iqAuthError(id))
	}
	owner := s.JID.Bare()
	if to != "" {
		owner = ParseJID(to).Bare()
	}
	var doc struct {
		XMLName xml.Name
		Ops     []pepOperation `xml:",any"`
	}
	if len(inner) > 256*1024 || xml.Unmarshal([]byte(inner), &doc) != nil || doc.XMLName.Local != "pubsub" {
		return s.writeString(iqBadRequest(id))
	}
	var op *pepOperation
	var form pepForm
	for i := range doc.Ops {
		if doc.Ops[i].XMLName.Local == "publish-options" {
			form = doc.Ops[i].Form
			continue
		}
		if op != nil {
			return s.writeString(iqBadRequest(id))
		}
		op = &doc.Ops[i]
	}
	if op == nil || op.Node == "" || len(op.Node) > 1024 {
		return s.writeString(iqBadRequest(id))
	}
	node := op.Node
	if typ == "set" && owner != s.JID.Bare() {
		return s.writeString(iqForbidden(id))
	}
	reply := func(content string) error {
		return s.writeString(fmt.Sprintf(`<iq type='result' id='%s' from='%s' to='%s'>%s</iq>`, xmlEscape(id), xmlEscape(owner), xmlEscape(s.JID.Full()), content))
	}
	// OMEMO key material is public. Other PEP data retains presence-based access.
	if typ == "get" && owner != s.JID.Bare() && !omemoNode(node) {
		var access string
		err := s.srv.db.QueryRowContext(ctx, s.srv.rebind(`SELECT access_model FROM xmpp_pep_nodes WHERE owner_bare_jid=? AND node=?`), owner, node).Scan(&access)
		if err != nil || access != "open" {
			return s.writeString(iqForbidden(id))
		}
	}
	switch op.XMLName.Local {
	case "publish", "configure":
		if op.XMLName.Local == "configure" {
			if owner != s.JID.Bare() || !omemoNode(node) {
				return s.writeString(iqForbidden(id))
			}
			if typ == "get" {
				var max int
				var last bool
				err := s.srv.db.QueryRowContext(ctx, s.srv.rebind(`SELECT max_items,send_last FROM xmpp_pep_nodes WHERE owner_bare_jid=? AND node=?`), owner, node).Scan(&max, &last)
				if err != nil {
					return s.writeString(iqBadRequest(id))
				}
				maxValue := "max"
				if max == 1 {
					maxValue = "1"
				}
				lastValue := "never"
				if last {
					lastValue = "on_sub_and_presence"
				}
				return reply(`<pubsub xmlns='http://jabber.org/protocol/pubsub#owner'><configure node='` + xmlEscape(node) + `'><x xmlns='jabber:x:data' type='form'><field var='FORM_TYPE' type='hidden'><value>http://jabber.org/protocol/pubsub#node_config</value></field><field var='pubsub#access_model'><value>open</value></field><field var='pubsub#persist_items'><value>true</value></field><field var='pubsub#max_items'><value>` + maxValue + `</value></field><field var='pubsub#send_last_published_item'><value>` + lastValue + `</value></field></x></configure></pubsub>`)
			}
			form = op.Form
		}
		if typ != "set" || (!omemoNode(node) && len(form.Fields) > 0) {
			return s.writeString(iqBadRequest(id))
		}
		for _, f := range form.Fields {
			if len(f.Values) != 1 {
				return s.writeString(iqBadRequest(id))
			}
			v := f.Values[0]
			switch f.Name {
			case "FORM_TYPE":
				if v != "http://jabber.org/protocol/pubsub#publish-options" && v != "http://jabber.org/protocol/pubsub#node_config" {
					return s.writeString(iqBadRequest(id))
				}
			case "pubsub#access_model":
				if v != "open" {
					return s.writeString(iqBadRequest(id))
				}
			case "pubsub#persist_items":
				if v != "true" && v != "1" && v != "max" {
					return s.writeString(iqBadRequest(id))
				}
			case "pubsub#max_items":
				if v != "max" && v != "1" {
					return s.writeString(iqBadRequest(id))
				}
			case "pubsub#send_last_published_item":
				if v != "on_sub_and_presence" && v != "never" {
					return s.writeString(iqBadRequest(id))
				}
			default:
				return s.writeString(iqBadRequest(id))
			}
		}
		if op.XMLName.Local == "publish" && (len(op.Items) != 1 || strings.TrimSpace(op.Items[0].Payload) == "") {
			return s.writeString(iqBadRequest(id))
		}
		nodeID, err := s.srv.ensurePEPNode(ctx, owner, node)
		if err != nil {
			return s.writeString(iqInternalError(id))
		}
		if omemoNode(node) {
			_, err = s.srv.db.ExecContext(ctx, s.srv.rebind(`UPDATE xmpp_pep_nodes SET access_model='open' WHERE id=?`), nodeID)
		}
		if err != nil {
			return s.writeString(iqInternalError(id))
		}
		for _, f := range form.Fields {
			switch f.Name {
			case "pubsub#max_items":
				max := 0
				if f.Values[0] == "1" {
					max = 1
				}
				_, err = s.srv.db.ExecContext(ctx, s.srv.rebind(`UPDATE xmpp_pep_nodes SET max_items=? WHERE id=?`), max, nodeID)
			case "pubsub#send_last_published_item":
				_, err = s.srv.db.ExecContext(ctx, s.srv.rebind(`UPDATE xmpp_pep_nodes SET send_last=? WHERE id=?`), f.Values[0] != "never", nodeID)
			}
			if err != nil {
				return s.writeString(iqInternalError(id))
			}
		}
		if op.XMLName.Local == "configure" {
			return reply("")
		}
		item := op.Items[0]
		if item.ID == "" {
			item.ID = "current"
		}
		if err = s.srv.publishPEP(ctx, owner, node, item.ID, item.Payload); err != nil {
			return s.writeString(iqInternalError(id))
		}
		s.notifyPEP(ctx, owner, node, `<item id='`+xmlEscape(item.ID)+`'>`+item.Payload+`</item>`)
		return reply(`<pubsub xmlns='http://jabber.org/protocol/pubsub'><publish node='` + xmlEscape(node) + `'><item id='` + xmlEscape(item.ID) + `'/></publish></pubsub>`)
	case "items":
		if typ != "get" {
			return s.writeString(iqBadRequest(id))
		}
		items, err := s.srv.getPEPItems(ctx, owner, node)
		if err != nil {
			return s.writeString(iqInternalError(id))
		}
		var b strings.Builder
		b.WriteString(`<pubsub xmlns='http://jabber.org/protocol/pubsub'><items node='` + xmlEscape(node) + `'>`)
		for _, item := range items {
			wanted := len(op.Items) == 0
			for _, requested := range op.Items {
				if requested.ID == item.ItemID {
					wanted = true
				}
			}
			if wanted {
				b.WriteString(`<item id='` + xmlEscape(item.ItemID) + `'>` + item.Payload + `</item>`)
			}
		}
		b.WriteString(`</items></pubsub>`)
		return reply(b.String())
	case "retract":
		if typ != "set" || len(op.Items) != 1 {
			return s.writeString(iqBadRequest(id))
		}
		_, err := s.srv.db.ExecContext(ctx, s.srv.rebind(`DELETE FROM xmpp_pep_items WHERE node_id IN (SELECT id FROM xmpp_pep_nodes WHERE owner_bare_jid=? AND node=?) AND item_id=?`), owner, node, op.Items[0].ID)
		if err != nil {
			return s.writeString(iqInternalError(id))
		}
		s.notifyPEP(ctx, owner, node, `<retract id='`+xmlEscape(op.Items[0].ID)+`'/>`)
		return reply("")
	case "delete":
		if typ != "set" {
			return s.writeString(iqBadRequest(id))
		}
		if err := s.srv.deletePEPNode(ctx, owner, node); err != nil && err != sql.ErrNoRows {
			return s.writeString(iqInternalError(id))
		}
		return reply("")
	}
	return s.writeString(iqBadRequest(id))
}

func (s *Session) notifyPEP(ctx context.Context, owner, node, payload string) {
	peers := map[string]bool{owner: true}
	roster, err := s.srv.listRoster(ctx, s.user.ID)
	if err == nil {
		for _, item := range roster {
			if item.Subscription == "both" || item.Subscription == "from" {
				peers[item.JID] = true
			}
		}
	}
	for peer := range peers {
		for _, session := range s.srv.hub.Sessions(peer) {
			event := fmt.Sprintf(`<message from='%s' to='%s'><event xmlns='http://jabber.org/protocol/pubsub#event'><items node='%s'>%s</items></event></message>`, xmlEscape(owner), xmlEscape(session.JID.Full()), xmlEscape(node), payload)
			session.SendRaw([]byte(event))
		}
	}
}

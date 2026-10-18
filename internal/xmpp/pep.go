package xmpp

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

const pepNS = "http://jabber.org/protocol/pubsub"

type pepForm struct {
	Fields []struct {
		Name   string   `xml:"var,attr"`
		Values []string `xml:"value"`
	} `xml:"field"`
}
type pepOperation struct {
	XMLName xml.Name
	Node    string `xml:"node,attr"`
	JID     string `xml:"jid,attr"`
	Items   []struct {
		ID      string `xml:"id,attr"`
		Payload string `xml:",innerxml"`
	} `xml:"item"`
	Form pepForm `xml:"jabber:x:data x"`
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
	reply := func(content string) error {
		return s.writeString(fmt.Sprintf(`<iq type='result' id='%s' from='%s' to='%s'>%s</iq>`, xmlEscape(id), xmlEscape(owner), xmlEscape(s.JID.Full()), content))
	}
	fail := func(condition string) error {
		return s.writeString(fmt.Sprintf(`<iq type='error' id='%s' from='%s' to='%s'><error type='cancel'><%s xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`, xmlEscape(id), xmlEscape(owner), xmlEscape(s.JID.Full()), condition))
	}
	var doc struct {
		XMLName xml.Name
		Ops     []pepOperation `xml:",any"`
	}
	if len(inner) > 256*1024 || xml.Unmarshal([]byte(inner), &doc) != nil || doc.XMLName.Local != "pubsub" || (doc.XMLName.Space != pepNS && doc.XMLName.Space != pepNS+"#owner") {
		return fail("bad-request")
	}
	var op *pepOperation
	var form pepForm
	var options, configuring bool
	for i := range doc.Ops {
		next := &doc.Ops[i]
		if next.XMLName.Space != doc.XMLName.Space {
			return fail("bad-request")
		}
		if next.XMLName.Local == "publish-options" {
			if options {
				return fail("bad-request")
			}
			options = true
			form = next.Form
			continue
		}
		if op != nil {
			if op.XMLName.Local == "create" && next.XMLName.Local == "configure" && !configuring && (next.Node == "" || next.Node == op.Node) {
				configuring = true
				form = next.Form
				continue
			}
			return fail("bad-request")
		}
		op = next
	}
	if op == nil || op.Node == "" || len(op.Node) > 1024 {
		return fail("bad-request")
	}
	operation, node := op.XMLName.Local, op.Node
	if options && operation != "publish" {
		return fail("bad-request")
	}
	if operation == "configure" || operation == "delete" {
		if doc.XMLName.Space != pepNS+"#owner" {
			return fail("bad-request")
		}
	} else if doc.XMLName.Space != pepNS {
		return fail("bad-request")
	}
	if operation == "configure" {
		form = op.Form
	}
	if s.srv.log != nil && omemoNode(node) {
		s.srv.log.Info("xmpp OMEMO PEP request", "user_id", s.user.ID, "operation", operation, "type", typ, "node", node)
	}
	if (operation != "items" && operation != "subscribe" && operation != "unsubscribe") && owner != s.JID.Bare() {
		return fail("forbidden")
	}
	var nodeID, access string
	err := s.srv.db.QueryRowContext(ctx, s.srv.rebind(`SELECT id,access_model FROM xmpp_pep_nodes WHERE owner_bare_jid=? AND node=?`), owner, node).Scan(&nodeID, &access)
	if err != nil && err != sql.ErrNoRows {
		return fail("internal-server-error")
	}
	exists := err == nil
	if !exists && operation != "create" && operation != "publish" {
		return fail("item-not-found")
	}
	if exists && operation == "create" {
		return fail("conflict")
	}
	canRead := owner == s.JID.Bare() || access == "open" || omemoNode(node) || (access == "presence" && s.srv.pepPresenceAllowed(ctx, owner, s.JID.Bare()))
	switch operation {
	case "subscribe", "unsubscribe":
		if typ != "set" {
			return fail("bad-request")
		}
		subscriber, err := NormalizeJID(op.JID)
		if err != nil || subscriber.Bare() != s.JID.Bare() {
			return fail("forbidden")
		}
		if operation == "unsubscribe" {
			if _, err = s.srv.db.ExecContext(ctx, s.srv.rebind(`DELETE FROM xmpp_pep_subscriptions WHERE node_id=? AND jid=?`), nodeID, subscriber.Full()); err != nil {
				return fail("internal-server-error")
			}
			return reply("")
		}
		if !canRead {
			return fail("forbidden")
		}
		subid := storage.NewID()
		if _, err = s.srv.db.ExecContext(ctx, s.srv.rebind(`INSERT INTO xmpp_pep_subscriptions(node_id,jid,subid) VALUES(?,?,?) ON CONFLICT(node_id,jid) DO NOTHING`), nodeID, subscriber.Full(), subid); err != nil {
			return fail("internal-server-error")
		}
		if err = s.srv.db.QueryRowContext(ctx, s.srv.rebind(`SELECT subid FROM xmpp_pep_subscriptions WHERE node_id=? AND jid=?`), nodeID, subscriber.Full()).Scan(&subid); err != nil {
			return fail("internal-server-error")
		}
		if err = reply(`<pubsub xmlns='` + pepNS + `'><subscription node='` + xmlEscape(node) + `' jid='` + xmlEscape(subscriber.Full()) + `' subscription='subscribed' subid='` + xmlEscape(subid) + `'/></pubsub>`); err != nil {
			return err
		}
		for _, session := range s.srv.hub.Sessions(subscriber.Bare()) {
			if subscriber.Resource == "" || subscriber.Full() == session.JID.Full() {
				session.sendLastPEP(ctx, owner, node)
			}
		}
		return nil
	case "create", "publish", "configure":
		if operation == "configure" && typ == "get" {
			var max int
			var last, retract, del bool
			if err := s.srv.db.QueryRowContext(ctx, s.srv.rebind(`SELECT max_items,send_last,notify_retract,notify_delete FROM xmpp_pep_nodes WHERE id=?`), nodeID).Scan(&max, &last, &retract, &del); err != nil {
				return fail("internal-server-error")
			}
			maxValue := "max"
			if max == 1 {
				maxValue = "1"
			}
			lastValue := "never"
			if last {
				lastValue = "on_sub_and_presence"
			}
			field := func(name, value string) string {
				return `<field var='` + name + `'><value>` + xmlEscape(value) + `</value></field>`
			}
			return reply(`<pubsub xmlns='` + pepNS + `#owner'><configure node='` + xmlEscape(node) + `'><x xmlns='jabber:x:data' type='form'><field var='FORM_TYPE' type='hidden'><value>` + pepNS + `#node_config</value></field>` + field("pubsub#access_model", access) + field("pubsub#persist_items", "true") + field("pubsub#max_items", maxValue) + field("pubsub#send_last_published_item", lastValue) + field("pubsub#notify_retract", fmt.Sprint(retract)) + field("pubsub#notify_delete", fmt.Sprint(del)) + `</x></configure></pubsub>`)
		}
		if typ != "set" {
			return fail("bad-request")
		}
		config, valid := parsePEPConfig(form)
		if !valid {
			return fail("bad-request")
		}
		if operation == "publish" && (len(op.Items) != 1 || strings.TrimSpace(op.Items[0].Payload) == "") {
			return fail("bad-request")
		}
		if !exists {
			nodeID, err = s.srv.ensurePEPNode(ctx, owner, node)
			if err != nil {
				return fail("internal-server-error")
			}
		}
		if omemoNode(node) {
			config["access_model"] = "open"
		}
		// Column names come only from parsePEPConfig, never from client input.
		for column, value := range config {
			if _, err = s.srv.db.ExecContext(ctx, s.srv.rebind(`UPDATE xmpp_pep_nodes SET `+column+`=? WHERE id=?`), value, nodeID); err != nil {
				return fail("internal-server-error")
			}
		}
		if operation == "configure" {
			return reply("")
		}
		if operation == "create" {
			return reply(`<pubsub xmlns='` + pepNS + `'><create node='` + xmlEscape(node) + `'/></pubsub>`)
		}
		item := op.Items[0]
		if item.ID == "" {
			item.ID = "current"
		}
		if err = s.srv.publishPEP(ctx, owner, node, item.ID, item.Payload); err != nil {
			return fail("internal-server-error")
		}
		s.notifyPEP(ctx, owner, node, `<item id='`+xmlEscape(item.ID)+`'>`+item.Payload+`</item>`)
		return reply(`<pubsub xmlns='` + pepNS + `'><publish node='` + xmlEscape(node) + `'><item id='` + xmlEscape(item.ID) + `'/></publish></pubsub>`)
	case "items":
		if typ != "get" {
			return fail("bad-request")
		}
		if !canRead {
			return fail("forbidden")
		}
		items, err := s.srv.getPEPItems(ctx, owner, node)
		if err != nil {
			return fail("internal-server-error")
		}
		var b strings.Builder
		b.WriteString(`<pubsub xmlns='` + pepNS + `'><items node='` + xmlEscape(node) + `'>`)
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
		if typ != "set" || len(op.Items) != 1 || op.Items[0].ID == "" {
			return fail("bad-request")
		}
		result, err := s.srv.db.ExecContext(ctx, s.srv.rebind(`DELETE FROM xmpp_pep_items WHERE node_id=? AND item_id=?`), nodeID, op.Items[0].ID)
		if err != nil {
			return fail("internal-server-error")
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return fail("item-not-found")
		}
		var notify bool
		if err = s.srv.db.QueryRowContext(ctx, s.srv.rebind(`SELECT notify_retract FROM xmpp_pep_nodes WHERE id=?`), nodeID).Scan(&notify); err != nil {
			return fail("internal-server-error")
		}
		if notify {
			s.notifyPEP(ctx, owner, node, `<retract id='`+xmlEscape(op.Items[0].ID)+`'/>`)
		}
		return reply("")
	case "delete":
		if typ != "set" {
			return fail("bad-request")
		}
		var notify bool
		if err = s.srv.db.QueryRowContext(ctx, s.srv.rebind(`SELECT notify_delete FROM xmpp_pep_nodes WHERE id=?`), nodeID).Scan(&notify); err != nil {
			return fail("internal-server-error")
		}
		recipients := s.pepRecipients(ctx, owner, node)
		if err = s.srv.deletePEPNode(ctx, owner, node); err != nil {
			return fail("internal-server-error")
		}
		if notify {
			for _, session := range recipients {
				session.sendPEPEvent(owner, `<delete node='`+xmlEscape(node)+`'/>`)
			}
		}
		return reply("")
	}
	return fail("feature-not-implemented")
}

func parsePEPConfig(form pepForm) (map[string]any, bool) {
	out := map[string]any{}
	seen := map[string]bool{}
	for _, f := range form.Fields {
		if len(f.Values) != 1 || seen[f.Name] {
			return nil, false
		}
		seen[f.Name] = true
		v := f.Values[0]
		switch f.Name {
		case "FORM_TYPE":
			if v != pepNS+"#publish-options" && v != pepNS+"#node_config" {
				return nil, false
			}
		case "pubsub#access_model":
			if v != "open" && v != "presence" {
				return nil, false
			}
			out["access_model"] = v
		case "pubsub#persist_items":
			if v != "true" && v != "1" {
				return nil, false
			}
		case "pubsub#max_items":
			if v != "max" && v != "1" {
				return nil, false
			}
			max := 0
			if v == "1" {
				max = 1
			}
			out["max_items"] = max
		case "pubsub#send_last_published_item":
			if v != "on_sub_and_presence" && v != "never" {
				return nil, false
			}
			out["send_last"] = v != "never"
		case "pubsub#notify_retract", "pubsub#notify_delete":
			if v != "true" && v != "false" && v != "0" && v != "1" {
				return nil, false
			}
			out[strings.TrimPrefix(f.Name, "pubsub#")] = v == "true" || v == "1"
		default:
			return nil, false
		}
	}
	return out, true
}

func pepDefaultMaxItems(node string) int {
	if node == "urn:xmpp:omemo:2:bundles" {
		return 0
	}
	return 1
}

package xmpp

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// Query each resource directly; do not trust an unverified shared caps cache.
func (s *Session) requestPEPFeatures(inner string) {
	var p struct {
		Caps struct {
			Node string `xml:"node,attr"`
			Ver  string `xml:"ver,attr"`
		} `xml:"http://jabber.org/protocol/caps c"`
	}
	if xml.Unmarshal([]byte("<presence>"+inner+"</presence>"), &p) != nil {
		return
	}
	node := ""
	if p.Caps.Node != "" && p.Caps.Ver != "" {
		node = p.Caps.Node + "#" + p.Caps.Ver
	}
	if len(node) > 2048 {
		return
	}
	s.mu.Lock()
	if s.pepCaps == node && (s.pepInterest != nil || (s.pepDiscoID != "" && time.Since(s.pepDiscoAt) < 30*time.Second)) {
		s.mu.Unlock()
		return
	}
	s.pepCaps = node
	s.pepInterest = nil
	s.pepDiscoID = randomID()
	s.pepDiscoNode = node
	s.pepDiscoAt = time.Now()
	id := s.pepDiscoID
	s.mu.Unlock()
	_ = s.writeString(fmt.Sprintf(`<iq type='get' from='%s' to='%s' id='%s'><query xmlns='http://jabber.org/protocol/disco#info'%s/></iq>`, xmlEscape(s.JID.Domain), xmlEscape(s.JID.Full()), xmlEscape(id), attrIf("node", node)))
}

func (s *Session) handlePEPFeatures(id, typ, from, to, inner string) bool {
	if typ != "result" && typ != "error" {
		return false
	}
	s.mu.Lock()
	if id == "" || id != s.pepDiscoID {
		s.mu.Unlock()
		return false
	}
	if (from != "" && from != s.JID.Full()) || (to != "" && to != s.JID.Domain) {
		s.mu.Unlock()
		return true
	}
	node := s.pepDiscoNode
	s.pepDiscoID = ""
	s.mu.Unlock()
	if typ == "error" {
		return true
	}
	var q struct {
		XMLName  xml.Name
		Node     string `xml:"node,attr"`
		Features []struct {
			Var string `xml:"var,attr"`
		} `xml:"http://jabber.org/protocol/disco#info feature"`
	}
	if len(inner) > 128*1024 || xml.Unmarshal([]byte(inner), &q) != nil || q.XMLName != (xml.Name{Space: "http://jabber.org/protocol/disco#info", Local: "query"}) || q.Node != node || len(q.Features) > 512 {
		return true
	}
	interests := map[string]bool{}
	for _, f := range q.Features {
		if strings.HasSuffix(f.Var, "+notify") && len(f.Var) > 7 && len(f.Var) <= 1031 {
			interests[strings.TrimSuffix(f.Var, "+notify")] = true
		}
	}
	s.mu.Lock()
	s.pepInterest = interests
	s.mu.Unlock()
	s.sendCurrentDeviceLists(context.Background())
	return true
}

func (s *Server) pepPresenceAllowed(ctx context.Context, owner, peer string) bool {
	var sub string
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT r.subscription FROM xmpp_roster r JOIN users u ON u.id=r.user_id WHERE u.email=? AND r.jid=?`), owner, peer).Scan(&sub)
	return err == nil && (sub == "from" || sub == "both")
}

// Select explicit subscribers and presence subscribers interested in this node.
// Explicit subscriptions never bypass the node's current access policy.
func (s *Session) pepRecipients(ctx context.Context, owner, node string) []*Session {
	var nodeID, access string
	if s.srv.db.QueryRowContext(ctx, s.srv.rebind(`SELECT id,access_model FROM xmpp_pep_nodes WHERE owner_bare_jid=? AND node=?`), owner, node).Scan(&nodeID, &access) != nil {
		return nil
	}
	explicit := map[string]bool{}
	rows, err := s.srv.db.QueryContext(ctx, s.srv.rebind(`SELECT jid FROM xmpp_pep_subscriptions WHERE node_id=?`), nodeID)
	if err != nil {
		return nil
	}
	for rows.Next() {
		var jid string
		if rows.Scan(&jid) != nil {
			rows.Close()
			return nil
		}
		explicit[jid] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil
	}
	peers := map[string]bool{owner: true}
	for jid := range explicit {
		peers[ParseJID(jid).Bare()] = true
	}
	user, err := s.srv.store.GetUserByEmail(ctx, owner)
	if err == nil {
		roster, err := s.srv.listRoster(ctx, user.ID)
		if err == nil {
			for _, item := range roster {
				if item.Subscription == "both" || item.Subscription == "from" {
					peers[item.JID] = true
				}
			}
		}
	}
	var result []*Session
	for peer := range peers {
		allowed := peer == owner || access == "open" || omemoNode(node) || (access == "presence" && s.srv.pepPresenceAllowed(ctx, owner, peer))
		if !allowed {
			continue
		}
		for _, session := range s.srv.hub.Sessions(peer) {
			session.mu.Lock()
			interested := session.pepInterest[node]
			session.mu.Unlock()
			if interested || explicit[session.JID.Full()] || explicit[peer] {
				result = append(result, session)
			}
		}
	}
	return result
}

func (s *Session) sendPEPEvent(owner, content string) {
	s.SendRaw([]byte(fmt.Sprintf(`<message type='headline' id='%s' from='%s' to='%s'><event xmlns='http://jabber.org/protocol/pubsub#event'>%s</event></message>`, xmlEscape(randomID()), xmlEscape(owner), xmlEscape(s.JID.Full()), content)))
}
func (s *Session) notifyPEP(ctx context.Context, owner, node, payload string) {
	for _, session := range s.pepRecipients(ctx, owner, node) {
		session.sendPEPEvent(owner, `<items node='`+xmlEscape(node)+`'>`+payload+`</items>`)
	}
}
func (s *Session) sendLastPEP(ctx context.Context, owner, node string) {
	var last bool
	if s.srv.db.QueryRowContext(ctx, s.srv.rebind(`SELECT send_last FROM xmpp_pep_nodes WHERE owner_bare_jid=? AND node=?`), owner, node).Scan(&last) != nil || !last {
		return
	}
	items, err := s.srv.getPEPItems(ctx, owner, node)
	if err != nil {
		return
	}
	for _, item := range items {
		s.sendPEPEvent(owner, `<items node='`+xmlEscape(node)+`'><item id='`+xmlEscape(item.ItemID)+`'>`+item.Payload+`</item></items>`)
	}
}
func (s *Session) sendCurrentDeviceLists(ctx context.Context) {
	if s.user == nil {
		return
	}
	s.mu.Lock()
	nodes := make([]string, 0, len(s.pepInterest))
	for node := range s.pepInterest {
		nodes = append(nodes, node)
	}
	s.mu.Unlock()
	roster, err := s.srv.listRoster(ctx, s.user.ID)
	if err != nil {
		return
	}
	owners := []string{s.JID.Bare()}
	for _, item := range roster {
		if item.Subscription == "to" || item.Subscription == "both" {
			owners = append(owners, item.JID)
		}
	}
	sent := map[string]bool{}
	for _, owner := range owners {
		for _, node := range nodes {
			s.sendLastPEP(ctx, owner, node)
			sent[owner+"\x00"+node] = true
		}
	}
	rows, err := s.srv.db.QueryContext(ctx, s.srv.rebind(`SELECT n.owner_bare_jid,n.node,n.access_model FROM xmpp_pep_subscriptions p JOIN xmpp_pep_nodes n ON n.id=p.node_id WHERE p.jid=? OR p.jid=?`), s.JID.Bare(), s.JID.Full())
	if err != nil {
		return
	}
	type subscription struct{ owner, node, access string }
	var subscriptions []subscription
	for rows.Next() {
		var sub subscription
		if rows.Scan(&sub.owner, &sub.node, &sub.access) != nil {
			rows.Close()
			return
		}
		subscriptions = append(subscriptions, sub)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return
	}
	for _, sub := range subscriptions {
		if !sent[sub.owner+"\x00"+sub.node] && (sub.owner == s.JID.Bare() || sub.access == "open" || omemoNode(sub.node) || (sub.access == "presence" && s.srv.pepPresenceAllowed(ctx, sub.owner, s.JID.Bare()))) {
			s.sendLastPEP(ctx, sub.owner, sub.node)
		}
	}
}

package xmpp

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *Server) changeSubscription(ctx context.Context, owner, peer string, toSide, enabled bool) error {
	user, err := s.store.GetUserByEmail(ctx, owner)
	if err != nil {
		return err
	}
	items, err := s.listRoster(ctx, user.ID)
	if err != nil {
		return err
	}
	item := rosterItem{JID: peer, Subscription: "none", GroupsJSON: "[]"}
	for _, v := range items {
		if v.JID == peer {
			item = v
			break
		}
	}
	to, from := item.Subscription == "to" || item.Subscription == "both", item.Subscription == "from" || item.Subscription == "both"
	if toSide {
		to = enabled
	} else {
		from = enabled
	}
	item.Subscription = "none"
	if to && from {
		item.Subscription = "both"
	} else if to {
		item.Subscription = "to"
	} else if from {
		item.Subscription = "from"
	}
	if err = s.upsertRoster(ctx, user.ID, item); err != nil {
		return err
	}
	groups := []string{}
	_ = json.Unmarshal([]byte(item.GroupsJSON), &groups)
	for _, session := range s.hub.Sessions(owner) {
		_ = session.pushRosterPush(peer, "", item.Name, item.Subscription, groups)
		break
	}
	if !toSide && enabled {
		s.currentPresence(ctx, ParseJID(owner), ParseJID(peer), func(raw []byte) { s.hub.Route(peer, raw) })
	}
	return nil
}
func (s *Session) sendCurrentDeviceLists(ctx context.Context) {
	if s.user == nil {
		return
	}
	roster, err := s.srv.listRoster(ctx, s.user.ID)
	if err != nil {
		return
	}
	owners := []string{s.JID.Bare()}
	for _, item := range roster {
		if item.Subscription == "both" || item.Subscription == "to" {
			owners = append(owners, item.JID)
		}
	}
	for _, owner := range owners {
		for _, node := range []string{"eu.siacs.conversations.axolotl.devicelist", "urn:xmpp:omemo:2:devices"} {
			var last bool
			if err := s.srv.db.QueryRowContext(ctx, s.srv.rebind(`SELECT send_last FROM xmpp_pep_nodes WHERE owner_bare_jid=? AND node=?`), owner, node).Scan(&last); err != nil || !last {
				continue
			}
			items, err := s.srv.getPEPItems(ctx, owner, node)
			if err != nil {
				continue
			}
			for _, item := range items {
				s.SendRaw([]byte(fmt.Sprintf(`<message from='%s' to='%s'><event xmlns='http://jabber.org/protocol/pubsub#event'><items node='%s'><item id='%s'>%s</item></items></event></message>`, xmlEscape(owner), xmlEscape(s.JID.Full()), xmlEscape(node), xmlEscape(item.ItemID), item.Payload)))
			}
		}
	}
}

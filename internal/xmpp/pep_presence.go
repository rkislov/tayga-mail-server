package xmpp

import (
	"context"
	"encoding/json"
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

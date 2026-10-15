package xmpp

import (
	"context"
	"fmt"
	"sort"
)

// currentPresence sends a snapshot only after checking the owner's consent.
// Copy under the lock and write afterwards: slow clients must not block updates.
func (s *Server) currentPresence(ctx context.Context, owner, recipient JID, send func([]byte)) {
	user, err := s.store.GetUserByEmail(ctx, owner.Bare())
	if err != nil {
		return
	}
	allowed := owner.Bare() == recipient.Bare()
	if !allowed {
		roster, err := s.listRoster(ctx, user.ID)
		if err != nil {
			return
		}
		for _, item := range roster {
			if item.JID == recipient.Bare() && (item.Subscription == "from" || item.Subscription == "both") {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return
	}
	s.mu.Lock()
	snapshot := make(map[string]presenceState)
	for resource, p := range s.presence[owner.Bare()] {
		if p.State != "offline" && (!owner.IsFull() || resource == owner.Resource) {
			snapshot[resource] = p
		}
	}
	s.mu.Unlock()
	resources := make([]string, 0, len(snapshot))
	for resource := range snapshot {
		resources = append(resources, resource)
	}
	sort.Strings(resources)
	for _, resource := range resources {
		p := snapshot[resource]
		from := JID{Local: owner.Local, Domain: owner.Domain, Resource: resource}
		show := ""
		if p.State != "online" {
			show = "<show>" + xmlEscape(p.State) + "</show>"
		}
		payload := show + "<status>" + xmlEscape(p.Status) + "</status>"
		if p.Payload != "" {
			payload = p.Payload
		}
		send([]byte(fmt.Sprintf(`<presence from='%s' to='%s'%s>%s</presence>`, xmlEscape(from.Full()), xmlEscape(recipient.Full()), attrIf("id", p.ID), payload)))
	}
}

func (s *Session) sendRosterPresence(ctx context.Context) {
	if s.user == nil {
		return
	}
	items, err := s.srv.listRoster(ctx, s.user.ID)
	if err != nil {
		return
	}
	s.srv.currentPresence(ctx, ParseJID(s.JID.Bare()), s.JID, s.SendRaw)
	for _, item := range items {
		if item.Subscription == "to" || item.Subscription == "both" {
			s.srv.currentPresence(ctx, ParseJID(item.JID), s.JID, s.SendRaw)
		}
	}
}

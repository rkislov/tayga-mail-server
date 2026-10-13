package xmpp

import (
	"context"
	"sort"
)

// effectiveRoster overlays the opt-in domain directory without modifying personal contacts.
// Directory membership alone does not grant presence subscriptions.
func (s *Server) effectiveRoster(ctx context.Context, userID string) ([]rosterItem, error) {
	items, err := s.listRoster(ctx, userID)
	if err != nil {
		return nil, err
	}
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Name = s.rosterDisplayName(ctx, userID, items[i].JID, items[i].Name)
	}
	enabled, _, err := s.store.GetSetting(ctx, "domain.xmpp_gal."+user.DomainID)
	if err != nil {
		return nil, err
	}
	if enabled != "true" || !user.Enabled {
		return items, nil
	}
	users, err := s.store.ListUsersByDomain(ctx, user.DomainID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		seen[ParseJID(item.JID).Bare()] = true
	}
	for _, colleague := range users {
		jid := ParseJID(colleague.Email).Bare()
		if colleague.ID == userID || !colleague.Enabled || colleague.TenantID != user.TenantID || seen[jid] {
			continue
		}
		name := colleague.DisplayName
		if name == "" {
			name = jid
		}
		items = append(items, rosterItem{JID: jid, Name: name, Subscription: "none", GroupsJSON: `["GAL"]`})
		seen[jid] = true
	}
	sort.Slice(items, func(i, j int) bool { return items[i].JID < items[j].JID })
	return items, nil
}

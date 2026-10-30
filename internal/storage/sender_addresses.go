package storage

import (
	"context"
	"errors"
	"sort"
	"strings"
)

// SenderAddresses lists the account's primary address and current aliases.
// No grants are cached: deleting an alias revokes it on the next submission.
func (s *Store) SenderAddresses(ctx context.Context, userID string) ([]string, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.Enabled || user.AuthSource == "resource" {
		return []string{}, nil
	}
	addresses := map[string]bool{user.Email: true}
	aliases, err := s.ListAliases(ctx, userID)
	if err != nil {
		return nil, err
	}
	byDomain := map[string][]string{user.DomainID: {user.LocalPart}}
	for _, alias := range aliases {
		addresses[alias.Email] = true
		byDomain[alias.DomainID] = append(byDomain[alias.DomainID], alias.LocalPart)
	}
	for domainID, locals := range byDomain {
		domains, err := s.ListDomainAliases(ctx, domainID)
		if err != nil {
			return nil, err
		}
		for _, domain := range domains {
			for _, local := range locals {
				address := local + "@" + domain.Name
				if len(address) <= 254 {
					addresses[address] = true
				}
			}
		}
	}
	out := []string{user.Email}
	delete(addresses, user.Email)
	rest := make([]string, 0, len(addresses))
	for address := range addresses {
		rest = append(rest, address)
	}
	sort.Strings(rest)
	return append(out, rest...), nil
}

func (s *Store) CanSendFrom(ctx context.Context, userID, email string) (bool, error) {
	actor, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}
	if !actor.Enabled || actor.AuthSource == "resource" {
		return false, nil
	}
	target, err := s.ResolveRecipient(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if target.AuthSource == "resource" || target.TenantID != actor.TenantID {
		return false, nil
	}
	if target.ID == actor.ID {
		return true, nil
	}
	grant, err := s.GetMailboxDelegate(ctx, target.ID, actor.ID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return grant.CanSendAs, nil
}

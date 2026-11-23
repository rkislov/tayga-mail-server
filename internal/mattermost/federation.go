// SPDX-License-Identifier: Apache-2.0
package mattermost

import (
	"context"
	"crypto/cipher"
	"github.com/tayga/tms/internal/mmfederation"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

func (s *API) EnableFederation(key cipher.AEAD, allowedCIDRs []string) error {
	client, e := mmfederation.NewHTTPClient(allowedCIDRs)
	if e != nil {
		return e
	}
	s.federation = mmfederation.New(s.store, s.chat, key, s.siteURL, client, s.log)
	s.federation.ScopeAllowed = func(ctx context.Context, tenant, domain string) bool {
		policy, e := s.policy.Resolve(ctx, mmsettings.Scope{TenantID: tenant, DomainID: domain})
		return e == nil && policy.Enabled
	}
	return nil
}
func (s *API) Federation() *mmfederation.Service { return s.federation }
func (s *API) StartFederation(ctx context.Context) {
	if s.federation != nil {
		go s.federation.Run(ctx)
	}
}
func (s *API) federatedUserVisible(ctx context.Context, actor, target *storage.User) bool {
	if target.AuthSource != "federated" {
		return target.Enabled
	}
	if actor.TenantID != target.TenantID {
		return false
	}
	var count int
	e := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_federation_users u JOIN chat_federation_links l ON l.peer_id=u.peer_id JOIN chat_members m ON m.channel_id=l.channel_id AND m.user_id=`+s.federationPlaceholder(1)+` WHERE u.local_user_id=`+s.federationPlaceholder(2)+` AND (EXISTS(SELECT 1 FROM chat_members member WHERE member.channel_id=l.channel_id AND member.user_id=u.local_user_id) OR EXISTS(SELECT 1 FROM chat_posts p WHERE p.channel_id=l.channel_id AND p.user_id=u.local_user_id))`, actor.ID, target.ID).Scan(&count)
	return e == nil && count > 0
}
func (s *API) federationPlaceholder(n int) string {
	if d, ok := s.store.(interface{ DialectName() string }); ok && d.DialectName() == "postgres" {
		if n == 1 {
			return "$1"
		}
		return "$2"
	}
	return "?"
}

package chat

import (
	"context"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

// PublicChannels exposes the tenant's channel directory without granting access
// to history. Search names as Unicode so Cyrillic display names are searchable.
func (s *Service) PublicChannels(ctx context.Context, u *storage.User, term string, limit, offset int) ([]Channel, error) {
	if len(term) > 256 || limit < 1 || limit > 200 || offset < 0 {
		return nil, ErrInvalid
	}
	term = strings.ToLower(strings.TrimSpace(term))
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT c.id,c.tenant_id,c.name,c.display_name,c.kind,c.creator_id,c.created_at,0,0,0
        FROM chat_channels c WHERE c.tenant_id=? AND c.kind='O' ORDER BY c.name,c.id`), u.TenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Channel, 0, limit)
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(c.Name, term) && !strings.Contains(strings.ToLower(c.DisplayName), term) {
			continue
		}
		if offset > 0 {
			offset--
			continue
		}
		result = append(result, c)
		if len(result) == limit {
			break
		}
	}
	return result, rows.Err()
}

// JoinPublicChannel adds only the caller and only to a public channel in the
// caller's tenant. The INSERT predicate keeps those checks atomic.
func (s *Service) JoinPublicChannel(ctx context.Context, u *storage.User, id string) error {
	if err := s.federationMemberAllowed(ctx, id, u); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO chat_members(channel_id,user_id)
        SELECT id,? FROM chat_channels WHERE id=? AND tenant_id=? AND kind='O'
        ON CONFLICT DO NOTHING`), u.ID, id, u.TenantID)
	if err != nil {
		return err
	}
	_, err = s.Channel(ctx, u, id)
	return err
}

// ChannelByName provides metadata for public channels and the caller's own
// private channels. Post, membership and attachment handlers still check access.
func (s *Service) ChannelByName(ctx context.Context, u *storage.User, name string) (Channel, error) {
	return scanChannel(s.db.QueryRowContext(ctx, s.q(`SELECT `+channelColumns+` FROM chat_channels c
        WHERE c.tenant_id=? AND c.name=? AND (c.kind='O' OR EXISTS
        (SELECT 1 FROM chat_members m WHERE m.channel_id=c.id AND m.user_id=?))`), u.TenantID, name, u.ID))
}

func (s *Service) PublicChannel(ctx context.Context, u *storage.User, id string) (Channel, error) {
	return scanChannel(s.db.QueryRowContext(ctx, s.q(`SELECT `+channelColumns+` FROM chat_channels c
        WHERE c.id=? AND c.tenant_id=? AND c.kind='O'`), id, u.TenantID))
}

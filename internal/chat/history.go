package chat

import (
	"context"
	"sort"

	"github.com/tayga/tms/internal/storage"
)

type ReadState struct{ LastViewedAt, Unread, UnreadRoots int64 }

func (s *Service) ReadState(ctx context.Context, u *storage.User, channelID string) (ReadState, error) {
	var result ReadState
	if _, e := s.Channel(ctx, u, channelID); e != nil {
		return result, e
	}
	e := s.db.QueryRowContext(ctx, s.q(`SELECT last_viewed_at FROM chat_members WHERE channel_id=? AND user_id=?`), channelID, u.ID).Scan(&result.LastViewedAt)
	if e != nil {
		return result, e
	}
	e = s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN root_id='' THEN 1 ELSE 0 END),0) FROM chat_posts WHERE channel_id=? AND created_at>? AND user_id<>?`), channelID, result.LastViewedAt, u.ID).Scan(&result.Unread, &result.UnreadRoots)
	return result, e
}
func (s *Service) PostsAroundUnread(ctx context.Context, u *storage.User, id string, before, after int) ([]Post, error) {
	if before < 0 || after < 0 || before+after > 200 {
		return nil, ErrInvalid
	}
	state, e := s.ReadState(ctx, u, id)
	if e != nil {
		return nil, e
	}
	out := []Post{}
	for _, window := range []struct {
		comparison, order string
		limit             int
	}{{"<=", "DESC", before}, {">", "ASC", after}} {
		if window.limit == 0 {
			continue
		}
		rows, e := s.db.QueryContext(ctx, s.q(`SELECT `+postColumns+` FROM chat_posts WHERE channel_id=? AND created_at`+window.comparison+`? ORDER BY created_at `+window.order+`,id `+window.order+` LIMIT ?`), id, state.LastViewedAt, window.limit)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			p, e := scanPost(rows)
			if e != nil {
				rows.Close()
				return nil, e
			}
			out = append(out, p)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out, nil
}
func (s *Service) PostsCursor(ctx context.Context, u *storage.User, id, cursor string, after bool, limit, offset int) ([]Post, error) {
	if limit < 1 || limit > 200 || offset < 0 {
		return nil, ErrInvalid
	}
	if _, e := s.Channel(ctx, u, id); e != nil {
		return nil, e
	}
	anchor, e := s.Post(ctx, u, cursor)
	if e != nil {
		return nil, e
	}
	if anchor.ChannelID != id {
		return nil, storage.ErrNotFound
	}
	comparison, order := "<", "DESC"
	if after {
		comparison, order = ">", "ASC"
	}
	rows, e := s.db.QueryContext(ctx, s.q(`SELECT `+postColumns+` FROM chat_posts WHERE channel_id=? AND (created_at`+comparison+`? OR (created_at=? AND id`+comparison+`?)) ORDER BY created_at `+order+`,id `+order+` LIMIT ? OFFSET ?`), id, anchor.CreatedAt, anchor.CreatedAt, anchor.ID, limit, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Post{}
	for rows.Next() {
		p, e := scanPost(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	if after {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out, rows.Err()
}

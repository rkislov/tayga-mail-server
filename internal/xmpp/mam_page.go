package xmpp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var errMAMCursor = errors.New("unknown archive cursor")

type mamFilter struct {
	With          string
	Start, End    *time.Time
	After, Before *string
	Limit         int
}

// mamPage uses an owner-scoped cursor and a stable secondary ordering for ties.
func (s *Server) mamPage(ctx context.Context, owner string, f mamFilter) ([]mamRow, int, bool, error) {
	if s.db == nil {
		return nil, 0, true, nil
	}
	where := "owner_bare_jid = ?"
	args := []any{owner}
	if f.With != "" {
		where += " AND with_bare_jid = ?"
		args = append(args, f.With)
	}
	if f.Start != nil {
		where += " AND created_at >= ?"
		args = append(args, *f.Start)
	}
	if f.End != nil {
		where += " AND created_at <= ?"
		args = append(args, *f.End)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, s.rebind("SELECT COUNT(*) FROM xmpp_mam WHERE "+where), args...).Scan(&total); err != nil {
		return nil, 0, false, err
	}
	reverse := f.Before != nil
	cursor := f.After
	op := ">"
	if reverse {
		cursor = f.Before
		op = "<"
	}
	if cursor != nil && *cursor != "" {
		var stamp time.Time
		if err := s.db.QueryRowContext(ctx, s.rebind("SELECT created_at FROM xmpp_mam WHERE "+where+" AND stanza_id = ?"), append(append([]any{}, args...), *cursor)...).Scan(&stamp); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, 0, false, errMAMCursor
			}
			return nil, 0, false, err
		}
		where += fmt.Sprintf(" AND (created_at %s ? OR (created_at = ? AND stanza_id %s ?))", op, op)
		args = append(args, stamp, stamp, *cursor)
	}
	order := "ASC"
	if reverse {
		order = "DESC"
	}
	args = append(args, f.Limit+1)
	rows, err := s.db.QueryContext(ctx, s.rebind("SELECT stanza_id, stanza, created_at FROM xmpp_mam WHERE "+where+" ORDER BY created_at "+order+", stanza_id "+order+" LIMIT ?"), args...)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()
	var result []mamRow
	for rows.Next() {
		var r mamRow
		if err = rows.Scan(&r.StanzaID, &r.Stanza, &r.CreatedAt); err != nil {
			return nil, 0, false, err
		}
		result = append(result, r)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, false, err
	}
	complete := len(result) <= f.Limit
	if !complete {
		result = result[:f.Limit]
	}
	if reverse {
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
	}
	return result, total, complete, nil
}

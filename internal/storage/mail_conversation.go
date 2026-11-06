package storage

import (
	"context"
	"strings"
)

// References may cross Sent/Inbox/Archive, but must never resolve into another
// user's mailbox. Bounded batches use the RFC Message-ID index.
func (s *Store) MessagesByInternetMessageIDs(ctx context.Context, userID string, ids []string) ([]*Message, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > 128 {
		ids = ids[:128]
	}
	args := []any{userID}
	for _, id := range ids {
		args = append(args, id)
	}
	cols := "m." + strings.ReplaceAll(msgCols, ", ", ", m.")
	query := `SELECT ` + cols + ` FROM messages m JOIN mailboxes b ON b.id=m.mailbox_id WHERE b.user_id=? AND m.message_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + `) ORDER BY m.internal_date,m.uid`
	rows, err := s.db.QueryContext(ctx, s.rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*Message
	for rows.Next() {
		m, err := s.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

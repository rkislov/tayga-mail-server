package storage

import (
	"context"
	"strings"
)

// DisposeMessages atomically deletes owned mail messages, or moves them into an
// owned destination. Missing/foreign IDs are omitted so callers can return an
// individual result for each item. Returned messages retain their original paths.
func (s *Store) DisposeMessages(ctx context.Context, userID string, ids []string, destinationID string, principal ...string) ([]*Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	actorID := userID
	scopedInbox := false
	if len(principal) > 0 && principal[0] != userID {
		userID = principal[0]
		var manage any
		var inbox string
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT d.can_manage_mail,d.ews_inbox_permission FROM mailbox_delegates d JOIN users owner ON owner.id=d.owner_id JOIN users actor ON actor.id=d.delegate_id WHERE d.owner_id=? AND d.delegate_id=? AND actor.enabled=TRUE AND owner.enabled=TRUE AND actor.tenant_id=owner.tenant_id`), userID, actorID).Scan(&manage, &inbox); err != nil {
			return nil, ErrUnauthorized
		}
		if !asBool(manage) {
			if inbox != "Editor" {
				return nil, ErrUnauthorized
			}
			scopedInbox = true
		}
	}
	if destinationID != "" {
		var owner string
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT user_id FROM mailboxes WHERE id=?`), destinationID).Scan(&owner); err != nil {
			return nil, mapErr(err)
		}
		if scopedInbox {
			var name string
			if err = tx.QueryRowContext(ctx, s.rebind(`SELECT name FROM mailboxes WHERE id=?`), destinationID).Scan(&name); err != nil {
				return nil, err
			}
			if name != "Trash" && name != "Recoverable Items/Deletions" {
				return nil, ErrUnauthorized
			}
		}
		if owner != userID {
			return nil, ErrUnauthorized
		}
	}
	seen := map[string]bool{}
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	var result []*Message
	// Bound SQL parameters for both SQLite and PostgreSQL. A single transaction
	// avoids one fsync and one full FTS scan per message during Outlook cleanup.
	for start := 0; start < len(unique); start += 500 {
		end := min(start+500, len(unique))
		marks := strings.TrimSuffix(strings.Repeat("?,", end-start), ",")
		args := []any{userID}
		for _, id := range unique[start:end] {
			args = append(args, id)
		}
		folderScope := ""
		if scopedInbox {
			folderScope = " AND name='INBOX'"
		}
		query := `SELECT ` + msgCols + ` FROM messages WHERE mailbox_id IN (SELECT id FROM mailboxes WHERE user_id=?` + folderScope + `) AND id IN (` + marks + `)`
		if s.dialect == DialectPostgres {
			query += ` FOR UPDATE`
		}
		rows, e := tx.QueryContext(ctx, s.rebind(query), args...)
		if e != nil {
			return nil, e
		}
		var messages []*Message
		for rows.Next() {
			msg, e := s.scanMessage(rows)
			if e != nil {
				rows.Close()
				return nil, e
			}
			messages = append(messages, msg)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		if len(messages) == 0 {
			continue
		}
		if destinationID == "" {
			ownedArgs := make([]any, 0, len(messages))
			for _, msg := range messages {
				ownedArgs = append(ownedArgs, msg.ID)
			}
			ownedMarks := strings.TrimSuffix(strings.Repeat("?,", len(messages)), ",")
			table, column := "message_fts", "message_id"
			if s.dialect == DialectPostgres {
				table = "message_search"
			}
			if _, e = tx.ExecContext(ctx, s.rebind(`DELETE FROM `+table+` WHERE `+column+` IN (`+ownedMarks+`)`), ownedArgs...); e != nil {
				return nil, e
			}
			if _, e = tx.ExecContext(ctx, s.rebind(`DELETE FROM messages WHERE id IN (`+ownedMarks+`)`), ownedArgs...); e != nil {
				return nil, e
			}
		} else {
			var next int64
			query := `SELECT uidnext FROM mailboxes WHERE id=?`
			if s.dialect == DialectPostgres {
				query += ` FOR UPDATE`
			}
			if e = tx.QueryRowContext(ctx, s.rebind(query), destinationID).Scan(&next); e != nil {
				return nil, mapErr(e)
			}
			for _, msg := range messages {
				if msg.MailboxID == destinationID {
					continue
				}
				if _, e = tx.ExecContext(ctx, s.rebind(`UPDATE messages SET mailbox_id=?,uid=? WHERE id=?`), destinationID, next, msg.ID); e != nil {
					return nil, e
				}
				if e = s.queueSpamLearning(ctx, tx, msg.ID, msg.MailboxID, destinationID); e != nil {
					return nil, e
				}
				next++
			}
			if _, e = tx.ExecContext(ctx, s.rebind(`UPDATE mailboxes SET uidnext=? WHERE id=?`), next, destinationID); e != nil {
				return nil, e
			}
		}
		result = append(result, messages...)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

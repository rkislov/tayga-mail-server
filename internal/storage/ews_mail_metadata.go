package storage

import (
	"context"
	"database/sql"
)

func (s *Store) EWSMailMetadata(ctx context.Context, id string) (string, error) {
	var data string
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT data FROM ews_mail_metadata WHERE message_id=?`), id).Scan(&data)
	if err == sql.ErrNoRows {
		return "{}", nil
	}
	return data, err
}

// EWSMailboxMetadata fetches the sparse set of custom properties in one query.
// Listing a large mailbox must not perform one SQL round trip per message.
func (s *Store) EWSMailboxMetadata(ctx context.Context, mailboxID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT e.message_id, e.data FROM ews_mail_metadata e JOIN messages m ON m.id=e.message_id WHERE m.mailbox_id=?`), mailboxID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		result[id] = data
	}
	return result, rows.Err()
}

func (s *Store) UpdateEWSMailMetadata(ctx context.Context, id, flags, data string, actor ...string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if len(actor) > 0 {
		var mailboxID string
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT mailbox_id FROM messages WHERE id=?`), id).Scan(&mailboxID); err != nil {
			return mapErr(err)
		}
		right := "w"
		if len(actor) > 1 && actor[1] == "s" {
			right = "s"
		}
		if err = s.mailboxRightTx(ctx, tx, actor[0], mailboxID, right); err != nil {
			return err
		}
	}
	res, err := tx.ExecContext(ctx, s.rebind(`UPDATE messages SET flags=? WHERE id=?`), flags, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO ews_mail_metadata(message_id,data) VALUES(?,?) ON CONFLICT(message_id) DO UPDATE SET data=excluded.data`), id, data)
	if err != nil {
		return err
	}
	if err = s.syncMessageTask(ctx, tx, id, flags); err != nil {
		return err
	}
	return tx.Commit()
}

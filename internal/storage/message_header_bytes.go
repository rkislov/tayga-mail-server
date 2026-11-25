package storage

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
)

type legacyMessageHeaders struct {
	id                string
	subject, from, to []byte
}

func (h legacyMessageHeaders) changed() bool { return h.subject != nil || h.from != nil || h.to != nil }
func preserveHeaderText(text *string) []byte {
	clean := normalizeSearchText(*text)
	if clean == *text {
		return nil
	}
	original := []byte(*text)
	*text = clean
	return original
}

// Keep exact malformed bytes outside PostgreSQL TEXT. Original MIME is untouched.
func (s *Store) preserveMessageHeadersTx(ctx context.Context, tx *sql.Tx, h legacyMessageHeaders) error {
	if !h.changed() {
		return nil
	}
	_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO message_header_bytes(message_id,subject,from_addr,to_addr) VALUES(?,?,?,?) ON CONFLICT(message_id) DO UPDATE SET subject=COALESCE(excluded.subject,message_header_bytes.subject),from_addr=COALESCE(excluded.from_addr,message_header_bytes.from_addr),to_addr=COALESCE(excluded.to_addr,message_header_bytes.to_addr)`), h.id, h.subject, h.from, h.to)
	if err != nil {
		return err
	}
	var subject, from, to []byte
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT subject,from_addr,to_addr FROM message_header_bytes WHERE message_id=?`), h.id).Scan(&subject, &from, &to); err != nil {
		return err
	}
	for _, v := range [][2][]byte{{h.subject, subject}, {h.from, from}, {h.to, to}} {
		if v[0] != nil && !bytes.Equal(v[0], v[1]) {
			return errors.New("original header bytes verification failed")
		}
	}
	return nil
}

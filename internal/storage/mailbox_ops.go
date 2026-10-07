package storage

import (
	"context"
	"strings"
	"time"
)

const msgCols = `id, mailbox_id, uid, size, flags, internal_date, file_path, message_id, subject, from_addr, to_addr, date_hdr, archived, created_at`

func (s *Store) scanMessage(row interface{ Scan(dest ...any) error }) (*Message, error) {
	m := &Message{}
	var archived int
	err := row.Scan(
		&m.ID, &m.MailboxID, &m.UID, &m.Size, &m.Flags, &m.InternalDate, &m.FilePath, &m.MessageID,
		&m.Subject, &m.FromAddr, &m.ToAddr, &m.DateHdr, &archived, &m.CreatedAt,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	m.Archived = archived != 0
	return m, nil
}

func (s *Store) ListMailboxes(ctx context.Context, userID string) ([]*Mailbox, error) {
	q := s.rebind(`SELECT id, user_id, name, path, uidnext, uidvalidity, created_at
		FROM mailboxes WHERE user_id = ? ORDER BY name`)
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Mailbox
	for rows.Next() {
		mb := &Mailbox{}
		if err := rows.Scan(&mb.ID, &mb.UserID, &mb.Name, &mb.Path, &mb.UIDNext, &mb.UIDValidity, &mb.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, mb)
	}
	return out, rows.Err()
}

func (s *Store) CreateMailbox(ctx context.Context, userID, name, path string) (*Mailbox, error) {
	return s.EnsureMailbox(ctx, userID, name, path)
}

func (s *Store) DeleteMailbox(ctx context.Context, userID, name string) error {
	if strings.EqualFold(name, "INBOX") {
		return ErrUnauthorized
	}
	q := s.rebind(`DELETE FROM mailboxes WHERE user_id = ? AND name = ?`)
	res, err := s.db.ExecContext(ctx, q, userID, name)
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
	return nil
}

func (s *Store) RenameMailbox(ctx context.Context, userID, oldName, newName, newPath string) error {
	if strings.EqualFold(oldName, "INBOX") {
		return ErrUnauthorized
	}
	q := s.rebind(`UPDATE mailboxes SET name = ?, path = ? WHERE user_id = ? AND name = ?`)
	res, err := s.db.ExecContext(ctx, q, newName, newPath, userID, oldName)
	if err != nil {
		return mapErr(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetMessageByUID(ctx context.Context, mailboxID string, uid int64) (*Message, error) {
	q := s.rebind(`SELECT ` + msgCols + ` FROM messages WHERE mailbox_id = ? AND uid = ?`)
	return s.scanMessage(s.db.QueryRowContext(ctx, q, mailboxID, uid))
}

func (s *Store) ListMessages(ctx context.Context, mailboxID string) ([]*Message, error) {
	q := s.rebind(`SELECT ` + msgCols + ` FROM messages WHERE mailbox_id = ? ORDER BY uid`)
	rows, err := s.db.QueryContext(ctx, q, mailboxID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Message
	for rows.Next() {
		m, err := s.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) UpdateMessageFlags(ctx context.Context, messageID, flags string) error {
	q := s.rebind(`UPDATE messages SET flags = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, flags, messageID)
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
	return nil
}

func (s *Store) UpdateMessagePath(ctx context.Context, messageID, filePath string) error {
	q := s.rebind(`UPDATE messages SET file_path = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, filePath, messageID)
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
	return nil
}

// UpdateMessageArchiveMeta sets path/size/archived and denormalized headers after archive/unarchive.
func (s *Store) UpdateMessageArchiveMeta(ctx context.Context, messageID, filePath string, size int64, archived bool, subject, fromAddr, toAddr, dateHdr string) error {
	arch := 0
	if archived {
		arch = 1
	}
	q := s.rebind(`UPDATE messages SET file_path = ?, size = ?, archived = ?, subject = ?, from_addr = ?, to_addr = ?, date_hdr = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, filePath, size, arch, subject, fromAddr, toAddr, dateHdr, messageID)
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
	return nil
}

// UpdateMessageHeaders fills denormalized header columns.
func (s *Store) UpdateMessageHeaders(ctx context.Context, messageID, subject, fromAddr, toAddr, dateHdr string) error {
	q := s.rebind(`UPDATE messages SET subject = ?, from_addr = ?, to_addr = ?, date_hdr = ? WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, q, subject, fromAddr, toAddr, dateHdr, messageID)
	return err
}

// MoveMessage assigns a new UID in dstMailboxID and updates mailbox_id (FlowSync MoveItems).
func (s *Store) MoveMessage(ctx context.Context, messageID, dstMailboxID string) (*Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	msg, err := s.scanMessage(tx.QueryRowContext(ctx, s.rebind(`SELECT `+msgCols+` FROM messages WHERE id = ?`), messageID))
	if err != nil {
		return nil, err
	}
	if msg.MailboxID == dstMailboxID {
		return msg, nil
	}

	var uid int64
	lockQ := s.rebind(`SELECT uidnext from mailboxes WHERE id = ?`)
	if s.dialect == DialectPostgres {
		lockQ = `SELECT uidnext FROM mailboxes WHERE id = $1 FOR UPDATE`
	}
	if err := tx.QueryRowContext(ctx, lockQ, dstMailboxID).Scan(&uid); err != nil {
		return nil, mapErr(err)
	}
	updMB := s.rebind(`UPDATE mailboxes SET uidnext = ? WHERE id = ?`)
	if _, err := tx.ExecContext(ctx, updMB, uid+1, dstMailboxID); err != nil {
		return nil, err
	}
	updMsg := s.rebind(`UPDATE messages SET mailbox_id = ?, uid = ? WHERE id = ?`)
	if _, err := tx.ExecContext(ctx, updMsg, dstMailboxID, uid, messageID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	msg.MailboxID = dstMailboxID
	msg.UID = uid
	return msg, nil
}

func (s *Store) DeleteMessage(ctx context.Context, messageID string) error {
	_ = s.DeleteMessageSearch(ctx, messageID)
	q := s.rebind(`DELETE FROM messages WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, messageID)
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
	return nil
}

func (s *Store) ExpungeMailbox(ctx context.Context, mailboxID string) ([]*Message, error) {
	msgs, err := s.ListMessages(ctx, mailboxID)
	if err != nil {
		return nil, err
	}
	var deleted []*Message
	for _, m := range msgs {
		if !HasFlag(m.Flags, `\Deleted`) {
			continue
		}
		q := s.rebind(`DELETE FROM messages WHERE id = ?`)
		if _, err := s.db.ExecContext(ctx, q, m.ID); err != nil {
			return deleted, err
		}
		deleted = append(deleted, m)
	}
	return deleted, nil
}

// HasFlag reports whether flags string contains an IMAP flag (case-insensitive).
func HasFlag(flags, want string) bool {
	want = strings.ToLower(want)
	for _, f := range strings.Fields(flags) {
		if strings.ToLower(f) == want {
			return true
		}
	}
	return false
}

// NormalizeFlags joins unique IMAP flags with spaces.
func NormalizeFlags(flags []string) string {
	seen := map[string]struct{}{}
	var out []string
	for _, f := range flags {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		key := strings.ToLower(f)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, f)
	}
	return strings.Join(out, " ")
}

// ParseFlags splits a flags string into a slice.
func ParseFlags(flags string) []string {
	return strings.Fields(flags)
}

// MessageIsRecent approximates IMAP \Recent for listing heuristics.
func MessageIsRecent(m *Message) bool {
	if m == nil {
		return false
	}
	if HasFlag(m.Flags, `\Seen`) {
		return false
	}
	return time.Since(m.InternalDate) < 24*time.Hour && strings.Contains(m.FilePath, "/new/")
}

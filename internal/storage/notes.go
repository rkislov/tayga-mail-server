package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// NoteFolder is a notes collection (group) owned by a user.
type NoteFolder struct {
	ID          string
	UserID      string
	Name        string
	DisplayName string
	ParentID    string
	Position    int
	CTag        string
	CreatedAt   time.Time
}

// NoteItem is a single note document.
type NoteItem struct {
	ID           string
	FolderID     string
	UserID       string
	Title        string
	DocumentJSON string
	BodyHTML     string
	BodyText     string
	Pinned       bool
	Archived     bool
	Color        string
	ETag         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NoteAttachment is a file/drawing linked to a note.
type NoteAttachment struct {
	ID          string
	NoteID      string
	UserID      string
	Filename    string
	ContentType string
	Size        int64
	StoragePath string
	Kind        string // file | drawing | preview
	CreatedAt   time.Time
}

func sanitizeNoteFolderName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '_' || r == '-' || r == '.':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 48 {
		out = out[:48]
		out = strings.Trim(out, "-")
	}
	if out == "" {
		out = "folder"
	}
	return out
}

func (s *Store) EnsureNoteDefaults(ctx context.Context, userID string) error {
	_, err := s.EnsureNoteFolder(ctx, userID, "notes", "Notes")
	return err
}

func (s *Store) EnsureNoteFolder(ctx context.Context, userID, name, displayName string) (*NoteFolder, error) {
	name = sanitizeNoteFolderName(name)
	f, err := s.GetNoteFolderByName(ctx, userID, name)
	if err == nil {
		return f, nil
	}
	if err != ErrNotFound {
		return nil, err
	}
	if displayName == "" {
		displayName = name
	}
	return s.CreateNoteFolder(ctx, &NoteFolder{
		UserID: userID, Name: name, DisplayName: displayName, CTag: newCTag(),
	})
}

func (s *Store) CreateNoteFolder(ctx context.Context, f *NoteFolder) (*NoteFolder, error) {
	now := time.Now().UTC()
	if f.ID == "" {
		f.ID = NewID()
	}
	if f.Name == "" {
		f.Name = sanitizeNoteFolderName(f.DisplayName)
	} else {
		f.Name = sanitizeNoteFolderName(f.Name)
	}
	if f.DisplayName == "" {
		f.DisplayName = f.Name
	}
	if f.CTag == "" {
		f.CTag = newCTag()
	}
	f.CreatedAt = now
	var parent any
	if f.ParentID != "" {
		parent = f.ParentID
	}
	q := s.rebind(`INSERT INTO note_folders (id, user_id, name, display_name, parent_id, position, ctag, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q, f.ID, f.UserID, f.Name, f.DisplayName, parent, f.Position, f.CTag, now)
	if err != nil {
		return nil, mapErr(err)
	}
	return f, nil
}

func (s *Store) ListNoteFolders(ctx context.Context, userID string) ([]*NoteFolder, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, parent_id, position, ctag, created_at
		FROM note_folders WHERE user_id = ? ORDER BY position, display_name`)
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*NoteFolder
	for rows.Next() {
		f, err := s.scanNoteFolder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) GetNoteFolderByName(ctx context.Context, userID, name string) (*NoteFolder, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, parent_id, position, ctag, created_at
		FROM note_folders WHERE user_id = ? AND name = ?`)
	return s.scanNoteFolder(s.db.QueryRowContext(ctx, q, userID, sanitizeNoteFolderName(name)))
}

func (s *Store) GetNoteFolderByID(ctx context.Context, userID, id string) (*NoteFolder, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, parent_id, position, ctag, created_at
		FROM note_folders WHERE user_id = ? AND id = ?`)
	return s.scanNoteFolder(s.db.QueryRowContext(ctx, q, userID, id))
}

func (s *Store) GetNoteFolder(ctx context.Context, id string) (*NoteFolder, error) {
	q := s.rebind(`SELECT id, user_id, name, display_name, parent_id, position, ctag, created_at
		FROM note_folders WHERE id = ?`)
	return s.scanNoteFolder(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) UpdateNoteFolder(ctx context.Context, f *NoteFolder) error {
	q := s.rebind(`UPDATE note_folders SET display_name=?, parent_id=?, position=?, ctag=? WHERE id=? AND user_id=?`)
	var parent any
	if f.ParentID != "" {
		parent = f.ParentID
	}
	if f.CTag == "" {
		f.CTag = newCTag()
	}
	res, err := s.db.ExecContext(ctx, q, f.DisplayName, parent, f.Position, f.CTag, f.ID, f.UserID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteNoteFolder(ctx context.Context, userID, id string) error {
	f, err := s.GetNoteFolderByID(ctx, userID, id)
	if err != nil {
		return err
	}
	if f.Name == "notes" {
		return fmt.Errorf("default notes folder cannot be deleted")
	}
	q := s.rebind(`DELETE FROM note_folders WHERE id = ? AND user_id = ?`)
	res, err := s.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) scanNoteFolder(row interface{ Scan(dest ...any) error }) (*NoteFolder, error) {
	f := &NoteFolder{}
	var parent sql.NullString
	err := row.Scan(&f.ID, &f.UserID, &f.Name, &f.DisplayName, &parent, &f.Position, &f.CTag, &f.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if parent.Valid {
		f.ParentID = parent.String
	}
	return f, nil
}

func (s *Store) CreateNoteItem(ctx context.Context, n *NoteItem) (*NoteItem, error) {
	now := time.Now().UTC()
	if n.ID == "" {
		n.ID = NewID()
	}
	if n.ETag == "" {
		n.ETag = ContentETag(n.DocumentJSON + n.Title)
	}
	n.CreatedAt = now
	n.UpdatedAt = now
	pinned, archived := 0, 0
	if n.Pinned {
		pinned = 1
	}
	if n.Archived {
		archived = 1
	}
	q := s.rebind(`INSERT INTO note_items
		(id, folder_id, user_id, title, document_json, body_html, body_text, pinned, archived, color, etag, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q,
		n.ID, n.FolderID, n.UserID, n.Title, n.DocumentJSON, n.BodyHTML, n.BodyText,
		pinned, archived, n.Color, n.ETag, now, now,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	_ = s.bumpNoteFolderCTag(ctx, n.FolderID)
	return n, nil
}

func (s *Store) UpdateNoteItem(ctx context.Context, n *NoteItem) error {
	now := time.Now().UTC()
	n.UpdatedAt = now
	n.ETag = ContentETag(n.DocumentJSON + n.Title + now.String())
	pinned, archived := 0, 0
	if n.Pinned {
		pinned = 1
	}
	if n.Archived {
		archived = 1
	}
	// Update by id only so collaborators (shared ACL) can save the same note.
	q := s.rebind(`UPDATE note_items SET folder_id=?, title=?, document_json=?, body_html=?, body_text=?,
		pinned=?, archived=?, color=?, etag=?, updated_at=? WHERE id=?`)
	res, err := s.db.ExecContext(ctx, q,
		n.FolderID, n.Title, n.DocumentJSON, n.BodyHTML, n.BodyText,
		pinned, archived, n.Color, n.ETag, now, n.ID,
	)
	if err != nil {
		return err
	}
	nn, _ := res.RowsAffected()
	if nn == 0 {
		return ErrNotFound
	}
	_ = s.bumpNoteFolderCTag(ctx, n.FolderID)
	return nil
}

func (s *Store) GetNoteItem(ctx context.Context, userID, id string) (*NoteItem, error) {
	q := s.rebind(`SELECT id, folder_id, user_id, title, document_json, body_html, body_text, pinned, archived, color, etag, created_at, updated_at
		FROM note_items WHERE id = ? AND user_id = ?`)
	return s.scanNoteItem(s.db.QueryRowContext(ctx, q, id, userID))
}

func (s *Store) GetNoteItemByID(ctx context.Context, id string) (*NoteItem, error) {
	q := s.rebind(`SELECT id, folder_id, user_id, title, document_json, body_html, body_text, pinned, archived, color, etag, created_at, updated_at
		FROM note_items WHERE id = ?`)
	return s.scanNoteItem(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) ListNoteItems(ctx context.Context, userID, folderID string, includeArchived bool) ([]*NoteItem, error) {
	q := `SELECT id, folder_id, user_id, title, document_json, body_html, body_text, pinned, archived, color, etag, created_at, updated_at
		FROM note_items WHERE user_id = ?`
	args := []any{userID}
	if folderID != "" {
		q += ` AND folder_id = ?`
		args = append(args, folderID)
	}
	if !includeArchived {
		q += ` AND archived = 0`
	}
	q += ` ORDER BY pinned DESC, updated_at DESC`
	rows, err := s.db.QueryContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*NoteItem
	for rows.Next() {
		n, err := s.scanNoteItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) DeleteNoteItem(ctx context.Context, userID, id string) error {
	n, err := s.GetNoteItem(ctx, userID, id)
	if err != nil {
		// Allow delete when caller is a collaborator with write (checked by API).
		n, err = s.GetNoteItemByID(ctx, id)
		if err != nil {
			return err
		}
	}
	return s.DeleteNoteItemByID(ctx, n.ID)
}

func (s *Store) DeleteNoteItemByID(ctx context.Context, id string) error {
	n, err := s.GetNoteItemByID(ctx, id)
	if err != nil {
		return err
	}
	q := s.rebind(`DELETE FROM note_items WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return err
	}
	nn, _ := res.RowsAffected()
	if nn == 0 {
		return ErrNotFound
	}
	_ = s.bumpNoteFolderCTag(ctx, n.FolderID)
	return nil
}

func (s *Store) scanNoteItem(row interface{ Scan(dest ...any) error }) (*NoteItem, error) {
	n := &NoteItem{}
	var pinned, archived any
	err := row.Scan(
		&n.ID, &n.FolderID, &n.UserID, &n.Title, &n.DocumentJSON, &n.BodyHTML, &n.BodyText,
		&pinned, &archived, &n.Color, &n.ETag, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	n.Pinned = asBool(pinned)
	n.Archived = asBool(archived)
	return n, nil
}

func (s *Store) bumpNoteFolderCTag(ctx context.Context, folderID string) error {
	q := s.rebind(`UPDATE note_folders SET ctag = ? WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, q, newCTag(), folderID)
	return err
}

func (s *Store) CreateNoteAttachment(ctx context.Context, a *NoteAttachment) (*NoteAttachment, error) {
	now := time.Now().UTC()
	if a.ID == "" {
		a.ID = NewID()
	}
	a.CreatedAt = now
	a.Filename = filepath.Base(strings.TrimSpace(a.Filename))
	if a.Filename == "" || a.Filename == "." {
		a.Filename = "file"
	}
	if a.ContentType == "" {
		a.ContentType = "application/octet-stream"
	}
	if a.Kind == "" {
		a.Kind = "file"
	}
	q := s.rebind(`INSERT INTO note_attachments
		(id, note_id, user_id, filename, content_type, size, storage_path, kind, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q,
		a.ID, a.NoteID, a.UserID, a.Filename, a.ContentType, a.Size, a.StoragePath, a.Kind, now,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	return a, nil
}

func (s *Store) GetNoteAttachment(ctx context.Context, id string) (*NoteAttachment, error) {
	q := s.rebind(`SELECT id, note_id, user_id, filename, content_type, size, storage_path, kind, created_at
		FROM note_attachments WHERE id = ?`)
	return s.scanNoteAttachment(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) ListNoteAttachments(ctx context.Context, noteID string) ([]*NoteAttachment, error) {
	q := s.rebind(`SELECT id, note_id, user_id, filename, content_type, size, storage_path, kind, created_at
		FROM note_attachments WHERE note_id = ? ORDER BY created_at`)
	rows, err := s.db.QueryContext(ctx, q, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*NoteAttachment
	for rows.Next() {
		a, err := s.scanNoteAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) DeleteNoteAttachment(ctx context.Context, id string) error {
	q := s.rebind(`DELETE FROM note_attachments WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) scanNoteAttachment(row interface{ Scan(dest ...any) error }) (*NoteAttachment, error) {
	a := &NoteAttachment{}
	err := row.Scan(&a.ID, &a.NoteID, &a.UserID, &a.Filename, &a.ContentType, &a.Size, &a.StoragePath, &a.Kind, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

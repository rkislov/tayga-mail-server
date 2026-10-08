package storage

import (
	"context"
	"strings"
)

// NoteACLEntry is a per-note share grant.
type NoteACLEntry struct {
	NoteID        string
	GranteeUserID string
	Rights        string // read | write
}

// NoteFolderACLEntry is a per-folder share grant (applies to all notes in the folder).
type NoteFolderACLEntry struct {
	FolderID      string
	GranteeUserID string
	Rights        string // read | write
}

func normalizeNoteRights(rights string) string {
	rights = strings.ToLower(strings.TrimSpace(rights))
	switch rights {
	case "write", "read":
		return rights
	case "rw", "edit", "owner":
		return "write"
	case "ro", "view":
		return "read"
	default:
		if rights == "" {
			return "write"
		}
		return "read"
	}
}

func noteRightsRank(r string) int {
	switch normalizeNoteRights(r) {
	case "write":
		return 2
	case "read":
		return 1
	default:
		return 0
	}
}

func maxNoteRights(a, b string) string {
	if noteRightsRank(a) >= noteRightsRank(b) {
		return normalizeNoteRights(a)
	}
	return normalizeNoteRights(b)
}

func (s *Store) SetNoteFolderACL(ctx context.Context, folderID, granteeUserID, rights string) error {
	rights = normalizeNoteRights(rights)
	if rights == "" {
		return s.DeleteNoteFolderACL(ctx, folderID, granteeUserID)
	}
	q := s.rebind(`INSERT INTO note_folder_acl(folder_id, grantee_user_id, rights) VALUES (?, ?, ?)
		ON CONFLICT(folder_id, grantee_user_id) DO UPDATE SET rights = excluded.rights`)
	_, err := s.db.ExecContext(ctx, q, folderID, granteeUserID, rights)
	return mapErr(err)
}

func (s *Store) DeleteNoteFolderACL(ctx context.Context, folderID, granteeUserID string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM note_folder_acl WHERE folder_id = ? AND grantee_user_id = ?`), folderID, granteeUserID)
	return err
}

func (s *Store) ListNoteFolderACL(ctx context.Context, folderID string) ([]*NoteFolderACLEntry, error) {
	q := s.rebind(`SELECT folder_id, grantee_user_id, rights FROM note_folder_acl WHERE folder_id = ?`)
	rows, err := s.db.QueryContext(ctx, q, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*NoteFolderACLEntry
	for rows.Next() {
		e := &NoteFolderACLEntry{}
		if err := rows.Scan(&e.FolderID, &e.GranteeUserID, &e.Rights); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) SetNoteACL(ctx context.Context, noteID, granteeUserID, rights string) error {
	rights = normalizeNoteRights(rights)
	if rights == "" {
		return s.DeleteNoteACL(ctx, noteID, granteeUserID)
	}
	q := s.rebind(`INSERT INTO note_acl(note_id, grantee_user_id, rights) VALUES (?, ?, ?)
		ON CONFLICT(note_id, grantee_user_id) DO UPDATE SET rights = excluded.rights`)
	_, err := s.db.ExecContext(ctx, q, noteID, granteeUserID, rights)
	return mapErr(err)
}

func (s *Store) DeleteNoteACL(ctx context.Context, noteID, granteeUserID string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM note_acl WHERE note_id = ? AND grantee_user_id = ?`), noteID, granteeUserID)
	return err
}

func (s *Store) ListNoteACL(ctx context.Context, noteID string) ([]*NoteACLEntry, error) {
	q := s.rebind(`SELECT note_id, grantee_user_id, rights FROM note_acl WHERE note_id = ?`)
	rows, err := s.db.QueryContext(ctx, q, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*NoteACLEntry
	for rows.Next() {
		e := &NoteACLEntry{}
		if err := rows.Scan(&e.NoteID, &e.GranteeUserID, &e.Rights); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// NoteFolderRightsForUser returns read|write for the folder, or ErrNotFound if no access.
func (s *Store) NoteFolderRightsForUser(ctx context.Context, folderID, userID string) (string, error) {
	f, err := s.GetNoteFolder(ctx, folderID)
	if err != nil {
		return "", err
	}
	if f.UserID == userID {
		return "write", nil
	}
	var rights string
	err = s.db.QueryRowContext(ctx, s.rebind(`SELECT rights FROM note_folder_acl WHERE folder_id = ? AND grantee_user_id = ?`), folderID, userID).Scan(&rights)
	if err != nil {
		return "", mapErr(err)
	}
	return normalizeNoteRights(rights), nil
}

// NoteRightsForUser returns read|write for a note (owner, note ACL, or folder ACL).
func (s *Store) NoteRightsForUser(ctx context.Context, noteID, userID string) (string, error) {
	n, err := s.GetNoteItemByID(ctx, noteID)
	if err != nil {
		return "", err
	}
	if n.UserID == userID {
		return "write", nil
	}
	best := ""
	var noteRights string
	err = s.db.QueryRowContext(ctx, s.rebind(`SELECT rights FROM note_acl WHERE note_id = ? AND grantee_user_id = ?`), noteID, userID).Scan(&noteRights)
	if err == nil {
		best = normalizeNoteRights(noteRights)
	}
	if fr, err := s.NoteFolderRightsForUser(ctx, n.FolderID, userID); err == nil {
		best = maxNoteRights(best, fr)
	}
	if best == "" {
		return "", ErrNotFound
	}
	return best, nil
}

// ListNoteFoldersForUser returns owned folders plus folders shared via note_folder_acl.
func (s *Store) ListNoteFoldersForUser(ctx context.Context, userID string) ([]*NoteFolder, error) {
	owned, err := s.ListNoteFolders(ctx, userID)
	if err != nil {
		return nil, err
	}
	shared, err := s.ListSharedNoteFolders(ctx, userID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(owned)+len(shared))
	out := make([]*NoteFolder, 0, len(owned)+len(shared))
	for _, f := range owned {
		seen[f.ID] = struct{}{}
		out = append(out, f)
	}
	for _, f := range shared {
		if _, ok := seen[f.ID]; ok {
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

func (s *Store) ListSharedNoteFolders(ctx context.Context, userID string) ([]*NoteFolder, error) {
	q := s.rebind(`SELECT f.id, f.user_id, f.name, f.display_name, f.parent_id, f.position, f.ctag, f.created_at
		FROM note_folders f
		INNER JOIN note_folder_acl a ON a.folder_id = f.id
		WHERE a.grantee_user_id = ?
		ORDER BY f.display_name`)
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

// ListSharedNoteItems returns notes shared via note_acl (not covered by folder ownership).
func (s *Store) ListSharedNoteItems(ctx context.Context, userID string) ([]*NoteItem, error) {
	q := s.rebind(`SELECT n.id, n.folder_id, n.user_id, n.title, n.document_json, n.body_html, n.body_text,
		n.pinned, n.archived, n.color, n.etag, n.created_at, n.updated_at
		FROM note_items n
		INNER JOIN note_acl a ON a.note_id = n.id
		WHERE a.grantee_user_id = ? AND n.user_id != ?
		ORDER BY n.pinned DESC, n.updated_at DESC`)
	rows, err := s.db.QueryContext(ctx, q, userID, userID)
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

// ListNoteItemsInFolder lists all notes in a folder (for shared notebook access).
func (s *Store) ListNoteItemsInFolder(ctx context.Context, folderID string, includeArchived bool) ([]*NoteItem, error) {
	q := `SELECT id, folder_id, user_id, title, document_json, body_html, body_text, pinned, archived, color, etag, created_at, updated_at
		FROM note_items WHERE folder_id = ?`
	args := []any{folderID}
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

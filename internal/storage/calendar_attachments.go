package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"time"
)

// CalendarAttachment is a file linked to a calendar event object.
type CalendarAttachment struct {
	ID               string
	CalendarObjectID string
	UserID           string
	Filename         string
	ContentType      string
	Size             int64
	StoragePath      string // relative to mailstore calendar-attachments root
	CreatedAt        time.Time
}

type calendarAttachmentChanges struct {
	add, remove []*CalendarAttachment
}

// SaveCalendarClientAttachments commits staged file records with the event,
// recipient copies and scheduling state. The caller owns blob staging/cleanup.
func (s *Store) SaveCalendarClientAttachments(ctx context.Context, actorID string, o *CalendarObject, files []*CalendarAttachment) (*CalendarObject, error) {
	if actorID == "" {
		return nil, ErrUnauthorized
	}
	if o.CreateOnly || o.ExpectedETag == "" || len(files) == 0 {
		return nil, ErrCalendarConflict
	}
	return s.saveCalendarObject(ctx, o, nil, actorID, true, calendarAttachmentChanges{add: files})
}

// DeleteCalendarClientAttachments revokes records in the same transaction as
// the updated event and recipient copies. Blobs may be removed after commit.
func (s *Store) DeleteCalendarClientAttachments(ctx context.Context, actorID string, o *CalendarObject, files []*CalendarAttachment) (*CalendarObject, error) {
	if actorID == "" {
		return nil, ErrUnauthorized
	}
	if o.CreateOnly || o.ExpectedETag == "" || len(files) == 0 {
		return nil, ErrCalendarConflict
	}
	return s.saveCalendarObject(ctx, o, nil, actorID, true, calendarAttachmentChanges{remove: files})
}

func (s *Store) CreateCalendarAttachment(ctx context.Context, a *CalendarAttachment) (*CalendarAttachment, error) {
	now := time.Now().UTC()
	if a.ID == "" {
		a.ID = NewID()
	}
	a.CreatedAt = now
	a.Filename = filepath.Base(strings.TrimSpace(a.Filename))
	if a.Filename == "" || a.Filename == "." || a.Filename == "/" {
		a.Filename = "file"
	}
	if a.ContentType == "" {
		a.ContentType = "application/octet-stream"
	}
	q := s.rebind(`INSERT INTO calendar_attachments
		(id, calendar_object_id, user_id, filename, content_type, size, storage_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q,
		a.ID, a.CalendarObjectID, a.UserID, a.Filename, a.ContentType, a.Size, a.StoragePath, a.CreatedAt,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	return a, nil
}

func (s *Store) GetCalendarAttachment(ctx context.Context, id string) (*CalendarAttachment, error) {
	q := s.rebind(`SELECT id, calendar_object_id, user_id, filename, content_type, size, storage_path, created_at
		FROM calendar_attachments WHERE id = ?`)
	return s.scanCalendarAttachment(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) ListCalendarAttachments(ctx context.Context, calendarObjectID string) ([]*CalendarAttachment, error) {
	q := s.rebind(`SELECT id, calendar_object_id, user_id, filename, content_type, size, storage_path, created_at
		FROM calendar_attachments WHERE calendar_object_id = ? ORDER BY created_at, filename`)
	rows, err := s.db.QueryContext(ctx, q, calendarObjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CalendarAttachment
	for rows.Next() {
		a, err := s.scanCalendarAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) DeleteCalendarAttachment(ctx context.Context, id string) error {
	q := s.rebind(`DELETE FROM calendar_attachments WHERE id = ?`)
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

func (s *Store) scanCalendarAttachment(row interface{ Scan(dest ...any) error }) (*CalendarAttachment, error) {
	a := &CalendarAttachment{}
	err := row.Scan(&a.ID, &a.CalendarObjectID, &a.UserID, &a.Filename, &a.ContentType, &a.Size, &a.StoragePath, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

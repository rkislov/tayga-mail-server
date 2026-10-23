package storage

import (
	"context"
	"time"
)

// MoveCalendarObject changes collection membership, not the meeting identity.
// Files, travel links, invitations and recurrence remain attached to the same
// object. A move alone must not schedule a new invitation or cancellation.
func (s *Store) MoveCalendarObject(ctx context.Context, userID, id, source, destination string) (*CalendarObject, error) {
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, calendarID := range []string{source, destination} {
		var count int
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM calendars WHERE id=? AND user_id=?`), calendarID, userID).Scan(&count); err != nil {
			return nil, err
		}
		if count != 1 {
			return nil, ErrUnauthorized
		}
	}
	o, err := s.scanCalObject(tx.QueryRowContext(ctx, s.rebind(`SELECT id,calendar_id,uid,href_name,etag,data,size,component,dtstart,dtend,updated_at FROM calendar_objects WHERE id=?`), id))
	if err != nil {
		return nil, err
	}
	if o.CalendarID != source && o.CalendarID != destination {
		return nil, ErrNotFound
	}
	if o.CalendarID == destination {
		return o, tx.Commit()
	} // lost response / retry
	var conflicts int
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM calendar_objects WHERE calendar_id=? AND (href_name=? OR uid=?)`), destination, o.HrefName, o.UID).Scan(&conflicts); err != nil {
		return nil, err
	}
	if conflicts != 0 {
		return nil, ErrCalendarConflict
	}
	o.CalendarID, o.UpdatedAt = destination, time.Now().UTC()
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendar_objects SET calendar_id=?,updated_at=? WHERE id=?`), destination, o.UpdatedAt, id); err != nil {
		return nil, err
	}
	for _, calendarID := range []string{source, destination} {
		if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendars SET ctag=? WHERE id=?`), newCTag(), calendarID); err != nil {
			return nil, err
		}
	}
	return o, tx.Commit()
}

// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"database/sql"
	"time"
)

type calDAVSchedulingKey struct{}
type calDAVSchedulingOptions struct{ Reply bool }

func WithCalDAVScheduling(ctx context.Context, reply bool) context.Context {
	return context.WithValue(ctx, calDAVSchedulingKey{}, calDAVSchedulingOptions{reply})
}

type CalendarSchedulingMessage struct {
	ID, Data, ETag string
	CreatedAt      time.Time
}

func (s *Store) ListCalendarSchedulingInbox(ctx context.Context, userID string) ([]CalendarSchedulingMessage, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id,data,etag,created_at FROM calendar_scheduling_inbox WHERE user_id=? ORDER BY created_at,id`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CalendarSchedulingMessage
	for rows.Next() {
		var m CalendarSchedulingMessage
		if err = rows.Scan(&m.ID, &m.Data, &m.ETag, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) DeleteCalendarSchedulingMessage(ctx context.Context, userID, id, etag string) error {
	q := `DELETE FROM calendar_scheduling_inbox WHERE user_id=? AND id=?`
	args := []any{userID, id}
	if etag != "" {
		q += ` AND etag=?`
		args = append(args, etag)
	}
	result, err := s.db.ExecContext(ctx, s.rebind(q), args...)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCalendarConflict
	}
	return nil
}
func (s *Store) bumpCalendarScheduleTagTx(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	tag := `"` + NewID() + `"`
	_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO calendar_schedule_tags(object_id,tag) VALUES(?,?) ON CONFLICT(object_id) DO UPDATE SET tag=excluded.tag`), id, tag)
	return tag, err
}
func (s *Store) calendarSchedulePreconditionTx(ctx context.Context, tx *sql.Tx, o *CalendarObject, currentID, currentETag string) error {
	if o.ExpectedScheduleTag == "" {
		return nil
	}
	if currentID == "" {
		return ErrCalendarConflict
	}
	var tag string
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT tag FROM calendar_schedule_tags WHERE object_id=?`), currentID).Scan(&tag); err != nil {
		return ErrCalendarConflict
	}
	if tag != o.ExpectedScheduleTag {
		return ErrCalendarConflict
	}
	if o.StrictETag && o.ExpectedETag != "" && o.ExpectedETag != currentETag {
		return ErrCalendarConflict
	}
	o.ExpectedETag = currentETag
	return nil
}

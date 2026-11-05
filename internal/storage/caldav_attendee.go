// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"database/sql"
	"github.com/tayga/tms/internal/calutil"
	"strings"
)

func (s *Store) calDAVInvitesTx(ctx context.Context, tx *sql.Tx, userID, objectID string) (map[string]*CalendarInvite, error) {
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT id,organizer_user_id,attendee_email,attendee_user_id,event_uid,calendar_object_id,summary,partstat,proposed_start,proposed_end,created_at,updated_at FROM calendar_invites WHERE attendee_user_id=? AND calendar_object_id=?`), userID, objectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*CalendarInvite{}
	for rows.Next() {
		inv, err := s.scanCalendarInvite(rows)
		if err != nil {
			return nil, err
		}
		out[strings.ToLower(inv.AttendeeEmail)] = inv
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if len(out) > 0 {
		var data string
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT data FROM calendar_objects WHERE id=?`), objectID).Scan(&data); err != nil {
			return nil, err
		}
		attendees, err := calutil.EventAttendees(data)
		if err != nil {
			return nil, err
		}
		active := map[string]bool{}
		for _, a := range attendees {
			active[strings.ToLower(a.Email)] = true
		}
		for email := range out {
			if !active[email] {
				delete(out, email)
			}
		}
	}
	return out, nil
}
func (s *Store) saveCalDAVAttendeeTx(ctx context.Context, tx *sql.Tx, userID string, o *CalendarObject, previous string, send bool) (*CalendarObject, error) {
	current, err := s.scanCalObject(tx.QueryRowContext(ctx, s.rebind(`SELECT id,calendar_id,uid,href_name,etag,data,size,component,dtstart,dtend,updated_at FROM calendar_objects WHERE calendar_id=? AND href_name=?`), o.CalendarID, o.HrefName))
	if err != nil {
		return nil, err
	}
	if o.ExpectedETag != "" && o.ExpectedETag != current.ETag {
		return nil, ErrCalendarConflict
	}
	invites, err := s.calDAVInvitesTx(ctx, tx, userID, current.ID)
	if err != nil {
		return nil, err
	}
	if len(invites) == 0 {
		return nil, ErrUnauthorized
	}
	var emails []string
	for email := range invites {
		emails = append(emails, email)
	}
	changes, err := calutil.CalDAVAttendeeResponses(o.Data, previous, emails)
	if err != nil {
		return nil, ErrUnauthorized
	}
	parsed, err := calutil.ParseICS(previous)
	if err != nil {
		return nil, err
	}
	serverAgent := parsed.Organizer.ScheduleAgent == "" || strings.EqualFold(parsed.Organizer.ScheduleAgent, "SERVER")
	for _, change := range changes {
		if !serverAgent {
			if change.RecurrenceID == "" {
				current.Data, err = calutil.WithAttendeeStatus(current.Data, change.Email, change.Status)
			} else {
				current.Data, err = calutil.WithOccurrenceAttendeeStatus(current.Data, change.RecurrenceID, change.Email, change.Status)
			}
			if err != nil {
				return nil, err
			}
			continue
		}
		var etag string
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT etag FROM calendar_objects WHERE id=?`), current.ID).Scan(&etag); err != nil {
			return nil, err
		}
		inv := invites[strings.ToLower(change.Email)]
		if _, err = s.replyCalendarInviteTx(ctx, tx, userID, inv.ID, change.Status, nil, nil, etag, change.RecurrenceID, send, true, ""); err != nil {
			return nil, err
		}
	}
	if serverAgent {
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT data,etag FROM calendar_objects WHERE id=?`), current.ID).Scan(&current.Data, &current.ETag); err != nil {
			return nil, err
		}
	}
	current.Data, err = calutil.MergeCalDAVPersonal(current.Data, o.Data)
	if err != nil {
		return nil, err
	}
	current.ExpectedETag = current.ETag
	if err = s.saveCalendarObjectTx(ctx, tx, current); err != nil {
		return nil, err
	}
	return current, nil
}

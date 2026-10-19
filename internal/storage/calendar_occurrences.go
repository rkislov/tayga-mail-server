package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

// ChangeCalendarOccurrence commits the organizer exception and existing local
// participant/resource copies together. Every changed row uses optimistic
// concurrency; a stale participant copy rolls the whole operation back.
func (s *Store) ChangeCalendarOccurrence(ctx context.Context, userID, objectID, etag, rid string, change calutil.OccurrenceChange) (*CalendarObject, error) {
	if etag == "" || rid == "" {
		return nil, ErrCalendarConflict
	}
	return s.changeCalendarSeries(ctx, userID, objectID, etag, rid, change, nil)
}

// ChangeCalendarRecurrence commits the rule, existing copies, RSVP reset and
// mail jobs together. Participant reminders are kept in their own copies.
func (s *Store) ChangeCalendarRecurrence(ctx context.Context, userID, objectID, etag string, rule calutil.RecurrenceRule) (*CalendarObject, error) {
	if etag == "" {
		return nil, ErrCalendarConflict
	}
	return s.changeCalendarSeries(ctx, userID, objectID, etag, "", calutil.OccurrenceChange{}, &rule)
}

func (s *Store) changeCalendarSeries(ctx context.Context, userID, objectID, etag, rid string, change calutil.OccurrenceChange, rule *calutil.RecurrenceRule) (*CalendarObject, error) {
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var email, name string
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT email, display_name FROM users WHERE id=? AND enabled=TRUE`), userID).Scan(&email, &name); err != nil {
		return nil, ErrUnauthorized
	}
	q := `SELECT o.id, o.calendar_id, o.uid, o.href_name, o.etag, o.data, o.size, o.component, o.dtstart, o.dtend, o.updated_at
	 FROM calendar_objects o JOIN calendars c ON c.id=o.calendar_id WHERE o.id=? AND
	 (c.user_id=? OR (c.name<>'personal' AND EXISTS (SELECT 1 FROM calendar_acl a WHERE a.calendar_id=c.id AND a.grantee_user_id=? AND a.rights='write')))`
	source, err := s.scanCalObject(tx.QueryRowContext(ctx, s.rebind(q), objectID, userID, userID))
	if err != nil {
		return nil, err
	}
	p, err := calutil.ParseICS(source.Data)
	if err != nil {
		return nil, err
	}
	if p.Organizer.Email != "" && !strings.EqualFold(p.Organizer.Email, email) {
		return nil, ErrUnauthorized
	}
	if source.ETag != etag || strings.EqualFold(p.Status, "CANCELLED") {
		return nil, ErrCalendarConflict
	}
	var data, wire string
	var instance calutil.Occurrence
	if rule == nil {
		data, wire, instance, err = calutil.ChangeOccurrence(source.Data, rid, change)
	} else {
		data, err = calutil.WithRecurrence(source.Data, *rule, calutil.NextSequence(source.Data))
		if err == nil {
			instance.ParseResult, err = calutil.ParseICS(data)
		}
	}
	if err != nil {
		return nil, err
	}
	// Fixed row ordering also keeps concurrent instance changes from taking
	// source/copy locks in opposite orders on PostgreSQL.
	q = `SELECT o.id, o.calendar_id, o.uid, o.href_name, o.etag, o.data, o.size, o.component, o.dtstart, o.dtend, o.updated_at
	 FROM calendar_objects o JOIN calendars c ON c.id=o.calendar_id WHERE o.id=? OR
	 (o.uid=? AND NOT EXISTS (SELECT 1 FROM calendar_resources r WHERE r.user_id=c.user_id) AND EXISTS (SELECT 1 FROM calendar_invites i WHERE i.event_uid=o.uid AND i.organizer_user_id=? AND i.attendee_user_id=c.user_id AND LOWER(i.attendee_email) IN (%s))) ORDER BY o.id`
	args := []any{objectID, source.UID, userID}
	placeholders := []string{}
	for _, a := range instance.Attendees {
		placeholders = append(placeholders, "?")
		args = append(args, strings.ToLower(a.Email))
	}
	if len(placeholders) == 0 {
		placeholders = append(placeholders, "?")
		args = append(args, "")
	}
	q = fmt.Sprintf(q, strings.Join(placeholders, ","))
	rows, err := tx.QueryContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	var objects []*CalendarObject
	for rows.Next() {
		o, err := s.scanCalObject(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		objects = append(objects, o)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	foundSource := false
	for _, o := range objects {
		updated := data
		if o.ID != objectID {
			copy, err := calutil.ParseICS(o.Data)
			if err != nil {
				return nil, err
			}
			if !strings.EqualFold(copy.Organizer.Email, email) || strings.EqualFold(copy.Status, "CANCELLED") {
				continue
			}
			if rule != nil && calutil.NextSequence(o.Data) > instance.Sequence {
				return nil, ErrCalendarConflict
			}
			if rule == nil {
				updated, err = calutil.MergeOccurrence(o.Data, wire)
			} else {
				updated, err = calutil.WithRecurrence(o.Data, *rule, instance.Sequence)
			}
			if err != nil {
				return nil, err
			}
		} else {
			foundSource = true
			if o.ETag != etag {
				return nil, ErrCalendarConflict
			}
		}
		result, err := tx.ExecContext(ctx, s.rebind(`UPDATE calendar_objects SET data=?, etag=?, size=?, updated_at=? WHERE id=? AND etag=?`), updated, ContentETag(updated), len(updated), now, o.ID, o.ETag)
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n != 1 {
			return nil, ErrCalendarConflict
		}
		if _, err := tx.ExecContext(ctx, s.rebind(`UPDATE calendars SET ctag=? WHERE id=?`), newCTag(), o.CalendarID); err != nil {
			return nil, err
		}
	}
	if !foundSource {
		return nil, ErrCalendarConflict
	}
	// Resource validation and copies belong to this same transaction.
	previous := source.Data
	source.Data = data
	if err = s.bookCalendarResourcesTx(ctx, tx, source, previous); err != nil {
		return nil, err
	}
	data = source.Data
	if rule == nil {
		instance, err = calutil.GetOccurrence(data, rid)
	} else {
		instance.ParseResult, err = calutil.ParseICS(data)
	}
	if err != nil {
		return nil, err
	}
	statuses := map[string]string{}
	for _, a := range instance.Attendees {
		if a.CUType == "RESOURCE" {
			statuses[strings.ToLower(a.Email)] = a.PartStat
		}
	}
	// Keep accepted resource responses visible in participant copies as well.
	if len(statuses) > 0 {
		for _, o := range objects {
			if o.ID == objectID {
				continue
			}
			var current string
			err = tx.QueryRowContext(ctx, s.rebind(`SELECT data FROM calendar_objects WHERE id=?`), o.ID).Scan(&current)
			if err != nil {
				return nil, err
			}
			cp, e := calutil.ParseICS(current)
			if e != nil || !strings.EqualFold(cp.Organizer.Email, email) || strings.EqualFold(cp.Status, "CANCELLED") {
				continue
			}
			updated, e := calutil.WithResourcePartStats(current, statuses)
			if e != nil {
				return nil, e
			}
			if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendar_objects SET data=?,etag=?,size=? WHERE id=?`), updated, ContentETag(updated), len(updated), o.ID); err != nil {
				return nil, err
			}
		}
	}
	var mails []CalendarMail
	if rule == nil {
		method, subject, text := "REQUEST", "Updated occurrence: "+instance.Summary, "One occurrence of this meeting has changed.\n"+calutil.FormatWhen(instance.Start, instance.End)
		if change.Cancel {
			method, subject, text = "CANCEL", "Cancelled occurrence: "+instance.Summary, "One occurrence of this meeting has been cancelled."
		}
		var ics string
		ics, err = calutil.OccurrenceMessage(instance, change.Cancel)
		mails = calendarAttendeeMail(email, name, instance.Attendees, subject, text, ics, method)
	} else {
		mails, err = CalendarEventMail(email, name, instance.Event, false)
		if err == nil {
			for _, a := range instance.Attendees {
				_, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendar_invites SET partstat='NEEDS-ACTION', proposed_start=NULL, proposed_end=NULL, updated_at=? WHERE event_uid=? AND organizer_user_id=? AND LOWER(attendee_email)=? AND NOT EXISTS (SELECT 1 FROM calendar_resources r WHERE r.user_id=calendar_invites.attendee_user_id)`), now, source.UID, userID, strings.ToLower(a.Email))
				if err != nil {
					break
				}
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if err = s.queueCalendarMail(ctx, tx, mails); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	source.Data, source.ETag, source.Size, source.UpdatedAt = data, ContentETag(data), int64(len(data)), now
	return source, nil
}

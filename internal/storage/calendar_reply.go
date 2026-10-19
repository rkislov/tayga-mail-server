package storage

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

var ErrCalendarConflict = errors.New("calendar changed; refresh before retrying")

// ReplyCalendarInvite commits participation and both local calendar copies
// together. A COUNTER stores a proposal; only the organizer can move a meeting.
func (s *Store) ReplyCalendarInvite(ctx context.Context, userID, inviteID, status string, proposedStart, proposedEnd *time.Time, etag string, comment ...string) (*CalendarInvite, error) {
	if status != "ACCEPTED" && status != "DECLINED" && status != "TENTATIVE" {
		return nil, ErrUnauthorized
	}
	if (proposedStart == nil) != (proposedEnd == nil) || proposedStart != nil && (!proposedEnd.After(*proposedStart) || status != "TENTATIVE") {
		return nil, ErrUnauthorized
	}
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	q := `SELECT id, organizer_user_id, attendee_email, attendee_user_id, event_uid, calendar_object_id,
		summary, partstat, proposed_start, proposed_end, created_at, updated_at FROM calendar_invites WHERE id=?`
	if s.dialect == DialectPostgres {
		q += ` FOR UPDATE`
	}
	inv, err := s.scanCalendarInvite(tx.QueryRowContext(ctx, s.rebind(q), inviteID))
	if err != nil {
		return nil, err
	}
	if inv.AttendeeUserID != userID {
		return nil, ErrUnauthorized
	}
	var organizerEmail string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT email FROM users WHERE id=? AND enabled=TRUE`), inv.OrganizerUserID).Scan(&organizerEmail); err != nil {
		return nil, ErrNotFound
	}
	q = `SELECT o.id, o.calendar_id, o.uid, o.href_name, o.etag, o.data, o.size, o.component, o.dtstart, o.dtend, o.updated_at
		FROM calendar_objects o JOIN calendars c ON c.id=o.calendar_id
		WHERE o.uid=? AND (c.user_id=? OR (c.user_id=? AND o.id=?) OR
		(c.name <> 'personal' AND EXISTS (SELECT 1 FROM calendar_acl a WHERE a.calendar_id=c.id AND a.grantee_user_id=? AND a.rights='write'))) ORDER BY o.id`
	if s.dialect == DialectPostgres {
		q += ` FOR UPDATE OF o`
	}
	rows, err := tx.QueryContext(ctx, s.rebind(q), inv.EventUID, inv.OrganizerUserID, userID, inv.CalendarObjectID, inv.OrganizerUserID)
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
	var reference *calutil.ParseResult
	for _, o := range objects {
		if o.ID == inv.CalendarObjectID {
			if etag != "" && etag != o.ETag {
				return nil, ErrCalendarConflict
			}
			parsed, err := calutil.ParseICS(o.Data)
			if err != nil {
				return nil, err
			}
			reference = &parsed
		}
	}
	if reference == nil || strings.EqualFold(reference.Status, "CANCELLED") || !strings.EqualFold(reference.Organizer.Email, organizerEmail) {
		return nil, ErrNotFound
	}
	foundOrganizer := false
	now := time.Now().UTC()
	for _, o := range objects {
		parsed, err := calutil.ParseICS(o.Data)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(parsed.Organizer.Email, organizerEmail) {
			continue
		}
		if parsed.Sequence != reference.Sequence || strings.EqualFold(parsed.Status, "CANCELLED") {
			return nil, ErrCalendarConflict
		}
		data, err := calutil.WithAttendeeStatus(o.Data, inv.AttendeeEmail, status)
		if err != nil {
			return nil, ErrCalendarConflict
		}
		if o.ID != inv.CalendarObjectID {
			foundOrganizer = true
		}
		if proposedStart != nil {
			continue
		}
		// The predicate protects against an edit between the read and write on
		// SQLite too; PostgreSQL also locks these rows above.
		result, err := tx.ExecContext(ctx, s.rebind(`UPDATE calendar_objects SET data=?, etag=?, size=?, updated_at=? WHERE id=? AND etag=?`), data, ContentETag(data), len(data), now, o.ID, o.ETag)
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
		if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendars SET ctag=? WHERE id=?`), newCTag(), o.CalendarID); err != nil {
			return nil, err
		}
	}
	if !foundOrganizer {
		return nil, ErrNotFound
	}
	var attendeeName string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT display_name FROM users WHERE id=?`), userID).Scan(&attendeeName); err != nil {
		return nil, err
	}
	ev := reference.Event
	if len(comment) > 0 {
		ev.Comment = comment[0]
	}
	responder := calutil.Attendee{Email: inv.AttendeeEmail, Name: attendeeName, PartStat: status, Role: "REQ-PARTICIPANT"}
	method := "REPLY"
	var ics string
	if proposedStart != nil {
		method = "COUNTER"
		ev.Start, ev.End = proposedStart, proposedEnd
		ics, err = calutil.BuildCounterICS(ev, responder)
	} else {
		ics, err = calutil.BuildReplyICS(ev, responder)
	}
	if err != nil {
		return nil, err
	}
	m := CalendarMail{From: inv.AttendeeEmail, FromName: attendeeName, To: organizerEmail, Subject: "Re: " + inv.Summary + " (" + strings.ToLower(status) + ")", Text: inv.AttendeeEmail + " responded: " + status + "\n" + ev.Comment, ICS: ics, Method: method}
	if err = s.queueCalendarMail(ctx, tx, []CalendarMail{m}); err != nil {
		return nil, err
	}
	// Proposing a time does not imply accepting or tentatively accepting the
	// existing time. Keep the previous RSVP until the attendee changes it.
	if proposedStart != nil {
		status = inv.PartStat
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendar_invites SET partstat=?, proposed_start=?, proposed_end=?, updated_at=? WHERE id=?`), status, proposedStart, proposedEnd, now, inv.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	inv.PartStat, inv.ProposedStart, inv.ProposedEnd, inv.UpdatedAt = status, proposedStart, proposedEnd, now
	return inv, nil
}

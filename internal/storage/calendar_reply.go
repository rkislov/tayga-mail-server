package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

var ErrCalendarConflict = errors.New("calendar changed; refresh before retrying")

// ReplyCalendarInvite commits participation and both local calendar copies
// together. A COUNTER stores a proposal; only the organizer can move a meeting.
func (s *Store) ReplyCalendarInvite(ctx context.Context, userID, inviteID, status string, proposedStart, proposedEnd *time.Time, etag string, comment ...string) (*CalendarInvite, error) {
	return s.replyCalendarInvite(ctx, userID, inviteID, status, proposedStart, proposedEnd, etag, "", true, false, comment...)
}

// RespondCalendarInvite applies a mobile RSVP without implicitly sending mail.
// Repeated transport requests are idempotent after validating the current event.
func (s *Store) RespondCalendarInvite(ctx context.Context, userID, inviteID, status, etag string, sendMail bool, comment string) (*CalendarInvite, error) {
	return s.replyCalendarInvite(ctx, userID, inviteID, status, nil, nil, etag, "", sendMail, true, comment)
}

// ReplyCalendarOccurrence changes a single recurrence identity, leaving the
// series RSVP and whole-series time proposal intact. A repeated response is a
// no-op, but still validates ownership, the occurrence and the current ETag.
func (s *Store) ReplyCalendarOccurrence(ctx context.Context, userID, inviteID, rid, status, etag string, sendMail bool, comment string) (*CalendarInvite, error) {
	if rid == "" || etag == "" {
		return nil, ErrCalendarConflict
	}
	return s.replyCalendarInvite(ctx, userID, inviteID, status, nil, nil, etag, rid, sendMail, true, comment)
}

func (s *Store) replyCalendarInvite(ctx context.Context, userID, inviteID, status string, proposedStart, proposedEnd *time.Time, etag, rid string, sendMail, idempotent bool, comment ...string) (*CalendarInvite, error) {
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
	result, err := s.replyCalendarInviteTx(ctx, tx, userID, inviteID, status, proposedStart, proposedEnd, etag, rid, sendMail, idempotent, comment...)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *Store) replyCalendarInviteTx(ctx context.Context, tx *sql.Tx, userID, inviteID, status string, proposedStart, proposedEnd *time.Time, etag, rid string, sendMail, idempotent bool, comment ...string) (*CalendarInvite, error) {
	var err error
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
			if strings.EqualFold(parsed.Status, "CANCELLED") {
				return nil, ErrNotFound
			}
			if rid != "" {
				instance, err := calutil.GetOccurrence(o.Data, rid)
				if err != nil {
					return nil, ErrCalendarConflict
				}
				parsed = instance.ParseResult
			}
			reference = &parsed
		}
	}
	if reference == nil || strings.EqualFold(reference.Status, "CANCELLED") || !strings.EqualFold(reference.Organizer.Email, organizerEmail) {
		return nil, ErrNotFound
	}
	var pending *CalendarOccurrenceProposal
	if rid != "" {
		pending, err = scanOccurrenceProposal(tx.QueryRowContext(ctx, s.rebind(`SELECT `+occurrenceProposalColumns+` FROM calendar_occurrence_proposals WHERE invite_id=? AND recurrence_id=?`), inviteID, rid))
		if err != nil && err != ErrNotFound {
			return nil, err
		}
	}
	foundOrganizer := false
	organizerObjectID := ""
	unchanged := idempotent && proposedStart == nil && pending == nil && (rid != "" || inv.PartStat == status && inv.ProposedStart == nil && inv.ProposedEnd == nil)
	now := time.Now().UTC()
	for _, o := range objects {
		parsed, err := calutil.ParseICS(o.Data)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(parsed.Organizer.Email, organizerEmail) {
			continue
		}
		if strings.EqualFold(parsed.Status, "CANCELLED") {
			return nil, ErrCalendarConflict
		}
		if rid != "" {
			instance, err := calutil.GetOccurrence(o.Data, rid)
			if err != nil {
				return nil, ErrCalendarConflict
			}
			parsed = instance.ParseResult
		}
		if parsed.Sequence != reference.Sequence || strings.EqualFold(parsed.Status, "CANCELLED") || !strings.EqualFold(parsed.Organizer.Email, organizerEmail) {
			return nil, ErrCalendarConflict
		}
		matchingAttendee := false
		sameResponse := false
		for _, a := range parsed.Attendees {
			if strings.EqualFold(a.Email, inv.AttendeeEmail) {
				matchingAttendee = true
				sameResponse = strings.EqualFold(a.PartStat, status)
				unchanged = unchanged && strings.EqualFold(a.PartStat, status)
			}
		}
		if !matchingAttendee {
			return nil, ErrCalendarConflict
		}
		if o.ID != inv.CalendarObjectID {
			foundOrganizer = true
			if organizerObjectID == "" {
				organizerObjectID = o.ID
			}
		}
		if proposedStart != nil {
			if rid != "" && (parsed.Start == nil || parsed.End == nil || reference.Start == nil || reference.End == nil || !parsed.Start.Equal(*reference.Start) || !parsed.End.Equal(*reference.End)) {
				return nil, ErrCalendarConflict
			}
			continue
		}
		if rid != "" && sameResponse {
			continue
		}
		var data string
		if rid != "" {
			data, err = calutil.WithOccurrenceAttendeeStatus(o.Data, rid, inv.AttendeeEmail, status)
		} else {
			data, err = calutil.WithAttendeeStatus(o.Data, inv.AttendeeEmail, status)
		}
		if err != nil {
			return nil, ErrCalendarConflict
		}
		if proposedStart != nil || (idempotent && data == o.Data) {
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
		if o.ID == inv.CalendarObjectID {
			if _, err = s.bumpCalendarScheduleTagTx(ctx, tx, o.ID); err != nil {
				return nil, err
			}
		}
		if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendars SET ctag=? WHERE id=?`), newCTag(), o.CalendarID); err != nil {
			return nil, err
		}
	}
	if !foundOrganizer {
		return nil, ErrNotFound
	}
	if rid != "" && proposedStart != nil && pending != nil && pending.ObjectID == organizerObjectID && pending.Sequence == reference.Sequence && pending.OriginalStart.Equal(*reference.Start) && pending.OriginalEnd.Equal(*reference.End) && pending.ProposedStart.Equal(*proposedStart) && pending.ProposedEnd.Equal(*proposedEnd) && (len(comment) == 0 || pending.Comment == comment[0]) {
		return inv, nil
	}
	if unchanged {
		return inv, nil
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
		if rid != "" && ev.AllDay && (ev.Start.UTC().Format("150405.999999999") != "000000" || ev.End.UTC().Format("150405.999999999") != "000000") {
			ev.AllDay = false
		}
		ics, err = calutil.BuildCounterICS(ev, responder)
	} else {
		ics, err = calutil.BuildReplyICS(ev, responder)
	}
	if err != nil {
		return nil, err
	}
	m := CalendarMail{From: inv.AttendeeEmail, FromName: attendeeName, To: organizerEmail, Subject: "Re: " + inv.Summary + " (" + strings.ToLower(status) + ")", Text: inv.AttendeeEmail + " responded: " + status + "\n" + ev.Comment, ICS: ics, Method: method}
	if sendMail {
		if err = s.queueCalendarMail(ctx, tx, []CalendarMail{m}); err != nil {
			return nil, err
		}
	}
	if rid != "" {
		if proposedStart != nil {
			_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO calendar_occurrence_proposals (`+occurrenceProposalColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT (invite_id,recurrence_id) DO UPDATE SET object_id=excluded.object_id,revision=excluded.revision,event_sequence=excluded.event_sequence,original_start=excluded.original_start,original_end=excluded.original_end,proposed_start=excluded.proposed_start,proposed_end=excluded.proposed_end,comment=excluded.comment,updated_at=excluded.updated_at`), inviteID, rid, organizerObjectID, NewID(), reference.Sequence, reference.Start.UTC(), reference.End.UTC(), proposedStart.UTC(), proposedEnd.UTC(), ev.Comment, now)
		} else {
			_, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM calendar_occurrence_proposals WHERE invite_id=? AND recurrence_id=?`), inviteID, rid)
		}
		if err != nil {
			return nil, err
		}
		return inv, nil
	}
	// Proposing a time does not imply accepting or tentatively accepting the
	// existing time. Keep the previous RSVP until the attendee changes it.
	if proposedStart != nil {
		status = inv.PartStat
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendar_invites SET partstat=?, proposed_start=?, proposed_end=?, updated_at=? WHERE id=?`), status, proposedStart, proposedEnd, now, inv.ID); err != nil {
		return nil, err
	}
	inv.PartStat, inv.ProposedStart, inv.ProposedEnd, inv.UpdatedAt = status, proposedStart, proposedEnd, now
	return inv, nil
}

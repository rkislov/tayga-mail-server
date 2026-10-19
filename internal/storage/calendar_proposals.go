package storage

import (
	"context"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

// CalendarProposal joins a pending counterproposal to the organizer's writable
// event. The attendee copy is never a valid target for an organizer decision.
type CalendarProposal struct {
	Invite *CalendarInvite
	Object *CalendarObject
}

func (s *Store) ListCalendarProposals(ctx context.Context, userID string) ([]*CalendarProposal, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id, organizer_user_id, attendee_email, attendee_user_id, event_uid, calendar_object_id,
		summary, partstat, proposed_start, proposed_end, created_at, updated_at FROM calendar_invites
		WHERE organizer_user_id=? AND proposed_start IS NOT NULL AND proposed_end IS NOT NULL ORDER BY updated_at DESC, id`), userID)
	if err != nil {
		return nil, err
	}
	var invites []*CalendarInvite
	for rows.Next() {
		inv, err := s.scanCalendarInvite(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		invites = append(invites, inv)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []*CalendarProposal
	for _, inv := range invites {
		q := `SELECT o.id, o.calendar_id, o.uid, o.href_name, o.etag, o.data, o.size, o.component, o.dtstart, o.dtend, o.updated_at
			FROM calendar_objects o JOIN calendars c ON c.id=o.calendar_id WHERE o.uid=? AND o.id<>? AND
			(c.user_id=? OR (c.name<>'personal' AND EXISTS (SELECT 1 FROM calendar_acl a WHERE a.calendar_id=c.id AND a.grantee_user_id=? AND a.rights='write'))) ORDER BY o.id`
		objects, err := s.db.QueryContext(ctx, s.rebind(q), inv.EventUID, inv.CalendarObjectID, userID, userID)
		if err != nil {
			return nil, err
		}
		var candidates []*CalendarObject
		for objects.Next() {
			o, err := s.scanCalObject(objects)
			if err != nil {
				objects.Close()
				return nil, err
			}
			candidates = append(candidates, o)
		}
		err = objects.Err()
		objects.Close()
		if err != nil {
			return nil, err
		}
		ref, err := s.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
		if err == ErrNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		reference, err := calutil.ParseICS(ref.Data)
		if err != nil || strings.EqualFold(reference.Status, "CANCELLED") {
			continue
		}
		for _, o := range candidates {
			p, err := calutil.ParseICS(o.Data)
			if err == nil && strings.EqualFold(p.Organizer.Email, user.Email) && !strings.EqualFold(p.Status, "CANCELLED") && p.Sequence == reference.Sequence {
				out = append(out, &CalendarProposal{Invite: inv, Object: o})
				break
			}
		}
	}
	return out, nil
}

// DecideCalendarProposal commits a decision and (on acceptance) the new
// organizer version together. Replays and stale proposals cannot move it twice.
func (s *Store) DecideCalendarProposal(ctx context.Context, userID, inviteID, objectID, revision, etag string, accept bool) (*CalendarProposal, error) {
	if revision == "" || etag == "" {
		return nil, ErrCalendarConflict
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
	if inv.OrganizerUserID != userID {
		return nil, ErrUnauthorized
	}
	if inv.ProposedStart == nil || inv.ProposedEnd == nil || !inv.ProposedEnd.After(*inv.ProposedStart) || inv.UpdatedAt.UTC().Format(time.RFC3339Nano) != revision {
		return nil, ErrCalendarConflict
	}
	var email, name string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT email, display_name FROM users WHERE id=? AND enabled=TRUE`), userID).Scan(&email, &name); err != nil {
		return nil, ErrUnauthorized
	}
	q = `SELECT o.id, o.calendar_id, o.uid, o.href_name, o.etag, o.data, o.size, o.component, o.dtstart, o.dtend, o.updated_at
		FROM calendar_objects o JOIN calendars c ON c.id=o.calendar_id WHERE o.id=? AND o.id<>? AND
		(c.user_id=? OR (c.name<>'personal' AND EXISTS (SELECT 1 FROM calendar_acl a WHERE a.calendar_id=c.id AND a.grantee_user_id=? AND a.rights='write')))`
	if s.dialect == DialectPostgres {
		q += ` FOR UPDATE OF o`
	}
	o, err := s.scanCalObject(tx.QueryRowContext(ctx, s.rebind(q), objectID, inv.CalendarObjectID, userID, userID))
	if err != nil {
		return nil, err
	}
	p, err := calutil.ParseICS(o.Data)
	if err != nil {
		return nil, err
	}
	if o.ETag != etag || o.UID != inv.EventUID || p.UID != inv.EventUID || !strings.EqualFold(p.Organizer.Email, email) || strings.EqualFold(p.Status, "CANCELLED") {
		return nil, ErrCalendarConflict
	}
	var reference string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT data FROM calendar_objects WHERE id=?`), inv.CalendarObjectID).Scan(&reference); err != nil {
		return nil, ErrCalendarConflict
	}
	rp, err := calutil.ParseICS(reference)
	if err != nil || rp.UID != inv.EventUID || rp.Sequence != p.Sequence || !strings.EqualFold(rp.Organizer.Email, email) || strings.EqualFold(rp.Status, "CANCELLED") {
		return nil, ErrCalendarConflict
	}
	found := false
	for _, a := range p.Attendees {
		if strings.EqualFold(a.Email, inv.AttendeeEmail) {
			found = true
		}
	}
	if !found {
		return nil, ErrCalendarConflict
	}
	now := time.Now().UTC()
	previous := o.Data
	if accept {
		ev := p.Event
		ev.Start, ev.End = inv.ProposedStart, inv.ProposedEnd
		// A counter containing a time is a timed event, even if the original
		// invitation was all-day. Avoid silently discarding the proposed hours.
		if ev.AllDay && (ev.Start.UTC().Format("150405") != "000000" || ev.End.UTC().Format("150405") != "000000") {
			ev.AllDay = false
		}
		ev.Sequence = calutil.NextSequence(o.Data)
		for i := range ev.Attendees {
			ev.Attendees[i].PartStat = "NEEDS-ACTION"
			ev.Attendees[i].RSVP = true
		}
		data, err := calutil.BuildRequestICS(ev)
		if err != nil {
			return nil, err
		}
		result, err := tx.ExecContext(ctx, s.rebind(`UPDATE calendar_objects SET data=?, etag=?, size=?, dtstart=?, dtend=?, updated_at=? WHERE id=? AND etag=?`), data, ContentETag(data), len(data), ev.Start, ev.End, now, o.ID, etag)
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
		o.Data, o.ETag, o.Size, o.DTStart, o.DTEnd, o.UpdatedAt = data, ContentETag(data), int64(len(data)), ev.Start, ev.End, now
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendar_invites SET proposed_start=NULL, proposed_end=NULL, updated_at=? WHERE id=?`), now, inv.ID); err != nil {
		return nil, err
	}
	var mails []CalendarMail
	if accept {
		if err = s.bookCalendarResourcesTx(ctx, tx, o, previous); err != nil {
			return nil, err
		}
		mails, err = s.syncCalendarMeetingTx(ctx, tx, userID, o, previous)
	} else {
		var ics string
		ics, err = calutil.BuildDeclineCounterICS(p.Event, calutil.Attendee{Email: inv.AttendeeEmail, PartStat: inv.PartStat})
		mails = []CalendarMail{{From: email, FromName: name, To: inv.AttendeeEmail, Subject: "Re: " + inv.Summary + " (time proposal declined)", Text: "The proposed time was declined. The existing meeting time has not changed.", ICS: ics, Method: "DECLINECOUNTER"}}
	}
	if err != nil {
		return nil, err
	}
	if err = s.queueCalendarMail(ctx, tx, mails); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &CalendarProposal{Invite: inv, Object: o}, nil
}

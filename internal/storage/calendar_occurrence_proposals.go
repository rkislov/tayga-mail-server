package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

// Each invite/date has its own proposal and opaque revision, independent of
// whole-series proposals and of proposals for other recurrence identities.
type CalendarOccurrenceProposal struct {
	InviteID, RecurrenceID, ObjectID, Revision             string
	Sequence                                               int
	OriginalStart, OriginalEnd, ProposedStart, ProposedEnd time.Time
	Comment                                                string
	UpdatedAt                                              time.Time
	Invite                                                 *CalendarInvite
	Object                                                 *CalendarObject
}

const occurrenceProposalColumns = `invite_id, recurrence_id, object_id, revision, event_sequence, original_start, original_end, proposed_start, proposed_end, comment, updated_at`

func scanOccurrenceProposal(row interface{ Scan(...any) error }) (*CalendarOccurrenceProposal, error) {
	p := &CalendarOccurrenceProposal{}
	err := row.Scan(&p.InviteID, &p.RecurrenceID, &p.ObjectID, &p.Revision, &p.Sequence, &p.OriginalStart, &p.OriginalEnd, &p.ProposedStart, &p.ProposedEnd, &p.Comment, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (s *Store) GetCalendarOccurrenceProposal(ctx context.Context, inviteID, rid string) (*CalendarOccurrenceProposal, error) {
	return scanOccurrenceProposal(s.db.QueryRowContext(ctx, s.rebind(`SELECT `+occurrenceProposalColumns+` FROM calendar_occurrence_proposals WHERE invite_id=? AND recurrence_id=?`), inviteID, rid))
}

func (s *Store) ProposeCalendarOccurrence(ctx context.Context, userID, inviteID, rid, etag string, start, end time.Time, comment string) (*CalendarInvite, error) {
	if rid == "" || etag == "" {
		return nil, ErrCalendarConflict
	}
	if !end.After(start) || end.Sub(start) > 31*24*time.Hour {
		return nil, ErrUnauthorized
	}
	return s.replyCalendarInvite(ctx, userID, inviteID, "TENTATIVE", &start, &end, etag, rid, true, false, comment)
}

// Both copies must still refer to the proposal's original local occurrence.
func occurrenceProposalEvent(p *CalendarOccurrenceProposal, inv *CalendarInvite, source, copy *CalendarObject, email string) (calutil.Occurrence, error) {
	var event calutil.Occurrence
	for _, o := range []*CalendarObject{source, copy} {
		master, err := calutil.ParseICS(o.Data)
		if err != nil || o.UID != inv.EventUID || master.UID != inv.EventUID || !strings.EqualFold(master.Organizer.Email, email) || strings.EqualFold(master.Status, "CANCELLED") {
			return event, ErrCalendarConflict
		}
		instance, err := calutil.GetOccurrence(o.Data, p.RecurrenceID)
		if err != nil || strings.EqualFold(instance.Status, "CANCELLED") || !strings.EqualFold(instance.Organizer.Email, email) || instance.Sequence != p.Sequence || instance.Start == nil || instance.End == nil || !instance.Start.Equal(p.OriginalStart) || !instance.End.Equal(p.OriginalEnd) {
			return event, ErrCalendarConflict
		}
		found := false
		for _, a := range instance.Attendees {
			if strings.EqualFold(a.Email, inv.AttendeeEmail) {
				found = true
			}
		}
		if !found {
			return event, ErrCalendarConflict
		}
		if o.ID == source.ID {
			event = instance
		}
	}
	return event, nil
}

func (s *Store) ListCalendarOccurrenceProposals(ctx context.Context, userID string) ([]*CalendarOccurrenceProposal, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT p.invite_id,p.recurrence_id,p.object_id,p.revision,p.event_sequence,p.original_start,p.original_end,p.proposed_start,p.proposed_end,p.comment,p.updated_at FROM calendar_occurrence_proposals p JOIN calendar_invites i ON i.id=p.invite_id WHERE i.organizer_user_id=? ORDER BY p.updated_at DESC,p.invite_id,p.recurrence_id`), userID)
	if err != nil {
		return nil, err
	}
	var pending []*CalendarOccurrenceProposal
	for rows.Next() {
		p, e := scanOccurrenceProposal(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		pending = append(pending, p)
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
	var out []*CalendarOccurrenceProposal
	for _, p := range pending {
		inv, e := s.GetCalendarInvite(ctx, p.InviteID)
		if e == ErrNotFound {
			continue
		}
		if e != nil {
			return nil, e
		}
		if inv.OrganizerUserID != userID {
			continue
		}
		source, e := s.GetCalendarObjectByID(ctx, p.ObjectID)
		if e == ErrNotFound {
			continue
		}
		if e != nil {
			return nil, e
		}
		cal, e := s.GetCalendarByID(ctx, userID, source.CalendarID)
		if e != nil && e != ErrNotFound {
			return nil, e
		}
		if e != nil {
			rights, err := s.CalendarRightsForUser(ctx, source.CalendarID, userID)
			if err != nil {
				return nil, err
			}
			if rights != "write" {
				continue
			}
			// Personal busy-only calendars cannot grant an organizer write target.
			var name string
			if e = s.db.QueryRowContext(ctx, s.rebind(`SELECT name FROM calendars WHERE id=?`), source.CalendarID).Scan(&name); e != nil {
				return nil, e
			}
			if name == "personal" {
				continue
			}
		}
		if cal != nil && cal.Name == "personal" && cal.UserID != userID {
			continue
		}
		copy, e := s.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
		if e == ErrNotFound {
			continue
		}
		if e != nil {
			return nil, e
		}
		if _, e = occurrenceProposalEvent(p, inv, source, copy, user.Email); e != nil {
			continue
		}
		p.Invite, p.Object = inv, source
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) DecideCalendarOccurrenceProposal(ctx context.Context, userID, inviteID, objectID, rid, revision, etag string, accept bool) (*CalendarOccurrenceProposal, error) {
	if rid == "" || revision == "" || etag == "" {
		return nil, ErrCalendarConflict
	}
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	p, err := scanOccurrenceProposal(tx.QueryRowContext(ctx, s.rebind(`SELECT `+occurrenceProposalColumns+` FROM calendar_occurrence_proposals WHERE invite_id=? AND recurrence_id=?`), inviteID, rid))
	if err == ErrNotFound {
		return nil, ErrCalendarConflict
	}
	if err != nil {
		return nil, err
	}
	inv, err := s.scanCalendarInvite(tx.QueryRowContext(ctx, s.rebind(`SELECT id, organizer_user_id, attendee_email, attendee_user_id, event_uid, calendar_object_id, summary, partstat, proposed_start, proposed_end, created_at, updated_at FROM calendar_invites WHERE id=?`), inviteID))
	if err != nil {
		return nil, err
	}
	if inv.OrganizerUserID != userID {
		return nil, ErrUnauthorized
	}
	if p.Revision != revision || p.ObjectID != objectID || objectID == inv.CalendarObjectID {
		return nil, ErrCalendarConflict
	}
	if !p.ProposedEnd.After(p.ProposedStart) || p.ProposedEnd.Sub(p.ProposedStart) > 31*24*time.Hour {
		return nil, ErrCalendarConflict
	}
	var email, name string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT email,display_name FROM users WHERE id=? AND enabled=TRUE`), userID).Scan(&email, &name); err != nil {
		return nil, ErrUnauthorized
	}
	source, err := s.scanCalObject(tx.QueryRowContext(ctx, s.rebind(`SELECT o.id,o.calendar_id,o.uid,o.href_name,o.etag,o.data,o.size,o.component,o.dtstart,o.dtend,o.updated_at FROM calendar_objects o JOIN calendars c ON c.id=o.calendar_id WHERE o.id=? AND (c.user_id=? OR (c.name<>'personal' AND EXISTS (SELECT 1 FROM calendar_acl a WHERE a.calendar_id=c.id AND a.grantee_user_id=? AND a.rights='write')))`), objectID, userID, userID))
	if err != nil {
		return nil, err
	}
	if source.ETag != etag {
		return nil, ErrCalendarConflict
	}
	copy, err := s.scanCalObject(tx.QueryRowContext(ctx, s.rebind(`SELECT id,calendar_id,uid,href_name,etag,data,size,component,dtstart,dtend,updated_at FROM calendar_objects WHERE id=?`), inv.CalendarObjectID))
	if err != nil {
		return nil, ErrCalendarConflict
	}
	instance, err := occurrenceProposalEvent(p, inv, source, copy, email)
	if err != nil {
		return nil, err
	}
	if accept {
		allDay := instance.AllDay && p.ProposedStart.UTC().Format("150405.999999999") == "000000" && p.ProposedEnd.UTC().Format("150405.999999999") == "000000"
		source, err = s.changeCalendarSeriesTx(ctx, tx, userID, source.ID, etag, rid, calutil.OccurrenceChange{Start: &p.ProposedStart, End: &p.ProposedEnd, AllDay: &allDay, ResetResponses: true}, nil)
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM calendar_occurrence_proposals WHERE recurrence_id=? AND invite_id IN (SELECT id FROM calendar_invites WHERE organizer_user_id=? AND event_uid=?)`), rid, userID, inv.EventUID)
	} else {
		status := inv.PartStat
		for _, a := range instance.Attendees {
			if strings.EqualFold(a.Email, inv.AttendeeEmail) {
				status = a.PartStat
			}
		}
		ics, e := calutil.BuildDeclineCounterICS(instance.Event, calutil.Attendee{Email: inv.AttendeeEmail, PartStat: status})
		if e != nil {
			return nil, e
		}
		if err = s.queueCalendarMail(ctx, tx, []CalendarMail{{From: email, FromName: name, To: inv.AttendeeEmail, Subject: "Re: " + instance.Summary + " (occurrence time proposal declined)", Text: "The proposed time for this occurrence was declined. The meeting time has not changed.", ICS: ics, Method: "DECLINECOUNTER"}}); err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM calendar_occurrence_proposals WHERE invite_id=? AND recurrence_id=?`), inviteID, rid)
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	p.Invite, p.Object = inv, source
	return p, nil
}

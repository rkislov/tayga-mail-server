package storage

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// CalendarInvite tracks RSVP state for a meeting invitee.
type CalendarInvite struct {
	ID               string
	OrganizerUserID  string
	AttendeeEmail    string
	AttendeeUserID   string // may be empty for external
	EventUID         string
	CalendarObjectID string
	Summary          string
	PartStat         string
	ProposedStart    *time.Time
	ProposedEnd      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// BusyInterval is a free/busy busy block (no summary).
type BusyInterval struct {
	Start time.Time
	End   time.Time
}

func (s *Store) UpsertCalendarInvite(ctx context.Context, inv *CalendarInvite) (*CalendarInvite, error) {
	now := time.Now().UTC()
	inv.AttendeeEmail = strings.ToLower(strings.TrimSpace(inv.AttendeeEmail))
	if inv.PartStat == "" {
		inv.PartStat = "NEEDS-ACTION"
	}
	inv.UpdatedAt = now
	if inv.ID == "" {
		inv.ID = NewID()
		inv.CreatedAt = now
		q := s.rebind(`INSERT INTO calendar_invites
			(id, organizer_user_id, attendee_email, attendee_user_id, event_uid, calendar_object_id, summary, partstat, proposed_start, proposed_end, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		var attUID any
		if inv.AttendeeUserID != "" {
			attUID = inv.AttendeeUserID
		}
		_, err := s.db.ExecContext(ctx, q,
			inv.ID, inv.OrganizerUserID, inv.AttendeeEmail, attUID, inv.EventUID, inv.CalendarObjectID,
			inv.Summary, inv.PartStat, inv.ProposedStart, inv.ProposedEnd, inv.CreatedAt, inv.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		return inv, nil
	}
	q := s.rebind(`UPDATE calendar_invites SET
		attendee_email=?, attendee_user_id=?, event_uid=?, calendar_object_id=?, summary=?, partstat=?,
		proposed_start=?, proposed_end=?, updated_at=? WHERE id=?`)
	var attUID any
	if inv.AttendeeUserID != "" {
		attUID = inv.AttendeeUserID
	}
	_, err := s.db.ExecContext(ctx, q,
		inv.AttendeeEmail, attUID, inv.EventUID, inv.CalendarObjectID, inv.Summary, inv.PartStat,
		inv.ProposedStart, inv.ProposedEnd, inv.UpdatedAt, inv.ID,
	)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (s *Store) GetCalendarInvite(ctx context.Context, id string) (*CalendarInvite, error) {
	q := s.rebind(`SELECT id, organizer_user_id, attendee_email, attendee_user_id, event_uid, calendar_object_id,
		summary, partstat, proposed_start, proposed_end, created_at, updated_at
		FROM calendar_invites WHERE id = ?`)
	return s.scanCalendarInvite(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) ListCalendarInvitesForUser(ctx context.Context, userID string, pendingOnly bool) ([]*CalendarInvite, error) {
	q := `SELECT id, organizer_user_id, attendee_email, attendee_user_id, event_uid, calendar_object_id,
		summary, partstat, proposed_start, proposed_end, created_at, updated_at
		FROM calendar_invites WHERE attendee_user_id = ?`
	if pendingOnly {
		q += ` AND UPPER(partstat) IN ('NEEDS-ACTION','TENTATIVE')`
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, s.rebind(q), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CalendarInvite
	for rows.Next() {
		inv, err := s.scanCalendarInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *Store) ListCalendarInvitesByEventUID(ctx context.Context, eventUID string) ([]*CalendarInvite, error) {
	q := s.rebind(`SELECT id, organizer_user_id, attendee_email, attendee_user_id, event_uid, calendar_object_id,
		summary, partstat, proposed_start, proposed_end, created_at, updated_at
		FROM calendar_invites WHERE event_uid = ? ORDER BY attendee_email`)
	rows, err := s.db.QueryContext(ctx, q, eventUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CalendarInvite
	for rows.Next() {
		inv, err := s.scanCalendarInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *Store) UpdateCalendarInvitePartStat(ctx context.Context, id, partstat string, proposedStart, proposedEnd *time.Time) error {
	q := s.rebind(`UPDATE calendar_invites SET partstat=?, proposed_start=?, proposed_end=?, updated_at=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, q, partstat, proposedStart, proposedEnd, time.Now().UTC(), id)
	return err
}

func (s *Store) scanCalendarInvite(row interface{ Scan(dest ...any) error }) (*CalendarInvite, error) {
	inv := &CalendarInvite{}
	var attUID sql.NullString
	var pStart, pEnd sql.NullTime
	err := row.Scan(
		&inv.ID, &inv.OrganizerUserID, &inv.AttendeeEmail, &attUID, &inv.EventUID, &inv.CalendarObjectID,
		&inv.Summary, &inv.PartStat, &pStart, &pEnd, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if attUID.Valid {
		inv.AttendeeUserID = attUID.String
	}
	if pStart.Valid {
		t := pStart.Time.UTC()
		inv.ProposedStart = &t
	}
	if pEnd.Valid {
		t := pEnd.Time.UTC()
		inv.ProposedEnd = &t
	}
	return inv, nil
}

// FreeBusyForUser returns busy intervals from owned and shared calendars with freebusy|read|write rights.
func (s *Store) FreeBusyForUser(ctx context.Context, userID string, from, to time.Time) ([]BusyInterval, error) {
	calIDs := map[string]struct{}{}
	owned, err := s.ListCalendars(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, c := range owned {
		calIDs[c.ID] = struct{}{}
	}
	shared, err := s.ListSharedCalendars(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, c := range shared {
		rights, err := s.CalendarRightsForUser(ctx, c.ID, userID)
		if err != nil || rights == "" {
			continue
		}
		// any non-empty ACL grants freebusy visibility
		calIDs[c.ID] = struct{}{}
	}
	var out []BusyInterval
	for id := range calIDs {
		objs, err := s.ListCalendarObjects(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, o := range objs {
			if o.DTStart == nil {
				continue
			}
			start := o.DTStart.UTC()
			end := start.Add(time.Hour)
			if o.DTEnd != nil {
				end = o.DTEnd.UTC()
			}
			if !end.After(from) || !start.Before(to) {
				continue
			}
			out = append(out, BusyInterval{Start: start, End: end})
		}
	}
	return out, nil
}

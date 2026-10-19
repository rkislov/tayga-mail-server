package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

// SaveCalendarMeeting is the organizer write path. Source, resources, personal
// copies, invitation state and immutable mail jobs commit together.
func (s *Store) SaveCalendarMeeting(ctx context.Context, actorID string, o *CalendarObject) (*CalendarObject, error) {
	if actorID == "" {
		return nil, ErrUnauthorized
	}
	if !o.CreateOnly && o.ExpectedETag == "" {
		return nil, ErrCalendarConflict
	}
	return s.saveCalendarObject(ctx, o, nil, actorID, false)
}

// SaveCalendarClientObject connects DAV writes to scheduling without treating
// echoes or personal edits as new organizer invitations. Preconditions and
// scheduling decisions are evaluated under the same database write lock.
func (s *Store) SaveCalendarClientObject(ctx context.Context, actorID string, o *CalendarObject) (*CalendarObject, error) {
	if actorID == "" {
		return nil, ErrUnauthorized
	}
	if !o.CreateOnly && o.ExpectedETag == "" {
		return nil, ErrCalendarConflict
	}
	return s.saveCalendarObject(ctx, o, nil, actorID, true)
}

func (s *Store) DeleteCalendarMeeting(ctx context.Context, actorID string, o *CalendarObject) error {
	if actorID == "" {
		return ErrUnauthorized
	}
	return s.deleteCalendarObject(ctx, o, nil, actorID)
}

// Recheck access inside the transaction, including shared calendar writes.
func (s *Store) calendarWriterTx(ctx context.Context, tx *sql.Tx, actorID, calendarID string) (email, name string, err error) {
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT u.email,u.display_name FROM users u JOIN calendars c ON c.id=? WHERE u.id=? AND u.enabled=TRUE AND (c.user_id=u.id OR (c.name<>'personal' AND EXISTS (SELECT 1 FROM calendar_acl a WHERE a.calendar_id=c.id AND a.grantee_user_id=u.id AND a.rights='write')))`), calendarID, actorID).Scan(&email, &name)
	if err == sql.ErrNoRows {
		err = ErrUnauthorized
	}
	return
}

// Includes disabled accounts so an existing local identity cannot accidentally
// become an external recipient. The delivery worker handles disabled users.
func (s *Store) calendarRecipientTx(ctx context.Context, tx *sql.Tx, email string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT id FROM users WHERE LOWER(email)=LOWER(?)`), email).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return "", fmt.Errorf("invalid calendar recipient")
	}
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT a.user_id FROM aliases a JOIN domains d ON d.id=a.domain_id WHERE LOWER(d.name)=LOWER(?) AND LOWER(a.local_part)=LOWER(?)`), email[at+1:], email[:at]).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

func (s *Store) participantCopiesTx(ctx context.Context, tx *sql.Tx, userID, uid, organizerEmail, sourceID string) ([]*CalendarObject, error) {
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT o.id,o.calendar_id,o.uid,o.href_name,o.etag,o.data,o.size,o.component,o.dtstart,o.dtend,o.updated_at FROM calendar_objects o JOIN calendars c ON c.id=o.calendar_id WHERE c.user_id=? AND o.uid=? AND o.id<>? ORDER BY o.id`), userID, uid, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var copies []*CalendarObject
	for rows.Next() {
		o, err := s.scanCalObject(rows)
		if err != nil {
			return nil, err
		}
		p, err := calutil.ParseICS(o.Data)
		if err != nil {
			return nil, err
		}
		if p.UID == uid && strings.EqualFold(p.Organizer.Email, organizerEmail) {
			copies = append(copies, o)
		}
	}
	return copies, rows.Err()
}

func (s *Store) newParticipantCopyTx(ctx context.Context, tx *sql.Tx, userID, organizerID, organizerEmail, uid string) (*CalendarObject, error) {
	ensure := func(name, display string) (string, error) {
		_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO calendars(id,user_id,name,display_name,ctag,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(user_id,name) DO NOTHING`), NewID(), userID, name, display, newCTag(), time.Now().UTC())
		if err != nil {
			return "", err
		}
		var id string
		err = tx.QueryRowContext(ctx, s.rebind(`SELECT id FROM calendars WHERE user_id=? AND name=?`), userID, name).Scan(&id)
		return id, err
	}
	calendarID, err := ensure("default", "Calendar")
	if err != nil {
		return nil, err
	}
	var n int
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM calendar_objects WHERE calendar_id=? AND uid=?`), calendarID, uid).Scan(&n); err != nil {
		return nil, err
	}
	if n > 0 {
		calendarID, err = ensure("meetings-"+organizerID, "Meetings · "+organizerEmail)
		if err != nil {
			return nil, err
		}
	}
	return &CalendarObject{CalendarID: calendarID, UID: uid, HrefName: fmt.Sprintf("meeting-%x.ics", sha256.Sum256([]byte(organizerID+"\x00"+uid))), CreateOnly: true}, nil
}

func (s *Store) syncCalendarMeetingTx(ctx context.Context, tx *sql.Tx, actorID string, source *CalendarObject, previous string) ([]CalendarMail, error) {
	email, name, err := s.calendarWriterTx(ctx, tx, actorID, source.CalendarID)
	if err != nil {
		return nil, err
	}
	p, err := calutil.ParseICS(source.Data)
	if err != nil {
		return nil, err
	}
	if p.UID != source.UID || !strings.EqualFold(p.Organizer.Email, email) {
		return nil, ErrUnauthorized
	}
	var before calutil.ParseResult
	if previous != "" {
		before, err = calutil.ParseICS(previous)
		if err != nil {
			return nil, err
		}
		if before.UID != p.UID || before.Organizer.Email != "" && !strings.EqualFold(before.Organizer.Email, email) {
			return nil, ErrCalendarConflict
		}
		if p.Sequence < calutil.NextSequence(previous)-1 {
			return nil, ErrCalendarConflict
		}
	}
	cancelled := strings.EqualFold(p.Status, "CANCELLED")
	current := map[string]calutil.Attendee{}
	attendees, err := calutil.SchedulingAttendees(source.Data)
	if err != nil {
		return nil, err
	}
	if !cancelled {
		for _, a := range attendees {
			current[strings.ToLower(strings.TrimSpace(a.Email))] = a
		}
	}
	all := map[string]calutil.Attendee{}
	for k, a := range current {
		all[k] = a
	}
	if previous != "" {
		oldAttendees, err := calutil.SchedulingAttendees(previous)
		if err != nil {
			return nil, err
		}
		for _, a := range oldAttendees {
			key := strings.ToLower(strings.TrimSpace(a.Email))
			if _, ok := all[key]; !ok {
				all[key] = a
			}
		}
	}
	// Resolve addresses first: an alias removed while another address of the
	// same account remains must not cancel that account's shared meeting copy.
	ids := map[string]string{}
	retained := map[string]bool{}
	keys := make([]string, 0, len(all))
	for key := range all {
		if key == "" || strings.EqualFold(key, email) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		id, err := s.calendarRecipientTx(ctx, tx, key)
		if err != nil {
			return nil, err
		}
		ids[key] = id
		if _, ok := current[key]; ok && id != "" {
			retained[id] = true
		}
	}
	processed := map[string]string{}
	for _, key := range keys {
		var resource int
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM calendar_resources WHERE LOWER(email)=?`), key).Scan(&resource); err != nil {
			return nil, err
		}
		if resource > 0 {
			continue
		} // Already handled by resource booking.
		active := false
		if _, ok := current[key]; ok {
			active = true
		}
		userID := ids[key]
		if userID != "" {
			var aliasResource int
			if err := tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM calendar_resources WHERE user_id=?`), userID).Scan(&aliasResource); err != nil {
				return nil, err
			}
			if aliasResource > 0 {
				return nil, &CalendarBookingError{Reason: "use the resource's primary address: " + key}
			}
		}
		var priorUser sql.NullString
		var priorObject string
		err = tx.QueryRowContext(ctx, s.rebind(`SELECT attendee_user_id,calendar_object_id FROM calendar_invites WHERE organizer_user_id=? AND event_uid=? AND attendee_email=?`), actorID, p.UID, key).Scan(&priorUser, &priorObject)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if priorUser.Valid && priorUser.String != userID {
			if active {
				return nil, ErrCalendarConflict
			} // Address changed owners.
			userID = priorUser.String // Cancel the original account's copy only.
		}
		objectID := source.ID
		if !active && retained[userID] && priorObject != "" {
			objectID = priorObject
		}
		if userID != "" && userID != actorID && (active || !retained[userID]) {
			if savedID, ok := processed[userID]; ok {
				objectID = savedID
			} else {
				copies, err := s.participantCopiesTx(ctx, tx, userID, p.UID, email, source.ID)
				if err != nil {
					return nil, err
				}
				if active && len(copies) == 0 {
					copy, err := s.newParticipantCopyTx(ctx, tx, userID, actorID, email, p.UID)
					if err != nil {
						return nil, err
					}
					copies = append(copies, copy)
				}
				for _, copy := range copies {
					data := source.Data
					if copy.Data != "" {
						cp, err := calutil.ParseICS(copy.Data)
						if err != nil {
							return nil, err
						}
						if calutil.NextSequence(copy.Data)-1 > p.Sequence {
							return nil, ErrCalendarConflict
						}
						copy.ExpectedETag = copy.ETag
						if !active {
							cp.Sequence = p.Sequence
							data, err = calutil.BuildCancelICS(cp.Event)
							if err != nil {
								return nil, err
							}
						}
					}
					data, err = calutil.MergeMeetingCopy(data, copy.Data)
					if err != nil {
						return nil, err
					}
					cp, err := calutil.ParseICS(data)
					if err != nil {
						return nil, err
					}
					copy.Data, copy.UID, copy.Component, copy.DTStart, copy.DTEnd = data, p.UID, "VEVENT", cp.Start, cp.End
					if err = s.saveCalendarObjectTx(ctx, tx, copy); err != nil {
						return nil, err
					}
					if objectID == source.ID {
						objectID = copy.ID
					}
				}
				processed[userID] = objectID
			}
		}
		status := "DECLINED"
		if active {
			status = current[key].PartStat
			if status == "" {
				status = "NEEDS-ACTION"
			}
		}
		var local any
		if userID != "" {
			local = userID
		}
		now := time.Now().UTC()
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO calendar_invites(id,organizer_user_id,attendee_email,attendee_user_id,event_uid,calendar_object_id,summary,partstat,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(organizer_user_id,event_uid,attendee_email) DO UPDATE SET attendee_user_id=excluded.attendee_user_id,calendar_object_id=excluded.calendar_object_id,summary=excluded.summary,partstat=excluded.partstat,proposed_start=NULL,proposed_end=NULL,updated_at=excluded.updated_at`), NewID(), actorID, key, local, p.UID, objectID, p.Summary, status, now, now)
		if err != nil {
			return nil, err
		}
	}
	var mails []CalendarMail
	if !cancelled {
		var wire string
		wire, err = calutil.BuildRequestICS(p.Event)
		if err != nil {
			return nil, err
		}
		mails = calendarAttendeeMail(email, name, attendees, "Invitation: "+p.Summary, "Meeting: "+p.Summary+"\n"+calutil.FormatWhen(p.Start, p.End), wire, "REQUEST")
	}
	if previous != "" {
		removed := before.Event
		removed.Sequence, removed.Attendees = p.Sequence, nil
		for _, key := range keys {
			if _, ok := current[key]; !ok {
				removed.Attendees = append(removed.Attendees, all[key])
			}
		}
		cancellations, err := CalendarEventMail(email, name, removed, true)
		if err != nil {
			return nil, err
		}
		mails = append(mails, cancellations...)
	}
	return mails, nil
}

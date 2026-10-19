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

// CalendarBookingError is safe to show to the organizer: it exposes the room
// and conflicting interval, never another meeting's title or participants.
type CalendarBookingError struct {
	Resource string
	Start    time.Time
	Reason   string
}

func (e *CalendarBookingError) Error() string {
	if e.Start.IsZero() {
		return "resource booking: " + e.Reason
	}
	return fmt.Sprintf("resource is busy: %s (%s)", e.Resource, e.Start.UTC().Format(time.RFC3339))
}

// All calendar interval writers take the same database lock BEFORE any reads.
// SQLite reserves its writer slot; PostgreSQL uses a transaction-scoped lock.
// This also covers an empty resource table and writers in other server processes.
// A coarse lock is deliberate: personal/shared calendar writes can also change
// resource availability. Narrowing it requires an explicit reservation index.
func (s *Store) beginCalendarWrite(ctx context.Context) (*sql.Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if s.dialect == DialectPostgres {
		_, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(6071222815910778194)`)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE calendar_resources SET id=id WHERE 1=0`)
	}
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func (s *Store) resourceObjectsTx(ctx context.Context, tx *sql.Tx, userID string) ([]*CalendarObject, error) {
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT o.id,o.calendar_id,o.uid,o.href_name,o.etag,o.data,o.size,o.component,o.dtstart,o.dtend,o.updated_at FROM calendar_objects o JOIN calendars c ON c.id=o.calendar_id WHERE c.user_id=? OR EXISTS (SELECT 1 FROM calendar_acl a WHERE a.calendar_id=c.id AND a.grantee_user_id=? AND a.rights<>'') ORDER BY o.id`), userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CalendarObject
	for rows.Next() {
		o, err := s.scanCalObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) checkResourceBookingTx(ctx context.Context, tx *sql.Tx, res *CalendarResource, o *CalendarObject, items []calutil.Occurrence, excludeMeeting bool) error {
	var wanted []BusyInterval
	for _, item := range items {
		if item.Transparent {
			continue
		}
		wanted = append(wanted, BusyInterval{Start: *item.Start, End: *item.End})
	}
	if len(wanted) == 0 {
		return nil
	}
	sort.Slice(wanted, func(i, j int) bool { return wanted[i].Start.Before(wanted[j].Start) })
	from, to := wanted[0].Start, wanted[0].End
	for i, b := range wanted {
		if i > 0 && b.Start.Before(to) {
			return &CalendarBookingError{Resource: res.DisplayName, Start: b.Start}
		}
		if b.End.After(to) {
			to = b.End
		}
	}
	p, err := calutil.ParseICS(o.Data)
	if err != nil {
		return err
	}
	objects, err := s.resourceObjectsTx(ctx, tx, res.UserID)
	if err != nil {
		return err
	}
	for _, other := range objects {
		if other.ID == o.ID || other.Component != "VEVENT" {
			continue
		}
		if excludeMeeting && other.UID == o.UID {
			op, err := calutil.ParseICS(other.Data)
			if err == nil && p.Organizer.Email != "" && strings.EqualFold(op.Organizer.Email, p.Organizer.Email) {
				continue
			}
		}
		intervals, err := CalendarBusyIntervals(other.Data, from, to)
		if err != nil {
			return &CalendarBookingError{Reason: "cannot verify availability of " + res.DisplayName}
		}
		// Both lists are ordered, allowing a linear sweep over the full series.
		i := 0
		for _, b := range intervals {
			for i < len(wanted) && !wanted[i].End.After(b.Start) {
				i++
			}
			if i < len(wanted) && wanted[i].Start.Before(b.End) {
				return &CalendarBookingError{Resource: res.DisplayName, Start: wanted[i].Start}
			}
		}
	}
	return nil
}

// bookCalendarResourcesTx runs after the source write, within the same locked
// transaction. On any failure the source, every room and all mail jobs roll back.
func (s *Store) bookCalendarResourcesTx(ctx context.Context, tx *sql.Tx, o *CalendarObject, previous string) error {
	if o.Component != "VEVENT" {
		return nil
	}
	var ownerID, ownerEmail, tenantID string
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT u.id,u.email,u.tenant_id FROM calendars c JOIN users u ON u.id=c.user_id WHERE c.id=?`), o.CalendarID).Scan(&ownerID, &ownerEmail, &tenantID)
	if err != nil {
		return err
	}
	direct, err := s.scanCalendarResource(tx.QueryRowContext(ctx, s.rebind(`SELECT id,tenant_id,domain_id,user_id,email,local_part,display_name,kind,capacity,description,auto_accept,enabled,created_at FROM calendar_resources WHERE user_id=?`), ownerID))
	if err != nil && err != ErrNotFound {
		return err
	}
	if direct != nil {
		items, err := calutil.BookingOccurrences(o.Data)
		if err != nil {
			return &CalendarBookingError{Reason: err.Error()}
		}
		return s.checkResourceBookingTx(ctx, tx, direct, o, items, false)
	}
	p, err := calutil.ParseICS(o.Data)
	if err != nil {
		return nil
	} // Preserve opaque legacy/non-scheduling objects.
	// An organizer rewrite must not strand old resource reservations, even
	// when the new organizer would otherwise make this look like an invite copy.
	if before, e := calutil.ParseICS(previous); e == nil && before.UID != "" && (before.UID != p.UID || !strings.EqualFold(before.Organizer.Email, p.Organizer.Email)) {
		var n int
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM calendar_invites i JOIN calendar_resources r ON r.user_id=i.attendee_user_id JOIN users u ON u.id=i.organizer_user_id WHERE i.event_uid=? AND LOWER(u.email)=?`), before.UID, strings.ToLower(before.Organizer.Email)).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return &CalendarBookingError{Reason: "cancel existing bookings before changing meeting identity"}
		}
	}
	organizerID := ownerID
	if !strings.EqualFold(p.Organizer.Email, ownerEmail) {
		// Only the organizer's own or writable shared calendar schedules resources.
		err = tx.QueryRowContext(ctx, s.rebind(`SELECT u.id,u.tenant_id FROM users u JOIN calendar_acl a ON a.grantee_user_id=u.id WHERE LOWER(u.email)=? AND a.calendar_id=? AND a.rights='write'`), strings.ToLower(p.Organizer.Email), o.CalendarID).Scan(&organizerID, &tenantID)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
	}
	attendees, err := calutil.EventAttendees(o.Data)
	if err != nil {
		return err
	}
	oldAttendees, _ := calutil.EventAttendees(previous)
	resources := map[string]*CalendarResource{}
	current := map[string]bool{}
	for i, a := range append(attendees, oldAttendees...) {
		email := strings.ToLower(a.Email)
		res, err := s.scanCalendarResource(tx.QueryRowContext(ctx, s.rebind(`SELECT id,tenant_id,domain_id,user_id,email,local_part,display_name,kind,capacity,description,auto_accept,enabled,created_at FROM calendar_resources WHERE email=?`), email))
		if err == ErrNotFound {
			if i < len(attendees) && !strings.EqualFold(p.Status, "CANCELLED") && (strings.EqualFold(a.CUType, "RESOURCE") || strings.EqualFold(a.CUType, "ROOM")) {
				return &CalendarBookingError{Reason: "resource is not configured: " + email}
			}
			continue
		}
		if err != nil {
			return err
		}
		resources[email] = res
		if i < len(attendees) {
			if res.TenantID != tenantID && !strings.EqualFold(p.Status, "CANCELLED") {
				return ErrUnauthorized
			}
			if !res.Enabled && !strings.EqualFold(p.Status, "CANCELLED") {
				return &CalendarBookingError{Reason: "resource is disabled: " + res.DisplayName}
			}
			current[email] = true
		}
	}
	if len(resources) == 0 {
		return nil
	}
	if p.UID != o.UID {
		return &CalendarBookingError{Reason: "booking UID does not match the calendar object"}
	}
	if before, e := calutil.ParseICS(previous); e == nil && before.UID != "" && (before.UID != p.UID || !strings.EqualFold(before.Organizer.Email, p.Organizer.Email)) {
		return &CalendarBookingError{Reason: "cancel existing bookings before changing meeting identity"}
	}

	var items []calutil.Occurrence
	if len(current) > 0 {
		items, err = calutil.BookingOccurrences(o.Data)
		if err != nil {
			return &CalendarBookingError{Reason: err.Error()}
		}
	}
	// Per-instance room reassignment has no UI/RSVP model yet. Refuse it rather
	// than reserving a room for occurrences to which it was not invited.
	for _, item := range items {
		for email := range current {
			present := false
			for _, a := range item.Attendees {
				if strings.EqualFold(a.Email, email) {
					present = true
				}
			}
			if !present {
				return &CalendarBookingError{Reason: "use the same resources for every occurrence"}
			}
		}
	}
	emails := make([]string, 0, len(resources))
	for email := range resources {
		emails = append(emails, email)
	}
	sort.Strings(emails)
	statuses := map[string]string{}
	for _, email := range emails {
		res := resources[email]
		if !current[email] || strings.EqualFold(p.Status, "CANCELLED") {
			continue
		}
		if err = s.checkResourceBookingTx(ctx, tx, res, o, items, true); err != nil {
			return err
		}
		statuses[email] = "NEEDS-ACTION"
		if res.AutoAccept {
			statuses[email] = "ACCEPTED"
		}
	}
	o.Data, err = calutil.WithResourcePartStats(o.Data, statuses)
	if err != nil {
		return err
	}
	o.ETag, o.Size = ContentETag(o.Data), int64(len(o.Data))
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendar_objects SET data=?,etag=?,size=? WHERE id=?`), o.Data, o.ETag, o.Size, o.ID); err != nil {
		return err
	}
	for _, email := range emails {
		res := resources[email]
		objects, err := s.resourceObjectsTx(ctx, tx, res.UserID)
		if err != nil {
			return err
		}
		var copies []*CalendarObject
		for _, other := range objects {
			if other.UID != o.UID {
				continue
			}
			op, err := calutil.ParseICS(other.Data)
			if err == nil && strings.EqualFold(op.Organizer.Email, p.Organizer.Email) {
				// Never overwrite a merely shared calendar (including the source).
				var owner string
				if err = tx.QueryRowContext(ctx, s.rebind(`SELECT user_id FROM calendars WHERE id=?`), other.CalendarID).Scan(&owner); err != nil {
					return err
				}
				if owner == res.UserID {
					copies = append(copies, other)
				}
			}
		}
		if !current[email] || strings.EqualFold(p.Status, "CANCELLED") {
			for _, copy := range copies {
				if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM calendar_objects WHERE id=?`), copy.ID); err != nil {
					return err
				}
				if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendars SET ctag=? WHERE id=?`), newCTag(), copy.CalendarID); err != nil {
					return err
				}
			}
			if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM calendar_invites WHERE organizer_user_id=? AND event_uid=? AND attendee_email=?`), organizerID, o.UID, email); err != nil {
				return err
			}
			continue
		}
		if len(copies) == 0 {
			var calendarID string
			// A separate collection only for colliding UIDs from another organizer.
			err = tx.QueryRowContext(ctx, s.rebind(`SELECT id FROM calendars WHERE user_id=? AND name='default'`), res.UserID).Scan(&calendarID)
			if err != nil {
				return err
			}
			var count int
			if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM calendar_objects WHERE calendar_id=? AND uid=?`), calendarID, o.UID).Scan(&count); err != nil {
				return err
			}
			if count > 0 {
				name := "meetings-" + organizerID
				_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO calendars(id,user_id,name,display_name,ctag,created_at) VALUES (?,?,?,?,?,?) ON CONFLICT(user_id,name) DO NOTHING`), NewID(), res.UserID, name, "Meetings · "+p.Organizer.Email, newCTag(), time.Now().UTC())
				if err != nil {
					return err
				}
				if err = tx.QueryRowContext(ctx, s.rebind(`SELECT id FROM calendars WHERE user_id=? AND name=?`), res.UserID, name).Scan(&calendarID); err != nil {
					return err
				}
			}
			copies = append(copies, &CalendarObject{CalendarID: calendarID, HrefName: fmt.Sprintf("meeting-%x.ics", sha256.Sum256([]byte(organizerID+"\x00"+o.UID)))})
		}
		for _, copy := range copies {
			copy.UID, copy.Data, copy.Component, copy.DTStart, copy.DTEnd = o.UID, o.Data, o.Component, o.DTStart, o.DTEnd
			if err = s.saveCalendarObjectTx(ctx, tx, copy); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO calendar_invites(id,organizer_user_id,attendee_email,attendee_user_id,event_uid,calendar_object_id,summary,partstat,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT(organizer_user_id,event_uid,attendee_email) DO UPDATE SET attendee_user_id=excluded.attendee_user_id,calendar_object_id=excluded.calendar_object_id,summary=excluded.summary,partstat=excluded.partstat,proposed_start=NULL,proposed_end=NULL,updated_at=excluded.updated_at`), NewID(), organizerID, email, res.UserID, o.UID, copies[0].ID, p.Summary, statuses[email], now, now)
		if err != nil {
			return err
		}
	}
	return nil
}

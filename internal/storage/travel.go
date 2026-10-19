package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/travel"
)

type TravelJob struct {
	MessageID, UserID, Token string
	Attempts                 int
}
type TravelCandidate struct {
	ID           string        `json:"id"`
	MessageID    string        `json:"message_id"`
	State        string        `json:"state"`
	Ticket       travel.Ticket `json:"ticket"`
	CreatedAt    int64         `json:"created_at"`
	EventID      string        `json:"event_id"`
	EventETag    string        `json:"event_etag"`
	EventMissing bool          `json:"event_missing"`
}

func (s *Store) TravelMode(ctx context.Context, user string) (string, error) {
	var mode string
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT mode FROM travel_preferences WHERE user_id=?`), user).Scan(&mode)
	if err == sql.ErrNoRows {
		return "off", nil
	}
	return mode, err
}
func (s *Store) SetTravelMode(ctx context.Context, user, mode string) error {
	if mode != "off" && mode != "suggest" && mode != "auto" {
		return fmt.Errorf("invalid travel mode")
	}
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO travel_preferences(user_id,mode) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET mode=excluded.mode`), user, mode)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) queueTravelMessageTx(ctx context.Context, tx *sql.Tx, msg *Message) error {
	// A copy/move may enqueue again; candidate fingerprints and event receipts
	// survive deletion of the original message and prevent duplicated trips.
	_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO travel_jobs(message_id,user_id) SELECT ?,m.user_id FROM mailboxes m JOIN travel_preferences p ON p.user_id=m.user_id WHERE m.id=? AND p.mode<>'off' AND LOWER(m.name) NOT IN ('sent','sent items','drafts','trash','deleted items','junk','spam') ON CONFLICT(message_id) DO NOTHING`), msg.ID, msg.MailboxID)
	return err
}
func (s *Store) ClaimTravelJob(ctx context.Context) (*TravelJob, error) {
	j := &TravelJob{}
	now := time.Now().UnixMilli()
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT j.message_id,j.user_id,j.attempts FROM travel_jobs j JOIN users u ON u.id=j.user_id WHERE j.state='pending' AND j.next_attempt<=? AND j.lease_until<=? AND u.enabled=TRUE ORDER BY j.next_attempt,j.message_id LIMIT 1`), now, now).Scan(&j.MessageID, &j.UserID, &j.Attempts)
	if err != nil {
		return nil, mapErr(err)
	}
	j.Token = NewID()
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE travel_jobs SET lease_token=?,lease_until=?,attempts=attempts+1 WHERE message_id=? AND state='pending' AND next_attempt<=? AND lease_until<=?`), j.Token, now+120000, j.MessageID, now, now)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrNotFound
	}
	j.Attempts++
	return j, nil
}
func (s *Store) FinishTravelJob(ctx context.Context, j *TravelJob, warning string, retry bool) error {
	state := "done"
	if retry && j.Attempts < 5 {
		state = "pending"
	}
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE travel_jobs SET state=?,warning=?,lease_token='',lease_until=0,next_attempt=? WHERE message_id=? AND lease_token=?`), state, warning, time.Now().Add(time.Duration(j.Attempts)*time.Minute).UnixMilli(), j.MessageID, j.Token)
	return err
}
func (s *Store) RecordTravelCandidates(ctx context.Context, user, message string, tickets []*travel.Ticket) ([]TravelCandidate, error) {
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var owner string
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT m.user_id FROM messages x JOIN mailboxes m ON m.id=x.mailbox_id WHERE x.id=?`), message).Scan(&owner)
	if err != nil {
		return nil, mapErr(err)
	}
	if owner != user {
		return nil, ErrUnauthorized
	}
	for i, t := range tickets {
		if i >= 64 {
			break
		}
		data, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO travel_candidates(id,user_id,fingerprint,identity_key,message_id,data,created_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(user_id,fingerprint) DO NOTHING`), NewID(), user, t.Fingerprint(), t.Identity(), message, string(data), time.Now().UnixMilli())
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO travel_sources(candidate_id,message_id) SELECT id,? FROM travel_candidates WHERE user_id=? AND fingerprint=? ON CONFLICT(candidate_id,message_id) DO NOTHING`), message, user, t.Fingerprint())
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	out, _, err := s.ListTravelCandidates(ctx, user, message, 64, 0)
	return out, err
}

const travelColumns = `c.id,c.message_id,c.state,c.data,c.created_at,COALESCE(e.event_id,''),COALESCE(o.etag,''),COALESCE(o.id,'')`
const travelFrom = ` FROM travel_candidates c LEFT JOIN travel_events e ON e.user_id=c.user_id AND e.identity_key=c.identity_key LEFT JOIN calendar_objects o ON o.id=e.event_id AND o.calendar_id IN (SELECT id FROM calendars WHERE user_id=c.user_id)`

func scanTravel(row interface{ Scan(...any) error }) (*TravelCandidate, error) {
	c := &TravelCandidate{}
	var data, existing string
	err := row.Scan(&c.ID, &c.MessageID, &c.State, &data, &c.CreatedAt, &c.EventID, &c.EventETag, &existing)
	if err != nil {
		return nil, mapErr(err)
	}
	if err = json.Unmarshal([]byte(data), &c.Ticket); err != nil {
		return nil, err
	}
	c.EventMissing = c.EventID != "" && existing == ""
	return c, nil
}
func (s *Store) ListTravelCandidates(ctx context.Context, user, message string, limit, offset int) ([]TravelCandidate, int, error) {
	if limit < 1 || limit > 64 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}
	where := ` WHERE c.user_id=?`
	args := []any{user}
	if message != "" {
		where += ` AND EXISTS(SELECT 1 FROM travel_sources ts WHERE ts.candidate_id=c.id AND ts.message_id=?)`
		args = append(args, message)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM travel_candidates c`+where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT `+travelColumns+travelFrom+where+` ORDER BY c.created_at DESC,c.id LIMIT ? OFFSET ?`), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []TravelCandidate{}
	for rows.Next() {
		c, err := scanTravel(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *c)
	}
	return out, total, rows.Err()
}
func (s *Store) GetTravelCandidate(ctx context.Context, user, id string) (*TravelCandidate, error) {
	return scanTravel(s.db.QueryRowContext(ctx, s.rebind(`SELECT `+travelColumns+travelFrom+` WHERE c.user_id=? AND c.id=?`), user, id))
}
func (s *Store) IgnoreTravelCandidate(ctx context.Context, user, id string) error {
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, s.rebind(`UPDATE travel_candidates SET state='ignored' WHERE user_id=? AND id=? AND state='review'`), user, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}

// SaveTravelEvent commits the event, candidate state and replay receipt together.
// Automatic mode never overwrites an existing event. Explicit updates require
// the ETag shown during review and keep user notes, reminders and attachments.
func (s *Store) SaveTravelEvent(ctx context.Context, user, id string, start, end time.Time, etag string, automatic bool) (*CalendarObject, error) {
	if start.IsZero() || !end.After(start) || end.Sub(start) > 31*24*time.Hour {
		return nil, fmt.Errorf("invalid trip interval")
	}
	// Persist instants in UTC on both backends. The ticket retains its separate
	// local wall times and offsets for review and display.
	start, end = start.UTC(), end.UTC()
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	c, err := scanTravel(tx.QueryRowContext(ctx, s.rebind(`SELECT `+travelColumns+travelFrom+` WHERE c.user_id=? AND c.id=?`), user, id))
	if err != nil {
		return nil, err
	}
	var enabled bool
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT enabled FROM users WHERE id=?`), user).Scan(&enabled); err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrUnauthorized
	}
	if c.EventMissing {
		return nil, ErrCalendarConflict
	}
	var o *CalendarObject
	if c.EventID != "" {
		o, err = s.scanCalObject(tx.QueryRowContext(ctx, s.rebind(`SELECT id,calendar_id,uid,href_name,etag,data,size,component,dtstart,dtend,updated_at FROM calendar_objects WHERE id=?`), c.EventID))
		if err != nil {
			return nil, err
		}
		if c.State == "saved" {
			return o, nil
		}
	}
	if c.State != "review" {
		return nil, ErrCalendarConflict
	}
	if automatic {
		var mode string
		err = tx.QueryRowContext(ctx, s.rebind(`SELECT mode FROM travel_preferences WHERE user_id=?`), user).Scan(&mode)
		if err != nil || mode != "auto" || !c.Ticket.Automatic() || o != nil {
			return nil, ErrCalendarConflict
		}
		a, b, e := c.Ticket.Times()
		if e != nil || !start.Equal(a) || !end.Equal(b) {
			return nil, ErrCalendarConflict
		}
	}
	kind := "Flight "
	if c.Ticket.Kind == "train" {
		kind = "Train "
	}
	ev := calutil.Event{UID: NewID() + "@tayga-travel", Summary: kind + c.Ticket.Number + " · " + c.Ticket.Origin + " → " + c.Ticket.Destination, Location: c.Ticket.Origin, Start: &start, End: &end, Status: "CONFIRMED"}
	if o != nil {
		if etag == "" || etag != o.ETag {
			return nil, ErrCalendarConflict
		}
		old, e := calutil.ParseICS(o.Data)
		if e != nil {
			return nil, e
		}
		ev = old.Event
		if ev.Organizer.Email != "" || len(ev.Attendees) != 0 {
			return nil, ErrCalendarConflict
		}
		ev.Start, ev.End = &start, &end
		ev.Sequence++
		o.ExpectedETag = etag
		ev.Description = travelDescription(c, ev.Description)
	} else {
		if c.Ticket.Cancelled {
			return nil, ErrCalendarConflict
		}
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO calendars(id,user_id,name,display_name,ctag,created_at) VALUES(?,?,'personal','Personal calendar',?,?) ON CONFLICT(user_id,name) DO NOTHING`), NewID(), user, newCTag(), time.Now().UTC())
		if err != nil {
			return nil, err
		}
		var calID string
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT id FROM calendars WHERE user_id=? AND name='personal'`), user).Scan(&calID); err != nil {
			return nil, err
		}
		o = &CalendarObject{CalendarID: calID, UID: ev.UID, HrefName: ev.UID + ".ics", Component: "VEVENT", CreateOnly: true}
		ev.Description = travelDescription(c, "")
	}
	if c.Ticket.Cancelled {
		ev.Status = "CANCELLED"
	}
	data, err := calutil.BuildRequestICS(ev)
	if err != nil {
		return nil, err
	}
	// A trip is private calendar data, never a scheduling invitation.
	data = strings.ReplaceAll(data, "METHOD:REQUEST\r\n", "")
	o.Data, o.DTStart, o.DTEnd = data, &start, &end
	if err = s.saveCalendarObjectTx(ctx, tx, o); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO travel_events(user_id,identity_key,event_id) VALUES(?,?,?) ON CONFLICT(user_id,identity_key) DO NOTHING`), user, c.Ticket.Identity(), o.ID)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, s.rebind(`UPDATE travel_candidates SET state='saved' WHERE user_id=? AND id=?`), user, id)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return o, nil
}

func travelDescription(c *TravelCandidate, previous string) string {
	const begin, end = "[Tayga ticket]\n", "\n[/Tayga ticket]"
	data := begin + strings.Join([]string{c.Ticket.Origin + " → " + c.Ticket.Destination, "Passenger: " + c.Ticket.Passenger, "Booking: " + c.Ticket.Booking, "Coach: " + c.Ticket.Coach, "Seat: " + c.Ticket.Seat, "Source: " + c.Ticket.Source, "Message: " + c.MessageID}, "\n") + end
	if a := strings.Index(previous, begin); a >= 0 {
		if b := strings.Index(previous[a+len(begin):], end); b >= 0 {
			return previous[:a] + data + previous[a+len(begin)+b+len(end):]
		}
	}
	if previous != "" {
		return previous + "\n\n" + data
	}
	return data
}

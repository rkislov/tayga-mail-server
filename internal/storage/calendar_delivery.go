package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

// CalendarMail is an immutable scheduling message to one recipient. It is
// inserted in the same transaction as the event/RSVP/decision that produced it.
type CalendarMail struct {
	From, FromName, To, Subject, Text, ICS, Method string
}

type CalendarDelivery struct {
	ID, Sender, Recipient, RecipientUserID, Subject, State, LeaseToken string
	Data                                                               []byte
	Attempts                                                           int
	CreatedAt                                                          int64
}

func CalendarEventMail(from, name string, ev calutil.Event, cancel bool) ([]CalendarMail, error) {
	method, subject, text := "REQUEST", "Invitation: "+ev.Summary, "Meeting: "+ev.Summary+"\n"+calutil.FormatWhen(ev.Start, ev.End)
	var ics string
	var err error
	if cancel {
		method, subject, text = "CANCEL", "Cancelled: "+ev.Summary, "This meeting has been cancelled."
		ics, err = calutil.BuildCancelICS(ev)
	} else {
		ics, err = calutil.BuildRequestICS(ev)
	}
	if err != nil {
		return nil, err
	}
	return calendarAttendeeMail(from, name, ev.Attendees, subject, text, ics, method), nil
}

func calendarAttendeeMail(from, name string, attendees []calutil.Attendee, subject, text, ics, method string) []CalendarMail {
	var out []CalendarMail
	seen := map[string]bool{}
	for _, a := range attendees {
		if a.ScheduleAgent != "" && !strings.EqualFold(a.ScheduleAgent, "SERVER") {
			continue
		}
		to := strings.ToLower(strings.TrimSpace(a.Email))
		if to == "" || seen[to] || strings.EqualFold(to, from) {
			continue
		}
		seen[to] = true
		out = append(out, CalendarMail{From: from, FromName: name, To: to, Subject: subject, Text: text, ICS: ics, Method: method})
	}
	return out
}

func (s *Store) queueCalendarMail(ctx context.Context, tx *sql.Tx, mails []CalendarMail) error {
	for _, m := range mails {
		var err error
		m.ICS, err = calutil.SchedulingMailICS(m.ICS)
		if err != nil {
			return err
		}
		// Booking resources receive calendar objects, not invitation mail.
		if m.Method == "REQUEST" {
			var n int
			if err := tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM calendar_resources WHERE LOWER(email)=LOWER(?) AND enabled=1`), m.To).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				continue
			}
		}
		// Resolve a local identity now: an account deleted before delivery must
		// never turn a previously local invitation into an external email.
		var userID string
		err = tx.QueryRowContext(ctx, s.rebind(`SELECT id FROM users WHERE LOWER(email)=LOWER(?)`), m.To).Scan(&userID)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == sql.ErrNoRows {
			at := strings.LastIndex(m.To, "@")
			if at < 1 {
				return fmt.Errorf("invalid calendar recipient")
			}
			err = tx.QueryRowContext(ctx, s.rebind(`SELECT a.user_id FROM aliases a JOIN domains d ON d.id=a.domain_id WHERE LOWER(d.name)=LOWER(?) AND LOWER(a.local_part)=LOWER(?)`), m.To[at+1:], m.To[:at]).Scan(&userID)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
		}
		if err = s.calendarMailDownloadLinks(ctx, tx, &m); err != nil {
			return err
		}
		id, now := NewID(), time.Now().UTC()
		raw := calutil.IMIPMessage(m.From, m.FromName, m.To, m.Subject, m.Text, m.ICS, m.Method, id, now)
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO calendar_deliveries(id,sender,recipient,recipient_user_id,subject,data,next_attempt,created_at) VALUES(?,?,?,?,?,?,?,?)`), id, m.From, m.To, userID, m.Subject, raw, now.UnixMilli(), now.UnixNano())
		if err != nil {
			return err
		}
	}
	return nil
}

// Claim uses a conditional update on both backends. Only the winner returns
// the job, and an expired worker cannot commit with an old fencing token.
func (s *Store) ClaimCalendarDelivery(ctx context.Context) (*CalendarDelivery, error) {
	now := time.Now().UnixMilli()
	job := &CalendarDelivery{}
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT id,sender,recipient,recipient_user_id,subject,data,state,attempts,created_at FROM calendar_deliveries WHERE state<>'done' AND next_attempt<=? AND lease_until<=? ORDER BY created_at,id LIMIT 1`), now, now).Scan(&job.ID, &job.Sender, &job.Recipient, &job.RecipientUserID, &job.Subject, &job.Data, &job.State, &job.Attempts, &job.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	job.LeaseToken = NewID()
	res, err := s.db.ExecContext(ctx, s.rebind(`UPDATE calendar_deliveries SET lease_token=?,lease_until=?,attempts=attempts+1 WHERE id=? AND state<>'done' AND next_attempt<=? AND lease_until<=?`), job.LeaseToken, now+int64((2*time.Minute)/time.Millisecond), job.ID, now, now)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrNotFound
	}
	job.Attempts++
	return job, nil
}

func (s *Store) RetryCalendarDelivery(ctx context.Context, job *CalendarDelivery, cause string) error {
	backoff := 5 * time.Second
	for i := 1; i < job.Attempts && backoff < time.Hour; i++ {
		backoff *= 2
	}
	if backoff > time.Hour {
		backoff = time.Hour
	}
	if len(cause) > 1024 {
		cause = cause[:1024]
	}
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE calendar_deliveries SET lease_until=0,lease_token='',next_attempt=?,last_error=? WHERE id=? AND lease_token=? AND state<>'done'`), time.Now().Add(backoff).UnixMilli(), cause, job.ID, job.LeaseToken)
	return err
}

// CommitCalendarDelivery atomically hands mail to SMTP or inserts it in the
// local mailbox and records the receipt. Deletion of that message/outbound row
// afterwards does not cause the job to send it again.
func (s *Store) CommitCalendarDelivery(ctx context.Context, job *CalendarDelivery, message *Message) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, s.rebind(`UPDATE calendar_deliveries SET state='notifying' WHERE id=? AND lease_token=? AND state='pending' AND lease_until>?`), job.ID, job.LeaseToken, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCalendarConflict
	}
	if job.RecipientUserID != "" {
		if message == nil {
			return fmt.Errorf("local calendar mail is missing")
		}
		var owner string
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT user_id FROM mailboxes WHERE id=?`), message.MailboxID).Scan(&owner); err != nil {
			return err
		}
		if owner != job.RecipientUserID {
			return ErrUnauthorized
		}
		message.ID = job.ID
		message.MessageID = "<" + job.ID + "@tayga>"
		if _, err = s.insertMessageTx(ctx, tx, message, ""); err != nil {
			return err
		}
	} else {
		now := time.Now().UTC()
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO outbound_queue(id,envelope_from,envelope_to,message_id,data,attempts,max_attempts,next_attempt_at,last_error,created_at,updated_at) VALUES(?,?,?,?,?,0,8,?,'',?,?)`), job.ID, job.Sender, job.Recipient, "<"+job.ID+"@tayga>", job.Data, now, now, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) CompleteCalendarDelivery(ctx context.Context, job *CalendarDelivery) error {
	res, err := s.db.ExecContext(ctx, s.rebind(`UPDATE calendar_deliveries SET state='done',data=?,completed_at=?,lease_until=0,lease_token='',last_error='' WHERE id=? AND lease_token=? AND state='notifying' AND lease_until>?`), []byte{}, time.Now().UnixMilli(), job.ID, job.LeaseToken, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCalendarConflict
	}
	return nil
}

// DeleteCalendarObjectWithMail prevents stale cancellation and queues CANCEL
// in the very transaction that removes the organizer's resource.
func (s *Store) DeleteCalendarObjectWithMail(ctx context.Context, o *CalendarObject, mails []CalendarMail) error {
	return s.deleteCalendarObject(ctx, o, mails, "")
}

func (s *Store) deleteCalendarObject(ctx context.Context, o *CalendarObject, mails []CalendarMail, actorID string) error {
	tx, err := s.beginCalendarWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stored, err := s.scanCalObject(tx.QueryRowContext(ctx, s.rebind(`SELECT id,calendar_id,uid,href_name,etag,data,size,component,dtstart,dtend,updated_at FROM calendar_objects WHERE id=?`), o.ID))
	if err == ErrNotFound {
		return ErrCalendarConflict
	}
	if err != nil {
		return err
	}
	if stored.ETag != o.ETag || stored.CalendarID != o.CalendarID {
		return ErrCalendarConflict
	}
	o = stored
	var actorEmail string
	if actorID != "" {
		if actorEmail, _, err = s.calendarWriterTx(ctx, tx, actorID, o.CalendarID); err != nil {
			return err
		}
	}

	res, err := tx.ExecContext(ctx, s.rebind(`DELETE FROM calendar_objects WHERE id=? AND etag=?`), o.ID, o.ETag)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCalendarConflict
	}
	if p, e := calutil.ParseICS(o.Data); e == nil {
		cancelled := *o
		p.Sequence = calutil.NextSequence(o.Data)
		cancelled.Data, e = calutil.BuildCancelICS(p.Event)
		if e != nil {
			return e
		}
		if actorID == "" || strings.EqualFold(p.Organizer.Email, actorEmail) {
			if e = s.bookCalendarResourcesTx(ctx, tx, &cancelled, o.Data); e != nil {
				return e
			}
		}
		if actorID != "" {
			// Deleting one's received copy must never cancel the organizer's meeting.
			if strings.EqualFold(p.Organizer.Email, actorEmail) {
				mails, e = s.syncCalendarMeetingTx(ctx, tx, actorID, &cancelled, o.Data)
				if e != nil {
					return e
				}
			}
		}
	}

	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE calendars SET ctag=? WHERE id=?`), newCTag(), o.CalendarID); err != nil {
		return err
	}
	if err = s.queueCalendarMail(ctx, tx, mails); err != nil {
		return err
	}
	return tx.Commit()
}

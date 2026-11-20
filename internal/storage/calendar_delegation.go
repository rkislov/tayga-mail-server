package storage

import (
	"context"
	"database/sql"
	"net/mail"
	"strings"
)

type calendarActorKey struct{}

func withCalendarActor(ctx context.Context, actorID string) context.Context {
	if _, exists := ctx.Value(calendarActorKey{}).(string); exists || actorID == "" {
		return ctx
	}
	return context.WithValue(ctx, calendarActorKey{}, actorID)
}

func (s *Store) calendarReplyDelegateTx(ctx context.Context, tx *sql.Tx, actorID string, inv *CalendarInvite) error {
	var n int
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM mailbox_delegates d
 JOIN users owner ON owner.id=d.owner_id JOIN users actor ON actor.id=d.delegate_id
 JOIN calendar_objects o ON o.id=? JOIN calendars c ON c.id=o.calendar_id
 WHERE d.owner_id=? AND d.delegate_id=? AND (d.can_respond_invites OR (d.ews_calendar_permission='Editor' AND c.name='default')) AND c.user_id=owner.id AND c.name<>'personal'
 AND owner.enabled=TRUE AND actor.enabled=TRUE AND owner.tenant_id=actor.tenant_id`), inv.CalendarObjectID, inv.AttendeeUserID, actorID).Scan(&n)
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrUnauthorized
	}
	return nil
}

// The transaction has already authorized the action. Keep the human actor in
// scheduling messages while From and the organizer/attendee remain the owner.
func (s *Store) calendarActorMailTx(ctx context.Context, tx *sql.Tx, from string, raw []byte) ([]byte, error) {
	actorID, _ := ctx.Value(calendarActorKey{}).(string)
	if actorID == "" {
		return raw, nil
	}
	var email, name string
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT email,display_name FROM users WHERE id=? AND enabled=TRUE`), actorID).Scan(&email, &name)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(email, from) {
		return raw, nil
	}
	return append([]byte("Sender: "+(&mail.Address{Name: name, Address: email}).String()+"\r\n"), raw...), nil
}

func (s *Store) calendarOrganizerDelegateTx(ctx context.Context, tx *sql.Tx, actorID, ownerID, objectID string) error {
	var n int
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM mailbox_delegates d
 JOIN users owner ON owner.id=d.owner_id JOIN users actor ON actor.id=d.delegate_id
 JOIN calendar_objects o ON o.id=? JOIN calendars c ON c.id=o.calendar_id
 WHERE d.owner_id=? AND d.delegate_id=? AND (d.can_manage_calendar OR (d.ews_calendar_permission='Editor' AND c.name='default')) AND c.user_id=owner.id AND c.name<>'personal'
 AND owner.enabled=TRUE AND actor.enabled=TRUE AND owner.tenant_id=actor.tenant_id`), objectID, ownerID, actorID).Scan(&n)
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrUnauthorized
	}
	return nil
}

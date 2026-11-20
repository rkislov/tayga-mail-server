package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/tayga/tms/internal/calutil"
	"time"
)

// Queue delegate copies in the same transaction as the owner's scheduling mail.
// These are mail snapshots, not independent calendar invitations or RSVPs.
func (s *Store) queueEWSMeetingCopies(ctx context.Context, tx *sql.Tx, m CalendarMail, ownerID string, raw []byte, now time.Time) error {
	if ownerID == "" || calutil.HasPrivateEvents(m.ICS) {
		return nil
	}
	event, err := calutil.ParseICS(m.ICS)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT actor.id,actor.email,owner.email FROM mailbox_delegates d JOIN users actor ON actor.id=d.delegate_id JOIN users owner ON owner.id=d.owner_id WHERE d.owner_id=? AND d.ews_receive_meetings AND actor.enabled=TRUE AND owner.enabled=TRUE AND actor.tenant_id=owner.tenant_id AND EXISTS (SELECT 1 FROM calendars c JOIN calendar_objects o ON o.calendar_id=c.id WHERE c.user_id=owner.id AND o.uid=? AND c.name<>'personal' AND (d.can_manage_calendar OR (d.ews_calendar_permission='Editor' AND c.name='default')))`), ownerID, event.UID)
	if err != nil {
		return err
	}
	type recipient struct{ id, email, owner string }
	var recipients []recipient
	for rows.Next() {
		var r recipient
		if err = rows.Scan(&r.id, &r.email, &r.owner); err != nil {
			rows.Close()
			return err
		}
		recipients = append(recipients, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range recipients {
		id := NewID()
		origin, _ := json.Marshal(map[string]string{"delegate_for": ownerID})
		data := append([]byte(fmt.Sprintf("X-Tayga-Delegate-For: %s\r\n", r.owner)), raw...)
		if _, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO calendar_deliveries(id,sender,recipient,recipient_user_id,subject,data,next_attempt,created_at,origin_json) VALUES(?,?,?,?,?,?,?,?,?)`), id, m.From, r.email, r.id, m.Subject, data, now.UnixMilli(), now.UnixNano(), string(origin)); err != nil {
			return err
		}
	}
	return nil
}

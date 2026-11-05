// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"database/sql"
	"github.com/tayga/tms/internal/calutil"
	"sort"
	"strings"
	"time"
)

func calendarMailAddress(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

type calendarSentSnapshot struct {
	Mail       CalendarMail
	Recipients []string
}

// Persist one logical REQUEST/CANCEL snapshot per recipient group. Per-recipient
// bearer links stay in delivery messages, never in this organizer snapshot.
func (s *Store) queueCalendarSentCopy(ctx context.Context, tx *sql.Tx, copy calendarSentSnapshot) error {
	m := copy.Mail
	var senderID string
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT id FROM users WHERE LOWER(email)=LOWER(?) AND enabled=TRUE`), m.From).Scan(&senderID); err != nil {
		return err
	}
	sort.Strings(copy.Recipients)
	to := make([]string, 0, len(copy.Recipients))
	seen := map[string]bool{}
	for _, recipient := range copy.Recipients {
		key := calendarMailAddress(recipient)
		if !seen[key] {
			to = append(to, recipient)
			seen[key] = true
		}
	}
	id, now := NewID(), time.Now().UTC()
	raw := calutil.IMIPMessageToMany(m.From, m.FromName, to, m.Subject, m.Text, m.ICS, m.Method, id, now)
	_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO calendar_deliveries(id,sender,recipient,recipient_user_id,subject,data,next_attempt,created_at,target_mailbox) VALUES(?,?,?,?,?,?,?,?,?)`), id, m.From, m.From, senderID, m.Subject, raw, now.UnixMilli(), now.UnixNano(), "Sent")
	return err
}

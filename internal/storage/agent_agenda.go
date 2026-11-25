package storage

import (
	"context"
	"time"
)

// AgendaCandidate carries only the source fields needed to expand an event.
type AgendaCandidate struct {
	CalendarName string
	Data         string
}

func (s *Store) ListOwnAgendaCandidates(ctx context.Context, userID string, from, to time.Time, limit int) ([]AgendaCandidate, error) {
	if userID == "" || limit < 1 || limit > 257 {
		return nil, ErrNotFound
	}
	// Recurrences and detached instances have to be considered even if their
	// master event lies outside the requested period. Oversized records count
	// toward the result boundary but their contents are never read into memory.
	const candidateQuery = `SELECT c.display_name, CASE WHEN o.size <= 131072 THEN o.data ELSE '' END
		FROM calendars c JOIN calendar_objects o ON o.calendar_id = c.id
		WHERE c.user_id = ? AND o.component IN ('', 'VEVENT') AND (
			o.dtstart IS NULL OR (o.dtstart < ? AND COALESCE(o.dtend, o.dtstart) >= ?)
			OR UPPER(o.data) LIKE '%RRULE:%' OR UPPER(o.data) LIKE '%RDATE%'
			OR UPPER(o.data) LIKE '%RECURRENCE-ID%')
		ORDER BY o.dtstart DESC, o.id LIMIT ?`
	rows, err := s.db.QueryContext(ctx, s.rebind(candidateQuery), userID, to, from, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgendaCandidate, 0)
	for rows.Next() {
		var event AgendaCandidate
		if err := rows.Scan(&event.CalendarName, &event.Data); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

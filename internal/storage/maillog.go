package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MailLogEntry is one searchable mail transaction line for admins.
type MailLogEntry struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id,omitempty"`
	Domain    string    `json:"domain,omitempty"`
	Event     string    `json:"event"`
	Direction string    `json:"direction,omitempty"`
	Peer      string    `json:"peer,omitempty"`
	MailFrom  string    `json:"mail_from,omitempty"`
	RcptTo    string    `json:"rcpt_to,omitempty"`
	MessageID string    `json:"message_id,omitempty"`
	Size      int64     `json:"size,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	Line      string    `json:"line"`
	CreatedAt time.Time `json:"created_at"`
}

// MailLogQuery filters admin mail log search.
type MailLogQuery struct {
	ID             string
	MessageID      string
	MailFrom       *string
	RcptTo         *string
	Direction      string
	CorrelationKey string
	CorrelationID  string
	TenantID       string
	Domains        []string // empty = all (global); otherwise restrict to these domain names
	Q              string
	Limit          int
	Offset         int
}

// InsertMailLog stores a log line and updates FTS (sqlite).
func (s *Store) InsertMailLog(ctx context.Context, e *MailLogEntry) error {
	if e == nil {
		return fmt.Errorf("nil mail log")
	}
	if e.ID == "" {
		e.ID = NewID()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if e.Line == "" {
		e.Line = FormatMailLogLine(e)
	}
	e.Domain = strings.ToLower(strings.TrimSpace(e.Domain))
	e.MailFrom = strings.ToLower(strings.TrimSpace(e.MailFrom))
	e.RcptTo = strings.ToLower(strings.TrimSpace(e.RcptTo))

	q := s.rebind(`INSERT INTO mail_log(
		id, tenant_id, domain, event, direction, peer, mail_from, rcpt_to,
		message_id, size, detail, line, created_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	_, err := s.db.ExecContext(ctx, q,
		e.ID, e.TenantID, e.Domain, e.Event, e.Direction, e.Peer, e.MailFrom, e.RcptTo,
		e.MessageID, e.Size, e.Detail, e.Line, e.CreatedAt,
	)
	if err != nil {
		return err
	}
	if s.dialect == DialectSQLite {
		_, _ = s.db.ExecContext(ctx, `INSERT INTO mail_log_fts(log_id, line) VALUES (?, ?)`, e.ID, e.Line)
	}
	return nil
}

// SearchMailLog returns newest matching lines.
func (s *Store) SearchMailLog(ctx context.Context, q MailLogQuery) ([]MailLogEntry, error) {
	if q.Limit <= 0 || q.Limit > 500 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	query := strings.TrimSpace(q.Q)

	base := `SELECT id, tenant_id, domain, event, direction, peer, mail_from, rcpt_to,
        message_id, size, detail, line, created_at FROM mail_log WHERE 1=1`
	args := []any{}
	for _, filter := range []struct{ column, value string }{
		{"tenant_id", q.TenantID}, {"id", q.ID}, {"message_id", q.MessageID}, {"direction", q.Direction},
	} {
		if filter.value != "" {
			base += " AND " + filter.column + " = ?"
			args = append(args, filter.value)
		}
	}
	for _, filter := range []struct {
		column string
		value  *string
	}{{"mail_from", q.MailFrom}, {"rcpt_to", q.RcptTo}} {
		if filter.value != nil {
			base += " AND " + filter.column + " = ?"
			args = append(args, strings.ToLower(strings.TrimSpace(*filter.value)))
		}
	}
	if len(q.Domains) > 0 {
		base += " AND domain IN (" + placeholders(len(q.Domains)) + ")"
		for _, domain := range q.Domains {
			args = append(args, strings.ToLower(domain))
		}
	}
	if q.CorrelationID != "" {
		if (q.CorrelationKey != "transaction_id" && q.CorrelationKey != "queue_id") || !ValidFileSpaceID(q.CorrelationID) {
			return nil, fmt.Errorf("invalid mail log correlation")
		}
		token := q.CorrelationKey + "=" + q.CorrelationID
		base += " AND (detail = ? OR substr(detail, 1, ?) = ?)"
		args = append(args, token, len(token)+1, token+" ")
	}
	var rows *sql.Rows
	var err error
	order := " ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?"
	if s.dialect == DialectSQLite && query != "" {
		if fts := ftsQuery(query); fts != "" {
			ftsArgs := append(append([]any{}, args...), fts, q.Limit, q.Offset)
			rows, err = s.db.QueryContext(ctx, base+" AND id IN (SELECT log_id FROM mail_log_fts WHERE mail_log_fts MATCH ?)"+order, ftsArgs...)
			if err != nil {
				rows = nil
			}
		}
	}
	if rows == nil {
		if query != "" {
			if s.dialect == DialectPostgres {
				base += " AND line ILIKE ?"
			} else {
				base += " AND lower(line) LIKE lower(?)"
			}
			args = append(args, "%"+query+"%")
		}
		args = append(args, q.Limit, q.Offset)
		rows, err = s.db.QueryContext(ctx, s.rebind(base+order), args...)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var out []MailLogEntry
	for rows.Next() {
		var e MailLogEntry
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.Domain, &e.Event, &e.Direction, &e.Peer, &e.MailFrom, &e.RcptTo,
			&e.MessageID, &e.Size, &e.Detail, &e.Line, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func placeholders(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func ftsQuery(q string) string {
	fields := strings.Fields(q)
	var parts []string
	for _, f := range fields {
		cleaned := strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				return r
			case r == '@' || r == '.' || r == '-' || r == '_' || r == '+':
				return r
			default:
				return -1
			}
		}, f)
		if cleaned == "" {
			continue
		}
		parts = append(parts, cleaned+"*")
	}
	return strings.Join(parts, " ")
}

// FormatMailLogLine builds a classic one-line mail log string.
func FormatMailLogLine(e *MailLogEntry) string {
	if e == nil {
		return ""
	}
	ts := e.CreatedAt.UTC().Format(time.RFC3339)
	parts := []string{ts, e.Event}
	if e.Direction != "" {
		parts = append(parts, e.Direction)
	}
	if e.Peer != "" {
		parts = append(parts, "peer="+e.Peer)
	}
	if e.MailFrom != "" {
		parts = append(parts, "from="+e.MailFrom)
	}
	if e.RcptTo != "" {
		parts = append(parts, "to="+e.RcptTo)
	}
	if e.MessageID != "" {
		parts = append(parts, "msgid="+e.MessageID)
	}
	if e.Size > 0 {
		parts = append(parts, fmt.Sprintf("size=%d", e.Size))
	}
	if e.Detail != "" {
		parts = append(parts, e.Detail)
	}
	return strings.Join(parts, " ")
}

// DomainOfEmail returns the domain part of an address.
func DomainOfEmail(addr string) string {
	addr = strings.ToLower(strings.Trim(addr, "<> "))
	at := strings.LastIndex(addr, "@")
	if at < 0 || at+1 >= len(addr) {
		return ""
	}
	return addr[at+1:]
}

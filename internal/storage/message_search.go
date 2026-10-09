package storage

import (
	"context"
	"fmt"
	"strings"
)

type MessageSearchOptions struct {
	Offset    int
	Sort      string
	Ascending bool
	Filter    string
}

// SearchHit is one ranked search result with optional FTS snippet.
type SearchHit struct {
	Message     *Message
	MailboxName string
	Snippet     string
	Rank        float64
}

// UpsertMessageSearch indexes or refreshes FTS document for a message.
func (s *Store) UpsertMessageSearch(ctx context.Context, messageID, subject, fromAddr, toAddr, body string) error {
	if s.dialect == DialectPostgres {
		q := `INSERT INTO message_search(message_id, subject, from_addr, to_addr, body, tsv)
			VALUES ($1, $2, $3, $4, $5, to_tsvector('simple', coalesce($2,'') || ' ' || coalesce($3,'') || ' ' || coalesce($4,'') || ' ' || coalesce($5,'')))
			ON CONFLICT (message_id) DO UPDATE SET
				subject = EXCLUDED.subject,
				from_addr = EXCLUDED.from_addr,
				to_addr = EXCLUDED.to_addr,
				body = EXCLUDED.body,
				tsv = EXCLUDED.tsv`
		_, err := s.db.ExecContext(ctx, q, messageID, subject, fromAddr, toAddr, body)
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM message_fts WHERE message_id = ?`, messageID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO message_fts(message_id, subject, from_addr, to_addr, body) VALUES (?, ?, ?, ?, ?)`,
		messageID, subject, fromAddr, toAddr, body)
	return err
}

// DeleteMessageSearch removes FTS rows for a message.
func (s *Store) DeleteMessageSearch(ctx context.Context, messageID string) error {
	if s.dialect == DialectPostgres {
		_, err := s.db.ExecContext(ctx, `DELETE FROM message_search WHERE message_id = $1`, messageID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM message_fts WHERE message_id = ?`, messageID)
	return err
}

// SearchMessages runs hybrid header LIKE + FTS over the user's mailboxes.
func (s *Store) SearchMessages(ctx context.Context, userID, mailboxID, fromFilter, toFilter, subjectFilter, ftsQuery string, limit int, options ...MessageSearchOptions) ([]SearchHit, error) {
	if limit <= 0 || limit > 201 {
		limit = 50
	}
	option := MessageSearchOptions{}
	if len(options) > 0 {
		option = options[0]
	}
	if option.Offset < 0 {
		option.Offset = 0
	}
	fromFilter = strings.TrimSpace(fromFilter)
	toFilter = strings.TrimSpace(toFilter)
	subjectFilter = strings.TrimSpace(subjectFilter)
	ftsQuery = strings.TrimSpace(ftsQuery)

	if s.dialect == DialectPostgres {
		return s.searchPostgres(ctx, userID, mailboxID, fromFilter, toFilter, subjectFilter, ftsQuery, limit, option)
	}
	return s.searchSQLite(ctx, userID, mailboxID, fromFilter, toFilter, subjectFilter, ftsQuery, limit, option)
}

func (s *Store) searchSQLite(ctx context.Context, userID, mailboxID, fromF, toF, subF, fts string, limit int, option MessageSearchOptions) ([]SearchHit, error) {
	var args []any
	var b strings.Builder
	b.WriteString(`SELECT m.id, m.mailbox_id, m.uid, m.size, m.flags, m.internal_date, m.file_path, m.message_id,
		m.subject, m.from_addr, m.to_addr, m.date_hdr, m.archived, m.created_at, mb.name,`)
	if fts != "" {
		b.WriteString(` snippet(message_fts, 3, '', '', '…', 24) AS snip, bm25(message_fts) AS rank`)
		b.WriteString(` FROM message_fts JOIN messages m ON m.id = message_fts.message_id`)
	} else {
		b.WriteString(` '' AS snip, 0.0 AS rank FROM messages m`)
	}
	b.WriteString(` JOIN mailboxes mb ON mb.id = m.mailbox_id WHERE mb.user_id = ?`)
	args = append(args, userID)
	if fts != "" {
		b.WriteString(` AND message_fts MATCH ?`)
		args = append(args, fts)
	}
	if option.Filter == "unread" {
		b.WriteString(` AND upper(m.flags) NOT LIKE '%\SEEN%'`)
	}
	if option.Filter == "flagged" {
		b.WriteString(` AND upper(m.flags) LIKE '%\FLAGGED%'`)
	}
	if mailboxID != "" {
		b.WriteString(` AND m.mailbox_id = ?`)
		args = append(args, mailboxID)
	}
	if fromF != "" {
		b.WriteString(` AND lower(m.from_addr) LIKE ?`)
		args = append(args, "%"+strings.ToLower(fromF)+"%")
	}
	if toF != "" {
		b.WriteString(` AND lower(m.to_addr) LIKE ?`)
		args = append(args, "%"+strings.ToLower(toF)+"%")
	}
	if subF != "" {
		b.WriteString(` AND lower(m.subject) LIKE ?`)
		args = append(args, "%"+strings.ToLower(subF)+"%")
	}
	if field := map[string]string{"from": "lower(m.from_addr)", "subject": "lower(m.subject)", "date": "m.internal_date"}[option.Sort]; field != "" {
		direction := " DESC"
		if option.Ascending {
			direction = " ASC"
		}
		b.WriteString(" ORDER BY " + field + direction + ", m.id")
	} else if fts != "" {
		b.WriteString(" ORDER BY rank" + ", m.id")
	} else {
		b.WriteString(" ORDER BY m.internal_date DESC, m.id")
	}
	b.WriteString(` LIMIT ? OFFSET ?`)
	args = append(args, limit, option.Offset)

	rows, err := s.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSearchHits(rows)
}

func (s *Store) searchPostgres(ctx context.Context, userID, mailboxID, fromF, toF, subF, fts string, limit int, option MessageSearchOptions) ([]SearchHit, error) {
	var args []any
	n := 1
	ph := func() string {
		p := fmt.Sprintf("$%d", n)
		n++
		return p
	}
	var b strings.Builder
	b.WriteString(`SELECT m.id, m.mailbox_id, m.uid, m.size, m.flags, m.internal_date, m.file_path, m.message_id,
		m.subject, m.from_addr, m.to_addr, m.date_hdr, m.archived, m.created_at, mb.name,`)
	if fts != "" {
		b.WriteString(` ts_headline('simple', coalesce(ms.body,''), plainto_tsquery('simple', ` + ph() + `), 'MaxWords=24, MinWords=8') AS snip,`)
		args = append(args, fts)
		b.WriteString(` ts_rank_cd(ms.tsv, plainto_tsquery('simple', ` + ph() + `)) AS rank`)
		args = append(args, fts)
		b.WriteString(` FROM messages m JOIN message_search ms ON ms.message_id = m.id`)
	} else {
		b.WriteString(` '' AS snip, 0.0::float8 AS rank FROM messages m`)
	}
	b.WriteString(` JOIN mailboxes mb ON mb.id = m.mailbox_id WHERE mb.user_id = ` + ph())
	args = append(args, userID)
	if fts != "" {
		b.WriteString(` AND ms.tsv @@ plainto_tsquery('simple', ` + ph() + `)`)
		args = append(args, fts)
	}
	if option.Filter == "unread" {
		b.WriteString(` AND upper(m.flags) NOT LIKE '%\SEEN%'`)
	}
	if option.Filter == "flagged" {
		b.WriteString(` AND upper(m.flags) LIKE '%\FLAGGED%'`)
	}
	if mailboxID != "" {
		b.WriteString(` AND m.mailbox_id = ` + ph())
		args = append(args, mailboxID)
	}
	if fromF != "" {
		b.WriteString(` AND lower(m.from_addr) LIKE ` + ph())
		args = append(args, "%"+strings.ToLower(fromF)+"%")
	}
	if toF != "" {
		b.WriteString(` AND lower(m.to_addr) LIKE ` + ph())
		args = append(args, "%"+strings.ToLower(toF)+"%")
	}
	if subF != "" {
		b.WriteString(` AND lower(m.subject) LIKE ` + ph())
		args = append(args, "%"+strings.ToLower(subF)+"%")
	}
	if field := map[string]string{"from": "lower(m.from_addr)", "subject": "lower(m.subject)", "date": "m.internal_date"}[option.Sort]; field != "" {
		direction := " DESC"
		if option.Ascending {
			direction = " ASC"
		}
		b.WriteString(" ORDER BY " + field + direction + ", m.id")
	} else if fts != "" {
		b.WriteString(" ORDER BY rank" + " DESC" + ", m.id")
	} else {
		b.WriteString(" ORDER BY m.internal_date DESC, m.id")
	}
	b.WriteString(` LIMIT ` + ph() + ` OFFSET ` + ph())
	args = append(args, limit, option.Offset)

	rows, err := s.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSearchHits(rows)
}

func scanSearchHits(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]SearchHit, error) {
	var out []SearchHit
	for rows.Next() {
		m := &Message{}
		var archived int
		var mailboxName string
		var snip string
		var rank float64
		if err := rows.Scan(
			&m.ID, &m.MailboxID, &m.UID, &m.Size, &m.Flags, sqlTime{&m.InternalDate}, &m.FilePath, &m.MessageID,
			&m.Subject, &m.FromAddr, &m.ToAddr, &m.DateHdr, &archived, &m.CreatedAt, &mailboxName,
			&snip, &rank,
		); err != nil {
			return nil, err
		}
		m.Archived = archived != 0
		out = append(out, SearchHit{Message: m, MailboxName: mailboxName, Snippet: snip, Rank: rank})
	}
	return out, rows.Err()
}

package xmpp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"
)

var ErrRoomAccess = errors.New("group unavailable")

// ErrRoomCursor means a cursor is absent from this room, including foreign-room IDs.
var ErrRoomCursor = errMAMCursor

const roomStampLayout = "2006-01-02T15:04:05.000000000Z"

type RoomArchivePage struct {
	Messages []HistoryMsg `json:"messages"`
	Total    int          `json:"total"`
	Complete bool         `json:"complete"`
	First    string       `json:"first,omitempty"`
	Last     string       `json:"last,omitempty"`
}

// roomArchiveSequence locks the room counter and repairs rows written by an older
// binary during rollback. Call in a transaction, with mucMu held. Sequence numbers
// preserve receive order even when timestamps tie or the system clock moves back.
func (s *Server) roomArchiveSequence(ctx context.Context, tx *sql.Tx, room string) (int64, error) {
	var seq int64
	if err := tx.QueryRowContext(ctx, s.rebind(`UPDATE web_chat_rooms SET next_archive_seq=next_archive_seq WHERE jid=? RETURNING next_archive_seq`), room).Scan(&seq); err != nil {
		return 0, err
	}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT id,created_at,sender FROM web_chat_messages WHERE room_jid=? AND archive_seq=0 ORDER BY created_at,id`), room)
	if err != nil {
		return 0, err
	}
	type legacy struct {
		id, stamp, sender string
		at                time.Time
	}
	var old []legacy
	for rows.Next() {
		var r legacy
		if err = rows.Scan(&r.id, &r.stamp, &r.sender); err != nil {
			rows.Close()
			return 0, err
		}
		r.at, err = time.Parse(time.RFC3339Nano, r.stamp)
		if err != nil {
			rows.Close()
			return 0, err
		}
		old = append(old, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	// Canonical timestamps are needed for date filtering; sort by actual time when
	// an old binary produced RFC3339 timestamps with differing fractional precision.
	sort.Slice(old, func(i, j int) bool {
		if old[i].at.Equal(old[j].at) {
			return old[i].id < old[j].id
		}
		return old[i].at.Before(old[j].at)
	})
	for _, r := range old {
		seq++
		if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE web_chat_messages SET archive_seq=?,created_at=?,sender_jid=? WHERE id=?`), seq, r.at.UTC().Format(roomStampLayout), r.sender, r.id); err != nil {
			return 0, err
		}
	}
	if len(old) > 0 {
		_, err = tx.ExecContext(ctx, s.rebind(`UPDATE web_chat_rooms SET next_archive_seq=? WHERE jid=?`), seq, room)
	}
	return seq, err
}

// RoomHistoryPage returns older messages in chronological order. Empty before
// starts at the newest page. Membership is checked anew, not cached at login.
func (s *Server) RoomHistoryPage(ctx context.Context, email, room, before string, limit int) (RoomArchivePage, error) {
	s.mucMu.Lock()
	defer s.mucMu.Unlock()
	if !s.isWebRoom(room) || ParseJID(room).Resource != "" {
		return RoomArchivePage{}, ErrRoomAccess
	}
	if limit < 1 || limit > 100 {
		limit = 80
	}
	return s.roomArchivePage(ctx, email, room, mamFilter{Before: &before, Limit: limit})
}

// Caller holds mucMu while taking the page snapshot. The native MAM handler
// keeps it through delivery; current members can read the full retained archive.
func (s *Server) roomArchivePage(ctx context.Context, email, room string, f mamFilter) (RoomArchivePage, error) {
	p := RoomArchivePage{Messages: []HistoryMsg{}}
	if err := s.requireRoomMember(ctx, email, room); err != nil {
		return p, err
	}
	var legacy bool
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT EXISTS(SELECT 1 FROM web_chat_messages WHERE room_jid=? AND archive_seq=0)`), room).Scan(&legacy); err != nil {
		return p, err
	}
	if legacy {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return p, err
		}
		defer tx.Rollback()
		if _, err = s.roomArchiveSequence(ctx, tx, room); err != nil {
			return p, err
		}
		if err = tx.Commit(); err != nil {
			return p, err
		}
	}
	where := "room_jid=? AND archive_seq>0"
	args := []any{room}
	if f.With != "" {
		if ParseJID(f.With).Resource != "" {
			where += " AND sender_jid=?"
		} else {
			where += " AND sender=?"
		}
		args = append(args, f.With)
	}
	if f.Start != nil {
		where += " AND created_at>=?"
		args = append(args, f.Start.UTC().Format(roomStampLayout))
	}
	if f.End != nil {
		where += " AND created_at<=?"
		args = append(args, f.End.UTC().Format(roomStampLayout))
	}
	if err := s.db.QueryRowContext(ctx, s.rebind("SELECT COUNT(*) FROM web_chat_messages WHERE "+where), args...).Scan(&p.Total); err != nil {
		return p, err
	}
	reverse := f.Before != nil
	cursor := f.After
	op, order := ">", "ASC"
	if reverse {
		cursor = f.Before
		op, order = "<", "DESC"
	}
	if cursor != nil && *cursor != "" {
		var seq int64
		err := s.db.QueryRowContext(ctx, s.rebind(`SELECT archive_seq FROM web_chat_messages WHERE room_jid=? AND id=?`), room, *cursor).Scan(&seq)
		if errors.Is(err, sql.ErrNoRows) {
			return p, ErrRoomCursor
		}
		if err != nil {
			return p, err
		}
		where += " AND archive_seq" + op + "?"
		args = append(args, seq)
	}
	args = append(args, f.Limit+1)
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id,sender,body,created_at,nick,origin_id,sender_jid FROM web_chat_messages WHERE `+where+` ORDER BY archive_seq `+order+` LIMIT ?`), args...)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var m HistoryMsg
		var at string
		if err = rows.Scan(&m.ID, &m.From, &m.Body, &at, &m.Nick, &m.OriginID, &m.SenderJID); err != nil {
			return p, err
		}
		m.To = room
		m.At, err = time.Parse(time.RFC3339Nano, at)
		if err != nil {
			return p, fmt.Errorf("invalid room timestamp: %w", err)
		}
		if m.Nick == "" {
			m.Nick = m.From
		}
		if m.SenderJID == "" {
			m.SenderJID = m.From
		}
		p.Messages = append(p.Messages, m)
	}
	if err = rows.Err(); err != nil {
		return p, err
	}
	p.Complete = len(p.Messages) <= f.Limit
	if !p.Complete {
		p.Messages = p.Messages[:f.Limit]
	}
	if reverse {
		for i, j := 0, len(p.Messages)-1; i < j; i, j = i+1, j-1 {
			p.Messages[i], p.Messages[j] = p.Messages[j], p.Messages[i]
		}
	}
	if len(p.Messages) > 0 {
		p.First = p.Messages[0].ID
		p.Last = p.Messages[len(p.Messages)-1].ID
	}
	return p, nil
}

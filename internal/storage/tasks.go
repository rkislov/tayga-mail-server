package storage

import (
	"context"
	"database/sql"
	"strings"
)

type Task struct {
	ID              string `json:"id"`
	UserID          string `json:"-"`
	SourceMessageID string `json:"source_message_id,omitempty"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	Completed       bool   `json:"completed"`
	Due             string `json:"due"`
	Revision        int64  `json:"revision"`
}

// syncMessageTask runs in the same transaction as a flag change. Reflagging
// reopens the original task; clearing the flag completes it.
func (s *Store) syncMessageTask(ctx context.Context, tx *sql.Tx, id, flags string) error {
	if HasFlag(flags, `\Flagged`) {
		_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO tasks(id,user_id,source_message_id,title) SELECT ?,b.user_id,m.id,m.subject FROM messages m JOIN mailboxes b ON b.id=m.mailbox_id WHERE m.id=? ON CONFLICT(source_message_id) DO UPDATE SET completed=FALSE,revision=tasks.revision+1 WHERE tasks.completed=TRUE`), "mailtask:"+id, id)
		return err
	}
	_, err := tx.ExecContext(ctx, s.rebind(`UPDATE tasks SET completed=TRUE,revision=revision+1 WHERE source_message_id=? AND completed=FALSE`), id)
	return err
}

func (s *Store) ListTasks(ctx context.Context, user string) ([]Task, error) {
	// Reconcile messages inserted with a flag (e.g. IMAP APPEND or migration), too.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT m.id,m.flags FROM messages m JOIN mailboxes b ON b.id=m.mailbox_id WHERE b.user_id=? AND (m.flags LIKE ? OR m.id IN (SELECT source_message_id FROM tasks WHERE user_id=?))`), user, `%\Flagged%`, user)
	if err != nil {
		return nil, err
	}
	pairs := [][2]string{}
	for rows.Next() {
		var p [2]string
		if err = rows.Scan(&p[0], &p[1]); err != nil {
			rows.Close()
			return nil, err
		}
		pairs = append(pairs, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, p := range pairs {
		if err = s.syncMessageTask(ctx, tx, p[0], p[1]); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	rows, err = s.db.QueryContext(ctx, s.rebind(`SELECT id,user_id,COALESCE(source_message_id,''),title,body,completed,due,revision FROM tasks WHERE user_id=? ORDER BY completed,due,title,id`), user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var t Task
		if err = rows.Scan(&t.ID, &t.UserID, &t.SourceMessageID, &t.Title, &t.Body, &t.Completed, &t.Due, &t.Revision); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) SaveTask(ctx context.Context, user string, t *Task) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if t.ID == "" {
		t.ID = "task:" + NewID()
		t.Revision = 1
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO tasks(id,user_id,title,body,completed,due) VALUES(?,?,?,?,?,?)`), t.ID, user, t.Title, t.Body, t.Completed, t.Due)
	} else {
		var source sql.NullString
		err = tx.QueryRowContext(ctx, s.rebind(`SELECT source_message_id FROM tasks WHERE id=? AND user_id=?`), t.ID, user).Scan(&source)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		res, e := tx.ExecContext(ctx, s.rebind(`UPDATE tasks SET title=?,body=?,completed=?,due=?,revision=revision+1 WHERE id=? AND user_id=? AND revision=?`), t.Title, t.Body, t.Completed, t.Due, t.ID, user, t.Revision)
		if e != nil {
			return e
		}
		n, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			return ErrNotFound
		}
		if source.Valid {
			if err = s.setTaskMailFlag(ctx, tx, source.String, !t.Completed); err != nil {
				return err
			}
		}
		t.Revision++
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) setTaskMailFlag(ctx context.Context, tx *sql.Tx, id string, flag bool) error {
	var flags string
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT flags FROM messages WHERE id=?`), id).Scan(&flags); err != nil {
		return err
	}
	out := []string{}
	for _, f := range strings.Fields(flags) {
		if !strings.EqualFold(f, `\Flagged`) {
			out = append(out, f)
		}
	}
	if flag {
		out = append(out, `\Flagged`)
	}
	_, err := tx.ExecContext(ctx, s.rebind(`UPDATE messages SET flags=? WHERE id=?`), strings.Join(out, " "), id)
	return err
}
func (s *Store) DeleteTask(ctx context.Context, user, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var source sql.NullString
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT source_message_id FROM tasks WHERE id=? AND user_id=?`), id, user).Scan(&source)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if source.Valid {
		if err = s.setTaskMailFlag(ctx, tx, source.String, false); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM tasks WHERE id=? AND user_id=?`), id, user)
	if err != nil {
		return err
	}
	return tx.Commit()
}

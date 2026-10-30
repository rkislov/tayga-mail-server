package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

type Task struct {
	Recurring          bool    `json:"recurring"`
	FullTaskResource   bool    `json:"-"` // Internal full-series write, never accepted from HTTP JSON.
	OwnerID            string  `json:"owner_id"`
	OwnerName          string  `json:"owner_name"`
	OwnerEmail         string  `json:"owner_email"`
	AssigneeID         string  `json:"assignee_id"`
	AssigneeName       string  `json:"assignee_name"`
	AssigneeEmail      string  `json:"assignee_email"`
	AssignmentState    string  `json:"assignment_state"`
	AssignmentRevision int64   `json:"-"`
	CanManage          bool    `json:"can_manage"`
	CanUpdate          bool    `json:"can_update"`
	CompletedAt        string  `json:"-"`
	EWSProperties      string  `json:"-"`
	ID                 string  `json:"id"`
	UserID             string  `json:"-"`
	SourceMessageID    string  `json:"source_message_id,omitempty"`
	Title              string  `json:"title"`
	Body               string  `json:"body"`
	Completed          bool    `json:"completed"`
	Status             string  `json:"status"`
	PercentComplete    float64 `json:"percent_complete"`
	Due                string  `json:"due"`
	Revision           int64   `json:"revision"`
}

const (
	TaskNotStarted      = "NotStarted"
	TaskInProgress      = "InProgress"
	TaskCompleted       = "Completed"
	TaskWaitingOnOthers = "WaitingOnOthers"
	TaskDeferred        = "Deferred"
)

var ErrInvalidTaskProgress = errors.New("invalid task status or percentage")
var ErrTaskSeries = errors.New("recurring tasks must be edited in Thunderbird; assignment is not supported")

func taskIsRecurring(raw string) bool {
	var props map[string]string
	return json.Unmarshal([]byte(raw), &props) == nil && props["_tayga:Recurring"] == "1"
}

var ErrTaskConflict = errors.New("task revision changed")

// CreateEWSTask records the create receipt atomically. A lost response can be
// retried without duplicating, overwriting, or resurrecting a task.
func (s *Store) CreateEWSTask(ctx context.Context, user, createID string, t *Task) (string, error) {
	if createID == "" || len(createID) > 256 || t.ID != "" {
		return "", ErrUnauthorized
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	id := "task:" + NewID()
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO flowsync_states(user_id,client_id,collection_id,state) VALUES(?,'ews-tasks',?,?) ON CONFLICT(user_id,client_id,collection_id) DO NOTHING`), user, createID, id)
	if err != nil {
		return "", err
	}
	var saved string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT state FROM flowsync_states WHERE user_id=? AND client_id='ews-tasks' AND collection_id=?`), user, createID).Scan(&saved); err != nil {
		return "", err
	}
	if saved == id {
		if err = s.insertTask(ctx, tx, user, id, t); err != nil {
			return "", err
		}
	}
	return saved, tx.Commit()
}

// SetStatus and SetPercentComplete apply ordered EWS property updates. In
// particular, a later status can reopen a task completed by an earlier field.
func (t *Task) SetStatus(status string) error {
	switch status {
	case TaskNotStarted:
		t.PercentComplete = 0
	case TaskCompleted:
		t.PercentComplete = 100
	case TaskInProgress, TaskWaitingOnOthers, TaskDeferred:
		if t.PercentComplete == 100 {
			t.PercentComplete = 0
		}
	default:
		return ErrInvalidTaskProgress
	}
	t.Status, t.Completed = status, status == TaskCompleted
	return nil
}

func (t *Task) SetPercentComplete(percent float64) error {
	if math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 0 || percent > 100 {
		return ErrInvalidTaskProgress
	}
	t.PercentComplete = percent
	if percent == 100 {
		t.Status = TaskCompleted
	} else if t.Status == "" || t.Status == TaskCompleted || t.Status == TaskNotStarted {
		t.Status = TaskNotStarted
		if percent > 0 {
			t.Status = TaskInProgress
		}
	}
	t.Completed = t.Status == TaskCompleted
	return nil
}

func (t *Task) normalizeProgress(previous *Task) error {
	if math.IsNaN(t.PercentComplete) || math.IsInf(t.PercentComplete, 0) || t.PercentComplete < 0 || t.PercentComplete > 100 {
		return ErrInvalidTaskProgress
	}
	// Old clients only know completed. A title/body edit must not erase progress;
	// a checkbox change must still work, including GET/spread/PUT clients.
	if t.Status == "" {
		if previous != nil && t.Completed == previous.Completed {
			t.Status, t.PercentComplete = previous.Status, previous.PercentComplete
		} else if t.Completed {
			return t.SetStatus(TaskCompleted)
		} else {
			return t.SetStatus(TaskNotStarted)
		}
	} else if previous != nil && t.Status == previous.Status && t.PercentComplete == previous.PercentComplete && t.Completed != previous.Completed {
		if t.Completed {
			return t.SetStatus(TaskCompleted)
		}
		return t.SetStatus(TaskNotStarted)
	}
	switch t.Status {
	case TaskInProgress, TaskWaitingOnOthers, TaskDeferred:
		if t.PercentComplete == 100 {
			return t.SetStatus(TaskCompleted)
		}
	}
	return t.SetStatus(t.Status)
}

func taskPropertiesWithoutCompletion(raw string) (string, error) {
	props := map[string]string{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &props); err != nil {
			return "", err
		}
	}
	delete(props, "task:CompleteDate")
	data, err := json.Marshal(props)
	return string(data), err
}

// Keep the first completion time stable for ActiveSync, independently of an
// optional EWS CompleteDate property (which EWS clients may explicitly delete).
func taskPropertiesForSave(t *Task) error {
	if t.EWSProperties == "" {
		t.EWSProperties = "{}"
	}
	if t.Completed {
		if t.CompletedAt == "" {
			t.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		return nil
	}
	t.CompletedAt = ""
	var err error
	t.EWSProperties, err = taskPropertiesWithoutCompletion(t.EWSProperties)
	return err
}

// syncMessageTask runs in the same transaction as a flag change. Reflagging
// reopens the original task; clearing the flag completes it.
func (s *Store) syncMessageTask(ctx context.Context, tx *sql.Tx, id, flags string) error {
	query := taskSelect + `WHERE t.source_message_id=?`
	if s.dialect == DialectPostgres {
		query += ` FOR UPDATE OF t`
	}
	before, err := scanTask(tx.QueryRowContext(ctx, s.rebind(query), id), "")
	if err != nil && err != ErrNotFound {
		return err
	}
	flagged := HasFlag(flags, `\Flagged`)
	if (before == nil && !flagged) || (before != nil && before.Completed != flagged) {
		return nil
	}
	var res sql.Result
	if flagged {
		// Remove only the obsolete completion date atomically, so a concurrent
		// metadata edit cannot be overwritten by a previously read snapshot.
		properties := `json_remove(tasks.ews_properties, '$."task:CompleteDate"')`
		if s.dialect == DialectPostgres {
			properties = `(tasks.ews_properties::jsonb - 'task:CompleteDate')::text`
		}
		res, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO tasks(id,user_id,source_message_id,title) SELECT ?,b.user_id,m.id,m.subject FROM messages m JOIN mailboxes b ON b.id=m.mailbox_id WHERE m.id=? ON CONFLICT(source_message_id) DO UPDATE SET completed=FALSE,completed_at='',status='NotStarted',percent_complete=0,ews_properties=`+properties+`,revision=tasks.revision+1 WHERE tasks.completed=TRUE`), "mailtask:"+id, id)
	} else {
		res, err = tx.ExecContext(ctx, s.rebind(`UPDATE tasks SET completed=TRUE,completed_at=?,status='Completed',percent_complete=100,revision=revision+1 WHERE source_message_id=? AND completed=FALSE`), time.Now().UTC().Format(time.RFC3339Nano), id)
	}
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil || n == 0 {
		return err
	}
	after, err := scanTask(tx.QueryRowContext(ctx, s.rebind(taskSelect+`WHERE t.source_message_id=?`), id), "")
	if err != nil {
		return err
	}
	if before == nil {
		return s.recordTaskEvent(ctx, tx, after.UserID, after, "create", []string{"task"})
	}
	return s.recordTaskUpdate(ctx, tx, after.UserID, before, after)
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
	rows, err = s.db.QueryContext(ctx, s.rebind(taskSelect+`WHERE `+taskAccess+` ORDER BY t.completed,t.due,t.title,t.id`), user, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, e := scanTask(rows, user)
		if e != nil {
			return nil, e
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (s *Store) SaveTask(ctx context.Context, user string, t *Task) error {
	if t.ID == "" && t.EWSProperties == "" {
		t.EWSProperties = "{}"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if t.ID == "" {
		err = s.insertTask(ctx, tx, user, "task:"+NewID(), t)
	} else {
		previous, e := s.taskInTx(ctx, tx, user, t.ID)
		if e != nil {
			return e
		}
		if (previous.Recurring || taskIsRecurring(t.EWSProperties)) && (!t.FullTaskResource || previous.AssigneeID != "" || previous.SourceMessageID != "") {
			return ErrTaskSeries
		}
		if !previous.CanUpdate {
			return ErrUnauthorized
		}
		t.UserID, t.OwnerID = previous.UserID, previous.UserID
		t.AssigneeID, t.AssignmentState, t.AssignmentRevision = previous.AssigneeID, previous.AssignmentState, previous.AssignmentRevision
		t.SourceMessageID = previous.SourceMessageID
		t.OwnerName, t.OwnerEmail = previous.OwnerName, previous.OwnerEmail
		t.AssigneeName, t.AssigneeEmail = previous.AssigneeName, previous.AssigneeEmail
		t.CanManage, t.CanUpdate = previous.CanManage, previous.CanUpdate
		if err = t.normalizeProgress(previous); err != nil {
			return err
		}
		if t.EWSProperties == "" {
			t.EWSProperties = previous.EWSProperties
		}
		if !previous.CanManage && !taskAssigneeEditAllowed(previous, t) {
			return ErrUnauthorized
		}
		t.CompletedAt = previous.CompletedAt
		if err = taskPropertiesForSave(t); err != nil {
			return err
		}
		res, e := tx.ExecContext(ctx, s.rebind(`UPDATE tasks SET title=?,body=?,completed=?,status=?,percent_complete=?,due=?,ews_properties=?,completed_at=?,revision=revision+1 WHERE id=? AND user_id=? AND revision=?`), t.Title, t.Body, t.Completed, t.Status, t.PercentComplete, t.Due, t.EWSProperties, t.CompletedAt, t.ID, previous.UserID, t.Revision)
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
		// Fetch the linked message internally; it is intentionally absent from
		// the assignee's public task view.
		var source sql.NullString
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT source_message_id FROM tasks WHERE id=?`), t.ID).Scan(&source); err != nil {
			return err
		}
		if source.Valid {
			if err = s.setTaskMailFlag(ctx, tx, source.String, !t.Completed); err != nil {
				return err
			}
		}
		t.Revision++
		if err = s.recordTaskUpdate(ctx, tx, user, previous, t); err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) insertTask(ctx context.Context, tx *sql.Tx, user, id string, t *Task) error {
	if taskIsRecurring(t.EWSProperties) && (!t.FullTaskResource || t.AssigneeID != "") {
		return ErrTaskSeries
	}
	if err := t.normalizeProgress(nil); err != nil {
		return err
	}
	if err := taskPropertiesForSave(t); err != nil {
		return err
	}
	t.ID, t.UserID, t.OwnerID, t.Revision = id, user, user, 1
	t.CanManage, t.CanUpdate = true, true
	t.SourceMessageID = ""
	t.AssignmentState, t.AssignmentRevision = "", 0
	var assignee any
	if t.AssigneeID != "" {
		if t.Completed {
			return ErrTaskAssignment
		}
		if err := s.validTaskAssignee(ctx, tx, user, t.AssigneeID); err != nil {
			return err
		}
		assignee = t.AssigneeID
		t.AssignmentState = "pending"
		t.AssignmentRevision = 1
	}
	_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO tasks(id,user_id,title,body,completed,status,percent_complete,due,ews_properties,completed_at,assignee_id,assignment_state,assignment_revision) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`), id, user, t.Title, t.Body, t.Completed, t.Status, t.PercentComplete, t.Due, t.EWSProperties, t.CompletedAt, assignee, t.AssignmentState, t.AssignmentRevision)
	if err != nil {
		return err
	}
	if err = s.recordTaskEvent(ctx, tx, user, t, "create", []string{"task"}); err != nil {
		return err
	}
	return s.taskNotice(ctx, tx, t, t.AssigneeID, "assign")
}

// The receipt and task commit together. A retry never overwrites a newer edit
// or resurrects a deleted task, even if the collection response was not saved.
func (s *Store) CreateFlowSyncTask(ctx context.Context, user, device, clientID string, t *Task) (string, error) {
	if device == "" || clientID == "" || len(clientID) > 256 || t.ID != "" {
		return "", ErrUnauthorized
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	id := "task:" + NewID()
	receipt := fmt.Sprintf("task-add:%x", sha256.Sum256([]byte(clientID)))
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO flowsync_states(user_id,client_id,collection_id,state) SELECT user_id,id,?,? FROM flowsync_devices WHERE id=? AND user_id=? ON CONFLICT(user_id,client_id,collection_id) DO NOTHING`), receipt, id, device, user)
	if err != nil {
		return "", err
	}
	var saved string
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT state FROM flowsync_states WHERE user_id=? AND client_id=? AND collection_id=?`), user, device, receipt).Scan(&saved)
	if err == sql.ErrNoRows {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}
	if saved == id {
		if err = s.insertTask(ctx, tx, user, id, t); err != nil {
			return "", err
		}
	}
	return saved, tx.Commit()
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
	return s.DeleteTaskVersion(ctx, user, id, 0)
}

// DeleteTaskVersion checks the version under the same lock as the deletion.
func (s *Store) DeleteTaskVersion(ctx context.Context, user, id string, revision int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	task, e := s.taskInTx(ctx, tx, user, id)
	if e != nil {
		return e
	}
	if revision != 0 && task.Revision != revision {
		return ErrTaskConflict
	}
	if !task.CanManage {
		return ErrUnauthorized
	}
	if task.AssignmentState != "declined" {
		if err = s.taskNotice(ctx, tx, task, task.AssigneeID, "delete"); err != nil {
			return err
		}
	}
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

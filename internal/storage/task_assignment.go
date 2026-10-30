package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrTaskAssignment = errors.New("invalid task assignment transition")

// Organizations are tenants in the current schema. Sharing crosses domains
// only within that tenant; administrative roles do not bypass this boundary.
const taskSelect = `SELECT t.id,t.user_id,COALESCE(t.source_message_id,''),t.title,t.body,t.completed,t.status,t.percent_complete,t.due,t.revision,t.ews_properties,t.completed_at,COALESCE(t.assignee_id,''),t.assignment_state,t.assignment_revision,o.display_name,o.email,COALESCE(a.display_name,''),COALESCE(a.email,'') FROM tasks t JOIN users o ON o.id=t.user_id LEFT JOIN users a ON a.id=t.assignee_id `
const taskAccess = `(t.user_id=? OR (t.assignee_id=? AND t.assignment_state IN ('pending','accepted') AND o.tenant_id=a.tenant_id AND o.enabled=TRUE AND a.enabled=TRUE))`

type taskScanner interface{ Scan(...any) error }

func scanTask(row taskScanner, viewer string) (*Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.UserID, &t.SourceMessageID, &t.Title, &t.Body, &t.Completed, &t.Status, &t.PercentComplete, &t.Due, &t.Revision, &t.EWSProperties, &t.CompletedAt, &t.AssigneeID, &t.AssignmentState, &t.AssignmentRevision, &t.OwnerName, &t.OwnerEmail, &t.AssigneeName, &t.AssigneeEmail)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Recurring = taskIsRecurring(t.EWSProperties)
	t.OwnerID = t.UserID
	t.CanManage = t.UserID == viewer
	t.CanUpdate = t.CanManage || (t.AssigneeID == viewer && t.AssignmentState == "accepted")
	if t.AssigneeID == "" {
		t.AssignmentState = ""
	}
	// A task assignment never grants access to the author's mailbox.
	if !t.CanManage {
		t.SourceMessageID = ""
	}
	return &t, nil
}

func (s *Store) GetTask(ctx context.Context, user, id string) (*Task, error) {
	return scanTask(s.db.QueryRowContext(ctx, s.rebind(taskSelect+`WHERE t.id=? AND `+taskAccess), id, user, user), user)
}
func (s *Store) taskInTx(ctx context.Context, tx *sql.Tx, user, id string) (*Task, error) {
	query := taskSelect + `WHERE t.id=? AND ` + taskAccess
	if s.dialect == DialectPostgres {
		query += ` FOR UPDATE OF t`
	}
	return scanTask(tx.QueryRowContext(ctx, s.rebind(query), id, user, user), user)
}

type TaskPerson struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func taskPage(limit, offset int) (int, int) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	if offset > 100000 {
		offset = 100000
	}
	return limit, offset
}
func (s *Store) TaskAssignees(ctx context.Context, user, q string, limit, offset int) ([]TaskPerson, int, error) {
	limit, offset = taskPage(limit, offset)
	// Literal search, not a SQL wildcard pattern. Names are also matched in Go
	// for Unicode case folding on both database backends.
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT a.id,a.display_name,a.email FROM users a JOIN users o ON o.tenant_id=a.tenant_id WHERE o.id=? AND o.enabled=TRUE AND a.enabled=TRUE AND a.id<>o.id AND NOT EXISTS (SELECT 1 FROM calendar_resources r WHERE r.user_id=a.id) ORDER BY a.display_name,a.email,a.id`), user)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []TaskPerson{}
	total := 0
	q = strings.ToLower(strings.TrimSpace(q))
	for rows.Next() {
		var p TaskPerson
		if err = rows.Scan(&p.ID, &p.Name, &p.Email); err != nil {
			return nil, 0, err
		}
		if q != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.Email), q) {
			continue
		}
		if total >= offset && len(out) < limit {
			out = append(out, p)
		}
		total++
	}
	return out, total, rows.Err()
}
func (s *Store) validTaskAssignee(ctx context.Context, tx *sql.Tx, owner, assignee string) error {
	var ok int
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT 1 FROM users o JOIN users a ON a.tenant_id=o.tenant_id WHERE o.id=? AND a.id=? AND a.id<>o.id AND o.enabled=TRUE AND a.enabled=TRUE AND NOT EXISTS (SELECT 1 FROM calendar_resources r WHERE r.user_id=a.id)`), owner, assignee).Scan(&ok)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	return err
}
func (s *Store) AssignTask(ctx context.Context, user, id, action, assignee string, revision int64) (*Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	task, err := s.taskInTx(ctx, tx, user, id)
	if err != nil {
		return nil, err
	}
	if task.Revision != revision {
		return nil, ErrNotFound
	}
	if task.Recurring {
		return nil, ErrTaskSeries
	}
	oldAssignee := task.AssigneeID
	oldState := task.AssignmentState
	switch action {
	case "assign":
		if !task.CanManage {
			return nil, ErrUnauthorized
		}
		if task.Completed || assignee == "" || (oldAssignee == assignee && oldState != "declined") {
			return nil, ErrTaskAssignment
		}
		if err = s.validTaskAssignee(ctx, tx, user, assignee); err != nil {
			return nil, err
		}
		task.AssigneeID, task.AssignmentState = assignee, "pending"
		task.AssignmentRevision = task.Revision + 1
	case "revoke":
		if !task.CanManage {
			return nil, ErrUnauthorized
		}
		if oldAssignee == "" {
			return nil, ErrTaskAssignment
		}
		task.AssigneeID, task.AssignmentState = "", ""
		task.AssignmentRevision = 0
	case "accept", "decline":
		if task.CanManage || task.AssigneeID != user {
			return nil, ErrUnauthorized
		}
		if task.AssignmentState != "pending" || task.Completed {
			return nil, ErrTaskAssignment
		}
		task.AssignmentState = "accepted"
		if action == "decline" {
			task.AssignmentState = "declined"
		}
	default:
		return nil, ErrTaskAssignment
	}
	var aid any
	if task.AssigneeID != "" {
		aid = task.AssigneeID
	}
	res, err := tx.ExecContext(ctx, s.rebind(`UPDATE tasks SET assignee_id=?,assignment_state=?,assignment_revision=?,revision=revision+1 WHERE id=? AND revision=?`), aid, task.AssignmentState, task.AssignmentRevision, id, revision)
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
	task.Revision++
	if err = s.recordTaskEvent(ctx, tx, user, task, action, []string{"assignment"}); err != nil {
		return nil, err
	}
	if action == "assign" {
		if oldAssignee != "" && oldAssignee != assignee && oldState != "declined" {
			if err = s.taskNotice(ctx, tx, task, oldAssignee, "revoke"); err != nil {
				return nil, err
			}
		}
		err = s.taskNotice(ctx, tx, task, assignee, action)
	} else if action == "revoke" {
		err = s.taskNotice(ctx, tx, task, oldAssignee, action)
	} else {
		err = s.taskNotice(ctx, tx, task, task.UserID, action)
	}
	if err != nil {
		return nil, err
	}
	// Declining removes access immediately. Return only acknowledgement at HTTP
	// level; reload the authoritative view for the other transitions after commit.
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if action == "decline" {
		return nil, nil
	}
	return s.GetTask(ctx, user, id)
}

type TaskEvent struct {
	ID        string          `json:"id"`
	Revision  int64           `json:"revision"`
	ActorID   string          `json:"actor_id"`
	ActorName string          `json:"actor_name"`
	Action    string          `json:"action"`
	Detail    TaskEventDetail `json:"detail"`
	CreatedAt string          `json:"created_at"`
}
type TaskEventDetail struct {
	Changes         []string `json:"changes"`
	Title           string   `json:"title"`
	Due             string   `json:"due"`
	Status          string   `json:"status"`
	PercentComplete float64  `json:"percent_complete"`
	AssigneeID      string   `json:"assignee_id"`
	AssigneeName    string   `json:"assignee_name"`
	AssigneeEmail   string   `json:"assignee_email"`
	AssignmentState string   `json:"assignment_state"`
}

func taskNow() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z") }
func (s *Store) recordTaskEvent(ctx context.Context, tx *sql.Tx, actor string, t *Task, action string, changes []string) error {
	var name string
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT CASE WHEN display_name='' THEN email ELSE display_name END FROM users WHERE id=?`), actor).Scan(&name); err != nil {
		return err
	}
	var assigneeName, assigneeEmail string
	if t.AssigneeID != "" {
		if err := tx.QueryRowContext(ctx, s.rebind(`SELECT display_name,email FROM users WHERE id=?`), t.AssigneeID).Scan(&assigneeName, &assigneeEmail); err != nil {
			return err
		}
	}
	data, err := json.Marshal(TaskEventDetail{Changes: changes, Title: t.Title, Due: t.Due, Status: t.Status, PercentComplete: t.PercentComplete, AssigneeID: t.AssigneeID, AssigneeName: assigneeName, AssigneeEmail: assigneeEmail, AssignmentState: t.AssignmentState})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO task_events(id,task_id,revision,actor_id,actor_name,action,detail,created_at) VALUES(?,?,?,?,?,?,?,?)`), NewID(), t.ID, t.Revision, actor, name, action, string(data), taskNow())
	return err
}
func (s *Store) TaskHistory(ctx context.Context, user, id string, limit, offset int) ([]TaskEvent, int, error) {
	limit, offset = taskPage(limit, offset)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	task, err := s.taskInTx(ctx, tx, user, id)
	if err != nil {
		return nil, 0, err
	}
	minRevision := int64(0)
	if !task.CanManage {
		minRevision = task.AssignmentRevision
	}
	var total int
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM task_events WHERE task_id=? AND revision>=?`), id, minRevision).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT id,revision,COALESCE(actor_id,''),actor_name,action,detail,created_at FROM task_events WHERE task_id=? AND revision>=? ORDER BY revision DESC LIMIT ? OFFSET ?`), id, minRevision, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []TaskEvent{}
	for rows.Next() {
		var e TaskEvent
		var raw string
		if err = rows.Scan(&e.ID, &e.Revision, &e.ActorID, &e.ActorName, &e.Action, &raw, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		if err = json.Unmarshal([]byte(raw), &e.Detail); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// Notifications deliberately contain no task title/body. They remain useful
// after revocation/deletion without disclosing later task content. The recipient
// must pass the current task ACL again when opening the task.
type TaskNotification struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	UserID    string `json:"-"`
	Action    string `json:"action"`
	CreatedAt string `json:"created_at"`
	ReadAt    string `json:"read_at"`
}

const taskNoticeScope = ` JOIN users u ON u.id=n.user_id JOIN users o ON o.id=n.owner_id WHERE u.tenant_id=n.tenant_id AND o.tenant_id=n.tenant_id AND u.enabled=TRUE AND o.enabled=TRUE `

func (s *Store) taskNotice(ctx context.Context, tx *sql.Tx, t *Task, recipient, action string) error {
	if recipient == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO task_notifications(id,task_id,owner_id,user_id,tenant_id,action,created_at) SELECT ?,?,o.id,u.id,o.tenant_id,?,? FROM users o JOIN users u ON u.tenant_id=o.tenant_id WHERE o.id=? AND u.id=? AND o.enabled=TRUE AND u.enabled=TRUE`), NewID(), t.ID, action, taskNow(), t.UserID, recipient)
	return err
}
func (s *Store) TaskNotifications(ctx context.Context, user string, limit, offset int) ([]TaskNotification, int, int, error) {
	limit, offset = taskPage(limit, offset)
	var total, unread int
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN n.read_at='' THEN 1 ELSE 0 END),0) FROM task_notifications n`+taskNoticeScope+` AND n.user_id=?`), user).Scan(&total, &unread)
	if err != nil {
		return nil, 0, 0, err
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT n.id,n.task_id,n.user_id,n.action,n.created_at,n.read_at FROM task_notifications n`+taskNoticeScope+` AND n.user_id=? ORDER BY n.created_at DESC,n.id DESC LIMIT ? OFFSET ?`), user, limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	out := []TaskNotification{}
	for rows.Next() {
		var n TaskNotification
		if err = rows.Scan(&n.ID, &n.TaskID, &n.UserID, &n.Action, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, 0, 0, err
		}
		out = append(out, n)
	}
	return out, total, unread, rows.Err()
}
func (s *Store) ReadTaskNotification(ctx context.Context, user, id string) error {
	res, err := s.db.ExecContext(ctx, s.rebind(`UPDATE task_notifications SET read_at=? WHERE id=? AND user_id=?`), taskNow(), id, user)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return ErrNotFound
	}
	return err
}
func (s *Store) PendingTaskNotifications(ctx context.Context) ([]TaskNotification, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT n.id,n.task_id,n.user_id,n.action,n.created_at,n.read_at FROM task_notifications n`+taskNoticeScope+` AND n.dispatched=FALSE ORDER BY n.created_at,n.id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TaskNotification{}
	for rows.Next() {
		var n TaskNotification
		if err = rows.Scan(&n.ID, &n.TaskID, &n.UserID, &n.Action, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
func (s *Store) DispatchTaskNotification(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE task_notifications SET dispatched=TRUE WHERE id=?`), id)
	return err
}

func taskChanges(before, after *Task) []string {
	out := []string{}
	if before.Title != after.Title {
		out = append(out, "title")
	}
	if before.Body != after.Body {
		out = append(out, "body")
	}
	if before.Due != after.Due {
		out = append(out, "due")
	}
	if before.Status != after.Status || before.PercentComplete != after.PercentComplete {
		out = append(out, "progress")
	}
	if before.EWSProperties != after.EWSProperties {
		out = append(out, "properties")
	}
	return out
}

// Assignees may only update progress and the accompanying completion date.
func taskAssigneeEditAllowed(before, after *Task) bool {
	if before.Title != after.Title || before.Body != after.Body || before.Due != after.Due {
		return false
	}
	a, e1 := taskPropertiesWithoutCompletion(before.EWSProperties)
	b, e2 := taskPropertiesWithoutCompletion(after.EWSProperties)
	return e1 == nil && e2 == nil && a == b
}
func (s *Store) recordTaskUpdate(ctx context.Context, tx *sql.Tx, actor string, before, after *Task) error {
	changes := taskChanges(before, after)
	if len(changes) == 0 {
		return nil
	}
	action := "update"
	if before.Completed != after.Completed {
		action = "reopen"
		if after.Completed {
			action = "complete"
		}
	}
	if err := s.recordTaskEvent(ctx, tx, actor, after, action, changes); err != nil {
		return err
	}
	recipient := after.AssigneeID
	if actor != after.UserID {
		recipient = after.UserID
	}
	if recipient == actor || (actor == after.UserID && after.AssignmentState == "declined") {
		return nil
	}
	return s.taskNotice(ctx, tx, after, recipient, action)
}

package migrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const (
	KindIMAP    = "imap"
	KindCalDAV  = "caldav"
	KindCardDAV = "carddav"

	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusPaused    = "paused"
	StatusDone      = "done"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Job is a user migration task (one-shot in v1; schedule fields reserved).
type Job struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	Kind               string     `json:"kind"`
	Status             string     `json:"status"`
	Host               string     `json:"host,omitempty"`
	URL                string     `json:"url,omitempty"`
	Port               int        `json:"port,omitempty"`
	TLS                bool       `json:"tls"`
	Username           string     `json:"username"`
	PasswordCiphertext []byte     `json:"-"`
	Options            string     `json:"options,omitempty"`
	Copied             int64      `json:"copied"`
	Skipped            int64      `json:"skipped"`
	Errors             int64      `json:"errors"`
	LastError          string     `json:"last_error,omitempty"`
	CursorJSON         string     `json:"cursor,omitempty"`
	Schedule           string     `json:"schedule,omitempty"`
	NextRunAt          *time.Time `json:"next_run_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
}

// CreateRequest is the API body for starting a job.
type CreateRequest struct {
	Kind     string          `json:"kind"`
	Host     string          `json:"host"`
	URL      string          `json:"url"`
	Port     int             `json:"port"`
	TLS      *bool           `json:"tls"`
	Username string          `json:"username"`
	Password string          `json:"password"`
	Options  json.RawMessage `json:"options"`
}

func (s *Service) rebind(q string) string {
	if s == nil || !s.postgres {
		return q
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteByte('$')
			fmt.Fprintf(&b, "%d", n)
			continue
		}
		b.WriteByte(q[i])
	}
	return b.String()
}

func (s *Service) CreateJob(ctx context.Context, userID string, req CreateRequest) (*Job, error) {
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	switch kind {
	case KindIMAP, KindCalDAV, KindCardDAV:
	default:
		return nil, fmt.Errorf("kind must be imap, caldav, or carddav")
	}
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		return nil, fmt.Errorf("username and password required")
	}
	if kind == KindIMAP && strings.TrimSpace(req.Host) == "" {
		return nil, fmt.Errorf("host required for imap")
	}
	if (kind == KindCalDAV || kind == KindCardDAV) && strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("url required for %s", kind)
	}
	active, err := s.hasActiveJob(ctx, userID, kind)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, fmt.Errorf("an active %s migration already exists", kind)
	}
	ct, err := encryptPassword(s.key, req.Password)
	if err != nil {
		return nil, err
	}
	tlsOn := true
	if req.TLS != nil {
		tlsOn = *req.TLS
	}
	opts := "{}"
	if len(req.Options) > 0 {
		opts = string(req.Options)
	}
	port := req.Port
	if port == 0 && kind == KindIMAP {
		if tlsOn {
			port = 993
		} else {
			port = 143
		}
	}
	now := time.Now().UTC()
	j := &Job{
		ID:                 storage.NewID(),
		UserID:             userID,
		Kind:               kind,
		Status:             StatusPending,
		Host:               strings.TrimSpace(req.Host),
		URL:                strings.TrimSpace(req.URL),
		Port:               port,
		TLS:                tlsOn,
		Username:           strings.TrimSpace(req.Username),
		PasswordCiphertext: ct,
		Options:            opts,
		CursorJSON:         "{}",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	tlsInt := 0
	if j.TLS {
		tlsInt = 1
	}
	_, err = s.db.ExecContext(ctx, s.rebind(`
		INSERT INTO migration_jobs(
			id, user_id, kind, status, host, url, port, tls, username, password_ciphertext,
			options, copied, skipped, errors, last_error, cursor_json, schedule, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,0,0,0,'','{}','',?,?)`),
		j.ID, j.UserID, j.Kind, j.Status, j.Host, j.URL, j.Port, tlsInt, j.Username, j.PasswordCiphertext,
		j.Options, now, now,
	)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (s *Service) hasActiveJob(ctx context.Context, userID, kind string) (bool, error) {
	q := s.rebind(`SELECT 1 FROM migration_jobs WHERE user_id = ? AND kind = ? AND status IN ('pending','running','paused') LIMIT 1`)
	var one int
	err := s.db.QueryRowContext(ctx, q, userID, kind).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) ListJobs(ctx context.Context, userID string) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`
		SELECT id, user_id, kind, status, host, url, port, tls, username,
			copied, skipped, errors, last_error, cursor_json, schedule,
			next_run_at, created_at, updated_at, started_at, finished_at
		FROM migration_jobs WHERE user_id = ? ORDER BY created_at DESC LIMIT 50`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		j, err := scanJobPublic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

func (s *Service) ListJobsByTenant(ctx context.Context, tenantID string) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`
		SELECT j.id, j.user_id, j.kind, j.status, j.host, j.url, j.port, j.tls, j.username,
			j.copied, j.skipped, j.errors, j.last_error, j.cursor_json, j.schedule,
			j.next_run_at, j.created_at, j.updated_at, j.started_at, j.finished_at
		FROM migration_jobs j
		JOIN users u ON u.id = j.user_id
		WHERE u.tenant_id = ?
		ORDER BY j.created_at DESC LIMIT 100`), tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		j, err := scanJobPublic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

func (s *Service) GetJob(ctx context.Context, userID, id string) (*Job, error) {
	row := s.db.QueryRowContext(ctx, s.rebind(`
		SELECT id, user_id, kind, status, host, url, port, tls, username,
			copied, skipped, errors, last_error, cursor_json, schedule,
			next_run_at, created_at, updated_at, started_at, finished_at
		FROM migration_jobs WHERE id = ? AND user_id = ?`), id, userID)
	return scanJobPublic(row)
}

func (s *Service) getJobInternal(ctx context.Context, id string) (*Job, error) {
	row := s.db.QueryRowContext(ctx, s.rebind(`
		SELECT id, user_id, kind, status, host, url, port, tls, username, password_ciphertext,
			options, copied, skipped, errors, last_error, cursor_json, schedule,
			next_run_at, created_at, updated_at, started_at, finished_at
		FROM migration_jobs WHERE id = ?`), id)
	return scanJobFull(row)
}

func (s *Service) CancelJob(ctx context.Context, userID, id string) error {
	res, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE migration_jobs SET status = ?, updated_at = ?, finished_at = ?, password_ciphertext = NULL
		WHERE id = ? AND user_id = ? AND status IN ('pending','running','paused')`),
		StatusCancelled, time.Now().UTC(), time.Now().UTC(), id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *Service) claimPending(ctx context.Context) (*Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var id string
	q := `SELECT id FROM migration_jobs WHERE status = 'pending' ORDER BY created_at LIMIT 1`
	if s.postgres {
		q = `SELECT id FROM migration_jobs WHERE status = 'pending' ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED`
	}
	err = tx.QueryRowContext(ctx, q).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, s.rebind(`
		UPDATE migration_jobs SET status = ?, updated_at = ?, started_at = COALESCE(started_at, ?) WHERE id = ?`),
		StatusRunning, now, now, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.getJobInternal(ctx, id)
}

func (s *Service) bumpProgress(ctx context.Context, id string, copied, skipped, errors int64, lastErr, cursor string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE migration_jobs SET copied = ?, skipped = ?, errors = ?, last_error = ?, cursor_json = ?, updated_at = ?
		WHERE id = ?`), copied, skipped, errors, lastErr, cursor, time.Now().UTC(), id)
	return err
}

func (s *Service) finishJob(ctx context.Context, id, status, lastErr string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, s.rebind(`
		UPDATE migration_jobs SET status = ?, last_error = ?, updated_at = ?, finished_at = ?
		WHERE id = ?`), status, lastErr, now, now, id)
	return err
}

func (s *Service) isCancelled(ctx context.Context, id string) bool {
	var st string
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT status FROM migration_jobs WHERE id = ?`), id).Scan(&st)
	return err == nil && st == StatusCancelled
}

func scanJobPublic(row interface{ Scan(dest ...any) error }) (*Job, error) {
	j := &Job{}
	var tlsVal any
	var next, started, finished sql.NullTime
	err := row.Scan(
		&j.ID, &j.UserID, &j.Kind, &j.Status, &j.Host, &j.URL, &j.Port, &tlsVal, &j.Username,
		&j.Copied, &j.Skipped, &j.Errors, &j.LastError, &j.CursorJSON, &j.Schedule,
		&next, &j.CreatedAt, &j.UpdatedAt, &started, &finished,
	)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	j.TLS = asBool(tlsVal)
	if next.Valid {
		t := next.Time.UTC()
		j.NextRunAt = &t
	}
	if started.Valid {
		t := started.Time.UTC()
		j.StartedAt = &t
	}
	if finished.Valid {
		t := finished.Time.UTC()
		j.FinishedAt = &t
	}
	return j, nil
}

func scanJobFull(row interface{ Scan(dest ...any) error }) (*Job, error) {
	j := &Job{}
	var tlsVal any
	var next, started, finished sql.NullTime
	var ct []byte
	err := row.Scan(
		&j.ID, &j.UserID, &j.Kind, &j.Status, &j.Host, &j.URL, &j.Port, &tlsVal, &j.Username, &ct,
		&j.Options, &j.Copied, &j.Skipped, &j.Errors, &j.LastError, &j.CursorJSON, &j.Schedule,
		&next, &j.CreatedAt, &j.UpdatedAt, &started, &finished,
	)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	j.PasswordCiphertext = ct
	j.TLS = asBool(tlsVal)
	if next.Valid {
		t := next.Time.UTC()
		j.NextRunAt = &t
	}
	if started.Valid {
		t := started.Time.UTC()
		j.StartedAt = &t
	}
	if finished.Valid {
		t := finished.Time.UTC()
		j.FinishedAt = &t
	}
	return j, nil
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case int:
		return t != 0
	case []byte:
		return string(t) == "1" || strings.EqualFold(string(t), "true")
	default:
		return false
	}
}

// RetryJob creates a fresh history entry; source identities prevent duplicates.
func (s *Service) RetryJob(ctx context.Context, userID, id, password string) (*Job, error) {
	old, err := s.getJobInternal(ctx, id)
	if err != nil || old.UserID != userID {
		return nil, storage.ErrNotFound
	}
	if old.Status == StatusPending || old.Status == StatusRunning || old.Status == StatusPaused {
		return nil, fmt.Errorf("migration is still active")
	}
	if password == "" {
		password, err = decryptPassword(s.key, old.PasswordCiphertext)
		if err != nil {
			return nil, fmt.Errorf("source password required for this older migration")
		}
	}
	tls := old.TLS
	return s.CreateJob(ctx, userID, CreateRequest{Kind: old.Kind, Host: old.Host, URL: old.URL, Port: old.Port, TLS: &tls, Username: old.Username, Password: password, Options: json.RawMessage(old.Options)})
}

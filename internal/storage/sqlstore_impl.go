package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Dialect int

const (
	DialectSQLite Dialect = iota
	DialectPostgres
)

// Store implements storage.Driver for SQLite and PostgreSQL.
type Store struct {
	db      *sql.DB
	dialect Dialect
}

var _ Driver = (*Store)(nil)

// NewSQLStore constructs a SQL-backed Driver.
func NewSQLStore(db *sql.DB, dialect Dialect) *Store {
	return &Store{db: db, dialect: dialect}
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) Migrate(ctx context.Context) error { return nil }

func (s *Store) ph(n int) string {
	if s.dialect == DialectPostgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func (s *Store) rebind(query string) string {
	if s.dialect != DialectPostgres {
		return query
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(fmt.Sprintf("%d", n))
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

func (s *Store) CreateTenant(ctx context.Context, name string) (*Tenant, error) {
	return s.InsertTenant(ctx, &Tenant{Name: name})
}

func (s *Store) InsertTenant(ctx context.Context, t *Tenant) (*Tenant, error) {
	now := time.Now().UTC()
	if t.ID == "" {
		t.ID = NewID()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	q := s.rebind(`INSERT INTO tenants(id, name, created_at) VALUES (?, ?, ?)`)
	if _, err := s.db.ExecContext(ctx, q, t.ID, t.Name, t.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return t, nil
}

func (s *Store) GetTenantByName(ctx context.Context, name string) (*Tenant, error) {
	t := &Tenant{}
	q := s.rebind(`SELECT id, name, created_at FROM tenants WHERE name = ?`)
	err := s.db.QueryRowContext(ctx, q, name).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return t, nil
}

func (s *Store) GetTenantByID(ctx context.Context, id string) (*Tenant, error) {
	t := &Tenant{}
	q := s.rebind(`SELECT id, name, created_at FROM tenants WHERE id = ?`)
	err := s.db.QueryRowContext(ctx, q, id).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return t, nil
}

func (s *Store) ListTenants(ctx context.Context) ([]*Tenant, error) {
	q := s.rebind(`SELECT id, name, created_at FROM tenants ORDER BY name`)
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Tenant
	for rows.Next() {
		t := &Tenant{}
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateDomain(ctx context.Context, tenantID, name string) (*Domain, error) {
	return s.InsertDomain(ctx, &Domain{TenantID: tenantID, Name: name})
}

func (s *Store) InsertDomain(ctx context.Context, d *Domain) (*Domain, error) {
	now := time.Now().UTC()
	d.Name = strings.ToLower(d.Name)
	if d.ID == "" {
		d.ID = NewID()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.MigrationEnabled == "" {
		d.MigrationEnabled = "inherit"
	}
	q := s.rebind(`INSERT INTO domains(id, tenant_id, name, migration_enabled, created_at) VALUES (?, ?, ?, ?, ?)`)
	if _, err := s.db.ExecContext(ctx, q, d.ID, d.TenantID, d.Name, d.MigrationEnabled, d.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return d, nil
}

func (s *Store) GetDomainByID(ctx context.Context, id string) (*Domain, error) {
	d := &Domain{}
	q := s.rebind(`SELECT id, tenant_id, name, COALESCE(migration_enabled,'inherit'), created_at FROM domains WHERE id = ?`)
	err := s.db.QueryRowContext(ctx, q, id).Scan(&d.ID, &d.TenantID, &d.Name, &d.MigrationEnabled, &d.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return d, nil
}

func (s *Store) GetDomainByName(ctx context.Context, name string) (*Domain, error) {
	d := &Domain{}
	name = strings.ToLower(name)
	var q string
	if s.dialect == DialectPostgres {
		q = `SELECT id, tenant_id, name, COALESCE(migration_enabled,'inherit'), created_at FROM domains WHERE LOWER(name) = LOWER($1)`
	} else {
		q = `SELECT id, tenant_id, name, COALESCE(migration_enabled,'inherit'), created_at FROM domains WHERE name = ? COLLATE NOCASE`
	}
	err := s.db.QueryRowContext(ctx, q, name).Scan(&d.ID, &d.TenantID, &d.Name, &d.MigrationEnabled, &d.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return d, nil
}

func (s *Store) ListDomainsByTenant(ctx context.Context, tenantID string) ([]*Domain, error) {
	q := s.rebind(`SELECT id, tenant_id, name, COALESCE(migration_enabled,'inherit'), created_at FROM domains WHERE tenant_id = ? ORDER BY name`)
	rows, err := s.db.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Domain
	for rows.Next() {
		d := &Domain{}
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.MigrationEnabled, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) UpdateDomainMigration(ctx context.Context, domainID, migrationEnabled string) error {
	migrationEnabled = normalizeMigrationPolicy(migrationEnabled)
	q := s.rebind(`UPDATE domains SET migration_enabled = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, migrationEnabled, domainID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeMigrationPolicy(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on", "true", "1", "enabled":
		return "on"
	case "off", "false", "0", "disabled":
		return "off"
	default:
		return "inherit"
	}
}

func (s *Store) CountUsersByDomain(ctx context.Context, domainID string) (int, error) {
	q := s.rebind(`SELECT COUNT(*) FROM users WHERE domain_id = ?`)
	var n int
	err := s.db.QueryRowContext(ctx, q, domainID).Scan(&n)
	return n, err
}

func (s *Store) DeleteDomain(ctx context.Context, tenantID, domainID string) error {
	n, err := s.CountUsersByDomain(ctx, domainID)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("domain still has %d users", n)
	}
	q := s.rebind(`DELETE FROM domains WHERE id = ? AND tenant_id = ?`)
	res, err := s.db.ExecContext(ctx, q, domainID, tenantID)
	if err != nil {
		return mapErr(err)
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, u *User) (*User, error) {
	now := time.Now().UTC()
	u.Email = strings.ToLower(u.Email)
	u.LocalPart = strings.ToLower(u.LocalPart)
	if u.AuthSource == "" {
		u.AuthSource = "local"
	}
	if u.ID == "" {
		u.ID = NewID()
	}
	u.CreatedAt = now
	enabled := 1
	if !u.Enabled {
		enabled = 0
	}

	u.Roles = JoinRoles(ParseRoles(u.Roles))
	if u.MigrationEnabled == "" {
		u.MigrationEnabled = "inherit"
	}
	q := s.rebind(`INSERT INTO users(
		id, tenant_id, domain_id, email, local_part, display_name, password_hash,
		auth_source, quota_bytes, enabled, roles, service_class_id, migration_enabled, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)

	args := []any{
		u.ID, u.TenantID, u.DomainID, u.Email, u.LocalPart, u.DisplayName, u.PasswordHash,
		u.AuthSource, u.QuotaBytes, enabled, u.Roles, u.ServiceClassID, u.MigrationEnabled, now,
	}
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return nil, mapErr(err)
	}
	return u, nil
}

func (s *Store) scanUser(row interface{ Scan(dest ...any) error }) (*User, error) {
	u := &User{}
	var enabled any
	err := row.Scan(
		&u.ID, &u.TenantID, &u.DomainID, &u.Email, &u.LocalPart, &u.DisplayName,
		&u.PasswordHash, &u.AuthSource, &u.QuotaBytes, &enabled, &u.Roles, &u.ServiceClassID,
		&u.MigrationEnabled, &u.CreatedAt,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	u.Enabled = asBool(enabled)
	u.Roles = JoinRoles(ParseRoles(u.Roles))
	if u.MigrationEnabled == "" {
		u.MigrationEnabled = "inherit"
	}
	return u, nil
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

const userCols = `id, tenant_id, domain_id, email, local_part, display_name, password_hash, auth_source, quota_bytes, enabled, roles, COALESCE(service_class_id,''), COALESCE(migration_enabled,'inherit'), created_at`

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	email = strings.ToLower(email)
	var q string
	if s.dialect == DialectPostgres {
		q = `SELECT ` + userCols + ` FROM users WHERE LOWER(email) = LOWER($1)`
	} else {
		q = `SELECT ` + userCols + ` FROM users WHERE email = ? COLLATE NOCASE`
	}
	return s.scanUser(s.db.QueryRowContext(ctx, q, email))
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	q := s.rebind(`SELECT ` + userCols + ` FROM users WHERE id = ?`)
	return s.scanUser(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	q := s.rebind(`UPDATE users SET password_hash = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, passwordHash, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateUserRoles(ctx context.Context, userID, roles string) error {
	roles = JoinRoles(ParseRoles(roles))
	q := s.rebind(`UPDATE users SET roles = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, roles, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateUserProfile(ctx context.Context, userID, displayName string) error {
	q := s.rebind(`UPDATE users SET display_name = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, displayName, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateUserQuota(ctx context.Context, userID string, quotaBytes int64) error {
	q := s.rebind(`UPDATE users SET quota_bytes = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, quotaBytes, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateUserEnabled(ctx context.Context, userID string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	q := s.rebind(`UPDATE users SET enabled = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, v, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes a user row (cascades calendars/mailboxes via FK).
func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	q := s.rebind(`DELETE FROM users WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, userID)
	if err != nil {
		return mapErr(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListUsersByTenant(ctx context.Context, tenantID string) ([]*User, error) {
	q := s.rebind(`SELECT ` + userCols + ` FROM users WHERE tenant_id = ? ORDER BY email`)
	rows, err := s.db.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := s.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) SumMailboxBytes(ctx context.Context, userID string) (int64, error) {
	q := s.rebind(`SELECT COALESCE(SUM(m.size), 0) FROM messages m
		JOIN mailboxes b ON m.mailbox_id = b.id WHERE b.user_id = ?`)
	var total int64
	if err := s.db.QueryRowContext(ctx, q, userID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) CreateAlias(ctx context.Context, domainID, userID, localPart string) (*Alias, error) {
	now := time.Now().UTC()
	localPart = strings.ToLower(localPart)
	a := &Alias{ID: NewID(), DomainID: domainID, UserID: userID, LocalPart: localPart, CreatedAt: now}
	q := s.rebind(`INSERT INTO aliases(id, domain_id, user_id, local_part, created_at) VALUES (?, ?, ?, ?, ?)`)
	if _, err := s.db.ExecContext(ctx, q, a.ID, domainID, userID, localPart, now); err != nil {
		return nil, mapErr(err)
	}
	return a, nil
}

func (s *Store) ResolveRecipient(ctx context.Context, email string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.GetUserByEmail(ctx, email)
	if err == nil {
		if !u.Enabled {
			return nil, ErrNotFound
		}
		return u, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	at := strings.LastIndex(email, "@")
	if at < 0 {
		return nil, ErrNotFound
	}
	local, domain := email[:at], email[at+1:]

	d, err := s.GetDomainByName(ctx, domain)
	if err != nil {
		return nil, err
	}

	var userID string
	var q string
	if s.dialect == DialectPostgres {
		q = `SELECT user_id FROM aliases WHERE domain_id = $1 AND LOWER(local_part) = LOWER($2)`
	} else {
		q = `SELECT user_id FROM aliases WHERE domain_id = ? AND local_part = ? COLLATE NOCASE`
	}
	err = s.db.QueryRowContext(ctx, q, d.ID, local).Scan(&userID)
	if err != nil {
		return nil, mapErr(err)
	}
	u, err = s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !u.Enabled {
		return nil, ErrNotFound
	}
	return u, nil
}

func (s *Store) EnsureMailbox(ctx context.Context, userID, name, path string) (*Mailbox, error) {
	mb, err := s.GetMailbox(ctx, userID, name)
	if err == nil {
		return mb, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	now := time.Now().UTC()
	mb = &Mailbox{
		ID:          NewID(),
		UserID:      userID,
		Name:        name,
		Path:        path,
		UIDNext:     1,
		UIDValidity: time.Now().Unix(),
		CreatedAt:   now,
	}
	q := s.rebind(`INSERT INTO mailboxes(id, user_id, name, path, uidnext, uidvalidity, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	args := []any{mb.ID, userID, name, path, mb.UIDNext, mb.UIDValidity, now}
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		if existing, gerr := s.GetMailbox(ctx, userID, name); gerr == nil {
			return existing, nil
		}
		return nil, mapErr(err)
	}
	return mb, nil
}

func (s *Store) GetMailbox(ctx context.Context, userID, name string) (*Mailbox, error) {
	mb := &Mailbox{}
	q := s.rebind(`SELECT id, user_id, name, path, uidnext, uidvalidity, created_at
		FROM mailboxes WHERE user_id = ? AND name = ?`)
	err := s.db.QueryRowContext(ctx, q, userID, name).Scan(
		&mb.ID, &mb.UserID, &mb.Name, &mb.Path, &mb.UIDNext, &mb.UIDValidity, &mb.CreatedAt,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	return mb, nil
}

func (s *Store) GetMailboxByID(ctx context.Context, id string) (*Mailbox, error) {
	mb := &Mailbox{}
	q := s.rebind(`SELECT id, user_id, name, path, uidnext, uidvalidity, created_at
		FROM mailboxes WHERE id = ?`)
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&mb.ID, &mb.UserID, &mb.Name, &mb.Path, &mb.UIDNext, &mb.UIDValidity, &mb.CreatedAt,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	return mb, nil
}

func (s *Store) InsertMessage(ctx context.Context, msg *Message) (*Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var uid int64
	lockQ := s.rebind(`SELECT uidnext FROM mailboxes WHERE id = ?`)
	if s.dialect == DialectPostgres {
		lockQ = `SELECT uidnext FROM mailboxes WHERE id = $1 FOR UPDATE`
	}
	if err := tx.QueryRowContext(ctx, lockQ, msg.MailboxID).Scan(&uid); err != nil {
		return nil, mapErr(err)
	}
	msg.UID = uid
	if msg.ID == "" {
		msg.ID = NewID()
	}
	if msg.InternalDate.IsZero() {
		msg.InternalDate = time.Now().UTC()
	}
	msg.CreatedAt = time.Now().UTC()

	upd := s.rebind(`UPDATE mailboxes SET uidnext = ? WHERE id = ?`)
	if _, err := tx.ExecContext(ctx, upd, uid+1, msg.MailboxID); err != nil {
		return nil, err
	}

	arch := 0
	if msg.Archived {
		arch = 1
	}
	ins := s.rebind(`INSERT INTO messages(id, mailbox_id, uid, size, flags, internal_date, file_path, message_id, subject, from_addr, to_addr, date_hdr, archived, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	args := []any{
		msg.ID, msg.MailboxID, msg.UID, msg.Size, msg.Flags, msg.InternalDate, msg.FilePath, msg.MessageID,
		msg.Subject, msg.FromAddr, msg.ToAddr, msg.DateHdr, arch, msg.CreatedAt,
	}
	if _, err := tx.ExecContext(ctx, ins, args...); err != nil {
		return nil, mapErr(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *Store) GetMessageByID(ctx context.Context, id string) (*Message, error) {
	q := s.rebind(`SELECT ` + msgCols + ` FROM messages WHERE id = ?`)
	return s.scanMessage(s.db.QueryRowContext(ctx, q, id))
}

// DialectName returns "sqlite" or "postgres".
func (s *Store) DialectName() string {
	if s.dialect == DialectPostgres {
		return "postgres"
	}
	return "sqlite"
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "constraint failed") {
		return fmt.Errorf("%w: %v", ErrAlreadyExists, err)
	}
	return err
}

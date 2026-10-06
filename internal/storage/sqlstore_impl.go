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
	now := time.Now().UTC()
	t := &Tenant{Name: name, CreatedAt: now}
	q := s.rebind(`INSERT INTO tenants(name, created_at) VALUES (?, ?)`)
	if s.dialect == DialectPostgres {
		err := s.db.QueryRowContext(ctx, q+` RETURNING id`, name, now).Scan(&t.ID)
		if err != nil {
			return nil, mapErr(err)
		}
		return t, nil
	}
	res, err := s.db.ExecContext(ctx, q, name, now)
	if err != nil {
		return nil, mapErr(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	t.ID = id
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

func (s *Store) CreateDomain(ctx context.Context, tenantID int64, name string) (*Domain, error) {
	now := time.Now().UTC()
	name = strings.ToLower(name)
	d := &Domain{TenantID: tenantID, Name: name, CreatedAt: now}
	q := s.rebind(`INSERT INTO domains(tenant_id, name, created_at) VALUES (?, ?, ?)`)
	if s.dialect == DialectPostgres {
		err := s.db.QueryRowContext(ctx, q+` RETURNING id`, tenantID, name, now).Scan(&d.ID)
		if err != nil {
			return nil, mapErr(err)
		}
		return d, nil
	}
	res, err := s.db.ExecContext(ctx, q, tenantID, name, now)
	if err != nil {
		return nil, mapErr(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	d.ID = id
	return d, nil
}

func (s *Store) GetDomainByName(ctx context.Context, name string) (*Domain, error) {
	d := &Domain{}
	name = strings.ToLower(name)
	var q string
	if s.dialect == DialectPostgres {
		q = `SELECT id, tenant_id, name, created_at FROM domains WHERE LOWER(name) = LOWER($1)`
	} else {
		q = `SELECT id, tenant_id, name, created_at FROM domains WHERE name = ? COLLATE NOCASE`
	}
	err := s.db.QueryRowContext(ctx, q, name).Scan(&d.ID, &d.TenantID, &d.Name, &d.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return d, nil
}

func (s *Store) CreateUser(ctx context.Context, u *User) (*User, error) {
	now := time.Now().UTC()
	u.Email = strings.ToLower(u.Email)
	u.LocalPart = strings.ToLower(u.LocalPart)
	if u.AuthSource == "" {
		u.AuthSource = "local"
	}
	u.CreatedAt = now
	enabled := 1
	if !u.Enabled {
		enabled = 0
	}

	q := s.rebind(`INSERT INTO users(
		tenant_id, domain_id, email, local_part, display_name, password_hash,
		auth_source, quota_bytes, enabled, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)

	args := []any{
		u.TenantID, u.DomainID, u.Email, u.LocalPart, u.DisplayName, u.PasswordHash,
		u.AuthSource, u.QuotaBytes, enabled, now,
	}

	if s.dialect == DialectPostgres {
		err := s.db.QueryRowContext(ctx, q+` RETURNING id`, args...).Scan(&u.ID)
		if err != nil {
			return nil, mapErr(err)
		}
		return u, nil
	}
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	u.ID = id
	return u, nil
}

func (s *Store) scanUser(row interface{ Scan(dest ...any) error }) (*User, error) {
	u := &User{}
	var enabled any
	err := row.Scan(
		&u.ID, &u.TenantID, &u.DomainID, &u.Email, &u.LocalPart, &u.DisplayName,
		&u.PasswordHash, &u.AuthSource, &u.QuotaBytes, &enabled, &u.CreatedAt,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	u.Enabled = asBool(enabled)
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

const userCols = `id, tenant_id, domain_id, email, local_part, display_name, password_hash, auth_source, quota_bytes, enabled, created_at`

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

func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	q := s.rebind(`SELECT ` + userCols + ` FROM users WHERE id = ?`)
	return s.scanUser(s.db.QueryRowContext(ctx, q, id))
}

func (s *Store) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
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

func (s *Store) CreateAlias(ctx context.Context, domainID, userID int64, localPart string) (*Alias, error) {
	now := time.Now().UTC()
	localPart = strings.ToLower(localPart)
	a := &Alias{DomainID: domainID, UserID: userID, LocalPart: localPart, CreatedAt: now}
	q := s.rebind(`INSERT INTO aliases(domain_id, user_id, local_part, created_at) VALUES (?, ?, ?, ?)`)
	if s.dialect == DialectPostgres {
		err := s.db.QueryRowContext(ctx, q+` RETURNING id`, domainID, userID, localPart, now).Scan(&a.ID)
		if err != nil {
			return nil, mapErr(err)
		}
		return a, nil
	}
	res, err := s.db.ExecContext(ctx, q, domainID, userID, localPart, now)
	if err != nil {
		return nil, mapErr(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	a.ID = id
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

	var userID int64
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

func (s *Store) EnsureMailbox(ctx context.Context, userID int64, name, path string) (*Mailbox, error) {
	mb, err := s.GetMailbox(ctx, userID, name)
	if err == nil {
		return mb, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	now := time.Now().UTC()
	mb = &Mailbox{
		UserID:      userID,
		Name:        name,
		Path:        path,
		UIDNext:     1,
		UIDValidity: time.Now().Unix(),
		CreatedAt:   now,
	}
	q := s.rebind(`INSERT INTO mailboxes(user_id, name, path, uidnext, uidvalidity, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	args := []any{userID, name, path, mb.UIDNext, mb.UIDValidity, now}
	if s.dialect == DialectPostgres {
		err = s.db.QueryRowContext(ctx, q+` RETURNING id`, args...).Scan(&mb.ID)
		if err != nil {
			// race: another writer created it
			if existing, gerr := s.GetMailbox(ctx, userID, name); gerr == nil {
				return existing, nil
			}
			return nil, mapErr(err)
		}
		return mb, nil
	}
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		if existing, gerr := s.GetMailbox(ctx, userID, name); gerr == nil {
			return existing, nil
		}
		return nil, mapErr(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	mb.ID = id
	return mb, nil
}

func (s *Store) GetMailbox(ctx context.Context, userID int64, name string) (*Mailbox, error) {
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
	if msg.InternalDate.IsZero() {
		msg.InternalDate = time.Now().UTC()
	}
	msg.CreatedAt = time.Now().UTC()

	upd := s.rebind(`UPDATE mailboxes SET uidnext = ? WHERE id = ?`)
	if _, err := tx.ExecContext(ctx, upd, uid+1, msg.MailboxID); err != nil {
		return nil, err
	}

	ins := s.rebind(`INSERT INTO messages(mailbox_id, uid, size, flags, internal_date, file_path, message_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	args := []any{msg.MailboxID, msg.UID, msg.Size, msg.Flags, msg.InternalDate, msg.FilePath, msg.MessageID, msg.CreatedAt}
	if s.dialect == DialectPostgres {
		err = tx.QueryRowContext(ctx, ins+` RETURNING id`, args...).Scan(&msg.ID)
	} else {
		res, e := tx.ExecContext(ctx, ins, args...)
		if e != nil {
			return nil, mapErr(e)
		}
		msg.ID, err = res.LastInsertId()
	}
	if err != nil {
		return nil, mapErr(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *Store) GetMessageByID(ctx context.Context, id int64) (*Message, error) {
	m := &Message{}
	q := s.rebind(`SELECT id, mailbox_id, uid, size, flags, internal_date, file_path, message_id, created_at
		FROM messages WHERE id = ?`)
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&m.ID, &m.MailboxID, &m.UID, &m.Size, &m.Flags, &m.InternalDate, &m.FilePath, &m.MessageID, &m.CreatedAt,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	return m, nil
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

package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

var ErrAliasInvalid = errors.New("invalid alias")

type DomainAlias struct {
	ID        string    `json:"id"`
	DomainID  string    `json:"domain_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// NormalizeAliasDomain uses the DNS spelling for international domain names.
func NormalizeAliasDomain(name string) (string, error) {
	name, err := idna.Lookup.ToASCII(strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), "."))
	if err != nil || len(name) > 253 || !strings.Contains(name, ".") {
		return "", ErrAliasInvalid
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrAliasInvalid
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", ErrAliasInvalid
			}
		}
	}
	return name, nil
}

// Serialize creation in the shared namespace of real domains and aliases.
func (s *Store) lockDomainNames(ctx context.Context, tx *sql.Tx) error {
	if s.dialect == DialectPostgres {
		_, err := tx.ExecContext(ctx, `LOCK TABLE domains IN SHARE ROW EXCLUSIVE MODE`)
		return err
	}
	return nil
}

func (s *Store) ListDomainAliases(ctx context.Context, domainID string) ([]*DomainAlias, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id,domain_id,name,created_at FROM domain_aliases WHERE domain_id=? ORDER BY name`), domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*DomainAlias{}
	for rows.Next() {
		a := &DomainAlias{}
		if err = rows.Scan(&a.ID, &a.DomainID, &a.Name, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) SaveDomainAlias(ctx context.Context, a *DomainAlias) (*DomainAlias, error) {
	if a == nil {
		return nil, ErrAliasInvalid
	}
	v := *a
	var err error
	v.Name, err = NormalizeAliasDomain(v.Name)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.lockDomainNames(ctx, tx); err != nil {
		return nil, err
	}
	if err = s.lockDirectoryDomain(ctx, tx, v.DomainID); err != nil {
		return nil, err
	}
	var n int
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT (SELECT COUNT(*) FROM domains WHERE LOWER(name)=?)+(SELECT COUNT(*) FROM domain_aliases WHERE name=? AND id<>?)`), v.Name, v.Name, v.ID).Scan(&n)
	if err != nil {
		return nil, err
	}
	if n != 0 {
		return nil, ErrAlreadyExists
	}
	if v.ID == "" {
		v.ID = NewID()
		v.CreatedAt = time.Now().UTC()
		if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM settings WHERE key=?`), "dkim.domain."+v.Name); err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO domain_aliases(id,domain_id,name,created_at) VALUES(?,?,?,?)`), v.ID, v.DomainID, v.Name, v.CreatedAt)
	} else {
		var oldName string
		err = tx.QueryRowContext(ctx, s.rebind(`SELECT created_at,name FROM domain_aliases WHERE id=? AND domain_id=?`), v.ID, v.DomainID).Scan(&v.CreatedAt, &oldName)
		if err != nil {
			return nil, mapErr(err)
		}
		if oldName != v.Name {
			if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM settings WHERE key=? OR key=?`), "dkim.domain."+oldName, "dkim.domain."+v.Name); err != nil {
				return nil, err
			}
		}
		_, err = tx.ExecContext(ctx, s.rebind(`UPDATE domain_aliases SET name=? WHERE id=? AND domain_id=?`), v.Name, v.ID, v.DomainID)
	}
	if err != nil {
		return nil, mapErr(err)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Store) DeleteDomainAlias(ctx context.Context, domainID, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockDomainNames(ctx, tx); err != nil {
		return err
	}
	if err = s.lockDirectoryDomain(ctx, tx, domainID); err != nil {
		return err
	}
	var name string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT name FROM domain_aliases WHERE domain_id=? AND id=?`), domainID, id).Scan(&name); err != nil {
		return mapErr(err)
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM settings WHERE key=?`), "dkim.domain."+name); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM domain_aliases WHERE domain_id=? AND id=?`), domainID, id); err != nil {
		return err
	}
	return tx.Commit()
}
func aliasDeleteResult(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ResolveMailDomain leaves provisioning/login lookups on the canonical domains.
func (s *Store) ResolveMailDomain(ctx context.Context, name string) (*Domain, error) {
	if normalized, err := NormalizeAliasDomain(name); err == nil {
		name = normalized
	}
	d, err := s.GetDomainByName(ctx, name)
	if err == nil || !errors.Is(err, ErrNotFound) {
		return d, err
	}
	var id string
	err = s.db.QueryRowContext(ctx, s.rebind(`SELECT domain_id FROM domain_aliases WHERE name=?`), strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))).Scan(&id)
	if err != nil {
		return nil, mapErr(err)
	}
	return s.GetDomainByID(ctx, id)
}

func (s *Store) ListAliases(ctx context.Context, userID string) ([]*Alias, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT a.id,a.domain_id,a.user_id,a.local_part,a.created_at,d.name FROM aliases a JOIN domains d ON d.id=a.domain_id WHERE a.user_id=? ORDER BY d.name,a.local_part`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Alias{}
	for rows.Next() {
		a := &Alias{}
		var domain string
		if err = rows.Scan(&a.ID, &a.DomainID, &a.UserID, &a.LocalPart, &a.CreatedAt, &domain); err != nil {
			return nil, err
		}
		a.Email = a.LocalPart + "@" + domain
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) SaveAlias(ctx context.Context, a *Alias) (*Alias, error) {
	if a == nil {
		return nil, ErrAliasInvalid
	}
	v := *a
	v.LocalPart = strings.ToLower(strings.TrimSpace(v.LocalPart))
	if len(v.LocalPart) > 64 || !groupLocalPart.MatchString(v.LocalPart) {
		return nil, ErrAliasInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.lockDirectoryDomain(ctx, tx, v.DomainID); err != nil {
		return nil, err
	}
	var domain, tenant, userTenant, source string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT name,tenant_id FROM domains WHERE id=?`), v.DomainID).Scan(&domain, &tenant); err != nil {
		return nil, mapErr(err)
	}
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT tenant_id,auth_source FROM users WHERE id=?`), v.UserID).Scan(&userTenant, &source); err != nil {
		return nil, mapErr(err)
	}
	if tenant != userTenant || source == "resource" || len(v.LocalPart)+1+len(domain) > 254 {
		return nil, ErrAliasInvalid
	}
	var n int
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT (SELECT COUNT(*) FROM users WHERE domain_id=? AND LOWER(local_part)=?)+(SELECT COUNT(*) FROM aliases WHERE domain_id=? AND LOWER(local_part)=? AND id<>?)+(SELECT COUNT(*) FROM directory_groups WHERE domain_id=? AND local_part=?)`), v.DomainID, v.LocalPart, v.DomainID, v.LocalPart, v.ID, v.DomainID, v.LocalPart).Scan(&n)
	if err != nil {
		return nil, err
	}
	if n != 0 {
		return nil, ErrAlreadyExists
	}
	if v.ID == "" {
		v.ID = NewID()
		v.CreatedAt = time.Now().UTC()
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO aliases(id,domain_id,user_id,local_part,created_at) VALUES(?,?,?,?,?)`), v.ID, v.DomainID, v.UserID, v.LocalPart, v.CreatedAt)
	} else {
		err = tx.QueryRowContext(ctx, s.rebind(`SELECT created_at FROM aliases WHERE id=? AND domain_id=? AND user_id=?`), v.ID, v.DomainID, v.UserID).Scan(&v.CreatedAt)
		if err != nil {
			return nil, mapErr(err)
		}
		_, err = tx.ExecContext(ctx, s.rebind(`UPDATE aliases SET local_part=? WHERE id=? AND domain_id=? AND user_id=?`), v.LocalPart, v.ID, v.DomainID, v.UserID)
	}
	if err != nil {
		return nil, mapErr(err)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	v.Email = v.LocalPart + "@" + domain
	return &v, nil
}
func (s *Store) DeleteAlias(ctx context.Context, domainID, userID, id string) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM aliases WHERE domain_id=? AND user_id=? AND id=?`), domainID, userID, id)
	return aliasDeleteResult(result, err)
}

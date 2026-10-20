package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrDirectoryConflict = errors.New("directory changed; reload before saving")
	ErrDirectoryInvalid  = errors.New("invalid department")
	ErrDirectoryChildren = errors.New("department contains child departments")
	ErrDirectoryManaged  = errors.New("directory is managed by LDAP; select manual mode to edit")
	ErrNoRecipients      = errors.New("distribution group has no active recipients")
	groupLocalPart       = regexp.MustCompile(`^[a-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+)*$`)
)

// A department is a distribution group. Its recipients are the union of direct
// members and descendants' members; membership never grants administrative rights.
type DirectoryGroup struct {
	ID          string   `json:"id"`
	DomainID    string   `json:"domain_id"`
	ParentID    string   `json:"parent_id"`
	LocalPart   string   `json:"local_part"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Email       string   `json:"email"`
	Revision    int64    `json:"revision"`
	MemberIDs   []string `json:"member_ids"`
	ExternalID  string   `json:"-"`
}

// Domain locking serializes hierarchy changes and all mailbox/alias creations.
// SQLite has one connection per Store and serializes writes; PostgreSQL needs
// an explicit lock to prevent simultaneous cross-table address collisions.
func (s *Store) lockDirectoryDomain(ctx context.Context, tx *sql.Tx, domainID string) error {
	q := `SELECT id FROM domains WHERE id=?`
	if s.dialect == DialectPostgres {
		q += ` FOR UPDATE`
	}
	var id string
	return mapErr(tx.QueryRowContext(ctx, s.rebind(q), domainID).Scan(&id))
}

func (s *Store) checkGroupAddress(ctx context.Context, tx *sql.Tx, domainID, local string) error {
	var count int
	if err := tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM directory_groups WHERE domain_id=? AND local_part=?`), domainID, strings.ToLower(local)).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return ErrAlreadyExists
	}
	return nil
}

func (s *Store) ListDirectoryGroups(ctx context.Context, domainID string) ([]*DirectoryGroup, error) {
	return s.listDirectoryGroups(ctx, s.db, domainID)
}

type directoryQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Store) listDirectoryGroups(ctx context.Context, db directoryQueryer, domainID string) ([]*DirectoryGroup, error) {
	// A single query gives a consistent hierarchy/membership snapshot.
	rows, err := db.QueryContext(ctx, s.rebind(`SELECT g.id,g.domain_id,COALESCE(g.parent_id,''),g.local_part,g.name,g.description,
		g.revision,d.name,COALESCE(m.user_id,''),g.external_id FROM directory_groups g
		JOIN domains d ON d.id=g.domain_id LEFT JOIN directory_group_members m ON m.group_id=g.id
		WHERE g.domain_id=? ORDER BY g.name,g.id,m.user_id`), domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*DirectoryGroup{}
	byID := map[string]*DirectoryGroup{}
	for rows.Next() {
		g := &DirectoryGroup{MemberIDs: []string{}}
		var domain, member string
		if err := rows.Scan(&g.ID, &g.DomainID, &g.ParentID, &g.LocalPart, &g.Name, &g.Description, &g.Revision, &domain, &member, &g.ExternalID); err != nil {
			return nil, err
		}
		g.Email = g.LocalPart + "@" + domain
		if existing := byID[g.ID]; existing != nil {
			g = existing
		} else {
			byID[g.ID] = g
			out = append(out, g)
		}
		if member != "" {
			g.MemberIDs = append(g.MemberIDs, member)
		}
	}
	return out, rows.Err()
}

func (s *Store) SaveDirectoryGroup(ctx context.Context, input *DirectoryGroup) (*DirectoryGroup, error) {
	if input == nil {
		return nil, ErrDirectoryInvalid
	}
	g := *input
	g.Name, g.Description = strings.TrimSpace(g.Name), strings.TrimSpace(g.Description)
	g.LocalPart = strings.ToLower(strings.TrimSpace(g.LocalPart))
	if g.Name == "" || len(g.Name) > 200 || len(g.Description) > 2000 || len(g.LocalPart) > 64 ||
		!groupLocalPart.MatchString(g.LocalPart) || len(g.MemberIDs) > 10000 {
		return nil, ErrDirectoryInvalid
	}
	create := g.ID == ""
	if create {
		g.ID = NewID()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.lockDirectoryDomain(ctx, tx, g.DomainID); err != nil {
		return nil, err
	}
	if err = s.requireManualDirectory(ctx, tx, g.DomainID); err != nil {
		return nil, err
	}
	parents := map[string]string{}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT id,COALESCE(parent_id,'') FROM directory_groups WHERE domain_id=?`), g.DomainID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, parent string
		if err = rows.Scan(&id, &parent); err != nil {
			rows.Close()
			return nil, err
		}
		parents[id] = parent
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if create && len(parents) >= 1000 {
		return nil, fmt.Errorf("%w: maximum 1000 departments per domain", ErrDirectoryInvalid)
	}
	if !create {
		if _, ok := parents[g.ID]; !ok {
			return nil, ErrNotFound
		}
	}
	if g.ParentID != "" {
		if _, ok := parents[g.ParentID]; !ok {
			return nil, fmt.Errorf("%w: parent must be in the same domain", ErrDirectoryInvalid)
		}
	}
	parents[g.ID] = g.ParentID
	for id := range parents {
		seen := map[string]bool{}
		for p := id; p != ""; p = parents[p] {
			if seen[p] || len(seen) >= 32 {
				return nil, fmt.Errorf("%w: cycle or hierarchy deeper than 32 levels", ErrDirectoryInvalid)
			}
			seen[p] = true
		}
	}
	var count int
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT
		(SELECT COUNT(*) FROM users WHERE domain_id=? AND LOWER(local_part)=?) +
		(SELECT COUNT(*) FROM aliases WHERE domain_id=? AND LOWER(local_part)=?) +
		(SELECT COUNT(*) FROM directory_groups WHERE domain_id=? AND local_part=? AND id<>?)`),
		g.DomainID, g.LocalPart, g.DomainID, g.LocalPart, g.DomainID, g.LocalPart, g.ID).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count != 0 {
		return nil, ErrAlreadyExists
	}
	var domain string
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT name FROM domains WHERE id=?`), g.DomainID).Scan(&domain); err != nil {
		return nil, err
	}
	g.Email = g.LocalPart + "@" + domain
	if create {
		g.Revision = 1
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO directory_groups(id,domain_id,parent_id,local_part,name,description,revision)
			VALUES (?,?,NULLIF(?,''),?,?,?,1)`), g.ID, g.DomainID, g.ParentID, g.LocalPart, g.Name, g.Description)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, s.rebind(`UPDATE directory_groups SET parent_id=NULLIF(?,''),local_part=?,name=?,description=?,revision=revision+1
			WHERE id=? AND domain_id=? AND revision=?`), g.ParentID, g.LocalPart, g.Name, g.Description, g.ID, g.DomainID, g.Revision)
		if err == nil {
			n, e := result.RowsAffected()
			if e != nil {
				return nil, e
			}
			if n != 1 {
				return nil, ErrDirectoryConflict
			}
			g.Revision++
		}
	}
	if err != nil {
		return nil, mapErr(err)
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM directory_group_members WHERE group_id=?`), g.ID); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	g.MemberIDs = []string{}
	for _, id := range input.MemberIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		// INSERT SELECT both validates ownership and locks the FK against deletion.
		result, e := tx.ExecContext(ctx, s.rebind(`INSERT INTO directory_group_members(group_id,user_id)
			SELECT ?,id FROM users WHERE id=? AND domain_id=? AND auth_source<>'resource'`), g.ID, id, g.DomainID)
		if e != nil {
			return nil, e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return nil, e
		}
		if n != 1 {
			return nil, fmt.Errorf("%w: member must be a mailbox in this domain", ErrDirectoryInvalid)
		}
		g.MemberIDs = append(g.MemberIDs, id)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Store) DeleteDirectoryGroup(ctx context.Context, domainID, id string, revision int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockDirectoryDomain(ctx, tx, domainID); err != nil {
		return err
	}
	if err = s.requireManualDirectory(ctx, tx, domainID); err != nil {
		return err
	}
	var current int64
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT revision FROM directory_groups WHERE id=? AND domain_id=?`), id, domainID).Scan(&current); err != nil {
		return mapErr(err)
	}
	if current != revision {
		return ErrDirectoryConflict
	}
	var children int
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM directory_groups WHERE parent_id=?`), id).Scan(&children); err != nil {
		return err
	}
	if children != 0 {
		return ErrDirectoryChildren
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM directory_groups WHERE id=? AND domain_id=?`), id, domainID); err != nil {
		return err
	}
	return tx.Commit()
}

// ResolveRecipients expands a local distribution address. Keep ResolveRecipient
// for identity lookups (login, DSNs, calendar owners), where a group is not a user.
func (s *Store) ResolveRecipients(ctx context.Context, email string) ([]*User, error) {
	user, err := s.ResolveRecipient(ctx, email)
	if err == nil {
		return []*User{user}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return nil, ErrNotFound
	}
	var id, domainID string
	if err = s.db.QueryRowContext(ctx, s.rebind(`SELECT g.id,g.domain_id FROM directory_groups g JOIN domains d ON d.id=g.domain_id WHERE g.local_part=? AND LOWER(d.name)=?`), email[:at], email[at+1:]).Scan(&id, &domainID); err != nil {
		return nil, mapErr(err)
	}
	// UNION (not UNION ALL) bounds traversal even if corrupted data contains a cycle.
	rows, err := s.db.QueryContext(ctx, s.rebind(`WITH RECURSIVE departments(id) AS (
		SELECT id FROM directory_groups WHERE id=? AND domain_id=?
		UNION SELECT g.id FROM directory_groups g JOIN departments p ON g.parent_id=p.id WHERE g.domain_id=?
	) SELECT DISTINCT m.user_id FROM directory_group_members m JOIN departments d ON d.id=m.group_id`), id, domainID, domainID)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var member string
		if err = rows.Scan(&member); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, member)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	users, err := s.ListUsersByDomain(ctx, domainID)
	if err != nil {
		return nil, err
	}
	members := map[string]bool{}
	for _, id := range ids {
		members[id] = true
	}
	out := []*User{}
	for _, user := range users {
		if members[user.ID] && user.Enabled && user.AuthSource != "resource" {
			out = append(out, user)
		}
	}
	if len(out) == 0 {
		return nil, ErrNoRecipients
	}
	return out, nil
}

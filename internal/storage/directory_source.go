package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DirectorySource contains no connection credentials. LDAP connection and read
// permissions come exclusively from the domain authentication configuration.
type DirectorySource struct {
	Mode            string    `json:"mode"`
	BaseDN          string    `json:"base_dn"`
	Filter          string    `json:"filter"`
	NameAttr        string    `json:"name_attr"`
	EmailAttr       string    `json:"email_attr"`
	MemberAttr      string    `json:"member_attr"`
	IntervalMinutes int       `json:"interval_minutes"`
	Revision        int64     `json:"revision"`
	LastAttempt     time.Time `json:"last_attempt,omitzero"`
	LastSuccess     time.Time `json:"last_success,omitzero"`
	LastError       string    `json:"last_error,omitempty"`
}

type DirectoryImportGroup struct {
	ExternalID, ParentExternalID, LocalPart, Name, Description string
	MemberIDs                                                  []string
}

func DefaultDirectorySource() DirectorySource {
	return DirectorySource{Mode: "manual", Filter: "(|(objectClass=group)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames))", NameAttr: "cn", EmailAttr: "mail", MemberAttr: "member"}
}

func directorySourceKey(domainID string) string { return "domain.directory.source." + domainID }

func decodeDirectorySource(raw string) (*DirectorySource, error) {
	source := DefaultDirectorySource()
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &source); err != nil {
			return nil, err
		}
	}
	return &source, nil
}

func (s *Store) GetDirectorySource(ctx context.Context, domainID string) (*DirectorySource, error) {
	raw, _, err := s.GetSetting(ctx, directorySourceKey(domainID))
	if err != nil {
		return nil, err
	}
	return decodeDirectorySource(raw)
}

func (s *Store) directorySourceTx(ctx context.Context, tx *sql.Tx, domainID string) (*DirectorySource, error) {
	var raw string
	err := tx.QueryRowContext(ctx, s.rebind(`SELECT value FROM settings WHERE key=?`), directorySourceKey(domainID)).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return decodeDirectorySource(raw)
}

func (s *Store) putDirectorySourceTx(ctx context.Context, tx *sql.Tx, domainID string, source *DirectorySource) error {
	raw, err := json.Marshal(source)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO settings(key,value,updated_at) VALUES (?,?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`), directorySourceKey(domainID), string(raw), time.Now().UTC())
	return err
}

var directoryAttribute = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]{0,63}$`)

func (s *Store) SaveDirectorySource(ctx context.Context, domainID string, source DirectorySource) (*DirectorySource, error) {
	source.BaseDN = strings.TrimSpace(source.BaseDN)
	if (source.Mode != "manual" && source.Mode != "ldap") || len(source.BaseDN) > 2000 || len(source.Filter) > 2000 ||
		source.IntervalMinutes < 0 || source.IntervalMinutes > 1440 || source.IntervalMinutes > 0 && source.IntervalMinutes < 5 {
		return nil, ErrDirectoryInvalid
	}
	if source.Mode == "ldap" && (source.BaseDN == "" || source.Filter == "" || !directoryAttribute.MatchString(source.NameAttr) ||
		!directoryAttribute.MatchString(source.EmailAttr) || !directoryAttribute.MatchString(source.MemberAttr)) {
		return nil, ErrDirectoryInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.lockDirectoryDomain(ctx, tx, domainID); err != nil {
		return nil, err
	}
	current, err := s.directorySourceTx(ctx, tx, domainID)
	if err != nil {
		return nil, err
	}
	if source.Revision != current.Revision {
		return nil, ErrDirectoryConflict
	}
	source.Revision++
	source.LastAttempt, source.LastSuccess, source.LastError = current.LastAttempt, current.LastSuccess, current.LastError
	if err = s.putDirectorySourceTx(ctx, tx, domainID, &source); err != nil {
		return nil, err
	}
	return &source, tx.Commit()
}

func (s *Store) requireManualDirectory(ctx context.Context, tx *sql.Tx, domainID string) error {
	source, err := s.directorySourceTx(ctx, tx, domainID)
	if err != nil {
		return err
	}
	if source.Mode == "ldap" {
		return ErrDirectoryManaged
	}
	return nil
}

// ApplyDirectoryImport validates the complete merged snapshot before publishing
// it in one transaction. Manual groups and their memberships are retained.
func (s *Store) ApplyDirectoryImport(ctx context.Context, domainID string, revision int64, imported []DirectoryImportGroup) (*DirectorySource, error) {
	return s.importDirectory(ctx, domainID, revision, imported, true)
}

func (s *Store) ValidateDirectoryImport(ctx context.Context, domainID string, revision int64, imported []DirectoryImportGroup) error {
	_, err := s.importDirectory(ctx, domainID, revision, imported, false)
	return err
}

func (s *Store) importDirectory(ctx context.Context, domainID string, revision int64, imported []DirectoryImportGroup, apply bool) (*DirectorySource, error) {
	if len(imported) == 0 || len(imported) > 1000 {
		return nil, fmt.Errorf("%w: LDAP snapshot must contain 1–1000 groups", ErrDirectoryInvalid)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = s.lockDirectoryDomain(ctx, tx, domainID); err != nil {
		return nil, err
	}
	source, err := s.directorySourceTx(ctx, tx, domainID)
	if err != nil {
		return nil, err
	}
	if source.Mode != "ldap" || source.Revision != revision {
		return nil, ErrDirectoryConflict
	}
	old, err := s.listDirectoryGroups(ctx, tx, domainID)
	if err != nil {
		return nil, err
	}
	external := map[string]*DirectoryGroup{}
	all := []*DirectoryGroup{}
	for _, group := range old {
		if group.ExternalID == "" {
			all = append(all, group)
		} else {
			external[group.ExternalID] = group
		}
	}
	byExternal := map[string]*DirectoryGroup{}
	for _, item := range imported {
		if item.ExternalID == "" || len(item.ExternalID) > 4000 || byExternal[item.ExternalID] != nil {
			return nil, fmt.Errorf("%w: duplicate or invalid LDAP identity", ErrDirectoryInvalid)
		}
		g := &DirectoryGroup{ID: NewID(), DomainID: domainID, ExternalID: item.ExternalID, Name: strings.TrimSpace(item.Name),
			LocalPart: strings.ToLower(strings.TrimSpace(item.LocalPart)), Description: strings.TrimSpace(item.Description), MemberIDs: item.MemberIDs, Revision: 1}
		if previous := external[item.ExternalID]; previous != nil {
			g.ID, g.Revision = previous.ID, previous.Revision+1
		}
		byExternal[item.ExternalID] = g
		all = append(all, g)
	}
	for _, item := range imported {
		if item.ParentExternalID != "" {
			parent := byExternal[item.ParentExternalID]
			if parent == nil {
				return nil, fmt.Errorf("%w: missing LDAP parent", ErrDirectoryInvalid)
			}
			byExternal[item.ExternalID].ParentID = parent.ID
		}
	}
	if len(all) > 1000 {
		return nil, fmt.Errorf("%w: maximum 1000 departments per domain", ErrDirectoryInvalid)
	}
	members := map[string]bool{}
	addresses := map[string]bool{}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT id,LOWER(local_part),auth_source FROM users WHERE domain_id=?`), domainID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, local, auth string
		if err = rows.Scan(&id, &local, &auth); err != nil {
			rows.Close()
			return nil, err
		}
		members[id] = auth != "resource"
		addresses[local] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, s.rebind(`SELECT LOWER(local_part) FROM aliases WHERE domain_id=?`), domainID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var local string
		if err = rows.Scan(&local); err != nil {
			rows.Close()
			return nil, err
		}
		addresses[local] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	parents := map[string]string{}
	total := 0
	for _, g := range all {
		if g.Name == "" || len(g.Name) > 200 || len(g.Description) > 2000 || len(g.LocalPart) > 64 || !groupLocalPart.MatchString(g.LocalPart) || len(g.MemberIDs) > 10000 {
			return nil, ErrDirectoryInvalid
		}
		if addresses[g.LocalPart] {
			return nil, fmt.Errorf("%w: group address %s is occupied", ErrAlreadyExists, g.LocalPart)
		}
		addresses[g.LocalPart] = true
		parents[g.ID] = g.ParentID
		unique := map[string]bool{}
		ids := []string{}
		for _, id := range g.MemberIDs {
			if !members[id] {
				return nil, fmt.Errorf("%w: member must be a mailbox in this domain", ErrDirectoryInvalid)
			}
			if !unique[id] {
				ids = append(ids, id)
				unique[id] = true
			}
		}
		g.MemberIDs = ids
		total += len(ids)
	}
	if total > 100000 {
		return nil, fmt.Errorf("%w: too many memberships", ErrDirectoryInvalid)
	}
	for id := range parents {
		seen := map[string]bool{}
		for p := id; p != ""; p = parents[p] {
			if _, ok := parents[p]; !ok {
				return nil, fmt.Errorf("%w: a retained manual group refers to a removed LDAP parent", ErrDirectoryInvalid)
			}
			if seen[p] || len(seen) >= 32 {
				return nil, fmt.Errorf("%w: cycle or hierarchy deeper than 32 levels", ErrDirectoryInvalid)
			}
			seen[p] = true
		}
	}
	if !apply {
		return source, nil
	}
	// Retain IDs across imports, including renames and address swaps. Only the
	// private hierarchy/member tables are rebuilt; no mailbox data is touched.
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE directory_groups SET parent_id=NULL WHERE domain_id=?`), domainID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM directory_groups WHERE domain_id=?`), domainID); err != nil {
		return nil, err
	}
	for _, g := range all {
		_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO directory_groups(id,domain_id,local_part,name,description,revision,external_id) VALUES(?,?,?,?,?,?,?)`),
			g.ID, domainID, g.LocalPart, g.Name, g.Description, g.Revision, g.ExternalID)
		if err != nil {
			return nil, err
		}
	}
	for _, g := range all {
		if g.ParentID != "" {
			if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE directory_groups SET parent_id=? WHERE id=?`), g.ParentID, g.ID); err != nil {
				return nil, err
			}
		}
		for _, id := range g.MemberIDs {
			if _, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO directory_group_members(group_id,user_id) VALUES(?,?)`), g.ID, id); err != nil {
				return nil, err
			}
		}
	}
	source.Revision++
	source.LastAttempt = time.Now().UTC()
	source.LastSuccess, source.LastError = source.LastAttempt, ""
	if err = s.putDirectorySourceTx(ctx, tx, domainID, source); err != nil {
		return nil, err
	}
	return source, tx.Commit()
}

func (s *Store) RecordDirectoryImportError(ctx context.Context, domainID string, revision int64, message string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockDirectoryDomain(ctx, tx, domainID); err != nil {
		return err
	}
	source, err := s.directorySourceTx(ctx, tx, domainID)
	if err != nil {
		return err
	}
	if source.Mode != "ldap" || source.Revision != revision {
		return ErrDirectoryConflict
	}
	source.LastAttempt, source.LastError = time.Now().UTC(), message
	source.Revision++
	if err = s.putDirectorySourceTx(ctx, tx, domainID, source); err != nil {
		return err
	}
	return tx.Commit()
}

package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrFileSpaceInvalid = errors.New("invalid file space")
var ErrFileSpaceConflict = errors.New("file space changed; reload before saving")
var ErrFileSpaceForbidden = errors.New("file space management required")

type FileSpaceGroup struct {
	GroupID string `json:"group_id"`
	Name    string `json:"name,omitempty"`
	Rights  string `json:"rights"`
}
type FileSpace struct {
	ID           string           `json:"id"`
	TenantID     string           `json:"tenant_id"`
	Name         string           `json:"name"`
	Kind         string           `json:"kind"`
	Description  string           `json:"description"`
	QuotaBytes   int64            `json:"quota_bytes"`
	VersionDays  int              `json:"version_days"`
	TrashDays    int              `json:"trash_days"`
	KeepVersions int              `json:"keep_versions"`
	Automatic    bool             `json:"automatic"`
	Revision     int64            `json:"revision"`
	Generation   int64            `json:"generation"`
	CreatedAt    int64            `json:"created_at"`
	Groups       []FileSpaceGroup `json:"groups"`
	Rights       string           `json:"rights,omitempty"`
}

const fileSpaceColumns = "id,tenant_id,name,kind,description,quota_bytes,version_days,trash_days,keep_versions,automatic,revision,created_at,generation"

func scanFileSpace(row interface{ Scan(...any) error }) (*FileSpace, error) {
	v := &FileSpace{Groups: []FileSpaceGroup{}}
	err := row.Scan(&v.ID, &v.TenantID, &v.Name, &v.Kind, &v.Description, &v.QuotaBytes, &v.VersionDays, &v.TrashDays, &v.KeepVersions, &v.Automatic, &v.Revision, &v.CreatedAt, &v.Generation)
	return v, mapErr(err)
}
func (s *Store) GetFileSpace(ctx context.Context, id string) (*FileSpace, error) {
	v, err := scanFileSpace(s.db.QueryRowContext(ctx, s.rebind("SELECT "+fileSpaceColumns+" FROM file_spaces WHERE id=?"), id))
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, s.rebind("SELECT a.group_id,g.name,a.rights FROM file_space_groups a JOIN directory_groups g ON g.id=a.group_id WHERE a.space_id=? ORDER BY g.name,g.id"), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var g FileSpaceGroup
		if err = rows.Scan(&g.GroupID, &g.Name, &g.Rights); err != nil {
			return nil, err
		}
		v.Groups = append(v.Groups, g)
	}
	return v, rows.Err()
}

type fileSpaceQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) fileSpaceRights(ctx context.Context, db fileSpaceQueryer, id, userID string) (string, error) {
	var tenant, roles string
	var enabled bool
	if err := db.QueryRowContext(ctx, s.rebind("SELECT tenant_id,roles,enabled FROM users WHERE id=?"), userID).Scan(&tenant, &roles, &enabled); err != nil {
		return "", mapErr(err)
	}
	if !enabled {
		return "", nil
	}
	var spaceTenant string
	if err := db.QueryRowContext(ctx, s.rebind("SELECT tenant_id FROM file_spaces WHERE id=?"), id).Scan(&spaceTenant); err != nil {
		return "", mapErr(err)
	}
	if IsGlobalAdmin(roles) {
		return "manage", nil
	}
	if tenant != spaceTenant {
		return "", nil
	}
	if HasRole(roles, RoleTenantAdmin) {
		return "manage", nil
	}
	var rank int
	err := db.QueryRowContext(ctx, s.rebind(`WITH RECURSIVE membership(id) AS (
 SELECT group_id FROM directory_group_members WHERE user_id=?
 UNION SELECT g.parent_id FROM directory_groups g JOIN membership m ON g.id=m.id WHERE g.parent_id IS NOT NULL
 ) SELECT COALESCE(MAX(CASE a.rights WHEN 'manage' THEN 3 WHEN 'write' THEN 2 ELSE 1 END),0)
 FROM file_space_groups a JOIN membership m ON m.id=a.group_id WHERE a.space_id=?`), userID, id).Scan(&rank)
	return map[int]string{0: "", 1: "read", 2: "write", 3: "manage"}[rank], err
}
func (s *Store) FileSpaceRights(ctx context.Context, id, userID string) (string, error) {
	return s.fileSpaceRights(ctx, s.db, id, userID)
}
func (s *Store) ListFileSpaces(ctx context.Context, userID string) ([]*FileSpace, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.Enabled {
		return []*FileSpace{}, nil
	}
	q := "SELECT id FROM file_spaces"
	args := []any{}
	if !IsGlobalAdmin(user.Roles) {
		q += " WHERE tenant_id=?"
		args = append(args, user.TenantID)
	}
	q += " ORDER BY name,id"
	rows, err := s.db.QueryContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []*FileSpace{}
	for _, id := range ids {
		rights, err := s.FileSpaceRights(ctx, id, userID)
		if err != nil {
			return nil, err
		}
		if rights == "" {
			continue
		}
		v, err := s.GetFileSpace(ctx, id)
		if err != nil {
			return nil, err
		}
		v.Rights = rights
		out = append(out, v)
	}
	return out, nil
}
func (s *Store) SaveFileSpace(ctx context.Context, actor string, input *FileSpace) (*FileSpace, error) {
	if input == nil {
		return nil, ErrFileSpaceInvalid
	}
	v := *input
	v.Name = strings.TrimSpace(v.Name)
	v.Description = strings.TrimSpace(v.Description)
	if v.Name == "" || len(v.Name) > 200 || len(v.Description) > 2000 || (v.Kind != "department" && v.Kind != "project") || v.QuotaBytes < 0 || v.VersionDays < 0 || v.VersionDays > 36500 || v.TrashDays < 0 || v.TrashDays > 36500 || v.KeepVersions < 0 || v.KeepVersions > 10000 || len(v.Groups) > 1000 {
		return nil, ErrFileSpaceInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if v.ID == "" {
		var tenant, roles string
		var enabled bool
		if err = tx.QueryRowContext(ctx, s.rebind("SELECT tenant_id,roles,enabled FROM users WHERE id=?"), actor).Scan(&tenant, &roles, &enabled); err != nil {
			return nil, mapErr(err)
		}
		if !enabled || (!IsGlobalAdmin(roles) && (!HasRole(roles, RoleTenantAdmin) || tenant != v.TenantID)) {
			return nil, ErrFileSpaceForbidden
		}
		v.ID = NewID()
		v.Revision = 1
		v.CreatedAt = time.Now().UTC().UnixNano()
		_, err = tx.ExecContext(ctx, s.rebind("INSERT INTO file_spaces("+fileSpaceColumns+") VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)"), v.ID, v.TenantID, v.Name, v.Kind, v.Description, v.QuotaBytes, v.VersionDays, v.TrashDays, v.KeepVersions, v.Automatic, v.Revision, v.CreatedAt, 0)
	} else {
		rights, e := s.fileSpaceRights(ctx, tx, v.ID, actor)
		if e != nil {
			return nil, e
		}
		if rights != "manage" {
			return nil, ErrFileSpaceForbidden
		}
		result, e := tx.ExecContext(ctx, s.rebind("UPDATE file_spaces SET name=?,kind=?,description=?,quota_bytes=?,version_days=?,trash_days=?,keep_versions=?,automatic=?,revision=revision+1 WHERE id=? AND tenant_id=? AND revision=?"), v.Name, v.Kind, v.Description, v.QuotaBytes, v.VersionDays, v.TrashDays, v.KeepVersions, v.Automatic, v.ID, v.TenantID, v.Revision)
		err = e
		if err == nil {
			n, _ := result.RowsAffected()
			if n != 1 {
				return nil, ErrFileSpaceConflict
			}
		}
		v.Revision++
	}
	if err != nil {
		return nil, mapErr(err)
	}
	if _, err = tx.ExecContext(ctx, s.rebind("DELETE FROM file_space_groups WHERE space_id=?"), v.ID); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, g := range v.Groups {
		if seen[g.GroupID] || (g.Rights != "read" && g.Rights != "write" && g.Rights != "manage") {
			return nil, ErrFileSpaceInvalid
		}
		seen[g.GroupID] = true
		var tenant string
		if err = tx.QueryRowContext(ctx, s.rebind("SELECT d.tenant_id FROM directory_groups g JOIN domains d ON d.id=g.domain_id WHERE g.id=?"), g.GroupID).Scan(&tenant); err != nil || tenant != v.TenantID {
			return nil, ErrFileSpaceInvalid
		}
		if _, err = tx.ExecContext(ctx, s.rebind("INSERT INTO file_space_groups(space_id,group_id,rights) VALUES(?,?,?)"), v.ID, g.GroupID, g.Rights); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetFileSpace(ctx, v.ID)
}

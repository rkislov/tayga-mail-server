package storage

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"time"
)

// Directory identities are references, not an instruction to rewind global membership.
type FileSpaceUserRef struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	TenantID string `json:"tenant_id"`
}
type FileSpaceGroupRef struct {
	ID       string `json:"id"`
	DomainID string `json:"domain_id"`
	TenantID string `json:"tenant_id"`
}
type FileSpaceSnapshot struct {
	Space  *FileSpace          `json:"space"`
	Access []FileAccess        `json:"access"`
	Shares []*FileShare        `json:"shares"`
	Users  []FileSpaceUserRef  `json:"users"`
	Groups []FileSpaceGroupRef `json:"group_refs"`
}

func (s *Store) SnapshotFileSpace(ctx context.Context, id string) (*FileSpaceSnapshot, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	space, err := scanFileSpace(tx.QueryRowContext(ctx, s.rebind("SELECT "+fileSpaceColumns+" FROM file_spaces WHERE id=?"), id))
	if err != nil {
		return nil, err
	}
	out := &FileSpaceSnapshot{Space: space, Access: []FileAccess{}, Shares: []*FileShare{}, Users: []FileSpaceUserRef{}, Groups: []FileSpaceGroupRef{}}
	rows, err := tx.QueryContext(ctx, s.rebind("SELECT a.group_id,g.name,a.rights,g.domain_id,d.tenant_id FROM file_space_groups a JOIN directory_groups g ON g.id=a.group_id JOIN domains d ON d.id=g.domain_id WHERE a.space_id=? ORDER BY a.group_id"), id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var grant FileSpaceGroup
		var ref FileSpaceGroupRef
		if err = rows.Scan(&grant.GroupID, &grant.Name, &grant.Rights, &ref.DomainID, &ref.TenantID); err != nil {
			rows.Close()
			return nil, err
		}
		ref.ID = grant.GroupID
		space.Groups = append(space.Groups, grant)
		out.Groups = append(out.Groups, ref)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, s.rebind("SELECT a.owner_id,a.path,a.grantee_id,a.rights,u.email,u.tenant_id FROM file_acl a JOIN users u ON u.id=a.grantee_id WHERE a.owner_id=? ORDER BY a.path,a.grantee_id"), id)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for rows.Next() {
		var a FileAccess
		var ref FileSpaceUserRef
		if err = rows.Scan(&a.OwnerID, &a.Path, &a.GranteeID, &a.Rights, &ref.Email, &ref.TenantID); err != nil {
			rows.Close()
			return nil, err
		}
		out.Access = append(out.Access, a)
		ref.ID = a.GranteeID
		if !seen[ref.ID] {
			out.Users = append(out.Users, ref)
			seen[ref.ID] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, s.rebind("SELECT id,user_id,path,token,expires_at,max_downloads,download_count,created_at,password_hash,rights FROM file_shares WHERE user_id=? ORDER BY id"), id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		share, e := s.scanFileShare(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		out.Shares = append(out.Shares, share)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
func validSpaceSnapshotPath(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "../") && !strings.ContainsAny(value, "\\\x00") && path.Clean(value) == value
}
func ValidFileSpaceID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
		} else if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
func validateSpaceSnapshotShape(v *FileSpaceSnapshot) error {
	if v == nil || v.Space == nil {
		return ErrFileSpaceInvalid
	}
	s := v.Space
	if !ValidFileSpaceID(s.ID) || !ValidFileSpaceID(s.TenantID) || strings.TrimSpace(s.Name) == "" || len(s.Name) > 200 || len(s.Description) > 2000 || (s.Kind != "department" && s.Kind != "project") || s.Generation < 0 || s.Generation >= 9223372036854775806 || s.QuotaBytes < 0 || s.VersionDays < 0 || s.VersionDays > 36500 || s.TrashDays < 0 || s.TrashDays > 36500 || s.KeepVersions < 0 || s.KeepVersions > 10000 || len(s.Groups) > 1000 || len(v.Access) > 100000 || len(v.Shares) > 100000 {
		return ErrFileSpaceInvalid
	}
	groups := map[string]bool{}
	refs := map[string]bool{}
	for _, g := range v.Groups {
		if refs[g.ID] || g.TenantID != s.TenantID {
			return ErrFileSpaceInvalid
		}
		refs[g.ID] = true
	}
	for _, g := range s.Groups {
		if groups[g.GroupID] || !refs[g.GroupID] || (g.Rights != "read" && g.Rights != "write" && g.Rights != "manage") {
			return ErrFileSpaceInvalid
		}
		groups[g.GroupID] = true
	}
	users := map[string]bool{}
	for _, u := range v.Users {
		if users[u.ID] || u.TenantID != s.TenantID || !ValidFileSpaceID(u.ID) || u.Email == "" {
			return ErrFileSpaceInvalid
		}
		users[u.ID] = true
	}
	acl := map[string]bool{}
	for _, a := range v.Access {
		key := a.Path + "\x00" + a.GranteeID
		if a.OwnerID != s.ID || !users[a.GranteeID] || !validSpaceSnapshotPath(a.Path) || (a.Rights != "read" && a.Rights != "write") || acl[key] {
			return ErrFileSpaceInvalid
		}
		acl[key] = true
	}
	shares, tokens := map[string]bool{}, map[string]bool{}
	for _, a := range v.Shares {
		if a == nil || a.UserID != s.ID || !ValidFileSpaceID(a.ID) || !validSpaceSnapshotPath(a.Path) || len(a.Token) != 32 || strings.ContainsAny(a.Token, "/\\\x00 ") || (a.Rights != "read" && a.Rights != "write" && a.Rights != "upload") || a.MaxDownloads < 0 || a.DownloadCount < 0 || len(a.PasswordHash) > 4096 || shares[a.ID] || tokens[a.Token] {
			return ErrFileSpaceInvalid
		}
		shares[a.ID] = true
		tokens[a.Token] = true
	}
	return nil
}
func (s *Store) validateSpaceSnapshotRefs(ctx context.Context, db fileSpaceQueryer, v *FileSpaceSnapshot) error {
	if err := validateSpaceSnapshotShape(v); err != nil {
		return err
	}
	var tenant string
	if err := db.QueryRowContext(ctx, s.rebind("SELECT id FROM tenants WHERE id=?"), v.Space.TenantID).Scan(&tenant); err != nil {
		return mapErr(err)
	}
	var n int
	if err := db.QueryRowContext(ctx, s.rebind("SELECT COUNT(*) FROM users WHERE id=?"), v.Space.ID).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return ErrFileSpaceInvalid
	}
	var currentTenant string
	err := db.QueryRowContext(ctx, s.rebind("SELECT tenant_id FROM file_spaces WHERE id=?"), v.Space.ID).Scan(&currentTenant)
	if err == nil && currentTenant != v.Space.TenantID {
		return ErrFileSpaceInvalid
	}
	if err != nil && !errors.Is(mapErr(err), ErrNotFound) {
		return err
	}
	for _, ref := range v.Groups {
		var domain, tenant string
		if err := db.QueryRowContext(ctx, s.rebind("SELECT g.domain_id,d.tenant_id FROM directory_groups g JOIN domains d ON d.id=g.domain_id WHERE g.id=?"), ref.ID).Scan(&domain, &tenant); err != nil || domain != ref.DomainID || tenant != ref.TenantID {
			return ErrFileSpaceInvalid
		}
	}
	for _, ref := range v.Users {
		var email, tenant string
		if err := db.QueryRowContext(ctx, s.rebind("SELECT email,tenant_id FROM users WHERE id=?"), ref.ID).Scan(&email, &tenant); err != nil || email != ref.Email || tenant != ref.TenantID {
			return ErrFileSpaceInvalid
		}
	}
	return nil
}
func (s *Store) ValidateFileSpaceSnapshot(ctx context.Context, v *FileSpaceSnapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return s.validateSpaceSnapshotRefs(ctx, tx, v)
}
func (s *Store) FileSpaceRestoreCommitted(ctx context.Context, id string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, s.rebind("SELECT COUNT(*) FROM file_space_restores WHERE id=?"), id).Scan(&n)
	return n != 0, err
}

// Called only after a durable, authorized restore decision. The operation marker
// commits in the same transaction, so replay after an uncertain commit is safe.
func (s *Store) RestoreFileSpaceSnapshot(ctx context.Context, v *FileSpaceSnapshot, operation string) error {
	if !ValidFileSpaceID(operation) {
		return ErrFileSpaceInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var applied int
	if err = tx.QueryRowContext(ctx, s.rebind("SELECT COUNT(*) FROM file_space_restores WHERE id=?"), operation).Scan(&applied); err != nil {
		return err
	}
	if applied != 0 {
		return nil
	}
	if err = s.validateSpaceSnapshotRefs(ctx, tx, v); err != nil {
		return err
	}
	current, err := scanFileSpace(tx.QueryRowContext(ctx, s.rebind("SELECT "+fileSpaceColumns+" FROM file_spaces WHERE id=?"), v.Space.ID))
	create := errors.Is(err, ErrNotFound)
	if err != nil && !create {
		return err
	}
	space := *v.Space
	space.Rights = ""
	if create {
		space.Revision = 1
		space.Generation = v.Space.Generation + 1
	} else {
		space.Revision = current.Revision + 1
		space.Generation = current.Generation
		if v.Space.Generation > space.Generation {
			space.Generation = v.Space.Generation
		}
		space.Generation++
	}
	if create {
		_, err = tx.ExecContext(ctx, s.rebind("INSERT INTO file_spaces("+fileSpaceColumns+") VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)"), space.ID, space.TenantID, space.Name, space.Kind, space.Description, space.QuotaBytes, space.VersionDays, space.TrashDays, space.KeepVersions, space.Automatic, space.Revision, space.CreatedAt, space.Generation)
	} else {
		_, err = tx.ExecContext(ctx, s.rebind("UPDATE file_spaces SET name=?,kind=?,description=?,quota_bytes=?,version_days=?,trash_days=?,keep_versions=?,automatic=?,revision=?,generation=? WHERE id=?"), space.Name, space.Kind, space.Description, space.QuotaBytes, space.VersionDays, space.TrashDays, space.KeepVersions, space.Automatic, space.Revision, space.Generation, space.ID)
	}
	if err != nil {
		return mapErr(err)
	}
	for _, spec := range []struct{ table, column string }{{"file_space_groups", "space_id"}, {"file_acl", "owner_id"}, {"file_shares", "user_id"}} {
		if _, err = tx.ExecContext(ctx, s.rebind("DELETE FROM "+spec.table+" WHERE "+spec.column+"=?"), space.ID); err != nil {
			return err
		}
	}
	for _, g := range space.Groups {
		if _, err = tx.ExecContext(ctx, s.rebind("INSERT INTO file_space_groups(space_id,group_id,rights) VALUES(?,?,?)"), space.ID, g.GroupID, g.Rights); err != nil {
			return err
		}
	}
	for _, a := range v.Access {
		if _, err = tx.ExecContext(ctx, s.rebind("INSERT INTO file_acl(owner_id,path,grantee_id,rights) VALUES(?,?,?,?)"), space.ID, a.Path, a.GranteeID, a.Rights); err != nil {
			return err
		}
	}
	for _, a := range v.Shares {
		if _, err = tx.ExecContext(ctx, s.rebind("INSERT INTO file_shares(id,user_id,path,token,expires_at,max_downloads,download_count,created_at,password_hash,rights) VALUES(?,?,?,?,?,?,?,?,?,?)"), a.ID, space.ID, a.Path, a.Token, a.ExpiresAt, a.MaxDownloads, a.DownloadCount, a.CreatedAt, a.PasswordHash, a.Rights); err != nil {
			return mapErr(err)
		}
	}
	if _, err = tx.ExecContext(ctx, s.rebind("INSERT INTO file_space_restores(id,space_id,applied_at) VALUES(?,?,?)"), operation, space.ID, time.Now().UTC().UnixNano()); err != nil {
		return err
	}
	return tx.Commit()
}
func CloneFileSpaceSnapshot(v *FileSpaceSnapshot) (*FileSpaceSnapshot, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out FileSpaceSnapshot
	err = json.Unmarshal(data, &out)
	return &out, err
}

func (s *Store) ListFileSpaceIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM file_spaces ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

package storage

import (
	"context"
	"strings"
)

// Move explicit grants/links with their files, or revoke them on trash. A newly
// created file at the old path must never inherit a deleted file's public link.
func (s *Store) RetargetFileReferences(ctx context.Context, owner, from, to string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, spec := range []struct{ table, owner, key string }{{"file_acl", "owner_id", "path"}, {"file_shares", "user_id", "id"}} {
		rows, err := tx.QueryContext(ctx, s.rebind("SELECT "+spec.key+", path FROM "+spec.table+" WHERE "+spec.owner+"=?"), owner)
		if err != nil {
			return err
		}
		type entry struct{ key, path string }
		items := []entry{}
		for rows.Next() {
			var v entry
			if err = rows.Scan(&v.key, &v.path); err != nil {
				rows.Close()
				return err
			}
			if v.path == from || strings.HasPrefix(v.path, from+"/") {
				items = append(items, v)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, v := range items {
			if to == "" {
				_, err = tx.ExecContext(ctx, s.rebind("DELETE FROM "+spec.table+" WHERE "+spec.owner+"=? AND "+spec.key+"=?"), owner, v.key)
			} else {
				_, err = tx.ExecContext(ctx, s.rebind("UPDATE "+spec.table+" SET path=? WHERE "+spec.owner+"=? AND "+spec.key+"=?"), to+strings.TrimPrefix(v.path, from), owner, v.key)
			}
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) SetFileAccess(ctx context.Context, ownerID, path, granteeID, rights string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`INSERT INTO file_acl(owner_id,path,grantee_id,rights) VALUES(?,?,?,?) ON CONFLICT(owner_id,path,grantee_id) DO UPDATE SET rights=excluded.rights`), ownerID, path, granteeID, rights)
	return err
}
func (s *Store) DeleteFileAccess(ctx context.Context, ownerID, path, granteeID string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM file_acl WHERE owner_id=? AND path=? AND grantee_id=?`), ownerID, path, granteeID)
	return err
}
func (s *Store) ListFileAccess(ctx context.Context, ownerID, granteeID string) ([]FileAccess, error) {
	q := `SELECT owner_id,path,grantee_id,rights FROM file_acl WHERE 1=1`
	args := []any{}
	if ownerID != "" {
		q += ` AND owner_id=?`
		args = append(args, ownerID)
	}
	if granteeID != "" {
		q += ` AND grantee_id=?`
		args = append(args, granteeID)
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FileAccess{}
	for rows.Next() {
		var item FileAccess
		if err = rows.Scan(&item.OwnerID, &item.Path, &item.GranteeID, &item.Rights); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

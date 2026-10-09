package storage

import "context"

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

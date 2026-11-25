package storage

import (
	"context"
	"encoding/json"
	"time"
)

// Retain the device's deny record. Deleting its identity would allow Ensure to
// register the same device ID again as an allowed client. No remote wipe occurs.
func (s *Store) RetireFlowSyncDevice(ctx context.Context, user, id string) (removed int64, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, s.rebind(`UPDATE flowsync_devices SET blocked=?,retired_at=COALESCE(retired_at,?) WHERE id=? AND user_id=?`), true, time.Now().UTC(), id, user)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, ErrUnauthorized
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM flowsync_synckeys WHERE device_row_id=?`), id); err != nil {
		return 0, err
	}
	rows, err := tx.QueryContext(ctx, s.rebind("SELECT s.collection_id,"+syncStateColumns+syncStateFrom+" WHERE s.user_id=? AND s.client_id=?"), user, id)
	if err != nil {
		return 0, err
	}
	type state struct{ collection, raw string }
	discard := []state{}
	for rows.Next() {
		var collection string
		var saved storedSyncState
		args := append([]any{&collection}, saved.targets()...)
		if err = rows.Scan(args...); err != nil {
			rows.Close()
			return 0, err
		}
		raw := saved.state
		decoded, e := saved.decode(user, collection)

		if e != nil {
			rows.Close()
			return 0, e
		}
		var value map[string]json.RawMessage
		// Only recognized collection snapshots. Unknown/idempotency records remain.
		if json.Unmarshal([]byte(decoded), &value) == nil && value["Key"] != nil && value["Items"] != nil {
			discard = append(discard, state{collection, raw})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, v := range discard {
		result, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM flowsync_states WHERE user_id=? AND client_id=? AND collection_id=? AND state=?`), user, id, v.collection, v.raw)
		if err != nil {
			return 0, err
		}
		n, err = result.RowsAffected()
		if err != nil {
			return 0, err
		}
		removed += n
	}
	return removed, tx.Commit()
}

// Restore only the registration, not access. Provisioning must start fresh and
// an old remote-wipe instruction must never be re-issued on restoration.
func (s *Store) RestoreFlowSyncDevice(ctx context.Context, user, id string) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE flowsync_devices SET retired_at=NULL,blocked=?,provisioned=?,wipe_status='none',policy_key='0' WHERE user_id=? AND id=? AND retired_at IS NOT NULL`), true, false, user, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrUnauthorized
	}
	return nil
}

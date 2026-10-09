package storage

import (
	"context"
	"database/sql"
	"time"
)

// FlowSyncDevice is a mobile/desktop client registered with FlowSync.
type FlowSyncDevice struct {
	Address     string
	Agent       string
	Version     string
	Provisioned bool
	Blocked     bool
	WipeStatus  string
	ID          string
	UserID      string
	DeviceID    string
	DeviceType  string
	PolicyKey   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// FlowSyncSyncKey tracks per-collection sync state for a device.
type FlowSyncSyncKey struct {
	ID           string
	DeviceRowID  string
	CollectionID string
	SyncKey      string
	UpdatedAt    time.Time
}

func (s *Store) EnsureFlowSyncDevice(ctx context.Context, userID, deviceID, deviceType string) (*FlowSyncDevice, error) {
	q := s.rebind(`SELECT id, user_id, device_id, device_type, policy_key, created_at, updated_at, address, agent, version, provisioned, blocked, wipe_status
		FROM flowsync_devices WHERE user_id = ? AND device_id = ?`)
	d := &FlowSyncDevice{}
	err := s.db.QueryRowContext(ctx, q, userID, deviceID).Scan(
		&d.ID, &d.UserID, &d.DeviceID, &d.DeviceType, &d.PolicyKey, &d.CreatedAt, &d.UpdatedAt, &d.Address, &d.Agent, &d.Version, &d.Provisioned, &d.Blocked, &d.WipeStatus,
	)
	if err == nil {
		if deviceType != "" && d.DeviceType != deviceType {
			_ = s.UpdateFlowSyncDeviceType(ctx, d.ID, deviceType)
			d.DeviceType = deviceType
		}
		return d, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	now := time.Now().UTC()
	d = &FlowSyncDevice{
		ID:         NewID(),
		UserID:     userID,
		DeviceID:   deviceID,
		DeviceType: deviceType,
		PolicyKey:  "0",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	ins := s.rebind(`INSERT INTO flowsync_devices (id, user_id, device_id, device_type, policy_key, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (user_id, device_id) DO NOTHING`)
	result, err := s.db.ExecContext(ctx, ins, d.ID, d.UserID, d.DeviceID, d.DeviceType, d.PolicyKey, now, now)
	if err != nil {
		return nil, err
	}
	if n, err := result.RowsAffected(); err == nil && n == 0 {
		return s.EnsureFlowSyncDevice(ctx, userID, deviceID, deviceType)
	}
	return d, nil
}

func (s *Store) UpdateFlowSyncDeviceType(ctx context.Context, id, deviceType string) error {
	q := s.rebind(`UPDATE flowsync_devices SET device_type = ?, updated_at = ? WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, q, deviceType, time.Now().UTC(), id)
	return err
}

func (s *Store) SetFlowSyncPolicyKey(ctx context.Context, deviceRowID, policyKey string) error {
	q := s.rebind(`UPDATE flowsync_devices SET policy_key = ?, updated_at = ? WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, q, policyKey, time.Now().UTC(), deviceRowID)
	return err
}

func (s *Store) GetFlowSyncSyncKey(ctx context.Context, deviceRowID, collectionID string) (string, error) {
	q := s.rebind(`SELECT sync_key FROM flowsync_synckeys WHERE device_row_id = ? AND collection_id = ?`)
	var key string
	err := s.db.QueryRowContext(ctx, q, deviceRowID, collectionID).Scan(&key)
	if err == sql.ErrNoRows {
		return "0", nil
	}
	return key, err
}

func (s *Store) SetFlowSyncSyncKey(ctx context.Context, deviceRowID, collectionID, syncKey string) error {
	q := s.rebind(`INSERT INTO flowsync_synckeys (id, device_row_id, collection_id, sync_key, updated_at)
 VALUES (?, ?, ?, ?, ?) ON CONFLICT (device_row_id, collection_id) DO UPDATE SET sync_key = excluded.sync_key, updated_at = excluded.updated_at`)
	_, err := s.db.ExecContext(ctx, q, NewID(), deviceRowID, collectionID, syncKey, time.Now().UTC())
	return err
}

func (s *Store) ListFlowSyncDevices(ctx context.Context) ([]*FlowSyncDevice, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,device_id,device_type,policy_key,created_at,updated_at,address,agent,version,provisioned,blocked,wipe_status FROM flowsync_devices ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*FlowSyncDevice{}
	for rows.Next() {
		d := &FlowSyncDevice{}
		if err = rows.Scan(&d.ID, &d.UserID, &d.DeviceID, &d.DeviceType, &d.PolicyKey, &d.CreatedAt, &d.UpdatedAt, &d.Address, &d.Agent, &d.Version, &d.Provisioned, &d.Blocked, &d.WipeStatus); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) TouchFlowSyncDevice(ctx context.Context, id, address, agent, version string, provisioned bool) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE flowsync_devices SET address=?,agent=?,version=?,provisioned=CASE WHEN ? THEN ? ELSE provisioned END,updated_at=? WHERE id=?`), address, agent, version, provisioned, true, time.Now().UTC(), id)
	return err
}
func (s *Store) SetFlowSyncDeviceControl(ctx context.Context, id string, blocked bool, wipe string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`UPDATE flowsync_devices SET blocked=?,wipe_status=? WHERE id=?`), blocked, wipe, id)
	return err
}

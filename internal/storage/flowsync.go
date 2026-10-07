package storage

import (
	"context"
	"database/sql"
	"time"
)

// FlowSyncDevice is a mobile/desktop client registered with FlowSync.
type FlowSyncDevice struct {
	ID         string
	UserID     string
	DeviceID   string
	DeviceType string
	PolicyKey  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
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
	q := s.rebind(`SELECT id, user_id, device_id, device_type, policy_key, created_at, updated_at
		FROM flowsync_devices WHERE user_id = ? AND device_id = ?`)
	d := &FlowSyncDevice{}
	err := s.db.QueryRowContext(ctx, q, userID, deviceID).Scan(
		&d.ID, &d.UserID, &d.DeviceID, &d.DeviceType, &d.PolicyKey, &d.CreatedAt, &d.UpdatedAt,
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
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if _, err := s.db.ExecContext(ctx, ins, d.ID, d.UserID, d.DeviceID, d.DeviceType, d.PolicyKey, now, now); err != nil {
		return nil, err
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
	now := time.Now().UTC()
	q := s.rebind(`SELECT id FROM flowsync_synckeys WHERE device_row_id = ? AND collection_id = ?`)
	var id string
	err := s.db.QueryRowContext(ctx, q, deviceRowID, collectionID).Scan(&id)
	if err == sql.ErrNoRows {
		ins := s.rebind(`INSERT INTO flowsync_synckeys (id, device_row_id, collection_id, sync_key, updated_at)
			VALUES (?, ?, ?, ?, ?)`)
		_, err = s.db.ExecContext(ctx, ins, NewID(), deviceRowID, collectionID, syncKey, now)
		return err
	}
	if err != nil {
		return err
	}
	upd := s.rebind(`UPDATE flowsync_synckeys SET sync_key = ?, updated_at = ? WHERE id = ?`)
	_, err = s.db.ExecContext(ctx, upd, syncKey, now, id)
	return err
}

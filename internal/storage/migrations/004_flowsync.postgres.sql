-- +goose Up
CREATE TABLE IF NOT EXISTS flowsync_devices (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    device_type TEXT NOT NULL DEFAULT '',
    policy_key TEXT NOT NULL DEFAULT '0',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_flowsync_devices_user ON flowsync_devices(user_id);

CREATE TABLE IF NOT EXISTS flowsync_synckeys (
    id TEXT PRIMARY KEY,
    device_row_id TEXT NOT NULL REFERENCES flowsync_devices(id) ON DELETE CASCADE,
    collection_id TEXT NOT NULL,
    sync_key TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(device_row_id, collection_id)
);

CREATE INDEX IF NOT EXISTS idx_flowsync_synckeys_device ON flowsync_synckeys(device_row_id);

-- +goose Down
DROP TABLE IF EXISTS flowsync_synckeys;
DROP TABLE IF EXISTS flowsync_devices;

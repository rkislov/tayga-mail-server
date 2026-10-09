-- +goose Up
ALTER TABLE flowsync_devices ADD COLUMN address TEXT NOT NULL DEFAULT '';
ALTER TABLE flowsync_devices ADD COLUMN agent TEXT NOT NULL DEFAULT '';
ALTER TABLE flowsync_devices ADD COLUMN version TEXT NOT NULL DEFAULT '';
ALTER TABLE flowsync_devices ADD COLUMN provisioned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE flowsync_devices ADD COLUMN blocked INTEGER NOT NULL DEFAULT 0;
ALTER TABLE flowsync_devices ADD COLUMN wipe_status TEXT NOT NULL DEFAULT 'none';
-- +goose Down
ALTER TABLE flowsync_devices DROP COLUMN address;
ALTER TABLE flowsync_devices DROP COLUMN agent;
ALTER TABLE flowsync_devices DROP COLUMN version;
ALTER TABLE flowsync_devices DROP COLUMN provisioned;
ALTER TABLE flowsync_devices DROP COLUMN blocked;
ALTER TABLE flowsync_devices DROP COLUMN wipe_status;

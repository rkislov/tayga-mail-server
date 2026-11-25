-- +goose Up
ALTER TABLE flowsync_devices ADD COLUMN retired_at TIMESTAMP;
-- +goose Down
ALTER TABLE flowsync_devices DROP COLUMN retired_at;

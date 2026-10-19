-- +goose Up
CREATE TABLE calendar_deliveries (
 id TEXT PRIMARY KEY,
 sender TEXT NOT NULL,
 recipient TEXT NOT NULL,
 recipient_user_id TEXT NOT NULL DEFAULT '',
 subject TEXT NOT NULL,
 data BLOB NOT NULL,
 state TEXT NOT NULL DEFAULT 'pending',
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt BIGINT NOT NULL,
 lease_until BIGINT NOT NULL DEFAULT 0,
 lease_token TEXT NOT NULL DEFAULT '',
 last_error TEXT NOT NULL DEFAULT '',
 created_at BIGINT NOT NULL,
 completed_at BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX calendar_deliveries_due ON calendar_deliveries(state,next_attempt,lease_until);
-- +goose Down
DROP TABLE calendar_deliveries;

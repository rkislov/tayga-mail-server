-- +goose Up
CREATE TABLE flowsync_state_blobs (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 collection_id TEXT NOT NULL,
 data BLOB NOT NULL,
 raw_bytes BIGINT NOT NULL,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE flowsync_states ADD COLUMN snapshot_id TEXT REFERENCES flowsync_state_blobs(id);
ALTER TABLE flowsync_states ADD COLUMN response_id TEXT REFERENCES flowsync_state_blobs(id);
ALTER TABLE flowsync_states ADD COLUMN seen_at TIMESTAMP;
CREATE INDEX flowsync_states_snapshot ON flowsync_states(snapshot_id);
CREATE INDEX flowsync_states_response ON flowsync_states(response_id);
CREATE INDEX flowsync_states_activity ON flowsync_states(seen_at,updated_at);
-- +goose Down
DROP INDEX flowsync_states_activity;
DROP INDEX flowsync_states_snapshot;
DROP INDEX flowsync_states_response;
ALTER TABLE flowsync_states DROP COLUMN seen_at;
ALTER TABLE flowsync_states DROP COLUMN snapshot_id;
ALTER TABLE flowsync_states DROP COLUMN response_id;
DROP TABLE flowsync_state_blobs;

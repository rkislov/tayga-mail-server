-- +goose Up
CREATE TABLE flowsync_states (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 client_id TEXT NOT NULL,
 collection_id TEXT NOT NULL,
 state TEXT NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (user_id, client_id, collection_id)
);
-- +goose Down
DROP TABLE flowsync_states;

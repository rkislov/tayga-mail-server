-- +goose Up
CREATE TABLE agent_artifacts (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 scope TEXT NOT NULL,
 name TEXT NOT NULL,
 mime TEXT NOT NULL,
 expires_at BIGINT NOT NULL,
 data BLOB NOT NULL
);
CREATE INDEX agent_artifacts_expiry ON agent_artifacts(expires_at);
CREATE INDEX agent_artifacts_owner ON agent_artifacts(user_id);
CREATE TABLE agent_schedules (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 scope TEXT NOT NULL,
 conversation_id TEXT NOT NULL,
 spec TEXT NOT NULL,
 next_at BIGINT NOT NULL,
 last_at BIGINT NOT NULL DEFAULT 0,
 status TEXT NOT NULL DEFAULT '',
 claim TEXT NOT NULL DEFAULT '',
 lease_until BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX agent_schedules_due ON agent_schedules(next_at);
CREATE INDEX agent_schedules_owner ON agent_schedules(user_id);
-- +goose Down
DROP TABLE agent_schedules;
DROP TABLE agent_artifacts;

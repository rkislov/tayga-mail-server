-- +goose Up
CREATE TABLE spam_learning_jobs (
 message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
 class TEXT NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1,
 lease_until BIGINT NOT NULL DEFAULT 0,
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt BIGINT NOT NULL,
 state TEXT NOT NULL DEFAULT 'pending',
 last_error TEXT NOT NULL DEFAULT '',
 updated_at BIGINT NOT NULL
);
CREATE INDEX spam_learning_pending ON spam_learning_jobs(state,next_attempt);
CREATE TABLE protection_counters (
 domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
 kind TEXT NOT NULL,
 outcome TEXT NOT NULL,
 bucket BIGINT NOT NULL,
 count BIGINT NOT NULL DEFAULT 0,
 PRIMARY KEY(domain_id,kind,outcome,bucket)
);
-- +goose Down
DROP TABLE protection_counters;
DROP TABLE spam_learning_jobs;

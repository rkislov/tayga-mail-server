-- +goose Up
CREATE TABLE travel_preferences (
 user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 mode TEXT NOT NULL DEFAULT 'off'
);
CREATE TABLE travel_jobs (
 message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 state TEXT NOT NULL DEFAULT 'pending',
 lease_token TEXT NOT NULL DEFAULT '',
 lease_until BIGINT NOT NULL DEFAULT 0,
 next_attempt BIGINT NOT NULL DEFAULT 0,
 attempts INTEGER NOT NULL DEFAULT 0,
 warning TEXT NOT NULL DEFAULT ''
);
CREATE INDEX travel_jobs_pending ON travel_jobs(state,next_attempt,lease_until);
CREATE TABLE travel_candidates (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 fingerprint TEXT NOT NULL,
 identity_key TEXT NOT NULL,
 message_id TEXT NOT NULL,
 data TEXT NOT NULL,
 state TEXT NOT NULL DEFAULT 'review',
 created_at BIGINT NOT NULL,
 UNIQUE(user_id,fingerprint)
);
CREATE INDEX travel_candidates_user ON travel_candidates(user_id,created_at);
CREATE TABLE travel_sources (
 candidate_id TEXT NOT NULL REFERENCES travel_candidates(id) ON DELETE CASCADE,
 message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
 PRIMARY KEY(candidate_id,message_id)
);
CREATE TABLE travel_events (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 identity_key TEXT NOT NULL,
 event_id TEXT NOT NULL,
 PRIMARY KEY(user_id,identity_key)
);
-- +goose Down
DROP TABLE travel_events;
DROP TABLE travel_sources;
DROP TABLE travel_candidates;
DROP TABLE travel_jobs;
DROP TABLE travel_preferences;

-- +goose Up
CREATE TABLE xmpp_push_registrations (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 owner TEXT NOT NULL, service TEXT NOT NULL, node TEXT NOT NULL,
 options_xml TEXT NOT NULL, resource TEXT NOT NULL,
 UNIQUE (user_id, service, node)
);
CREATE TABLE xmpp_push_jobs (
 registration_id TEXT PRIMARY KEY REFERENCES xmpp_push_registrations(id) ON DELETE CASCADE,
 generation BIGINT NOT NULL DEFAULT 1, published_generation BIGINT NOT NULL DEFAULT 0,
 request_id TEXT NOT NULL DEFAULT '', attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX xmpp_push_jobs_due ON xmpp_push_jobs(next_attempt);
CREATE INDEX xmpp_push_owner ON xmpp_push_registrations(owner);
-- +goose Down
DROP TABLE xmpp_push_jobs;
DROP TABLE xmpp_push_registrations;

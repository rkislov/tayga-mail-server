-- +goose Up
CREATE TABLE webpush_keys (id INTEGER PRIMARY KEY, public_key TEXT NOT NULL, private_key TEXT NOT NULL);
CREATE TABLE webpush_subscriptions (endpoint TEXT PRIMARY KEY, user_id TEXT NOT NULL, subscription TEXT NOT NULL);
CREATE INDEX webpush_owner ON webpush_subscriptions(user_id);
CREATE TABLE webpush_jobs (id TEXT PRIMARY KEY, endpoint TEXT NOT NULL, payload TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, next_attempt BIGINT NOT NULL, FOREIGN KEY(endpoint) REFERENCES webpush_subscriptions(endpoint) ON DELETE CASCADE);
CREATE INDEX webpush_due ON webpush_jobs(next_attempt);
-- +goose Down
DROP TABLE webpush_jobs;
DROP TABLE webpush_subscriptions;
DROP TABLE webpush_keys;

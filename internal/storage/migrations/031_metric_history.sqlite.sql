-- +goose Up
CREATE TABLE metric_history (
 scope TEXT NOT NULL,
 recorded_at BIGINT NOT NULL,
 payload TEXT NOT NULL,
 PRIMARY KEY (scope, recorded_at)
);
CREATE INDEX metric_history_age ON metric_history(recorded_at);
-- +goose Down
DROP TABLE metric_history;

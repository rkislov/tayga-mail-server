-- +goose Up
CREATE TABLE inbox_rule_sets (
 user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 revision INTEGER NOT NULL,
 rules_json TEXT NOT NULL,
 script TEXT NOT NULL
);
-- +goose Down
DROP TABLE inbox_rule_sets;

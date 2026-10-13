-- +goose Up
ALTER TABLE tasks ADD COLUMN ews_properties TEXT NOT NULL DEFAULT '{}';
-- +goose Down
ALTER TABLE tasks DROP COLUMN ews_properties;

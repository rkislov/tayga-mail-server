-- +goose Up
ALTER TABLE file_shares ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE file_shares ADD COLUMN rights TEXT NOT NULL DEFAULT 'read';

-- +goose Down
ALTER TABLE file_shares DROP COLUMN rights;
ALTER TABLE file_shares DROP COLUMN password_hash;

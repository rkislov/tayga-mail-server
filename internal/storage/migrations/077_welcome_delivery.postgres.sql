-- +goose Up
ALTER TABLE calendar_deliveries ADD COLUMN kind TEXT NOT NULL DEFAULT 'calendar' CHECK (kind IN ('calendar','welcome'));
ALTER TABLE calendar_deliveries ADD COLUMN welcome_user_id TEXT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE calendar_deliveries ADD COLUMN origin_json TEXT NOT NULL DEFAULT '{}';
CREATE UNIQUE INDEX welcome_delivery_user ON calendar_deliveries(welcome_user_id) WHERE welcome_user_id IS NOT NULL;
-- +goose Down
DROP INDEX welcome_delivery_user;
ALTER TABLE calendar_deliveries DROP COLUMN origin_json;
ALTER TABLE calendar_deliveries DROP COLUMN welcome_user_id;
ALTER TABLE calendar_deliveries DROP COLUMN kind;

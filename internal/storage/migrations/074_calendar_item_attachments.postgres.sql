-- +goose Up
ALTER TABLE calendar_attachments ADD COLUMN ews_item_type TEXT NOT NULL DEFAULT '' CHECK (ews_item_type IN ('', 'Message'));
-- +goose Down
ALTER TABLE calendar_attachments DROP COLUMN ews_item_type;

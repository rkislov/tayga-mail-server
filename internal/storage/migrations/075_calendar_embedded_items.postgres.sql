-- +goose Up
ALTER TABLE calendar_attachments ADD COLUMN ews_item_xml TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE calendar_attachments DROP COLUMN ews_item_xml;

-- +goose Up
CREATE TABLE IF NOT EXISTS service_classes (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);
ALTER TABLE users ADD COLUMN IF NOT EXISTS service_class_id TEXT NOT NULL DEFAULT '';

-- +goose Down
DROP TABLE IF EXISTS service_classes;

-- +goose Up
CREATE TABLE IF NOT EXISTS service_classes (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);
ALTER TABLE users ADD COLUMN service_class_id TEXT NOT NULL DEFAULT '';

-- +goose Down
DROP TABLE IF EXISTS service_classes;

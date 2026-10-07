-- +goose Up
CREATE TABLE IF NOT EXISTS mail_log (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    domain TEXT NOT NULL DEFAULT '',
    event TEXT NOT NULL DEFAULT '',
    direction TEXT NOT NULL DEFAULT '',
    peer TEXT NOT NULL DEFAULT '',
    mail_from TEXT NOT NULL DEFAULT '',
    rcpt_to TEXT NOT NULL DEFAULT '',
    message_id TEXT NOT NULL DEFAULT '',
    size BIGINT NOT NULL DEFAULT 0,
    detail TEXT NOT NULL DEFAULT '',
    line TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mail_log_created ON mail_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mail_log_domain ON mail_log(domain);
CREATE INDEX IF NOT EXISTS idx_mail_log_tenant ON mail_log(tenant_id);

-- +goose Down
DROP TABLE IF EXISTS mail_log;

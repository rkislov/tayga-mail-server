-- +goose Up
CREATE TABLE IF NOT EXISTS dmarc_agg (
    id TEXT PRIMARY KEY,
    domain TEXT NOT NULL,
    day TEXT NOT NULL,
    source_ip TEXT NOT NULL DEFAULT '',
    envelope_domain TEXT NOT NULL DEFAULT '',
    header_from TEXT NOT NULL DEFAULT '',
    spf_result TEXT NOT NULL DEFAULT 'none',
    dkim_result TEXT NOT NULL DEFAULT 'none',
    disposition TEXT NOT NULL DEFAULT 'none',
    policy TEXT NOT NULL DEFAULT 'none',
    rua TEXT NOT NULL DEFAULT '',
    count INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(domain, day, source_ip, envelope_domain, header_from, spf_result, dkim_result, disposition)
);

CREATE INDEX IF NOT EXISTS idx_dmarc_agg_day ON dmarc_agg(day);
CREATE INDEX IF NOT EXISTS idx_dmarc_agg_domain_day ON dmarc_agg(domain, day);

-- +goose Down
DROP TABLE IF EXISTS dmarc_agg;

# Tayga Mail Server (TMS)

Corporate-class multi-domain / multi-tenant mail server in Go.

**Author:** Кислов Роман Сергеевич (Roman Sergeyevich Kislov)  
**License:** [Apache License 2.0](LICENSE)

See [docs/architecture.md](docs/architecture.md) and `configs/tayga.example.yaml`.

## Quick start

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tayga-mail ./cmd/tayga
./tayga-mail version
# Standard ports need root (or CAP_NET_BIND_SERVICE); TLS auto-generates self-signed certs when enabled
sudo ./tayga-mail -config configs/tayga.example.yaml
```

### Cross-platform release builds

```bash
VERSION=v0.3.0 ./scripts/crossbuild.sh   # → dist/*.tar.gz|zip + checksums.txt
```

GitHub Releases: push a `v*` tag; [GoReleaser](https://goreleaser.com) (`.github/workflows/release.yml`) publishes linux/darwin/windows/freebsd amd64+arm64 archives.

| Service | Address |
|---------|---------|
| HTTPS / UI / DAV / FlowSync | `https://127.0.0.1/` (HTTP `:80` redirects) |
| SMTP MX / submission / SMTPS | `:25` / `:587` / `:465` |
| IMAP / IMAPS | `:143` / `:993` |
| POP3 / POP3S | `:110` / `:995` |
| ManageSieve | `:4190` |

Seed: `admin@example.com` / `changeme`. TLS: `tls.auto_generate` or `scripts/gen-dev-certs.sh`.

- LDAP: optional per-domain hybrid auth in `ldap.domains` (see example config)
- Quotas: set `users.quota_bytes` (0 = unlimited); SMTP returns `552` when full
- Auth API: `POST /api/v1/auth/login` → opaque tokens; TOTP MFA; OIDC per-domain
- IMAP/SMTP: SASL `XOAUTH2` / `OAUTHBEARER` with access tokens from the auth API
- CalDAV / CardDAV: `/dav/cal/`, `/dav/card/`; `/.well-known/caldav|carddav`
- FlowSync (original): ActiveSync `/Microsoft-Server-ActiveSync`, EWS `/EWS/Exchange.asmx`, Autodiscover (POX/JSON) + Mozilla autoconfig
- Admin: tenant domains, users, TLS certs, monitoring, backup (`/api/v1/admin/*`)
- Metrics: `GET /metrics` (Prometheus `tayga_*`)
- Backup/restore: `./tayga-mail backup|restore -config …` or Admin UI; see [docs/ha.md](docs/ha.md)
- HA: Postgres + shared maildir; cross-node IMAP IDLE; `ha.mode: active_standby` (`fence: mx|writers`) or `sticky` per-user writers
- Outbound: `smtp.relay` / `smtp.outbound_direct` + optional `smtp.dkim` for authenticated external recipients
- SMTP rate limits: `smtp.rate_limit.per_ip` / `per_user`
- Outbound queue + DSN: `smtp.queue` retries failed remote delivery; bounces local senders; admin `/api/v1/admin/outbound`
- Object store: optional S3/MinIO write-through for maildir (`mailstore.object_store`); `tayga-mail sync-objects` to reconcile
- Virus scan: `scan.enabled` with ClamAV/exec → reject, quarantine folder, or tag headers
- Spam: `spam.enabled` Rspamd `/checkv2` → reject / greylist / tag / Junk quarantine; admin `/api/v1/admin/quarantine`
- Inbound DKIM: `dkim_verify.enabled` → `Authentication-Results` (tag or reject on MX)
- WebAuthn MFA: `/api/v1/auth/webauthn/*` (passkeys; UUID credential/session IDs)
- Account API: `GET /api/v1/me`; admin: `/api/v1/admin/users` (`http.admins`)
- IDs: all entity PKs/FKs are UUIDs (fresh DB required after integer-ID builds)

## License

Copyright © Кислов Роман Сергеевич. Licensed under the Apache License, Version 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).

UI abstract backdrop art © Кислов Роман Сергеевич.

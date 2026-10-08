# Tayga Mail Server (TMS)

Corporate-class multi-domain / multi-tenant mail server in Go.

**Author:** Кислов Роман Сергеевич (Roman Sergeyevich Kislov)  
**License:** [Apache License 2.0](LICENSE)

See [docs/architecture.md](docs/architecture.md) and `configs/tayga.example.yaml` (minimal bootstrap YAML). Mail policies and most options are edited in **Admin → Server settings** (`/api/v1/admin/settings`).

**Первоначальная настройка (RU):** [docs/setup.md](docs/setup.md) — bootstrap, seed-админ, DNS, TLS, исходящая почта, чеклист.

## Quick start

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tayga-mail ./cmd/tayga
./tayga-mail version
# Standard ports need root (or CAP_NET_BIND_SERVICE); TLS auto-generates self-signed certs when enabled
sudo ./tayga-mail -config configs/tayga.example.yaml
```

Полный сценарий ввода в эксплуатацию: [docs/setup.md](docs/setup.md).  
Консоль: `./tayga-mail menu -config …` — [docs/cli-menu.md](docs/cli-menu.md).

### Cross-platform release builds

```bash
VERSION=0.9.2 ./scripts/crossbuild.sh   # → dist/*.tar.gz|zip + checksums.txt
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

- LDAP: hybrid auth; group→admin roles; nested groups; `tayga-mail ldap-sync`
- Quotas: set `users.quota_bytes` (0 = unlimited); SMTP returns `552` when full
- Auth API: `POST /api/v1/auth/login` → opaque tokens; TOTP MFA; OIDC per-domain
- IMAP/SMTP: SASL `XOAUTH2` / `OAUTHBEARER` with access tokens from the auth API
- CalDAV / CardDAV: `/dav/cal/`, `/dav/card/`; `/.well-known/caldav|carddav`
- Notes: `/api/v1/notes*` (folders, rich notes, attachments, drawing); folder/note ACL for multi-user edit; FlowSync Notes (type 10) + EWS sticky notes
- FlowSync (original): ActiveSync `/Microsoft-Server-ActiveSync`, EWS `/EWS/Exchange.asmx`, Autodiscover (POX/JSON) + Mozilla autoconfig
- Admin: tenant domains, users, УЦ, **XMPP** (C2S / bots), **Server settings** (DB-backed config), monitoring, backup (`/api/v1/admin/*`)
- Metrics: `GET /metrics` (Prometheus `tayga_*`)
- Backup/restore: `./tayga-mail backup|restore -config …` or Admin UI; see [docs/ha.md](docs/ha.md)
- HA: Postgres + shared maildir; cross-node IMAP IDLE; `ha.mode: active_standby` (`fence: mx|writers`) or `sticky` per-user writers
- Outbound: `smtp.relay` / `smtp.outbound_direct` + optional `smtp.dkim` for authenticated external recipients
- MTA-STS / TLS-RPT / DANE: `smtp.mta_sts` (+ `publish`), `smtp.tls_rpt`, `smtp.dane` TLSA on direct MX
- SMTP rate limits: `smtp.rate_limit.per_ip` / `per_user`
- Outbound queue + DSN: `smtp.queue` retries failed remote delivery; bounces local senders; admin `/api/v1/admin/outbound`
- Object store: optional S3/MinIO write-through for maildir (`mailstore.object_store`); `tayga-mail sync-objects` to reconcile
- Virus scan: `scan.enabled` with ClamAV / exec / ICAP → reject, quarantine folder, or tag headers
- Spam: `spam.enabled` Rspamd `/checkv2` → reject / greylist / tag / Junk quarantine; admin `/api/v1/admin/quarantine`
- Inbound auth: optional DB-backed `greylist`; then `helo` → `iprev` → `spf` → `dkim_verify` → ARC → `dmarc` on MX
- DMARC rua/ruf: aggregate XML (`report.enabled`) and AFRF failure reports (`report.failure`)
- DMARC ARC trust: `dmarc.arc_trust` softens fail when ARC `cv=pass`
- ARC: `arc.enabled` verify/seal (RFC 8617) on inbound MX
- Quarantine admin: filters, spam/virus meta, preview (`/api/v1/admin/quarantine`)
- WebAuthn MFA: `/api/v1/auth/webauthn/*` (passkeys; UUID credential/session IDs)
- Account API: `GET /api/v1/me`; admin: `/api/v1/admin/users` (`http.admins`)
- Web apps: `/api/v1/mail`, `/api/v1/calendar`, `/api/v1/contacts`, `/api/v1/files`, `/api/v1/chat` + embedded UI (ru/en)
- Archive mailbox (gzip on disk) + FTS mail search — [docs/mail-archive-search.md](docs/mail-archive-search.md) (RU), [architecture.md](docs/architecture.md#archive-mailbox) (EN)
- Initial server setup (RU): [docs/setup.md](docs/setup.md)
- User migration IMAP/CalDAV/CardDAV: [docs/migration.md](docs/migration.md)
- In-app / browser notifications (new mail, upcoming calendar) via SSE `/api/v1/notifications/stream`
- Certificate authority (УЦ): self-signed / commercial PEM / Let’s Encrypt ACME — [docs/ca.md](docs/ca.md)
- XMPP (optional C2S) + web Chat; bots via XEP-0114 / HTTP — see [docs/xmpp.md](docs/xmpp.md)
- Web UI: floating rounded panels; independent day/night mode under **Appearance**
- Server configuration: 27 grouped sections, typed modal forms and Russian help for every parameter; bootstrap storage remains read-only
- Calendar: click a date to open its day; double-click to create an event on that date; invitations are in a separate inbox with a pending badge
- UI screenshots: [docs/screenshots/](docs/screenshots/)
- IDs: all entity PKs/FKs are UUIDs (fresh DB required after integer-ID builds)

## License

Copyright © Кислов Роман Сергеевич. Licensed under the Apache License, Version 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).

UI backdrop art (tayga / cosmos / city / kalyazin / temple) © Алиса Кислова; `moscow` (Москва-Сити) — generated backdrop.

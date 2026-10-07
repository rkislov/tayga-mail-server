# Tayga Mail Server (TMS)

Corporate-class multi-domain / multi-tenant mail server in Go.

**Author:** Кислов Роман Сергеевич (Roman Sergeyevich Kislov)  
**License:** [Apache License 2.0](LICENSE)

See [docs/architecture.md](docs/architecture.md) and `configs/tayga.example.yaml`.

## Quick start

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tayga-mail ./cmd/tayga
./tayga-mail -config configs/tayga.example.yaml
```

- Health: http://127.0.0.1:8080/healthz
- Metrics: http://127.0.0.1:8080/metrics
- SMTP submission: `localhost:1587` (seed user `admin@example.com` / `changeme`)
- SMTP MX: `localhost:1025`
- IMAP: `localhost:1143`
- POP3: `localhost:1110`
- ManageSieve: `localhost:14190`
- LDAP: optional per-domain hybrid auth in `ldap.domains` (see example config)
- Quotas: set `users.quota_bytes` (0 = unlimited); SMTP returns `552` when full
- Auth API: `POST /api/v1/auth/login` → opaque tokens; TOTP MFA; OIDC per-domain
- IMAP/SMTP: SASL `XOAUTH2` / `OAUTHBEARER` with access tokens from the auth API
- CalDAV: `http://127.0.0.1:8080/dav/cal/` (Basic/Bearer); `/.well-known/caldav`
- CardDAV: `http://127.0.0.1:8080/dav/card/` (Basic/Bearer); `/.well-known/carddav`
- FlowSync (original): ActiveSync `/Microsoft-Server-ActiveSync` (mail + calendar + contacts + folders/move), EWS `/EWS/Exchange.asmx`, Autodiscover
- WebAuthn MFA: `/api/v1/auth/webauthn/*` (passkeys; UUID credential/session IDs)
- Admin UI: `http://127.0.0.1:8080/` (login, TOTP, passkeys, quota)
- Account API: `GET /api/v1/me`; admin: `/api/v1/admin/users` (`http.admins`)
- IDs: all entity PKs/FKs are UUIDs (fresh DB required after integer-ID builds)

## License

Copyright © Кислов Роман Сергеевич. Licensed under the Apache License, Version 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).

UI abstract backdrop art © Кислов Роман Сергеевич.

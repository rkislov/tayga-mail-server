# Tayga Mail Server — Architecture (Phase 1–3)

## Overview

TMS is a single-binary mail server (`tayga-mail`) built in Go with `CGO_ENABLED=0`.

```
cmd/tayga
  ├─ config (YAML)
  ├─ storage (sqlite | postgres)
  ├─ auth (local Argon2id; LDAP/OIDC stubs)
  ├─ mailstore (Maildir++)
  ├─ smtp (MX + submission)
  ├─ imap (go-imap backend)
  ├─ pop3 (RFC 1939 INBOX)
  └─ httpapi (/healthz, /readyz, /metrics, embedded UI placeholder)
```

## Storage

- Shared schema with `tenant_id` isolation.
- Message bodies on disk (Maildir++); metadata in `messages` (UID, flags, path).

## Protocols (dev ports)

| Protocol   | Config              | Default |
|------------|---------------------|---------|
| SMTP MX    | `smtp.mx`           | `:1025` |
| Submission | `smtp.submission`   | `:1587` |
| IMAP       | `imap.listen`       | `:1143` |
| POP3       | `pop3.listen`       | `:1110` |
| HTTP       | `http.listen`       | `:8080` |

## IMAP MVP

LOGIN, LIST, SELECT, FETCH, STORE, APPEND, SEARCH (basic), COPY, EXPUNGE,
CREATE/DELETE/RENAME. Auth via local Argon2id. Bodies read from Maildir.

## POP3 MVP

USER/PASS, STAT, LIST, RETR, DELE (+QUIT), RSET, UIDL, TOP, CAPA on INBOX.

## Build

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tayga-mail ./cmd/tayga
./tayga-mail -config configs/tayga.example.yaml
```

Seed user: `admin@example.com` / `changeme`

## Next milestones

Sieve/ManageSieve → multi-tenant LDAP → OIDC/MFA → CalDAV/CardDAV → …

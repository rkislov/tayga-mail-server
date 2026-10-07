# Tayga Mail Server — Architecture (Phase 1–4)

## Overview

TMS is a single-binary mail server (`tayga-mail`) built in Go with `CGO_ENABLED=0`.

```
cmd/tayga
  ├─ config (YAML)
  ├─ storage (sqlite | postgres)
  ├─ auth (local Argon2id; LDAP hybrid per-domain; OIDC stub)
  ├─ mailstore (Maildir++)
  ├─ smtp / imap / pop3 / managesieve
  ├─ sieve (foxcpp/go-sieve on delivery)
  └─ httpapi (/healthz, /readyz, /metrics, embedded UI placeholder)
```

## Storage

- Shared schema with `tenant_id` isolation.
- Message bodies on disk (Maildir++); metadata in `messages` (UID, flags, path).
- User mailbox quota: `users.quota_bytes` (0 = unlimited), enforced on SMTP/IMAP/Sieve writes.

## Auth

- **Local**: Argon2id password hashes.
- **LDAP hybrid** (`ldap.domains.<domain>`): service bind → search → user bind; JIT creates `auth_source=ldap` users. Existing local users never fall through to LDAP.
- Response cache: `ldap.cache_ttl` (negative cache is shorter).
- OIDC: stub for a later milestone.

## Protocols (dev ports)

| Protocol    | Config                 | Default   |
|-------------|------------------------|-----------|
| SMTP MX     | `smtp.mx`              | `:1025`   |
| Submission  | `smtp.submission`      | `:1587`   |
| IMAP        | `imap.listen`          | `:1143`   |
| POP3        | `pop3.listen`          | `:1110`   |
| ManageSieve | `managesieve.listen`   | `:14190`  |
| HTTP        | `http.listen`          | `:8080`   |

## Sieve MVP

Active script on SMTP local delivery; ManageSieve on `:14190`.

## Build

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tayga-mail ./cmd/tayga
./tayga-mail -config configs/tayga.example.yaml
```

Seed user: `admin@example.com` / `changeme`

## Next milestones

OIDC per-domain + MFA (TOTP/WebAuthn) → CalDAV/CardDAV → ActiveSync/EWS → …

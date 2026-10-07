# Tayga Mail Server — Architecture (Phase 1–5)

## Overview

TMS is a single-binary mail server (`tayga-mail`) built in Go with `CGO_ENABLED=0`.

```
cmd/tayga
  ├─ config (YAML)
  ├─ storage (sqlite | postgres)
  ├─ auth (local Argon2id; LDAP hybrid; OIDC; TOTP MFA; opaque tokens)
  ├─ mailstore (Maildir++)
  ├─ smtp / imap / pop3 / managesieve
  ├─ sieve (foxcpp/go-sieve on delivery)
  └─ httpapi (/healthz, /readyz, /metrics, /api/v1/auth/*, embedded UI)
```

## Storage

- Shared schema with `tenant_id` isolation.
- Message bodies on disk (Maildir++); metadata in `messages` (UID, flags, path).
- User mailbox quota: `users.quota_bytes` (0 = unlimited), enforced on SMTP/IMAP/Sieve writes.
- MFA: `user_mfa`, `mfa_challenges`, `oauth_tokens` (migration `002_mfa_oauth`).

## Auth

- **Local**: Argon2id password hashes.
- **LDAP hybrid** (`ldap.domains.<domain>`): service bind → search → user bind; JIT creates `auth_source=ldap` users.
- **OIDC** (`oidc.domains.<domain>`): authorization-code flow; JIT creates `auth_source=oidc` users.
- **MFA (TOTP)**: enroll via HTTP API; backup codes (hashed). When enabled and `mfa.require_token_for_mfa_users`, password auth on protocols is rejected.
- **Tokens**: opaque access/refresh pairs; use with SASL `XOAUTH2` / `OAUTHBEARER` on IMAP and SMTP submission.

### HTTP auth API

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/auth/login` | password → tokens or MFA challenge |
| POST | `/api/v1/auth/mfa/verify` | challenge + TOTP → tokens |
| POST | `/api/v1/auth/token` | refresh_token grant |
| POST | `/api/v1/auth/mfa/setup` | begin TOTP (Bearer) |
| POST | `/api/v1/auth/mfa/confirm` | enable TOTP (Bearer) |
| POST | `/api/v1/auth/mfa/disable` | disable TOTP (Bearer) |
| GET | `/api/v1/auth/oidc/{domain}/start` | redirect to IdP |
| GET | `/api/v1/auth/oidc/callback` | code exchange → tokens |

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

WebAuthn MFA → CalDAV/CardDAV → ActiveSync/EWS → …

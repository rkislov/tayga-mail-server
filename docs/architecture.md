# Tayga Mail Server — Architecture (Phase 1–2)

## Overview

TMS is a single-binary mail server (`tayga-mail`) built in Go with `CGO_ENABLED=0`.
This milestone delivers the core process skeleton: configuration, dual-backend storage,
local authentication, Maildir++ delivery, and a minimal SMTP stack.

```
cmd/tayga
  ├─ config (YAML)
  ├─ storage (sqlite | postgres)
  ├─ auth (local Argon2id; LDAP/OIDC stubs)
  ├─ mailstore (Maildir++)
  ├─ smtp (MX + submission via emersion/go-smtp)
  └─ httpapi (/healthz, /readyz, /metrics, embedded UI placeholder)
```

## Storage

- Shared schema with `tenant_id` isolation (not separate DB schemas yet).
- Message **bodies** live on disk under `mailstore.root` (Maildir++).
- Message **metadata** (UID, flags, path) lives in the `messages` table.
- Driver selected by `storage.driver`; protocol code only depends on `storage.Driver`.

## Auth

- Local users: Argon2id PHC-encoded hashes.
- `auth.LDAPProvider` / `auth.OIDCProvider` interfaces exist; stubs return unsupported.
- SMTP AUTH: PLAIN and LOGIN (insecure auth allowed when TLS is not configured).

## SMTP MVP

| Listener    | Config key         | Auth required |
|-------------|--------------------|---------------|
| MX          | `smtp.mx`          | no            |
| Submission  | `smtp.submission`  | yes           |
| SMTPS       | `smtp.smtps`       | yes (+ TLS)   |

Accepted mail is delivered to the local recipient INBOX. Outbound relay, queues,
SPF/DKIM/DMARC, and greylisting are out of scope for this milestone.

## Build

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tayga-mail ./cmd/tayga
./tayga-mail -config configs/tayga.example.yaml
```

## Next milestones

IMAP/POP3/Sieve → multi-tenant LDAP → OIDC/MFA → CalDAV/CardDAV → ActiveSync/EWS →
WebDAV → ICAP/ThreatFox → backup/migration → full Web UI (street art + nature).

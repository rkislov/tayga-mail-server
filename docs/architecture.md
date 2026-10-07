# Tayga Mail Server — Architecture (Phase 1–7)

## Overview

TMS is a single-binary mail server (`tayga-mail`) built in Go with `CGO_ENABLED=0`.

```
cmd/tayga
  ├─ config (YAML)
  ├─ storage (sqlite | postgres; UUID PKs everywhere)
  ├─ auth (local Argon2id; LDAP hybrid; OIDC; TOTP MFA; opaque tokens)
  ├─ mailstore (Maildir++)
  ├─ smtp / imap / pop3 / managesieve
  ├─ sieve (foxcpp/go-sieve on delivery)
  ├─ dav (CalDAV + CardDAV via emersion/go-webdav)
  ├─ flowsync (proprietary Tayga sync engine — ActiveSync/EWS wire)
  └─ httpapi (/healthz, /readyz, /metrics, /api/v1/auth/*, /dav/*, FlowSync, UI)
```

## Storage

- **All entity primary keys and FKs are UUID strings (`TEXT`)**, including FlowSync device rows, policy keys, and sync-state tokens. IMAP `uid` / `uidnext` / `uidvalidity` stay numeric protocol counters.
- Shared schema with `tenant_id` isolation.
- Migrations: `001_init`, `002_mfa_oauth`, `003_dav`, `004_flowsync`.

## Auth

- Local Argon2id; LDAP hybrid; OIDC; TOTP MFA; opaque Bearer tokens for IMAP/SMTP/DAV/FlowSync.

## CalDAV / CardDAV

| URL | Purpose |
|-----|---------|
| `/.well-known/caldav` | → `/dav/cal/` |
| `/.well-known/carddav` | → `/dav/card/` |
| `/dav/cal/{email}/…` | calendars (UUID-backed objects) |
| `/dav/card/{email}/…` | address books |

## FlowSync (proprietary)

Tayga's in-house sync engine. Wire-compatible with ActiveSync-class mobile clients and EWS-class desktop clients. **Not** a fork of Z-Push/SOGo/OpenChange — original Tayga code. Responses carry `X-FlowSync: Tayga-Proprietary`.

| Endpoint | Role |
|----------|------|
| `/Autodiscover/Autodiscover.xml` | POX Autodiscover → FlowSync URLs |
| `/autodiscover/autodiscover.json/…` | JSON Autodiscover |
| `/Microsoft-Server-ActiveSync` | FolderSync, Sync, Ping, Provision, GetItemEstimate |
| `/EWS/Exchange.asmx` | FindItem, GetItem, SyncFolderItems, FindFolder |

Collection/item IDs exposed to clients are **UUIDs** (mailbox / message / device / policy).

Enable: `flowsync.enabled: true` (default).

## Protocols (dev ports)

| Protocol | Default |
|----------|---------|
| SMTP MX / submission | `:1025` / `:1587` |
| IMAP / POP3 / ManageSieve | `:1143` / `:1110` / `:14190` |
| HTTP / DAV / FlowSync | `:8080` |

## Build

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tayga-mail ./cmd/tayga
./tayga-mail -config configs/tayga.example.yaml
```

Seed: `admin@example.com` / `changeme`. UUID schema requires a fresh DB when upgrading from integer IDs.

## Next milestones

WebAuthn MFA → richer FlowSync (WBXML, calendar/contacts sync, policies) → …

# Tayga Mail Server — Architecture (Phase 1–7)

## Overview

TMS is a single-binary mail server (`tayga-mail`) built in Go with `CGO_ENABLED=0`.

```
cmd/tayga
  ├─ config (YAML)
  ├─ storage (sqlite | postgres; UUID PKs everywhere)
  ├─ auth (local Argon2id; LDAP hybrid; OIDC; TOTP + WebAuthn MFA; opaque tokens)
  ├─ mailstore (Maildir++)
  ├─ smtp / imap / pop3 / managesieve
  ├─ sieve (foxcpp/go-sieve on delivery)
  ├─ dav (CalDAV + CardDAV via emersion/go-webdav)
  ├─ flowsync (proprietary Tayga sync engine — ActiveSync/EWS wire)
  └─ httpapi (/healthz, /readyz, /metrics, /api/v1/auth/*, /dav/*, FlowSync, embedded admin UI)
```

## Storage

- **All entity primary keys and FKs are UUID strings (`TEXT`)**, including FlowSync device rows, policy keys, and sync-state tokens. IMAP `uid` / `uidnext` / `uidvalidity` stay numeric protocol counters.
- Shared schema with `tenant_id` isolation.
- Migrations: `001_init`, `002_mfa_oauth`, `003_dav`, `004_flowsync`, `005_webauthn`.

## Auth

- Local Argon2id; LDAP hybrid; OIDC; TOTP + WebAuthn (passkeys) MFA; opaque Bearer tokens for IMAP/SMTP/DAV/FlowSync.
- WebAuthn credential rows and ceremony sessions use **UUID** primary keys; user handle is the user’s UUID (16 bytes).

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
| `/Microsoft-Server-ActiveSync` | FolderSync, Sync (Email/Calendar/Contacts read+write), Ping, Provision, GetItemEstimate (XML + WBXML) |
| `/EWS/Exchange.asmx` | Find/Get/Create/Update/Delete + SyncFolderItems for mail, calendar, contacts; FindFolder |

Collection/item IDs exposed to clients are **UUIDs** (mailbox / message / calendar / address book / event / contact / device / policy).

FolderSync advertises mailboxes plus CalDAV calendars (type 8) and CardDAV address books (type 9). Sync write-back: calendar/contact Add·Change·Delete; mail Change (`Read`) and Delete. EWS CreateItem/UpdateItem/DeleteItem for the same. Provision returns a richer device policy with a UUID `PolicyKey`.

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

### WebAuthn API

| Endpoint | Role |
|----------|------|
| `POST /api/v1/auth/webauthn/register/begin` | creation options + UUID `session_id` (Bearer) |
| `POST /api/v1/auth/webauthn/register/finish` | store credential (UUID PK) |
| `POST /api/v1/auth/webauthn/login/begin` | assertion options after MFA challenge |
| `POST /api/v1/auth/webauthn/login/finish` | issue tokens |
| `GET/DELETE /api/v1/auth/webauthn/credentials[/{uuid}]` | list / revoke |

Enable: `mfa.webauthn.enabled: true` (RP ID from `http.public_url`).

## Admin UI

Embedded at `/` (`internal/frontend/dist`): sign-in, TOTP + WebAuthn MFA, passkey enrollment/revoke, quota usage, tenant user admin.

### Account / admin API

| Endpoint | Role |
|----------|------|
| `GET/PATCH /api/v1/me` | profile + `quota_bytes` / `used_bytes` |
| `GET /api/v1/admin/users` | list tenant users (admins only) |
| `POST /api/v1/admin/users` | create local user |
| `PUT /api/v1/admin/users/{uuid}/quota` | set quota (0 = unlimited) |
| `PATCH /api/v1/admin/users/{uuid}` | enable/disable |

Admins: `http.admins` email list (default `admin@example.com`).

## Next milestones

FlowSync FolderCreate/MoveItems → richer admin UI (create user form) → …

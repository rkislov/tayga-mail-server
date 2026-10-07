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
  ├─ flowsync (original FlowSync engine — ActiveSync/EWS wire; Apache-2.0)
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

## FlowSync (original)

Original sync engine by Кислов Роман Сергеевич (Apache-2.0). Wire-compatible with ActiveSync-class mobile clients and EWS-class desktop clients. **Not** a fork of Z-Push/SOGo/OpenChange. Responses carry `X-FlowSync: Tayga-Proprietary` (engine brand; project license remains Apache-2.0).

| Endpoint | Role |
|----------|------|
| `/Autodiscover/Autodiscover.xml` | Outlook POX (IMAP/SMTP/POP + EXPR/EXCH FlowSync) |
| `/autodiscover/autodiscover.json/…` | Mobile JSON Autodiscover |
| `/.well-known/autoconfig/mail/config-v1.1.xml` | Mozilla/Thunderbird autoconfig |
| `/Microsoft-Server-ActiveSync` | FolderSync/Create/Delete/Update, Sync, MoveItems, Ping, Provision, GetItemEstimate (XML + WBXML) |
| `/EWS/Exchange.asmx` | Find/Get/Create/Update/Delete + SyncFolderItems for mail, calendar, contacts; FindFolder |

Collection/item IDs exposed to clients are **UUIDs** (mailbox / message / calendar / address book / event / contact / device / policy).

FolderSync advertises mailboxes plus CalDAV calendars (type 8) and CardDAV address books (type 9). Sync write-back (XML and WBXML): calendar/contact Add·Change·Delete; mail Change (`Read`) and Delete; FolderCreate/MoveItems for mailboxes. EWS CreateItem/UpdateItem/DeleteItem for the same. Provision returns a richer device policy with a UUID `PolicyKey`.

Enable: `flowsync.enabled: true` (default).

## Protocols (standard ports)

Privileged ports (<1024) require root or `CAP_NET_BIND_SERVICE`. TLS certs: `tls.cert_file` / `tls.key_file` (`auto_generate: true` creates a self-signed pair for lab use).

| Protocol | Port | TLS |
|----------|------|-----|
| SMTP MX | `25` | STARTTLS (optional) |
| SMTP submission | `587` | STARTTLS |
| SMTPS | `465` | implicit TLS |
| IMAP | `143` | STARTTLS |
| IMAPS | `993` | implicit TLS |
| POP3 | `110` | STLS |
| POP3S | `995` | implicit TLS |
| ManageSieve | `4190` | STARTTLS |
| HTTP | `80` | redirect → HTTPS when enabled |
| HTTPS | `443` | implicit TLS (HTTP/2 ALPN) |

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

Embedded at `/` (`internal/frontend/dist`): sign-in, TOTP + WebAuthn MFA, passkey enrollment/revoke, Sieve filters, quota usage, tenant user admin, TLS certificate management.

### Account / admin API

| Endpoint | Role |
|----------|------|
| `GET/PATCH /api/v1/me` | profile + `quota_bytes` / `used_bytes` |
| `GET/PUT/DELETE /api/v1/sieve/scripts[/{name}]` | list / save / delete Sieve scripts |
| `POST /api/v1/sieve/scripts/{name}/activate` | set active script |
| `GET /api/v1/admin/tenant` | current tenant + domains |
| `GET/POST /api/v1/admin/domains` | list / add mail domains |
| `DELETE /api/v1/admin/domains/{name}` | remove empty domain |
| `GET /api/v1/admin/users` | list tenant users (admins only) |
| `POST /api/v1/admin/users` | create local user |
| `PUT /api/v1/admin/users/{uuid}/quota` | set quota (0 = unlimited) |
| `PATCH /api/v1/admin/users/{uuid}` | enable/disable |
| `PUT /api/v1/admin/users/{uuid}/password` | reset local password (min 8 chars) |
| `GET /api/v1/admin/tls` | active certificate status |
| `PUT /api/v1/admin/tls` | install PEM certificate + private key (hot reload) |
| `POST /api/v1/admin/tls` | generate self-signed cert (`hosts`, `days`) |
| `GET /api/v1/admin/status` | monitoring snapshot (tenant + server counters) |
| `GET /api/v1/admin/backup?include_mail=1` | download tenant backup `.tar.gz` |

Prometheus: `/metrics` includes `tayga_*` gauges (users, messages, bytes, …).

CLI backup / restore:

```bash
./tayga-mail backup -config configs/tayga.example.yaml -out backup.tar.gz
./tayga-mail restore -config configs/tayga.example.yaml -in backup.tar.gz
```

Admin restore: `POST /api/v1/admin/backup/restore` (multipart `file`).

HA / ops: see [docs/ha.md](ha.md).

Admins: `http.admins` email list (default `admin@example.com`).

## License & authorship

- **License:** Apache License 2.0 (`LICENSE`, `NOTICE`)
- **Author:** Кислов Роман Сергеевич (Roman Sergeyevich Kislov)
- **UI:** abstract cosmic backgrounds (author artwork under `internal/frontend/dist/assets/`)

## Mailstore

Local Maildir++ under `mailstore.root`. Optional S3-compatible write-through (`mailstore.object_store`): messages mirrored to the bucket; cache miss on `Read` pulls from object storage.

CLI: `tayga-mail sync-objects` reconciles indexed message paths between local maildir and the object store.

## Next milestones

Leader election / fencing for multi-writer maildir; periodic in-process object sync ticker.

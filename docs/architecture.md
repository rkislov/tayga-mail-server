# Tayga Mail Server — Architecture (Phase 1–7)

## Overview

TMS is a single-binary mail server (`tayga-mail`) built in Go with `CGO_ENABLED=0`.

```
cmd/tayga
  ├─ config (bootstrap YAML) + settings hub (DB overlay)
  ├─ storage (sqlite | postgres; UUID PKs everywhere)
  ├─ auth (local Argon2id; LDAP hybrid; OIDC; TOTP + WebAuthn MFA; opaque tokens)
  ├─ mailstore (Maildir++)
  ├─ smtp / imap / pop3 / managesieve
  ├─ sieve (foxcpp/go-sieve on delivery)
  ├─ dav (CalDAV + CardDAV via emersion/go-webdav)
  ├─ flowsync (original FlowSync engine — ActiveSync/EWS wire; Apache-2.0)
  └─ httpapi (/healthz, /readyz, /metrics, /api/v1/auth/*, /dav/*, FlowSync, embedded admin UI)
```

## Configuration

**Bootstrap YAML** (required to open the DB): `server.hostname`, `storage.*`, `mailstore.root`. Optional: `seed`, `http`/`tls`/`log` emergency overrides.

**Runtime settings** live in the `settings` table (JSON per section: `spam`, `scan`, `dmarc`, `smtp`, `ldap`, …). Admins edit them in the UI (**Server settings**) or via:

- `GET /api/v1/admin/settings`
- `GET|PUT /api/v1/admin/settings/{section}`

Secrets are redacted on GET (`***`); PUT with empty/`***` keeps the previous value. Changing most sections sets `restart_required` (listeners and SMTP policies are applied at process start). `storage` and `mailstore.root` stay file-only.

## Storage

- **All entity primary keys and FKs are UUID strings (`TEXT`)**, including FlowSync device rows, policy keys, and sync-state tokens. IMAP `uid` / `uidnext` / `uidvalidity` stay numeric protocol counters.
- Shared schema with `tenant_id` isolation.
- Migrations through `011_settings` (plus greylist, DMARC agg, sticky, outbound, …).

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
./tayga-mail version
./tayga-mail -config configs/tayga.example.yaml
# Cross builds: VERSION=v0.6.0 ./scripts/crossbuild.sh
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

## Web UI

Embedded at `/` (`internal/frontend/dist`): app shell with **mail**, **calendar**, **contacts**, **files** (Outlook-style panes; ≤900px master-detail), user settings (profile / security / appearance / Sieve+vacation / language ru|en), and admin (monitor charts / tenants / TLS / server / CoS). Roles: `global_admin`, `domain_admin`, `user`. Unauthenticated users are sent to the login card. Themes via `localStorage` (`tayga.theme`). Thunderbird companion: `extensions/thunderbird-tayga/`.

### User apps API

| Endpoint | Role |
|----------|------|
| `GET /api/v1/mail/mailboxes` | list mailboxes (+ shared) |
| `GET/PUT/DELETE …/mailboxes/{id}/acl` | folder ACL |
| `GET /api/v1/mail/mailboxes/{id}/messages` | list messages |
| `GET/PATCH/DELETE /api/v1/mail/messages/{id}` | read / flags / delete |
| `POST /api/v1/mail/send` | send (local + outbound queue) |
| `GET/PUT /api/v1/mail/vacation` | out-of-office (Sieve vacation) |
| `GET/POST/DELETE /api/v1/mail/delegates` | secretary / delegate access |
| `GET /api/v1/calendar/calendars` | list calendars (+ shared) |
| `GET/PUT/DELETE …/calendars/{id}/acl` | calendar ACL |
| `GET/POST /api/v1/calendar/calendars/{id}/events` | list / create events |
| `GET/DELETE /api/v1/calendar/events/{id}` | read / delete |
| `GET /api/v1/contacts/books` | list address books |
| `GET/POST /api/v1/contacts/books/{id}/cards` | list / create cards |
| `GET/DELETE /api/v1/contacts/cards/{id}` | read / delete |
| `GET /api/v1/files?path=` | list directory under `mailstore.root/files/{user}` |
| `GET/PUT /api/v1/files/content?path=` | download / upload |
| `POST /api/v1/files/mkdir` | create folder |
| `DELETE /api/v1/files?path=` | remove file or folder |
| `GET/POST/DELETE /api/v1/files/shares` | create/list/revoke public links |
| `GET /s/{token}` | anonymous download of a file share |

### Account / admin API

| Endpoint | Role |
|----------|------|
| `GET/PATCH /api/v1/me` | profile + roles / `admin_scope` / quota |
| `GET/PUT/DELETE /api/v1/sieve/scripts[/{name}]` | list / save / delete Sieve scripts |
| `POST /api/v1/sieve/scripts/{name}/activate` | set active script |
| `GET /api/v1/admin/tenant` | current tenant + domains |
| `GET/POST /api/v1/admin/domains` | list / add mail domains (`global_admin` to add/delete) |
| `DELETE /api/v1/admin/domains/{name}` | remove empty domain |
| `GET /api/v1/admin/users` | list users in admin scope |
| `POST /api/v1/admin/users` | create local user |
| `PUT /api/v1/admin/users/{uuid}/quota` | set quota (0 = unlimited) |
| `PATCH /api/v1/admin/users/{uuid}` | enable/disable / roles / domain_ids / service_class_id |
| `PUT /api/v1/admin/users/{uuid}/password` | reset local password (min 8 chars) |
| `GET/POST/PUT/DELETE /api/v1/admin/service-classes` | CoS templates (`global_admin` write) |
| `GET /api/v1/admin/tls` | active certificate status (`global_admin`) |
| `PUT /api/v1/admin/tls` | install PEM certificate + private key (hot reload) |
| `POST /api/v1/admin/tls` | generate self-signed cert (`hosts`, `days`) |
| `GET /api/v1/admin/status` | monitoring snapshot (tenant + server counters) |
| `GET /api/v1/admin/outbound` | list outbound retry queue |
| `POST /api/v1/admin/outbound/{uuid}/retry` | schedule immediate retry |
| `DELETE /api/v1/admin/outbound/{uuid}` | drop queued message |
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
- **UI:** Tailwind SPA shell (mail / calendar / contacts / files + settings/admin); i18n ru/en; themes (localStorage): taiga (default), cosmos, city, kalyazin, temple, moscow — picker in **Внешний вид** (backdrop art © Алиса Кислова except moscow; assets under `internal/frontend/dist/assets/`)

## Mailstore

Local Maildir++ under `mailstore.root`. Optional S3-compatible write-through (`mailstore.object_store`): messages mirrored to the bucket; cache miss on `Read` pulls from object storage.

CLI: `tayga-mail sync-objects` reconciles indexed message paths between local maildir and the object store.

## HA fencing

`ha.mode: active_standby` (Postgres): advisory lock; `ha.fence: mx|writers` gates MX-only or all maildir writers.  
`ha.mode: sticky`: per-user writer leases (`user_writer_leases`); see [ha.md](ha.md).

## Outbound SMTP

Authenticated submission may RCPT external addresses when `smtp.relay.host` is set **or** `smtp.outbound_direct: true`. Messages are optionally DKIM-signed (`smtp.dkim`), then sent via smart-host or direct MX (port 25). Unauthenticated inbound MX remains local-only.

SMTP `rate_limit.per_ip` / `per_user` (windowed) returns `421` when exceeded.

Outbound messages are enqueued (`smtp.queue`, table `outbound_queue`) and retried with exponential backoff. After `max_attempts`, a multipart/report DSN is delivered to the local envelope-from mailbox.

Metric: `tayga_outbound_queued`. Admin: `GET/DELETE /api/v1/admin/outbound[/{id}]`, `POST …/{id}/retry` (Monitoring card).

## Virus scan / quarantine

Inbound SMTP local delivery can run `scan` (ClamAV clamd INSTREAM or exec). Actions: `reject` (550), `quarantine` (folder, default `Quarantine`), `tag` (deliver with `X-Virus-*` headers). See `scan:` in `configs/tayga.example.yaml`.

## Spam (Rspamd)

After virus scan and before Sieve, optional `spam:` checks Rspamd HTTP `/checkv2`. With `follow_rspamd: true`, Rspamd `reject` / `greylist` / `add header` map to SMTP reject, 421 greylist, or `X-Spam-*` tags. Score thresholds (`reject_above`, `quarantine_above`, `tag_above`) apply when set; quarantine files into `spam.folder` (default `Junk`).

Admin: `GET /api/v1/admin/quarantine` (folders Quarantine+Junk), `POST …/{id}/release` (move to INBOX), `DELETE …/{id}` (Monitoring card).

## Inbound mail auth (SPF / DKIM / DMARC / ARC)

On unauthenticated MX (before virus/spam), optional checks in order:

1. `greylist:` — first-seen `(ip, MAIL FROM, RCPT)` deferred with `421` until `delay` elapses (persisted in `greylist` table for HA; `ipv4_net` can key by /24)
2. `helo:` — client HELO/EHLO syntax (`require_fqdn`, reject `localhost` / bare IP)
3. `iprev:` — PTR + forward-confirmed reverse DNS (`Authentication-Results: … iprev=pass|fail`)
4. `spf:` — `blitiri.com.ar/go/spf` against client IP + client HELO + MAIL FROM
5. `dkim_verify:` — DNS TXT public key via go-msgauth
6. `arc:` verify — existing ARC chain (`Authentication-Results: … arc=pass|fail|none`)
7. `dmarc:` — policy lookup + SPF/DKIM alignment (`action: follow` honors `p=reject`); with `dmarc.arc_trust: true`, ARC `cv=pass` softens fail (no reject, still records `dmarc=fail reason="arc-pass"`)
8. `arc:` seal — optional new ARC set with our auth results (`seal: true` + RSA key)

Each step may inject `Authentication-Results`. Authenticated submission skips all of these. Admin quarantine UI shows spam/virus headers, filters (`folder`/`kind`/`q`), and message preview. See `configs/tayga.example.yaml`.

### DMARC aggregate reports (rua)

With `dmarc.report.enabled`, each evaluation is aggregated in `dmarc_agg` (per UTC day). A background flusher builds RFC 7489 XML (`gzip`), emails `mailto:` addresses from the published `rua=`, and deletes sent days. Requires outbound (`smtp.queue` / relay / `outbound_direct`).

### DMARC failure reports (ruf)

With `dmarc.report.failure`, a DMARC fail that matches published `fo=` (default `0` = all mechanisms failed alignment) triggers an RFC 6591 AFRF message to `mailto:` addresses in `ruf=`. Sent immediately via the same outbound path as rua.

## LDAP group sync

Per-domain `ldap.domains.*.groups` maps membership to Tayga roles (`admin`) via `memberof` or group search. Roles are stored on `users.roles` and grant admin API access alongside `http.admins`. Optional `sync:` provisions members of `group_dns` / `admin_groups` on an interval or via `tayga-mail ldap-sync`.

With `groups.nested: true`, membership is expanded transitively: `nested_mode: walk` BFS via group `memberOf` (or parent search on `member`), or `nested_mode: chain` using AD `LDAP_MATCHING_RULE_IN_CHAIN` (`1.2.840.113556.1.4.1941`). Depth is capped by `max_depth` (default 8).

## ARC (Authenticated Received Chain)

`arc:` uses `github.com/rest-mail/go-arc` (RFC 8617). Verify runs before DMARC so `dmarc.arc_trust` can override disposition; seal runs after so the sealed set includes DMARC results.

## Outbound MTA-STS / TLS-RPT

With `smtp.outbound_direct`, delivery uses opportunistic STARTTLS (EHLO = `server.hostname`). When `smtp.mta_sts.enabled`, recipient `_mta-sts` TXT + `https://mta-sts.<domain>/.well-known/mta-sts.txt` are fetched and cached (`max_age`). Mode `enforce` restricts MX to policy patterns and requires successful STARTTLS; `testing` applies the same checks for reporting but still delivers on failure. `smtp.mta_sts.fail_open` continues without STS if the policy cannot be fetched.

With `smtp.tls_rpt.enabled`, outbound TLS outcomes are aggregated and emailed as RFC 8460 JSON to `mailto:` addresses from `_smtp._tls.<domain>` (`rua=`).

To publish our own policy for inbound peers, set `smtp.mta_sts.publish.enabled`. Tayga serves `GET /.well-known/mta-sts.txt` (TLS required in practice). Operators must also publish:

- `mta-sts.<domain>` A/AAAA (and certificate SAN) pointing at the HTTPS listener
- `_mta-sts.<domain>` TXT `v=STSv1; id=<publish.id>`
- optional `_smtp._tls.<domain>` TXT `v=TLSRPTv1; rua=mailto:…` for inbound TLS reports from others

## Outbound DANE (TLSA)

With `smtp.dane.enabled`, direct MX delivery looks up `_25._tcp.<mx>` TLSA (via system resolvers). If records exist, STARTTLS is required and the peer certificate must match (usages 0–3, selectors 0–1, matching 0–2); plaintext fallback is disabled for that host. Usage 3 (DANE-EE) skips PKIX and trusts TLSA alone. Lookup failures follow `fail_open` (default true). Note: without DNSSEC validation this is opportunistic DANE (TLSA can be spoofed on the path to the resolver).

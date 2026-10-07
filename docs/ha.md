# High availability & operations notes

Tayga Mail is a **single-binary** server. This document describes practical HA and ops patterns; there is no built-in cluster coordinator yet.

## What can be shared

| Component | Sharing model |
|-----------|----------------|
| **PostgreSQL** | Preferred for multi-node. Point every node at the same DSN (`storage.driver: postgres`). |
| **SQLite** | Local only. Do not share a SQLite file over NFS for writers. |
| **Maildir** (`mailstore.root`) | Put on shared POSIX storage (NFS/CephFS) **or** keep node-local and pin users to a node. Concurrent writers on the same maildir need careful locking; prefer one active writer per user tree. |
| **TLS certs** | Same `tls.cert_file` / `tls.key_file` on all nodes, or terminate TLS at a load balancer. Hot-reload via admin UI applies per-process. |

## Recommended topologies

### 1. Active / standby (simplest)

1. One active `tayga-mail` process.
2. Warm standby with the same config, DB replica (Postgres), and mailstore snapshot/replication.
3. Fail over VIP / DNS to the standby; start or promote the standby process.
4. Run regular backups (`tayga-mail backup`) from the active node.

### 2. Multiple frontends, shared Postgres + shared maildir

1. N identical Tayga nodes behind L4/L7 load balancer (sticky sessions help IMAP IDLE).
2. All nodes use the same Postgres DSN and the same `mailstore.root`.
3. SMTP MX can round-robin; submission/IMAP should prefer session affinity.
4. Admin TLS uploads must be repeated on each node (or mount shared certs).

### 3. LB terminates TLS

Expose only HTTP `:80` on Tayga; LB presents `:443` / `:993` / `:465`. Set `http.public_url` to the external HTTPS URL so Autodiscover and WebAuthn origins stay correct.

## Backups & restore

```bash
# Full export (metadata + maildir)
./tayga-mail backup -config configs/tayga.example.yaml -out backup.tar.gz

# Restore into an empty or existing instance (idempotent for known emails)
./tayga-mail restore -config configs/tayga.example.yaml -in backup.tar.gz
```

Archive contents: tenants, domains, users (with password hashes), sieve scripts, optional maildir. On restore, maildir files are written and **reindexed** into IMAP mailboxes.

Also available from the admin UI (Monitoring card) and:

- `GET /api/v1/admin/backup?include_mail=1`
- `POST /api/v1/admin/backup/restore` (multipart `file`)

**Secrets:** backup archives contain password hashes — store them encrypted at rest.

## Health checks

| Endpoint | Use |
|----------|-----|
| `GET /healthz` | process up |
| `GET /readyz` | DB ping |
| `GET /metrics` | Prometheus (`tayga_*` gauges) |

LB should mark a node unhealthy if `/readyz` fails.

## IMAP IDLE (cross-node)

IDLE uses an in-process update hub. When `storage.driver` is **postgres**, Tayga also starts a **LISTEN/NOTIFY** bus on channel `tayga_idle`: a delivery on node A wakes IDLE clients on every peer that shares the same DSN. Each node ignores its own NOTIFY echo.

SQLite builds stay single-node (no bus). Sticky IMAP affinity is still recommended so mailbox sessions stay on one frontend, but EXISTS notifications no longer require it for multi-node Postgres HA.

## What is not included yet

- Automatic leader election / fencing
- Built-in object-storage maildir backend
- Point-in-time DB restore orchestration (use Postgres tooling)

## Checklist before production HA

- [ ] Postgres with automated backups and PITR
- [ ] Shared or replicated maildir strategy documented
- [ ] TLS certs provisioned (or LB TLS) and `http.public_url` correct
- [ ] Periodic `tayga-mail backup` off-box
- [ ] Restore drill on a staging instance
- [ ] Monitoring scrapes `/metrics` and alerts on `/readyz`

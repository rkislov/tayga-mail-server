# XMPP + OMEMO (TMS-XMPP-001) — implementation notes

Canonical requirements: [tms-xmpp-001.md](./tms-xmpp-001.md).

## Status

| Stage | Content | Status |
|-------|---------|--------|
| 1 | C2S RFC 6120/6121: stream, STARTTLS, SASL PLAIN/X-OAUTH2, bind, message/presence, roster, offline flush | **done** (lab) |
| 2 | PubSub/PEP publish + get (OMEMO device list / bundles nodes) | **done** (lab) |
| 3 | MAM archive + query (`urn:xmpp:mam:2`); message carbons enable/sent | **done** (lab) |
| Bots | XEP-0114 components + HTTP bot tokens | **done** (lab) |
| Web UI | Built-in **Chat** app (plaintext over REST + SSE) | **done** (lab) |
| Admin | Админка **XMPP**: C2S, компоненты, HTTP-боты | **done** |
| — | S2S federation | pending |
| 4–5 | Client OMEMO (WASM/JS); private keys never on server | pending |
| 6 | Interop Gajim / Conversations / Dino | pending |

Module: `internal/xmpp`. C2S listeners default **`xmpp.enabled: false`**; the web Chat app uses the same hub/MAM even when C2S is off. Enable C2S for external clients.

## Web messenger (user UI)

In the shell nav: **Чат / Chat**.

| Endpoint | Role |
|----------|------|
| `GET /api/v1/chat/roster` | roster list |
| `POST /api/v1/chat/roster` | add/open peer `{jid,name}` |
| `GET /api/v1/chat/history?with=` | MAM history |
| `POST /api/v1/chat/send` | send plaintext `{to,body}` |
| `GET /api/v1/chat/events` | SSE live feed (`Authorization` or `?access_token=`) |

Messages are stored in MAM as XMPP stanzas and fan out to online C2S sessions, components, and other web SSE subscribers. **OMEMO in the browser is not enabled yet** — treat web chat as plaintext until Stage 4–5.

## Admin UI

В навбаре админа: **XMPP** (между УЦ и Сервер).

| Действие | Как |
|----------|-----|
| Включить C2S / порты | чекбокс + listen / listen_tls / component_listen → **Сохранить C2S** (`PUT /api/v1/admin/settings/xmpp`) |
| Статус | `GET /api/v1/admin/bots` → `status` + список ботов и доменов компонентов |
| HTTP-бот | форма на странице → `POST /api/v1/admin/bots` (токен один раз) |
| Компоненты (секреты) | секция `xmpp` на странице **Сервер** (JSON) или YAML |

Смена слушателей требует **перезапуска** `tayga-mail`. Веб-чат работает и при `xmpp.enabled: false`.

## Bots

### 1. XMPP Component (XEP-0114)

```yaml
xmpp:
  enabled: true
  component_listen: ":5347"
  components:
    - name: support
      subdomain: bots
      secret: "shared-secret"
```

Handshake: SHA-1 hex of `streamID + secret`.

### 2. HTTP bot API (mailbox-linked)

- Admin UI **XMPP** или `POST /api/v1/admin/bots` → `token` once
- `DELETE /api/v1/admin/bots/{id}`
- `POST /api/v1/bots/xmpp/send`, `GET /api/v1/bots/xmpp/inbox`

## Server vs client crypto

| Layer | Responsibility |
|-------|----------------|
| **Server** | Routing, roster, offline, PEP (public), MAM (opaque), carbons, web SSE |
| **Client (future OMEMO)** | Encrypt/decrypt, private keys in IndexedDB |

## License ADR

Tayga is **Apache-2.0**. AGPL/GPL Signal stacks are **not** linked into `tayga-mail`.

## Config

```yaml
xmpp:
  enabled: false   # set true for Gajim/Conversations on :5222
  listen: ":5222"
  listen_tls: ":5223"
  require_tls: true
  component_listen: ":5347"
```

## Wallpapers

Theme **Стрит-арт** mural advertises **Tayga Mail Server** and the new **Chat / Messenger** feature (`assets/wallpapers/street-art/`).

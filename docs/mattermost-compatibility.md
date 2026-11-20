# Experimental Mattermost client compatibility

TMS contains an independent chat engine (`internal/chat`) and an API v4 adapter
(`internal/mattermost`). Neither the Mattermost server nor its web application
is embedded. This is a protocol prototype, not a supported replacement for a
complete Mattermost installation. Existing XMPP and the TMS web chat are unchanged
and use their existing history; the two histories are not bridged or migrated.

## Enable

Set this in the server YAML and restart:

```yaml
mattermost:
  enabled: true
http:
  public_url: https://mail.example.com
```

The switch is disabled by default. It is also exposed through the `mattermost`
server-settings section and requires a restart. Use the existing HTTPS address
as the client server URL; API requests use `/api/v4/`. A reverse proxy must forward
WebSocket upgrades at `/api/v4/websocket`. Disabled instances return JSON 404 for
this namespace rather than serving the TMS HTML application.

Sign in using the full TMS email address and password. Existing account status,
tenant boundaries, and MFA policies apply. TOTP and backup codes are handled via
the login request's `token` field. Accounts requiring WebAuthn alone cannot complete
this prototype's login; it does not bypass their second factor. Sessions use the
existing TMS access-token expiry and require a fresh login after expiry; a native
Mattermost session renewal contract has not yet been implemented.

A TMS tenant is represented as one private team. On first authenticated access,
the user joins its `town-square` channel. Other channels require membership;
private-channel invitations require the creator. Cross-tenant lookup, direct
messages and membership changes are denied. Stable user/team IDs are derived from
TMS IDs in the 26-character alphabet accepted by Mattermost clients.

Migration `061_chat` creates persistent channels, members and posts for SQLite and
PostgreSQL. Only SQLite has been exercised by the integration suite. Restarting
the adapter preserves history. These new tables are **not included in the logical
TMS tar backup yet**; take a database backup when testing data that needs keeping.

## Implemented surface

All paths below are relative to `/api/v4`:

- Discovery: `GET /system/ping`, `/config/client`, `/license/client`.
- Authentication: `POST /users/login`, `/users/logout`; bearer tokens and
  WebSocket `authentication_challenge`. No browser session cookies.
- Users: `GET /users`, `/users/{id}`, `/users/me`; `POST /users/ids`,
  `/users/status/ids`. User lists are tenant-scoped; `in_team` and membership-checked `in_channel`
  filters are supported. Other directory filters are not implemented. Status queries currently return offline.
- Initial client state: `GET /users/me/teams`, `/users/me/teams/members`,
  `/users/me/preferences`, `/users/me/channels`,
  `/users/me/teams/{team_id}/channels`,
  `/users/me/teams/{team_id}/channels/members`, `/users/me/teams/unread`,
  `/teams/{team_id}`; `POST /roles/names`.
  The current user's explicit ID can replace `me`. Preferences are an empty,
  read-only set. Roles expose only prototype permissions.
- Channels: `POST /channels`, `/channels/direct`; `GET /channels/{id}`,
  `/channels/{id}/members`, `/channels/{id}/members/{user_id}`,
  `/channels/{id}/stats`; `POST /channels/{id}/members`,
  `/channels/members/me/view`.
- Messages: `POST /posts`, `GET /posts/{id}`, `/posts/{id}/thread`,
  `/channels/{id}/posts`. Posts support text and replies to a root post.
  History supports `page`, `per_page` (1–200), and `since` in milliseconds;
  before/after cursors are explicitly unsupported. Thread retrieval is capped
  at 200 posts. Retrying the same nonempty `pending_post_id` returns the existing
  post, and a conflicting payload is rejected.
- WebSocket: `GET /websocket`; `hello`, `posted`, and `ping` responses.
  `posted.data.post` is a JSON string, matching the client wire contract.
  A new connection ID is issued on reconnect; clients must resync history.
  Slow consumers are disconnected rather than silently losing live events.
  Membership and token validity are checked before delivery; idle connections
  revalidate the token every 15 seconds. Browser origins must match the request
  host or the configured public URL; native clients may omit Origin.

Unsupported API operations return structured JSON errors (normally 501).
File attachments, reactions, editing/deleting messages, search, preferences writes,
team administration, push notifications, calls, and XMPP history migration are not
implemented. Public channels currently require an invitation from an existing
member; public browsing and self-join are not implemented. Events for new channel
creation and membership changes are not yet emitted, so a client refresh may be
needed. Live fan-out is process-local: multi-node WebSocket delivery needs a shared
event bus before this can be used with HA.

The adapter reports `10.11.0` as its experimental API compatibility target and
`tms-prototype` as its build; this does not assert full behavior compatibility or
certification with any released app. Official Android/iOS apps have **not yet been
run against it**. The desktop app also requires a compatible web UI, which this
adapter does not serve.

## Validation and next checkpoint

```sh
go test ./internal/chat ./internal/mattermost
go test ./internal/httpapi -run TestMattermostOptIn
go test -race ./internal/chat ./internal/mattermost
```

A separate smoke run with the official Go client module
`github.com/mattermost/mattermost/server/public` at **v0.6.0** passed login,
team/channel discovery, post creation, history, thread retrieval, WebSocket
`hello`/`posted` decoding and logout. That check used a temporary module; no
Mattermost server or SDK dependency was added to TMS. It validates those SDK
contracts, not the complete mobile app startup.

Tests use a real SQLite database, HTTP handler and WebSocket connections. They
exercise login/logout, MFA rejection, private membership, tenant isolation,
threads, counters, idempotent/concurrent retries, persistence across adapter
recreation, origin checks, live delivery, revoked socket sessions, and slow-consumer
recovery from persisted history.

The next compatibility checkpoint is a pinned official Android or iOS build:
server discovery → sign-in → initial data load → open a channel → send/receive →
reconnect and recover missed history. Capture unsupported endpoint calls, implement
their actual contracts and record the app version and result here. Until that
checkpoint passes, keep XMPP enabled for users who need a working external client.

## Protocol references

- [Official API v4 specifications](https://github.com/mattermost/mattermost/tree/master/api/v4/source)
- [Official client bootstrap flow](https://github.com/mattermost/mattermost-mobile/blob/main/app/actions/remote/entry/common.ts)
- [Official WebSocket event models](https://github.com/mattermost/mattermost/blob/master/server/public/model/websocket_message.go)

## iOS test deployment — 2026-11-08

The prototype is enabled at `https://mail.kislovs.ru`. Discovery, client config,
license and readiness returned HTTP 200. A native WebSocket upgrade returned
101. HTTPS, SMTPS, IMAPS, POP3S and XMPP direct TLS certificates were verified.
Phone-side sign-in and messaging remain to be tested by the user.

The deployed overlay was built from **e9973dd / TMS 0.10.8**, preserving that
server's mail fixes, as `0.10.8+mattermost.1`. Its source is in the local worktree
`/Users/roman/dev/tms-mattermost-ios`, branch `codex/mattermost-ios`. Use that
worktree for subsequent iOS compatibility fixes. On this newer database the chat
migration is **067_chat**; 061 is already used by XMPP SCRAM. The older working
copy's migration numbering must not be used to update this server.

Before installation a consistent SQLite backup passed `quick_check` and was
compressed. The previous binary/configuration, database snapshot, source overlay,
build hashes and rollback script are retained on the server under
`/var/lib/tayga/backups/pre-mattermost-ios-20261108/`. Rollback restores the
previous binary and Mattermost setting while keeping current mail and chat data:

```sh
sudo python3 /var/lib/tayga/backups/pre-mattermost-ios-20261108/rollback.py
```

Unsupported API requests log `mattermost unsupported API` with method, path and
user agent, excluding request bodies, query strings and bearer tokens. To inspect
failures during phone testing:

```sh
sudo journalctl -u tayga.service --since '10 minutes ago' -o cat --no-pager | grep -i mattermost
```

In Mattermost iOS, add server `https://mail.kislovs.ru` (without `/api/v4`) and
sign in with the full TMS email address. Start with opening Town Square and
sending a text message. Record the app version and the first failing screen;
then check receive, replies and history after closing/reopening the app.

### iOS 2.45.1 (build 832): server identity fix

The first phone attempt stopped at server selection because `/config/client`
omitted `DiagnosticId`. Fixed and deployed as `0.10.8+mattermost.2`: the adapter
now returns a 26-character installation ID, atomically initialized in the settings
table under `mattermost.diagnostic_id` and preserved across adapter recreation and
public-URL changes. No schema migration was needed. Regression tests, including
race checks, passed; the live HTTPS endpoint returned HTTP 200 and the same nonempty
ID on repeated requests. Phone-side retry is still pending.

In the server form, Display Name is a label such as "Tayga"; email/password belong
to the subsequent login screen. Leave the advanced authentication secret empty.

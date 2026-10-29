# XMPP compatibility implementation plan

Updated 2026-10-29. This is an implementation checklist, not a claim of complete RFC/XEP conformance.

## Current batch: core connection and archive correctness

- [x] Close replaced resource sessions outside the hub mutex to prevent recursive-lock deadlock.
- [x] Route full-JID messages only to the addressed resource.
- [x] Require authentication before resource binding and validate bind namespaces.
- [x] Validate STARTTLS and SASL namespaces during negotiation.
- [x] Parse MAM query forms with XML rather than regular expressions.
- [x] Add owner-scoped RSM before/after cursors, including empty before for the last page.
- [x] Add date/peer filters, bounded page sizes, count-only requests, first/last IDs and accurate completion.
- [x] Regression tests for replacement, resource routing, binding and stable cursor traversal.

## Additional work in progress

- [x] XEP-0198 stanza acknowledgements, modulo counters and bounded outstanding window.
- [x] Reject acknowledgements beyond the sent count; SM control elements do not affect counters.
- [x] Normalize authentication and binding JIDs using PRECIS/IDNA and validate length/characters.
- [x] XEP-0198 resumption after reauthentication, owner-bound tokens, acknowledged-prefix removal and replay without recounting.
- [x] Bound replay storage: 120 seconds, 1 MiB per session, 4 retained sessions per user, 256 globally and 16 MiB aggregate.
- [x] Treat `encoding/xml` unexpected EOF on an open stream as a transport loss; preserve counters for a partially received stanza without acknowledging it. Clean stream closure still invalidates resumption.
- [x] Resume an authenticated owner's stream even when the old TCP connection has not detected the disconnect. Validate the token and acknowledgement first, close only that transport, and wait for reader/room cleanup before using its snapshot.
- [x] Bound the complete replay to a 10-second write deadline; retain pending messages and room state if that replay disconnects again.
- [x] Loopback TCP tests: SASL, two resources, received/sent Carbons, invalid counters, undetected and repeated disconnects, partial incoming XML, replay without duplicate Carbons, and explicit logout. Additional tests cover the cleanup window, cross-account rejection, replay failure and counter wrap.
- [x] Monal/iPhone on server 0.9.36: operator confirmed background push delivery and reconnection with history intact after Wi-Fi → mobile data → Wi-Fi (2026-10-28; app version not recorded).
- [ ] Repeat device acceptance for Conversations and ChatSecure, and large-history paging in native clients. Retained streams are in-memory and do not survive a server restart; offline messages remain database-backed.
- [ ] Existing-identity normalization collision audit before any stored identity migration.

Validation on 2026-10-28: `go test ./...` and `go test -race ./internal/xmpp` passed for stream recovery. Commit `de68881` was installed as `0.9.34+xmpp.2` after a verified SQLite/binary/configuration backup, and is included in release 0.9.36. The service passed `/readyz`; STARTTLS 5222 and direct TLS 5223 both negotiated TLS 1.3 with certificate verification. Three Monal push registrations remained. Subsequent Monal device acceptance is recorded above and in [native push](xmpp-push.md).

Archive acceptance added on 2026-10-28: a loopback TCP scenario loads 257
messages through two authenticated resources, traverses forward/backward
pages, commits and delivers a new live message between queries, and resumes
the phone stream before continuing from its last cursor. It checks result
correlation, forwarded namespaces, stable IDs, count-only requests, foreign
cursor rejection and that history is not copied as Carbons to the other
resource. The server's 100-message page cap is exercised by requesting 500.
Separate regressions cover cursors outside the new peer/time filters and
unsupported, duplicate, multivalued or malformed filter input. Unsupported
filters return an error before any history is streamed.

Validation for the archive changes: `go test ./...`,
`go test -race ./internal/xmpp` and `go vet ./internal/xmpp` passed. The tests
use temporary SQLite databases and local TCP sockets; no production message
contents or push tokens are copied into fixtures.

Deployment on 2026-10-28: archive commit `bc37051` installed as
`0.9.36+xmpp.1`; binary SHA-256
`1ae9f8c347e9cd057494649ab227d3bd651dc4dfac860f715b58eee89b0a0fcf`.
The previous binary, configuration and a verified SQLite backup are in
`/var/backups/tayga/pre-0.9.36-xmpp.1-vUWjj9d1`. Readiness passed, and external
STARTTLS 5222/direct TLS 5223 probes negotiated TLS 1.3 with verified
certificates. Three Monal registrations remained and the push queue was
empty; no ERROR-level entries were found in the post-restart journal check.

Probe limitation: two external attempts with `to` preceding the namespace
attributes in the initial XML stream header timed out before STARTTLS.
The same header worked over server loopback, while external probes with
namespace attributes first completed on both ports. The cause has not been
established; this is a separate network-path diagnostic item, not evidence
that the server parser rejects that valid XML attribute order.

## Web OMEMO session isolation (2026-10-29)

Pending browser operations now retain their originating login, including time
spent waiting for a Web Lock, PEP requests, token refresh/retry, cached plaintext
and encrypted-file previews. Logout and relogin invalidate that context without
deleting local keys. Late refresh replies cannot replace a newer login's tokens
or share its pending refresh promise. Already dispatched requests may finish for
the original account; subsequent work is rejected after a login change.

Validation: the session-isolation, device-settings and independent browser-device
scripts passed, as did `go test ./...`. An isolated Go server with the built web
assets passed the actual HTTP API scenario: two browser devices, encrypted
delivery, deduplication, settings, logout/relogin, retained device ID and cached
plaintext. Independent Node.js AES-GCM encryption/decryption verifies the file
wire format in both directions. Native Monal/Conversations encrypted-file and
history acceptance remains pending; this batch does not claim that interop.
See [web OMEMO](web-omemo.md) for commands and storage boundaries.

Deployment on 2026-10-29: commit `4ed691e` installed as `0.9.36+xmpp.2`
(built `2026-10-29T11:20:40Z`), binary SHA-256
`2741108375c3288949da0530af54385db2f504b1b6edb7a0a62f8f0faa16b5da`.
The binary/configuration/verified SQLite backup is
`/var/backups/tayga/pre-0.9.36-xmpp.2-oXTiYHnp`.
Public readiness and the served JavaScript hash matched the build; canonical
external STARTTLS 5222 and direct TLS 5223 probes negotiated TLS 1.3 with
certificate verification. All three push registrations remained, the queue was
empty, and the recent journal contained no ERROR-level application entries.
These probes did not send user messages or trigger native push acceptance.

## Encrypted attachment transport (2026-10-29)

The web receiver accepts the historical Conversations 16-byte IV alongside the
XEP-0454 12-byte IV; new uploads still emit only the 12-byte form. Malformed URLs
are rejected before download. Chat upload/download capability URLs now support
credential-free CORS and preflight requests. HEAD retains its method when
delegating to the public share handler, returns no body and does not consume a
download count; ordinary public file/folder shares also accept metadata-only HEAD.

The HTTP regression reproduced the previous 405 preflight responses and HEAD
body/download-count changes before the fix. `go test ./...`, the OMEMO device,
settings and session scripts, independent Apple CryptoKit and Node.js wire-format
checks all passed. A disposable HTTPS instance with two browser origins passed
the UI upload → encrypted message → PNG/PDF preview/download cycle, including
legacy-IV receipt and no key fragments, credentials or referrers in file GETs.
These are transport/format tests, not native-app acceptance. Sources, commands
and remaining device checks are recorded in [web OMEMO](web-omemo.md).

Deployment on 2026-10-29: commit `ffe9e53` installed as `0.9.36+xmpp.3`
(built `2026-10-29T13:54:25Z`), binary SHA-256
`6999ccc9373cef0aa4d95cd9e8f02ef31bfd15ae57f0a2f27192499b395f0e0c`.
Previous binary/configuration and verified SQLite backup:
`/var/backups/tayga/pre-0.9.36-xmpp.3-5JZLrieS`.
External readiness, served JavaScript hash and PUT/GET CORS preflights passed.
Canonical STARTTLS 5222/direct TLS 5223 probes both negotiated verified TLS 1.3;
three push registrations remained, the queue was empty, and the recent journal
check found no ERROR-level application entries. On 2026-10-29 the operator
confirmed encrypted files open in both directions, web ↔ Monal, on this build.
The app version was not recorded; Conversations acceptance and native bytewise
comparison remain unverified.

## Local OMEMO device exclusion (2026-10-29)

An explicit setting excludes an old device from new encrypted sends while keeping
delivery to remaining verified devices. It applies to this browser/account only,
including other own devices; it neither changes the public PEP list nor revokes
account access. Missing or rotated excluded bundles do not block other recipients,
but unknown active devices still do. Sending with no recipient device fails even
if own-device copies are available. Cached history remains readable; uncached
incoming messages from an excluded sender are rejected until explicit verification.

Exclusions survive reload, PEP removal and reappearance. Restoring a device requires
fresh publication and a fingerprint comparison; a stale verification dialog cannot
undo an exclusion saved in another tab. Trust revocation remains distinct and
continues to block sends pending verification or explicit exclusion.

The four-browser scenario in `scripts/check-omemo-devices.mjs` verifies real ratchet
delivery/decryption, missing and changed keys, own/recipient exclusion, empty
recipient rejection, cached history, independent browser trust, restoration and
mobile layout. The actual HTTP API scenario also checks exclusion/restoration with
the embedded web build. Global device withdrawal and key-loss recovery remain
separate work; see [web OMEMO](web-omemo.md).

Deployment on 2026-10-29: commit `d322d4b` installed as `0.9.36+xmpp.4`
(built `2026-10-29T16:24:19Z`), binary SHA-256
`c686dff049f4cfa17b5456e2e945848ccd17d3f664f23ed441acf28a7bc69127`.
Previous binary/configuration and verified SQLite backup:
`/var/backups/tayga/pre-0.9.36-xmpp.4-pTZ0ZgfA`.
Public readiness and the served JavaScript SHA-256 matched the build
(`9831625cad5bb135cfef6683ecd1b96be22a2c2607db13c0a98976c4a0aa1002`).
External canonical STARTTLS 5222 and direct TLS 5223 probes negotiated TLS 1.3
with certificate verification. The four-device, settings, session, HTTP API and
HTTPS file scenarios passed, as did `go test ./...`.
The temporary upload hit `/tmp`'s per-user quota before installation; the partial
upload was removed and the verified package installed from `/var/tmp`. The working
binary was not changed until the complete package and backup had been verified.

## Next batches

- [ ] RFC 6120/6121: complete stream-state/error checks, IQ routing, presence priority/probes and roster transition tests.
- [ ] RFC 7622 (with RFC 9844): strict JID normalization and validation; audit existing identities for collisions before migration.
- [x] XEP-0198: acknowledgements and bounded resumable sessions; disconnect, replay, expiry and ownership tests (real-client verification remains above).
- [x] MAM query-form discovery and owner-scoped persistent archive preferences (XEP-0441).
- [x] XEP-0313/0059: TCP paging beyond 100 messages on two resources, live archive writes between pages and continuation after SM resume; owner-scoped cursors remain valid when query filters change.
- [ ] Full MAM interoperability: exact resource-specific JID filtering, insertion ordering for timestamp ties, a consistent snapshot within each page under concurrent writes, and the unadvertised `mam:2#extended` profile. Current bare-JID filtering and timestamp/stanza-ID ordering do not complete these requirements.
- [x] XEP-0359: account-assigned stable IDs, matching MAM cursors and live/offline/Carbons/web copies across C2S, web, OMEMO, bot and component entry points.
- [x] XEP-0280: shared sent/received copies for supported direct-message paths; expanded MUC eligibility remains pending.
- [x] XEP-0352: active/inactive state; suppress only repeated identical availability while inactive. Messages and subscription requests remain immediate.
- [ ] XEP-0363 and OMEMO: test key publication, multi-device trust, encrypted attachments and history with Conversations.
- [x] XEP-0045 local members-only profile: shared web/native rooms, membership, affiliations, invitations, bounded join history and tenant isolation. See [limits](xmpp.md); this is not complete XEP-0045 conformance.
- [ ] RFC 7395: authenticated XMPP WebSocket transport with framing, origin checks and limits.
- [x] Web Push: persistent subscriptions, VAPID, service worker, delivery retries, subscription removal and notification privacy. Browser/device delivery verification remains pending.
- [x] XEP-0357 publisher: persistent registrations, bounded queue, component and allowlisted TLS EXTERNAL/bidi routes, authenticated acknowledgements and retry/removal. See [native push](xmpp-push.md). Real ChatSecure background delivery is still blocked by vendor DNS/availability.
- [ ] RFC 6120/7712: S2S federation, DNS discovery, authenticated domain association, queues and abuse limits.
- [ ] RFC 5122, 5437, 7259, 8266, 8284: XMPP links, Sieve notifications, mail Jabber-ID, nicknames and LDAP integration.

Secret-iOS source inspected: `Secret-Messenger-App/Secret-iOS`, `ServerCapabilityInfo.swift` and `OTRXMPPManager.m`. Its capability screen checks XEP-0198/0357/0363/0352/0359/0313; the inspected implementation comments out standard push initialization and sends a custom `register-push-apns` command. Native notifications need the app operator's service or credentials for a separately controlled build.

## Acceptance

Advertise capabilities only after protocol handling and negative tests exist. Run race tests and real-client interop for reconnects, two simultaneous devices, background operation, pagination beyond 100 messages, attachments and account isolation. No complete compliance or production verification is implied by passing unit tests.

References: [RFC index](https://xmpp.org/rfcs/), [Stream Management](https://xmpp.org/extensions/xep-0198.html), [MAM](https://xmpp.org/extensions/xep-0313.html), [RSM](https://xmpp.org/extensions/xep-0059.html), [Push](https://xmpp.org/extensions/xep-0357.html).

## Web Push implementation (deployed; browser interop still pending)

The server now persists VAPID keys, per-user browser subscriptions and delivery jobs in SQL. Mail delivery, calendar reminders and incoming chat messages enqueue generic notifications; subjects, message bodies and OMEMO plaintext are never included in the push payload. A single worker retries temporary failures with backoff (five attempts maximum), removes expired subscriptions on HTTP 404/410 and cascades queued-job removal. Queue insertion is limited to 10,000 pending jobs and each account to 20 browser subscriptions. A server restart retains the queue and signing identity. Multi-worker claim coordination is not implemented; run one delivery worker per database.

The notification button enables/disables Web Push after browser permission. Logout removes that browser subscription. `/sw.js` handles background notifications and opens Mail, Calendar or Chat; the manifest allows standalone installation. HTTPS and a browser supporting Push API are required. On iOS, install the web app on the home screen first. This provides web-app notifications, not APNs notifications inside Secret-iOS.

Endpoints are restricted to Google FCM, Mozilla and Apple browser push services; redirects, arbitrary hosts, custom ports and invalid encryption keys are rejected. Edge/WNS and self-hosted push gateways are not currently supported. Outbound HTTPS to these vendors must be allowed. VAPID keys live in dedicated database tables and are not exposed by the settings API; include the database in protected backups.

Validation: subscription ownership, persistent keys, payload privacy, deduplication of pending event jobs, retry scheduling, expired endpoint removal, SSE unsubscribe races and worker click routing are covered by local tests. Real delivery with closed browser windows and iOS installation remains to be verified after deployment.

## Archive preferences

MAM preferences use [XEP-0441](https://xmpp.org/extensions/xep-0441.html), separated from XEP-0313. IQ get/set persists each authenticated owner’s default (`always`, `never`, `roster`) and always/never JID exceptions. Resource-specific exceptions match the original stanza addresses. The default remains `always`; changing preferences affects future archive writes only. The protocol is optional and XEP-0441 cautions that disabling archiving can interfere with offline synchronization. Tests cover owner isolation, invalid values, normalized JIDs and roster/resource matching.

## Multi-device message copies

Carbons now use a shared path for C2S, web plaintext, web OMEMO and bot outgoing messages. Direct full-JID delivery also sends received copies to other Carbons-enabled resources without duplicating the original on the selected resource. Forwarded message/body namespaces are explicit, and private/no-copy hints and already wrapped carbon messages suppress copying. Invalid enable/disable commands are rejected. Expanded `urn:xmpp:carbons:rules:0` is deliberately not advertised: full MUC eligibility and error-response tracking remain incomplete. Component replacement also closes the previous connection outside the hub lock to prevent recursive unregister deadlocks.

References: [Message Carbons](https://xmpp.org/extensions/xep-0280.html), [MAM Preferences](https://xmpp.org/extensions/xep-0441.html).

## IQ dispatch hardening (2026-10-15)

IQ request handlers now match the root element and its namespace for ping, roster, vCard, session, PEP and upload discovery. Namespace text inside an unrelated element cannot invoke these handlers. Result/error stanzas are ignored by server-command dispatch to prevent reply loops; unknown request types and missing IDs are rejected. Regression tests exercise valid ping, nested/spoofed namespaces and unsolicited replies. This is a bounded correctness update, not completion of peer IQ routing or the remaining RFC/XEP checklist above.

## Presence restoration (2026-10-15)

Initial availability now returns current presence for subscribed local contacts and the owner's other resources. Approving a subscription sends existing presence immediately, including native clients that were already online. Local probes are handled with owner-side roster authorization; a full-JID lookup never returns other resources. Original presence payloads (including capability extensions), IDs and status text are retained for the connection lifetime and replayed. Tests cover consent, revocation, resource filtering, initial state, escaping and extension preservation. Full presence priority routing, directed-presence tracking and federation remain pending; this does not assert complete RFC 6121 conformance.

## Stable message IDs (2026-10-17)

[XEP-0359](https://xmpp.org/extensions/xep-0359.html) is now advertised for the server/account domains and local bare account JIDs. Each accepted message gets a server-generated UUID; the local accounts assign that value under their own bare JIDs. Client `message/@id` and `origin-id` remain intact for receipts and client correlation. Repeated or absent client IDs no longer collide in the archive. Incoming direct-child `stanza-id` claims for the assigning accounts are removed before stamping, including when MAM preferences disable storage; third-party IDs are preserved.

Both local archive copies are committed in one transaction before delivery. The live stanza, offline copy, Carbons and web events share the assigned ID with MAM/history. This also applies to opaque OMEMO messages. Existing MAM cursor values remain unchanged; when old rows are read, the server adds the corresponding account `stanza-id` and explicit `jabber:client` namespace to the forwarded message. No history reset or database migration is needed. As of 0.9.18, local groups share history with the native MUC profile documented in [XMPP](xmpp.md).

Regression coverage: all five send entry points, duplicate/absent client IDs, spoofed IDs, disabled archiving, offline replay, database reopen/cursors, legacy MAM namespaces, self-message deduplication, discovery scope and transaction rollback on an archive failure. Native-client display of the capability is a separate device check.

## Push assessment before 0.9.17 (historical)

Before 0.9.17, native XEP-0357 was **not implemented or advertised**. The implementation and current limits are now documented in [native push](xmpp-push.md). The inspected [Secret-iOS revision](https://github.com/Secret-Messenger-App/Secret-iOS/tree/6f3083bb839b9ded22faa6feca30163be31df273) disables standard `XMPPPushModule` initialization and sends a custom `register-push-apns` command. Its capability check and its actual registration path are different. Advertising `urn:xmpp:push:0` alone would only change the checkmark.

The next server batch needs an authenticated route to the app's XMPP push service (federation or a configured component), owner-scoped persistent enable/disable registrations, bounded notification queues, publish acknowledgements/errors/retries and removal of rejected registrations. Notifications should omit message bodies and encryption keys. End-to-end acceptance requires background delivery on the target app. The app operator's APNs credentials are not needed by Tayga when a working vendor push gateway exists, but that gateway must support this client. The existing browser Web Push implementation cannot wake the native Secret app.

## ChatSecure follow-up, 2026-10-17

The later client reference is [ChatSecure](https://chatsecure.org/i/), not Secret-iOS. In [ChatSecure-iOS revision 38d6abba3e1c21156095ac3a1096d5e829df4b96](https://github.com/ChatSecure/ChatSecure-iOS/blob/38d6abba3e1c21156095ac3a1096d5e829df4b96/ChatSecureCore/Classes/Controllers/XMPP/OTRXMPPManager.m), `XMPPPushModule` is active and standard registration sends the vendor endpoint and token as publish options. The Secret-specific finding above must not be applied to ChatSecure.

DNS was checked both locally and on the mail server: `chatsecure.org` resolved, but `push.chatsecure.org` had no usable address (`getaddrinfo`: name or service not known). A public DNS query returned a CNAME to `push.chatsecure.org.herokudns.com` without an A address. The vendor push API could not be contacted. Recheck its availability before end-to-end tests; do not treat DNS failure as evidence that the XEP is implemented. At the time of this check, Tayga 0.9.16 did not advertise or implement XEP-0357. The persistent registrations, notification delivery route, retry handling and native-client delivery tests listed above remain work to do.

## Connection visibility, 0.9.16

Bound native resources update their activity in the shared session registry and identify their XMPP resource in the user/admin connection list. Administrative disconnection disables Stream Management resumption before closing the socket, so a disconnected retained stream cannot be resumed. Tests verify activity updates, cleanup and token invalidation. Native push was pending in that release; 0.9.17 adds the publisher described below.

## IQ routing and native push, 0.9.17

Full-JID IQ requests and replies now reach the addressed device; spoofed `from` attributes are replaced with the authenticated sender. Missing resources return service-unavailable for requests and never generate reply loops. A request must have exactly one payload. IQs to authenticated components are forwarded as well. Server-owned roster/session operations reject another account as their target. This enables device capability queries without misinterpreting them as server commands; presence-priority routing and the other RFC 6121 items remain pending.

The XEP-0357 publisher is implemented with conditional discovery and real protocol handling. External transport is restricted to configured push domains using verified TLS, EXTERNAL and bidi. Native ChatSecure delivery still depends on its vendor service. See [configuration, tests and limits](xmpp-push.md).

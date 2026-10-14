# XMPP compatibility implementation plan

Updated 2026-10-14. This is an implementation checklist, not a claim of complete RFC/XEP conformance.

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
- [ ] Real-client network-switch/background interop verification. Retained streams are in-memory and do not survive a server restart; offline messages remain database-backed.
- [ ] Existing-identity normalization collision audit before any stored identity migration.

## Next batches

- [ ] RFC 6120/6121: complete stream-state/error checks, IQ routing, presence priority/probes and roster transition tests.
- [ ] RFC 7622 (with RFC 9844): strict JID normalization and validation; audit existing identities for collisions before migration.
- [x] XEP-0198: acknowledgements and bounded resumable sessions; disconnect, replay, expiry and ownership tests (real-client verification remains above).
- [ ] XEP-0313/0059: MAM query discovery/preferences and full protocol interoperability, including concurrent archive writes.
- [ ] XEP-0359/0280: archive-assigned stable IDs and correct sent/received carbons across all entry points.
- [x] XEP-0352: active/inactive state; suppress only repeated identical availability while inactive. Messages and subscription requests remain immediate.
- [ ] XEP-0363 and OMEMO: test key publication, multi-device trust, encrypted attachments and history with Conversations.
- [ ] XEP-0045: interoperable MUC with membership, roles, invitations, history and domain policies.
- [ ] RFC 7395: authenticated XMPP WebSocket transport with framing, origin checks and limits.
- [ ] Web Push: persistent subscriptions, VAPID, service worker, delivery retries, subscription removal and notification privacy.
- [ ] XEP-0357: real push registration/delivery only when a reachable client push service exists; Web Push does not substitute for native-app APNs.
- [ ] RFC 6120/7712: S2S federation, DNS discovery, authenticated domain association, queues and abuse limits.
- [ ] RFC 5122, 5437, 7259, 8266, 8284: XMPP links, Sieve notifications, mail Jabber-ID, nicknames and LDAP integration.

Secret-iOS source inspected: `Secret-Messenger-App/Secret-iOS`, `ServerCapabilityInfo.swift` and `OTRXMPPManager.m`. Its capability screen checks XEP-0198/0357/0363/0352/0359/0313; the inspected implementation comments out standard push initialization and sends a custom `register-push-apns` command. Native notifications need the app operator's service or credentials for a separately controlled build.

## Acceptance

Advertise capabilities only after protocol handling and negative tests exist. Run race tests and real-client interop for reconnects, two simultaneous devices, background operation, pagination beyond 100 messages, attachments and account isolation. No complete compliance or production verification is implied by passing unit tests.

References: [RFC index](https://xmpp.org/rfcs/), [Stream Management](https://xmpp.org/extensions/xep-0198.html), [MAM](https://xmpp.org/extensions/xep-0313.html), [RSM](https://xmpp.org/extensions/xep-0059.html), [Push](https://xmpp.org/extensions/xep-0357.html).

## Web Push implementation (local, pending deployment and browser interop)

The server now persists VAPID keys, per-user browser subscriptions and delivery jobs in SQL. Mail delivery, calendar reminders and incoming chat messages enqueue generic notifications; subjects, message bodies and OMEMO plaintext are never included in the push payload. A single worker retries temporary failures with backoff (five attempts maximum), removes expired subscriptions on HTTP 404/410 and cascades queued-job removal. Queue insertion is limited to 10,000 pending jobs and each account to 20 browser subscriptions. A server restart retains the queue and signing identity. Multi-worker claim coordination is not implemented; run one delivery worker per database.

The notification button enables/disables Web Push after browser permission. Logout removes that browser subscription. `/sw.js` handles background notifications and opens Mail, Calendar or Chat; the manifest allows standalone installation. HTTPS and a browser supporting Push API are required. On iOS, install the web app on the home screen first. This provides web-app notifications, not APNs notifications inside Secret-iOS.

Endpoints are restricted to Google FCM, Mozilla and Apple browser push services; redirects, arbitrary hosts, custom ports and invalid encryption keys are rejected. Edge/WNS and self-hosted push gateways are not currently supported. Outbound HTTPS to these vendors must be allowed. VAPID keys live in dedicated database tables and are not exposed by the settings API; include the database in protected backups.

Validation: subscription ownership, persistent keys, payload privacy, deduplication of pending event jobs, retry scheduling, expired endpoint removal, SSE unsubscribe races and worker click routing are covered by local tests. Real delivery with closed browser windows and iOS installation remains to be verified after deployment.

## Archive preferences

MAM preferences use [XEP-0441](https://xmpp.org/extensions/xep-0441.html), separated from XEP-0313. IQ get/set persists each authenticated owner’s default (`always`, `never`, `roster`) and always/never JID exceptions. Resource-specific exceptions match the original stanza addresses. The default remains `always`; changing preferences affects future archive writes only. The protocol is optional and XEP-0441 cautions that disabling archiving can interfere with offline synchronization. Tests cover owner isolation, invalid values, normalized JIDs and roster/resource matching.

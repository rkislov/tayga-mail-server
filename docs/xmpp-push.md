# Native XMPP push (0.9.17)

Tayga implements the **publisher/server side** of [XEP-0357](https://xmpp.org/extensions/xep-0357.html): account-scoped enable/disable, publish options, persistent registrations and a bounded delivery queue. This is separate from the existing browser Web Push service. It does not supply APNs credentials or replace a mobile app's vendor gateway.

## Delivery routes

Two explicit routes are supported:

1. An authenticated XEP-0114 component with `push: true`. Run it locally or through a protected tunnel: the component listener itself does not provide TLS. The component must implement PubSub push publication and the application's device delivery.
2. An administrator-allowlisted external XMPP service using DNS SRV `_xmpp-server._tcp`, verified STARTTLS, SASL EXTERNAL and [XEP-0288 bidirectional S2S](https://xmpp.org/extensions/xep-0288.html). This is an outbound push-only transport, **not general XMPP federation**. There is no dialback or plaintext fallback. A service requiring other authentication or lacking bidi is rejected and logged.

```yaml
xmpp:
  enabled: true
  # Use the service domain supplied by the application's push registration.
  push_services:
    - push.chatsecure.org
    - eu.prod.push.monal-im.org
  # Alternative: a trusted component already configured on this server.
  # component_listen: '127.0.0.1:5347'
  # components:
  #   - name: mobile-push
  #     domain: push.example.com
  #     secret: REPLACE_WITH_A_RANDOM_SECRET
  #     push: true
```

Restart Tayga after changing XMPP settings. The admin configuration editor exposes these fields. The XMPP page lists configured gateways, registration count and pending notification count; these are configuration/queue indicators, not proof of delivery to a phone.

The local TLS certificate must cover the **account domain**, e.g. `kislovs.ru` for `roman@kislovs.ru`, as well as any transport hostname that clients use. The remote service must accept that certificate for EXTERNAL authentication. The remote certificate is checked against the original service domain, not an arbitrary DNS SRV target. Outbound TCP to the service's SRV port (5269 when there is no SRV record) must be allowed. Private, loopback, link-local and special-use destination addresses are refused and resolved IPs are pinned for the connection.

`urn:xmpp:push:0` is advertised on the authenticated owner's bare-JID discovery only when a configured component is connected, or an external route is configured and the certificate covers that account domain. A gateway outage is reported as a delivery failure; the server's protocol capability and the app vendor's availability are different things.

## Queue behavior and privacy

A user can register up to 20 distinct service/node pairs. Re-enabling replaces the options and invalidates old in-flight acknowledgements; disabling one node or all nodes of a service removes its queued work. Incoming direct chat, attachment and encrypted-message events enqueue notifications for disconnected or CSI-inactive registered resources. An active connected resource does not receive an extra push. Registration for a resource is updated by the client when it reconnects with a different resource name.

Notifications contain an empty `notification` element plus the vendor-supplied publish options. No message body, sender, attachment URL, unread count or OMEMO plaintext/key is sent. The options can contain vendor credentials and remain in the protected database; database backups are sensitive.

There is at most one coalesced job per registration and at most 10,000 pending jobs globally. A generation counter preserves messages arriving while an earlier notification awaits a reply. Successful replies dequeue the acknowledged generation. Non-temporary IQ errors remove the rejected registration; temporary errors/timeouts retry with exponential backoff, up to five attempts, then disable the registration. A disconnected configured component retains pending work until it reconnects. Run one push worker per database. A process restart retains registrations and jobs; it can repeat a notification whose remote acknowledgement was lost, so vendor delivery is at least once.

Logs use `xmpp push attempt`, `xmpp push delivery failed` and `xmpp push rejected` with request ID, service and attempt number. Publish tokens and message content are not logged. Successful publication confirms the gateway's acceptance, not the operating system's notification display.

## ChatSecure availability

On 2026-10-17, DNS lookups performed on `mail.kislovs.ru` resolved `chatsecure.org` but returned no usable address for `push.chatsecure.org` or its Heroku target. That prevents vendor registration/delivery independently of Tayga. Recheck DNS and the actual endpoint supplied by the installed app before interpreting a capability check as end-to-end success. ChatSecure's [standard push module](https://github.com/ChatSecure/ChatSecure-iOS/blob/38d6abba3e1c21156095ac3a1096d5e829df4b96/ChatSecureCore/Classes/Controllers/XMPP/OTRXMPPManager.m) is distinct from Secret-iOS's custom APNs command.

## Validation and remaining interoperability

### Monal (2026-10-18)

For manual connection settings, use `mail.kislovs.ru:5222` with STARTTLS
(Direct TLS disabled), or port `5223` with Direct TLS. Both transports have
been checked against the deployed server. The account JID remains
`user@kislovs.ru`. A blank capability screen before a successful login does
not establish that the server lacks every listed feature.

Monal 6.4.23 also crashed during `handleVersionResponse` when Tayga answered
the server's `jabber:iq:version` query with an unsupported-operation error
without `from`. Its client treated that response as a contact with a missing
resource. Tayga now implements XEP-0092 at the account domain and physical
server hostname, returning an explicit matching `from`, the authenticated
requester's full `to`, product name and build version. It does not report
Tayga's version on behalf of another account or a remote server. Full-JID
queries continue through normal client routing.

Monal's European production push service is `eu.prod.push.monal-im.org`.
Only add the service actually selected by the app to `xmpp.push_services`;
do not allow arbitrary remote domains. Without that route, Monal receives
`service-unavailable` when enabling push even if XEP-0357 is advertised for
another configured gateway.

Both European gateway nodes were checked from the mail server using TLS 1.3,
verified certificates, SASL EXTERNAL and a discovery IQ round trip. They
advertise bidi as `urn:xmpp:bidi` instead of XEP-0288's
`urn:xmpp:features:bidi`. Tayga accepts both spellings after verified TLS;
EXTERNAL authentication, the allowlist and response identity validation
remain required. These checks did not publish notifications and do not
prove delivery through APNs to a phone.

Monal also requires a broader PEP profile than the existing OMEMO node
support. SASL2/SCRAM/channel binding, roster versioning, blocking and external
STUN/TURN discovery remain separate compatibility work; they are not
advertised as implemented.

Automated tests cover registration ownership, malformed forms, replacement, cancellation, restart persistence, queue coalescing, retry exhaustion, privacy and forged acknowledgements. A test gateway completes STARTTLS with mutually verified certificates, bidi, EXTERNAL, stream restart and publication acknowledgement; negative cases reject untrusted certificates, wrong hostnames, missing TLS/bidi, failed authentication and spoofed reply identities.

Real background delivery in ChatSecure remains unverified while its vendor endpoint is unavailable. General S2S federation, dialback, native MUC, arbitrary gateway compatibility and a full XMPP conformance claim are outside this implementation. A browser Web Push notification cannot wake the native ChatSecure application.

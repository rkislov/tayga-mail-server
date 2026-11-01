# Tayga IMAP wire server

This is the `server` package from `github.com/emersion/go-imap` **v1.2.1**,
including its upstream protocol tests and MIT license. Protocol types, parser,
client, commands and responses still use the pinned upstream dependency.

Local changes address concurrent IDLE/SELECT delivery:

- Serialize command/backend context access with notification delivery per
  connection. IDLE keeps receiving updates while waiting for DONE.
- Fence queued notifications by selection generation; check user and mailbox
  at delivery, under the same lock as SELECT.
- Bound and order per-connection update queues. Disconnect a stalled client
  on overflow so it resynchronizes; other connections continue receiving.
- Cancel writes and notification waiters on close; run backend Logout once,
  on the command goroutine, after transport closure.
- Build consumable response channels separately for each recipient.
- Stop the dispatcher on server shutdown and initialize it once per server.

**Backend contract:** command methods enqueue updates without synchronously
waiting for `Update.Done()`. Tayga's Hub follows this contract. Waiting for
delivery while holding the command lock would deadlock. This internal copy is
not a replacement for arbitrary third-party go-imap backends or extensions.

When updating upstream, preserve these changes and run both `internal/imapwire`
and `internal/imap` tests with `-race`, including the live TCP IDLE fixtures.

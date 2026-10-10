package smtp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tayga/tms/internal/ha"
	"github.com/tayga/tms/internal/siem"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
)

// QueueConfig controls the outbound retry worker.
type QueueConfig struct {
	Enabled      bool
	Workers      int
	PollInterval time.Duration
	MaxAttempts  int
	BatchSize    int
}

func defaultQueueConfig(c QueueConfig) QueueConfig {
	if c.Workers <= 0 {
		c.Workers = 1
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 5 * time.Second
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 8
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 10
	}
	return c
}

// OutboundQueue accepts remote mail and retries delivery with DSN on final failure.
type OutboundQueue struct {
	store        storage.Driver
	sender       outboundSender
	dkim         *dkimSigner
	sieve        *sieve.Engine
	hostname     string
	cfg          QueueConfig
	log          *slog.Logger
	ha           ha.Gate
	fenceWriters bool
	writers      ha.WriterGate
	siem         *siem.Exporter
}

// SetHA restricts queue drain to the lease holder when fenceWriters is set.
func (q *OutboundQueue) SetHA(gate ha.Gate, fenceWriters bool) {
	if q == nil {
		return
	}
	q.ha = gate
	q.fenceWriters = fenceWriters
}

// SetWriters gates DSN local delivery under sticky/cluster writer policy.
func (q *OutboundQueue) SetWriters(w ha.WriterGate) {
	if q == nil {
		return
	}
	q.writers = w
}

// SetSIEM attaches CEF syslog export for queue mail-log events.
func (q *OutboundQueue) SetSIEM(e *siem.Exporter) {
	if q != nil {
		q.siem = e
	}
}

func newOutboundQueue(store storage.Driver, sender outboundSender, dkim *dkimSigner, eng *sieve.Engine, hostname string, cfg QueueConfig, log *slog.Logger) *OutboundQueue {
	if log == nil {
		log = slog.Default()
	}
	return &OutboundQueue{
		store: store, sender: sender, dkim: dkim, sieve: eng,
		hostname: hostname, cfg: defaultQueueConfig(cfg), log: log,
	}
}

func (q *OutboundQueue) Enqueue(ctx context.Context, from string, to []string, data []byte, msgid string) error {
	if q == nil || q.sender == nil {
		return fmt.Errorf("outbound queue not available")
	}
	for _, rcpt := range to {
		_, err := q.store.EnqueueOutbound(ctx, &storage.OutboundItem{
			EnvelopeFrom: from,
			EnvelopeTo:   rcpt,
			MessageID:    msgid,
			Data:         data,
			MaxAttempts:  q.cfg.MaxAttempts,
			NextAttempt:  time.Now().UTC(),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (q *OutboundQueue) Start(ctx context.Context) {
	if q == nil || !q.cfg.Enabled {
		return
	}
	for i := 0; i < q.cfg.Workers; i++ {
		go q.loop(ctx, i)
	}
	q.log.Info("smtp outbound queue started",
		"workers", q.cfg.Workers,
		"poll", q.cfg.PollInterval.String(),
		"max_attempts", q.cfg.MaxAttempts,
	)
}

func (q *OutboundQueue) loop(ctx context.Context, worker int) {
	t := time.NewTicker(q.cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			q.drain(ctx, worker)
		}
	}
}

func (q *OutboundQueue) drain(ctx context.Context, worker int) {
	if q.fenceWriters && q.ha != nil && !q.ha.IsLeader() {
		return
	}
	items, err := q.store.ClaimOutboundDue(ctx, q.cfg.BatchSize)
	if err != nil {
		q.log.Warn("outbound claim failed", "worker", worker, "err", err)
		return
	}
	for _, it := range items {
		q.process(ctx, it)
	}
}

func (q *OutboundQueue) process(ctx context.Context, it *storage.OutboundItem) {
	data := it.Data
	err := q.sender.Send(it.EnvelopeFrom, []string{it.EnvelopeTo}, data)
	if err == nil {
		if delErr := q.store.DeleteOutbound(ctx, it.ID); delErr != nil {
			q.log.Warn("outbound delete after success", "id", it.ID, "err", delErr)
		} else {
			q.log.Info("outbound delivered", "to", it.EnvelopeTo, "attempts", it.Attempts+1)
			q.writeMailLog(it, "sent", "")
		}
		return
	}

	attempts := it.Attempts + 1
	q.log.Warn("outbound attempt failed",
		"to", it.EnvelopeTo, "attempt", attempts, "max", it.MaxAttempts, "err", err)

	if attempts >= it.MaxAttempts {
		if bounceErr := q.bounceDSN(ctx, it, err); bounceErr != nil {
			q.log.Warn("dsn bounce failed", "to", it.EnvelopeFrom, "err", bounceErr)
		}
		_ = q.store.DeleteOutbound(ctx, it.ID)
		q.writeMailLog(it, "failed", truncateErr(err))
		return
	}

	next := time.Now().UTC().Add(backoff(attempts))
	detail := fmt.Sprintf("attempt %d/%d next %s: %s", attempts, it.MaxAttempts, next.Format(time.RFC3339), truncateErr(err))
	q.writeMailLog(it, "deferred", detail)
	if resErr := q.store.RescheduleOutbound(ctx, it.ID, attempts, next, truncateErr(err)); resErr != nil {
		q.log.Warn("outbound reschedule failed", "id", it.ID, "err", resErr)
	}
}

func (q *OutboundQueue) writeMailLog(it *storage.OutboundItem, event, detail string) {
	if q == nil || q.store == nil || it == nil {
		return
	}
	from := strings.ToLower(strings.TrimSpace(it.EnvelopeFrom))
	to := strings.ToLower(strings.TrimSpace(it.EnvelopeTo))
	domain := storage.DomainOfEmail(from)
	tenantID := ""
	if domain != "" {
		if d, err := q.store.GetDomainByName(context.Background(), domain); err == nil && d != nil {
			tenantID = d.TenantID
		}
	}
	e := &storage.MailLogEntry{
		TenantID:  tenantID,
		Domain:    domain,
		Event:     event,
		Direction: "outbound",
		MailFrom:  from,
		RcptTo:    to,
		MessageID: it.MessageID,
		Size:      int64(len(it.Data)),
		Detail:    detail,
	}
	if q.siem != nil {
		q.siem.EmitMail(e.Event, e.Direction, e.Peer, e.MailFrom, e.RcptTo, e.MessageID, e.Detail, e.Size)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := q.store.InsertMailLog(ctx, e); err != nil {
		q.log.Warn("mail log write failed", "err", err)
	}
}

func backoff(attempt int) time.Duration {
	// attempt is 1-based after failure
	schedule := []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		30 * time.Minute,
		1 * time.Hour,
		2 * time.Hour,
		4 * time.Hour,
		8 * time.Hour,
	}
	if attempt <= 0 {
		return schedule[0]
	}
	if attempt > len(schedule) {
		return schedule[len(schedule)-1]
	}
	return schedule[attempt-1]
}

func truncateErr(err error) string {
	s := err.Error()
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

func (q *OutboundQueue) bounceDSN(ctx context.Context, it *storage.OutboundItem, fail error) error {
	from := normalizeAddr(it.EnvelopeFrom)
	if from == "" || strings.EqualFold(from, "<>") {
		return nil
	}
	u, err := q.store.ResolveRecipient(ctx, from)
	if err != nil {
		// External envelope-from: no local mailbox to bounce into.
		q.log.Info("dsn skipped; sender not local", "from", from, "failed_to", it.EnvelopeTo)
		return nil
	}
	if q.writers != nil {
		if err := q.writers.AllowWrite(ctx, u.ID); err != nil {
			return err
		}
	}
	dsn := buildDSN(q.hostname, it, fail)
	msgid := extractMessageID(dsn)
	if q.sieve != nil {
		return q.sieve.Deliver(ctx, u, "mailer-daemon@"+q.hostname, from, dsn, msgid)
	}
	return fmt.Errorf("no local delivery engine for DSN")
}

func buildDSN(hostname string, it *storage.OutboundItem, fail error) []byte {
	if hostname == "" {
		hostname = "localhost"
	}
	boundary := "tayga_dsn_" + it.ID
	origHdr := headerOnly(it.Data, 8<<10)
	diag := truncateErr(fail)
	var b strings.Builder
	fmt.Fprintf(&b, "From: Mail Delivery Subsystem <mailer-daemon@%s>\r\n", hostname)
	fmt.Fprintf(&b, "To: <%s>\r\n", it.EnvelopeFrom)
	fmt.Fprintf(&b, "Subject: Undelivered Mail Returned to Sender\r\n")
	fmt.Fprintf(&b, "Message-ID: <dsn.%s@%s>\r\n", it.ID, hostname)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/report; report-type=delivery-status; boundary=\"%s\"\r\n", boundary)
	fmt.Fprintf(&b, "Auto-Submitted: auto-replied\r\n")
	fmt.Fprintf(&b, "\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=utf-8\r\n\r\n")
	fmt.Fprintf(&b, "This is the mail system at host %s.\r\n\r\n", hostname)
	fmt.Fprintf(&b, "I'm sorry to have to inform you that your message could not\r\n")
	fmt.Fprintf(&b, "be delivered to one or more recipients.\r\n\r\n")
	fmt.Fprintf(&b, "<%s>: %s\r\n\r\n", it.EnvelopeTo, diag)
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: message/delivery-status\r\n\r\n")
	fmt.Fprintf(&b, "Reporting-MTA: dns; %s\r\n", hostname)
	fmt.Fprintf(&b, "Arrival-Date: %s\r\n\r\n", it.CreatedAt.UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Final-Recipient: rfc822; %s\r\n", it.EnvelopeTo)
	fmt.Fprintf(&b, "Action: failed\r\n")
	fmt.Fprintf(&b, "Status: 5.0.0\r\n")
	fmt.Fprintf(&b, "Diagnostic-Code: smtp; %s\r\n\r\n", diag)
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: message/rfc822\r\n\r\n")
	b.WriteString(origHdr)
	if !strings.HasSuffix(origHdr, "\r\n") {
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}

func headerOnly(data []byte, max int) string {
	s := string(data)
	if len(s) > max {
		s = s[:max]
	}
	// Prefer cut at end of headers.
	if i := strings.Index(s, "\r\n\r\n"); i >= 0 {
		return s[:i+4]
	}
	if i := strings.Index(s, "\n\n"); i >= 0 {
		return s[:i+2]
	}
	return s
}

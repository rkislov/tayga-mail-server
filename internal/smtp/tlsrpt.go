package smtp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

type tlsEvent struct {
	PolicyDomain string
	MXHost       string
	ResultType   string
	SendingMTA   string
	Count        int
}

type tlsReporter struct {
	queue    *OutboundQueue
	sender   outboundSender
	orgName  string
	contact  string
	hostname string
	interval time.Duration
	log      *slog.Logger
	lookupTXT func(name string) ([]string, error)

	mu  sync.Mutex
	buf []*tlsEvent
}

func newTLSReporter(org, contact, hostname string, interval time.Duration, log *slog.Logger) *tlsReporter {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if org == "" {
		org = "Tayga Mail"
	}
	if contact == "" {
		contact = "tlsrpt-noreply@" + hostname
	}
	return &tlsReporter{
		orgName: org, contact: contact, hostname: hostname,
		interval: interval, log: log, lookupTXT: net.LookupTXT,
	}
}

func (r *tlsReporter) SetOutbound(q *OutboundQueue, sender outboundSender) {
	if r == nil {
		return
	}
	r.queue = q
	r.sender = sender
}

func (r *tlsReporter) Record(ev *tlsEvent) {
	if r == nil || ev == nil || ev.PolicyDomain == "" {
		return
	}
	if ev.Count <= 0 {
		ev.Count = 1
	}
	r.mu.Lock()
	// Coalesce identical keys
	for _, e := range r.buf {
		if e.PolicyDomain == ev.PolicyDomain && e.MXHost == ev.MXHost && e.ResultType == ev.ResultType {
			e.Count += ev.Count
			r.mu.Unlock()
			return
		}
	}
	cp := *ev
	r.buf = append(r.buf, &cp)
	r.mu.Unlock()
}

func (r *tlsReporter) Start(ctx context.Context) {
	if r == nil {
		return
	}
	go func() {
		tick := time.NewTicker(r.interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				_ = r.Flush(context.Background())
				return
			case <-tick.C:
				if err := r.Flush(ctx); err != nil && r.log != nil {
					r.log.Warn("tlsrpt flush", "err", err)
				}
			}
		}
	}()
	if r.log != nil {
		r.log.Info("tls-rpt reporter enabled", "interval", r.interval.String(), "contact", r.contact)
	}
}

func (r *tlsReporter) Flush(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	batch := r.buf
	r.buf = nil
	r.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	byDomain := map[string][]*tlsEvent{}
	for _, ev := range batch {
		byDomain[ev.PolicyDomain] = append(byDomain[ev.PolicyDomain], ev)
	}
	end := time.Now().UTC()
	begin := end.Add(-r.interval)
	for domain, events := range byDomain {
		rua, err := r.lookupRUA(domain)
		if err != nil {
			if r.log != nil {
				r.log.Warn("tlsrpt rua lookup", "domain", domain, "err", err)
			}
			continue
		}
		if len(rua) == 0 {
			continue
		}
		body, err := buildTLSReportJSON(r.orgName, r.contact, r.hostname, domain, begin, end, events)
		if err != nil {
			return err
		}
		msg, err := buildTLSReportMessage(r.contact, rua, domain, body)
		if err != nil {
			return err
		}
		for _, to := range rua {
			if err := r.deliver(ctx, r.contact, to, msg); err != nil {
				return fmt.Errorf("tlsrpt to %s: %w", to, err)
			}
		}
		if r.log != nil {
			r.log.Info("tlsrpt sent", "domain", domain, "recipients", len(rua), "events", len(events))
		}
	}
	return nil
}

func (r *tlsReporter) deliver(ctx context.Context, from, to string, data []byte) error {
	if r.queue != nil {
		return r.queue.Enqueue(ctx, from, []string{to}, data, "")
	}
	if r.sender != nil {
		return r.sender.Send(from, []string{to}, data)
	}
	return fmt.Errorf("no outbound path for tlsrpt")
}

func (r *tlsReporter) lookupRUA(domain string) ([]string, error) {
	lookup := r.lookupTXT
	if lookup == nil {
		lookup = net.LookupTXT
	}
	txts, err := lookup("_smtp._tls." + domain)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return nil, nil
		}
		s := strings.ToLower(err.Error())
		if strings.Contains(s, "no such host") || strings.Contains(s, "not found") {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	seen := map[string]struct{}{}
	for _, txt := range txts {
		txt = strings.TrimSpace(txt)
		if !strings.HasPrefix(strings.ToLower(txt), "v=tlsrptv1") {
			continue
		}
		for _, part := range strings.Split(txt, ";") {
			part = strings.TrimSpace(part)
			if len(part) < 5 || !strings.EqualFold(part[:3], "rua") || part[3] != '=' {
				continue
			}
			for _, uri := range strings.Split(part[4:], ",") {
				addr := mailtoAddress(strings.TrimSpace(uri))
				if addr == "" {
					continue
				}
				if _, ok := seen[addr]; ok {
					continue
				}
				seen[addr] = struct{}{}
				out = append(out, addr)
			}
		}
	}
	return out, nil
}

type tlsReportJSON struct {
	OrganizationName string             `json:"organization-name"`
	DateRange        tlsDateRange       `json:"date-range"`
	ContactInfo      string             `json:"contact-info"`
	ReportID         string             `json:"report-id"`
	Policies         []tlsPolicyJSON    `json:"policies"`
}

type tlsDateRange struct {
	StartDatetime string `json:"start-datetime"`
	EndDatetime   string `json:"end-datetime"`
}

type tlsPolicyJSON struct {
	Policy         tlsPolicyDetail `json:"policy"`
	Summary        tlsSummary      `json:"summary"`
	FailureDetails []tlsFailure    `json:"failure-details,omitempty"`
}

type tlsPolicyDetail struct {
	PolicyType   string   `json:"policy-type"`
	PolicyDomain string   `json:"policy-domain"`
	PolicyString []string `json:"policy-string,omitempty"`
}

type tlsSummary struct {
	TotalSuccessfulSessionCount int `json:"total-successful-session-count"`
	TotalFailureSessionCount    int `json:"total-failure-session-count"`
}

type tlsFailure struct {
	ResultType            string `json:"result-type"`
	SendingMTAIP          string `json:"sending-mta-ip,omitempty"`
	ReceivingMXHostname   string `json:"receiving-mx-hostname,omitempty"`
	FailedSessionCount    int    `json:"failed-session-count"`
}

func buildTLSReportJSON(org, contact, hostname, domain string, begin, end time.Time, events []*tlsEvent) ([]byte, error) {
	success, failure := 0, 0
	var fails []tlsFailure
	for _, ev := range events {
		if ev.ResultType == "successful-session" {
			success += ev.Count
			continue
		}
		failure += ev.Count
		fails = append(fails, tlsFailure{
			ResultType:          ev.ResultType,
			ReceivingMXHostname: ev.MXHost,
			FailedSessionCount:  ev.Count,
		})
	}
	rep := tlsReportJSON{
		OrganizationName: org,
		DateRange: tlsDateRange{
			StartDatetime: begin.UTC().Format(time.RFC3339),
			EndDatetime:   end.UTC().Format(time.RFC3339),
		},
		ContactInfo: contact,
		ReportID:    fmt.Sprintf("%s:%s:%d", hostname, domain, begin.Unix()),
		Policies: []tlsPolicyJSON{{
			Policy: tlsPolicyDetail{
				PolicyType:   "sts",
				PolicyDomain: domain,
			},
			Summary: tlsSummary{
				TotalSuccessfulSessionCount: success,
				TotalFailureSessionCount:    failure,
			},
			FailureDetails: fails,
		}},
	}
	return json.MarshalIndent(rep, "", "  ")
}

func buildTLSReportMessage(from string, to []string, domain string, jsonBody []byte) ([]byte, error) {
	boundary := "tayga_tlsrpt_" + fmt.Sprintf("%d", time.Now().UnixNano())
	var out strings.Builder
	fmt.Fprintf(&out, "From: %s\r\n", from)
	fmt.Fprintf(&out, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&out, "Subject: Report Domain: %s Submitter: %s Report-ID: %s\r\n", domain, from, time.Now().UTC().Format("2006-01-02"))
	fmt.Fprintf(&out, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&out, "Content-Type: multipart/report; report-type=tlsrpt; boundary=\"%s\"\r\n", boundary)
	fmt.Fprintf(&out, "\r\n")

	fmt.Fprintf(&out, "--%s\r\n", boundary)
	fmt.Fprintf(&out, "Content-Type: text/plain; charset=utf-8\r\n\r\n")
	fmt.Fprintf(&out, "This is an aggregate TLS report from %s covering %s.\r\n\r\n", from, domain)

	fmt.Fprintf(&out, "--%s\r\n", boundary)
	fmt.Fprintf(&out, "Content-Type: application/tlsrpt+json\r\n")
	fmt.Fprintf(&out, "Content-Disposition: attachment; filename=\"tlsrpt.json\"\r\n\r\n")
	out.Write(jsonBody)
	fmt.Fprintf(&out, "\r\n--%s--\r\n", boundary)
	return []byte(out.String()), nil
}

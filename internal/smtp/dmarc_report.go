package smtp

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tayga/tms/internal/storage"
)

type dmarcReporter struct {
	store     storage.Driver
	queue     *OutboundQueue
	sender    outboundSender
	orgName   string
	contact   string
	hostname  string
	interval  time.Duration
	aggregate bool // rua buffering + periodic send
	failure   bool // per-message ruf
	log       *slog.Logger

	mu  sync.Mutex
	buf []*dmarcEvent
}

func newDMARCReporter(store storage.Driver, org, contact, hostname string, interval time.Duration, aggregate, failure bool, log *slog.Logger) *dmarcReporter {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if org == "" {
		org = "Tayga Mail"
	}
	if contact == "" {
		contact = "dmarc-noreply@" + hostname
	}
	return &dmarcReporter{
		store: store, orgName: org, contact: contact, hostname: hostname,
		interval: interval, aggregate: aggregate, failure: failure, log: log,
	}
}

func (r *dmarcReporter) SetOutbound(q *OutboundQueue, sender outboundSender) {
	if r == nil {
		return
	}
	r.queue = q
	r.sender = sender
}

func (r *dmarcReporter) Record(ev *dmarcEvent) {
	if r == nil || !r.aggregate || ev == nil || ev.Domain == "" {
		return
	}
	r.mu.Lock()
	r.buf = append(r.buf, ev)
	r.mu.Unlock()
}

func (r *dmarcReporter) Start(ctx context.Context) {
	if r == nil || !r.aggregate {
		if r != nil && r.failure && r.log != nil {
			r.log.Info("dmarc ruf reporter enabled", "contact", r.contact)
		}
		return
	}
	go func() {
		flushTick := time.NewTicker(30 * time.Second)
		reportTick := time.NewTicker(r.interval)
		defer flushTick.Stop()
		defer reportTick.Stop()
		for {
			select {
			case <-ctx.Done():
				r.flushBuffer(context.Background())
				return
			case <-flushTick.C:
				r.flushBuffer(ctx)
			case <-reportTick.C:
				r.flushBuffer(ctx)
				if err := r.SendDue(ctx); err != nil && r.log != nil {
					r.log.Warn("dmarc report send", "err", err)
				}
			}
		}
	}()
	if r.log != nil {
		r.log.Info("dmarc rua reporter enabled", "interval", r.interval.String(), "contact", r.contact)
		if r.failure {
			r.log.Info("dmarc ruf reporter enabled", "contact", r.contact)
		}
	}
}

func (r *dmarcReporter) flushBuffer(ctx context.Context) {
	r.mu.Lock()
	batch := r.buf
	r.buf = nil
	r.mu.Unlock()
	if len(batch) == 0 {
		return
	}
	day := time.Now().UTC().Format("2006-01-02")
	for _, ev := range batch {
		row := &storage.DMARCAggRow{
			Domain:         strings.ToLower(ev.Domain),
			Day:            day,
			SourceIP:       ev.SourceIP,
			EnvelopeDomain: strings.ToLower(ev.EnvelopeDomain),
			HeaderFrom:     strings.ToLower(ev.HeaderFrom),
			SPFResult:      ev.SPFResult,
			DKIMResult:     ev.DKIMResult,
			Disposition:    ev.Disposition,
			Policy:         ev.Policy,
			RUA:            strings.Join(ev.RUA, ","),
			Count:          1,
		}
		if err := r.store.UpsertDMARCAgg(ctx, row); err != nil && r.log != nil {
			r.log.Warn("dmarc agg upsert", "err", err)
		}
	}
}

// SendDue builds and emails aggregate reports for completed UTC days.
func (r *dmarcReporter) SendDue(ctx context.Context) error {
	if r == nil || r.store == nil {
		return nil
	}
	today := time.Now().UTC().Format("2006-01-02")
	days, err := r.store.ListDMARCAggDaysBefore(ctx, today)
	if err != nil {
		return err
	}
	for _, day := range days {
		if err := r.sendDay(ctx, day); err != nil {
			if r.log != nil {
				r.log.Warn("dmarc report day", "day", day, "err", err)
			}
			continue
		}
		_ = r.store.DeleteDMARCAggByDay(ctx, day)
	}
	return nil
}

func (r *dmarcReporter) sendDay(ctx context.Context, day string) error {
	rows, err := r.store.ListDMARCAggByDay(ctx, day)
	if err != nil {
		return err
	}
	byDomain := map[string][]*storage.DMARCAggRow{}
	for _, row := range rows {
		byDomain[row.Domain] = append(byDomain[row.Domain], row)
	}
	begin, end, err := dayBoundsUnix(day)
	if err != nil {
		return err
	}
	for domain, list := range byDomain {
		rua := collectRUA(list)
		if len(rua) == 0 {
			continue
		}
		xmlBody, err := buildAggregateXML(r.orgName, r.contact, r.hostname, domain, begin, end, list)
		if err != nil {
			return err
		}
		gz, err := gzipBytes(xmlBody)
		if err != nil {
			return err
		}
		filename := fmt.Sprintf("%s!%s!%d!%d.xml.gz", r.hostname, domain, begin, end)
		msg, err := buildReportMessage(r.contact, rua, domain, day, filename, gz)
		if err != nil {
			return err
		}
		for _, to := range rua {
			if err := r.deliverReport(ctx, r.contact, to, msg); err != nil {
				return fmt.Errorf("send to %s: %w", to, err)
			}
		}
		if r.log != nil {
			r.log.Info("dmarc rua sent", "domain", domain, "day", day, "recipients", len(rua), "rows", len(list))
		}
	}
	return nil
}

func (r *dmarcReporter) deliverReport(ctx context.Context, from, to string, data []byte) error {
	if r.queue != nil {
		return r.queue.Enqueue(ctx, from, []string{to}, data, "")
	}
	if r.sender != nil {
		return r.sender.Send(from, []string{to}, data)
	}
	return fmt.Errorf("no outbound path for dmarc reports (enable smtp.queue/relay/outbound_direct)")
}

func collectRUA(rows []*storage.DMARCAggRow) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, row := range rows {
		for _, part := range strings.Split(row.RUA, ",") {
			addr := mailtoAddress(part)
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
	return out
}

func mailtoAddress(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	if strings.Contains(uri, ":") {
		u, err := url.Parse(uri)
		if err != nil || !strings.EqualFold(u.Scheme, "mailto") {
			return ""
		}
		uri = u.Opaque
		if uri == "" {
			uri = strings.TrimPrefix(u.Path, "/")
		}
	}
	// Strip !size / !max optional extensions: user@dom!50m
	if i := strings.IndexByte(uri, '!'); i >= 0 {
		uri = uri[:i]
	}
	return strings.ToLower(strings.TrimSpace(uri))
}

func dayBoundsUnix(day string) (begin, end int64, err error) {
	t, err := time.ParseInLocation("2006-01-02", day, time.UTC)
	if err != nil {
		return 0, 0, err
	}
	begin = t.Unix()
	end = t.Add(24*time.Hour).Unix() - 1
	return begin, end, nil
}

func gzipBytes(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(in); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type feedbackXML struct {
	XMLName         xml.Name           `xml:"feedback"`
	ReportMetadata  reportMetadataXML  `xml:"report_metadata"`
	PolicyPublished policyPublishedXML `xml:"policy_published"`
	Records         []recordXML        `xml:"record"`
}

type reportMetadataXML struct {
	OrgName   string       `xml:"org_name"`
	Email     string       `xml:"email"`
	ReportID  string       `xml:"report_id"`
	DateRange dateRangeXML `xml:"date_range"`
}

type dateRangeXML struct {
	Begin int64 `xml:"begin"`
	End   int64 `xml:"end"`
}

type policyPublishedXML struct {
	Domain string `xml:"domain"`
	ADKIM  string `xml:"adkim"`
	ASPF   string `xml:"aspf"`
	P      string `xml:"p"`
	SP     string `xml:"sp"`
	Pct    int    `xml:"pct"`
}

type recordXML struct {
	Row         rowXML         `xml:"row"`
	Identifiers identifiersXML `xml:"identifiers"`
	AuthResults authResultsXML `xml:"auth_results"`
}

type rowXML struct {
	SourceIP        string             `xml:"source_ip"`
	Count           int                `xml:"count"`
	PolicyEvaluated policyEvaluatedXML `xml:"policy_evaluated"`
}

type policyEvaluatedXML struct {
	Disposition string `xml:"disposition"`
	DKIM        string `xml:"dkim"`
	SPF         string `xml:"spf"`
}

type identifiersXML struct {
	HeaderFrom string `xml:"header_from"`
}

type authResultsXML struct {
	DKIM *dkimAuthXML `xml:"dkim,omitempty"`
	SPF  *spfAuthXML  `xml:"spf,omitempty"`
}

type dkimAuthXML struct {
	Domain string `xml:"domain"`
	Result string `xml:"result"`
}

type spfAuthXML struct {
	Domain string `xml:"domain"`
	Result string `xml:"result"`
}

func buildAggregateXML(org, contact, hostname, domain string, begin, end int64, rows []*storage.DMARCAggRow) ([]byte, error) {
	policy := "none"
	if len(rows) > 0 && rows[0].Policy != "" {
		policy = rows[0].Policy
	}
	fb := feedbackXML{
		ReportMetadata: reportMetadataXML{
			OrgName:   org,
			Email:     contact,
			ReportID:  fmt.Sprintf("%s:%s:%d-%d", hostname, domain, begin, end),
			DateRange: dateRangeXML{Begin: begin, End: end},
		},
		PolicyPublished: policyPublishedXML{
			Domain: domain, ADKIM: "r", ASPF: "r", P: policy, SP: policy, Pct: 100,
		},
	}
	for _, row := range rows {
		rec := recordXML{
			Row: rowXML{
				SourceIP: row.SourceIP,
				Count:    row.Count,
				PolicyEvaluated: policyEvaluatedXML{
					Disposition: row.Disposition,
					DKIM:        row.DKIMResult,
					SPF:         row.SPFResult,
				},
			},
			Identifiers: identifiersXML{HeaderFrom: row.HeaderFrom},
		}
		if row.DKIMResult != "" && row.DKIMResult != "none" {
			rec.AuthResults.DKIM = &dkimAuthXML{Domain: row.HeaderFrom, Result: row.DKIMResult}
		}
		if row.SPFResult != "" && row.SPFResult != "none" {
			dom := row.EnvelopeDomain
			if dom == "" {
				dom = row.HeaderFrom
			}
			rec.AuthResults.SPF = &spfAuthXML{Domain: dom, Result: row.SPFResult}
		}
		fb.Records = append(fb.Records, rec)
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(fb); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildReportMessage(from string, to []string, domain, day, filename string, gz []byte) ([]byte, error) {
	boundary := "tayga_dmarc_" + fmt.Sprintf("%d", time.Now().UnixNano())
	var out bytes.Buffer
	fmt.Fprintf(&out, "From: %s\r\n", from)
	fmt.Fprintf(&out, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&out, "Subject: Report Domain: %s Submitter: %s Report-ID: %s\r\n", domain, from, day)
	fmt.Fprintf(&out, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&out, "Content-Type: multipart/report; report-type=feedback-report; boundary=\"%s\"\r\n", boundary)
	fmt.Fprintf(&out, "\r\n")

	fmt.Fprintf(&out, "--%s\r\n", boundary)
	fmt.Fprintf(&out, "Content-Type: text/plain; charset=utf-8\r\n\r\n")
	fmt.Fprintf(&out, "This is a DMARC aggregate report for %s covering %s (UTC).\r\n\r\n", domain, day)

	fmt.Fprintf(&out, "--%s\r\n", boundary)
	fmt.Fprintf(&out, "Content-Type: message/feedback-report\r\n\r\n")
	fmt.Fprintf(&out, "Feedback-Type: auth-failure\r\n")
	fmt.Fprintf(&out, "User-Agent: Tayga-Mail\r\n")
	fmt.Fprintf(&out, "Version: 1\r\n\r\n")

	fmt.Fprintf(&out, "--%s\r\n", boundary)
	fmt.Fprintf(&out, "Content-Type: application/gzip\r\n")
	fmt.Fprintf(&out, "Content-Disposition: attachment; filename=\"%s\"\r\n", filename)
	fmt.Fprintf(&out, "Content-Transfer-Encoding: base64\r\n\r\n")
	enc := base64.NewEncoder(base64.StdEncoding, &out)
	if _, err := enc.Write(gz); err != nil {
		return nil, err
	}
	_ = enc.Close()
	fmt.Fprintf(&out, "\r\n--%s--\r\n", boundary)
	return out.Bytes(), nil
}

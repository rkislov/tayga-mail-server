package smtp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dmarc"
)

// shouldSendFailureReport applies RFC 7489 fo= against mechanism outcomes.
func shouldSendFailureReport(fo dmarc.FailureOptions, spfAligned, dkimAligned bool) bool {
	if fo == 0 {
		fo = dmarc.FailureAll // default "0"
	}
	if fo&dmarc.FailureAll != 0 && !spfAligned && !dkimAligned {
		return true
	}
	if fo&dmarc.FailureAny != 0 && (!spfAligned || !dkimAligned) {
		return true
	}
	if fo&dmarc.FailureSPF != 0 && !spfAligned {
		return true
	}
	if fo&dmarc.FailureDKIM != 0 && !dkimAligned {
		return true
	}
	return false
}

func collectRUF(uris []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, uri := range uris {
		addr := mailtoAddress(uri)
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}
	return out
}

// SendFailure emails an RFC 6591 AFRF failure report to published ruf= addresses.
func (r *dmarcReporter) SendFailure(ctx context.Context, ev *dmarcEvent, ruf []string, authResults string, original []byte) {
	if r == nil || !r.failure || ev == nil || ev.Domain == "" {
		return
	}
	addrs := collectRUF(ruf)
	if len(addrs) == 0 {
		return
	}
	msg, err := buildFailureMessage(r.contact, addrs, ev, authResults, original)
	if err != nil {
		if r.log != nil {
			r.log.Warn("dmarc ruf build", "domain", ev.Domain, "err", err)
		}
		return
	}
	for _, to := range addrs {
		if err := r.deliverReport(ctx, r.contact, to, msg); err != nil {
			if r.log != nil {
				r.log.Warn("dmarc ruf send", "domain", ev.Domain, "to", to, "err", err)
			}
		}
	}
	if r.log != nil {
		r.log.Info("dmarc ruf sent", "domain", ev.Domain, "recipients", len(addrs))
	}
}

func buildFailureMessage(from string, to []string, ev *dmarcEvent, authResults string, original []byte) ([]byte, error) {
	boundary := "tayga_dmarc_fail_" + fmt.Sprintf("%d", time.Now().UnixNano())
	var out strings.Builder
	fmt.Fprintf(&out, "From: %s\r\n", from)
	fmt.Fprintf(&out, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&out, "Subject: DMARC Failure Report for %s\r\n", ev.Domain)
	fmt.Fprintf(&out, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&out, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&out, "Content-Type: multipart/report; report-type=feedback-report; boundary=\"%s\"\r\n", boundary)
	fmt.Fprintf(&out, "\r\n")

	fmt.Fprintf(&out, "--%s\r\n", boundary)
	fmt.Fprintf(&out, "Content-Type: text/plain; charset=utf-8\r\n\r\n")
	fmt.Fprintf(&out, "This is an authentication failure report for an email message received from IP %s on %s.\r\n",
		ev.SourceIP, time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&out, "DMARC disposition: %s; SPF=%s DKIM=%s.\r\n\r\n", ev.Disposition, ev.SPFResult, ev.DKIMResult)

	fmt.Fprintf(&out, "--%s\r\n", boundary)
	fmt.Fprintf(&out, "Content-Type: message/feedback-report\r\n\r\n")
	fmt.Fprintf(&out, "Feedback-Type: auth-failure\r\n")
	fmt.Fprintf(&out, "User-Agent: Tayga-Mail\r\n")
	fmt.Fprintf(&out, "Version: 1\r\n")
	fmt.Fprintf(&out, "Auth-Failure: dmarc\r\n")
	if authResults != "" {
		fmt.Fprintf(&out, "Authentication-Results: %s\r\n", authResults)
	}
	mailFrom := ev.EnvelopeFrom
	if mailFrom == "" {
		mailFrom = ev.EnvelopeDomain
	}
	if mailFrom != "" {
		fmt.Fprintf(&out, "Original-Mail-From: <%s>\r\n", mailFrom)
	}
	fmt.Fprintf(&out, "Source-IP: %s\r\n", ev.SourceIP)
	fmt.Fprintf(&out, "Reported-Domain: %s\r\n", ev.Domain)
	fmt.Fprintf(&out, "Arrival-Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&out, "\r\n")

	fmt.Fprintf(&out, "--%s\r\n", boundary)
	fmt.Fprintf(&out, "Content-Type: text/rfc822-headers\r\n")
	fmt.Fprintf(&out, "Content-Transfer-Encoding: 8bit\r\n\r\n")
	out.WriteString(string(extractHeaders(original)))
	fmt.Fprintf(&out, "\r\n--%s--\r\n", boundary)
	return []byte(out.String()), nil
}

func extractHeaders(data []byte) []byte {
	const max = 64 << 10 // 64 KiB
	if len(data) > max {
		data = data[:max]
	}
	for i := 0; i+1 < len(data); i++ {
		if data[i] == '\n' && (data[i+1] == '\n' || (data[i+1] == '\r' && i+2 < len(data) && data[i+2] == '\n')) {
			return data[:i+1]
		}
	}
	return data
}

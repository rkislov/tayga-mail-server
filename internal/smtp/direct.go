package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"sort"
	"strings"
	"time"
)

type directSender struct {
	timeout  time.Duration
	hostname string
	sts      *stsResolver
	tlsrpt   *tlsReporter
	log      *slog.Logger
}

func newDirectSender(timeout time.Duration, hostname string, sts *stsResolver, tlsrpt *tlsReporter, log *slog.Logger) *directSender {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if hostname == "" {
		hostname = "localhost"
	}
	return &directSender{timeout: timeout, hostname: hostname, sts: sts, tlsrpt: tlsrpt, log: log}
}

func (d *directSender) Send(from string, to []string, data []byte) error {
	byDomain := map[string][]string{}
	for _, rcpt := range to {
		dom := domainOf(rcpt)
		if dom == "" {
			return fmt.Errorf("invalid recipient %q", rcpt)
		}
		byDomain[dom] = append(byDomain[dom], rcpt)
	}
	for dom, rcpts := range byDomain {
		if err := d.sendDomain(from, dom, rcpts, data); err != nil {
			return err
		}
	}
	return nil
}

func (d *directSender) sendDomain(from, domain string, rcpts []string, data []byte) error {
	hosts, err := mxHosts(domain)
	if err != nil {
		return err
	}
	var pol *stsPolicy
	if d.sts != nil {
		pol, err = d.sts.Policy(context.Background(), domain)
		if err != nil {
			return fmt.Errorf("mta-sts: %w", err)
		}
	}
	enforce := pol != nil && pol.Mode == "enforce"
	if enforce || (pol != nil && pol.Mode == "testing") {
		filtered := filterMXBySTS(hosts, pol)
		if len(filtered) == 0 {
			d.recordTLS(&tlsEvent{
				PolicyDomain: domain, ResultType: "sts-policy-invalid",
				SendingMTA: d.hostname, Count: 1,
			})
			if enforce {
				return fmt.Errorf("mta-sts: no MX hosts match policy for %s", domain)
			}
		} else {
			hosts = filtered
		}
	}

	var last error
	for _, host := range hosts {
		addr := net.JoinHostPort(host, "25")
		last = d.sendHost(addr, host, domain, from, rcpts, data, enforce)
		if last == nil {
			return nil
		}
	}
	if last == nil {
		last = fmt.Errorf("no MX hosts for %s", domain)
	}
	return last
}

func (d *directSender) sendHost(addr, serverName, policyDomain, from string, rcpts []string, data []byte, enforce bool) error {
	err := d.dialAndSend(addr, serverName, policyDomain, from, rcpts, data, true)
	if err == nil {
		return nil
	}
	if enforce {
		return err
	}
	// Opportunistic / testing: plaintext fallback after TLS problems.
	if isTLSRelated(err) {
		return d.dialAndSend(addr, serverName, policyDomain, from, rcpts, data, false)
	}
	return err
}

func isTLSRelated(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "starttls") || strings.Contains(s, "tls") || strings.Contains(s, "certificate")
}

func (d *directSender) dialAndSend(addr, serverName, policyDomain, from string, rcpts []string, data []byte, wantTLS bool) error {
	conn, err := net.DialTimeout("tcp", addr, d.timeout)
	if err != nil {
		d.recordTLS(&tlsEvent{
			PolicyDomain: policyDomain, MXHost: serverName,
			ResultType: "network-error", SendingMTA: d.hostname, Count: 1,
		})
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(d.timeout))
	c, err := smtp.NewClient(conn, serverName)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Hello(d.hostname); err != nil {
		return err
	}

	if wantTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			tlsCfg := &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}
			if err := c.StartTLS(tlsCfg); err != nil {
				d.recordTLS(&tlsEvent{
					PolicyDomain: policyDomain, MXHost: serverName,
					ResultType: "starttls-failure", SendingMTA: d.hostname, Count: 1,
				})
				return fmt.Errorf("starttls: %w", err)
			}
			d.recordTLS(&tlsEvent{
				PolicyDomain: policyDomain, MXHost: serverName,
				ResultType: "successful-session", SendingMTA: d.hostname, Count: 1,
			})
		} else {
			d.recordTLS(&tlsEvent{
				PolicyDomain: policyDomain, MXHost: serverName,
				ResultType: "starttls-not-supported", SendingMTA: d.hostname, Count: 1,
			})
			return fmt.Errorf("starttls not supported by %s", serverName)
		}
	}

	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range rcpts {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func (d *directSender) recordTLS(ev *tlsEvent) {
	if d == nil || d.tlsrpt == nil || ev == nil {
		return
	}
	d.tlsrpt.Record(ev)
}

func mxHosts(domain string) ([]string, error) {
	mxs, err := net.LookupMX(domain)
	if err != nil {
		// Fall back to A/AAAA of the domain itself (implicit MX).
		return []string{domain}, nil
	}
	sort.Slice(mxs, func(i, j int) bool { return mxs[i].Pref < mxs[j].Pref })
	out := make([]string, 0, len(mxs))
	for _, mx := range mxs {
		h := strings.TrimSuffix(mx.Host, ".")
		if h != "" {
			out = append(out, h)
		}
	}
	if len(out) == 0 {
		return []string{domain}, nil
	}
	return out, nil
}

func domainOf(email string) string {
	i := strings.LastIndex(email, "@")
	if i < 0 || i == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[i+1:])
}

// outboundSender abstracts relay vs direct MX.
type outboundSender interface {
	Send(from string, to []string, data []byte) error
}

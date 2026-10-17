package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"time"
)

type directSender struct {
	timeout  time.Duration
	hostname string
	sts      *stsResolver
	dane     *daneResolver
	tlsrpt   *tlsReporter
	log      *slog.Logger
	trace    *deliveryTrace
}

func newDirectSender(timeout time.Duration, hostname string, sts *stsResolver, dane *daneResolver, tlsrpt *tlsReporter, log *slog.Logger) *directSender {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if hostname == "" {
		hostname = "localhost"
	}
	return &directSender{timeout: timeout, hostname: hostname, sts: sts, dane: dane, tlsrpt: tlsrpt, log: log}
}

func (d *directSender) Send(from string, to []string, data []byte) error {
	return d.SendTrace(from, to, data, &deliveryTrace{log: d.log})
}
func (d *directSender) SendTrace(from string, to []string, data []byte, trace *deliveryTrace) error {
	clone := *d
	clone.trace = trace
	d = &clone
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
	d.trace.command("dns", "MX "+domain)
	hosts, err := mxHosts(domain)
	if err != nil {
		return err
	}
	d.trace.record("route", strings.Join(hosts, ", "))
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

func (d *directSender) sendHost(addr, serverName, policyDomain, from string, rcpts []string, data []byte, stsEnforce bool) error {
	daneReq := false
	if d.dane != nil {
		if _, req, err := d.dane.tlsConfigFor(serverName); err == nil {
			daneReq = req
		}
	}
	err := d.dialAndSend(addr, serverName, policyDomain, from, rcpts, data, true)
	if err == nil {
		return nil
	}
	if stsEnforce || daneReq {
		return err
	}
	// Opportunistic / testing: plaintext fallback after TLS problems (not when DANE TLSA present).
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
	return strings.Contains(s, "starttls") || strings.Contains(s, "tls") || strings.Contains(s, "certificate") || strings.Contains(s, "dane")
}

func (d *directSender) dialAndSend(addr, serverName, policyDomain, from string, rcpts []string, data []byte, wantTLS bool) (result error) {
	trace := d.trace
	if trace != nil {
		trace.peer = addr
	}
	trace.command("connect", "TCP "+addr)
	defer func() { result = trace.failure(result) }()
	conn, err := net.DialTimeout("tcp", addr, d.timeout)
	if err != nil {
		d.recordTLS(&tlsEvent{
			PolicyDomain: policyDomain, MXHost: serverName,
			ResultType: "network-error", SendingMTA: d.hostname, Count: 1,
		})
		return err
	}
	defer conn.Close()
	if trace != nil {
		trace.peer = serverName + "[" + conn.RemoteAddr().String() + "]"
	}
	trace.record("connected", "TCP connected")
	_ = conn.SetDeadline(time.Now().Add(d.timeout))
	c, err := newTracedClient(conn, serverName, trace)
	if err != nil {
		return err
	}
	defer c.Close()
	trace.command("ehlo", "EHLO "+d.hostname)
	if err := c.Hello(d.hostname); err != nil {
		return err
	}

	if wantTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			tlsCfg := &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}
			daneReq := false
			if d.dane != nil {
				var derr error
				tlsCfg, daneReq, derr = d.dane.tlsConfigFor(serverName)
				if derr != nil {
					d.recordTLS(&tlsEvent{
						PolicyDomain: policyDomain, MXHost: serverName,
						ResultType: "tlsa-invalid", SendingMTA: d.hostname, Count: 1,
					})
					return derr
				}
			}
			trace.command("starttls", "STARTTLS")
			if err := c.StartTLS(tlsCfg); err != nil {
				result := "starttls-failure"
				if daneReq || strings.Contains(strings.ToLower(err.Error()), "dane") {
					result = "validation-failure"
				}
				d.recordTLS(&tlsEvent{
					PolicyDomain: policyDomain, MXHost: serverName,
					ResultType: result, SendingMTA: d.hostname, Count: 1,
				})
				if daneReq {
					return fmt.Errorf("dane starttls: %w", err)
				}
				return fmt.Errorf("starttls: %w", err)
			}
			traceTLSReady(c, trace)
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

	trace.command("mail", "MAIL FROM:<"+from+">")
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range rcpts {
		trace.command("rcpt", "RCPT TO:<"+rcpt+">")
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	trace.command("data", "DATA")
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return err
	}
	trace.command("data_result", fmt.Sprintf("[message body omitted: %d bytes] .", len(data)))
	if err := w.Close(); err != nil {
		return err
	}
	trace.command("quit", "QUIT")
	// A final 250 accepted the message; a QUIT failure must not resend it.
	if err := c.Quit(); err != nil {
		trace.record("warning", err.Error())
	}
	return nil
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

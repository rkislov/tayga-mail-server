package smtp

import (
	"fmt"
	"net"
	"net/smtp"
	"sort"
	"strings"
	"time"
)

type directSender struct {
	timeout time.Duration
}

func newDirectSender(timeout time.Duration) *directSender {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &directSender{timeout: timeout}
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
	var last error
	for _, host := range hosts {
		addr := net.JoinHostPort(host, "25")
		last = d.sendHost(addr, host, from, rcpts, data)
		if last == nil {
			return nil
		}
	}
	if last == nil {
		last = fmt.Errorf("no MX hosts for %s", domain)
	}
	return last
}

func (d *directSender) sendHost(addr, serverName, from string, rcpts []string, data []byte) error {
	conn, err := net.DialTimeout("tcp", addr, d.timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(d.timeout))
	c, err := smtp.NewClient(conn, serverName)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Hello("localhost"); err != nil {
		return err
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

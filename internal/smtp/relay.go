package smtp

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/tayga/tms/internal/config"
)

type outboundRelay struct {
	host       string
	username   string
	password   string
	startTLS   bool
	requireTLS bool
	helo       string
	route      string
	timeout    time.Duration
}

func newOutboundRelay(cfg config.SMTPRelayConfig, timeout time.Duration) *outboundRelay {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &outboundRelay{
		host:     cfg.Host,
		username: cfg.Username,
		password: cfg.Password,
		startTLS: !cfg.DisableSTARTTLS,
		timeout:  timeout,
	}
}

func (r *outboundRelay) Send(from string, to []string, data []byte) error {
	return r.SendTrace(from, to, data, nil)
}
func (r *outboundRelay) SendTrace(from string, to []string, data []byte, trace *deliveryTrace) (result error) {
	if r == nil {
		return fmt.Errorf("outbound relay not configured")
	}
	if trace != nil {
		trace.peer = r.host
	}
	if r.route != "" {
		trace.command("route", r.route)
	}
	trace.command("connect", "TCP "+r.host)
	defer func() { result = trace.failure(result) }()
	host, _, err := net.SplitHostPort(r.host)
	if err != nil {
		host = r.host
	}
	conn, err := net.DialTimeout("tcp", r.host, r.timeout)
	if err != nil {
		return fmt.Errorf("relay dial: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(r.timeout))

	c, err := newTracedClient(conn, host, trace)
	if err != nil {
		return err
	}
	defer c.Close()

	helo := r.helo
	if helo == "" {
		helo = "localhost"
	}
	trace.command("ehlo", "EHLO "+helo)
	if err := c.Hello(helo); err != nil {
		return err
	}
	if r.startTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			trace.command("starttls", "STARTTLS")
			if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("relay starttls: %w", err)
			}
			traceTLSReady(c, trace)
		} else if r.requireTLS {
			return fmt.Errorf("DLP gateway requires STARTTLS; plaintext delivery refused")
		}
	}
	if r.username != "" {
		auth := smtp.PlainAuth("", r.username, r.password, host)
		trace.command("auth", "AUTH PLAIN [credentials redacted]")
		if trace != nil {
			trace.auth = true
		}
		err := c.Auth(auth)
		if trace != nil {
			trace.auth = false
		}
		if err != nil {
			return fmt.Errorf("relay authentication rejected")
		}
	}
	trace.command("mail", "MAIL FROM:<"+from+">")
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
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
	if err := c.Quit(); err != nil {
		trace.record("warning", err.Error())
	}
	return nil
}

func newDLPRelay(cfg config.SMTPDLPConfig, hostname string, timeout time.Duration) *outboundRelay {
	if !cfg.Enabled {
		return nil
	}
	r := newOutboundRelay(config.SMTPRelayConfig{Host: cfg.Host, Username: cfg.Username, Password: cfg.Password}, timeout)
	if r == nil {
		return nil
	}
	r.requireTLS = !cfg.AllowPlaintext
	r.helo = hostname
	r.route = "DLP SMTP gateway (external recipients)"
	return r
}

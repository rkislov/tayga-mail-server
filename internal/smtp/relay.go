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
	host     string
	username string
	password string
	startTLS bool
	timeout  time.Duration
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
	if r == nil {
		return fmt.Errorf("outbound relay not configured")
	}
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

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()

	if r.startTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("relay starttls: %w", err)
			}
		}
	}
	if r.username != "" {
		auth := smtp.PlainAuth("", r.username, r.password, host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("relay auth: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
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

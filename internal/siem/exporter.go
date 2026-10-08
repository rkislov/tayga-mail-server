package siem

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// Config controls syslog CEF export to a SIEM.
type Config struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Protocol   string `yaml:"protocol" json:"protocol"` // udp | tcp | tls
	Address    string `yaml:"address" json:"address"`   // host:port
	Facility   string `yaml:"facility" json:"facility"` // local0..local7 | auth | mail | user | daemon
	Format     string `yaml:"format" json:"format"`     // cef (only)
	Vendor     string `yaml:"vendor" json:"vendor"`
	Product    string `yaml:"product" json:"product"`
	Version    string `yaml:"version" json:"version"` // device version in CEF
	Hostname   string `yaml:"hostname" json:"hostname"`
	TLSSkipVerify bool `yaml:"tls_skip_verify" json:"tls_skip_verify"`
	QueueSize  int    `yaml:"queue_size" json:"queue_size"`
}

// Event is one security/ops event for SIEM.
type Event struct {
	Signature string
	Name      string
	Severity  int // 0–10 CEF
	Ext       map[string]string
}

// Exporter sends CEF over syslog asynchronously.
type Exporter struct {
	cfg    Config
	log    *slog.Logger
	ch     chan Event
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// New creates an exporter. Returns nil when disabled.
func New(cfg Config, log *slog.Logger) (*Exporter, error) {
	cfg = normalize(cfg)
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.Address == "" {
		return nil, fmt.Errorf("siem.address is required")
	}
	if _, _, err := net.SplitHostPort(cfg.Address); err != nil {
		return nil, fmt.Errorf("siem.address: %w", err)
	}
	switch cfg.Protocol {
	case "udp", "tcp", "tls":
	default:
		return nil, fmt.Errorf("siem.protocol must be udp, tcp, or tls")
	}
	if strings.ToLower(cfg.Format) != "cef" {
		return nil, fmt.Errorf("siem.format must be cef")
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := &Exporter{
		cfg:    cfg,
		log:    log,
		ch:     make(chan Event, cfg.QueueSize),
		cancel: cancel,
	}
	e.wg.Add(1)
	go e.loop(ctx)
	return e, nil
}

func normalize(c Config) Config {
	if c.Protocol == "" {
		c.Protocol = "udp"
	}
	c.Protocol = strings.ToLower(strings.TrimSpace(c.Protocol))
	if c.Format == "" {
		c.Format = "cef"
	}
	c.Format = strings.ToLower(strings.TrimSpace(c.Format))
	if c.Facility == "" {
		c.Facility = "local0"
	}
	if c.Vendor == "" {
		c.Vendor = "Tayga"
	}
	if c.Product == "" {
		c.Product = "TaygaMail"
	}
	if c.Hostname == "" {
		if h, err := os.Hostname(); err == nil {
			c.Hostname = h
		} else {
			c.Hostname = "tayga-mail"
		}
	}
	if c.QueueSize <= 0 {
		c.QueueSize = 256
	}
	return c
}

// Close stops the worker.
func (e *Exporter) Close() {
	if e == nil {
		return
	}
	e.cancel()
	e.wg.Wait()
}

// Emit queues an event (non-blocking; drops when full).
func (e *Exporter) Emit(ev Event) {
	if e == nil {
		return
	}
	select {
	case e.ch <- ev:
	default:
		if e.log != nil {
			e.log.Warn("siem queue full; dropping event", "signature", ev.Signature)
		}
	}
}

// EmitAuth is a convenience for login outcomes.
func (e *Exporter) EmitAuth(success bool, email, src, detail string) {
	sev := 3
	sig := "auth:login"
	name := "Login success"
	outcome := "success"
	if !success {
		sev = 7
		sig = "auth:login_fail"
		name = "Login failed"
		outcome = "failure"
	}
	e.Emit(Event{
		Signature: sig,
		Name:      name,
		Severity:  sev,
		Ext: map[string]string{
			"src":     src,
			"suser":   email,
			"outcome": outcome,
			"msg":     detail,
		},
	})
}

// EmitMail maps a mail-log style event to CEF.
func (e *Exporter) EmitMail(event, direction, peer, from, to, msgid, detail string, size int64) {
	sev := 3
	switch strings.ToLower(event) {
	case "failed", "rejected":
		sev = 6
	case "deferred":
		sev = 4
	case "sent", "delivered", "queued":
		sev = 3
	}
	ext := map[string]string{
		"src":     peer,
		"suser":   from,
		"duser":   to,
		"msg":     detail,
		"outcome": event,
		"cs1":     msgid,
		"cs1Label": "MessageID",
		"cs2":     direction,
		"cs2Label": "Direction",
	}
	if size > 0 {
		ext["cn1"] = fmt.Sprintf("%d", size)
		ext["cn1Label"] = "Bytes"
	}
	e.Emit(Event{
		Signature: "mail:" + strings.ToLower(event),
		Name:      "Mail " + event,
		Severity:  sev,
		Ext:       ext,
	})
}

func (e *Exporter) loop(ctx context.Context) {
	defer e.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-e.ch:
			if err := e.send(ev); err != nil && e.log != nil {
				e.log.Warn("siem send failed", "err", err, "signature", ev.Signature)
			}
		}
	}
}

func (e *Exporter) send(ev Event) error {
	cef := FormatCEF(e.cfg.Vendor, e.cfg.Product, e.cfg.Version, ev.Signature, ev.Name, ev.Severity, ev.Ext)
	line := formatSyslog(e.cfg, cef)
	return dialAndWrite(e.cfg, []byte(line))
}

func formatSyslog(cfg Config, msg string) string {
	pri := facilityCode(cfg.Facility)*8 + 6 // informational
	ts := time.Now().UTC().Format(time.RFC3339)
	// RFC5424
	return fmt.Sprintf("<%d>1 %s %s tayga-mail - %s - %s\n", pri, ts, cfg.Hostname, sanitizeMSGID(msgSignature(msg)), msg)
}

func msgSignature(cef string) string {
	// Device Event Class ID is 5th CEF field
	parts := strings.SplitN(cef, "|", 6)
	if len(parts) >= 5 {
		return parts[4]
	}
	return "-"
}

func sanitizeMSGID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	s = strings.ReplaceAll(s, " ", "_")
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}

func facilityCode(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "kern":
		return 0
	case "user":
		return 1
	case "mail":
		return 2
	case "daemon":
		return 3
	case "auth", "security":
		return 4
	case "syslog":
		return 5
	case "lpr":
		return 6
	case "news":
		return 7
	case "uucp":
		return 8
	case "cron":
		return 9
	case "authpriv":
		return 10
	case "ftp":
		return 11
	case "local0":
		return 16
	case "local1":
		return 17
	case "local2":
		return 18
	case "local3":
		return 19
	case "local4":
		return 20
	case "local5":
		return 21
	case "local6":
		return 22
	case "local7":
		return 23
	default:
		return 16
	}
}

func dialAndWrite(cfg Config, payload []byte) error {
	d := 5 * time.Second
	switch cfg.Protocol {
	case "udp":
		conn, err := net.DialTimeout("udp", cfg.Address, d)
		if err != nil {
			return err
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(d))
		_, err = conn.Write(payload)
		return err
	case "tcp":
		conn, err := net.DialTimeout("tcp", cfg.Address, d)
		if err != nil {
			return err
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(d))
		_, err = conn.Write(payload)
		return err
	case "tls":
		dialer := &net.Dialer{Timeout: d}
		conn, err := tls.DialWithDialer(dialer, "tcp", cfg.Address, &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.TLSSkipVerify, //nolint:gosec // optional for lab SIEMs
		})
		if err != nil {
			return err
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(d))
		_, err = conn.Write(payload)
		return err
	default:
		return fmt.Errorf("unsupported protocol %q", cfg.Protocol)
	}
}

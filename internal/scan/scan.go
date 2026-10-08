package scan

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrUnavailable = errors.New("scanner unavailable")
)

// Result is the outcome of scanning a message body.
type Result struct {
	Clean    bool
	Virus    string // signature / reason when !Clean
	Scanner  string
	Duration time.Duration
}

// Scanner inspects raw RFC822 message bytes.
type Scanner interface {
	Name() string
	Scan(ctx context.Context, data []byte) (*Result, error)
}

// Action after a positive detection.
type Action string

const (
	ActionReject     Action = "reject"
	ActionQuarantine Action = "quarantine"
	ActionTag        Action = "tag" // deliver with X-Virus-Status header
)

// Config is loaded from YAML (see config.ScanConfig).
type Config struct {
	Enabled          bool
	Backend          string // none | clamav | exec | icap
	Action           Action
	QuarantineFolder string
	Timeout          time.Duration
	FailOpen         bool // if true, scanner errors do not block delivery
	ClamAVAddress    string
	ExecCommand      []string
	ICAPURL          string
}

func (c Config) Normalized() Config {
	if c.Action == "" {
		c.Action = ActionQuarantine
	}
	if c.QuarantineFolder == "" {
		c.QuarantineFolder = "Quarantine"
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if c.Backend == "" {
		c.Backend = "clamav"
	}
	if c.ClamAVAddress == "" {
		c.ClamAVAddress = "127.0.0.1:3310"
	}
	if c.ICAPURL == "" {
		c.ICAPURL = "icap://127.0.0.1:1344/reqmod"
	}
	return c
}

// New builds a Scanner from config. Returns nil when disabled.
func New(cfg Config) (Scanner, error) {
	cfg = cfg.Normalized()
	if !cfg.Enabled {
		return nil, nil
	}
	switch cfg.Backend {
	case "none", "noop":
		return Noop{}, nil
	case "clamav":
		return NewClamAV(cfg.ClamAVAddress, cfg.Timeout), nil
	case "exec":
		if len(cfg.ExecCommand) == 0 {
			return nil, fmt.Errorf("scan.exec.command is required for backend exec")
		}
		return NewExec(cfg.ExecCommand, cfg.Timeout), nil
	case "icap":
		return NewICAP(cfg.ICAPURL, cfg.Timeout)
	default:
		return nil, fmt.Errorf("unknown scan backend %q", cfg.Backend)
	}
}

// Noop always reports clean (for tests / dry-run wiring).
type Noop struct{}

func (Noop) Name() string { return "noop" }

func (Noop) Scan(_ context.Context, _ []byte) (*Result, error) {
	return &Result{Clean: true, Scanner: "noop"}, nil
}

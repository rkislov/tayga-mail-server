package spam

import (
	"context"
	"fmt"
	"time"
)

// Result is the outcome of a spam check.
type Result struct {
	Score    float64
	Required float64
	Action   string // no action | add header | rewrite subject | greylist | reject | soft reject
	Symbols  []string
	Scanner  string
}

// Checker scores a raw message.
type Checker interface {
	Name() string
	Check(ctx context.Context, meta Meta, data []byte) (*Result, error)
}

// Meta is envelope context for the checker.
type Meta struct {
	From       string
	To         string
	Helo       string
	IP         string
	User       string // authenticated submitter, if any
	Hostname   string
}

// Action is what Tayga does after scoring.
type Action string

const (
	ActionPass       Action = "pass"
	ActionTag        Action = "tag"
	ActionQuarantine Action = "quarantine"
	ActionReject     Action = "reject"
	ActionGreylist   Action = "greylist"
)

// Config from YAML.
type Config struct {
	Enabled          bool
	Backend          string // rspamd | none
	URL              string
	Password         string
	Timeout          time.Duration
	FailOpen         bool
	Folder           string  // quarantine/junk folder
	FollowRspamd     bool    // map rspamd action when true
	RejectAbove      float64 // used when !FollowRspamd; 0 = unset
	QuarantineAbove  float64
	TagAbove         float64
}

func (c Config) Normalized() Config {
	if c.Backend == "" {
		c.Backend = "rspamd"
	}
	if c.URL == "" {
		c.URL = "http://127.0.0.1:11333"
	}
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if c.Folder == "" {
		c.Folder = "Junk"
	}
	return c
}

// New builds a Checker. nil when disabled.
func New(cfg Config) (Checker, error) {
	cfg = cfg.Normalized()
	if !cfg.Enabled {
		return nil, nil
	}
	switch cfg.Backend {
	case "none", "noop":
		return Noop{}, nil
	case "rspamd":
		return NewRspamd(cfg.URL, cfg.Password, cfg.Timeout), nil
	default:
		return nil, fmt.Errorf("unknown spam backend %q", cfg.Backend)
	}
}

// MapAction converts a check result into a Tayga action.
// With FollowRspamd, reject/greylist from Rspamd win first; score thresholds
// (if set) then apply; remaining Rspamd "add header"/"rewrite subject" become tag.
func MapAction(cfg Config, res *Result) Action {
	if res == nil {
		return ActionPass
	}
	cfg = cfg.Normalized()
	if cfg.FollowRspamd {
		switch res.Action {
		case "reject":
			return ActionReject
		case "soft reject", "greylist":
			return ActionGreylist
		}
	}
	if cfg.RejectAbove > 0 && res.Score >= cfg.RejectAbove {
		return ActionReject
	}
	if cfg.QuarantineAbove > 0 && res.Score >= cfg.QuarantineAbove {
		return ActionQuarantine
	}
	if cfg.TagAbove > 0 && res.Score >= cfg.TagAbove {
		return ActionTag
	}
	if cfg.FollowRspamd {
		switch res.Action {
		case "add header", "rewrite subject", "rewrite_subject":
			return ActionTag
		}
	}
	return ActionPass
}

// Noop always passes.
type Noop struct{}

func (Noop) Name() string { return "noop" }

func (Noop) Check(_ context.Context, _ Meta, _ []byte) (*Result, error) {
	return &Result{Action: "no action", Scanner: "noop"}, nil
}

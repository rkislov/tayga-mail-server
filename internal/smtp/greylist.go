package smtp

import (
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	gosmtp "github.com/emersion/go-smtp"
)

type greylistPolicy struct {
	delay   time.Duration
	passTTL time.Duration
	ipv4Net int // e.g. 24; 32 = exact IP
	log     *slog.Logger

	mu      sync.Mutex
	entries map[string]*greyEntry
}

type greyEntry struct {
	firstSeen time.Time
	passed    bool
	lastSeen  time.Time
}

func newGreylistPolicy(delay, passTTL time.Duration, ipv4Net int, log *slog.Logger) *greylistPolicy {
	if delay <= 0 {
		delay = 5 * time.Minute
	}
	if passTTL <= 0 {
		passTTL = 36 * time.Hour
	}
	if ipv4Net <= 0 || ipv4Net > 32 {
		ipv4Net = 32
	}
	p := &greylistPolicy{
		delay: delay, passTTL: passTTL, ipv4Net: ipv4Net, log: log,
		entries: map[string]*greyEntry{},
	}
	go p.cleanupLoop()
	return p
}

func (p *greylistPolicy) check(ipStr, from, rcpt string) error {
	if p == nil {
		return nil
	}
	key := greylistKey(ipStr, from, rcpt, p.ipv4Net)
	now := time.Now().UTC()

	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[key]
	if !ok {
		p.entries[key] = &greyEntry{firstSeen: now, lastSeen: now}
		return greylistDefer()
	}
	e.lastSeen = now
	if e.passed {
		return nil
	}
	if now.Sub(e.firstSeen) >= p.delay {
		e.passed = true
		return nil
	}
	return greylistDefer()
}

func greylistDefer() error {
	return &gosmtp.SMTPError{Code: 421, EnhancedCode: gosmtp.EnhancedCode{4, 7, 1}, Message: "Try again later (greylisted)"}
}

func greylistKey(ipStr, from, rcpt string, ipv4Net int) string {
	ip := normalizeGreyIP(ipStr, ipv4Net)
	from = strings.ToLower(strings.Trim(strings.TrimSpace(from), "<>"))
	rcpt = strings.ToLower(strings.Trim(strings.TrimSpace(rcpt), "<>"))
	return ip + "\x00" + from + "\x00" + rcpt
}

func normalizeGreyIP(ipStr string, ipv4Net int) string {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return strings.ToLower(strings.TrimSpace(ipStr))
	}
	if v4 := ip.To4(); v4 != nil && ipv4Net < 32 {
		mask := net.CIDRMask(ipv4Net, 32)
		return v4.Mask(mask).String() + "/" + strconv.Itoa(ipv4Net)
	}
	return ip.String()
}

func (p *greylistPolicy) cleanupLoop() {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for range t.C {
		p.cleanup(time.Now().UTC())
	}
}

func (p *greylistPolicy) cleanup(now time.Time) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, e := range p.entries {
		if now.Sub(e.lastSeen) > p.passTTL {
			delete(p.entries, k)
		}
	}
}

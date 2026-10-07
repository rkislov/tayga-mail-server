package smtp

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/storage"
)

type greylistStore interface {
	GreylistTouch(ctx context.Context, clientIP, envelopeFrom, rcpt string, delay time.Duration) (allowed bool, err error)
	DeleteExpiredGreylist(ctx context.Context, olderThan time.Time) (int64, error)
}

type greylistPolicy struct {
	store   greylistStore
	delay   time.Duration
	passTTL time.Duration
	ipv4Net int // e.g. 24; 32 = exact IP
	log     *slog.Logger
}

func newGreylistPolicy(store greylistStore, delay, passTTL time.Duration, ipv4Net int, log *slog.Logger) *greylistPolicy {
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
		store: store, delay: delay, passTTL: passTTL, ipv4Net: ipv4Net, log: log,
	}
	go p.cleanupLoop()
	return p
}

func (p *greylistPolicy) check(ipStr, from, rcpt string) error {
	if p == nil || p.store == nil {
		return nil
	}
	ip := normalizeGreyIP(ipStr, p.ipv4Net)
	from = strings.ToLower(strings.Trim(strings.TrimSpace(from), "<>"))
	rcpt = strings.ToLower(strings.Trim(strings.TrimSpace(rcpt), "<>"))

	allowed, err := p.store.GreylistTouch(context.Background(), ip, from, rcpt, p.delay)
	if err != nil {
		if p.log != nil {
			p.log.Warn("greylist touch failed; fail-open", "err", err)
		}
		return nil
	}
	if !allowed {
		return greylistDefer()
	}
	return nil
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
		if p.store == nil {
			continue
		}
		cutoff := time.Now().UTC().Add(-p.passTTL)
		if n, err := p.store.DeleteExpiredGreylist(context.Background(), cutoff); err != nil {
			if p.log != nil {
				p.log.Warn("greylist cleanup", "err", err)
			}
		} else if n > 0 && p.log != nil {
			p.log.Debug("greylist cleanup", "deleted", n)
		}
	}
}

// Ensure storage.Driver satisfies greylistStore at compile time.
var _ greylistStore = (storage.Driver)(nil)

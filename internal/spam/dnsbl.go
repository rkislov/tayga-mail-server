package spam

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// DNSBL augments message scoring for unauthenticated public SMTP peers.
// Lists are opt-in; DNS lookup failures never count as a listing.
type dnsblCacheEntry struct {
	addresses []net.IP
	expires   time.Time
}

type DNSBL struct {
	mu     sync.Mutex
	cache  map[string]dnsblCacheEntry
	next   Checker
	zones  []string
	score  float64
	lookup func(context.Context, string) ([]net.IP, error)
}

func (d *DNSBL) Name() string { return d.next.Name() + "+dnsbl" }
func (d *DNSBL) Check(ctx context.Context, meta Meta, data []byte) (*Result, error) {
	result, err := d.next.Check(ctx, meta, data)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &Result{}
	}
	ip := net.ParseIP(meta.IP)
	if meta.User != "" || ip == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return result, nil
	}
	reverse := ""
	if v4 := ip.To4(); v4 != nil {
		reverse = fmt.Sprintf("%d.%d.%d.%d", v4[3], v4[2], v4[1], v4[0])
	} else {
		hex := fmt.Sprintf("%032x", []byte(ip.To16()))
		parts := []string{}
		for i := len(hex) - 1; i >= 0; i-- {
			parts = append(parts, string(hex[i]))
		}
		reverse = strings.Join(parts, ".")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for _, zone := range d.zones {
		addresses, err := d.cachedLookup(ctx, reverse+"."+strings.TrimSuffix(zone, "."))
		if err != nil {
			continue
		}
		listed := false
		for _, answer := range addresses {
			v4 := answer.To4()
			if v4 != nil && v4[0] == 127 && v4[1] == 0 && v4[2] == 0 && v4[3] >= 2 && v4[3] <= 11 {
				listed = true
				break
			}
		}
		if listed {
			result.Score += d.score
			result.Symbols = append(result.Symbols, "DNSBL:"+zone)
		}
	}
	return result, nil
}
func withDNSBL(checker Checker, cfg Config) Checker {
	if !cfg.DNSBLEnabled || len(cfg.DNSBLZones) == 0 {
		return checker
	}
	score := cfg.DNSBLScore
	if score <= 0 {
		score = 3
	}
	return &DNSBL{next: checker, zones: cfg.DNSBLZones, score: score, lookup: func(ctx context.Context, host string) ([]net.IP, error) {
		return net.DefaultResolver.LookupIP(ctx, "ip4", host)
	}}
}

func (d *DNSBL) cachedLookup(ctx context.Context, host string) ([]net.IP, error) {
	d.mu.Lock()
	cached, ok := d.cache[host]
	d.mu.Unlock()
	if ok && time.Now().Before(cached.expires) {
		return cached.addresses, nil
	}
	addresses, err := d.lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cache == nil || len(d.cache) >= 4096 {
		d.cache = make(map[string]dnsblCacheEntry)
	}
	d.cache[host] = dnsblCacheEntry{addresses: addresses, expires: time.Now().Add(5 * time.Minute)}
	return addresses, nil
}

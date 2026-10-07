package smtp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// stsPolicy is a parsed MTA-STS policy (RFC 8461).
type stsPolicy struct {
	ID      string
	Mode    string // enforce | testing | none
	MXs     []string
	MaxAge  time.Duration
	Fetched time.Time
}

type stsCacheEntry struct {
	policy *stsPolicy
	err    error // cached negative (no policy)
	until  time.Time
}

type stsResolver struct {
	timeout  time.Duration
	failOpen bool
	log      *slog.Logger
	client   *http.Client
	lookupTXT func(name string) ([]string, error)

	mu    sync.Mutex
	cache map[string]stsCacheEntry
}

func newSTSResolver(timeout time.Duration, failOpen bool, log *slog.Logger) *stsResolver {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &stsResolver{
		timeout:  timeout,
		failOpen: failOpen,
		log:      log,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // RFC 8461: no redirects
			},
		},
		lookupTXT: net.LookupTXT,
		cache:     map[string]stsCacheEntry{},
	}
}

// Policy returns the MTA-STS policy for domain, or nil if none / fetch failed with fail-open.
func (r *stsResolver) Policy(ctx context.Context, domain string) (*stsPolicy, error) {
	if r == nil {
		return nil, nil
	}
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	if domain == "" {
		return nil, nil
	}

	r.mu.Lock()
	if e, ok := r.cache[domain]; ok && time.Now().Before(e.until) {
		r.mu.Unlock()
		return e.policy, e.err
	}
	r.mu.Unlock()

	pol, err := r.fetch(ctx, domain)
	negTTL := 5 * time.Minute
	until := time.Now().Add(negTTL)
	if pol != nil && pol.MaxAge > 0 {
		until = time.Now().Add(pol.MaxAge)
	}
	r.mu.Lock()
	r.cache[domain] = stsCacheEntry{policy: pol, err: err, until: until}
	r.mu.Unlock()
	if err != nil {
		if r.failOpen {
			if r.log != nil {
				r.log.Warn("mta-sts policy fetch failed; fail-open", "domain", domain, "err", err)
			}
			return nil, nil
		}
		return nil, err
	}
	return pol, nil
}

func (r *stsResolver) fetch(ctx context.Context, domain string) (*stsPolicy, error) {
	id, err := r.lookupPolicyID(domain)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, nil // no STS
	}
	url := "https://mta-sts." + domain + "/.well-known/mta-sts.txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mta-sts https: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mta-sts https status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	pol, err := parseSTSPolicy(string(body))
	if err != nil {
		return nil, err
	}
	pol.ID = id
	pol.Fetched = time.Now().UTC()
	return pol, nil
}

func (r *stsResolver) lookupPolicyID(domain string) (string, error) {
	lookup := r.lookupTXT
	if lookup == nil {
		lookup = net.LookupTXT
	}
	txts, err := lookup("_mta-sts." + domain)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return "", nil
		}
		// NXDOMAIN / no such host often surfaces differently
		if strings.Contains(strings.ToLower(err.Error()), "no such host") ||
			strings.Contains(strings.ToLower(err.Error()), "not found") {
			return "", nil
		}
		return "", err
	}
	for _, txt := range txts {
		txt = strings.TrimSpace(txt)
		if !strings.HasPrefix(strings.ToLower(txt), "v=stsv1") {
			continue
		}
		id := ""
		for _, part := range strings.Split(txt, ";") {
			part = strings.TrimSpace(part)
			if len(part) >= 3 && strings.EqualFold(part[:2], "id") && part[2] == '=' {
				id = strings.TrimSpace(part[3:])
			}
		}
		if id != "" {
			return id, nil
		}
	}
	return "", nil
}

func parseSTSPolicy(body string) (*stsPolicy, error) {
	pol := &stsPolicy{Mode: "none"}
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(v)
		switch k {
		case "version":
			if !strings.EqualFold(v, "STSv1") {
				return nil, fmt.Errorf("unsupported mta-sts version %q", v)
			}
		case "mode":
			pol.Mode = strings.ToLower(v)
		case "mx":
			if v != "" {
				pol.MXs = append(pol.MXs, strings.ToLower(strings.TrimSuffix(v, ".")))
			}
		case "max_age":
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid max_age %q", v)
			}
			pol.MaxAge = time.Duration(n) * time.Second
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	switch pol.Mode {
	case "enforce", "testing", "none":
	default:
		return nil, fmt.Errorf("invalid mta-sts mode %q", pol.Mode)
	}
	if pol.Mode != "none" && len(pol.MXs) == 0 {
		return nil, fmt.Errorf("mta-sts policy missing mx")
	}
	if pol.MaxAge <= 0 {
		pol.MaxAge = 24 * time.Hour
	}
	return pol, nil
}

// matchSTSMX reports whether host matches any STS mx: pattern (RFC 8461 §3.1).
func matchSTSMX(patterns []string, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(p), "."))
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "*.") {
			suffix := p[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) && host != suffix[1:] {
				// host must have a label before the suffix
				rest := strings.TrimSuffix(host, suffix)
				if rest != "" && !strings.Contains(rest, ".") {
					return true
				}
			}
			continue
		}
		if host == p {
			return true
		}
	}
	return false
}

func filterMXBySTS(hosts []string, pol *stsPolicy) []string {
	if pol == nil || pol.Mode == "none" || len(pol.MXs) == 0 {
		return hosts
	}
	var out []string
	for _, h := range hosts {
		if matchSTSMX(pol.MXs, h) {
			out = append(out, h)
		}
	}
	return out
}

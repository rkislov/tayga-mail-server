// SPDX-License-Identifier: Apache-2.0
package mmfederation

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// NewHTTPClient pins a validated address for each connection. A domain-scoped
// administrator cannot turn the federation transport into an internal proxy.
func NewHTTPClient(allowedCIDRs []string) (*http.Client, error) {
	prefixes := []netip.Prefix{}
	for _, raw := range allowedCIDRs {
		p, e := netip.ParsePrefix(raw)
		if e != nil {
			return nil, fmt.Errorf("invalid federation CIDR")
		}
		prefixes = append(prefixes, p)
	}
	permitted := func(ip netip.Addr) bool {
		ip = ip.Unmap()
		if !ip.IsValid() || ip.IsUnspecified() || ip.IsMulticast() {
			return false
		}
		for _, p := range prefixes {
			if p.Contains(ip) {
				return true
			}
		}
		// These special-use ranges are unicast but are not public peers.
		for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/23", "2001:db8::/32", "2002::/16"} {
			if netip.MustParsePrefix(raw).Contains(ip) {
				return false
			}
		}
		return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
	}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if e != nil {
			return nil, fmt.Errorf("federation DNS lookup failed")
		}
		for _, ip := range ips {
			if !permitted(ip) {
				return nil, ErrForbidden
			}
		}
		var last error
		for _, ip := range ips {
			c, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if e == nil {
				return c, nil
			}
			last = e
		}
		if last == nil {
			last = ErrForbidden
		}
		return nil, last
	}
	transport := &http.Transport{Proxy: nil, DialContext: dial, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 30 * time.Second, MaxIdleConns: 16, MaxIdleConnsPerHost: 2, IdleConnTimeout: 60 * time.Second}
	return &http.Client{Transport: transport, Timeout: 30 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}
func (s *Service) request(ctx context.Context, p Peer, method, path string, body io.Reader, kind string) (*http.Response, error) {
	if !strings.HasPrefix(path, "/api/v4/") || s.HTTP == nil || (s.ScopeAllowed != nil && !s.ScopeAllowed(ctx, p.TenantID, p.DomainID)) {
		return nil, ErrInvalid
	}
	// Re-read credentials immediately before each request, including retries.
	c, e := s.credentials(ctx, p)
	if e != nil {
		return nil, e
	}
	r, e := http.NewRequestWithContext(ctx, method, p.SiteURL+path, body)
	if e != nil {
		return nil, ErrInvalid
	}
	if kind != "" {
		r.Header.Set("Content-Type", kind)
	}
	if p.Mode == "native" {
		r.Header.Set("X-RemoteCluster-Id", p.ID)
		token := c.RemoteToken
		if path == "/api/v4/remotecluster/confirm_invite" && c.BootstrapToken != "" {
			token = c.BootstrapToken
		}
		r.Header.Set("X-RemoteCluster-Token", token)
	} else {
		r.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}
	response, e := s.HTTP.Do(r)
	if e != nil {
		return nil, fmt.Errorf("federation transport failed")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, fmt.Errorf("federation HTTP status %d", response.StatusCode)
	}
	return response, nil
}
func decodeJSON(r io.Reader, v any, limit int64) error {
	raw, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil || int64(len(raw)) > limit {
		return ErrInvalid
	}
	if e = json.Unmarshal(raw, v); e != nil {
		return ErrInvalid
	}
	return nil
}

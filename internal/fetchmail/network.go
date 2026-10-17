package fetchmail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

func publicIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48", "2002::/16", "2001::/32"} {
		if netip.MustParsePrefix(cidr).Contains(addr) {
			return false
		}
	}
	return true
}
func dialPublic(ctx context.Context, host string, port int) (net.Conn, error) {
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, errors.New("server DNS lookup failed")
	}
	if len(addresses) == 0 {
		return nil, errors.New("server has no IP addresses")
	}
	// Pin validated IPs for this connection; a DNS rebinding cannot reach local services.
	for _, ip := range addresses {
		if !publicIP(ip) {
			return nil, errors.New("external server must use public IP addresses")
		}
	}
	// Do not create an import feedback loop by connecting back to this server.
	local, e := net.InterfaceAddrs()
	if e != nil {
		return nil, errors.New("cannot verify external server address")
	}
	for _, address := range local {
		prefix, e := netip.ParsePrefix(address.String())
		if e != nil {
			continue
		}
		for _, ip := range addresses {
			a, ok := netip.AddrFromSlice(ip)
			if ok && a.Unmap() == prefix.Addr().Unmap() {
				return nil, errors.New("collection from this server is not allowed")
			}
		}
	}
	d := net.Dialer{Timeout: 20 * time.Second}
	var last error
	for _, ip := range addresses {
		c, e := d.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), strconv.Itoa(port)))
		if e == nil {
			return c, nil
		}
		last = e
	}
	return nil, fmt.Errorf("connection failed: %w", last)
}
func (s *Service) connect(ctx context.Context, a *Source) (net.Conn, error) {
	for _, local := range s.localHosts {
		if local != "" && strings.EqualFold(strings.TrimSuffix(a.Host, "."), strings.TrimSuffix(local, ".")) {
			return nil, errors.New("collection from this server is not allowed")
		}
	}
	c, err := s.dial(ctx, strings.Trim(a.Host, "[]"), a.Port)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		c.SetDeadline(deadline)
	}
	if a.Security == "tls" {
		tc := tls.Client(c, s.tlsConfig(a))
		if err = tc.HandshakeContext(ctx); err != nil {
			c.Close()
			return nil, fmt.Errorf("TLS verification failed: %w", err)
		}
		c = tc
	}
	return c, nil
}
func tlsConfig(a *Source) *tls.Config {
	return &tls.Config{ServerName: strings.Trim(a.Host, "[]"), MinVersion: tls.VersionTLS12}
}
func (s *Service) collect(ctx context.Context, a *Source, test bool) (int64, bool, error) {
	u, err := s.store.GetUserByID(ctx, a.UserID)
	if err != nil || !u.Enabled {
		return 0, false, errors.New("local account unavailable")
	}
	if !test && s.Writers != nil {
		if err = s.Writers.AllowWrite(ctx, u.ID); err != nil {
			return 0, false, err
		}
	}
	password, err := s.password(a)
	if err != nil {
		return 0, false, errors.New("stored credentials unavailable")
	}
	conn, err := s.connect(ctx, a)
	if err != nil {
		return 0, false, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if a.Protocol == "imap" {
		return s.imap(ctx, a, conn, password, test)
	}
	return s.pop3(ctx, a, conn, password, test)
}

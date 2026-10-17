package xmpp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const streamNS = "http://etherx.jabber.org/streams"
const tlsNS = "urn:ietf:params:xml:ns:xmpp-tls"
const saslNS = "urn:ietf:params:xml:ns:xmpp-sasl"

// This is an outbound publisher transport, not general-purpose federation.
// Only administrator-allowlisted service domains can be contacted. Require
// PKIX TLS, client-certificate EXTERNAL and bidi; never downgrade to dialback.
func (s *Server) pushS2SAllowed(service string) bool {
	if s.cfg == nil || s.tlsCfg == nil {
		return false
	}
	domain := ParseJID(service).Domain
	for _, allowed := range s.cfg.XMPP.PushServices {
		if strings.EqualFold(allowed, domain) {
			return true
		}
	}
	return false
}
func (s *Server) pushClientCertificate(domain string) (*tls.Certificate, error) {
	if s.tlsCfg == nil {
		return nil, fmt.Errorf("XMPP TLS not configured")
	}
	var cert *tls.Certificate
	var err error
	if s.tlsCfg.GetCertificate != nil {
		cert, err = s.tlsCfg.GetCertificate(&tls.ClientHelloInfo{ServerName: domain})
	} else if len(s.tlsCfg.Certificates) > 0 {
		cert = &s.tlsCfg.Certificates[0]
	}
	if err != nil || cert == nil || len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("XMPP client certificate unavailable")
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil || leaf.VerifyHostname(domain) != nil {
		return nil, fmt.Errorf("XMPP certificate does not cover account domain %s", domain)
	}
	if now := time.Now(); now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		return nil, fmt.Errorf("XMPP certificate expired or not yet valid")
	}
	return cert, nil
}
func publicPushIP(ip net.IP) bool {
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/32", "2002::/16", "64:ff9b::/96", "64:ff9b:1::/48"} {
		if netip.MustParsePrefix(raw).Contains(a) {
			return false
		}
	}
	return true
}
func dialPushService(ctx context.Context, domain string) (net.Conn, error) {
	if net.ParseIP(domain) != nil {
		return nil, fmt.Errorf("push service must use a DNS domain")
	}
	_, records, err := net.DefaultResolver.LookupSRV(ctx, "xmpp-server", "tcp", domain)
	if err != nil {
		var dnsErr *net.DNSError
		if e, ok := err.(*net.DNSError); ok {
			dnsErr = e
		}
		if dnsErr == nil || !dnsErr.IsNotFound {
			return nil, fmt.Errorf("push SRV lookup failed: %w", err)
		}
	}
	if len(records) == 0 {
		records = []*net.SRV{{Target: domain, Port: 5269}}
	}
	var last error
	for i, record := range records {
		if i >= 8 {
			break
		}
		host := strings.TrimSuffix(record.Target, ".")
		if host == "" || record.Port == 0 {
			return nil, fmt.Errorf("push service disabled in DNS")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			last = fmt.Errorf("push address lookup failed: %w", err)
			continue
		}
		// Pin a resolved public IP. A private address in a mixed DNS answer rejects
		// the whole target. TLS still authenticates the original service domain.
		for _, ip := range ips {
			if !publicPushIP(ip.IP) {
				return nil, fmt.Errorf("push DNS returned a non-public address")
			}
		}
		for n, ip := range ips {
			if n >= 4 {
				break
			}
			dialer := net.Dialer{Timeout: 5 * time.Second}
			conn, e := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip.IP.String(), strconv.Itoa(int(record.Port))))
			if e == nil {
				return conn, nil
			}
			last = e
		}
	}
	if last == nil {
		last = fmt.Errorf("push DNS returned no address")
	}
	return nil, last
}
func (s *Server) publishPushS2S(ctx context.Context, owner, service, id string, raw []byte) error {
	if !s.pushS2SAllowed(service) {
		return fmt.Errorf("push service not allowlisted")
	}
	local, remote := ParseJID(owner).Domain, ParseJID(service).Domain
	cert, err := s.pushClientCertificate(local)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	conn, err := dialPushService(ctx, remote)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	config := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: remote, GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return cert, nil }}
	typ, inner, err := pushS2SExchange(ctx, conn, config, local, remote, owner, service, id, raw)
	if err != nil {
		return err
	}
	s.acceptPushReply(service, owner, id, typ, inner)
	return nil
}
func nextPushElement(d *xml.Decoder) (xml.StartElement, error) {
	for {
		tok, err := d.Token()
		if err != nil {
			return xml.StartElement{}, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se, nil
		}
		if _, ok := tok.(xml.EndElement); ok {
			return xml.StartElement{}, fmt.Errorf("push stream closed")
		}
	}
}
func pushFeatures(d *xml.Decoder, local, remote string) (map[xml.Name]bool, []string, error) {
	start, err := nextPushElement(d)
	if err != nil {
		return nil, nil, err
	}
	if start.Name != (xml.Name{Space: streamNS, Local: "stream"}) || attr(start, "from") != remote || (attr(start, "to") != "" && attr(start, "to") != local) {
		return nil, nil, fmt.Errorf("invalid push stream identity")
	}
	se, err := nextPushElement(d)
	if err != nil {
		return nil, nil, err
	}
	if se.Name != (xml.Name{Space: streamNS, Local: "features"}) {
		return nil, nil, fmt.Errorf("missing push stream features")
	}
	var features struct {
		Children []struct {
			XMLName    xml.Name
			Mechanisms []string `xml:"mechanism"`
		} `xml:",any"`
	}
	if err = d.DecodeElement(&features, &se); err != nil {
		return nil, nil, err
	}
	found := map[xml.Name]bool{}
	var mechanisms []string
	for _, child := range features.Children {
		found[child.XMLName] = true
		if child.XMLName == (xml.Name{Space: saslNS, Local: "mechanisms"}) {
			mechanisms = child.Mechanisms
		}
	}
	return found, mechanisms, nil
}
func pushS2SExchange(ctx context.Context, conn net.Conn, config *tls.Config, local, remote, owner, service, id string, raw []byte) (string, string, error) {
	deadline := time.Now().Add(25 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	stream := fmt.Sprintf(`<stream:stream xmlns:stream='%s' xmlns='jabber:server' from='%s' to='%s' version='1.0'>`, streamNS, xmlEscape(local), xmlEscape(remote))
	write := func(raw string) error { _, err := io.WriteString(conn, raw); return err }
	decoder := func() *xml.Decoder { return xml.NewDecoder(io.LimitReader(conn, 128<<10)) }
	if err := write(stream); err != nil {
		return "", "", err
	}
	d := decoder()
	features, _, err := pushFeatures(d, local, remote)
	if err != nil {
		return "", "", err
	}
	if !features[xml.Name{Space: tlsNS, Local: "starttls"}] {
		return "", "", fmt.Errorf("push gateway does not offer STARTTLS")
	}
	if err = write(`<starttls xmlns='` + tlsNS + `'/>`); err != nil {
		return "", "", err
	}
	se, err := nextPushElement(d)
	if err != nil {
		return "", "", err
	}
	if se.Name != (xml.Name{Space: tlsNS, Local: "proceed"}) {
		return "", "", fmt.Errorf("push gateway refused TLS")
	}
	if err = d.Skip(); err != nil {
		return "", "", err
	}
	secure := tls.Client(conn, config)
	if err = secure.HandshakeContext(ctx); err != nil {
		return "", "", fmt.Errorf("push TLS: %w", err)
	}
	conn = secure
	if err = write(stream); err != nil {
		return "", "", err
	}
	d = decoder()
	features, mechanisms, err := pushFeatures(d, local, remote)
	if err != nil {
		return "", "", err
	}
	external := false
	for _, m := range mechanisms {
		if m == "EXTERNAL" {
			external = true
		}
	}
	if !external || !features[xml.Name{Space: "urn:xmpp:features:bidi", Local: "bidi"}] {
		return "", "", fmt.Errorf("push gateway requires unsupported authentication or lacks bidirectional S2S")
	}
	if err = write(`<bidi xmlns='urn:xmpp:bidi'/><auth xmlns='` + saslNS + `' mechanism='EXTERNAL'>` + base64.StdEncoding.EncodeToString([]byte(local)) + `</auth>`); err != nil {
		return "", "", err
	}
	se, err = nextPushElement(d)
	if err != nil {
		return "", "", err
	}
	if se.Name != (xml.Name{Space: saslNS, Local: "success"}) {
		return "", "", fmt.Errorf("push gateway rejected EXTERNAL authentication")
	}
	if err = d.Skip(); err != nil {
		return "", "", err
	}
	if err = write(stream); err != nil {
		return "", "", err
	}
	d = decoder()
	if _, _, err = pushFeatures(d, local, remote); err != nil {
		return "", "", err
	}
	if err = write(string(raw)); err != nil {
		return "", "", err
	}
	for n := 0; n < 32; n++ {
		se, err = nextPushElement(d)
		if err != nil {
			return "", "", err
		}
		inner, e := readInnerXML(d, se)
		if e != nil {
			return "", "", e
		}
		if se.Name == (xml.Name{Space: "jabber:server", Local: "iq"}) && attr(se, "id") == id && (attr(se, "type") == "result" || attr(se, "type") == "error") {
			if attr(se, "from") != service || attr(se, "to") != owner {
				return "", "", fmt.Errorf("push reply identity mismatch")
			}
			// The acknowledged publish is final. A failed closing write cannot retry it.
			return attr(se, "type"), inner, nil
		}
	}
	return "", "", fmt.Errorf("push response limit exceeded")
}

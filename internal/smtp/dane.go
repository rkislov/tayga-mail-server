package smtp

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// daneResolver looks up TLSA records for outbound SMTP (RFC 6698 / 7672).
type daneResolver struct {
	timeout  time.Duration
	failOpen bool
	log      *slog.Logger
	exchange func(host string) ([]tlsaRecord, error)
}

type tlsaRecord struct {
	Usage        uint8
	Selector     uint8
	MatchingType uint8
	Certificate  []byte
}

func newDANEResolver(timeout time.Duration, failOpen bool, log *slog.Logger) *daneResolver {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	r := &daneResolver{timeout: timeout, failOpen: failOpen, log: log}
	r.exchange = r.lookupTLSA
	return r
}

// tlsConfigFor returns a tls.Config for STARTTLS. If TLSA records exist, DANE verification is required
// and plaintext fallback must not be used (caller checks daneRequired).
func (r *daneResolver) tlsConfigFor(serverName string) (cfg *tls.Config, daneRequired bool, err error) {
	if r == nil {
		return &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}, false, nil
	}
	recs, err := r.exchange(serverName)
	if err != nil {
		if r.failOpen {
			if r.log != nil {
				r.log.Warn("dane tlsa lookup failed; fail-open", "mx", serverName, "err", err)
			}
			return &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}, false, nil
		}
		return nil, false, fmt.Errorf("dane lookup: %w", err)
	}
	if len(recs) == 0 {
		return &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}, false, nil
	}

	onlyEE := true
	for _, rec := range recs {
		if rec.Usage != 3 {
			onlyEE = false
			break
		}
	}
	cfg = &tls.Config{
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
		// DANE-EE (usage 3): PKIX optional; we verify via TLSA in VerifyConnection.
		InsecureSkipVerify: onlyEE,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if err := matchTLSA(cs, recs); err != nil {
				return fmt.Errorf("dane: %w", err)
			}
			return nil
		},
	}
	return cfg, true, nil
}

func (r *daneResolver) lookupTLSA(host string) ([]tlsaRecord, error) {
	name := "_25._tcp." + dns.Fqdn(host)
	c := &dns.Client{Timeout: r.timeout, Net: "udp"}
	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeTLSA)
	m.RecursionDesired = true

	in, _, err := c.Exchange(m, dnsResolver())
	if err != nil {
		// Retry TCP
		c.Net = "tcp"
		in, _, err = c.Exchange(m, dnsResolver())
		if err != nil {
			return nil, err
		}
	}
	if in.Rcode == dns.RcodeNameError {
		return nil, nil
	}
	if in.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("tlsa rcode %s", dns.RcodeToString[in.Rcode])
	}
	var out []tlsaRecord
	for _, rr := range in.Answer {
		tlsa, ok := rr.(*dns.TLSA)
		if !ok {
			continue
		}
		cert, err := hex.DecodeString(tlsa.Certificate)
		if err != nil {
			continue
		}
		out = append(out, tlsaRecord{
			Usage: tlsa.Usage, Selector: tlsa.Selector,
			MatchingType: tlsa.MatchingType, Certificate: cert,
		})
	}
	return out, nil
}

func dnsResolver() string {
	cfg, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil || len(cfg.Servers) == 0 {
		return "1.1.1.1:53"
	}
	return netJoinHostPort(cfg.Servers[0], cfg.Port)
}

func netJoinHostPort(host, port string) string {
	if port == "" {
		port = "53"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}

func matchTLSA(cs tls.ConnectionState, recs []tlsaRecord) error {
	if len(cs.PeerCertificates) == 0 {
		return fmt.Errorf("no peer certificate")
	}
	for _, rec := range recs {
		switch rec.Usage {
		case 3: // DANE-EE
			if matchTLSACert(cs.PeerCertificates[0], rec) {
				return nil
			}
		case 1: // PKIX-EE — PKIX already validated (unless InsecureSkipVerify)
			if matchTLSACert(cs.PeerCertificates[0], rec) {
				return nil
			}
		case 2, 0: // DANE-TA / PKIX-TA — match any cert in chain as trust anchor material
			for _, cert := range cs.PeerCertificates {
				if matchTLSACert(cert, rec) {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("no matching TLSA record")
}

func matchTLSACert(cert *x509.Certificate, rec tlsaRecord) bool {
	var data []byte
	switch rec.Selector {
	case 0: // full cert
		data = cert.Raw
	case 1: // SubjectPublicKeyInfo
		data = cert.RawSubjectPublicKeyInfo
	default:
		return false
	}
	switch rec.MatchingType {
	case 0: // exact
		return bytesEqual(data, rec.Certificate)
	case 1: // SHA-256
		sum := sha256.Sum256(data)
		return bytesEqual(sum[:], rec.Certificate)
	case 2: // SHA-512
		sum := sha512.Sum512(data)
		return bytesEqual(sum[:], rec.Certificate)
	default:
		return false
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

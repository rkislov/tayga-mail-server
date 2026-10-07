package smtp

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log/slog"
	"os"
	"strings"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/rest-mail/go-arc"
	godkim "github.com/rest-mail/go-dkim"
	"github.com/tayga/tms/internal/scan"
)

type arcPolicy struct {
	verify     bool
	seal       bool
	action     string // tag | reject
	failOpen   bool
	authservID string
	domain     string
	selector   string
	key        *rsa.PrivateKey
	resolver   godkim.TXTResolver // nil → system DNS; injectable in tests
	log        *slog.Logger
}

func newARCPolicy(verify, seal bool, action string, failOpen bool, authservID, domain, selector, keyFile string, log *slog.Logger) (*arcPolicy, error) {
	p := &arcPolicy{
		verify: verify, seal: seal, action: action, failOpen: failOpen,
		authservID: authservID, domain: strings.ToLower(domain), selector: selector, log: log,
	}
	if action == "" {
		p.action = "tag"
	}
	if !p.verify && !p.seal {
		p.verify = true
	}
	if p.seal {
		raw, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("arc key: %w", err)
		}
		key, err := godkim.ParsePrivateKey(string(raw))
		if err != nil {
			return nil, fmt.Errorf("arc key: %w", err)
		}
		p.key = key
	}
	return p, nil
}

func (p *arcPolicy) apply(ctx context.Context, data []byte) ([]byte, error) {
	if p == nil {
		return data, nil
	}
	cv := "none"
	reason := ""
	if p.verify {
		cv, reason = arc.Verify(ctx, data, p.resolver)
		ar := p.authservID + "; arc=" + cv
		if reason != "" && cv != "pass" && cv != "none" {
			ar += " reason=\"" + sanitizeARReason(reason) + "\""
		}
		data = scan.InjectHeader(data, "Authentication-Results", ar)
		if cv == "fail" && p.action == "reject" {
			return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 29}, Message: "ARC validation failed"}
		}
	}
	if p.seal {
		authResults := collectAuthResultsForARC(data, p.authservID)
		res, err := arc.Seal(ctx, data, arc.SealOptions{
			Domain:      p.domain,
			Selector:    p.selector,
			PrivateKey:  p.key,
			AuthResults: authResults,
			Resolver:    p.resolver,
			Headers:     []string{"from", "to", "subject", "date", "message-id"},
		})
		if err != nil {
			if p.failOpen {
				if p.log != nil {
					p.log.Warn("arc seal failed; fail-open", "err", err)
				}
				return data, nil
			}
			return nil, &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 7, 0}, Message: "ARC seal unavailable"}
		}
		data = res.Message
		if p.log != nil {
			p.log.Debug("arc sealed", "i", res.Instance, "cv", res.ChainValidation)
		}
	}
	_ = reason
	return data, nil
}

// collectAuthResultsForARC builds the AAR payload (without i=) from injected Authentication-Results.
func collectAuthResultsForARC(data []byte, authservID string) string {
	id := strings.TrimSpace(authservID)
	if id == "" {
		id = "localhost"
	}
	var parts []string
	parts = append(parts, id)
	for _, line := range extractAuthResultsBodies(data) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Strip leading authserv-id if present.
		if i := strings.IndexByte(line, ';'); i >= 0 {
			rest := strings.TrimSpace(line[i+1:])
			if rest != "" {
				parts = append(parts, rest)
			}
			continue
		}
		parts = append(parts, line)
	}
	if len(parts) == 1 {
		return id + "; none"
	}
	return strings.Join(parts, "; ")
}

func extractAuthResultsBodies(data []byte) []string {
	const name = "authentication-results:"
	var out []string
	i := 0
	for i < len(data) {
		// Find end of headers.
		end := len(data)
		if j := indexCRLFCRLF(data); j >= 0 {
			end = j
		}
		chunk := data[:end]
		lower := strings.ToLower(string(chunk))
		off := 0
		for {
			idx := strings.Index(lower[off:], name)
			if idx < 0 {
				break
			}
			start := off + idx + len(name)
			// Unfold continued lines until blank or non-ws start.
			var b strings.Builder
			pos := start
			raw := string(chunk)
			for pos < len(raw) {
				nl := strings.IndexAny(raw[pos:], "\r\n")
				if nl < 0 {
					b.WriteString(strings.TrimSpace(raw[pos:]))
					break
				}
				line := raw[pos : pos+nl]
				b.WriteString(strings.TrimSpace(line))
				pos += nl
				for pos < len(raw) && (raw[pos] == '\r' || raw[pos] == '\n') {
					pos++
				}
				if pos >= len(raw) {
					break
				}
				// Continuation?
				if raw[pos] == ' ' || raw[pos] == '\t' {
					b.WriteByte(' ')
					continue
				}
				break
			}
			out = append(out, b.String())
			off = start
			if off >= len(lower) {
				break
			}
		}
		break
	}
	return out
}

func indexCRLFCRLF(data []byte) int {
	if i := strings.Index(string(data), "\r\n\r\n"); i >= 0 {
		return i
	}
	return strings.Index(string(data), "\n\n")
}

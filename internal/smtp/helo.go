package smtp

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"unicode"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/scan"
)

type heloPolicy struct {
	action      string // tag | reject
	requireFQDN bool
	authservID  string
	log         *slog.Logger
}

func (p *heloPolicy) apply(helo string, data []byte) ([]byte, error) {
	if p == nil {
		return data, nil
	}
	result, reason := evaluateHelo(helo, p.requireFQDN)
	ar := fmt.Sprintf("%s; helo=%s smtp.helo=%s", p.authservID, result, sanitizeARToken(helo))
	if result == "fail" && reason != "" {
		ar += " reason=\"" + sanitizeARReason(reason) + "\""
	}
	data = scan.InjectHeader(data, "Authentication-Results", ar)
	if result == "fail" && p.action == "reject" {
		return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 1}, Message: "Invalid HELO/EHLO"}
	}
	return data, nil
}

func evaluateHelo(helo string, requireFQDN bool) (result, reason string) {
	helo = strings.TrimSpace(helo)
	if helo == "" {
		return "fail", "empty"
	}
	// Reject obvious garbage / address literals we don't accept as FQDN.
	if strings.HasPrefix(helo, "[") && strings.HasSuffix(helo, "]") {
		inner := helo[1 : len(helo)-1]
		if ip := net.ParseIP(inner); ip != nil {
			return "pass", ""
		}
		return "fail", "bad-address-literal"
	}
	lower := strings.ToLower(helo)
	if lower == "localhost" || lower == "localhost.localdomain" {
		return "fail", "localhost"
	}
	if ip := net.ParseIP(helo); ip != nil {
		return "fail", "bare-ip"
	}
	if !isDNSHostname(helo) {
		return "fail", "syntax"
	}
	if requireFQDN && !strings.Contains(helo, ".") {
		return "fail", "not-fqdn"
	}
	return "pass", ""
}

func isDNSHostname(s string) bool {
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	labels := strings.Split(s, ".")
	for _, lab := range labels {
		if lab == "" || len(lab) > 63 {
			return false
		}
		if lab[0] == '-' || lab[len(lab)-1] == '-' {
			return false
		}
		for i := 0; i < len(lab); i++ {
			c := lab[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
				continue
			}
			return false
		}
	}
	return true
}

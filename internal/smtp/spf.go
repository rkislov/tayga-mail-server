package smtp

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	"blitiri.com.ar/go/spf"
	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/scan"
)

type spfPolicy struct {
	action     string // tag | reject
	failOpen   bool
	authservID string
	check      func(ip net.IP, helo, sender string) (spf.Result, error)
	log        *slog.Logger
}

func (p *spfPolicy) apply(ipStr, helo, mailFrom string, data []byte) ([]byte, spf.Result, error) {
	if p == nil {
		return data, spf.None, nil
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		data = scan.InjectHeader(data, "Authentication-Results", p.authservID+"; spf=none smtp.mailfrom="+sanitizeARToken(mailFrom))
		return data, spf.None, nil
	}
	sender := mailFrom
	if sender == "" {
		sender = "postmaster@" + helo
	}
	check := p.check
	if check == nil {
		check = func(ip net.IP, helo, sender string) (spf.Result, error) {
			return spf.CheckHostWithSender(ip, helo, sender)
		}
	}
	res, err := check(ip, helo, sender)
	if err != nil && p.log != nil {
		p.log.Debug("spf check", "result", res, "err", err, "ip", ipStr, "from", sender)
	}
	ar := fmt.Sprintf("%s; spf=%s smtp.mailfrom=%s", p.authservID, string(res), sanitizeARToken(sender))
	if ipStr != "" {
		ar += " smtp.remote-ip=" + sanitizeARToken(ipStr)
	}
	data = scan.InjectHeader(data, "Authentication-Results", ar)

	switch res {
	case spf.TempError:
		if !p.failOpen {
			return nil, res, &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 7, 24}, Message: "SPF temporary failure"}
		}
	case spf.Fail:
		if p.action == "reject" {
			return nil, res, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 23}, Message: "SPF validation failed"}
		}
	}
	return data, res, nil
}

func spfNoneResult() spf.Result { return spf.None }

func sanitizeARToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ";", "")
	s = strings.ReplaceAll(s, "\"", "")
	if len(s) > 200 {
		s = s[:200]
	}
	if s == "" {
		return "<>"
	}
	return s
}

package smtp

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/scan"
)

type iprevPolicy struct {
	action     string // tag | reject
	failOpen   bool
	authservID string
	lookupAddr func(ip string) ([]string, error)
	lookupIP   func(host string) ([]net.IP, error)
	log        *slog.Logger
}

func (p *iprevPolicy) apply(ipStr string, data []byte) ([]byte, error) {
	if p == nil {
		return data, nil
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		data = scan.InjectHeader(data, "Authentication-Results", p.authservID+"; iprev=none")
		return data, nil
	}

	lookupAddr := p.lookupAddr
	if lookupAddr == nil {
		lookupAddr = net.LookupAddr
	}
	lookupIP := p.lookupIP
	if lookupIP == nil {
		lookupIP = net.LookupIP
	}

	ptrs, err := lookupAddr(ip.String())
	if err != nil {
		if isDNSNotFound(err) {
			data = scan.InjectHeader(data, "Authentication-Results",
				fmt.Sprintf("%s; iprev=fail policy.iprev=fail smtp.remote-ip=%s", p.authservID, sanitizeARToken(ipStr)))
			if p.action == "reject" {
				return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 25}, Message: "Reverse DNS required"}
			}
			return data, nil
		}
		if p.log != nil {
			p.log.Debug("iprev lookup", "ip", ipStr, "err", err)
		}
		if !p.failOpen {
			return nil, &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 7, 25}, Message: "Reverse DNS temporary failure"}
		}
		data = scan.InjectHeader(data, "Authentication-Results",
			fmt.Sprintf("%s; iprev=temperror smtp.remote-ip=%s", p.authservID, sanitizeARToken(ipStr)))
		return data, nil
	}
	if len(ptrs) == 0 {
		data = scan.InjectHeader(data, "Authentication-Results",
			fmt.Sprintf("%s; iprev=fail policy.iprev=fail smtp.remote-ip=%s", p.authservID, sanitizeARToken(ipStr)))
		if p.action == "reject" {
			return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 25}, Message: "Reverse DNS required"}
		}
		return data, nil
	}

	ptrHost := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(ptrs[0])), ".")
	fwd, err := lookupIP(ptrHost)
	if err != nil {
		if !p.failOpen && !isDNSNotFound(err) {
			return nil, &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 7, 25}, Message: "Forward DNS temporary failure"}
		}
		data = scan.InjectHeader(data, "Authentication-Results",
			fmt.Sprintf("%s; iprev=fail policy.iprev=fail smtp.remote-ip=%s policy.ptr=%s",
				p.authservID, sanitizeARToken(ipStr), sanitizeARToken(ptrHost)))
		if p.action == "reject" {
			return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 25}, Message: "Forward-confirmed reverse DNS failed"}
		}
		return data, nil
	}
	confirmed := false
	for _, fip := range fwd {
		if fip.Equal(ip) {
			confirmed = true
			break
		}
	}
	if confirmed {
		data = scan.InjectHeader(data, "Authentication-Results",
			fmt.Sprintf("%s; iprev=pass smtp.remote-ip=%s policy.iprev=pass policy.ptr=%s",
				p.authservID, sanitizeARToken(ipStr), sanitizeARToken(ptrHost)))
		return data, nil
	}
	data = scan.InjectHeader(data, "Authentication-Results",
		fmt.Sprintf("%s; iprev=fail policy.iprev=fail smtp.remote-ip=%s policy.ptr=%s",
			p.authservID, sanitizeARToken(ipStr), sanitizeARToken(ptrHost)))
	if p.action == "reject" {
		return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 25}, Message: "Forward-confirmed reverse DNS failed"}
	}
	return data, nil
}

func isDNSNotFound(err error) bool {
	if err == nil {
		return false
	}
	if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no such host") || strings.Contains(s, "not found")
}

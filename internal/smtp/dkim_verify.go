package smtp

import (
	"bytes"
	"log/slog"
	"strings"

	"github.com/emersion/go-msgauth/dkim"
	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/scan"
)

type dkimVerifyPolicy struct {
	action           string // tag | reject
	requireSignature bool
	failOpen         bool
	authservID       string
	lookupTXT        func(domain string) ([]string, error) // nil → net.LookupTXT
	maxVerifications int
	log              *slog.Logger
}

func (p *dkimVerifyPolicy) apply(data []byte) ([]byte, error) {
	if p == nil {
		return data, nil
	}
	opts := &dkim.VerifyOptions{
		LookupTXT:        p.lookupTXT,
		MaxVerifications: p.maxVerifications,
	}
	if opts.MaxVerifications <= 0 {
		opts.MaxVerifications = 5
	}
	vers, err := dkim.VerifyWithOptions(bytes.NewReader(data), opts)
	if err != nil && !dkim.IsTempFail(err) && !dkim.IsPermFail(err) {
		// e.g. ErrTooManySignatures — still may have partial results
		if p.log != nil {
			p.log.Warn("dkim verify", "err", err)
		}
	}
	if err != nil && dkim.IsTempFail(err) {
		if !p.failOpen {
			return nil, &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 7, 5}, Message: "DKIM temporary failure"}
		}
		data = scan.InjectHeader(data, "Authentication-Results", p.authservID+"; dkim=temperror")
		return data, nil
	}

	ar := formatAuthResults(p.authservID, vers)
	data = scan.InjectHeader(data, "Authentication-Results", ar)

	pass, fail, temperror := summarizeDKIM(vers)
	if temperror && !pass && !fail {
		if !p.failOpen {
			return nil, &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 7, 5}, Message: "DKIM temporary failure"}
		}
	}
	if p.requireSignature && len(vers) == 0 {
		if p.action == "reject" {
			return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 20}, Message: "DKIM signature required"}
		}
	}
	if fail && p.action == "reject" {
		return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 20}, Message: "DKIM verification failed"}
	}
	_ = pass
	return data, nil
}

func summarizeDKIM(vers []*dkim.Verification) (pass, fail, temperror bool) {
	for _, v := range vers {
		if v == nil {
			continue
		}
		if v.Err == nil {
			pass = true
			continue
		}
		if dkim.IsTempFail(v.Err) {
			temperror = true
			continue
		}
		fail = true
	}
	return
}

func formatAuthResults(authservID string, vers []*dkim.Verification) string {
	id := strings.TrimSpace(authservID)
	if id == "" {
		id = "localhost"
	}
	if len(vers) == 0 {
		return id + "; dkim=none"
	}
	var parts []string
	parts = append(parts, id)
	for _, v := range vers {
		if v == nil {
			continue
		}
		result := "pass"
		reason := ""
		if v.Err != nil {
			switch {
			case dkim.IsTempFail(v.Err):
				result = "temperror"
			case dkim.IsPermFail(v.Err):
				result = "permerror"
			default:
				result = "fail"
			}
			reason = sanitizeARReason(v.Err.Error())
		}
		seg := "dkim=" + result
		if reason != "" {
			seg += " reason=\"" + reason + "\""
		}
		if v.Domain != "" {
			seg += " header.d=" + v.Domain
		}
		if v.Identifier != "" {
			seg += " header.i=" + v.Identifier
		}
		parts = append(parts, seg)
	}
	return strings.Join(parts, "; ")
}

func sanitizeARReason(s string) string {
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

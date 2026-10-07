package smtp

import (
	"context"
	"fmt"
	"log/slog"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/scan"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
)

type scanPolicy struct {
	scanner  scan.Scanner
	action   scan.Action
	folder   string
	failOpen bool
	log      *slog.Logger
}

func (p *scanPolicy) apply(ctx context.Context, u *storage.User, data []byte, msgid string, deliverQuarantine func(context.Context, *storage.User, []byte, string) error) ([]byte, error) {
	if p == nil || p.scanner == nil {
		return data, nil
	}
	res, err := p.scanner.Scan(ctx, data)
	if err != nil {
		if p.failOpen {
			if p.log != nil {
				p.log.Warn("virus scan failed; fail-open", "err", err, "user", u.Email)
			}
			return data, nil
		}
		return nil, &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "Virus scanner unavailable"}
	}
	if res == nil || res.Clean {
		return data, nil
	}
	virus := res.Virus
	if virus == "" {
		virus = "unknown"
	}
	if p.log != nil {
		p.log.Warn("virus detected", "user", u.Email, "virus", virus, "scanner", res.Scanner, "action", p.action)
	}
	switch p.action {
	case scan.ActionReject:
		return nil, &gosmtp.SMTPError{
			Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 1},
			Message: fmt.Sprintf("Message rejected: malware detected (%s)", virus),
		}
	case scan.ActionTag:
		data = scan.InjectHeader(data, "X-Virus-Status", "Infected")
		data = scan.InjectHeader(data, "X-Virus-Name", virus)
		return data, nil
	default: // quarantine
		qdata := scan.InjectHeader(data, "X-Virus-Status", "Quarantined")
		qdata = scan.InjectHeader(qdata, "X-Virus-Name", virus)
		if err := deliverQuarantine(ctx, u, qdata, msgid); err != nil {
			return nil, err
		}
		return nil, errQuarantined
	}
}

// errQuarantined signals deliver() that the message was filed and should not go to sieve/INBOX.
var errQuarantined = fmt.Errorf("message quarantined")

func (s *session) deliverQuarantine(ctx context.Context, u *storage.User, data []byte, msgid string) error {
	folder := "Quarantine"
	if s.backend.scan != nil && s.backend.scan.folder != "" {
		folder = s.backend.scan.folder
	}
	if s.backend.sieve != nil {
		return s.backend.sieve.FileInto(ctx, u, folder, nil, data, msgid)
	}
	// Fallback without sieve engine: reuse minimal path via temporary engine-like call.
	eng := &sieve.Engine{Store: s.backend.store, Mailstore: s.backend.mailstore, Log: s.backend.log}
	return eng.FileInto(ctx, u, folder, nil, data, msgid)
}

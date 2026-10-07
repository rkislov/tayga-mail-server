package smtp

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/scan"
	"github.com/tayga/tms/internal/spam"
	"github.com/tayga/tms/internal/storage"
)

type spamPolicy struct {
	checker spam.Checker
	cfg     spam.Config
	log     *slog.Logger
}

func (p *spamPolicy) apply(ctx context.Context, u *storage.User, meta spam.Meta, data []byte, msgid string, deliverJunk func(context.Context, *storage.User, string, []byte, string) error) ([]byte, error) {
	if p == nil || p.checker == nil {
		return data, nil
	}
	res, err := p.checker.Check(ctx, meta, data)
	if err != nil {
		if p.cfg.FailOpen {
			if p.log != nil {
				p.log.Warn("spam check failed; fail-open", "err", err, "user", u.Email)
			}
			return data, nil
		}
		return nil, &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "Spam filter unavailable"}
	}
	action := spam.MapAction(p.cfg, res)
	score := "0"
	if res != nil {
		score = strconv.FormatFloat(res.Score, 'f', 2, 64)
	}
	switch action {
	case spam.ActionPass:
		return data, nil
	case spam.ActionTag:
		data = scan.InjectHeader(data, "X-Spam-Status", "Yes")
		data = scan.InjectHeader(data, "X-Spam-Score", score)
		if res != nil && res.Action != "" {
			data = scan.InjectHeader(data, "X-Spam-Action", res.Action)
		}
		return data, nil
	case spam.ActionGreylist:
		return nil, &gosmtp.SMTPError{Code: 421, EnhancedCode: gosmtp.EnhancedCode{4, 7, 1}, Message: "Try again later (greylisted)"}
	case spam.ActionReject:
		return nil, &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 1}, Message: fmt.Sprintf("Message rejected as spam (score %s)", score)}
	default: // quarantine
		if p.log != nil {
			p.log.Warn("spam quarantined", "user", u.Email, "score", score)
		}
		folder := p.cfg.Folder
		if folder == "" {
			folder = "Junk"
		}
		qdata := scan.InjectHeader(data, "X-Spam-Status", "Quarantined")
		qdata = scan.InjectHeader(qdata, "X-Spam-Score", score)
		if err := deliverJunk(ctx, u, folder, qdata, msgid); err != nil {
			return nil, err
		}
		return nil, errQuarantined
	}
}

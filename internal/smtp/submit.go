package smtp

import (
	"bytes"
	"context"
	"fmt"
	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/storage"
)

// SubmitAuthenticated reuses SMTP submission policy, quotas, scanning, routing,
// DKIM and the outbound queue for an already-authenticated protocol caller.
func (s *Server) SubmitAuthenticated(ctx context.Context, u *storage.User, recipients []string, raw []byte) error {
	if s.submission == nil || u == nil || !u.Enabled {
		return fmt.Errorf("submission unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(recipients) == 0 || len(recipients) > 100 {
		return fmt.Errorf("invalid recipient count")
	}
	session := &session{backend: s.submission, user: u, remote: "127.0.0.1:0", helo: s.cfg.Server.Hostname}
	if err := session.Mail(u.Email, &gosmtp.MailOptions{}); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := session.Rcpt(recipient, nil); err != nil {
			return err
		}
	}
	return session.Data(bytes.NewReader(raw))
}

package smtp

import (
	"bytes"
	"context"
	"fmt"
	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/storage"
	"net/mail"
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
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	from, err := message.Header.AddressList("From")
	if err != nil || len(from) != 1 || len(message.Header["From"]) != 1 {
		return fmt.Errorf("invalid sender")
	}
	session := &session{backend: s.submission, user: u, remote: "127.0.0.1:0", helo: s.cfg.Server.Hostname}
	if err := session.Mail(from[0].Address, &gosmtp.MailOptions{}); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := session.Rcpt(recipient, nil); err != nil {
			return err
		}
	}
	return session.Data(bytes.NewReader(raw))
}

package storage

import (
	"context"
	"errors"
	"net/mail"
	"strings"
)

var ErrDelegateExists = errors.New("delegate already exists")

// AuthorizeMailSender resolves an active sending identity on every submission.
// The mode is explicit: send-as must never silently become send-on-behalf.
func AuthorizeMailSender(ctx context.Context, st Driver, actorID, from, mode string) (*User, error) {
	if mode != "" && mode != "send_as" && mode != "on_behalf" {
		return nil, ErrUnauthorized
	}
	actor, err := st.GetUserByID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	owner, err := st.ResolveRecipient(ctx, strings.ToLower(strings.TrimSpace(from)))
	if err != nil {
		return nil, err
	}
	if !actor.Enabled || !owner.Enabled || actor.AuthSource == "resource" || owner.AuthSource == "resource" || actor.TenantID != owner.TenantID {
		return nil, ErrUnauthorized
	}
	if owner.ID == actor.ID {
		if mode == "on_behalf" {
			return nil, ErrUnauthorized
		}
		return owner, nil
	}
	grant, err := st.GetMailboxDelegate(ctx, owner.ID, actorID)
	if err != nil {
		return nil, err
	}
	if mode == "on_behalf" && grant.CanSendOnBehalf || mode != "on_behalf" && grant.CanSendAs {
		return owner, nil
	}
	return nil, ErrUnauthorized
}

// AuthorizeMessageSender validates RFC 5322 Sender as the authenticated actor.
// A different From requires either send-as or an explicit on-behalf Sender.
func AuthorizeMessageSender(ctx context.Context, st Driver, actor *User, header mail.Header) error {
	from, err := header.AddressList("From")
	if err != nil || len(from) != 1 || len(header["From"]) != 1 {
		return ErrUnauthorized
	}
	mode := "send_as"
	if len(header["Sender"]) > 0 {
		sender, err := header.AddressList("Sender")
		if err != nil || len(sender) != 1 || len(header["Sender"]) != 1 || !strings.EqualFold(sender[0].Address, actor.Email) {
			return ErrUnauthorized
		}
		target, err := st.ResolveRecipient(ctx, from[0].Address)
		if err != nil {
			return err
		}
		if target.ID != actor.ID {
			mode = "on_behalf"
		}
	}
	_, err = AuthorizeMailSender(ctx, st, actor.ID, from[0].Address, mode)
	return err
}

// Envelope authorization is provisional; DATA rechecks the exact header mode.
func CanSubmitFrom(ctx context.Context, st Driver, actorID, from string) (bool, error) {
	_, err := AuthorizeMailSender(ctx, st, actorID, from, "send_as")
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFound) {
		_, err = AuthorizeMailSender(ctx, st, actorID, from, "on_behalf")
	}
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

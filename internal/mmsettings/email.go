package mmsettings

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
)

// EmailOverrides stages routing only. This configuration has no delivery switch;
// saving a draft cannot start forwarding or send notifications.
type EmailOverrides struct {
	IncomingAddress   *string `json:"incoming_address,omitempty"`
	IncomingChannelID *string `json:"incoming_channel_id,omitempty"`
	OutgoingFrom      *string `json:"outgoing_from,omitempty"`
	SubjectPrefix     *string `json:"subject_prefix,omitempty"`
}
type EmailEffective struct {
	IncomingAddress   string `json:"incoming_address"`
	IncomingChannelID string `json:"incoming_channel_id"`
	OutgoingFrom      string `json:"outgoing_from"`
	SubjectPrefix     string `json:"subject_prefix"`
	IncomingEnabled   bool   `json:"incoming_enabled"`
	OutgoingEnabled   bool   `json:"outgoing_enabled"`
	UserOptInRequired bool   `json:"user_opt_in_required"`
	RecipientPolicy   string `json:"recipient_policy"`
}

func applyEmail(result *Effective, value *EmailOverrides, scope Scope) {
	if value == nil {
		return
	}
	fields := []struct {
		key string
		src *string
		dst *string
	}{
		{"incoming_address", value.IncomingAddress, &result.Email.IncomingAddress},
		{"incoming_channel_id", value.IncomingChannelID, &result.Email.IncomingChannelID},
		{"outgoing_from", value.OutgoingFrom, &result.Email.OutgoingFrom},
		{"subject_prefix", value.SubjectPrefix, &result.Email.SubjectPrefix},
	}
	for _, field := range fields {
		if field.src != nil {
			*field.dst = *field.src
			result.Sources["email."+field.key] = scope.Key()
		}
	}
}
func (s *Service) validateEmail(ctx context.Context, scope Scope, value *EmailOverrides) error {
	if value == nil {
		return nil
	}
	if value.SubjectPrefix != nil && (len(*value.SubjectPrefix) > 80 || strings.ContainsAny(*value.SubjectPrefix, "\r\n\x00")) {
		return fmt.Errorf("invalid email subject prefix")
	}
	// Concrete routing is domain-bound. Parent scopes can set presentation defaults
	// without accidentally routing a child domain's mail into another domain.
	for _, field := range []*string{value.IncomingAddress, value.IncomingChannelID, value.OutgoingFrom} {
		if field != nil && *field != "" && scope.DomainID == "" {
			return fmt.Errorf("email routes must be configured in a domain scope")
		}
	}
	if scope.DomainID == "" {
		return nil
	}
	domain, err := s.Store.GetDomainByID(ctx, scope.DomainID)
	if err != nil {
		return err
	}
	for _, field := range []*string{value.IncomingAddress, value.OutgoingFrom} {
		if field == nil || *field == "" {
			continue
		}
		address, e := mail.ParseAddress(*field)
		if e != nil || address.Address != *field || len(*field) > 254 || strings.ContainsAny(*field, "\r\n\x00") {
			return fmt.Errorf("invalid email address")
		}
		_, host, ok := strings.Cut(address.Address, "@")
		if !ok || !strings.EqualFold(host, domain.Name) {
			return fmt.Errorf("email address must belong to the selected domain")
		}
	}
	if value.IncomingChannelID != nil && *value.IncomingChannelID != "" {
		id := *value.IncomingChannelID
		if len(id) != 26 || strings.Trim(id, "abcdefghijklmnopqrstuvwxyz234567") != "" {
			return fmt.Errorf("invalid incoming channel id")
		}
		var tenant string
		if err = s.Store.DB().QueryRowContext(ctx, "SELECT tenant_id FROM chat_channels WHERE id=$1", id).Scan(&tenant); err != nil || tenant != scope.TenantID {
			return fmt.Errorf("incoming channel must belong to the selected tenant")
		}
	}
	return nil
}

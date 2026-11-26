package httpapi

import (
	"context"
	"net/url"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/vks"
)

func (s *Server) mattermostChannelCallsAllowed(ctx context.Context, u *storage.User) (bool, error) {
	policy, err := s.vksPolicies().Resolve(ctx, vks.Scope{TenantID: u.TenantID, DomainID: u.DomainID})
	return vksAvailable(policy, s.cfgLive().VKS.Enabled), err
}

func (s *Server) startMattermostChannelCall(ctx context.Context, u *storage.User, channelID string) (string, error) {
	cfg := s.cfgLive()
	policy, err := s.vksPolicies().Resolve(ctx, vks.Scope{TenantID: u.TenantID, DomainID: u.DomainID})
	if err != nil {
		return "", err
	}
	if !vksAvailable(policy, cfg.VKS.Enabled) {
		return "", chat.ErrForbidden
	}
	if policy.Provider == vks.ProviderJitsi {
		meetURL, err := url.Parse(cfg.VKS.PublicURL)
		if err != nil || meetURL.Scheme != "https" || meetURL.Hostname() == "" || cfg.VKS.KeyID == "" {
			return "", chat.ErrInvalid
		}
		if _, err := vks.LoadSigningKey(cfg.VKS.SigningKeyFile); err != nil {
			return "", err
		}
	}
	link, err := url.Parse(cfg.HTTP.PublicURL)
	if err != nil || link.Scheme != "https" || link.Hostname() == "" {
		return "", chat.ErrInvalid
	}
	meeting, err := (vks.Meetings{Store: s.store, Policy: policy}).CreateForChannel(ctx, u, channelID)
	if err != nil {
		return "", err
	}
	link.Path = "/"
	link.RawPath = ""
	link.Fragment = ""
	link.RawQuery = url.Values{"app": {"vks"}, "room": {meeting.Room}}.Encode()
	return link.String(), nil
}

func (s *Server) allowsVKSMeeting(ctx context.Context, meeting vks.Meeting, u *storage.User) bool {
	if !(vks.Meetings{Store: s.store}).CanJoin(ctx, meeting, u) {
		return false
	}
	policy, err := s.vksPolicies().Resolve(ctx, vks.Scope{TenantID: meeting.TenantID, DomainID: meeting.DomainID})
	return err == nil && vksAvailable(policy, s.cfgLive().VKS.Enabled) && policy.Provider == meeting.Backend() &&
		(meeting.Backend() != vks.ProviderKontur || policy.KonturURL == meeting.KonturURL)
}

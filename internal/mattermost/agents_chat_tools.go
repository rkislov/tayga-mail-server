package mattermost

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

type agentChatChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type agentChatPost struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channel_id"`
	UserID    string    `json:"user_id"`
	RootID    string    `json:"root_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Text      string    `json:"text"`
}

func (s *API) agentChatChannels(ctx context.Context, u *storage.User) (any, error) {
	channels, err := s.agentRecentChannels(ctx, u)
	if err != nil {
		return nil, err
	}
	const channelLimit = 100
	out := make([]agentChatChannel, 0, min(channelLimit, len(channels)))
	for _, channel := range channels[:min(channelLimit, len(channels))] {
		name := channel.DisplayName
		if channel.Kind == "D" || channel.Kind == "G" {
			members, err := s.chat.Members(ctx, u, channel.ID)
			if errors.Is(err, storage.ErrNotFound) || errors.Is(err, chat.ErrForbidden) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if containsTaygaAgent(members) {
				continue
			}
			names := make([]string, 0, min(len(members), 8))
			for _, m := range members {
				if m.UserID == u.ID {
					continue
				}
				peer, err := s.store.GetUserByID(ctx, m.UserID)
				if err != nil || peer.DomainID != u.DomainID || peer.TenantID != u.TenantID {
					continue
				}
				label := peer.DisplayName
				if label == "" {
					label = peer.LocalPart
				}
				names = append(names, clipAgentText(label, 80))
				if len(names) >= 8 {
					break
				}
			}
			if name == "" {
				name = strings.Join(names, ", ")
			}
		}
		if name == "" {
			name = channel.Name
		}
		out = append(out, agentChatChannel{ID: channel.ID, Name: clipAgentText(name, 240), Kind: channel.Kind})
	}
	return map[string]any{"channels": out, "incomplete": len(channels) > channelLimit}, nil
}

func (s *API) agentChatSearch(ctx context.Context, u *storage.User, channelID, query string) (any, error) {
	channel, err := s.chat.Channel(ctx, u, channelID)
	if err != nil {
		return nil, err
	}
	posts, err := s.agentReadablePosts(ctx, u, channel, 200)
	if err != nil {
		return nil, err
	}
	matches := matchingAgentPosts(posts, query)
	return map[string]any{"messages": matches[:min(len(matches), 10)], "scanned": len(posts)}, nil
}

func (s *API) agentChatSearchRecent(ctx context.Context, u *storage.User, query string) (any, error) {
	if strings.TrimSpace(query) == "" {
		return nil, chat.ErrInvalid
	}
	channels, err := s.agentRecentChannels(ctx, u)
	if err != nil {
		return nil, err
	}
	const channelLimit, channelPostLimit = 12, 50
	matches := make([]agentChatPost, 0)
	for _, ch := range channels[:min(len(channels), channelLimit)] {
		posts, err := s.agentReadablePosts(ctx, u, ch, channelPostLimit)
		if errors.Is(err, storage.ErrNotFound) || errors.Is(err, chat.ErrForbidden) {
			continue
		}
		if err != nil {
			return nil, err
		}
		matches = append(matches, matchingAgentPosts(posts, query)...)
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].CreatedAt.After(matches[j].CreatedAt) })
	return map[string]any{
		"messages":     matches[:min(len(matches), 10)],
		"max_channels": channelLimit, "max_messages_per_channel": channelPostLimit,
	}, nil
}

func (s *API) agentRecentChannels(ctx context.Context, u *storage.User) ([]chat.Channel, error) {
	channels, err := s.chat.Channels(ctx, u)
	sort.SliceStable(channels, func(i, j int) bool { return channels[i].LastPostAt > channels[j].LastPostAt })
	return channels, err
}

func (s *API) agentReadablePosts(ctx context.Context, u *storage.User, channel chat.Channel, limit int) ([]chat.Post, error) {
	if channel.Kind == "D" || channel.Kind == "G" {
		members, err := s.chat.Members(ctx, u, channel.ID)
		if err != nil {
			return nil, err
		}
		if containsTaygaAgent(members) {
			return nil, chat.ErrForbidden
		}
	}
	return s.chat.Posts(ctx, u, channel.ID, limit, 0, 0)
}

func containsTaygaAgent(members []chat.Member) bool {
	for _, m := range members {
		if strings.HasPrefix(m.UserID, "tms-agent:") {
			return true
		}
	}
	return false
}

func matchingAgentPosts(posts []chat.Post, query string) []agentChatPost {
	query = strings.ToLower(strings.TrimSpace(query))
	matches := make([]agentChatPost, 0, min(len(posts), 10))
	for _, p := range posts {
		if p.Type != "" || !strings.Contains(strings.ToLower(p.Message), query) {
			continue
		}
		matches = append(matches, agentChatPost{
			ID: p.ID, ChannelID: p.ChannelID, UserID: chat.ID(p.UserID), RootID: p.RootID,
			CreatedAt: time.UnixMilli(p.CreatedAt).UTC(), Text: clipAgentText(p.Message, 500),
		})
		if len(matches) >= 10 {
			break
		}
	}
	return matches
}

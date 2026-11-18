package mattermost

import (
	"context"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

type ChannelCallStarter func(ctx context.Context, u *storage.User, channelID string) (string, error)

func (s *API) SetChannelCallStarter(start ChannelCallStarter, allowed func(context.Context, *storage.User) (bool, error)) {
	s.startChannelCall = start
	s.canStartChannelCall = allowed
}

func (s *API) callCommand(ctx context.Context, u *storage.User, args model.CommandArgs) (*model.CommandResponse, error) {
	if s.startChannelCall == nil {
		return nil, chat.ErrForbidden
	}
	if strings.TrimSpace(args.Command) != "/meet" {
		return nil, chat.ErrInvalid
	}
	link, err := s.publishChannelCall(ctx, u, args.ChannelId)
	if err != nil {
		return nil, err
	}
	return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: "Ссылка на звонок отправлена в канал.", GotoLocation: link}, nil
}

func (s *API) publishChannelCall(ctx context.Context, u *storage.User, channelID string) (string, error) {
	link, err := s.startChannelCall(ctx, u, channelID)
	if err != nil {
		return "", err
	}
	message := "Групповой звонок Jitsi\n[Присоединиться к звонку](" + link + ")"
	if _, err := s.chat.SendRich(ctx, u, channelID, "", message, "", map[string]any{"tms_meeting_url": link}, ""); err != nil {
		return "", err
	}
	return link, nil
}

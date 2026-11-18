package mmplugins

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func (r *Runtime) bind(q string) string {
	if d, ok := r.options.Store.(interface{ DialectName() string }); ok && d.DialectName() == "postgres" {
		n := 0
		var b strings.Builder
		for _, c := range q {
			if c == '?' {
				n++
				b.WriteString("$" + strconv.Itoa(n))
			} else {
				b.WriteRune(c)
			}
		}
		return b.String()
	}
	return q
}
func (a *hostAPI) GetChannel(id string) (*model.Channel, *model.AppError) {
	var creator string
	err := a.runtime.options.Store.DB().QueryRowContext(context.Background(), a.runtime.bind("SELECT creator_id FROM chat_channels WHERE id=?"), id).Scan(&creator)
	if err != nil {
		return nil, appError(err)
	}
	u, err := a.runtime.options.Store.GetUserByID(context.Background(), creator)
	if err != nil {
		return nil, appError(err)
	}
	if a.runtime.options.TenantID != "" && u.TenantID != a.runtime.options.TenantID {
		return nil, appError(chat.ErrForbidden)
	}
	c, err := a.runtime.options.Chat.Channel(context.Background(), u, id)
	if err != nil {
		return nil, appError(err)
	}
	team := chat.ID(c.TenantID)
	if c.Kind == "D" {
		team = ""
	}
	return &model.Channel{Id: c.ID, TeamId: team, Name: c.Name, DisplayName: c.DisplayName, Type: model.ChannelType(c.Kind), CreatorId: chat.ID(c.CreatorID), CreateAt: c.CreatedAt, UpdateAt: c.CreatedAt, LastPostAt: c.LastPostAt, TotalMsgCount: c.TotalMsgCount, Props: model.StringInterface{}}, nil
}
func (a *hostAPI) GetChannelByName(teamID, name string, includeDeleted bool) (*model.Channel, *model.AppError) {
	tenants, err := a.runtime.options.Store.ListTenants(context.Background())
	if err != nil {
		return nil, appError(err)
	}
	for _, tenant := range tenants {
		if chat.ID(tenant.ID) != teamID {
			continue
		}
		if a.runtime.options.TenantID != "" && tenant.ID != a.runtime.options.TenantID {
			return nil, appError(chat.ErrForbidden)
		}
		var id string
		err = a.runtime.options.Store.DB().QueryRowContext(context.Background(), a.runtime.bind("SELECT id FROM chat_channels WHERE tenant_id=? AND name=?"), tenant.ID, name).Scan(&id)
		if err != nil {
			return nil, appError(err)
		}
		return a.GetChannel(id)
	}
	return nil, appError(storage.ErrNotFound)
}
func (a *hostAPI) CreatePost(p *model.Post) (*model.Post, *model.AppError) {
	if p == nil || len(p.FileIds) > 0 {
		return nil, model.NewAppError("CreatePost", "tms.plugin.file_upload_unsupported", nil, "File posts are not supported", 501)
	}
	u, err := a.user(p.UserId)
	if err != nil {
		return nil, appError(err)
	}
	c, appErr := a.GetChannel(p.ChannelId)
	if appErr != nil {
		return nil, appErr
	}
	if _, err = a.runtime.options.Chat.Channel(context.Background(), u, p.ChannelId); err != nil {
		if u.AuthSource != "bot" {
			return nil, appError(err)
		}
		if _, appErr = a.AddChannelMember(c.Id, p.UserId); appErr != nil {
			return nil, appErr
		}
	}
	post, err := a.runtime.options.Chat.SendRich(context.Background(), u, p.ChannelId, p.RootId, p.Message, p.PendingPostId, map[string]any(p.Props), p.Type)
	if err != nil {
		return nil, appError(err)
	}
	return pluginPost(post), nil
}
func pluginPost(p chat.Post) *model.Post {
	return &model.Post{Id: p.ID, UserId: chat.ID(p.UserID), ChannelId: p.ChannelID, RootId: p.RootID, Message: p.Message, PendingPostId: p.PendingID, CreateAt: p.CreatedAt, UpdateAt: p.CreatedAt, Props: model.StringInterface(p.Props), Type: p.Type}
}
func (a *hostAPI) GetPost(id string) (*model.Post, *model.AppError) {
	var channelID string
	err := a.runtime.options.Store.DB().QueryRowContext(context.Background(), a.runtime.bind("SELECT channel_id FROM chat_posts WHERE id=?"), id).Scan(&channelID)
	if err != nil {
		return nil, appError(err)
	}
	c, e := a.GetChannel(channelID)
	if e != nil {
		return nil, e
	}
	u, err := a.user(c.CreatorId)
	if err != nil {
		return nil, appError(err)
	}
	p, err := a.runtime.options.Chat.Post(context.Background(), u, id)
	if err != nil {
		return nil, appError(err)
	}
	return pluginPost(p), nil
}
func (a *hostAPI) AddChannelMember(channelID, userID string) (*model.ChannelMember, *model.AppError) {
	c, e := a.GetChannel(channelID)
	if e != nil {
		return nil, e
	}
	creator, err := a.user(c.CreatorId)
	if err != nil {
		return nil, appError(err)
	}
	if err = a.runtime.options.Chat.AddMember(context.Background(), creator, channelID, userID); err != nil {
		return nil, appError(err)
	}
	return &model.ChannelMember{ChannelId: channelID, UserId: userID, Roles: "channel_user", SchemeUser: true, NotifyProps: model.StringMap{}}, nil
}
func (a *hostAPI) GetDirectChannel(first, second string) (*model.Channel, *model.AppError) {
	u, err := a.user(first)
	if err != nil {
		return nil, appError(err)
	}
	c, err := a.runtime.options.Chat.Direct(context.Background(), u, second)
	if err != nil {
		return nil, appError(err)
	}
	return a.GetChannel(c.ID)
}
func (a *hostAPI) SendEphemeralPost(userID string, p *model.Post) *model.Post {
	u, err := a.user(userID)
	if err != nil || p == nil {
		return nil
	}
	if _, err = a.runtime.options.Chat.Channel(context.Background(), u, p.ChannelId); err != nil {
		return nil
	}
	clone := *p
	clone.Id = chat.ID(fmt.Sprintf("ephemeral:%s:%d", a.manifest.Id, time.Now().UnixNano()))
	clone.Type = "system_ephemeral"
	clone.CreateAt = time.Now().UnixMilli()
	clone.UpdateAt = clone.CreateAt
	raw, err := clone.ToJSON()
	if err != nil {
		return nil
	}
	if err = a.runtime.options.Chat.PublishEvent(chat.Event{Type: "ephemeral_message", UserID: userID, Data: map[string]any{"post": string(raw)}}); err != nil {
		return nil
	}
	return &clone
}
func (a *hostAPI) PublishWebSocketEvent(event string, payload map[string]any, b *model.WebsocketBroadcast) {
	if b == nil {
		b = &model.WebsocketBroadcast{}
	}
	if a.runtime.options.TenantID != "" && b.TeamId != "" && b.TeamId != chat.ID(a.runtime.options.TenantID) {
		a.LogWarn("cross-tenant plugin event rejected")
		return
	}
	teamID := b.TeamId
	if teamID == "" && a.runtime.options.TenantID != "" {
		teamID = chat.ID(a.runtime.options.TenantID)
	}
	if b.UserId != "" {
		if _, err := a.user(b.UserId); err != nil {
			return
		}
	}
	if b.ChannelId != "" {
		if _, err := a.GetChannel(b.ChannelId); err != nil {
			return
		}
	}
	if err := a.runtime.options.Chat.PublishEvent(chat.Event{Type: "custom_" + a.manifest.Id + "_" + event, Data: payload, UserID: b.UserId, ChannelID: b.ChannelId, TeamID: teamID, OmitUsers: b.OmitUsers}); err != nil {
		a.LogError("plugin event failed", "error", err)
	}
}
func (a *hostAPI) GetLicense() *model.License { return nil }
func (a *hostAPI) HasPermissionTo(userID string, p *model.Permission) bool {
	if p == nil {
		return false
	}
	u, err := a.user(userID)
	if err != nil {
		return false
	}
	if p.Id == "manage_system" {
		for _, email := range a.runtime.options.Admins {
			if strings.EqualFold(email, u.Email) {
				return true
			}
		}
		return strings.Contains(","+u.Roles+",", ",global_admin,")
	}
	switch p.Id {
	case "create_direct_channel", "view_members":
		return true
	}
	return false
}
func (a *hostAPI) HasPermissionToChannel(userID, channelID string, p *model.Permission) bool {
	if p == nil {
		return false
	}
	u, err := a.user(userID)
	if err != nil {
		return false
	}
	if _, err = a.runtime.options.Chat.Channel(context.Background(), u, channelID); err != nil {
		return false
	}
	switch p.Id {
	case "read_channel", "create_post", "read_public_channel":
		return true
	}
	return false
}
func (a *hostAPI) HasPermissionToTeam(userID, teamID string, p *model.Permission) bool {
	u, err := a.user(userID)
	if err != nil || p == nil || chat.ID(u.TenantID) != teamID {
		return false
	}
	switch p.Id {
	case "view_team", "list_team_channels", "create_public_channel", "create_private_channel":
		return true
	}
	return false
}

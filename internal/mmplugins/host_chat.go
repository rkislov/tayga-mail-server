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
	if !a.userInScope(u) {
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
	if _, err := a.user(userID); err != nil {
		return nil, appError(err)
	}
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
	if _, err := a.user(second); err != nil {
		return nil, appError(err)
	}
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
	if err := a.runtime.options.Chat.PublishEvent(chat.Event{Type: "custom_" + a.manifest.Id + "_" + event, Data: payload, UserID: b.UserId, ChannelID: b.ChannelId, TeamID: teamID, DomainID: a.runtime.options.DomainID, OmitUsers: b.OmitUsers}); err != nil {
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
	if _, e := a.GetChannel(channelID); e != nil {
		return false
	}
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
	case "view_team", "list_team_channels", "create_public_channel", "create_private_channel", model.PermissionPublicPlaybookCreate.Id, model.PermissionPrivatePlaybookCreate.Id, model.PermissionRunCreate.Id:
		return true
	}
	return false
}

func (a *hostAPI) CreateChannel(channel *model.Channel) (*model.Channel, *model.AppError) {
	if channel == nil || a.runtime.options.TenantID == "" || channel.TeamId != chat.ID(a.runtime.options.TenantID) {
		return nil, appError(chat.ErrForbidden)
	}
	creatorID := channel.CreatorId
	if creatorID == "" {
		// Server-side plugins create channels without an interactive user. Attribute
		// those channels to a service account owned by this plugin and domain.
		id, err := a.serviceBotUserID()
		if err != nil {
			return nil, appError(err)
		}
		creatorID = id
	}
	user, err := a.user(creatorID)
	if err != nil {
		return nil, appError(err)
	}
	created, err := a.runtime.options.Chat.CreateChannel(context.Background(), user, channel.Name, channel.DisplayName, string(channel.Type))
	if err != nil {
		return nil, appError(err)
	}
	return a.GetChannel(created.ID)
}

func (a *hostAPI) GetChannelMember(channelID, userID string) (*model.ChannelMember, *model.AppError) {
	user, err := a.user(userID)
	if err != nil {
		return nil, appError(err)
	}
	if _, appErr := a.GetChannel(channelID); appErr != nil {
		return nil, appErr
	}
	members, err := a.runtime.options.Chat.Members(context.Background(), user, channelID)
	if err != nil {
		return nil, appError(err)
	}
	for _, member := range members {
		if chat.ID(member.UserID) == userID {
			roles := "channel_user"
			channel, e := a.GetChannel(channelID)
			if e != nil {
				return nil, e
			}
			admin := channel.CreatorId == userID
			if admin {
				roles += " channel_admin"
			}
			return &model.ChannelMember{ChannelId: channelID, UserId: userID, Roles: roles, SchemeUser: true, SchemeAdmin: admin, LastViewedAt: member.LastViewedAt, MsgCount: member.ReadCount, NotifyProps: model.StringMap{}}, nil
		}
	}
	return nil, appError(storage.ErrNotFound)
}

func (a *hostAPI) GetTeamMember(teamID, userID string) (*model.TeamMember, *model.AppError) {
	user, err := a.user(userID)
	if err != nil {
		return nil, appError(err)
	}
	if teamID != chat.ID(user.TenantID) {
		return nil, appError(chat.ErrForbidden)
	}
	return &model.TeamMember{TeamId: teamID, UserId: userID, Roles: "team_user", SchemeUser: true}, nil
}

// TMS membership follows the user's tenant. A plugin cannot move users between
// tenants; adding an existing member is an idempotent lookup.
func (a *hostAPI) CreateTeamMember(teamID, userID string) (*model.TeamMember, *model.AppError) {
	return a.GetTeamMember(teamID, userID)
}

func (a *hostAPI) AddUserToChannel(channelID, userID, asUserID string) (*model.ChannelMember, *model.AppError) {
	if asUserID != "" {
		if _, err := a.user(asUserID); err != nil {
			return nil, appError(err)
		}
	}
	return a.AddChannelMember(channelID, userID)
}

func (a *hostAPI) GetChannelMembers(channelID string, page, perPage int) (model.ChannelMembers, *model.AppError) {
	if page < 0 || perPage < 1 || perPage > 1000 || page > 100000 {
		return nil, appError(ErrInvalid)
	}
	channel, err := a.GetChannel(channelID)
	if err != nil {
		return nil, err
	}
	creator, e := a.user(channel.CreatorId)
	if e != nil {
		return nil, appError(e)
	}
	members, e := a.runtime.options.Chat.Members(context.Background(), creator, channelID)
	if e != nil {
		return nil, appError(e)
	}
	visible := model.ChannelMembers{}
	for _, member := range members {
		id := chat.ID(member.UserID)
		if _, e = a.user(id); e != nil {
			continue
		}
		roles := "channel_user"
		admin := channel.CreatorId == id
		if admin {
			roles += " channel_admin"
		}
		visible = append(visible, model.ChannelMember{ChannelId: channelID, UserId: id, Roles: roles, SchemeAdmin: admin, SchemeUser: true, LastViewedAt: member.LastViewedAt, MsgCount: member.ReadCount, NotifyProps: model.StringMap{}})
	}
	start := page * perPage
	if start >= len(visible) {
		return model.ChannelMembers{}, nil
	}
	end := start + perPage
	if end > len(visible) {
		end = len(visible)
	}
	return visible[start:end], nil
}

// TMS currently models channel administration through its creator. Promote the
// owner of a plugin-created run by transferring that real authority, rather than
// merely returning an admin role that the chat service would not enforce.
func (a *hostAPI) UpdateChannelMemberRoles(channelID, userID, roles string) (*model.ChannelMember, *model.AppError) {
	if roles != "channel_user channel_admin" && roles != "channel_admin channel_user" {
		return nil, appError(ErrInvalid)
	}
	user, err := a.user(userID)
	if err != nil {
		return nil, appError(err)
	}
	member, appErr := a.GetChannelMember(channelID, userID)
	if appErr != nil {
		return nil, appErr
	}
	channel, appErr := a.GetChannel(channelID)
	if appErr != nil {
		return nil, appErr
	}
	if channel.CreatorId != userID {
		creator, err := a.user(channel.CreatorId)
		if err != nil {
			return nil, appError(err)
		}
		owner, ok, err := a.runtime.options.Store.GetSetting(context.Background(), "mmplugin.bot_owner:"+creator.ID)
		if err != nil {
			return nil, appError(err)
		}
		if !ok || creator.AuthSource != "bot" || owner != a.runtime.options.Scope+":"+a.manifest.Id {
			return nil, appError(chat.ErrForbidden)
		}
		result, err := a.runtime.options.Store.DB().ExecContext(context.Background(), a.runtime.bind(`UPDATE chat_channels SET creator_id=? WHERE id=? AND creator_id=?`), user.ID, channelID, creator.ID)
		if err != nil {
			return nil, appError(err)
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return nil, appError(chat.ErrForbidden)
		}
	}
	member.Roles = "channel_user channel_admin"
	member.SchemeAdmin = true
	return member, nil
}

func (a *hostAPI) serviceBotUserID() (string, error) {
	users, err := a.runtime.options.Store.ListUsersByDomain(context.Background(), a.runtime.options.DomainID)
	if err != nil {
		return "", err
	}
	for _, user := range users {
		if user.AuthSource != "bot" || !a.userInScope(user) {
			continue
		}
		owner, ok, err := a.runtime.options.Store.GetSetting(context.Background(), "mmplugin.bot_owner:"+user.ID)
		if err != nil {
			return "", err
		}
		if ok && owner == a.runtime.options.Scope+":"+a.manifest.Id {
			return chat.ID(user.ID), nil
		}
	}
	return a.EnsureBotUser(&model.Bot{Username: "svc-" + chat.ID(a.manifest.Id)[:16], DisplayName: a.manifest.Name})
}
func (a *hostAPI) DeleteChannelMember(channelID, userID string) *model.AppError {
	if _, err := a.user(userID); err != nil {
		return appError(err)
	}
	channel, e := a.GetChannel(channelID)
	if e != nil {
		return e
	}
	creator, err := a.user(channel.CreatorId)
	if err != nil {
		return appError(err)
	}
	return appError(a.runtime.options.Chat.RemoveMember(context.Background(), creator, channelID, userID))
}

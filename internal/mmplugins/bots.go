package mmplugins

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

var botName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func (a *hostAPI) ensureBot(bot *model.Bot) (*model.Bot, error) {
	if bot == nil || !botName.MatchString(bot.Username) || a.runtime.options.TenantID == "" || a.runtime.options.DomainID == "" {
		return nil, fmt.Errorf("bot provisioning requires a tenant/domain scope and a valid username")
	}
	a.runtime.botMu.Lock()
	defer a.runtime.botMu.Unlock()
	ctx := context.Background()
	domain, err := a.runtime.options.Store.GetDomainByID(ctx, a.runtime.options.DomainID)
	if err != nil {
		return nil, err
	}
	if domain.TenantID != a.runtime.options.TenantID {
		return nil, chat.ErrForbidden
	}
	email := bot.Username + "@" + domain.Name
	u, err := a.runtime.options.Store.GetUserByEmail(ctx, email)
	owner := a.runtime.options.Scope + ":" + a.manifest.Id
	if err == storage.ErrNotFound {
		u, err = a.runtime.options.Store.CreateUser(ctx, &storage.User{TenantID: domain.TenantID, DomainID: domain.ID, LocalPart: bot.Username, Email: email, DisplayName: bot.DisplayName, AuthSource: "bot", Roles: "bot", Enabled: true})
		if err != nil {
			return nil, err
		}
		if err = a.runtime.options.Store.PutSetting(ctx, "mmplugin.bot_owner:"+u.ID, owner); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		existing, ok, e := a.runtime.options.Store.GetSetting(ctx, "mmplugin.bot_owner:"+u.ID)
		if e != nil {
			return nil, e
		}
		if !ok || existing != owner || u.AuthSource != "bot" {
			return nil, fmt.Errorf("bot username is already owned by another account")
		}
	}
	result := *bot
	result.UserId = chat.ID(u.ID)
	result.OwnerId = a.manifest.Id
	result.CreateAt = u.CreatedAt.UnixMilli()
	result.UpdateAt = time.Now().UnixMilli()
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if err = a.runtime.options.Store.PutSetting(ctx, a.runtime.stateKey("bot", a.manifest.Id+":"+result.UserId), string(raw)); err != nil {
		return nil, err
	}
	return &result, nil
}
func (a *hostAPI) EnsureBotUser(bot *model.Bot) (string, error) {
	b, err := a.ensureBot(bot)
	if err != nil {
		return "", err
	}
	return b.UserId, nil
}
func (a *hostAPI) CreateBot(bot *model.Bot) (*model.Bot, *model.AppError) {
	b, err := a.ensureBot(bot)
	return b, appError(err)
}
func (a *hostAPI) GetBot(id string, includeDeleted bool) (*model.Bot, *model.AppError) {
	raw, ok, err := a.runtime.options.Store.GetSetting(context.Background(), a.runtime.stateKey("bot", a.manifest.Id+":"+id))
	if err != nil {
		return nil, appError(err)
	}
	if !ok {
		return nil, appError(storage.ErrNotFound)
	}
	var b model.Bot
	if err = json.Unmarshal([]byte(raw), &b); err != nil {
		return nil, appError(err)
	}
	if !includeDeleted && b.DeleteAt != 0 {
		return nil, appError(storage.ErrNotFound)
	}
	return &b, nil
}
func botUsername(u *storage.User) string {
	if u.AuthSource == "bot" {
		return u.LocalPart
	}
	return u.LocalPart + "-" + chat.ID(u.ID)[:8]
}

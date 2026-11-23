// SPDX-License-Identifier: Apache-2.0
package mmfederation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func (s *Service) remoteUser(ctx context.Context, p Peer, foreign string, profile *model.User, bridge bool) (*storage.User, error) {
	if !model.IsValidId(foreign) {
		return nil, ErrInvalid
	}
	key := foreign
	if bridge {
		key = "bridge"
	}
	localID := chat.ID("federated-user:" + p.ID + ":" + key)
	u, e := s.Store.GetUserByID(ctx, localID)
	display := foreign
	if profile != nil {
		display = strings.TrimSpace(profile.FirstName + " " + profile.LastName)
		if display == "" {
			display = profile.Nickname
		}
		if display == "" {
			display = profile.Username
		}
	}
	if len(display) > 256 {
		display = display[:256]
	}
	if bridge {
		display = "Мост · " + p.Name
	}
	if errors.Is(e, storage.ErrNotFound) {
		username := "remote-" + localID[:12] + "-" + p.Name
		if bridge {
			username = "bridge-" + localID[:12]
		}
		if len(username) > 64 {
			username = username[:64]
		}
		u, e = s.Store.CreateUser(storage.WithoutWelcomeMail(ctx), &storage.User{ID: localID, TenantID: p.TenantID, DomainID: p.DomainID, Email: localID + "@federation.invalid", LocalPart: username, DisplayName: display + " · " + p.Name, AuthSource: "federated", Enabled: false})
		if e != nil {
			u, e = s.Store.GetUserByID(ctx, localID)
		}
	}
	if e != nil {
		return nil, e
	}
	if u.TenantID != p.TenantID || u.DomainID != p.DomainID || u.AuthSource != "federated" || u.Enabled {
		return nil, ErrForbidden
	}
	if profile != nil && !bridge && u.DisplayName != display+" · "+p.Name {
		if e = s.Store.UpdateUserProfile(ctx, u.ID, display+" · "+p.Name); e != nil {
			return nil, e
		}
		u.DisplayName = display + " · " + p.Name
	}
	{
		_, e = s.db.ExecContext(ctx, s.q(`INSERT INTO chat_federation_users(peer_id,remote_user_id,local_user_id) VALUES(?,?,?) ON CONFLICT DO NOTHING`), p.ID, key, u.ID)
	}
	return u, e
}
func (s *Service) member(ctx context.Context, p Peer, l Link, u *storage.User) error {
	if e := s.active(ctx, p, l); e != nil {
		return e
	}
	if u.TenantID != p.TenantID || u.DomainID != p.DomainID || u.AuthSource != "federated" {
		return ErrForbidden
	}
	_, e := s.db.ExecContext(ctx, s.q(`INSERT INTO chat_members(channel_id,user_id,last_viewed_at) VALUES(?,?,0) ON CONFLICT DO NOTHING`), l.ChannelID, u.ID)
	return e
}
func (s *Service) mappedPost(ctx context.Context, l Link, foreign string) (string, string, int64, error) {
	var local, origin string
	var updated int64
	e := s.db.QueryRowContext(ctx, s.q(`SELECT local_post_id,origin,last_update_at FROM chat_federation_posts WHERE link_id=? AND remote_post_id=?`), l.ID, foreign).Scan(&local, &origin, &updated)
	return local, origin, updated, e
}
func (s *Service) localRoot(ctx context.Context, l Link, foreign string) (string, error) {
	if foreign == "" {
		return "", nil
	}
	local, _, _, e := s.mappedPost(ctx, l, foreign)
	if e != nil {
		return "", e
	}
	return local, nil
}
func (s *Service) importPost(ctx context.Context, p Peer, l Link, post *model.Post, profiles map[string]*model.User, files []string) (chat.Post, error) {
	if e := s.active(ctx, p, l); e != nil {
		return chat.Post{}, e
	}
	if post == nil || !model.IsValidId(post.Id) || !model.IsValidId(post.UserId) || post.ChannelId != l.RemoteChannelID || len(post.Message) > 65535 {
		return chat.Post{}, ErrInvalid
	}
	local, origin, version, e := s.mappedPost(ctx, l, post.Id)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return chat.Post{}, e
	}
	if origin == "local" {
		return chat.Post{ID: local}, nil
	}
	updated := post.UpdateAt
	if updated < post.CreateAt {
		updated = post.CreateAt
	}
	if updated < post.DeleteAt {
		updated = post.DeleteAt
	}
	if e == nil && updated <= version {
		return chat.Post{ID: local}, nil
	}
	user, e := s.remoteUser(ctx, p, post.UserId, profiles[post.UserId], p.Mode == "rest")
	if e != nil {
		return chat.Post{}, e
	}
	if e = s.member(ctx, p, l, user); e != nil {
		return chat.Post{}, e
	}
	text := post.Message
	if p.Mode == "rest" {
		author := post.UserId
		if profile := profiles[post.UserId]; profile != nil && profile.Username != "" {
			author = profile.Username
		}
		text = "**" + strings.NewReplacer("*", "", "[", "", "]", "", "\n", " ", "\r", " ").Replace(author) + " · " + p.Name + "**\n\n" + text
		if len(text) > 65535 {
			return chat.Post{}, ErrInvalid
		}
	}
	if local != "" {
		// A peer may edit/delete only a post previously imported through this link.
		current, e := s.Chat.Post(ctx, user, local)
		if e != nil {
			return chat.Post{}, e
		}
		if current.UserID != user.ID || current.ChannelID != l.ChannelID {
			return chat.Post{}, ErrForbidden
		}
		tx, e := s.db.BeginTx(ctx, nil)
		if e != nil {
			return chat.Post{}, e
		}
		defer tx.Rollback()
		// Fence concurrent imports before mutating the post: an older retry must
		// never overwrite an edit that committed after the initial read.
		result, e := tx.ExecContext(ctx, s.q(`UPDATE chat_federation_posts SET last_update_at=?,deleted_at=? WHERE link_id=? AND remote_post_id=? AND origin='remote' AND last_update_at=?`), updated, post.DeleteAt, l.ID, post.Id, version)
		if e != nil {
			return chat.Post{}, e
		}
		changed, e := result.RowsAffected()
		if e != nil || changed != 1 {
			return chat.Post{}, ErrInvalid
		}
		if post.DeleteAt == 0 {
			// Replacing attachments on an existing message is not supported yet.
			// Reject it rather than acknowledging a silently incomplete edit.
			if len(current.Attachments) != len(files) {
				return chat.Post{}, ErrInvalid
			}
			for i, file := range current.Attachments {
				if file.ID != files[i] {
					return chat.Post{}, ErrInvalid
				}
			}
		}
		if post.DeleteAt > 0 {
			_, e = tx.ExecContext(ctx, s.q(`DELETE FROM chat_posts WHERE id=? AND user_id=? AND channel_id=?`), local, user.ID, l.ChannelID)
		} else {
			_, e = tx.ExecContext(ctx, s.q(`UPDATE chat_posts SET message=? WHERE id=? AND user_id=? AND channel_id=?`), text, local, user.ID, l.ChannelID)
		}
		if e != nil {
			return chat.Post{}, e
		}
		if e = tx.Commit(); e != nil {
			return chat.Post{}, e
		}
		current.Message = text
		typ := "post_edited"
		if post.DeleteAt > 0 {
			typ = "post_deleted"
		}
		_ = s.Chat.PublishEvent(chat.Event{Type: typ, Post: current, ChannelID: l.ChannelID, DomainID: p.DomainID})
		return current, nil
	}
	if post.DeleteAt > 0 {
		_, e = s.db.ExecContext(ctx, s.q(`INSERT INTO chat_federation_posts(link_id,remote_post_id,local_post_id,origin,last_update_at,deleted_at) VALUES(?,?,?,'remote',?,?) ON CONFLICT DO NOTHING`), l.ID, post.Id, "deleted-"+chat.ID(l.ID+post.Id), updated, post.DeleteAt)
		return chat.Post{}, e
	}
	root, e := s.localRoot(ctx, l, post.RootId)
	if e != nil {
		return chat.Post{}, e
	}
	props := map[string]any{"tms_federation": map[string]string{"peer_id": p.ID, "post_id": post.Id}}
	// Imported interactive props are deliberately not executed by local plugins.
	saved, e := s.Chat.SendWithFiles(ctx, user, l.ChannelID, root, text, "federation:"+p.ID+":"+post.Id, props, files)
	if e != nil {
		return chat.Post{}, e
	}
	_, e = s.db.ExecContext(ctx, s.q(`INSERT INTO chat_federation_posts(link_id,remote_post_id,local_post_id,origin,last_update_at) VALUES(?,?,?,'remote',?) ON CONFLICT DO NOTHING`), l.ID, post.Id, saved.ID, updated)
	return saved, e
}
func (s *Service) outbound(ctx context.Context, p Peer, l Link) ([]chat.Post, error) {
	owner, e := s.Store.GetUserByID(ctx, p.OwnerID)
	if e != nil || !owner.Enabled {
		return nil, ErrForbidden
	}
	if _, e = s.Chat.Channel(ctx, owner, l.ChannelID); e != nil {
		return nil, e
	}
	rows, e := s.db.QueryContext(ctx, s.q(`SELECT id FROM chat_posts WHERE channel_id=? AND (created_at>? OR (created_at=? AND id>?)) ORDER BY created_at,id LIMIT 50`), l.ChannelID, l.LocalCursorAt, l.LocalCursorAt, l.LocalCursorID)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []chat.Post{}
	for _, id := range ids {
		item, e := s.Chat.Post(ctx, owner, id)
		if e != nil {
			return nil, e
		}
		out = append(out, item)
	}
	return out, nil
}
func (s *Service) skipImported(ctx context.Context, l Link, post chat.Post) (bool, error) {
	if _, ok := post.Props["tms_federation_origin"]; ok {
		return true, nil
	}
	var count int
	e := s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM chat_federation_posts WHERE local_post_id=? AND origin='remote'`), post.ID).Scan(&count)
	return count > 0, e
}
func (s *Service) advanceLocal(ctx context.Context, l Link, post chat.Post) error {
	res, e := s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_links SET local_cursor_at=?,local_cursor_id=? WHERE id=? AND enabled=1 AND lease_token=?`), post.CreatedAt, post.ID, l.ID, l.LeaseToken)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrForbidden
	}
	return nil
}
func (s *Service) exportModel(ctx context.Context, p Peer, l Link, post chat.Post) (*model.Post, *model.User, error) {
	u, e := s.Store.GetUserByID(ctx, post.UserID)
	if e != nil || !u.Enabled || u.TenantID != p.TenantID || u.DomainID != p.DomainID {
		return nil, nil, ErrForbidden
	}
	username := u.LocalPart
	if len(username) > 40 {
		username = username[:40]
	}
	if username == "" {
		username = "user"
	}
	username += "-" + chat.ID(u.ID)[:6]
	user := &model.User{Id: chat.ID(u.ID), Username: username, Email: chat.ID(u.ID) + "@federation.invalid", Nickname: u.DisplayLabel(), FirstName: u.DisplayName, CreateAt: u.CreatedAt.UnixMilli(), UpdateAt: u.CreatedAt.UnixMilli(), Roles: "system_user"}
	root := post.RootID
	if root != "" {
		var foreign string
		e = s.db.QueryRowContext(ctx, s.q(`SELECT remote_post_id FROM chat_federation_posts WHERE link_id=? AND local_post_id=?`), l.ID, root).Scan(&foreign)
		if e != nil {
			return nil, nil, e
		}
		root = foreign
	}
	return &model.Post{Id: post.ID, ChannelId: l.RemoteChannelID, UserId: user.Id, RootId: root, Message: post.Message, CreateAt: post.CreatedAt, UpdateAt: post.CreatedAt, Props: model.StringInterface{"tms_federation": map[string]string{"peer_id": p.ID, "post_id": post.ID}}}, user, nil
}
func (s *Service) recordExport(ctx context.Context, l Link, post chat.Post, foreign string) error {
	if !model.IsValidId(foreign) {
		return ErrInvalid
	}
	_, e := s.db.ExecContext(ctx, s.q(`INSERT INTO chat_federation_posts(link_id,remote_post_id,local_post_id,origin,last_update_at) VALUES(?,?,?,'local',?) ON CONFLICT DO NOTHING`), l.ID, foreign, post.ID, post.CreatedAt)
	return e
}
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	slots := make(chan struct{}, 4)
	var wg sync.WaitGroup
	defer wg.Wait()
	launch := func(work func()) {
		select {
		case slots <- struct{}{}:
			wg.Add(1)
			go func() { defer wg.Done(); defer func() { <-slots }(); work() }()
		default:
		}
	}
	for {
		if ctx.Err() != nil {
			return
		}
		rows, e := s.db.QueryContext(ctx, `SELECT `+peerColumns+` FROM chat_federation_peers WHERE enabled=1 ORDER BY id`)
		peers := []Peer{}
		if e == nil {
			for rows.Next() {
				p, err := scanPeer(rows)
				if err != nil {
					break
				}
				peers = append(peers, p)
			}
			rows.Close()
		}
		for _, p := range peers {
			if !p.Confirmed {
				if p.Mode == "native" {
					launch(func() { _ = s.ConfirmPending(ctx, p) })
				}
				continue
			}
			links, err := s.Links(ctx, p)
			if err != nil {
				continue
			}
			for _, l := range links {
				if l.Enabled {
					launch(func() {
						if e := s.Sync(ctx, p, l); e != nil && ctx.Err() == nil {
							s.Log.Warn("federation sync deferred", "peer_id", p.ID, "link_id", l.ID)
						}
					})
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Service) Sync(ctx context.Context, p Peer, l Link) error {
	if e := s.active(ctx, p, l); e != nil {
		return e
	}
	token := model.NewId()
	now := time.Now().UnixMilli()
	res, e := s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_links SET lease_token=?,lease_until=? WHERE id=? AND enabled=1 AND lease_until<?`), token, now+10*60*1000, l.ID, now)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e != nil || n == 0 {
		return e
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = s.db.ExecContext(cleanup, s.q(`UPDATE chat_federation_links SET lease_until=0,lease_token='' WHERE id=? AND lease_token=?`), l.ID, token)
	}()
	l, e = s.Link(ctx, p, l.ID)
	if e != nil {
		return e
	}
	l.LeaseToken = token
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-workCtx.Done():
				return
			case <-ticker.C:
				if s.active(workCtx, p, l) != nil {
					cancel()
					return
				}
				result, err := s.db.ExecContext(workCtx, s.q(`UPDATE chat_federation_links SET lease_until=? WHERE id=? AND lease_token=? AND enabled=1`), time.Now().Add(10*time.Minute).UnixMilli(), l.ID, token)
				if err != nil {
					cancel()
					return
				}
				n, err := result.RowsAffected()
				if err != nil || n != 1 {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-done }()
	ctx = workCtx
	if p.Mode == "rest" {
		e = s.syncREST(ctx, p, l)
	} else {
		e = s.syncNative(ctx, p, l)
	}
	if e != nil {
		_, _ = s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_peers SET last_error=? WHERE id=?`), "synchronization deferred", p.ID)
	} else {
		_, _ = s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_peers SET last_success_at=?,last_error='' WHERE id=?`), time.Now().UnixMilli(), p.ID)
	}
	return e
}

// Keep standard JSON snapshots deterministic for durable idempotency checks.
func snapshot(v any) string { raw, _ := json.Marshal(v); return string(raw) }

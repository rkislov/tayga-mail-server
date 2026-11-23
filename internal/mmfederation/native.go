// SPDX-License-Identifier: Apache-2.0
package mmfederation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
)

type NativeResponse struct {
	Status  string          `json:"status"`
	Err     string          `json:"err,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func nativeReply(w http.ResponseWriter, status int, payload any, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	response := NativeResponse{Status: "OK"}
	if err != nil {
		response.Status = "FAIL"
		response.Err = "federation request rejected"
	} else if payload != nil {
		response.Payload, _ = json.Marshal(payload)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
func (s *Service) saveCredentials(ctx context.Context, p Peer, c Credentials, confirmed bool) error {
	secret, e := s.seal(p, c)
	if e != nil {
		return e
	}
	value := 0
	if confirmed {
		value = 1
	}
	_, e = s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_peers SET secret=?,confirmed=? WHERE id=? AND enabled=1`), secret, value, p.ID)
	return e
}
func (s *Service) Invite(ctx context.Context, p Peer, password string) (string, error) {
	if p.Mode != "native" || !p.Enabled || p.Confirmed || len(password) < 12 || len(password) > 256 {
		return "", ErrInvalid
	}
	c, e := s.credentials(ctx, p)
	if e != nil {
		return "", e
	}
	own, e := siteURL(s.PublicURL)
	if e != nil {
		return "", e
	}
	c.Token = model.NewId()
	if e = s.saveCredentials(ctx, p, c, false); e != nil {
		return "", e
	}
	_, e = s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_peers SET created_at=? WHERE id=? AND enabled=1 AND confirmed=0`), time.Now().UnixMilli(), p.ID)
	if e != nil {
		return "", e
	}
	invite := model.RemoteClusterInvite{RemoteId: p.ID, SiteURL: own, Token: c.Token, Version: 3}
	raw, e := invite.Encrypt(password)
	if e != nil {
		return "", ErrInvalid
	}
	return base64.URLEncoding.EncodeToString(raw), nil
}
func (s *Service) AcceptInvite(ctx context.Context, p Peer, code, password string) (Peer, error) {
	if len(code) > 16384 || len(password) < 1 || len(password) > 256 {
		return Peer{}, ErrInvalid
	}
	raw, e := base64.URLEncoding.DecodeString(strings.TrimSpace(code))
	if e != nil {
		return Peer{}, ErrInvalid
	}
	var invite model.RemoteClusterInvite
	if e = invite.Decrypt(raw, password); e != nil {
		return Peer{}, ErrInvalid
	}
	if invite.IsValid() != nil || invite.Version < 1 || invite.Version > 3 {
		return Peer{}, ErrInvalid
	}
	if _, e = siteURL(s.PublicURL); e != nil {
		return Peer{}, e
	}
	p.ID = invite.RemoteId
	p.Mode = "native"
	p.SiteURL = invite.SiteURL
	c := Credentials{Token: model.NewId(), RemoteToken: model.NewId(), BootstrapToken: invite.Token}
	if invite.Version < 2 {
		c.RemoteToken = invite.Token
	}
	p, e = s.CreatePeer(ctx, p, c)
	if e != nil {
		return Peer{}, e
	}
	if e = s.ConfirmPending(ctx, p); e != nil {
		return p, e
	}
	p.Confirmed = true
	return p, nil
}
func (s *Service) sendFrame(ctx context.Context, p Peer, path, topic string, payload any, result any) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	raw, e := json.Marshal(payload)
	if e != nil {
		return e
	}
	frame := model.RemoteClusterFrame{RemoteId: p.ID, Msg: model.NewRemoteClusterMsg(topic, raw)}
	wire, e := json.Marshal(frame)
	if e != nil {
		return e
	}
	response, e := s.request(ctx, p, "POST", path, bytes.NewReader(wire), "application/json")
	if e != nil {
		return e
	}
	defer response.Body.Close()
	var envelope NativeResponse
	if e = decodeJSON(response.Body, &envelope, 8<<20); e != nil {
		return e
	}
	if envelope.Status != "OK" {
		return ErrForbidden
	}
	if result != nil {
		if len(envelope.Payload) == 0 {
			return ErrInvalid
		}
		if e = json.Unmarshal(envelope.Payload, result); e != nil {
			return ErrInvalid
		}
	}
	return nil
}
func (s *Service) authenticateNative(r *http.Request) (Peer, error) {
	id := r.Header.Get(model.HeaderRemoteclusterId)
	token := r.Header.Get(model.HeaderRemoteclusterToken)
	if !model.IsValidId(id) || len(token) > 256 || token == "" {
		return Peer{}, ErrForbidden
	}
	p, e := s.Peer(r.Context(), id)
	if e != nil || p.Mode != "native" || !p.Enabled || (s.ScopeAllowed != nil && !s.ScopeAllowed(r.Context(), p.TenantID, p.DomainID)) {
		return Peer{}, ErrForbidden
	}
	c, e := s.credentials(r.Context(), p)
	if e != nil {
		return Peer{}, ErrForbidden
	}
	a, b := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(c.Token))
	if subtle.ConstantTimeCompare(a[:], b[:]) != 1 {
		return Peer{}, ErrForbidden
	}
	return p, nil
}
func (s *Service) ServeNative(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		nativeReply(w, 405, nil, ErrInvalid)
		return
	}
	p, e := s.authenticateNative(r)
	if e != nil {
		nativeReply(w, 403, nil, e)
		return
	}
	if strings.Contains(r.URL.Path, "/upload/") {
		s.receiveNativeFile(w, r, p)
		return
	}
	var frame model.RemoteClusterFrame
	if e = decodeJSON(r.Body, &frame, 8<<20); e != nil || frame.RemoteId != p.ID || frame.IsValid() != nil {
		nativeReply(w, 400, nil, ErrInvalid)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v4/remotecluster")
	if path == "/confirm_invite" {
		if p.Confirmed || time.Now().UnixMilli()-p.CreatedAt > 24*60*60*1000 {
			nativeReply(w, 409, nil, ErrForbidden)
			return
		}
		var confirmation model.RemoteClusterInvite
		if json.Unmarshal(frame.Msg.Payload, &confirmation) != nil || confirmation.RemoteId != p.ID || confirmation.IsValid() != nil || confirmation.Version < 2 || confirmation.RefreshedToken == "" || len(confirmation.Token) > 256 || len(confirmation.RefreshedToken) > 256 {
			nativeReply(w, 400, nil, ErrInvalid)
			return
		}
		normalized, e := siteURL(confirmation.SiteURL)
		if e != nil || normalized != p.SiteURL {
			nativeReply(w, 403, nil, ErrForbidden)
			return
		}
		c, e := s.credentials(r.Context(), p)
		if e != nil || confirmation.RefreshedToken == c.Token {
			nativeReply(w, 403, nil, ErrForbidden)
			return
		}
		c.RemoteToken = confirmation.Token
		c.Token = confirmation.RefreshedToken
		e = s.saveCredentials(r.Context(), p, c, true)
		nativeReply(w, 200, nil, e)
		return
	}
	if !p.Confirmed {
		nativeReply(w, 403, nil, ErrForbidden)
		return
	}
	if path == "/ping" {
		var ping model.RemoteClusterPing
		if frame.Msg.Topic != "ping" || json.Unmarshal(frame.Msg.Payload, &ping) != nil {
			nativeReply(w, 400, nil, ErrInvalid)
			return
		}
		ping.RecvAt = time.Now().UnixMilli()
		nativeReply(w, 200, ping, nil)
		return
	}
	if path != "/msg" {
		nativeReply(w, 404, nil, ErrInvalid)
		return
	}
	switch frame.Msg.Topic {
	case "sharedchannel_invite":
		var invite struct {
			ChannelID string `json:"channel_id"`
			Name      string `json:"name"`
			Type      string `json:"type"`
			ReadOnly  bool   `json:"read_only"`
		}
		if json.Unmarshal(frame.Msg.Payload, &invite) != nil || !model.IsValidId(invite.ChannelID) || len(invite.Name) > 64 || (invite.Type != "O" && invite.Type != "P") {
			nativeReply(w, 400, nil, ErrInvalid)
			return
		}
		var count int
		e = s.db.QueryRowContext(r.Context(), s.q(`SELECT COUNT(*) FROM chat_federation_invites WHERE peer_id=?`), p.ID).Scan(&count)
		if e != nil || count >= 64 {
			nativeReply(w, 429, nil, ErrForbidden)
			return
		}
		_, e = s.db.ExecContext(r.Context(), s.q(`INSERT INTO chat_federation_invites(peer_id,remote_channel_id,name,kind,created_at) VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING`), p.ID, invite.ChannelID, invite.Name, invite.Type, time.Now().UnixMilli())
		if e != nil {
			nativeReply(w, 500, nil, e)
			return
		}
		l, e := scanLink(s.db.QueryRowContext(r.Context(), s.q(`SELECT `+linkColumns+` FROM chat_federation_links WHERE peer_id=? AND remote_channel_id=? AND enabled=1`), p.ID, invite.ChannelID))
		if e != nil || s.active(r.Context(), p, l) != nil {
			nativeReply(w, 200, nil, ErrForbidden)
			return
		}
		localRO := 0
		if invite.ReadOnly {
			localRO = 1
		}
		_, _ = s.db.ExecContext(r.Context(), s.q(`UPDATE chat_federation_links SET local_readonly=? WHERE id=? AND peer_id=?`), localRO, l.ID, p.ID)
		nativeReply(w, 200, nil, nil)
	case "sharedchannel_sync", "sharedchannel_membership":
		var msg model.SyncMsg
		if json.Unmarshal(frame.Msg.Payload, &msg) != nil || !model.IsValidId(msg.ChannelId) || len(msg.Posts) > 200 || len(msg.Users) > 200 || len(msg.MembershipChanges) > 200 {
			nativeReply(w, 400, nil, ErrInvalid)
			return
		}
		l, e := scanLink(s.db.QueryRowContext(r.Context(), s.q(`SELECT `+linkColumns+` FROM chat_federation_links WHERE peer_id=? AND remote_channel_id=? AND enabled=1`), p.ID, msg.ChannelId))
		if e != nil || s.active(r.Context(), p, l) != nil {
			nativeReply(w, 200, nil, ErrForbidden)
			return
		}
		response := s.receiveSync(r.Context(), p, l, &msg)
		nativeReply(w, 200, response, nil)
	case "sharedchannel_upload":
		s.createNativeUpload(w, r, p, frame.Msg.Payload)
	default:
		nativeReply(w, 200, nil, ErrInvalid)
	}
}
func (s *Service) receiveSync(ctx context.Context, p Peer, l Link, msg *model.SyncMsg) model.SyncResponse {
	response := model.SyncResponse{UserErrors: []string{}, UsersSyncd: []string{}, PostErrors: []string{}, ReactionErrors: []string{}, StatusErrors: []string{}, MembershipErrors: []string{}}
	for id, user := range msg.Users {
		if user == nil || user.Id != id {
			response.UserErrors = append(response.UserErrors, id)
			continue
		}
		u, e := s.remoteUser(ctx, p, id, user, false)
		if e == nil {
			e = s.member(ctx, p, l, u)
		}
		if e != nil {
			response.UserErrors = append(response.UserErrors, id)
		} else {
			response.UsersSyncd = append(response.UsersSyncd, id)
			if user.UpdateAt > response.UsersLastUpdateAt {
				response.UsersLastUpdateAt = user.UpdateAt
			}
		}
	}
	posts := append([]*model.Post(nil), msg.Posts...)
	sort.SliceStable(posts, func(i, j int) bool {
		if posts[i] == nil {
			return false
		}
		if posts[j] == nil {
			return true
		}
		return posts[i].CreateAt < posts[j].CreateAt
	})
	for _, post := range posts {
		if post == nil {
			continue
		}
		if l.ReadOnly {
			response.PostErrors = append(response.PostErrors, post.Id)
			continue
		}
		files, e := s.importedFiles(ctx, p, l, post.FileIds)
		if e == nil {
			_, e = s.importPost(ctx, p, l, post, msg.Users, files)
		}
		if e != nil {
			response.PostErrors = append(response.PostErrors, post.Id)
		} else {
			updated := post.UpdateAt
			if updated < post.DeleteAt {
				updated = post.DeleteAt
			}
			if updated > response.PostsLastUpdateAt {
				response.PostsLastUpdateAt = updated
			}
		}
	}
	for _, change := range msg.MembershipChanges {
		if change == nil || change.ChannelId != l.RemoteChannelID {
			continue
		}
		u, e := s.remoteUser(ctx, p, change.UserId, msg.Users[change.UserId], false)
		if e == nil {
			if change.IsAdd {
				e = s.member(ctx, p, l, u)
			} else {
				_, e = s.db.ExecContext(ctx, s.q(`DELETE FROM chat_members WHERE channel_id=? AND user_id=?`), l.ChannelID, u.ID)
			}
		}
		if e != nil {
			response.MembershipErrors = append(response.MembershipErrors, change.UserId)
		}
	}
	for _, reaction := range msg.Reactions {
		if reaction != nil {
			response.ReactionErrors = append(response.ReactionErrors, reaction.PostId+":"+reaction.UserId+":"+reaction.EmojiName)
		}
	}
	for _, status := range msg.Statuses {
		if status != nil {
			response.StatusErrors = append(response.StatusErrors, status.UserId)
		}
	}
	for _, ack := range msg.Acknowledgements {
		if ack != nil {
			response.AcknowledgementErrors = append(response.AcknowledgementErrors, ack.PostId)
		}
	}
	return response
}
func (s *Service) syncNative(ctx context.Context, p Peer, l Link) error {
	owner, e := s.Store.GetUserByID(ctx, p.OwnerID)
	if e != nil {
		return e
	}
	channel, e := s.Chat.Channel(ctx, owner, l.ChannelID)
	if e != nil {
		return e
	}
	invitation := map[string]any{"channel_id": l.RemoteChannelID, "name": channel.Name, "display_name": channel.DisplayName, "type": channel.Kind, "creator_id": chat.ID(owner.ID), "read_only": l.ReadOnly}
	if e = s.sendFrame(ctx, p, "/api/v4/remotecluster/msg", "sharedchannel_invite", invitation, nil); e != nil {
		return e
	}
	posts, e := s.outbound(ctx, p, l)
	if e != nil {
		return e
	}
	for _, post := range posts {
		if e = s.active(ctx, p, l); e != nil {
			return e
		}
		skip, e := s.skipImported(ctx, l, post)
		if e != nil {
			return e
		}
		if skip || l.LocalReadOnly {
			if e = s.advanceLocal(ctx, l, post); e != nil {
				return e
			}
			continue
		}
		exported, user, e := s.exportModel(ctx, p, l, post)
		if e != nil {
			return e
		}
		if len(post.Attachments) > 0 {
			var response model.SyncResponse
			profiles := model.SyncMsg{Id: model.NewId(), ChannelId: l.RemoteChannelID, Users: map[string]*model.User{user.Id: user}}
			if e = s.sendFrame(ctx, p, "/api/v4/remotecluster/msg", "sharedchannel_sync", profiles, &response); e != nil {
				return e
			}
			if len(response.UserErrors) > 0 {
				return ErrInvalid
			}
		}
		for _, file := range post.Attachments {
			if e = s.exportNativeFile(ctx, p, l, post, file, user); e != nil {
				return e
			}
			exported.FileIds = append(exported.FileIds, file.ID)
		}
		msg := model.SyncMsg{Id: model.NewId(), ChannelId: l.RemoteChannelID, Users: map[string]*model.User{user.Id: user}, Posts: []*model.Post{exported}}
		var response model.SyncResponse
		if e = s.sendFrame(ctx, p, "/api/v4/remotecluster/msg", "sharedchannel_sync", msg, &response); e != nil {
			return e
		}
		if len(response.PostErrors) > 0 || len(response.UserErrors) > 0 {
			return ErrInvalid
		}
		if e = s.recordExport(ctx, l, post, post.ID); e != nil {
			return e
		}
		if e = s.advanceLocal(ctx, l, post); e != nil {
			return e
		}
	}
	return nil
}

func (s *Service) ConfirmPending(ctx context.Context, p Peer) error {
	if p.Mode != "native" || p.Confirmed || !p.Enabled || (s.ScopeAllowed != nil && !s.ScopeAllowed(ctx, p.TenantID, p.DomainID)) {
		return ErrInvalid
	}
	c, e := s.credentials(ctx, p)
	if e != nil || c.BootstrapToken == "" {
		return ErrInvalid
	}
	own, e := siteURL(s.PublicURL)
	if e != nil {
		return e
	}
	confirmation := model.RemoteClusterInvite{RemoteId: p.ID, SiteURL: own, Token: c.Token, RefreshedToken: c.RemoteToken, Version: 3}
	e = s.sendFrame(ctx, p, "/api/v4/remotecluster/confirm_invite", "invitation", confirmation, nil)
	if e != nil {
		// Confirmation may have committed even if its HTTP response was lost.
		// A ping authenticated with the refreshed token proves that commit.
		var response model.RemoteClusterPing
		e = s.sendFrame(ctx, p, "/api/v4/remotecluster/ping", "ping", model.RemoteClusterPing{SentAt: time.Now().UnixMilli()}, &response)
	}
	if e != nil {
		return e
	}
	c.BootstrapToken = ""
	return s.saveCredentials(ctx, p, c, true)
}

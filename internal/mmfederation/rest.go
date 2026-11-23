// SPDX-License-Identifier: Apache-2.0
package mmfederation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

type restPosts struct {
	Order []string               `json:"order"`
	Posts map[string]*model.Post `json:"posts"`
}

func (s *Service) restHistory(ctx context.Context, p Peer, l Link, path string) (restPosts, error) {
	var posts restPosts
	response, e := s.request(ctx, p, "GET", path, nil, "")
	if e != nil {
		return posts, e
	}
	defer response.Body.Close()
	e = decodeJSON(response.Body, &posts, 16<<20)
	if len(posts.Posts) > 1000 {
		return posts, ErrInvalid
	}
	return posts, e
}
func (s *Service) syncREST(ctx context.Context, p Peer, l Link) error {
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
		if skip {
			if e = s.advanceLocal(ctx, l, post); e != nil {
				return e
			}
			continue
		}
		var already int
		if e = s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM chat_federation_posts WHERE link_id=? AND local_post_id=? AND origin='local'`), l.ID, post.ID).Scan(&already); e != nil {
			return e
		}
		if already > 0 {
			if e = s.advanceLocal(ctx, l, post); e != nil {
				return e
			}
			continue
		}
		// Reconcile delivery using an origin marker before retrying a lost response.
		serverID, e := s.Chat.ServerID(ctx)
		if e != nil {
			return e
		}
		var foreign string
		var attempt int
		if e = s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM chat_federation_attempts WHERE link_id=? AND local_post_id=?`), l.ID, post.ID).Scan(&attempt); e != nil {
			return e
		}
		if attempt > 0 {
			foreign, e = s.findRESTOrigin(ctx, p, l, serverID, post.ID)
			if e != nil {
				return e
			}
		}
		if foreign == "" {
			exported, _, e := s.exportModel(ctx, p, l, post)
			if e != nil {
				return e
			}
			exported.Id = ""
			exported.UserId = ""
			exported.CreateAt = 0
			exported.UpdateAt = 0
			exported.PendingPostId = post.ID
			author, e := s.Store.GetUserByID(ctx, post.UserID)
			if e != nil {
				return e
			}
			exported.Message = "**" + author.DisplayLabel() + " · Tayga**\n\n" + post.Message
			exported.AddProp("tms_federation_origin", map[string]string{"server_id": serverID, "post_id": post.ID})
			_, e = s.db.ExecContext(ctx, s.q(`INSERT INTO chat_federation_attempts(link_id,local_post_id,created_at) VALUES(?,?,?) ON CONFLICT DO NOTHING`), l.ID, post.ID, time.Now().UnixMilli())
			if e != nil {
				return e
			}
			var savedFiles string
			if e = s.db.QueryRowContext(ctx, s.q(`SELECT files FROM chat_federation_attempts WHERE link_id=? AND local_post_id=?`), l.ID, post.ID).Scan(&savedFiles); e != nil {
				return e
			}
			if e = json.Unmarshal([]byte(savedFiles), &exported.FileIds); e != nil {
				return e
			}
			if len(exported.FileIds) == 0 {
				exported.FileIds, e = s.pushRESTFiles(ctx, p, l, post)
				if e == nil {
					_, e = s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_attempts SET files=? WHERE link_id=? AND local_post_id=?`), snapshot(exported.FileIds), l.ID, post.ID)
				}
			}
			if e != nil {
				return e
			}
			raw, e := json.Marshal(exported)
			if e != nil {
				return e
			}
			response, e := s.request(ctx, p, "POST", "/api/v4/posts", bytes.NewReader(raw), "application/json")
			if e != nil {
				return e
			}
			var created model.Post
			e = decodeJSON(response.Body, &created, 1<<20)
			response.Body.Close()
			if e != nil || created.ChannelId != l.RemoteChannelID || !model.IsValidId(created.Id) {
				return ErrInvalid
			}
			foreign = created.Id
		}
		if e = s.recordExport(ctx, l, post, foreign); e != nil {
			return e
		}
		if e = s.advanceLocal(ctx, l, post); e != nil {
			return e
		}
		_, _ = s.db.ExecContext(ctx, s.q(`DELETE FROM chat_federation_attempts WHERE link_id=? AND local_post_id=?`), l.ID, post.ID)
	}
	if l.ReadOnly {
		return nil
	}
	// since uses the remote server's update clock, never a locally generated post ID.
	received := restPosts{Posts: map[string]*model.Post{}}
	for page := 0; page < 100; page++ {
		path := fmt.Sprintf("/api/v4/channels/%s/posts?since=%d&per_page=200&page=%d", l.RemoteChannelID, max(l.RemoteCursorAt-1, 0), page)
		batch, e := s.restHistory(ctx, p, l, path)
		if e != nil {
			return e
		}
		added := 0
		for id, post := range batch.Posts {
			if _, ok := received.Posts[id]; !ok {
				received.Posts[id] = post
				added++
			}
		}
		if len(received.Posts) >= 1000 {
			return fmt.Errorf("remote result exceeds safe synchronization window")
		}
		if len(batch.Posts) < 200 || added == 0 {
			break
		}
		if page == 99 {
			return ErrInvalid
		}
	}
	ordered := []*model.Post{}
	for _, post := range received.Posts {
		if post != nil {
			ordered = append(ordered, post)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CreateAt == ordered[j].CreateAt {
			return ordered[i].Id < ordered[j].Id
		}
		return ordered[i].CreateAt < ordered[j].CreateAt
	})
	cursor := l.RemoteCursorAt
	profiles := map[string]*model.User{}
	for _, post := range ordered {
		if !model.IsValidId(post.Id) || !model.IsValidId(post.UserId) || post.ChannelId != l.RemoteChannelID {
			return ErrForbidden
		}
		cursor = max(cursor, post.UpdateAt, post.CreateAt, post.DeleteAt)
		if post.GetProp("tms_federation") != nil || post.GetProp("tms_federation_origin") != nil {
			continue
		}
		if _, origin, _, e := s.mappedPost(ctx, l, post.Id); e == nil && origin == "local" {
			continue
		}
		if profiles[post.UserId] == nil {
			response, e := s.request(ctx, p, "GET", "/api/v4/users/"+post.UserId, nil, "")
			if e != nil {
				return e
			}
			var user model.User
			e = decodeJSON(response.Body, &user, 1<<20)
			response.Body.Close()
			if e != nil || user.Id != post.UserId {
				return ErrInvalid
			}
			profiles[user.Id] = &user
		}
		user, e := s.remoteUser(ctx, p, post.UserId, profiles[post.UserId], true)
		if e != nil {
			return e
		}
		if e = s.member(ctx, p, l, user); e != nil {
			return e
		}
		files, e := s.pullRESTFiles(ctx, p, l, post, user)
		if e != nil {
			return e
		}
		if _, e = s.importPost(ctx, p, l, post, profiles, files); e != nil {
			return e
		}
		cursor = max(cursor, post.UpdateAt, post.CreateAt, post.DeleteAt)
	}
	// Never silently advance past a potentially truncated response.
	if len(received.Posts) >= 1000 {
		return fmt.Errorf("remote result requires a smaller synchronization window")
	}
	_, e = s.db.ExecContext(ctx, s.q(`UPDATE chat_federation_links SET remote_cursor_at=? WHERE id=? AND enabled=1 AND lease_token=?`), cursor, l.ID, l.LeaseToken)
	return e
}
func (s *Service) findRESTOrigin(ctx context.Context, p Peer, l Link, serverID, postID string) (string, error) {
	// A complete paginated scan avoids duplicating a delivery whose HTTP response
	// was lost. Bounded scans fail closed instead of guessing that no post exists.
	for page := 0; page < 1000; page++ {
		result, e := s.restHistory(ctx, p, l, fmt.Sprintf("/api/v4/channels/%s/posts?page=%d&per_page=200", l.RemoteChannelID, page))
		if e != nil {
			return "", e
		}
		for _, post := range result.Posts {
			if post == nil || post.ChannelId != l.RemoteChannelID {
				continue
			}
			value, _ := post.GetProp("tms_federation_origin").(map[string]any)
			if value["server_id"] == serverID && value["post_id"] == postID {
				return post.Id, nil
			}
		}
		if len(result.Posts) < 200 {
			return "", nil
		}
	}
	return "", fmt.Errorf("origin reconciliation limit reached")
}

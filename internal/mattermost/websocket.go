package mattermost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tayga/tms/internal/chat"
	"golang.org/x/net/websocket"
)

// A fresh hello/connection_id on reconnect requires the client to resync REST
// history. This prototype does not advertise event replay across connections.
func (s *API) websocket(w http.ResponseWriter, r *http.Request) {
	server := websocket.Server{
		Handshake: func(c *websocket.Config, r *http.Request) error {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return nil
			}
			o, err := url.Parse(origin)
			if err != nil || o.Host == "" {
				return fmt.Errorf("invalid origin")
			}
			if strings.EqualFold(o.Host, r.Host) {
				return nil
			}
			base, err := url.Parse(s.siteURL)
			if err == nil && base.Host != "" && strings.EqualFold(o.Host, base.Host) {
				return nil
			}
			return fmt.Errorf("origin denied")
		},
		Handler: func(ws *websocket.Conn) { s.serveSocket(ws, r) },
	}
	server.ServeHTTP(w, r)
}
func (s *API) serveSocket(ws *websocket.Conn, r *http.Request) {
	defer ws.Close()
	ws.MaxPayloadBytes = 64 << 10
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	// Subscribe before authentication/history fetch to avoid a handshake gap.
	events, unsubscribe := s.chat.Subscribe()
	defer unsubscribe()
	type request struct {
		Seq    int64  `json:"seq"`
		Action string `json:"action"`
		Data   struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	token := bearer(r)
	challengeSeq := int64(0)
	challenged := token == ""
	if token == "" {
		_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
		var req request
		if err := websocket.JSON.Receive(ws, &req); err != nil || req.Action != "authentication_challenge" {
			return
		}
		token = req.Data.Token
		challengeSeq = req.Seq
	}
	u, err := s.auth.AuthenticateToken(ctx, "", token)
	if err != nil {
		return
	}
	if err = s.chat.EnsureDefault(ctx, u); err != nil {
		return
	}
	var writeMu sync.Mutex
	send := func(value any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return websocket.JSON.Send(ws, value)
	}
	seq := int64(0)
	event := func(name string, data map[string]any, channelID string) error {
		e := map[string]any{"event": name, "data": data, "broadcast": map[string]any{"channel_id": channelID, "team_id": "", "user_id": "", "omit_users": map[string]bool{}}, "seq": seq}
		seq++
		return send(e)
	}
	if challenged {
		if send(map[string]any{"status": "OK", "seq_reply": challengeSeq}) != nil {
			return
		}
	}
	if event("hello", map[string]any{"server_version": CompatibilityVersion, "connection_id": chat.ID(fmt.Sprintf("%s:%d", u.ID, time.Now().UnixNano()))}, "") != nil {
		return
	}
	_ = ws.SetReadDeadline(time.Now().Add(90 * time.Second))
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer ws.Close()
		for {
			var req request
			if websocket.JSON.Receive(ws, &req) != nil {
				return
			}
			_ = ws.SetReadDeadline(time.Now().Add(90 * time.Second))
			if _, err := s.auth.AuthenticateToken(ctx, "", token); err != nil {
				return
			}
			if req.Action == "ping" {
				if send(map[string]any{"status": "OK", "seq_reply": req.Seq, "data": map[string]string{"text": "pong"}}) != nil {
					return
				}
			} else {
				if send(map[string]any{"status": "FAIL", "seq_reply": req.Seq, "error": map[string]any{"id": "tms.mattermost.unsupported", "message": "Unsupported websocket action", "status_code": 501}}) != nil {
					return
				}
			}
		}
	}()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			if _, err := s.auth.AuthenticateToken(ctx, "", token); err != nil {
				return
			}
		case ev, ok := <-events:
			if !ok {
				return
			}
			current, err := s.auth.AuthenticateToken(ctx, "", token)
			if err != nil {
				return
			}
			c, err := s.chat.Channel(ctx, current, ev.Post.ChannelID)
			if err != nil {
				continue
			}
			raw, err := json.Marshal(postModel(ev.Post))
			if err != nil {
				return
			}
			if event("posted", map[string]any{"post": string(raw), "channel_type": c.Kind, "channel_name": c.Name, "channel_display_name": c.DisplayName, "sender_name": "", "team_id": chat.ID(c.TenantID), "mentions": "[]"}, c.ID) != nil {
				return
			}
		}
	}
}

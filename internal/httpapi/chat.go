package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/xmpp"
)

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if s.xmpp == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "chat unavailable"})
		return
	}
	au, err := s.userFromBearer(r)
	if err != nil {
		// EventSource cannot set Authorization — allow ?access_token=
		if tok := r.URL.Query().Get("access_token"); tok != "" {
			u, err2 := s.authn.AuthenticateToken(r.Context(), "", tok)
			if err2 == nil {
				au = &authUser{ID: u.ID, Email: u.Email}
				err = nil
			}
		}
	}
	if err != nil || au == nil {
		if err == nil {
			err = auth.ErrInvalidToken
		}
		writeAuthError(w, err)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/chat"), "/")
	if path == "rooms/omemo" {
		bridge, ok := s.xmpp.(interface {
			RoomOMEMOContext(context.Context, string, string) (xmpp.RoomOMEMOContext, error)
			SendRoomOMEMO(context.Context, string, string, string, string) (xmpp.ChatEvent, error)
		})
		if !ok {
			writeJSON(w, 503, map[string]string{"error": "group OMEMO unavailable"})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		var result any
		var err error
		switch r.Method {
		case http.MethodGet:
			result, err = bridge.RoomOMEMOContext(r.Context(), au.Email, r.URL.Query().Get("room"))
		case http.MethodPost:
			var req struct {
				Room     string `json:"room"`
				Revision string `json:"revision"`
				XML      string `json:"xml"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 512*1024)).Decode(&req) != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid group OMEMO request"})
				return
			}
			result, err = bridge.SendRoomOMEMO(r.Context(), au.Email, req.Room, req.Revision, req.XML)
		default:
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		if err != nil {
			status := 400
			if errors.Is(err, xmpp.ErrRoomAccess) {
				status = 403
			}
			if errors.Is(err, xmpp.ErrRoomChanged) {
				status = 409
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if path == "rooms/history" {
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		archive, ok := s.xmpp.(interface {
			RoomHistoryPage(context.Context, string, string, string, int) (xmpp.RoomArchivePage, error)
		})
		if !ok {
			writeJSON(w, 503, map[string]string{"error": "room archive unavailable"})
			return
		}
		q := r.URL.Query()
		limit := 80
		if q.Get("limit") != "" {
			n, e := strconv.Atoi(q.Get("limit"))
			if e != nil || n < 1 || n > 100 {
				writeJSON(w, 400, map[string]string{"error": "limit must be 1–100"})
				return
			}
			limit = n
		}
		page, e := archive.RoomHistoryPage(r.Context(), au.Email, q.Get("room"), q.Get("before"), limit)
		if e != nil {
			status := 500
			if errors.Is(e, xmpp.ErrRoomAccess) {
				status = 403
			} else if errors.Is(e, xmpp.ErrRoomCursor) {
				status = 404
			}
			writeJSON(w, status, map[string]string{"error": e.Error()})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, 200, page)
		return
	}
	if path == "rooms" || strings.HasPrefix(path, "rooms/") {
		groups, ok := s.xmpp.(interface {
			CreateChatRoom(context.Context, string, string, []string) (xmpp.ChatRoom, error)
			ListChatRooms(context.Context, string) ([]xmpp.ChatRoom, error)
			LeaveChatRoom(context.Context, string, string) error
			UpdateChatRoom(context.Context, string, string, string, string, bool) error
		})
		if !ok {
			writeJSON(w, 503, map[string]string{"error": "groups unavailable"})
			return
		}
		switch {
		case strings.HasPrefix(path, "rooms/") && r.Method == http.MethodPatch:
			var req struct {
				Email       string `json:"email"`
				Affiliation string `json:"affiliation"`
				Destroy     bool   `json:"destroy"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid room update"})
				return
			}
			if err := groups.UpdateChatRoom(r.Context(), au.Email, strings.TrimPrefix(path, "rooms/"), req.Email, req.Affiliation, req.Destroy); err != nil {
				writeJSON(w, 403, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
		case path == "rooms" && r.Method == http.MethodGet:
			rooms, e := groups.ListChatRooms(r.Context(), au.Email)
			if e != nil {
				writeJSON(w, 500, map[string]string{"error": e.Error()})
				return
			}
			writeJSON(w, 200, map[string]any{"rooms": rooms})
		case path == "rooms" && r.Method == http.MethodPost:
			var req struct {
				Name    string   `json:"name"`
				Members []string `json:"members"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req) != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid group"})
				return
			}
			room, e := groups.CreateChatRoom(r.Context(), au.Email, req.Name, req.Members)
			if e != nil {
				writeJSON(w, 400, map[string]string{"error": e.Error()})
				return
			}
			writeJSON(w, 201, room)
		case strings.HasPrefix(path, "rooms/") && r.Method == http.MethodDelete:
			if e := groups.LeaveChatRoom(r.Context(), au.Email, strings.TrimPrefix(path, "rooms/")); e != nil {
				writeJSON(w, 403, map[string]string{"error": e.Error()})
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
		default:
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		}
		return
	}
	switch {
	case path == "omemo/devices":
		bridge, ok := s.xmpp.(interface {
			RetiredWebOMEMO(context.Context, string) ([]xmpp.RetiredOMEMODevice, error)
			RetireWebOMEMO(context.Context, string, uint32, string) error
		})
		if !ok {
			writeJSON(w, 503, map[string]string{"error": "OMEMO retirement unavailable"})
			return
		}
		if r.Method == http.MethodGet {
			devices, err := bridge.RetiredWebOMEMO(r.Context(), au.Email)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "cannot read retired keys"})
				return
			}
			writeJSON(w, 200, map[string]any{"retired": devices})
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		var req struct {
			ID          uint32 `json:"id"`
			IdentityKey string `json:"identity_key"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid device"})
			return
		}
		if err := bridge.RetireWebOMEMO(r.Context(), au.Email, req.ID, req.IdentityKey); err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, xmpp.ErrOMEMOKeyChanged) {
				status = http.StatusConflict
			}
			if errors.Is(err, sql.ErrNoRows) {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	case path == "omemo":
		bridge, ok := s.xmpp.(interface {
			ReadWebOMEMO(context.Context, string, string) ([]xmpp.OMEMOItem, error)
			PublishWebOMEMO(context.Context, string, string, string) error
			SendWebOMEMO(context.Context, string, string, string) (xmpp.ChatEvent, error)
		})
		if !ok {
			writeJSON(w, 503, map[string]string{"error": "OMEMO unavailable"})
			return
		}
		if r.Method == http.MethodGet {
			items, e := bridge.ReadWebOMEMO(r.Context(), r.URL.Query().Get("owner"), r.URL.Query().Get("node"))
			if e != nil {
				writeJSON(w, 400, map[string]string{"error": e.Error()})
				return
			}
			writeJSON(w, 200, map[string]any{"items": items})
			return
		}
		var req struct {
			Node string `json:"node"`
			XML  string `json:"xml"`
			To   string `json:"to"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 512*1024)).Decode(&req) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid OMEMO request"})
			return
		}
		if r.Method == http.MethodPut {
			if e := bridge.PublishWebOMEMO(r.Context(), au.Email, req.Node, req.XML); e != nil {
				writeJSON(w, 400, map[string]string{"error": e.Error()})
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		if r.Method == http.MethodPost {
			ev, e := bridge.SendWebOMEMO(r.Context(), au.Email, req.To, req.XML)
			if e != nil {
				writeJSON(w, 400, map[string]string{"error": e.Error()})
				return
			}
			writeJSON(w, 200, ev)
			return
		}
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	case path == "presence" && r.Method == http.MethodPost:
		var req struct {
			State  string `json:"state"`
			Status string `json:"status"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid presence"})
			return
		}
		if err := s.xmpp.SetChatPresence(r.Context(), au.ID, req.State, req.Status); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"state": req.State})
	case path == "subscription" && r.Method == http.MethodPost:
		var req struct {
			JID    string `json:"jid"`
			Action string `json:"action"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid subscription"})
			return
		}
		if err := s.xmpp.ChatSubscription(r.Context(), au.ID, req.JID, req.Action); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	case path == "roster" && r.Method == http.MethodGet:
		list, err := s.xmpp.ListChatRoster(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		presence := "online"
		if raw, ok, e := s.store.GetSetting(r.Context(), "xmpp.presence."+au.ID); e == nil && ok {
			var saved struct {
				State string `json:"state"`
			}
			if json.Unmarshal([]byte(raw), &saved) == nil {
				presence = saved.State
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"roster": list, "me": au.Email, "presence": presence})
	case path == "roster" && r.Method == http.MethodPost:
		var req struct {
			JID  string `json:"jid"`
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.JID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "jid required"})
			return
		}
		if err := s.xmpp.UpsertChatRoster(r.Context(), au.ID, req.JID, req.Name); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case path == "history" && r.Method == http.MethodGet:
		with := r.URL.Query().Get("with")
		if with == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "with required"})
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		msgs, err := s.xmpp.ChatHistory(r.Context(), au.Email, with, limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
	case path == "send" && r.Method == http.MethodPost:
		var req struct {
			To   string `json:"to"`
			Body string `json:"body"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 131072)).Decode(&req); err != nil || req.To == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to required"})
			return
		}
		ev, err := s.xmpp.SendChatMessage(r.Context(), au.Email, req.To, req.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if ev.Room == "" {
			_ = s.xmpp.UpsertChatRoster(r.Context(), au.ID, req.To, "")
		}
		writeJSON(w, http.StatusOK, ev)
	case path == "events" && r.Method == http.MethodGet:
		s.handleChatSSE(w, r, au.Email)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Server) handleChatSSE(w http.ResponseWriter, r *http.Request, email string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stream unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsub := s.xmpp.SubscribeChat(email)
	defer unsub()

	_, _ = fmt.Fprintf(w, "event: ready\ndata: {\"me\":%q}\n\n", email)
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", b)
			flusher.Flush()
		}
	}
}

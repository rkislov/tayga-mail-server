package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

type agentScheduleSpec struct {
	Prompt    string         `json:"prompt"`
	LocalTime string         `json:"local_time"`
	Timezone  string         `json:"timezone"`
	Frequency string         `json:"frequency"`
	Weekdays  []time.Weekday `json:"weekdays"`
	Paused    bool           `json:"paused"`
}

type agentScheduleView struct {
	agentScheduleSpec
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	NextAt         int64  `json:"next_at"`
	LastAt         int64  `json:"last_at"`
	Status         string `json:"status"`
}

const agentScheduleTimeFormat = "2006-01-02T15:04"

func (spec agentScheduleSpec) next(now time.Time) (int64, error) {
	if len(spec.Timezone) > 100 || spec.Timezone == "" || spec.Timezone == "Local" {
		return 0, chat.ErrInvalid
	}
	zone, err := time.LoadLocation(spec.Timezone)
	if err != nil {
		return 0, chat.ErrInvalid
	}
	wall, err := time.Parse(agentScheduleTimeFormat, spec.LocalTime)
	if err != nil {
		return 0, chat.ErrInvalid
	}
	if spec.Frequency != "once" && spec.Frequency != "daily" && spec.Frequency != "weekly" {
		return 0, chat.ErrInvalid
	}
	if len(spec.Weekdays) > 7 || (spec.Frequency == "weekly" && len(spec.Weekdays) == 0) {
		return 0, chat.ErrInvalid
	}
	days := make(map[time.Weekday]bool)
	for _, day := range spec.Weekdays {
		if day < time.Sunday || day > time.Saturday {
			return 0, chat.ErrInvalid
		}
		days[day] = true
	}
	local := func(day time.Time) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), wall.Hour(), wall.Minute(), 0, 0, zone)
	}
	first := local(wall)
	if spec.Frequency == "once" {
		if first.Format(agentScheduleTimeFormat) != spec.LocalTime {
			return 0, chat.ErrInvalid
		}
		if spec.Paused || !first.After(now) {
			return 0, nil
		}
		return first.UnixMilli(), nil
	}
	if spec.Paused {
		return 0, nil
	}
	current := now.In(zone)
	day := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC)
	firstDay := time.Date(wall.Year(), wall.Month(), wall.Day(), 0, 0, 0, 0, time.UTC)
	if day.Before(firstDay) {
		day = firstDay
	}
	for i := 0; i < 15; i++ {
		candidate := local(day)
		// Skip a wall time that does not exist during the spring DST jump.
		if candidate.Hour() == wall.Hour() && candidate.Minute() == wall.Minute() && candidate.After(now) &&
			(spec.Frequency == "daily" || days[day.Weekday()]) {
			return candidate.UnixMilli(), nil
		}
		day = day.AddDate(0, 0, 1)
	}
	return 0, chat.ErrInvalid
}

func scheduleView(item storage.AgentSchedule) (agentScheduleView, error) {
	view := agentScheduleView{ID: item.ID, ConversationID: item.ConversationID, NextAt: item.NextAt, LastAt: item.LastAt, Status: item.Status}
	err := json.Unmarshal([]byte(item.Spec), &view.agentScheduleSpec)
	return view, err
}

func (s *API) agentScheduleHTTP(w http.ResponseWriter, r *http.Request, u *storage.User, path string) {
	ctx := r.Context()
	scope := userScope(u).Key()
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet && path == "" {
		items, err := s.store.ListAgentSchedules(ctx, u.ID, scope)
		if err != nil {
			s.failure(w, err)
			return
		}
		out := []agentScheduleView{}
		for _, item := range items {
			view, err := scheduleView(item)
			if err != nil {
				s.failure(w, err)
				return
			}
			out = append(out, view)
		}
		write(w, 200, out)
		return
	}
	id := strings.TrimPrefix(path, "/")
	switch r.Method {
	case http.MethodDelete:
		if len(id) != 32 {
			s.failure(w, storage.ErrNotFound)
			return
		}
		if err := s.store.DeleteAgentSchedule(ctx, id, u.ID, scope); err != nil {
			s.failure(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPost, http.MethodPut:
		creating := r.Method == http.MethodPost
		if creating && path != "" || !creating && len(id) != 32 {
			s.failure(w, storage.ErrNotFound)
			return
		}
		var payload struct {
			agentScheduleSpec
			ConversationID string `json:"conversation_id"`
		}
		if !decode(w, r, &payload) {
			return
		}
		payload.Prompt = strings.TrimSpace(payload.Prompt)
		if len(payload.Prompt) == 0 || len(payload.Prompt) > 8000 {
			s.failure(w, chat.ErrInvalid)
			return
		}
		next, err := payload.next(time.Now())
		if err != nil || next == 0 && !payload.Paused {
			fail(w, 400, "tms.agents.schedule_time", "Укажите действительные будущие дату и время задания")
			return
		}
		raw, err := json.Marshal(payload.agentScheduleSpec)
		if err != nil {
			s.failure(w, err)
			return
		}
		s.agentsMu.Lock()
		defer s.agentsMu.Unlock()
		item := storage.AgentSchedule{ID: id, UserID: u.ID, Scope: scope, ConversationID: payload.ConversationID, Spec: string(raw), NextAt: next}
		if creating {
			if err = s.scheduleConversation(ctx, u, payload.ConversationID); err != nil {
				s.failure(w, err)
				return
			}
			item.ID = agentID()
			err = s.store.CreateAgentSchedule(ctx, item)
		} else {
			err = s.store.UpdateAgentSchedule(ctx, item)
		}
		if errors.Is(err, storage.ErrAlreadyExists) {
			fail(w, 409, "tms.agents.schedule_limit", "В личном расписании может быть до 20 заданий")
			return
		}
		if err != nil {
			s.failure(w, err)
			return
		}
		s.agentAudit(u, "schedule_saved", item.ID)
		w.WriteHeader(http.StatusNoContent)
	default:
		wrongMethod(w)
	}
}

func (s *API) scheduleConversation(ctx context.Context, u *storage.User, id string) error {
	h := agentHistory{}
	if err := s.agentLoad(ctx, agentHistoryKey(u), &h); err != nil {
		return err
	}
	for _, conversation := range h.Conversations {
		if conversation.ID == id {
			_, err := s.agentBot(ctx, u, conversation.AgentID)
			return err
		}
	}
	return storage.ErrNotFound
}

type agentScheduledKey struct{}
type agentScheduledRun struct {
	item  storage.AgentSchedule
	claim string
}

func (s *API) agentRunActor(ctx context.Context, r *http.Request) (*storage.User, error) {
	run, ok := r.Context().Value(agentScheduledKey{}).(agentScheduledRun)
	if !ok {
		return s.authenticate(ctx, bearer(r))
	}
	active, err := s.store.AgentScheduleClaimActive(ctx, run.item.ID, run.claim, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, chat.ErrForbidden
	}
	u, err := s.store.GetUserByID(ctx, run.item.UserID)
	if err != nil {
		return nil, err
	}
	if !u.Enabled || u.AuthSource == "bot" || u.AuthSource == "federated" || userScope(u).Key() != run.item.Scope {
		return nil, chat.ErrForbidden
	}
	policy, err := s.policy.Resolve(ctx, userScope(u))
	if err != nil {
		return nil, err
	}
	if !policy.Enabled {
		return nil, chat.ErrForbidden
	}
	return u, nil
}

type agentScheduleResponse struct {
	status  int
	errorID string
}

func (w *agentScheduleResponse) Header() http.Header { return make(http.Header) }
func (w *agentScheduleResponse) Write(b []byte) (int, error) {
	if w.status == 409 {
		var response struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(b, &response) == nil {
			w.errorID = response.ID
		}
	}
	return len(b), nil
}
func (w *agentScheduleResponse) WriteHeader(status int) { w.status = status }

func (s *API) runAgentSchedule(ctx context.Context, item storage.AgentSchedule, now time.Time) error {
	claim := agentID()
	claimed, err := s.store.ClaimAgentSchedule(ctx, item, claim, now.UnixMilli(), now.Add(4*time.Minute).UnixMilli())
	if err != nil || !claimed {
		return err
	}
	next, last, retryAt := int64(0), now.UnixMilli(), int64(0)
	status := "unavailable"
	spec, parseErr := scheduleView(item)
	if parseErr == nil {
		runCtx, cancel := context.WithTimeout(context.WithValue(ctx, agentScheduledKey{}, agentScheduledRun{item, claim}), 140*time.Second)
		defer cancel()
		body, _ := json.Marshal(map[string]string{"content": spec.Prompt, "pending_post_id": fmt.Sprintf("job-%s-%d", item.ID, item.NextAt)})
		r, reqErr := http.NewRequestWithContext(runCtx, http.MethodPost, "/api/v4/tms/agents/conversations/"+item.ConversationID+"/messages", bytes.NewReader(body))
		if reqErr != nil {
			return reqErr
		}
		if u, err := s.agentRunActor(runCtx, r); err == nil {
			w := &agentScheduleResponse{}
			s.agentTurn(w, r, u, item.ConversationID)
			switch w.status {
			case 200:
				status = "completed"
			case 429:
				status = "waiting"
				next, last, retryAt = item.NextAt, item.LastAt, time.Now().Add(30*time.Second).UnixMilli()
			case 409:
				status = "conversation_busy"
				if w.errorID == "tms.agents.busy" {
					status = "waiting"
					next, last, retryAt = item.NextAt, item.LastAt, time.Now().Add(30*time.Second).UnixMilli()
				}
			case 502:
				status = "error"
			}
		}
		if status != "waiting" && spec.Frequency != "once" {
			next, _ = spec.next(maxTime(now, time.Now()))
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return s.store.FinishAgentSchedule(ctx, item.ID, claim, status, next, last, retryAt)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func (s *API) StartAgentBackground(ctx context.Context) {
	go func() {
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		var nextCleanup time.Time
		for ctx.Err() == nil {
			now := time.Now()
			if !now.Before(nextCleanup) {
				if err := s.store.CleanupAgentArtifacts(ctx, now.UnixMilli()); err != nil && ctx.Err() == nil {
					s.log.Warn("agent artifact cleanup failed", "error", err)
				}
				nextCleanup = now.Add(time.Minute)
			}
			if err := s.runDueAgentSchedules(ctx, now); err != nil && ctx.Err() == nil {
				s.log.Warn("agent scheduler failed", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
}

func (s *API) runDueAgentSchedules(ctx context.Context, now time.Time) error {
	items, err := s.store.DueAgentSchedules(ctx, now.UnixMilli())
	if err != nil {
		return err
	}
	var group sync.WaitGroup
	for _, item := range items[:min(len(items), 2)] {
		group.Go(func() {
			if err := s.runAgentSchedule(ctx, item, now); err != nil && ctx.Err() == nil {
				s.log.Warn("scheduled agent task failed", "id", item.ID, "error", err)
			}
		})
	}
	group.Wait()
	return nil
}

package notify

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/tayga/tms/internal/storage"
)

// ReminderLoop scans calendars for events starting soon and notifies online users.
type ReminderLoop struct {
	Hub    *Hub
	Store  storage.Driver
	Log    *slog.Logger
	Lead   time.Duration // how far ahead to remind (default 15m)
	Poll   time.Duration // scan interval (default 1m)

	mu    sync.Mutex
	fired map[string]time.Time // key userID|objectID → fired at
}

// Start runs until ctx is cancelled.
func (r *ReminderLoop) Start(ctx context.Context) {
	if r == nil || r.Hub == nil || r.Store == nil {
		return
	}
	if r.Log == nil {
		r.Log = slog.Default()
	}
	if r.Lead <= 0 {
		r.Lead = 15 * time.Minute
	}
	if r.Poll <= 0 {
		r.Poll = time.Minute
	}
	r.fired = make(map[string]time.Time)
	go r.loop(ctx)
}

func (r *ReminderLoop) loop(ctx context.Context) {
	t := time.NewTicker(r.Poll)
	defer t.Stop()
	r.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.tick(ctx)
		}
	}
}

func (r *ReminderLoop) tick(ctx context.Context) {
	ids := r.Hub.OnlineUserIDs()
	if len(ids) == 0 {
		return
	}
	now := time.Now().UTC()
	until := now.Add(r.Lead)
	for _, userID := range ids {
		cals, err := r.Store.ListCalendars(ctx, userID)
		if err != nil {
			continue
		}
		for _, cal := range cals {
			objs, err := r.Store.ListCalendarObjects(ctx, cal.ID)
			if err != nil {
				continue
			}
			for _, o := range objs {
				if o.DTStart == nil {
					continue
				}
				start := o.DTStart.UTC()
				if start.Before(now) || start.After(until) {
					continue
				}
				key := userID + "|" + o.ID
				r.mu.Lock()
				prev, ok := r.fired[key]
				if ok && now.Sub(prev) < r.Lead+time.Minute {
					r.mu.Unlock()
					continue
				}
				r.fired[key] = now
				r.mu.Unlock()

				title := o.UID
				if o.HrefName != "" {
					title = o.HrefName
				}
				// Prefer SUMMARY from stored ICS if present in Data
				if sum := icsSummary(o.Data); sum != "" {
					title = sum
				}
				when := start.Local().Format("15:04")
				r.Hub.PublishCalendar(userID, title, when)
			}
		}
	}
	// prune old fired keys
	r.mu.Lock()
	for k, at := range r.fired {
		if now.Sub(at) > 2*r.Lead {
			delete(r.fired, k)
		}
	}
	r.mu.Unlock()
}

func icsSummary(data string) string {
	// lightweight scan for SUMMARY: line
	const prefix = "SUMMARY"
	for i := 0; i+len(prefix) < len(data); i++ {
		if (i == 0 || data[i-1] == '\n') && len(data) >= i+len(prefix) &&
			(data[i:i+7] == "SUMMARY" || data[i:i+8] == "SUMMARY;") {
			j := i
			for j < len(data) && data[j] != ':' {
				j++
			}
			if j >= len(data) || data[j] != ':' {
				continue
			}
			j++
			end := j
			for end < len(data) && data[end] != '\n' && data[end] != '\r' {
				end++
			}
			s := data[j:end]
			// unfold simple trailing spaces
			for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
				s = s[:len(s)-1]
			}
			if s != "" {
				return s
			}
		}
	}
	return ""
}

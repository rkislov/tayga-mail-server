package mattermost

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

type agentCalendarEvent struct {
	Calendar string    `json:"calendar"`
	Title    string    `json:"title"`
	Location string    `json:"location,omitempty"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	AllDay   bool      `json:"all_day"`
}

func (s *API) agentCalendarEvents(ctx context.Context, u *storage.User, fromText, toText string) (any, error) {
	from, fromErr := time.Parse(time.RFC3339, fromText)
	to, toErr := time.Parse(time.RFC3339, toText)
	if fromErr != nil || toErr != nil || !from.Before(to) || to.After(from.AddDate(0, 0, 31)) {
		return nil, chat.ErrInvalid
	}
	const candidateLimit = 256
	const resultLimit = 40
	candidates, err := s.store.ListOwnAgendaCandidates(ctx, u.ID, from.UTC(), to.UTC(), candidateLimit+1)
	if err != nil {
		return nil, err
	}
	incomplete := len(candidates) > candidateLimit
	if incomplete {
		candidates = candidates[:candidateLimit]
	}
	events := make([]agentCalendarEvent, 0)
	for _, event := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if event.Data == "" {
			incomplete = true
			continue
		}
		occurrences, err := calutil.ExpandOccurrences(event.Data, from, to)
		if err != nil {
			incomplete = true
			continue
		}
		for _, item := range occurrences {
			if item.Start == nil || item.End == nil || strings.EqualFold(item.Status, "CANCELLED") {
				continue
			}
			events = append(events, agentCalendarEvent{
				Calendar: clipAgentText(event.CalendarName, 128),
				Title:    clipAgentText(item.Summary, 300),
				Location: clipAgentText(item.Location, 240),
				Start:    *item.Start, End: *item.End, AllDay: item.AllDay,
			})
		}
		sort.SliceStable(events, func(i, j int) bool { return events[i].Start.Before(events[j].Start) })
		if len(events) > resultLimit {
			events = events[:resultLimit]
			incomplete = true
		}
	}
	return map[string]any{"events": events, "incomplete": incomplete, "from": from, "to": to}, nil
}

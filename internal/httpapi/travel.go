package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleTravel(w http.ResponseWriter, r *http.Request) {
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	parts := splitPath(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/travel"), "/"))
	switch {
	case len(parts) == 1 && parts[0] == "preferences":
		if r.Method == http.MethodPut {
			var req struct {
				Mode string `json:"mode"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil || (req.Mode != "off" && req.Mode != "suggest" && req.Mode != "auto") {
				writeJSON(w, 400, map[string]string{"error": "invalid mode"})
				return
			}
			if err = s.store.SetTravelMode(r.Context(), u.ID, req.Mode); err != nil {
				writeJSON(w, 500, map[string]string{"error": "cannot save preference"})
				return
			}
		} else if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		mode, err := s.store.TravelMode(r.Context(), u.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot read preference"})
			return
		}
		writeJSON(w, 200, map[string]string{"mode": mode})
	case len(parts) == 0 && r.Method == http.MethodGet:
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset < 0 {
			offset = 0
		}
		items, total, err := s.store.ListTravelCandidates(r.Context(), u.ID, "", 10, offset)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot list trips"})
			return
		}
		writeJSON(w, 200, map[string]any{"candidates": items, "total": total, "limit": 10, "offset": offset})
	case len(parts) == 1 && r.Method == http.MethodGet:
		c, err := s.store.GetTravelCandidate(r.Context(), u.ID, parts[0])
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, 200, c)
	case len(parts) == 2 && parts[1] == "ignore" && r.Method == http.MethodPost:
		if err = s.store.IgnoreTravelCandidate(r.Context(), u.ID, parts[0]); err != nil {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ignored"})
	case len(parts) == 2 && parts[1] == "event" && r.Method == http.MethodPost:
		var req struct {
			Start, End time.Time
			ETag       string
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || req.Start.IsZero() || !req.End.After(req.Start) || req.End.Sub(req.Start) > 31*24*time.Hour {
			writeJSON(w, 400, map[string]string{"error": "valid start/end with timezone required"})
			return
		}
		event, err := s.store.SaveTravelEvent(r.Context(), u.ID, parts[0], req.Start, req.End, req.ETag, false)
		if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrUnauthorized) {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		if errors.Is(err, storage.ErrCalendarConflict) {
			writeJSON(w, 409, map[string]string{"error": "trip or calendar event changed; reopen the review"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot save trip"})
			return
		}
		writeJSON(w, 200, map[string]string{"event_id": event.ID, "calendar_id": event.CalendarID, "etag": event.ETag})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) runTravelJobs(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.drainTravelJobs(ctx); err != nil && ctx.Err() == nil {
			s.log.Warn("travel queue unavailable")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Server) drainTravelJobs(ctx context.Context) error {
	for i := 0; i < 16; i++ {
		job, err := s.store.ClaimTravelJob(ctx)
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		jobCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		warning, err := s.processTravelJob(jobCtx, job)
		cancel()
		if err != nil {
			warning = "analysis_failed"
			s.log.Warn("travel analysis retry", "message_id", job.MessageID, "attempt", job.Attempts)
		}
		if e := s.store.FinishTravelJob(ctx, job, warning, err != nil); e != nil {
			return e
		}
	}
	return nil
}
func (s *Server) processTravelJob(ctx context.Context, job *storage.TravelJob) (string, error) {
	mode, err := s.store.TravelMode(ctx, job.UserID)
	if err != nil {
		return "", err
	}
	if mode == "off" {
		return "", nil
	}
	msg, err := s.store.GetMessageByID(ctx, job.MessageID)
	if errors.Is(err, storage.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	mb, err := s.store.GetMailboxByID(ctx, msg.MailboxID)
	if err != nil {
		return "", err
	}
	if mb.UserID != job.UserID {
		return "", storage.ErrUnauthorized
	}
	switch strings.ToLower(mb.Name) {
	case "spam", "junk", "trash", "deleted items", "sent", "sent items", "drafts":
		return "", nil
	}
	if msg.Size > 32<<20 {
		return "message_size_limit", nil
	}
	raw, err := s.ms.Read(msg.FilePath)
	if err != nil {
		return "", err
	}
	tickets, warnings := extractTravelTickets(ctx, raw)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	items, err := s.store.RecordTravelCandidates(ctx, job.UserID, job.MessageID, tickets)
	if err != nil {
		return "", err
	}
	if mode == "auto" && len(warnings) == 0 {
		for _, c := range items {
			if c.State != "review" || c.EventID != "" || !c.Ticket.Automatic() {
				continue
			}
			start, end, _ := c.Ticket.Times()
			_, err = s.store.SaveTravelEvent(ctx, job.UserID, c.ID, start, end, "", true)
			if err != nil && !errors.Is(err, storage.ErrCalendarConflict) {
				return "", err
			}
		}
	}
	return strings.Join(warnings, ","), nil
}

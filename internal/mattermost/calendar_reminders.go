package mattermost

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

const calendarChatLead = 15 * time.Minute

func (s *API) calendarReminderState(ctx context.Context, u *storage.User) (available, enabled bool, err error) {
	if !u.Enabled || u.AuthSource == "bot" || u.AuthSource == "federated" {
		return false, false, chat.ErrForbidden
	}
	policy, err := s.policy.Resolve(ctx, userScope(u))
	if err != nil {
		return false, false, err
	}
	raw, _, err := s.store.GetSetting(ctx, storage.CalendarChatSubscriptionPrefix+u.ID)
	return policy.Enabled && policy.CalendarReminders, raw == "true", err
}

func (s *API) calendarReminderHTTP(w http.ResponseWriter, r *http.Request, u *storage.User) {
	available, enabled, err := s.calendarReminderState(r.Context(), u)
	if err != nil {
		s.failure(w, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPut:
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if !decode(w, r, &body) {
			return
		}
		if body.Enabled == nil {
			s.failure(w, chat.ErrInvalid)
			return
		}
		enabled = *body.Enabled
		if err := s.setCalendarReminder(r.Context(), u, available, enabled); err != nil {
			s.failure(w, err)
			return
		}
	default:
		wrongMethod(w)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, map[string]bool{"available": available, "enabled": enabled})
}

func (s *API) setCalendarReminder(ctx context.Context, u *storage.User, available, enabled bool) error {
	if enabled && !available {
		return chat.ErrForbidden
	}
	return s.store.PutSetting(ctx, storage.CalendarChatSubscriptionPrefix+u.ID, fmt.Sprint(enabled))
}

func (s *API) calendarReminderCommand(w http.ResponseWriter, r *http.Request, u *storage.User, args []string) {
	available, enabled, err := s.calendarReminderState(r.Context(), u)
	if err != nil {
		s.failure(w, err)
		return
	}
	text := ""
	if len(args) > 1 || len(args) == 1 && args[0] != "on" && args[0] != "off" && args[0] != "status" {
		text = "Используйте /calendar-reminders on, off или status."
	} else if len(args) == 1 && args[0] != "status" && (available || args[0] == "off") {
		enabled = args[0] == "on"
		if err := s.setCalendarReminder(r.Context(), u, available, enabled); err != nil {
			s.failure(w, err)
			return
		}
	}
	if text == "" {
		switch {
		case !available:
			text = "Личные напоминания календаря выключены администратором домена."
		case enabled:
			text = "Личные напоминания включены. Бот «Календарь» напишет примерно за 15 минут до события."
		default:
			text = "Личные напоминания выключены. Для включения используйте /calendar-reminders on."
		}
	}
	write(w, 200, &model.CommandResponse{ResponseType: "ephemeral", Text: text})
}

func (s *API) StartCalendarChatReminders(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			run, cancel := context.WithTimeout(ctx, 45*time.Second)
			s.sendCalendarChatReminders(run, time.Now().UTC())
			cancel()
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *API) sendCalendarChatReminders(ctx context.Context, now time.Time) {
	cursor := ""
	for ctx.Err() == nil {
		const pageSize = 100
		ids, err := s.store.CalendarChatSubscribers(ctx, cursor, pageSize)
		if err != nil {
			if ctx.Err() == nil {
				s.log.Warn("calendar chat subscriber lookup failed", "error", err)
			}
			return
		}
		for _, id := range ids {
			if err := s.sendCalendarChatRemindersForUser(ctx, id, now); err != nil &&
				ctx.Err() == nil && !errors.Is(err, storage.ErrNotFound) && !errors.Is(err, chat.ErrForbidden) {
				s.log.Warn("calendar chat reminder failed", "error", err)
			}
		}
		if len(ids) < pageSize {
			return
		}
		cursor = ids[len(ids)-1]
	}
}

func (s *API) sendCalendarChatRemindersForUser(ctx context.Context, userID string, now time.Time) error {
	u, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	available, enabled, err := s.calendarReminderState(ctx, u)
	if err != nil || !available || !enabled {
		return err
	}
	until := now.Add(calendarChatLead)
	candidates, err := s.store.ListOwnAgendaCandidates(ctx, u.ID, now, until, 256)
	if err != nil {
		return err
	}
	ctx = chat.WithDomainScope(ctx, u.DomainID)
	var sender *storage.User
	var dm chat.Channel
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		occurrences, err := calutil.ExpandOccurrences(candidate.Data, now, until)
		if err != nil {
			continue
		}
		for _, event := range occurrences {
			if event.Start == nil || event.Start.Before(now) || !event.Start.Before(until) {
				continue
			}
			available, enabled, err = s.calendarReminderState(ctx, u)
			if err != nil || !available || !enabled {
				return err
			}
			if sender == nil {
				sender, err = s.calendarReminderBot(ctx, u)
				if err != nil {
					return err
				}
				dm, err = s.chat.Direct(ctx, u, chat.ID(sender.ID))
				if err != nil {
					return err
				}
			}
			identity := event.UID
			if identity == "" {
				identity = candidate.ObjectID
			}
			pending := "calendar-reminder:" + chat.ID(identity+"\x00"+event.Start.UTC().Format(time.RFC3339))
			title := strings.Join(strings.Fields(event.Summary), " ")
			if title == "" {
				title = "Событие"
			}
			title = strings.ReplaceAll(title, "@", "＠")
			message := fmt.Sprintf("Календарь · %s\n%s", event.Start.Local().Format("02.01.2006 15:04 -07:00"), clipAgentText(title, 300))
			if _, err = s.chat.Send(ctx, sender, dm.ID, "", message, pending); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *API) calendarReminderBot(ctx context.Context, recipient *storage.User) (*storage.User, error) {
	suffix := chat.ID(userScope(recipient).Key())
	botID := "tms-calendar:" + suffix
	bot, err := s.store.GetUserByID(ctx, botID)
	if errors.Is(err, storage.ErrNotFound) {
		domain, lookupErr := s.store.GetDomainByID(ctx, recipient.DomainID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		username := "calendar-" + suffix
		bot, err = s.store.CreateUser(ctx, &storage.User{
			ID: botID, TenantID: recipient.TenantID, DomainID: recipient.DomainID,
			LocalPart: username, Email: username + "@" + domain.Name,
			DisplayName: "Календарь", AuthSource: "bot", Roles: "bot", Enabled: true,
		})
		if err != nil {
			bot, err = s.store.GetUserByID(ctx, botID)
		}
	}
	if err != nil {
		return nil, err
	}
	if !bot.Enabled || bot.AuthSource != "bot" || bot.DomainID != recipient.DomainID || bot.TenantID != recipient.TenantID {
		return nil, chat.ErrForbidden
	}
	return bot, nil
}

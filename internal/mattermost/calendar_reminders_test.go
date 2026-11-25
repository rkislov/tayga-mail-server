package mattermost

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

func TestCalendarReminderPrivateSurvivesNextTickAndRespectsOptOut(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	allow := true
	if err := f.api.policy.Save(ctx, userScope(f.users[0]), mmsettings.Overrides{CalendarReminders: &allow}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 26, 9, 50, 0, 0, time.UTC)
	for i, name := range []string{"Командная встреча", "PRIVATE OTHER USER"} {
		user := f.users[i]
		cal, err := f.store.EnsureCalendar(ctx, user.ID, "work", "Календарь")
		if err != nil {
			t.Fatal(err)
		}
		data := fmt.Sprintf("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:meeting\r\nDTSTAMP:20261125T100000Z\r\nDTSTART:20261125T100000Z\r\nDTEND:20261125T110000Z\r\nRRULE:FREQ=DAILY;COUNT=3\r\nSUMMARY:%s\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", name)
		if _, err := f.store.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, HrefName: "meeting.ics", UID: "meeting", Data: data}); err != nil {
			t.Fatal(err)
		}
	}
	route := "/tms/calendar-reminders"
	f.call(t, "PUT", route, f.tokens[3], map[string]bool{"enabled": true}, 403)
	f.call(t, "PUT", route, f.tokens[0], map[string]bool{"enabled": true}, 200)
	f.api.sendCalendarChatReminders(ctx, now)
	f.api.sendCalendarChatReminders(ctx, now.Add(time.Minute))
	bot, err := f.api.calendarReminderBot(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	channels, err := f.api.chat.Channels(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, ch := range channels {
		if ch.Kind == "D" {
			id = ch.ID
		}
	}
	if id == "" {
		t.Fatal("missing private calendar DM")
	}
	posts, err := f.api.chat.Posts(ctx, f.users[0], id, 50, 0, 0)
	if err != nil || len(posts) != 1 || posts[0].UserID != bot.ID ||
		!strings.Contains(posts[0].Message, "Командная встреча") {
		t.Fatalf("unexpected reminder posts: %#v, %v", posts, err)
	}
	if _, err := f.api.chat.Channel(ctx, f.users[1], id); err == nil {
		t.Fatal("another user can see the calendar DM")
	}
	f.call(t, "PUT", route, f.tokens[0], map[string]bool{"enabled": false}, 200)
	f.api.sendCalendarChatReminders(ctx, now.AddDate(0, 0, 1))
	posts, _ = f.api.chat.Posts(chat.WithDomainScope(ctx, f.users[0].DomainID), f.users[0], id, 50, 0, 0)
	if len(posts) != 1 {
		t.Fatal("reminder was delivered after opting out")
	}
}

package httpapi

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCalendarInvitesExposeReplyScopeAndCancellation(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	calendar, err := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/calendar/calendars/"+calendar.ID+"/events", `{"uid":"invitation-view","summary":"Series","start":"2026-11-10T00:00:00Z","end":"2026-11-11T00:00:00Z","all_day":true,"location":"Комната А","recurrence":{"frequency":"DAILY","interval":1,"count":2},"attendees":[{"email":"reader@example.com"}]}`, 200)
	type inviteView struct {
		ID        string `json:"id"`
		ETag      string `json:"etag"`
		CanReply  bool   `json:"can_reply"`
		Cancelled bool   `json:"cancelled"`
		Recurring bool   `json:"recurring"`
		AllDay    bool   `json:"all_day"`
		Location  string `json:"location"`
	}
	read := func() inviteView {
		t.Helper()
		response := f.call(1, "GET", "/api/v1/calendar/invites?all=1", "", 200)
		var data struct {
			Invites []inviteView `json:"invites"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil || len(data.Invites) != 1 {
			t.Fatal(response.Body.String(), err)
		}
		return data.Invites[0]
	}
	view := read()
	if !view.CanReply || view.Cancelled || !view.Recurring || !view.AllDay || view.ETag == "" || view.Location != "Комната А" {
		t.Fatalf("missing invitation details: %+v", view)
	}
	objects, err := f.s.store.ListCalendarObjects(ctx, calendar.ID)
	if err != nil || len(objects) != 1 {
		t.Fatal(err, objects)
	}
	f.call(0, "DELETE", "/api/v1/calendar/events/"+objects[0].ID, "", 200, "If-Match", objects[0].ETag)
	cancelled := read()
	if cancelled.ID != view.ID || cancelled.CanReply || !cancelled.Cancelled {
		t.Fatalf("cancelled invitation is replyable: %+v", cancelled)
	}
}

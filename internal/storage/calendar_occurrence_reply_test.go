package storage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
)

func TestOccurrenceReplyConcurrentDatesAndPersistence(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	participant := meetingUser(t, f, "participant")
	object := f.event(t, "occurrence-response", 19, 3)
	object.Data = strings.Replace(object.Data, "END:VEVENT", "ATTENDEE:mailto:"+participant.Email+"\r\nEND:VEVENT", 1)
	source, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, object)
	if err != nil {
		t.Fatal(err)
	}
	invites, err := f.s.ListCalendarInvitesForUser(ctx, participant.ID, false)
	if err != nil || len(invites) != 1 {
		t.Fatal(invites, err)
	}
	invite := invites[0]
	copy, _ := f.s.GetCalendarObjectByID(ctx, invite.CalendarObjectID)
	other, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	rids := []string{"2026-10-26T10:00:00Z", "2026-11-02T10:00:00Z"}
	start := make(chan struct{})
	results := make(chan struct {
		index int
		err   error
	}, 2)
	for i, driver := range []Driver{f.s, other} {
		go func(i int, driver Driver) {
			<-start
			_, err := driver.ReplyCalendarOccurrence(ctx, participant.ID, invite.ID, rids[i], "DECLINED", copy.ETag, true, "")
			results <- struct {
				index int
				err   error
			}{i, err}
		}(i, driver)
	}
	close(start)
	failed := -1
	success := 0
	for range rids {
		result := <-results
		if result.err == nil {
			success++
		} else if errors.Is(result.err, ErrCalendarConflict) {
			failed = result.index
		} else {
			t.Fatal(result.err)
		}
	}
	if success != 1 || failed < 0 {
		t.Fatalf("concurrent writes: %d, failed %d", success, failed)
	}
	copy, _ = f.s.GetCalendarObjectByID(ctx, copy.ID)
	if _, err = other.ReplyCalendarOccurrence(ctx, participant.ID, invite.ID, rids[failed], "DECLINED", copy.ETag, true, ""); err != nil {
		t.Fatal(err)
	}
	// A fresh connection sees both committed dates and unchanged resource RSVP.
	reopened, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, id := range []string{source.ID, copy.ID} {
		saved, err := reopened.GetCalendarObjectByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		master, _ := calutil.ParseICS(saved.Data)
		for _, attendee := range master.Attendees {
			if attendee.Email == participant.Email && attendee.PartStat == "DECLINED" {
				t.Fatal("changed series RSVP")
			}
		}
		for _, rid := range rids {
			instance, err := calutil.GetOccurrence(saved.Data, rid)
			if err != nil {
				t.Fatal(err)
			}
			for _, attendee := range instance.Attendees {
				if attendee.Email == participant.Email && attendee.PartStat != "DECLINED" {
					t.Fatal("lost concurrent response")
				}
				if attendee.Email == f.room.Email && attendee.PartStat != "ACCEPTED" {
					t.Fatal("changed room RSVP")
				}
			}
		}
	}
}

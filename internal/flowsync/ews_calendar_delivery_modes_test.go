// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
	"net/mail"
	"strings"
	"testing"
)

func calendarModeJobs(t *testing.T, jobs []*storage.CalendarDelivery, recipients map[string]string, copies int) {
	t.Helper()
	sent := 0
	for _, j := range jobs {
		if j.TargetMailbox == "Sent" {
			sent++
			continue
		}
		method, ok := recipients[j.Recipient]
		if !ok || !bytes.Contains(j.Data, []byte("METHOD:"+method)) {
			t.Fatal("unexpected delivery", j.Recipient, string(j.Data))
		}
		delete(recipients, j.Recipient)
	}
	if len(recipients) != 0 || sent != copies {
		t.Fatal("missing recipients/copies", recipients, sent, copies)
	}
}

func TestEWSCalendarDeliveryModesAndSentSnapshots(t *testing.T) {
	ctx, st, owner, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	bob := standardMeetingPeer(t, ctx, st, owner, "mode-bob")
	carol := standardMeetingPeer(t, ctx, st, owner, "mode-carol")
	dan := standardMeetingPeer(t, ctx, st, owner, "mode-dan")
	cal, _ := st.GetCalendarByName(ctx, owner.ID, "default")
	roster := func(emails ...string) string {
		var b strings.Builder
		for _, email := range emails {
			b.WriteString(standardAttendee(email, email))
		}
		return b.String()
	}
	created := calendarSOAP(t, h, ctx, owner, standardMeetingCreate(cal.ID, "SendToAllAndSaveCopy", `<t:RequiredAttendees>`+roster(bob.Email, carol.Email, "outside@remote.example")+`</t:RequiredAttendees>`))
	if created.find("ResponseCode").Text != "NoError" {
		t.Fatal(created.render())
	}
	id := extractAttr(created.render(), "ItemId", "Id")
	source, _ := st.GetCalendarObjectByID(ctx, id)
	jobs := drainCalendarJobs(t, ctx, st)
	calendarModeJobs(t, jobs, map[string]string{bob.Email: "REQUEST", carol.Email: "REQUEST", "outside@remote.example": "REQUEST"}, 1)
	for _, j := range jobs {
		if j.TargetMailbox == "Sent" {
			if j.RecipientUserID != owner.ID || j.Recipient != owner.Email {
				t.Fatal("copy sent to another account", j)
			}
			message, err := mail.ReadMessage(bytes.NewReader(j.Data))
			if err != nil {
				t.Fatal(err)
			}
			to, err := mail.ParseAddressList(message.Header.Get("To"))
			if err != nil || len(to) != 3 {
				t.Fatal("invalid grouped To", to, err)
			}
		}
	}
	invites, _ := st.ListCalendarInvitesForUser(ctx, bob.ID, false)
	received, _ := st.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	if _, err := st.ReplyCalendarInvite(ctx, bob.ID, invites[0].ID, "ACCEPTED", nil, nil, received.ETag); err != nil {
		t.Fatal(err)
	}
	drainCalendarJobs(t, ctx, st)
	updates := standardMeetingField("SetItemField", "RequiredAttendees", roster(bob.Email, dan.Email, "outside@remote.example"))
	source, _ = st.GetCalendarObjectByID(ctx, id)
	updated := calendarSOAP(t, h, ctx, owner, standardMeetingUpdate(id, ewsCalendarChangeKey(source, nil), "SendToChangedAndSaveCopy", updates))
	if updated.find("ResponseCode").Text != "NoError" {
		t.Fatal(updated.render())
	}
	jobs = drainCalendarJobs(t, ctx, st)
	calendarModeJobs(t, jobs, map[string]string{dan.Email: "REQUEST", carol.Email: "CANCEL"}, 2)
	source, _ = st.GetCalendarObjectByID(ctx, id)
	parsed, _ := calutil.ParseICS(source.Data)
	for _, a := range parsed.Attendees {
		if a.Email == bob.Email && a.PartStat != "ACCEPTED" {
			t.Fatal("unchanged attendee RSVP reset")
		}
	}
	invites, _ = st.ListCalendarInvitesForUser(ctx, bob.ID, false)
	if invites[0].PartStat != "ACCEPTED" {
		t.Fatal("unchanged invitation RSVP reset")
	}
	carolInvites, _ := st.ListCalendarInvitesForUser(ctx, carol.ID, false)
	copy, _ := st.GetCalendarObjectByID(ctx, carolInvites[0].CalendarObjectID)
	cancelled, _ := calutil.ParseICS(copy.Data)
	if cancelled.Status != "CANCELLED" {
		t.Fatal("removed participant copy remained active")
	}
	// A fresh-key echo is not another send/copy operation.
	echo := calendarSOAP(t, h, ctx, owner, standardMeetingUpdate(id, ewsCalendarChangeKey(source, nil), "SendToChangedAndSaveCopy", updates))
	if echo.find("ResponseCode").Text != "NoError" {
		t.Fatal(echo.render())
	}
	if jobs = drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("echo queued duplicates", len(jobs))
	}
	// Changed does not mean all attendees affected by a subject/time change.
	source, _ = st.GetCalendarObjectByID(ctx, id)
	subject := standardMeetingField("SetItemField", "Subject", "Changed without roster edits")
	updated = calendarSOAP(t, h, ctx, owner, standardMeetingUpdate(id, ewsCalendarChangeKey(source, nil), "SendOnlyToChanged", subject))
	if updated.find("ResponseCode").Text != "NoError" {
		t.Fatal(updated.render())
	}
	if jobs = drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("unchanged addresses received update")
	}
	source, _ = st.GetCalendarObjectByID(ctx, id)
	oldKey := ewsCalendarChangeKey(source, nil)
	updated = calendarSOAP(t, h, ctx, owner, standardMeetingUpdate(id, oldKey, "SendToAllAndSaveCopy", standardMeetingField("SetItemField", "Subject", "Notify everyone")))
	if updated.find("ResponseCode").Text != "NoError" {
		t.Fatal(updated.render())
	}
	calendarModeJobs(t, drainCalendarJobs(t, ctx, st), map[string]string{bob.Email: "REQUEST", dan.Email: "REQUEST", "outside@remote.example": "REQUEST"}, 1)
	stale := calendarSOAP(t, h, ctx, owner, standardMeetingUpdate(id, oldKey, "SendToAllAndSaveCopy", subject))
	if stale.find("ResponseCode").Text != "ErrorIrresolvableConflict" {
		t.Fatal(stale.render())
	}
	if jobs = drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("stale write queued copy")
	}
	source, _ = st.GetCalendarObjectByID(ctx, id)
	deleted := calendarSOAP(t, h, ctx, owner, `<m:DeleteItem SendMeetingCancellations="SendToAllAndSaveCopy"><m:ItemIds><t:ItemId Id="`+id+`" ChangeKey="`+ewsCalendarChangeKey(source, nil)+`"/></m:ItemIds></m:DeleteItem>`)
	if deleted.find("ResponseCode").Text != "NoError" {
		t.Fatal(deleted.render())
	}
	calendarModeJobs(t, drainCalendarJobs(t, ctx, st), map[string]string{bob.Email: "CANCEL", dan.Email: "CANCEL", "outside@remote.example": "CANCEL"}, 1)
}

func TestEWSCalendarSentQueueFailureRollsBackCreateUpdateAndDelete(t *testing.T) {
	for _, op := range []string{"CreateItem", "UpdateItem", "DeleteItem"} {
		t.Run(op, func(t *testing.T) {
			ctx, st, owner, eas, _, _ := syncFixture(t)
			h := &ewsHandler{store: st, ms: eas.ms}
			peer := standardMeetingPeer(t, ctx, st, owner, "sent-rollback")
			cal, _ := st.GetCalendarByName(ctx, owner.ID, "default")
			fields := `<t:RequiredAttendees>` + standardAttendee(peer.Email, "Peer") + `</t:RequiredAttendees>`
			var source, copy *storage.CalendarObject
			if op != "CreateItem" {
				created := calendarSOAP(t, h, ctx, owner, standardMeetingCreate(cal.ID, "SendOnlyToAll", fields))
				source, _ = st.GetCalendarObjectByID(ctx, extractAttr(created.render(), "ItemId", "Id"))
				invs, _ := st.ListCalendarInvitesForUser(ctx, peer.ID, false)
				copy, _ = st.GetCalendarObjectByID(ctx, invs[0].CalendarObjectID)
				drainCalendarJobs(t, ctx, st)
			}
			db := st.(*storage.Store).DB()
			var before int
			db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&before)
			if _, err := db.Exec(`CREATE TRIGGER fail_ews_sent_copy BEFORE INSERT ON calendar_deliveries WHEN NEW.target_mailbox='Sent' BEGIN SELECT RAISE(ABORT,'injected sent-copy failure'); END`); err != nil {
				t.Fatal(err)
			}
			body := standardMeetingCreate(cal.ID, "SendToAllAndSaveCopy", fields)
			if op == "UpdateItem" {
				body = standardMeetingUpdate(source.ID, ewsCalendarChangeKey(source, nil), "SendToAllAndSaveCopy", standardMeetingField("SetItemField", "Subject", "Must roll back"))
			}
			if op == "DeleteItem" {
				body = `<m:DeleteItem SendMeetingCancellations="SendToAllAndSaveCopy"><m:ItemIds><t:ItemId Id="` + source.ID + `"/></m:ItemIds></m:DeleteItem>`
			}
			response := calendarSOAP(t, h, ctx, owner, body)
			if response.find("ResponseCode").Text != "ErrorInternalServerError" {
				t.Fatal(response.render())
			}
			var after int
			db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&after)
			if after != before {
				t.Fatal("partial outbox committed")
			}
			if source != nil {
				for _, old := range []*storage.CalendarObject{source, copy} {
					saved, err := st.GetCalendarObjectByID(ctx, old.ID)
					if err != nil || saved.ETag != old.ETag || saved.Data != old.Data {
						t.Fatal("partial event/copy committed", err)
					}
				}
			} else {
				objects, _ := st.ListCalendarObjects(ctx, cal.ID)
				invs, _ := st.ListCalendarInvitesForUser(ctx, peer.ID, false)
				if len(objects) != 0 || len(invs) != 0 {
					t.Fatal("create leaked objects/invites")
				}
			}
		})
	}
}

func TestEWSCalendarOccurrenceDeliveryModes(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	data, err := calutil.WithRecurrence(f.original.Data, calutil.RecurrenceRule{Frequency: "DAILY", Interval: 1, Count: 3}, calutil.NextSequence(f.original.Data))
	if err != nil {
		t.Fatal(err)
	}
	edit := *f.original
	edit.Data = data
	edit.ExpectedETag = edit.ETag
	source, err := f.st.SaveCalendarClientObject(f.ctx, f.owner.ID, &edit)
	if err != nil {
		t.Fatal(err)
	}
	drainCalendarJobs(t, f.ctx, f.st)
	occurrence := `<t:OccurrenceItemId RecurringMasterId="` + source.ID + `" InstanceIndex="2"/>`
	update := func(mode, subject string) *protocolNode {
		return calendarSOAP(t, h, f.ctx, f.owner, `<m:UpdateItem SendMeetingInvitationsOrCancellations="`+mode+`"><m:ItemChanges><t:ItemChange>`+occurrence+`<t:Updates>`+standardMeetingField("SetItemField", "Subject", subject)+`</t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`)
	}
	if out := update("SendToAllAndSaveCopy", "Instance with copy"); out.find("ResponseCode").Text != "NoError" {
		t.Fatal(out.render())
	}
	jobs := drainCalendarJobs(t, f.ctx, f.st)
	calendarModeJobs(t, jobs, map[string]string{f.guest.Email: "REQUEST"}, 1)
	for _, j := range jobs {
		if !bytes.Contains(j.Data, []byte("RECURRENCE-ID:20261023T170000Z")) {
			t.Fatal("instance identity lost", string(j.Data))
		}
	}
	if out := update("SendToChangedAndSaveCopy", "Instance without membership change"); out.find("ResponseCode").Text != "NoError" {
		t.Fatal(out.render())
	}
	if jobs = drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("unchanged instance roster received mail")
	}
	deleted := calendarSOAP(t, h, f.ctx, f.owner, `<m:DeleteItem SendMeetingCancellations="SendToAllAndSaveCopy"><m:ItemIds>`+occurrence+`</m:ItemIds></m:DeleteItem>`)
	if deleted.find("ResponseCode").Text != "NoError" {
		t.Fatal(deleted.render())
	}
	calendarModeJobs(t, drainCalendarJobs(t, f.ctx, f.st), map[string]string{f.guest.Email: "CANCEL"}, 1)
}

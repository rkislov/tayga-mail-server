package flowsync

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestEASNativeMeetingInboxAndResponse(t *testing.T) {
	for _, version := range []string{"14.1", "16.1"} {
		t.Run(version, func(t *testing.T) {
			f := newMeetingFixture(t)
			ctx := context.WithValue(f.ctx, easVersionKey{}, version)
			response, _, err := f.h.syncMailCollection(ctx, f.guest, f.mail.MailboxID, "next", false, nil)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := encodeWire(response)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeWire(wire)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := parseProtocolXML(decoded)
			if err != nil {
				t.Fatal(err)
			}
			app := doc.find("ApplicationData")
			meeting := app.child("MeetingRequest")
			if app.value("MessageClass") != "IPM.Schedule.Meeting.Request" || meeting == nil || meeting.value("MeetingMessageType") != "1" || meeting.value("ResponseRequested") != "1" {
				t.Fatal(string(decoded))
			}
			if meeting.value("TimeZone") == "" || meeting.value("DtStamp") == "" || meeting.value("StartTime") == "" {
				t.Fatal("missing required meeting data", meeting.render())
			}
			if version == "16.1" {
				if meeting.value("UID") != f.original.UID || meeting.child("GlobalObjId") != nil {
					t.Fatal(meeting.render())
				}
			} else if meeting.value("GlobalObjId") != easGlobalObjectID(f.original.UID) {
				t.Fatal(meeting.render())
			}
			// The native inbox responds using the message ID, not the calendar copy ID.
			result := f.respond(t, f.guest, f.mail.MailboxID, f.mail.ID, "1", version, "")
			if result.value("Status") != "1" || result.value("CalendarId") != f.received.ID {
				t.Fatal(result.render())
			}
			invite, err := f.st.GetCalendarInvite(f.ctx, f.invite.ID)
			if err != nil || invite.PartStat != "ACCEPTED" {
				t.Fatal(invite, err)
			}
			// Sent invitations are not invitations for their organizer to accept.
			raw, err := f.h.ms.Read(f.mail.FilePath)
			if err != nil {
				t.Fatal(err)
			}
			content, err := parseMessageContent(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := easMeetingMail(ctx, content, f.owner.Email); got != "" {
				t.Fatal(got)
			}
			// Plain mail keeps its normal schema.
			if got := easMeetingMail(ctx, messageContent{Text: "normal"}, f.guest.Email); got != "" {
				t.Fatal(got)
			}
			// Fetch must retain the same native meeting properties as Sync.
			dev, _ := f.st.EnsureFlowSyncDevice(f.ctx, f.guest.ID, "native-inbox-fetch", "iPhone")
			sync := func(key, commands string) *protocolNode {
				t.Helper()
				body := `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + f.mail.MailboxID + `</CollectionId><GetChanges>0</GetChanges>` + commands + `</Collection></Collections></Sync>`
				r, e := f.h.syncCollections(ctx, f.guest, dev, []byte(body))
				if e != nil {
					t.Fatal(e)
				}
				d, e := parseProtocolXML([]byte(r))
				if e != nil {
					t.Fatal(e)
				}
				return d.find("Collection")
			}
			key := sync("0", "").value("SyncKey")
			fetch := sync(key, `<Commands><Fetch><ServerId>`+f.mail.ID+`</ServerId></Fetch></Commands>`)
			if !strings.Contains(fetch.render(), "MeetingRequest") {
				t.Fatal(fetch.render())
			}
		})
	}
}

func TestEASMeetingMailCancellationAndRecurrence(t *testing.T) {
	f := newMeetingFixture(t)
	raw, err := f.h.ms.Read(f.mail.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	c, err := parseMessageContent(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(f.ctx, easVersionKey{}, "16.1")
	for i := range c.Attachments {
		if c.Attachments[i].ContentType != "text/calendar" {
			continue
		}
		original := string(c.Attachments[i].Data)
		cancelled := strings.Replace(original, "METHOD:REQUEST", "METHOD:CANCEL", 1)
		c.Attachments[i].Data = []byte(cancelled)
		out := easMeetingMail(ctx, c, f.guest.Email)
		if !strings.Contains(out, "IPM.Schedule.Meeting.Canceled") || !strings.Contains(out, ">3</Email2:MeetingMessageType>") || !strings.Contains(out, ">0</Email:ResponseRequested>") {
			t.Fatal(out)
		}
		c.Attachments[i].Data = []byte(strings.Replace(original, "END:VEVENT", "RRULE:FREQ=DAILY;COUNT=3\r\nEND:VEVENT", 1))
		out = easMeetingMail(ctx, c, f.guest.Email)
		wire, e := encodeWire(`<ApplicationData xmlns="AirSync:">` + out + `</ApplicationData>`)
		if e != nil {
			t.Fatal(e)
		}
		back, e := decodeWire(wire)
		if e != nil {
			t.Fatal(e)
		}
		d, e := parseProtocolXML(back)
		if e != nil {
			t.Fatal(e)
		}
		if d.find("Recurrence").value("Occurrences") != "3" || d.find("MeetingRequest").value("InstanceType") != "1" {
			t.Fatal(string(back))
		}
	}
}

func TestEASExistingInvitationResentWithNativeSchemaOnly(t *testing.T) {
	f := newMeetingFixture(t)
	if err := f.st.UpdateMessageHeaders(f.ctx, f.mail.ID, "Invitation: Тест", f.owner.Email, f.guest.Email, ""); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(f.ctx, easVersionKey{}, "16.1")
	metaCtx := context.WithValue(ctx, mailSnapshotKey{}, true)
	current, _, err := f.h.syncMailCollection(metaCtx, f.guest, f.mail.MailboxID, "next", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseProtocolXML([]byte(current))
	if err != nil {
		t.Fatal(err)
	}
	commands := doc.find("Commands")
	hashes, _ := snapshotItemHashes(kindMail, commands, syncPreferences{}, nil)
	// Restore the exact old metadata digest to model a previously acknowledged invite.
	body := commands.find("Body").child("Data")
	body.Text = digest(fmt.Sprintf("%s:%d:%s", f.mail.FilePath, f.mail.Size, f.mail.Flags))
	previous, _ := snapshotItemHashes(kindMail, commands, syncPreferences{}, nil)
	changed, _, _ := diffItems(previous, hashes, 100)
	if len(changed) != 1 || changed[0] != f.mail.ID {
		t.Fatal("existing invite did not receive native schema update", changed)
	}
}

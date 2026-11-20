package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

func TestCalendarDownloadExternalInvitationsAndRevocation(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	if err := f.s.SetCalendarAttachmentLinks("https://mail.example.com", bytes.Repeat([]byte{42}, 32)); err != nil {
		t.Fatal(err)
	}
	guest := meetingUser(t, f, "guest")
	obj := meetingWithParticipant(t, f, guest.Email)
	file, err := f.s.CreateCalendarAttachment(ctx, &CalendarAttachment{CalendarObjectID: obj.ID, UserID: f.user.ID, Filename: "Билет.pdf", ContentType: "application/pdf", Size: 12, StoragePath: "ticket.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	// An organizer edit queues a separately personalized payload for each guest.
	updated := changedMeeting(t, obj, "Flight with file")
	obj, err = f.s.SaveCalendarMeeting(ctx, f.user.ID, updated)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := f.s.db.Query(`SELECT recipient,data FROM calendar_deliveries WHERE subject LIKE '%Flight with file%'`)
	if err != nil {
		t.Fatal(err)
	}
	payloads := map[string][]byte{}
	for rows.Next() {
		var email string
		var raw []byte
		if err = rows.Scan(&email, &raw); err != nil {
			t.Fatal(err)
		}
		payloads[email] = raw
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if len(payloads) != 2 {
		t.Fatalf("expected local and external invitations, got %d", len(payloads))
	}
	links := map[string]string{}
	for email, raw := range payloads {
		m, err := mail.ReadMessage(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		_, params, _ := mime.ParseMediaType(m.Header.Get("Content-Type"))
		reader := multipart.NewReader(m.Body, params["boundary"])
		parts := map[string]string{}
		for {
			part, e := reader.NextPart()
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(e)
			}
			body, e := io.ReadAll(part)
			if e != nil {
				t.Fatal(e)
			}
			kind, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
			parts[kind] = string(body)
		}
		event, err := calutil.ParseICS(parts["text/calendar"])
		if err != nil || len(event.Attachments) != 1 {
			t.Fatal(err, "missing signed attachment")
		}
		link := event.Attachments[0].URI
		links[email] = link
		if !strings.Contains(parts["text/plain"], link) || !strings.Contains(parts["text/html"], `href="`+link+`"`) || !strings.Contains(event.Description, link) || !strings.Contains(parts["text/calendar"], "RRULE:") || event.UID != obj.UID {
			t.Fatal("incomplete invitation or recurrence lost")
		}
		u, _ := url.Parse(link)
		got, err := f.s.CalendarDownloadAttachment(ctx, strings.TrimPrefix(u.Path, CalendarDownloadPath))
		if err != nil || got.ID != file.ID {
			t.Fatal("signed download", err)
		}
	}
	external := links["external@elsewhere.example"]
	if external == "" || external == links[guest.Email] {
		t.Fatal("links are not personalized")
	}
	current, _ := f.s.GetCalendarObjectByID(ctx, obj.ID)
	if strings.Contains(current.Data, CalendarDownloadPath) {
		t.Fatal("recipient capability leaked into source")
	}
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, guest.ID, false)
	guestLinks, err := f.s.CalendarAttachmentLinks(ctx, invites[0].CalendarObjectID, guest.ID)
	if err != nil || len(guestLinks) != 1 || guestLinks[0].URL != links[guest.Email] {
		t.Fatal("unstable/native guest link", err)
	}
	if _, err = f.s.CalendarAttachmentLinks(ctx, invites[0].CalendarObjectID, f.user.ID); err == nil {
		t.Fatal("other user's copy allowed")
	}
	u, _ := url.Parse(external)
	token := strings.TrimPrefix(u.Path, CalendarDownloadPath)
	if _, err = f.s.CalendarDownloadAttachment(ctx, token+"x"); err == nil {
		t.Fatal("tampered signature allowed")
	}
	// Even a correctly signed expired capability must fail.
	bits := strings.Split(token, ".")
	raw, _ := base64.RawURLEncoding.DecodeString(bits[0])
	var claim calendarDownloadClaim
	_ = json.Unmarshal(raw, &claim)
	claim.Expires = time.Now().Add(-time.Minute).Unix()
	raw, _ = json.Marshal(claim)
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, f.s.calendarLinkKey)
	mac.Write([]byte(payload))
	if _, err = f.s.CalendarDownloadAttachment(ctx, payload+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil))); err == nil {
		t.Fatal("expired capability allowed")
	}
	// Moving the event keeps its attachment and download identity.
	cal, _ := f.s.EnsureCalendar(ctx, f.user.ID, "work", "Work")
	obj, err = f.s.MoveCalendarObject(ctx, f.user.ID, obj.ID, obj.CalendarID, cal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.CalendarDownloadAttachment(ctx, token); err != nil {
		t.Fatal("move revoked file", err)
	}
	invs, _ := f.s.ListCalendarInvitesByEventUID(ctx, obj.UID)
	var externalInvite *CalendarInvite
	for _, inv := range invs {
		if inv.AttendeeEmail == "external@elsewhere.example" {
			externalInvite = inv
		}
	}
	if externalInvite == nil {
		t.Fatal("missing external invite")
	}
	if err = f.s.UpdateCalendarInvitePartStat(ctx, externalInvite.ID, "DECLINED", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.CalendarDownloadAttachment(ctx, token); err == nil {
		t.Fatal("declined guest still downloads")
	}
	if err = f.s.UpdateCalendarInvitePartStat(ctx, externalInvite.ID, "ACCEPTED", nil, nil); err != nil {
		t.Fatal(err)
	}
	p, _ := calutil.ParseICS(obj.Data)
	p.Status = "CANCELLED"
	obj.Data, _ = calutil.BuildRequestICS(p.Event)
	obj, err = f.s.UpsertCalendarObject(ctx, obj)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.CalendarDownloadAttachment(ctx, token); err == nil {
		t.Fatal("cancelled event still downloads")
	}
	p.Status = "CONFIRMED"
	p.Attendees = nil
	obj.Data, _ = calutil.BuildRequestICS(p.Event)
	if _, err = f.s.UpsertCalendarObject(ctx, obj); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.CalendarDownloadAttachment(ctx, token); err == nil {
		t.Fatal("removed guest still downloads")
	}
}

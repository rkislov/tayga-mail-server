package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func TestDelegatedMailIdentitiesAndRevocation(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	grant := func(as, behalf bool) {
		f.call(0, "POST", "/api/v1/mail/delegates", fmt.Sprintf(`{"email":"reader@example.com","can_read":true,"can_send_as":%t,"can_send_on_behalf":%t}`, as, behalf), 200)
	}
	send := func(mode string, status int) {
		f.call(1, "POST", "/api/v1/mail/send", fmt.Sprintf(`{"from":"owner@example.com","sender_mode":%q,"to":["editor@example.com"],"subject":"delegated","text":"hello"}`, mode), status)
	}
	send("send_as", 403)
	send("on_behalf", 403)
	grant(false, true)
	f.call(2, "POST", "/api/v1/mail/send", `{"to":["owner@example.com"],"subject":"Reply as owner","text":"hello"}`, 200)
	ownerBox, _ := f.s.store.GetMailbox(ctx, f.users[0], "INBOX")
	incoming, _ := f.s.store.ListMessages(ctx, ownerBox.ID)
	viewed := f.call(1, "GET", "/api/v1/mail/messages/"+incoming[0].ID, "", 200)
	var reply struct {
		ReplyFrom string `json:"reply_from"`
	}
	if err := json.Unmarshal(viewed.Body.Bytes(), &reply); err != nil || reply.ReplyFrom != "owner@example.com" {
		t.Fatal(viewed.Body.String(), err)
	}
	var result struct {
		Addresses  []string
		Identities []struct{ Email, Mode string }
	}
	w := f.call(1, "GET", "/api/v1/mail/senders", "", 200)
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Identities) != 2 || result.Identities[1].Email != "owner@example.com" || result.Identities[1].Mode != "on_behalf" {
		t.Fatal(w.Body.String())
	}
	send("send_as", 403)
	send("on_behalf", 200)
	inbox, err := f.s.store.GetMailbox(ctx, f.users[2], "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	messages, err := f.s.store.ListMessages(ctx, inbox.ID)
	if err != nil || len(messages) != 1 {
		t.Fatal(messages, err)
	}
	raw, err := f.s.ms.Read(messages[0].FilePath)
	if err != nil {
		t.Fatal(err)
	}
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	from, _ := message.Header.AddressList("From")
	sender, _ := message.Header.AddressList("Sender")
	if from[0].Address != "owner@example.com" || from[0].Name != "owner" || len(sender) != 1 || sender[0].Address != "reader@example.com" {
		t.Fatal(message.Header)
	}
	grant(true, false)
	send("on_behalf", 403)
	send("send_as", 200)
	send("invented", 403)
	messages, _ = f.s.store.ListMessages(ctx, inbox.ID)
	for _, m := range messages {
		raw, _ := f.s.ms.Read(m.FilePath)
		header, _ := mail.ReadMessage(bytes.NewReader(raw))
		if header.Header.Get("Sender") == "" {
			if !strings.Contains(header.Header.Get("From"), "owner") {
				t.Fatal(header.Header)
			}
		}
	}
	for _, userID := range f.users[:2] {
		sent, err := f.s.store.GetMailbox(ctx, userID, "Sent")
		if err != nil {
			t.Fatal(err)
		}
		copies, err := f.s.store.ListMessages(ctx, sent.ID)
		if err != nil || len(copies) != 2 {
			t.Fatal("missing sent copies", copies, err)
		}
	}
	f.call(0, "GET", "/api/v1/mail/mailboxes", "", 200)
	ownerInbox, _ := f.s.store.GetMailbox(ctx, f.users[0], "INBOX")
	f.call(1, "GET", "/api/v1/mail/mailboxes/"+ownerInbox.ID+"/messages", "", 200)
	f.call(2, "GET", "/api/v1/mail/mailboxes/"+ownerInbox.ID+"/messages", "", 403)
	f.call(0, "DELETE", "/api/v1/mail/delegates/"+f.users[1], "", 200)
	send("send_as", 403)
	send("on_behalf", 403)
	f.call(1, "GET", "/api/v1/mail/mailboxes/"+ownerInbox.ID+"/messages", "", 403)
}

func TestSecretaryCalendarMeetingAndReplies(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	st := f.s.store
	cal, err := st.EnsureCalendar(ctx, f.users[0], "default", "Owner calendar")
	if err != nil {
		t.Fatal(err)
	}
	personal, err := st.EnsureCalendar(ctx, f.users[0], "personal", "Personal")
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/calendar/calendars/" + cal.ID + "/events"
	grant := func(read, manage, respond bool) {
		f.call(0, "POST", "/api/v1/mail/delegates", fmt.Sprintf(`{"email":"reader@example.com","can_read":false,"can_read_calendar":%t,"can_manage_calendar":%t,"can_respond_invites":%t}`, read, manage, respond), 200)
	}
	grant(true, false, false)
	f.call(1, "GET", path, "", 200)
	f.call(1, "POST", path, meetingPayload("", "", "Cannot create", `[]`), 404)
	f.call(1, "GET", "/api/v1/calendar/calendars/"+personal.ID+"/events", "", 404)
	grant(false, true, true)
	w := f.call(1, "POST", path, meetingPayload("", "", "Secretary meeting", `[{"email":"editor@example.com"}]`), 200)
	var event struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	source, err := st.GetCalendarObjectByID(ctx, event.ID)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := calutil.ParseICS(source.Data)
	if err != nil || parsed.Organizer.Email != "owner@example.com" {
		t.Fatal(source.Data, err)
	}
	invites, err := st.ListCalendarInvitesForUser(ctx, f.users[2], false)
	if err != nil || len(invites) != 1 || invites[0].OrganizerUserID != f.users[0] {
		t.Fatal(invites, err)
	}
	var queued []byte
	err = st.(*storage.Store).DB().QueryRow(`SELECT data FROM calendar_deliveries WHERE recipient='editor@example.com'`).Scan(&queued)
	if err != nil || !bytes.Contains(queued, []byte("Sender: \"reader\" <reader@example.com>")) {
		t.Fatalf("mail %s: %v", queued, err)
	}
	f.call(2, "POST", "/api/v1/calendar/invites/"+invites[0].ID+"/reply", `{"action":"accept"}`, 200)
	source, _ = st.GetCalendarObjectByID(ctx, event.ID)
	f.call(1, "POST", path, meetingPayload(event.ID, source.ETag, "Secretary moved", `[{"email":"editor@example.com"}]`), 200)
	// The secretary can process a counterproposal addressed to the organizer.
	f.call(2, "POST", "/api/v1/calendar/invites/"+invites[0].ID+"/reply", `{"action":"counter","start":"2026-10-19T14:00:00Z","end":"2026-10-19T15:00:00Z"}`, 200)
	proposals, err := st.ListCalendarProposals(ctx, f.users[0])
	if err != nil || len(proposals) != 1 {
		t.Fatal(proposals, err)
	}
	w = f.call(1, "GET", "/api/v1/calendar/invites?all=1", "", 200)
	if !strings.Contains(w.Body.String(), proposals[0].Object.ID) {
		t.Fatal(w.Body.String())
	}
	proposal := proposals[0]
	f.call(1, "POST", "/api/v1/calendar/invites/"+proposal.Invite.ID+"/decision", fmt.Sprintf(`{"action":"accept","event_id":%q,"revision":%q,"etag":%q}`, proposal.Object.ID, proposal.Invite.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), proposal.Object.ETag), 200)
	source, _ = st.GetCalendarObjectByID(ctx, event.ID)
	f.call(1, "PUT", "/api/v1/calendar/events/"+event.ID+"/recurrence", fmt.Sprintf(`{"etag":%q,"rule":{"frequency":"DAILY","interval":1,"count":3}}`, source.ETag), 200)
	source, _ = st.GetCalendarObjectByID(ctx, event.ID)
	f.call(1, "POST", "/api/v1/calendar/events/"+event.ID+"/occurrences", fmt.Sprintf(`{"etag":%q,"recurrence_id":"2026-10-20T14:00:00Z","action":"update","start":"2026-10-20T16:00:00Z","end":"2026-10-20T17:00:00Z"}`, source.ETag), 200)
	source, _ = st.GetCalendarObjectByID(ctx, event.ID)
	grant(true, false, false)
	f.call(1, "DELETE", "/api/v1/calendar/events/"+event.ID, "", 403)
	// An invitation addressed to the owner appears to the secretary only with RSVP permission.
	other, _ := st.EnsureCalendar(ctx, f.users[2], "default", "Other calendar")
	f.call(2, "POST", "/api/v1/calendar/calendars/"+other.ID+"/events", strings.ReplaceAll(meetingPayload("", "", "Invitation to owner", `[{"email":"owner@example.com"}]`), `"atomic"`, `"to-owner"`), 200)
	ownerInvites, _ := st.ListCalendarInvitesForUser(ctx, f.users[0], false)
	if len(ownerInvites) != 1 {
		t.Fatal(ownerInvites)
	}
	inv := ownerInvites[0]
	f.call(1, "POST", "/api/v1/calendar/invites/"+inv.ID+"/reply", `{"action":"accept"}`, 403)
	grant(false, false, true)
	w = f.call(1, "GET", "/api/v1/calendar/invites?all=1", "", 200)
	if !strings.Contains(w.Body.String(), inv.ID) || !strings.Contains(w.Body.String(), `"delegated":true`) {
		t.Fatal(w.Body.String())
	}
	f.call(1, "POST", "/api/v1/calendar/invites/"+inv.ID+"/reply", `{"action":"accept"}`, 200)
	updated, _ := st.GetCalendarInvite(ctx, inv.ID)
	if updated.PartStat != "ACCEPTED" {
		t.Fatal(updated)
	}
	// Direct storage writes must also enforce revocation.
	f.call(0, "DELETE", "/api/v1/mail/delegates/"+f.users[1], "", 200)
	if _, err = st.ReplyCalendarInvite(ctx, f.users[1], inv.ID, "DECLINED", nil, nil, ""); err != storage.ErrUnauthorized {
		t.Fatal("revoked reply", err)
	}
	source.CreateOnly = false
	source.ExpectedETag = source.ETag
	if _, err = st.SaveCalendarMeeting(ctx, f.users[1], source); err != storage.ErrUnauthorized {
		t.Fatal("revoked write", err)
	}
	grant(false, true, false)
	f.call(1, "DELETE", "/api/v1/calendar/events/"+event.ID, "", 200)
}

func TestDelegationTenantBoundary(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	st := f.s.store
	tenant, _ := st.CreateTenant(ctx, "other")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "other.example")
	u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "foreign@other.example", LocalPart: "foreign", Enabled: true, AuthSource: "local"})
	if err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/mail/delegates", `{"email":"foreign@other.example","can_send_as":true,"can_manage_calendar":true}`, 403)
	if err = st.UpsertMailboxDelegate(ctx, &storage.MailboxDelegate{OwnerID: f.users[0], DelegateID: u.ID, CanRead: true}); err != storage.ErrUnauthorized {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/mail/delegates", `{"email":"owner@example.com"}`, 400)
	f.call(1, "POST", "/api/v1/mail/delegates?owner_id="+f.users[0], `{"email":"editor@example.com"}`, 403)
}

func TestMailboxDelegateReadAndManageAreSeparate(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	f.call(2, "POST", "/api/v1/mail/send", `{"to":["owner@example.com"],"subject":"private","text":"body"}`, 200)
	box, _ := f.s.store.GetMailbox(ctx, f.users[0], "INBOX")
	messages, _ := f.s.store.ListMessages(ctx, box.ID)
	if len(messages) != 1 {
		t.Fatal(messages)
	}
	id := messages[0].ID
	f.call(0, "POST", "/api/v1/mail/delegates", `{"email":"reader@example.com","can_read":true}`, 200)
	f.call(1, "PATCH", "/api/v1/mail/messages/"+id, `{"seen":true}`, 200)
	f.call(1, "PATCH", "/api/v1/mail/messages/"+id, `{"flagged":true}`, 403)
	f.call(1, "DELETE", "/api/v1/mail/messages/"+id, "", 403)
	f.call(1, "POST", "/api/v1/mail/messages/"+id+"/archive", "", 403)
	f.call(0, "POST", "/api/v1/mail/delegates", `{"email":"reader@example.com","can_read":false,"can_manage_mail":true}`, 200)
	f.call(1, "PATCH", "/api/v1/mail/messages/"+id, `{"flagged":true}`, 200)
	f.call(1, "DELETE", "/api/v1/mail/messages/"+id, "", 200)
	moved, _ := f.s.store.GetMessageByID(ctx, id)
	trash, _ := f.s.store.GetMailbox(ctx, f.users[0], "Trash")
	if moved.MailboxID != trash.ID {
		t.Fatal("deleted into secretary's Trash", moved.MailboxID, trash.ID)
	}
	f.call(1, "DELETE", "/api/v1/mail/messages/"+id, "", 200)
}

func TestCalendarDelegationBeforeOwnerOpensCalendar(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "POST", "/api/v1/mail/delegates", `{"email":"reader@example.com","can_read":false,"can_manage_calendar":true}`, 200)
	ownerCalendar, err := f.s.store.GetCalendarByName(context.Background(), f.users[0], "default")
	if err != nil {
		t.Fatal(err)
	}
	response := f.call(1, "GET", "/api/v1/calendar/calendars", "", 200)
	if !strings.Contains(response.Body.String(), ownerCalendar.ID) {
		t.Fatal(response.Body.String())
	}
}

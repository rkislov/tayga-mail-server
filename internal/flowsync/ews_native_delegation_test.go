package flowsync

import (
	"bytes"
	"context"
	"net/mail"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func ewsOwnerFolder(owner *storage.User, name string) string {
	return `<t:DistinguishedFolderId Id="` + name + `"><t:Mailbox><t:EmailAddress>` + owner.Email + `</t:EmailAddress></t:Mailbox></t:DistinguishedFolderId>`
}
func delegateRequest(op string, owner, delegate *storage.User, permissions string) string {
	if op == "GetDelegate" {
		return `<m:GetDelegate IncludePermissions="true"><m:Mailbox><t:EmailAddress>` + owner.Email + `</t:EmailAddress></m:Mailbox></m:GetDelegate>`
	}
	ids := `<m:DelegateUsers><t:DelegateUser><t:UserId><t:PrimarySmtpAddress>` + delegate.Email + `</t:PrimarySmtpAddress></t:UserId>` + permissions + `</t:DelegateUser></m:DelegateUsers>`
	if op == "RemoveDelegate" {
		ids = `<m:UserIds><t:UserId><t:PrimarySmtpAddress>` + delegate.Email + `</t:PrimarySmtpAddress></t:UserId></m:UserIds>`
	}
	return `<m:` + op + `><m:Mailbox><t:EmailAddress>` + owner.Email + `</t:EmailAddress></m:Mailbox>` + ids + `</m:` + op + `>`
}
func assertEWSCode(t *testing.T, result *protocolNode, code string) {
	t.Helper()
	if result.find("ResponseCode") == nil || result.find("ResponseCode").Text != code {
		t.Fatalf("want %s: %s", code, result.render())
	}
}
func assertDelegateCode(t *testing.T, result *protocolNode, code string) {
	t.Helper()
	node := result.find("DelegateUserResponseMessageType")
	if node == nil || node.value("ResponseCode") != code {
		t.Fatalf("want delegate %s: %s", code, result.render())
	}
}

func TestEWSNativeDelegatesScopedFoldersAndRevocation(t *testing.T) {
	ctx, st, owner, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	assistant, err := st.CreateUser(ctx, &storage.User{TenantID: owner.TenantID, DomainID: owner.DomainID, LocalPart: "secretary", Email: "secretary@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	call := func(u *storage.User, body string) *protocolNode { return calendarSOAP(t, h, ctx, u, body) }
	permissions := `<t:DelegatePermissions><t:InboxFolderPermissionLevel>Reviewer</t:InboxFolderPermissionLevel><t:CalendarFolderPermissionLevel>Editor</t:CalendarFolderPermissionLevel></t:DelegatePermissions>`
	assertEWSCode(t, call(assistant, delegateRequest("AddDelegate", owner, assistant, permissions)), "ErrorAccessDenied")
	assertDelegateCode(t, call(owner, delegateRequest("AddDelegate", owner, assistant, permissions)), "NoError")
	assertDelegateCode(t, call(owner, delegateRequest("AddDelegate", owner, assistant, permissions)), "ErrorDelegateAlreadyExists")
	result := call(owner, delegateRequest("GetDelegate", owner, assistant, ""))
	assertDelegateCode(t, result, "NoError")
	if result.find("InboxFolderPermissionLevel").Text != "Reviewer" || result.find("CalendarFolderPermissionLevel").Text != "Editor" {
		t.Fatal(result.render())
	}
	// Omitted permissions survive UpdateDelegate; unsupported profiles are rejected.
	assertDelegateCode(t, call(owner, delegateRequest("UpdateDelegate", owner, assistant, `<t:DelegatePermissions><t:CalendarFolderPermissionLevel>Author</t:CalendarFolderPermissionLevel></t:DelegatePermissions>`)), "ErrorInvalidDelegatePermission")
	assertDelegateCode(t, call(owner, delegateRequest("UpdateDelegate", owner, assistant, `<t:ViewPrivateItems>true</t:ViewPrivateItems>`)), "ErrorInvalidDelegatePermission")
	inbox, _ := st.GetMailbox(ctx, owner.ID, "INBOX")
	sent, _ := st.GetMailbox(ctx, owner.ID, "Sent")
	msg := composeSource(t, ctx, eas, owner, []byte("From: remote@outside.example\r\nTo: "+owner.Email+"\r\nSubject: Owner inbox\r\n\r\nDelegated body"))
	assertEWSCode(t, call(assistant, `<m:FindItem><m:ParentFolderIds>`+ewsOwnerFolder(owner, "inbox")+`</m:ParentFolderIds></m:FindItem>`), "NoError")
	assertEWSCode(t, call(assistant, `<m:GetItem><m:ItemIds><t:ItemId Id="`+msg.ID+`"/></m:ItemIds></m:GetItem>`), "NoError")
	assertEWSCode(t, call(assistant, `<m:FindItem><m:ParentFolderIds>`+ewsOwnerFolder(owner, "sentitems")+`</m:ParentFolderIds></m:FindItem>`), "ErrorAccessDenied")
	assertEWSCode(t, call(assistant, `<m:GetFolder><m:FolderIds>`+ewsOwnerFolder(owner, "inbox")+ewsOwnerFolder(owner, "calendar")+`</m:FolderIds></m:GetFolder>`), "NoError")
	raw, _ := h.findFolder(ctx, assistant)
	if strings.Contains(raw, sent.ID) {
		t.Fatal("Inbox permission opened Sent", raw)
	}
	update := `<m:UpdateItem><m:ItemChanges><t:ItemChange><t:ItemId Id="` + msg.ID + `"/><t:Updates><t:SetItemField><t:FieldURI FieldURI="message:IsRead"/><t:Message><t:IsRead>true</t:IsRead></t:Message></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`
	assertEWSCode(t, call(assistant, update), "NoError")
	assertEWSCode(t, call(assistant, `<m:DeleteItem DeleteType="HardDelete"><m:ItemIds><t:ItemId Id="`+msg.ID+`"/></m:ItemIds></m:DeleteItem>`), "ErrorAccessDenied")
	assertDelegateCode(t, call(owner, delegateRequest("UpdateDelegate", owner, assistant, `<t:DelegatePermissions><t:InboxFolderPermissionLevel>Editor</t:InboxFolderPermissionLevel></t:DelegatePermissions>`)), "NoError")
	assertEWSCode(t, call(assistant, `<m:DeleteItem DeleteType="MoveToDeletedItems"><m:ItemIds><t:ItemId Id="`+msg.ID+`"/></m:ItemIds></m:DeleteItem>`), "NoError")
	moved, _ := st.GetMessageByID(ctx, msg.ID)
	trash, _ := st.GetMailbox(ctx, owner.ID, "Trash")
	if moved.MailboxID != trash.ID {
		t.Fatal("wrong owner's Trash")
	}
	// Capture both kinds of replay tokens before revocation.
	syncBody := `<m:SyncFolderItems><m:SyncFolderId><t:FolderId Id="` + inbox.ID + `"/></m:SyncFolderId></m:SyncFolderItems>`
	sync := call(assistant, syncBody)
	assertEWSCode(t, sync, "NoError")
	hierarchy := call(assistant, `<m:SyncFolderHierarchy/>`)
	assertEWSCode(t, hierarchy, "NoError")
	assertDelegateCode(t, call(owner, delegateRequest("RemoveDelegate", owner, assistant, "")), "NoError")
	assertEWSCode(t, call(assistant, syncBody), "ErrorAccessDenied")
	assertEWSCode(t, call(assistant, `<m:GetItem><m:ItemIds><t:ItemId Id="`+msg.ID+`"/></m:ItemIds></m:GetItem>`), "ErrorAccessDenied")
	result = call(assistant, `<m:SyncFolderHierarchy><m:SyncState>`+hierarchy.find("SyncState").Text+`</m:SyncState></m:SyncFolderHierarchy>`)
	if strings.Contains(result.render(), "Owner inbox") {
		t.Fatal("revoked cached data")
	}
	assertDelegateCode(t, call(owner, delegateRequest("RemoveDelegate", owner, assistant, "")), "ErrorNotDelegate")
}

func TestEWSDelegateCalendarCreateUpdatePrivacyAndRSVP(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	ctx := f.ctx
	assistant, err := f.st.CreateUser(ctx, &storage.User{TenantID: f.owner.TenantID, DomainID: f.owner.DomainID, LocalPart: "secretary", Email: "secretary@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	call := func(u *storage.User, body string) *protocolNode { return calendarSOAP(t, h, ctx, u, body) }
	permissions := `<t:DelegatePermissions><t:CalendarFolderPermissionLevel>Editor</t:CalendarFolderPermissionLevel></t:DelegatePermissions>`
	assertDelegateCode(t, call(f.owner, delegateRequest("AddDelegate", f.owner, assistant, permissions)), "NoError")
	created := call(assistant, `<m:CreateItem SendMeetingInvitations="SendOnlyToAll"><m:SavedItemFolderId>`+ewsOwnerFolder(f.owner, "calendar")+`</m:SavedItemFolderId><m:Items><t:CalendarItem><t:Subject>Secretary event</t:Subject><t:Start>2026-11-20T10:00:00Z</t:Start><t:End>2026-11-20T11:00:00Z</t:End></t:CalendarItem></m:Items></m:CreateItem>`)
	assertEWSCode(t, created, "NoError")
	id := nodeAttr(created.find("ItemId"), "Id")
	event, err := f.st.GetCalendarObjectByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := calutil.ParseICS(event.Data)
	if parsed.Organizer.Email != f.owner.Email {
		t.Fatal(event.Data)
	}
	updated := call(assistant, meetingEWSUpdate(id, ewsCalendarChangeKey(event, nil), "SendOnlyToAll", `<t:Subject>Secretary changed</t:Subject>`))
	assertEWSCode(t, updated, "NoError")
	work, _ := f.st.EnsureCalendar(ctx, f.owner.ID, "work", "Work")
	assertEWSCode(t, call(assistant, `<m:FindItem><m:ParentFolderIds><t:FolderId Id="`+work.ID+`"/></m:ParentFolderIds></m:FindItem>`), "ErrorAccessDenied")
	event, _ = f.st.GetCalendarObjectByID(ctx, id)
	event.Data = strings.Replace(event.Data, "END:VEVENT", "CLASS:PRIVATE\r\nEND:VEVENT", 1)
	event, err = f.st.UpsertCalendarObject(ctx, event)
	if err != nil {
		t.Fatal(err)
	}
	assertEWSCode(t, call(assistant, `<m:GetItem><m:ItemIds><t:ItemId Id="`+id+`"/></m:ItemIds></m:GetItem>`), "ErrorAccessDenied")
	list := call(assistant, `<m:FindItem><m:ParentFolderIds>`+ewsOwnerFolder(f.owner, "calendar")+`</m:ParentFolderIds></m:FindItem>`)
	if strings.Contains(list.render(), "Secretary changed") {
		t.Fatal("private event exposed")
	}
	assertEWSCode(t, call(assistant, meetingEWSDelete(id, ewsCalendarChangeKey(event, nil), "SendOnlyToAll")), "ErrorAccessDenied")
	// A delegate with Calendar Editor can answer for the attendee principal.
	assertDelegateCode(t, call(f.guest, delegateRequest("AddDelegate", f.guest, assistant, permissions)), "NoError")
	response := `<m:CreateItem MessageDisposition="SendOnly"><m:Items><t:AcceptItem><t:ReferenceItemId Id="` + f.received.ID + `" ChangeKey="` + ewsCalendarChangeKey(f.received, nil) + `"/></t:AcceptItem></m:Items></m:CreateItem>`
	assertEWSCode(t, call(assistant, response), "NoError")
	inv, _ := f.st.GetCalendarInvite(ctx, f.invite.ID)
	if inv.PartStat != "ACCEPTED" {
		t.Fatal(inv)
	}
	jobs := drainCalendarJobs(t, ctx, f.st)
	found := false
	for _, job := range jobs {
		if job.Recipient == f.owner.Email && bytes.Contains(job.Data, []byte("Sender:")) {
			found = true
		}
	}
	if !found {
		t.Fatal("missing delegated response mail")
	}
	assertDelegateCode(t, call(f.guest, delegateRequest("RemoveDelegate", f.guest, assistant, "")), "NoError")
	assertEWSCode(t, call(assistant, response), "ErrorAccessDenied")
}

func TestEWSDelegateSendAndSaveOwnersSent(t *testing.T) {
	ctx, st, owner, eas, _, _ := syncFixture(t)
	assistant, err := st.CreateUser(ctx, &storage.User{TenantID: owner.TenantID, DomainID: owner.DomainID, LocalPart: "secretary", Email: "secretary@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	h := &ewsHandler{store: st, ms: eas.ms}
	calls := 0
	h.sender = &mailSubmission{store: st, ms: eas.ms, submit: func(_ context.Context, actor *storage.User, recipients []string, raw []byte) error {
		calls++
		if actor.ID != assistant.ID {
			t.Fatal("impersonated actor")
		}
		message, err := mail.ReadMessage(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		from, _ := message.Header.AddressList("From")
		sender, _ := message.Header.AddressList("Sender")
		if from[0].Address != owner.Email || len(sender) != 1 || sender[0].Address != assistant.Email {
			t.Fatal(message.Header)
		}
		return nil
	}}
	assertDelegateCode(t, calendarSOAP(t, h, ctx, owner, delegateRequest("AddDelegate", owner, assistant, `<t:DelegatePermissions><t:CalendarFolderPermissionLevel>Reviewer</t:CalendarFolderPermissionLevel></t:DelegatePermissions>`)), "NoError")
	body := `<m:CreateItem MessageDisposition="SendAndSaveCopy"><m:SavedItemFolderId>` + ewsOwnerFolder(owner, "sentitems") + `</m:SavedItemFolderId><m:Items><t:Message><t:ToRecipients><t:Mailbox><t:EmailAddress>remote@outside.example</t:EmailAddress></t:Mailbox></t:ToRecipients><t:Subject>Secretary sent</t:Subject><t:Body BodyType="Text">hello</t:Body></t:Message></m:Items></m:CreateItem>`
	assertEWSCode(t, calendarSOAP(t, h, ctx, assistant, body), "NoError")
	for _, u := range []*storage.User{owner, assistant} {
		sent, err := st.GetMailbox(ctx, u.ID, "Sent")
		if err != nil {
			t.Fatal(err)
		}
		messages, err := st.ListMessages(ctx, sent.ID)
		if err != nil || len(messages) != 1 {
			t.Fatal(messages, err)
		}
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}

func TestEWSDelegateReceivesAndAnswersMeetingCopy(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	ctx := f.ctx
	assistant, err := f.st.CreateUser(ctx, &storage.User{TenantID: f.owner.TenantID, DomainID: f.owner.DomainID, LocalPart: "secretary", Email: "secretary@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	permissions := `<t:DelegatePermissions><t:CalendarFolderPermissionLevel>Editor</t:CalendarFolderPermissionLevel></t:DelegatePermissions><t:ReceiveCopiesOfMeetingMessages>true</t:ReceiveCopiesOfMeetingMessages>`
	assertDelegateCode(t, calendarSOAP(t, h, ctx, f.guest, delegateRequest("AddDelegate", f.guest, assistant, permissions)), "NoError")
	updated := calendarSOAP(t, h, ctx, f.owner, meetingEWSUpdate(f.original.ID, ewsCalendarChangeKey(f.original, nil), "SendOnlyToAll", `<t:Subject>Meeting copied to secretary</t:Subject>`))
	assertEWSCode(t, updated, "NoError")
	jobs := drainCalendarJobs(t, ctx, f.st)
	var copied *storage.Message
	for _, job := range jobs {
		if job.RecipientUserID == assistant.ID {
			if !bytes.Contains(job.Data, []byte("X-Tayga-Delegate-For: "+f.guest.Email)) {
				t.Fatal(string(job.Data))
			}
			copied = composeSource(t, ctx, f.h, assistant, job.Data)
		}
	}
	if copied == nil {
		t.Fatal("missing secretary copy", len(jobs))
	}
	response := `<m:CreateItem MessageDisposition="SendOnly"><m:Items><t:AcceptItem><t:ReferenceItemId Id="` + copied.ID + `"/></t:AcceptItem></m:Items></m:CreateItem>`
	assertEWSCode(t, calendarSOAP(t, h, ctx, assistant, response), "NoError")
	inv, _ := f.st.GetCalendarInvite(ctx, f.invite.ID)
	if inv.PartStat != "ACCEPTED" {
		t.Fatal(inv)
	}
	// A private source update must not be copied to a delegate who cannot view it.
	drainCalendarJobs(t, ctx, f.st)
	source, _ := f.st.GetCalendarObjectByID(ctx, f.original.ID)
	source.Data = strings.Replace(source.Data, "END:VEVENT", "CLASS:PRIVATE\r\nEND:VEVENT", 1)
	source, err = f.st.UpsertCalendarObject(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	assertEWSCode(t, calendarSOAP(t, h, ctx, f.owner, meetingEWSUpdate(source.ID, ewsCalendarChangeKey(source, nil), "SendOnlyToAll", `<t:Subject>Private change</t:Subject>`)), "NoError")
	for _, job := range drainCalendarJobs(t, ctx, f.st) {
		if job.RecipientUserID == assistant.ID {
			t.Fatal("private meeting copied")
		}
	}
	assertDelegateCode(t, calendarSOAP(t, h, ctx, f.guest, delegateRequest("RemoveDelegate", f.guest, assistant, "")), "NoError")
	denied := calendarSOAP(t, h, ctx, assistant, response)
	if denied.find("ResponseCode").Text == "NoError" {
		t.Fatal("revoked copy answered")
	}
}

func TestEWSDelegateAttachmentsAndReplySentCopy(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms, publicURL: "https://mail.example.com"}
	ctx := f.ctx
	assistant, err := f.st.CreateUser(ctx, &storage.User{TenantID: f.owner.TenantID, DomainID: f.owner.DomainID, LocalPart: "secretary", Email: "secretary@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	permissions := `<t:DelegatePermissions><t:CalendarFolderPermissionLevel>Editor</t:CalendarFolderPermissionLevel></t:DelegatePermissions>`
	assertDelegateCode(t, calendarSOAP(t, h, ctx, f.owner, delegateRequest("AddDelegate", f.owner, assistant, permissions)), "NoError")
	response := calendarSOAP(t, h, ctx, assistant, ewsAttachmentCreate(f.original.ID, ewsCalendarChangeKey(f.original, nil), ewsFileUpload("delegated.txt", "text/plain", []byte("owner attachment"))))
	assertEWSCode(t, response, "NoError")
	reference := nodeAttr(response.find("AttachmentId"), "Id")
	files, err := f.st.ListCalendarAttachments(ctx, f.original.ID)
	if err != nil || len(files) != 1 || files[0].UserID != f.owner.ID {
		t.Fatal(files, err)
	}
	assertEWSCode(t, calendarSOAP(t, h, ctx, assistant, `<m:GetAttachment><m:AttachmentIds><t:AttachmentId Id="`+reference+`"/></m:AttachmentIds></m:GetAttachment>`), "NoError")
	assertEWSCode(t, calendarSOAP(t, h, ctx, assistant, `<m:DeleteAttachment><m:AttachmentIds><t:AttachmentId Id="`+reference+`"/></m:AttachmentIds></m:DeleteAttachment>`), "NoError")
	assertDelegateCode(t, calendarSOAP(t, h, ctx, f.guest, delegateRequest("AddDelegate", f.guest, assistant, permissions)), "NoError")
	copy, _ := f.st.GetCalendarObjectByID(ctx, f.received.ID)
	response = calendarSOAP(t, h, ctx, assistant, `<m:CreateItem MessageDisposition="SendAndSaveCopy"><m:Items><t:AcceptItem><t:ReferenceItemId Id="`+copy.ID+`"/></t:AcceptItem></m:Items></m:CreateItem>`)
	assertEWSCode(t, response, "NoError")
	jobs := drainCalendarJobs(t, ctx, f.st)
	sent := map[string]bool{}
	for _, job := range jobs {
		if job.TargetMailbox == "Sent" {
			sent[job.RecipientUserID] = true
		}
	}
	if !sent[f.guest.ID] || !sent[assistant.ID] {
		t.Fatal("missing principal/actor reply Sent copies", sent)
	}
}

func TestEWSDelegatePrivacyReplayAndTenantBoundary(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	ctx := f.ctx
	assistant, err := f.st.CreateUser(ctx, &storage.User{TenantID: f.owner.TenantID, DomainID: f.owner.DomainID, LocalPart: "secretary", Email: "secretary@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	permissions := `<t:DelegatePermissions><t:CalendarFolderPermissionLevel>Editor</t:CalendarFolderPermissionLevel></t:DelegatePermissions>`
	assertDelegateCode(t, calendarSOAP(t, h, ctx, f.owner, delegateRequest("AddDelegate", f.owner, assistant, permissions)), "NoError")
	syncBody := func(key string) string {
		return `<m:SyncFolderItems><m:SyncFolderId><t:FolderId Id="` + f.original.CalendarID + `"/></m:SyncFolderId><m:SyncState>` + key + `</m:SyncState></m:SyncFolderItems>`
	}
	first := calendarSOAP(t, h, ctx, assistant, syncBody(""))
	assertEWSCode(t, first, "NoError")
	request := syncBody(first.find("SyncState").Text)
	assertEWSCode(t, calendarSOAP(t, h, ctx, f.owner, meetingEWSUpdate(f.original.ID, ewsCalendarChangeKey(f.original, nil), "SendToNone", `<t:Subject>Public before privacy change</t:Subject>`)), "NoError")
	second := calendarSOAP(t, h, ctx, assistant, request)
	assertEWSCode(t, second, "NoError")
	if !strings.Contains(second.render(), "Public before privacy change") {
		t.Fatal(second.render())
	}
	source, _ := f.st.GetCalendarObjectByID(ctx, f.original.ID)
	source.Data = strings.Replace(source.Data, "END:VEVENT", "CLASS:PRIVATE\r\nEND:VEVENT", 1)
	if _, err = f.st.UpsertCalendarObject(ctx, source); err != nil {
		t.Fatal(err)
	}
	assertEWSCode(t, calendarSOAP(t, h, ctx, assistant, request), "ErrorInvalidSyncStateData")
	// Updating a removed grant must not recreate it, including at the SQL boundary.
	grant, _ := f.st.GetMailboxDelegate(ctx, f.owner.ID, assistant.ID)
	assertDelegateCode(t, calendarSOAP(t, h, ctx, f.owner, delegateRequest("RemoveDelegate", f.owner, assistant, "")), "NoError")
	if err = f.st.UpsertMailboxDelegate(ctx, grant, "update"); err != storage.ErrNotFound {
		t.Fatal("stale update recreated grant", err)
	}
	tenant, _ := f.st.CreateTenant(ctx, "Foreign")
	domain, _ := f.st.CreateDomain(ctx, tenant.ID, "foreign.example")
	foreign, err := f.st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "foreign", Email: "foreign@foreign.example", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	assertDelegateCode(t, calendarSOAP(t, h, ctx, f.owner, delegateRequest("AddDelegate", f.owner, foreign, permissions)), "ErrorInvalidUserInfo")
}

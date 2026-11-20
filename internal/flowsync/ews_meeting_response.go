package flowsync

import (
	"bytes"
	"context"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
	"net/mail"
	"strings"
)

func (h *ewsHandler) respondEWSMeeting(ctx context.Context, actor *storage.User, operation *protocolNode, node *protocolNode) (string, error) {
	fail := func(code, message string) (string, error) { return ewsOperationError("CreateItem", code, message), nil }
	status := map[string]string{"AcceptItem": "ACCEPTED", "TentativelyAcceptItem": "TENTATIVE", "DeclineItem": "DECLINED"}[node.Name.Local]
	reference := node.child("ReferenceItemId")
	if reference == nil || nodeAttr(reference, "Id") == "" {
		return fail("ErrorInvalidId", "ReferenceItemId required")
	}
	for _, child := range node.Children {
		switch child.Name.Local {
		case "ReferenceItemId", "Body":
		default:
			return fail("ErrorInvalidRequest", "Unsupported meeting response field")
		}
	}
	id := nodeAttr(reference, "Id")
	rid := ""
	var err error
	if strings.HasPrefix(id, "ews-occ:") {
		id, rid, err = parseOccurrenceID(id)
		if err != nil {
			return fail("ErrorInvalidId", "Invalid occurrence")
		}
	}
	var principal *storage.User
	var collection string
	if obj, e := h.store.GetCalendarObjectByID(ctx, id); e == nil {
		cal, e := h.ewsCalendarObject(ctx, actor.ID, obj, false)
		if e != nil {
			return fail("ErrorAccessDenied", "Calendar unavailable")
		}
		principal, err = h.store.GetUserByID(ctx, cal.UserID)
		collection = cal.ID
	} else if msg, e := h.store.GetMessageByID(ctx, id); e == nil {
		if e = h.mailboxRight(ctx, actor.ID, msg.MailboxID, "r"); e != nil {
			return fail("ErrorAccessDenied", "Mailbox unavailable")
		}
		mb, e := h.store.GetMailboxByID(ctx, msg.MailboxID)
		if e != nil {
			return "", e
		}
		principal, err = h.store.GetUserByID(ctx, mb.UserID)
		collection = mb.ID
	} else {
		return fail("ErrorItemNotFound", "Invitation unavailable")
	}
	if err != nil {
		return "", err
	}
	inv, obj, err := (&easHandler{store: h.store, ms: h.ms}).meetingResponseTarget(ctx, principal, collection, id, "16.1")
	if err != nil && principal.ID == actor.ID {
		if msg, e := h.store.GetMessageByID(ctx, id); e == nil {
			inv, obj, err = h.copiedMeetingTarget(ctx, actor, msg)
		}
	}
	if err != nil {
		return fail("ErrorItemNotFound", "Invitation unavailable")
	}
	if _, err = h.ewsCalendarObject(ctx, actor.ID, obj, false); err != nil {
		return fail("ErrorAccessDenied", "Invitation unavailable")
	}
	if key := nodeAttr(reference, "ChangeKey"); key != "" {
		files, e := h.store.ListCalendarEventAttachments(ctx, obj.ID)
		if e != nil {
			return "", e
		}
		if hmsg, e := h.store.GetMessageByID(ctx, id); e == nil {
			metadata, e := h.store.EWSMailMetadata(ctx, id)
			if e != nil {
				return "", e
			}
			if key != digest(hmsg.ID+hmsg.Flags+metadata) {
				return fail("ErrorIrresolvableConflict", "Invitation changed")
			}
		} else if key != ewsCalendarChangeKey(obj, files) {
			return fail("ErrorIrresolvableConflict", "Invitation changed")
		}
	}
	disposition := nodeAttr(operation, "MessageDisposition")
	if disposition == "" {
		disposition = "SaveOnly"
	}
	if disposition != "SaveOnly" && disposition != "SendOnly" && disposition != "SendAndSaveCopy" {
		return fail("ErrorInvalidRequest", "Invalid MessageDisposition")
	}
	comment := node.value("Body")
	if body := node.child("Body"); body != nil && nodeAttr(body, "BodyType") == "HTML" {
		comment = mailsearch.ExtractHTMLText(comment)
	}
	if len(comment) > 64<<10 {
		return fail("ErrorInvalidRequest", "Response body too large")
	}
	if disposition == "SendAndSaveCopy" {
		ctx = storage.WithCalendarMailOptions(ctx, true, false)
	}
	if rid != "" {
		_, err = h.store.ReplyCalendarOccurrence(ctx, actor.ID, inv.ID, rid, status, obj.ETag, disposition != "SaveOnly", comment)
	} else {
		_, err = h.store.RespondCalendarInvite(ctx, actor.ID, inv.ID, status, obj.ETag, disposition != "SaveOnly", comment)
	}
	if err != nil {
		return calendarWriteError("CreateItem", err)
	}
	return ewsNotificationSuccess("CreateItem", "<m:Items/>"), nil
}

// A copied meeting mail lives in the secretary's Inbox but its ATTENDEE remains
// the principal. The header identifies that principal; fresh grants and the
// matching current organizer/UID/sequence are still required before any RSVP.
func (h *ewsHandler) copiedMeetingTarget(ctx context.Context, actor *storage.User, msg *storage.Message) (*storage.CalendarInvite, *storage.CalendarObject, error) {
	raw, err := h.ms.Read(msg.FilePath)
	if err != nil {
		return nil, nil, err
	}
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, err
	}
	ownerEmail := message.Header.Get("X-Tayga-Delegate-For")
	if ownerEmail == "" {
		return nil, nil, storage.ErrNotFound
	}
	owner, err := h.store.ResolveRecipient(ctx, ownerEmail)
	if err != nil || owner.TenantID != actor.TenantID || !owner.Enabled {
		return nil, nil, storage.ErrUnauthorized
	}
	grant, err := h.store.GetMailboxDelegate(ctx, owner.ID, actor.ID)
	if err != nil || (!grant.CanRespondInvites && grant.EWSCalendarPermission != "Editor") {
		return nil, nil, storage.ErrUnauthorized
	}
	content, err := parseMessageContent(raw)
	if err != nil {
		return nil, nil, err
	}
	var event *calutil.ParseResult
	for _, attachment := range content.Attachments {
		if attachment.ContentType != "text/calendar" {
			continue
		}
		p, e := calutil.ParseICS(string(attachment.Data))
		if e != nil || p.Method != "REQUEST" || event != nil {
			return nil, nil, storage.ErrNotFound
		}
		event = &p
	}
	if event == nil {
		return nil, nil, storage.ErrNotFound
	}
	invites, err := h.store.ListCalendarInvitesByEventUID(ctx, event.UID)
	if err != nil {
		return nil, nil, err
	}
	for _, inv := range invites {
		if inv.AttendeeUserID != owner.ID {
			continue
		}
		obj, e := h.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
		if e != nil {
			continue
		}
		if _, e = h.ewsCalendarObject(ctx, actor.ID, obj, false); e != nil {
			continue
		}
		p, e := calutil.ParseICS(obj.Data)
		if e != nil || p.Sequence != event.Sequence || !strings.EqualFold(p.Organizer.Email, event.Organizer.Email) || strings.EqualFold(p.Status, "CANCELLED") {
			continue
		}
		return inv, obj, nil
	}
	return nil, nil, storage.ErrNotFound
}

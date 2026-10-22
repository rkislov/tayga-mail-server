// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
)

// MeetingResponse is a separate ActiveSync command, not a Sync calendar edit.
func (h *easHandler) meetingResponse(ctx context.Context, u *storage.User, body []byte) (string, error) {
	doc, err := parseProtocolXML(body)
	if err != nil {
		return "", err
	}
	root := doc.child("MeetingResponse")
	if root == nil || len(root.Children) == 0 || len(root.Children) > 100 {
		return "", fmt.Errorf("invalid meeting response")
	}
	var out strings.Builder
	successes, failures := 0, 0
	out.WriteString(`<MeetingResponse xmlns="MeetingResponse:">`)
	for _, req := range root.Children {
		id, status := "", 2
		if req.Name.Local == "Request" {
			id, status = h.respondMeetingRequest(ctx, u, req)
		}
		if status == 1 {
			successes++
		} else {
			failures++
		}
		fmt.Fprintf(&out, `<Result><RequestId>%s</RequestId><Status>%d</Status>`, xmlEscape(req.value("RequestId")), status)
		if status == 1 && req.value("UserResponse") != "3" {
			fmt.Fprintf(&out, `<CalendarId>%s</CalendarId>`, xmlEscape(id))
		}
		out.WriteString(`</Result>`)
	}
	out.WriteString(`</MeetingResponse>`)
	if trace, ok := ctx.Value(traceKey{}).(*requestTrace); ok {
		trace.Result = fmt.Sprintf("meeting_response_ok_%d_failed_%d", successes, failures)
	}
	return out.String(), nil
}

func meetingResponseStatus(err error) int {
	if err == nil {
		return 1
	}
	if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrUnauthorized) {
		return 2
	}
	return 4 // conflicts and temporary storage failures can be retried
}

func (h *easHandler) respondMeetingRequest(ctx context.Context, u *storage.User, req *protocolNode) (string, int) {
	status := map[string]string{"1": "ACCEPTED", "2": "TENTATIVE", "3": "DECLINED"}[req.value("UserResponse")]
	version, _ := ctx.Value(easVersionKey{}).(string)
	send := req.child("SendResponse")
	// Never apply an unsupported occurrence response or time proposal to the
	// entire meeting. Only the organizer may change the scheduled time.
	if status == "" || req.value("CollectionId") == "" || req.value("RequestId") == "" || req.child("InstanceId") != nil || req.child("LongId") != nil ||
		(send != nil && (version != "16.0" && version != "16.1" || send.child("ProposedStartTime") != nil || send.child("ProposedEndTime") != nil)) {
		return "", 2
	}
	seen := map[string]bool{}
	for _, child := range req.Children {
		if seen[child.Name.Local] {
			return "", 2
		}
		seen[child.Name.Local] = true
		switch child.Name.Local {
		case "UserResponse", "CollectionId", "RequestId", "SendResponse":
		default:
			return "", 2
		}
	}
	comment := ""
	if send != nil {
		if len(send.Children) > 1 {
			return "", 2
		}
		for _, child := range send.Children {
			if child.Name.Local != "Body" || (child.value("Type") != "1" && child.value("Type") != "2") {
				return "", 2
			}
			comment = child.value("Data")
			if child.value("Type") == "2" {
				comment = mailsearch.ExtractHTMLText(comment)
			}
			if len(comment) > 64<<10 {
				return "", 2
			}
		}
	}
	inv, obj, err := h.meetingResponseTarget(ctx, u, req.value("CollectionId"), req.value("RequestId"), version)
	if err != nil {
		return "", meetingResponseStatus(err)
	}
	_, err = h.store.RespondCalendarInvite(ctx, u.ID, inv.ID, status, obj.ETag, send != nil, comment)
	return obj.ID, meetingResponseStatus(err)
}

func (h *easHandler) meetingResponseTarget(ctx context.Context, u *storage.User, collection, id, version string) (*storage.CalendarInvite, *storage.CalendarObject, error) {
	kind, err := h.resolveCollection(ctx, u.ID, collection)
	if err != nil {
		return nil, nil, storage.ErrNotFound
	}
	var event calutil.ParseResult
	var objectID string
	switch kind {
	case kindCalendar:
		if version == "14.0" {
			return nil, nil, storage.ErrUnauthorized
		}
		obj, err := h.store.GetCalendarObjectByID(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		if obj.CalendarID != collection {
			return nil, nil, storage.ErrUnauthorized
		}
		event, err = calutil.ParseICS(obj.Data)
		if err != nil {
			return nil, nil, storage.ErrNotFound
		}
		objectID = obj.ID
	case kindMail:
		msg, err := h.store.GetMessageByID(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		if msg.MailboxID != collection {
			return nil, nil, storage.ErrUnauthorized
		}
		if h.ms == nil {
			return nil, nil, fmt.Errorf("mail storage unavailable")
		}
		raw, err := h.ms.Read(msg.FilePath)
		if err != nil {
			return nil, nil, err
		}
		content, err := parseMessageContent(raw)
		if err != nil {
			return nil, nil, storage.ErrNotFound
		}
		found := false
		for _, a := range content.Attachments {
			if a.ContentType != "text/calendar" {
				continue
			}
			p, err := calutil.ParseICS(string(a.Data))
			if err != nil || p.Method != "REQUEST" || found {
				return nil, nil, storage.ErrNotFound
			}
			event, found = p, true
		}
		if !found {
			return nil, nil, storage.ErrNotFound
		}
	default:
		return nil, nil, storage.ErrNotFound
	}
	if event.UID == "" || event.Organizer.Email == "" || strings.EqualFold(event.Organizer.Email, u.Email) || strings.EqualFold(event.Status, "CANCELLED") {
		return nil, nil, storage.ErrUnauthorized
	}
	invites, err := h.store.ListCalendarInvitesByEventUID(ctx, event.UID)
	if err != nil {
		return nil, nil, err
	}
	for _, inv := range invites {
		if inv.AttendeeUserID != u.ID || objectID != "" && inv.CalendarObjectID != objectID {
			continue
		}
		obj, err := h.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
		if err != nil {
			return nil, nil, err
		}
		if _, err = h.store.GetCalendarByID(ctx, u.ID, obj.CalendarID); err != nil {
			return nil, nil, storage.ErrUnauthorized
		}
		p, err := calutil.ParseICS(obj.Data)
		if err != nil || p.Sequence != event.Sequence || !strings.EqualFold(p.Organizer.Email, event.Organizer.Email) || strings.EqualFold(p.Status, "CANCELLED") {
			return nil, nil, storage.ErrNotFound
		}
		return inv, obj, nil
	}
	return nil, nil, storage.ErrNotFound
}

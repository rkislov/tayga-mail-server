package flowsync

import (
	"context"
	"encoding/json"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
	"sort"
	"strings"
)

func (h *ewsHandler) mailboxRight(ctx context.Context, userID, id, right string) error {
	rights, err := h.store.MailboxRightsForUser(ctx, id, userID)
	if err != nil {
		return err
	}
	if !strings.Contains(rights, right) {
		return storage.ErrUnauthorized
	}
	return nil
}
func (h *ewsHandler) ewsCalendar(ctx context.Context, userID, id string, write bool) (*storage.Calendar, error) {
	if cal, err := h.store.GetCalendarByID(ctx, userID, id); err == nil {
		return cal, nil
	}
	rights, err := h.store.CalendarRightsForUser(ctx, id, userID)
	if err != nil || rights == "" || rights == "freebusy" || write && rights != "write" {
		return nil, storage.ErrUnauthorized
	}
	shared, err := h.store.ListSharedCalendars(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, cal := range shared {
		if cal.ID == id && cal.Name != "personal" {
			return cal, nil
		}
	}
	return nil, storage.ErrUnauthorized
}
func (h *ewsHandler) ewsCalendarObject(ctx context.Context, userID string, o *storage.CalendarObject, write bool) (*storage.Calendar, error) {
	cal, err := h.ewsCalendar(ctx, userID, o.CalendarID, write)
	if err != nil {
		return nil, err
	}
	if cal.UserID != userID && calutil.HasPrivateEvents(o.Data) {
		return nil, storage.ErrUnauthorized
	}

	return cal, nil
}
func (h *ewsHandler) calendarPrincipal(ctx context.Context, u *storage.User, id string) (*storage.User, error) {
	cal, err := h.ewsCalendar(ctx, u.ID, id, true)
	if err != nil {
		return nil, err
	}
	if cal.UserID == u.ID {
		return u, nil
	}
	d, err := h.store.GetMailboxDelegate(ctx, cal.UserID, u.ID)
	if err == nil && (d.CanManageCalendar || cal.Name == "default" && d.EWSCalendarPermission == "Editor") {
		return h.store.GetUserByID(ctx, cal.UserID)
	}
	return u, nil // ordinary calendar ACLs retain the authenticated organizer
}
func (h *ewsHandler) delegatedMailbox(ctx context.Context, u *storage.User, email string) (*storage.User, error) {
	owner, err := h.store.ResolveRecipient(ctx, strings.TrimSpace(email))
	if err != nil {
		return nil, err
	}
	if !owner.Enabled || owner.TenantID != u.TenantID || owner.AuthSource == "resource" {
		return nil, storage.ErrUnauthorized
	}
	if owner.ID == u.ID {
		return u, nil
	}
	d, err := h.store.GetMailboxDelegate(ctx, owner.ID, u.ID)
	if err != nil {
		return nil, err
	}
	if !(d.CanSendAs || d.CanSendOnBehalf || d.CanRead || d.CanManageMail || d.CanReadCalendar || d.CanManageCalendar || d.CanRespondInvites || d.EWSInboxPermission == "Reviewer" || d.EWSInboxPermission == "Editor" || d.EWSCalendarPermission == "Reviewer" || d.EWSCalendarPermission == "Editor") {
		return nil, storage.ErrUnauthorized
	}
	return owner, nil
}

func (h *ewsHandler) calendarObjectWrite(ctx context.Context, userID string, o *storage.CalendarObject) error {
	_, err := h.ewsCalendarObject(ctx, userID, o, true)
	return err
}

// Access changes invalidate a cached replay without making ordinary arrivals
// invalidate retry tokens. Mail/calendar data is intentionally absent here.
func (h *ewsHandler) ewsAccessSignature(ctx context.Context, u *storage.User) (string, error) {
	grants, err := h.store.ListDelegationsFor(ctx, u.ID)
	if err != nil {
		return "", err
	}
	sort.Slice(grants, func(i, j int) bool { return grants[i].OwnerID < grants[j].OwnerID })
	raw, err := json.Marshal(grants)
	if err != nil {
		return "", err
	}
	parts := []string{string(raw)}
	mailboxes, err := h.store.ListSharedMailboxes(ctx, u.ID)
	if err != nil {
		return "", err
	}
	for _, mb := range mailboxes {
		rights, err := h.store.MailboxRightsForUser(ctx, mb.ID, u.ID)
		if err != nil {
			return "", err
		}
		parts = append(parts, "m:"+mb.ID+":"+rights)
	}
	calendars, err := h.store.ListSharedCalendars(ctx, u.ID)
	if err != nil {
		return "", err
	}
	for _, cal := range calendars {
		rights, err := h.store.CalendarRightsForUser(ctx, cal.ID, u.ID)
		if err != nil {
			return "", err
		}
		parts = append(parts, "c:"+cal.ID+":"+rights)
	}
	if len(grants) == 0 && len(mailboxes) == 0 && len(calendars) == 0 {
		return "", nil
	}
	sort.Strings(parts)
	return digest(strings.Join(parts, "\x00")), nil
}
func (h *ewsHandler) replayCalendarPrivacy(ctx context.Context, u *storage.User, response string) bool {
	doc, err := parseProtocolXML([]byte(response))
	if err != nil {
		return false
	}
	var visit func(*protocolNode) bool
	visit = func(n *protocolNode) bool {
		if n.Name.Local == "CalendarItem" {
			id := nodeAttr(n.child("ItemId"), "Id")
			if strings.HasPrefix(id, "ews-occ:") {
				id, _, err = parseOccurrenceID(id)
				if err != nil {
					return false
				}
			}
			o, err := h.store.GetCalendarObjectByID(ctx, id)
			if err != nil {
				return false
			}
			if _, err = h.ewsCalendarObject(ctx, u.ID, o, false); err != nil {
				return false
			}
		}
		for _, child := range n.Children {
			if !visit(child) {
				return false
			}
		}
		return true
	}
	return visit(doc)
}

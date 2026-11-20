package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

const CalendarDownloadPath = "/calendar-download/"

type CalendarAttachmentLink struct {
	Attachment *CalendarAttachment
	URL        string
}
type calendarDownloadClaim struct {
	Invite, File string
	Expires      int64
}

// Configure once before starting protocol listeners. A purpose-derived key
// keeps download signatures separate from every other use of the server key.
func (s *Store) SetCalendarAttachmentLinks(base string, key []byte) error {
	if base == "" {
		return nil
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(key) < 32 {
		return fmt.Errorf("invalid calendar download configuration")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("tayga/calendar-download/v1"))
	s.calendarLinkKey, s.calendarLinkBase = mac.Sum(nil), strings.TrimRight(base, "/")
	return nil
}

func (s *Store) calendarDownloadURL(inv *CalendarInvite, file *CalendarAttachment, end *time.Time) string {
	if len(s.calendarLinkKey) == 0 {
		return ""
	}
	expires := inv.CreatedAt.AddDate(0, 0, 30)
	if v := file.CreatedAt.AddDate(0, 0, 30); v.After(expires) {
		expires = v
	}
	if end != nil {
		if v := end.AddDate(0, 0, 30); v.After(expires) {
			expires = v
		}
	}
	if !expires.After(time.Now()) {
		return ""
	}
	data, _ := json.Marshal(calendarDownloadClaim{Invite: inv.ID, File: file.ID, Expires: expires.Unix()})
	payload := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, s.calendarLinkKey)
	mac.Write([]byte(payload))
	return s.calendarLinkBase + CalendarDownloadPath + payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Links are issued only for the authenticated recipient's own invitation copy.
// They are bearer links, scoped to one attachment and one invitation, and never
// alter either calendar object or the recipient's RSVP during synchronization.
func (s *Store) CalendarAttachmentLinks(ctx context.Context, objectID, userID string) ([]CalendarAttachmentLink, error) {
	if len(s.calendarLinkKey) == 0 {
		return nil, nil
	}
	obj, err := s.GetCalendarObjectByID(ctx, objectID)
	if err != nil {
		return nil, err
	}
	if _, err = s.GetCalendarByID(ctx, userID, obj.CalendarID); err != nil {
		return nil, err
	}
	var id string
	err = s.db.QueryRowContext(ctx, s.rebind(`SELECT id FROM calendar_invites WHERE calendar_object_id=? AND attendee_user_id=? AND event_uid=? AND partstat IN ('NEEDS-ACTION','ACCEPTED','TENTATIVE') ORDER BY id LIMIT 1`), objectID, userID, obj.UID).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	inv, err := s.GetCalendarInvite(ctx, id)
	if err != nil {
		return nil, err
	}
	files, err := s.ListCalendarEventAttachments(ctx, objectID)
	if err != nil {
		return nil, err
	}
	var links []CalendarAttachmentLink
	for _, file := range files {
		if _, err = s.calendarInviteFile(ctx, inv, file.ID); err != nil {
			if err == ErrNotFound {
				continue
			}
			return nil, err
		}
		if link := s.calendarDownloadURL(inv, file, obj.DTEnd); link != "" {
			links = append(links, CalendarAttachmentLink{file, link})
		}
	}
	return links, nil
}

func (s *Store) calendarInviteFile(ctx context.Context, inv *CalendarInvite, fileID string) (*CalendarAttachment, error) {
	if inv.PartStat != "NEEDS-ACTION" && inv.PartStat != "ACCEPTED" && inv.PartStat != "TENTATIVE" {
		return nil, ErrNotFound
	}
	file, err := s.GetCalendarAttachment(ctx, fileID)
	if err != nil {
		return nil, err
	}
	obj, err := s.GetCalendarObjectByID(ctx, file.CalendarObjectID)
	if err != nil {
		return nil, err
	}
	if obj.UID != inv.EventUID {
		return nil, ErrNotFound
	}
	if _, err = s.GetCalendarByID(ctx, inv.OrganizerUserID, obj.CalendarID); err != nil {
		return nil, ErrNotFound
	}
	org, err := s.GetUserByID(ctx, inv.OrganizerUserID)
	if err != nil {
		return nil, err
	}
	p, err := calutil.ParseICS(obj.Data)
	if err != nil || p.UID != inv.EventUID || strings.EqualFold(p.Status, "CANCELLED") || !strings.EqualFold(p.Organizer.Email, org.Email) {
		return nil, ErrNotFound
	}
	for _, a := range p.Attendees {
		if strings.EqualFold(a.Email, inv.AttendeeEmail) && !strings.EqualFold(a.PartStat, "DECLINED") {
			return file, nil
		}
	}
	return nil, ErrNotFound
}

// Validation includes current membership, so a forwarded URL stops working as
// soon as the invitation is declined, the guest is removed, or the event/file
// is cancelled/deleted. A signature alone never bypasses those checks.
func (s *Store) CalendarDownloadAttachment(ctx context.Context, token string) (*CalendarAttachment, error) {
	if len(s.calendarLinkKey) == 0 || len(token) > 1024 {
		return nil, ErrNotFound
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, ErrNotFound
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrNotFound
	}
	mac := hmac.New(sha256.New, s.calendarLinkKey)
	mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, ErrNotFound
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrNotFound
	}
	var claim calendarDownloadClaim
	if json.Unmarshal(data, &claim) != nil || claim.Expires <= time.Now().Unix() || claim.Invite == "" || claim.File == "" {
		return nil, ErrNotFound
	}
	inv, err := s.GetCalendarInvite(ctx, claim.Invite)
	if err != nil {
		return nil, err
	}
	return s.calendarInviteFile(ctx, inv, claim.File)
}

// The invitation is personalized before entering the transactional outbox.
// Never store recipient bearer links in the organizer's canonical ICS.
func (s *Store) calendarMailDownloadLinks(ctx context.Context, tx *sql.Tx, m *CalendarMail) error {
	if m.Method != "REQUEST" || len(s.calendarLinkKey) == 0 {
		return nil
	}
	p, err := calutil.ParseICS(m.ICS)
	if err != nil {
		return err
	}
	var inv CalendarInvite
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT i.id,i.created_at FROM calendar_invites i JOIN users u ON u.id=i.organizer_user_id WHERE i.event_uid=? AND LOWER(u.email)=LOWER(?) AND LOWER(i.attendee_email)=LOWER(?) AND i.partstat IN ('NEEDS-ACTION','ACCEPTED','TENTATIVE')`), p.UID, m.From, m.To).Scan(&inv.ID, &inv.CreatedAt)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, s.rebind(`SELECT a.id,a.calendar_object_id,a.user_id,a.filename,a.content_type,a.size,a.storage_path,a.created_at FROM calendar_attachments a JOIN calendar_objects o ON o.id=a.calendar_object_id JOIN calendars c ON c.id=o.calendar_id JOIN users u ON u.id=c.user_id WHERE o.uid=? AND LOWER(u.email)=LOWER(?) ORDER BY a.id`), p.UID, m.From)
	if err != nil {
		return err
	}
	var files []*CalendarAttachment
	for rows.Next() {
		a, e := s.scanCalendarAttachment(rows)
		if e != nil {
			rows.Close()
			return e
		}
		files = append(files, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var attachments []calutil.Attachment
	for _, a := range files {
		if link := s.calendarDownloadURL(&inv, a, p.End); link != "" {
			attachments = append(attachments, calutil.Attachment{URI: link, Filename: a.Filename, ContentType: a.ContentType})
			m.Text += "\n\n" + a.Filename + "\n" + link
		}
	}
	if len(attachments) == 0 {
		return nil
	}
	m.ICS, err = calutil.WithDownloadAttachments(m.ICS, s.calendarLinkBase, attachments)
	return err
}

package storage

import (
	"context"
	"strings"

	"github.com/tayga/tms/internal/calutil"
)

// ListCalendarEventAttachments returns local files and the organizer's files
// explicitly shared with this invitation copy. Callers must separately verify
// read access to the requested event. No blobs or file records are duplicated.
func (s *Store) ListCalendarEventAttachments(ctx context.Context, objectID string) ([]*CalendarAttachment, error) {
	files, err := s.ListCalendarAttachments(ctx, objectID)
	if err != nil {
		return nil, err
	}
	// The invitation must name this exact copy, owned by its intended recipient.
	// A guessed UID or a forged ORGANIZER property alone never grants access.
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT src.id, src.data, org.email, i.attendee_email, dst.data
		FROM calendar_invites i
		JOIN calendar_objects dst ON dst.id=i.calendar_object_id AND dst.uid=i.event_uid
		JOIN calendars recipient ON recipient.id=dst.calendar_id AND recipient.user_id=i.attendee_user_id
		JOIN users org ON org.id=i.organizer_user_id
		JOIN calendars sender ON sender.user_id=org.id
		JOIN calendar_objects src ON src.calendar_id=sender.id AND src.uid=i.event_uid AND src.id<>dst.id
		WHERE dst.id=? AND i.partstat IN ('NEEDS-ACTION','ACCEPTED','TENTATIVE')
		ORDER BY src.id`), objectID)
	if err != nil {
		return nil, err
	}
	var sources []string
	for rows.Next() {
		var id, data, organizer, attendee, copyData string
		if err = rows.Scan(&id, &data, &organizer, &attendee, &copyData); err != nil {
			rows.Close()
			return nil, err
		}
		meeting, e1 := calutil.ParseICS(data)
		copy, e2 := calutil.ParseICS(copyData)
		if e1 != nil || e2 != nil || meeting.UID != copy.UID || !strings.EqualFold(meeting.Organizer.Email, organizer) || !strings.EqualFold(copy.Organizer.Email, organizer) || strings.EqualFold(meeting.Status, "CANCELLED") || strings.EqualFold(copy.Status, "CANCELLED") {
			continue
		}
		for _, guest := range meeting.Attendees {
			if strings.EqualFold(guest.Email, attendee) && !strings.EqualFold(guest.PartStat, "DECLINED") {
				sources = append(sources, id)
				break
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, f := range files {
		seen[f.ID] = true
	}
	for _, source := range sources {
		shared, err := s.ListCalendarAttachments(ctx, source)
		if err != nil {
			return nil, err
		}
		for _, f := range shared {
			if !seen[f.ID] {
				files = append(files, f)
				seen[f.ID] = true
			}
		}
	}
	return files, nil
}

// CalendarAttachmentSharedWith authorizes a download through an existing local
// invitation. It does not grant permission to edit/delete the organizer's file.
func (s *Store) CalendarAttachmentSharedWith(ctx context.Context, attachmentID, userID string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT DISTINCT dst.id FROM calendar_attachments a
		JOIN calendar_objects src ON src.id=a.calendar_object_id
		JOIN calendar_invites i ON i.event_uid=src.uid
		JOIN calendar_objects dst ON dst.id=i.calendar_object_id
		JOIN calendars c ON c.id=dst.calendar_id AND c.user_id=i.attendee_user_id
		WHERE a.id=? AND c.user_id=?`), attachmentID, userID)
	if err != nil {
		return false, err
	}
	var copies []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return false, err
		}
		copies = append(copies, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	for _, id := range copies {
		files, err := s.ListCalendarEventAttachments(ctx, id)
		if err != nil {
			return false, err
		}
		for _, f := range files {
			if f.ID == attachmentID {
				return true, nil
			}
		}
	}
	return false, nil
}

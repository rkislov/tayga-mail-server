package flowsync

import (
	"context"
	"fmt"
	"github.com/tayga/tms/internal/calutil"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// Calendar files use AirSyncBase attachments in EAS 16.x. FileReference is an
// opaque, stable identifier. Invited guests can also use their signed URL
// directly in a browser without requiring the organizer's web session.
func (h *easHandler) calendarAttachmentsXML(ctx context.Context, objectID string, links ...storage.CalendarAttachmentLink) (string, error) {
	version, _ := ctx.Value(easVersionKey{}).(string)
	if version != "16.0" && version != "16.1" {
		return "", nil
	}
	files, err := h.store.ListCalendarEventAttachments(ctx, objectID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<Attachments xmlns="AirSyncBase:">`)
	refs := map[string]string{}
	for _, link := range links {
		refs[link.Attachment.ID] = link.URL
	}
	for _, file := range files {
		ref := "calendar:" + objectID + ":" + file.ID
		if signed := refs[file.ID]; signed != "" {
			ref = signed
		}
		fmt.Fprintf(&b, `<Attachment><DisplayName>%s</DisplayName><FileReference>%s</FileReference><Method>1</Method><EstimatedDataSize>%d</EstimatedDataSize><IsInline>0</IsInline></Attachment>`, xmlEscape(file.Filename), xmlEscape(ref), file.Size)
	}
	b.WriteString(`</Attachments>`)
	return b.String(), nil
}

func (h *easHandler) calendarAttachment(ctx context.Context, u *storage.User, reference string) (messageAttachment, error) {
	return readCalendarAttachment(ctx, h.store, h.ms, u, reference)
}

// Both EAS and EWS apply the same current-event ownership and file checks.
func readCalendarAttachment(ctx context.Context, store storage.Driver, ms *mailstore.Store, u *storage.User, reference string) (messageAttachment, error) {
	if ms == nil {
		return messageAttachment{}, storage.ErrNotFound
	}
	if token := calendarDownloadToken(reference); token != "" {
		file, err := store.CalendarDownloadAttachment(ctx, token)
		if err != nil {
			return messageAttachment{}, err
		}
		return readCalendarFile(ms, file)
	}
	parts := strings.Split(reference, ":")
	if len(parts) != 3 || parts[0] != "calendar" || ms == nil {
		return messageAttachment{}, storage.ErrNotFound
	}
	object, err := store.GetCalendarObjectByID(ctx, parts[1])
	if err != nil {
		return messageAttachment{}, err
	}
	// A known file ID grants no access. Match the current, owned event as well
	// as the attachment; removal of either revokes an old reference immediately.
	if _, err = store.GetCalendarByID(ctx, u.ID, object.CalendarID); err != nil {
		return messageAttachment{}, err
	}
	file, err := store.GetCalendarAttachment(ctx, parts[2])
	if err != nil {
		return messageAttachment{}, err
	}
	if file.CalendarObjectID != object.ID {
		files, err := store.ListCalendarEventAttachments(ctx, object.ID)
		if err != nil {
			return messageAttachment{}, err
		}
		allowed := false
		for _, shared := range files {
			if shared.ID == file.ID {
				allowed = true
				break
			}
		}
		if !allowed {
			return messageAttachment{}, storage.ErrNotFound
		}
	}
	return readCalendarFile(ms, file)
}

func readCalendarFile(ms *mailstore.Store, file *storage.CalendarAttachment) (messageAttachment, error) {
	if !filepath.IsLocal(file.StoragePath) || file.Size < 0 || file.Size > 32<<20 {
		return messageAttachment{}, storage.ErrNotFound
	}
	root, err := os.OpenRoot(filepath.Join(ms.Root, "calendar-attachments"))
	if err != nil {
		return messageAttachment{}, err
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(file.StoragePath))
	if err != nil {
		return messageAttachment{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != file.Size {
		return messageAttachment{}, storage.ErrNotFound
	}
	data, err := io.ReadAll(io.LimitReader(f, file.Size+1))
	if err != nil || int64(len(data)) != file.Size {
		return messageAttachment{}, fmt.Errorf("calendar attachment read failed")
	}
	return messageAttachment{Name: file.Filename, ContentType: file.ContentType, Data: data}, nil
}

func calendarDownloadToken(reference string) string {
	u, err := url.Parse(reference)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	_, token, ok := strings.Cut(u.Path, storage.CalendarDownloadPath)
	if !ok || token == "" || strings.Contains(token, "/") {
		return ""
	}
	return token
}

func calendarDownloadDescription(description string, links []storage.CalendarAttachmentLink) string {
	var attachments []calutil.Attachment
	for _, link := range links {
		attachments = append(attachments, calutil.Attachment{URI: link.URL, Filename: link.Attachment.Filename})
	}
	return calutil.DownloadDescription(description, attachments)
}

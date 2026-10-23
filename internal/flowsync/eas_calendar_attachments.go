package flowsync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// Calendar files use AirSyncBase attachments in EAS 16.x. FileReference is an
// opaque, stable identifier, not the web URL (which requires a web session).
func (h *easHandler) calendarAttachmentsXML(ctx context.Context, objectID string) (string, error) {
	version, _ := ctx.Value(easVersionKey{}).(string)
	if version != "16.0" && version != "16.1" {
		return "", nil
	}
	files, err := h.store.ListCalendarAttachments(ctx, objectID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<Attachments xmlns="AirSyncBase:">`)
	for _, file := range files {
		fmt.Fprintf(&b, `<Attachment><DisplayName>%s</DisplayName><FileReference>%s</FileReference><Method>1</Method><EstimatedDataSize>%d</EstimatedDataSize><IsInline>0</IsInline></Attachment>`, xmlEscape(file.Filename), xmlEscape("calendar:"+objectID+":"+file.ID), file.Size)
	}
	b.WriteString(`</Attachments>`)
	return b.String(), nil
}

func (h *easHandler) calendarAttachment(ctx context.Context, u *storage.User, reference string) (messageAttachment, error) {
	return readCalendarAttachment(ctx, h.store, h.ms, u, reference)
}

// Both EAS and EWS apply the same current-event ownership and file checks.
func readCalendarAttachment(ctx context.Context, store storage.Driver, ms *mailstore.Store, u *storage.User, reference string) (messageAttachment, error) {
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
	if file.CalendarObjectID != object.ID || !filepath.IsLocal(file.StoragePath) || file.Size < 0 || file.Size > 32<<20 {
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

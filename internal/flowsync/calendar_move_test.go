package flowsync

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestCalendarMovesPreserveMeetingAndFiles(t *testing.T) {
	f := newMeetingFixture(t)
	ctx := context.WithValue(f.ctx, easVersionKey{}, "16.1")
	f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "1", "16.1", "")
	drainCalendarJobs(t, ctx, f.st)
	original, _ := f.st.GetCalendarObjectByID(ctx, f.original.ID)
	destination, err := f.st.EnsureCalendar(ctx, f.owner.ID, "work", "Work")
	if err != nil {
		t.Fatal(err)
	}
	source, _ := f.st.GetCalendarByID(ctx, f.owner.ID, original.CalendarID)
	data := []byte("%PDF-1.7\nboarding pass\n")
	root := filepath.Join(f.h.ms.Root, "calendar-attachments")
	if err = os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "boarding.pdf"), data, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := f.st.CreateCalendarAttachment(ctx, &storage.CalendarAttachment{CalendarObjectID: original.ID, UserID: f.owner.ID, Filename: "boarding.pdf", ContentType: "application/pdf", Size: int64(len(data)), StoragePath: "boarding.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	move := func(user *storage.User, src, dst string) string {
		t.Helper()
		body := `<MoveItems xmlns="Move:"><Move><SrcMsgId>` + original.ID + `</SrcMsgId><SrcFldId>` + src + `</SrcFldId><DstFldId>` + dst + `</DstFldId></Move></MoveItems>`
		wire, err := encodeWire(body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=MoveItems&DeviceId=move-phone&DeviceType=iPhone", bytes.NewReader(wire)).WithContext(withUser(ctx, user))
		r.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
		r.Header.Set("MS-ASProtocolVersion", "16.1")
		w := httptest.NewRecorder()
		f.h.ServeHTTP(w, r)
		xml, err := decodeWire(w.Body.Bytes())
		if err != nil || w.Code != 200 {
			t.Fatal(w.Code, err)
		}
		return string(xml)
	}
	for i := 0; i < 2; i++ {
		result := move(f.owner, source.ID, destination.ID)
		if extractTag(result, "Status") != "3" || extractTag(result, "DstMsgId") != original.ID {
			t.Fatal(result)
		}
	}
	saved, _ := f.st.GetCalendarObjectByID(ctx, original.ID)
	if saved.CalendarID != destination.ID || saved.Data != original.Data || saved.UID != original.UID || saved.ETag != original.ETag {
		t.Fatal("move changed meeting", saved)
	}
	items, _ := f.st.ListCalendarObjects(ctx, source.ID)
	if len(items) != 0 {
		t.Fatal("source retained duplicate")
	}
	newSource, _ := f.st.GetCalendarByID(ctx, f.owner.ID, source.ID)
	newDestination, _ := f.st.GetCalendarByID(ctx, f.owner.ID, destination.ID)
	if newSource.CTag == source.CTag || newDestination.CTag == destination.CTag {
		t.Fatal("move did not notify both collections")
	}
	ref := "calendar:" + original.ID + ":" + file.ID
	got, err := f.h.calendarAttachment(ctx, f.owner, ref)
	if err != nil || !bytes.Equal(got.Data, data) {
		t.Fatal("move lost file", err)
	}
	// Fetch metadata through the binary transport, then use the returned opaque reference.
	xml, _, err := f.h.syncCalendar(ctx, f.owner, destination.ID, "key", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := encodeWire(xml)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeWire(wire)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := parseProtocolXML(decoded)
	if doc.find("FileReference") == nil || doc.find("FileReference").Text != ref {
		t.Fatal("binary sync lost attachment reference", string(decoded))
	}
	// An Add replay carrying the original UID after a move must reuse it.
	op := calendarOp(t, "add", "", `<ClientUid>`+original.UID+`</ClientUid><Subject>stale</Subject>`)
	id, err := f.h.wbCalendar(ctx, f.owner, destination.ID, op)
	if err != nil || id != original.ID {
		t.Fatal("replayed move created duplicate", id, err)
	}
	if result := move(f.guest, destination.ID, source.ID); extractTag(result, "Status") == "3" {
		t.Fatal("foreign move allowed", result)
	}
	if result := move(f.owner, destination.ID, f.received.CalendarID); extractTag(result, "Status") != "2" {
		t.Fatal("foreign destination allowed", result)
	}
	// Outlook can move it back, preserving identity and downloadable files.
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	body := `<MoveItem><ToFolderId><FolderId Id="` + source.ID + `"/></ToFolderId><ItemIds><ItemId Id="` + original.ID + `"/></ItemIds></MoveItem>`
	result, err := h.moveItem(ctx, f.owner, body)
	if err != nil || !strings.Contains(result, "NoError") || !strings.Contains(result, "CalendarItem") {
		t.Fatal(result, err)
	}
	saved, _ = f.st.GetCalendarObjectByID(ctx, original.ID)
	if saved.CalendarID != source.ID || saved.Data != original.Data {
		t.Fatal("Outlook move corrupted event")
	}
	if jobs := drainCalendarJobs(t, ctx, f.st); len(jobs) != 0 {
		t.Fatal("move sent invitations", len(jobs))
	}
	guest, _ := f.st.GetCalendarObjectByID(ctx, f.received.ID)
	if !strings.Contains(guest.Data, "PARTSTAT=ACCEPTED") {
		t.Fatal("move lost RSVP")
	}
	// A real UID collision must roll back rather than overwrite the destination.
	_, err = f.st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: destination.ID, UID: original.UID, HrefName: "collision.ics", Component: "VEVENT", Data: original.Data})
	if err != nil {
		t.Fatal(err)
	}
	if result := move(f.owner, source.ID, destination.ID); extractTag(result, "Status") != "5" {
		t.Fatal(result)
	}
	saved, _ = f.st.GetCalendarObjectByID(ctx, original.ID)
	if saved.CalendarID != source.ID {
		t.Fatal("failed move changed source")
	}
}

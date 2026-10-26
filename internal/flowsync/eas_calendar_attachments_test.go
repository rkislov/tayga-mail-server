package flowsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestCalendarAttachmentsSyncDownloadAndRemoval(t *testing.T) {
	ctx, st, user, handler, device, _ := syncFixture(t)
	ctx = context.WithValue(ctx, easVersionKey{}, "16.1")
	calendar, _ := st.GetCalendarByName(ctx, user.ID, "default")
	id, err := handler.wbCalendar(ctx, user, calendar.ID, calendarOp(t, "add", "", `<Subject>Flight</Subject><StartTime>20261023T161000Z</StartTime><EndTime>20261023T171000Z</EndTime>`))
	if err != nil {
		t.Fatal(err)
	}
	call := func(key string) string {
		t.Helper()
		body := `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + calendar.ID + `</CollectionId></Collection></Collections></Sync>`
		result, err := handler.syncCollections(ctx, user, device, []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	initial := call(extractTag(call("0"), "SyncKey"))
	key := extractTag(initial, "SyncKey")
	data := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte{0, 1, 127, 255}, 330000)...)
	root := filepath.Join(handler.ms.Root, "calendar-attachments")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "boarding.pdf"), data, 0600); err != nil {
		t.Fatal(err)
	}
	attachment, err := st.CreateCalendarAttachment(ctx, &storage.CalendarAttachment{CalendarObjectID: id, UserID: user.ID, Filename: "Посадочный талон & билет.pdf", ContentType: "application/pdf", Size: int64(len(data)), StoragePath: "boarding.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	ref := "calendar:" + id + ":" + attachment.ID
	changed := call(key)
	if !strings.Contains(changed, "<Change ") || strings.Contains(changed, "<Add ") {
		t.Fatal("attachment did not update the existing event", changed)
	}
	doc, err := parseProtocolXML([]byte(changed))
	if err != nil {
		t.Fatal(err)
	}
	file := doc.find("Attachment")
	if file == nil || file.Name.Space != "AirSyncBase:" || file.value("FileReference") != ref || file.value("DisplayName") != attachment.Filename || file.value("Method") != "1" {
		t.Fatal(changed)
	}
	if call(key) != changed {
		t.Fatal("retry not stable")
	}
	key = extractTag(changed, "SyncKey")
	idle := call(key)
	if strings.Contains(idle, "ApplicationData") {
		t.Fatal("unchanged attachment repeated")
	}
	key = extractTag(idle, "SyncKey")
	// Exercise the real WBXML/HTTP download used by iPhone, including bytes
	// which would be corrupted if binary were treated as UTF-8 text.
	request := `<ItemOperations xmlns="ItemOperations:"><Fetch><Store>Mailbox</Store><FileReference xmlns="AirSyncBase:">` + ref + `</FileReference></Fetch></ItemOperations>`
	wire, err := encodeWire(request)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=ItemOperations&DeviceId=test", bytes.NewReader(wire)).WithContext(withUser(ctx, user))
	r.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
	r.Header.Set("MS-ASProtocolVersion", "16.1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	xml, err := decodeWire(w.Body.Bytes())
	if err != nil || w.Code != 200 {
		t.Fatal(w.Code, err)
	}
	doc, err = parseProtocolXML(xml)
	if err != nil || doc.find("Fetch").value("Status") != "1" {
		t.Fatal(string(xml), err)
	}
	got, err := base64.StdEncoding.DecodeString(doc.find("Properties").value("Data"))
	if err != nil || !bytes.Equal(got, data) || doc.find("ContentType").Text != "application/pdf" {
		t.Fatal("file corrupted", err)
	}
	ranged, err := handler.itemOperations(ctx, user, []byte(strings.Replace(request, "</Fetch>", "<Options><Range>1-1024</Range></Options></Fetch>", 1)))
	if err != nil {
		t.Fatal(err)
	}
	doc, _ = parseProtocolXML([]byte(ranged))
	chunk, err := base64.StdEncoding.DecodeString(doc.find("Properties").value("Data"))
	if err != nil || !bytes.Equal(chunk, data[1:1025]) || doc.find("Properties").value("Range") != "1-1024" {
		t.Fatal("invalid partial attachment", err)
	}
	other := *user
	other.ID = "another-user"
	denied, err := handler.itemOperations(ctx, &other, []byte(request))
	if err != nil || strings.Contains(denied, "<Data>") || !strings.Contains(denied, "<Status>6</Status>") {
		t.Fatal("file leaked", denied, err)
	}
	for _, v := range []string{"14.1", "16.0", "16.1"} {
		result, err := handler.calendarAttachmentsXML(context.WithValue(ctx, easVersionKey{}, v), id)
		if err != nil || (v == "14.1" && result != "") || (v != "14.1" && !strings.Contains(result, ref)) {
			t.Fatal(v, result, err)
		}
	}
	// An ordinary iPhone title edit must retain the separately stored files.
	if _, err := handler.wbCalendar(ctx, user, calendar.ID, calendarOp(t, "change", id, `<Subject>Updated flight</Subject>`)); err != nil {
		t.Fatal(err)
	}
	files, _ := st.ListCalendarAttachments(ctx, id)
	if len(files) != 1 {
		t.Fatal("calendar edit lost attachment")
	}
	updated := call(key)
	key = extractTag(updated, "SyncKey")
	if err := st.DeleteCalendarAttachment(ctx, attachment.ID); err != nil {
		t.Fatal(err)
	}
	removed := call(key)
	doc, _ = parseProtocolXML([]byte(removed))
	if doc.find("Change") == nil || doc.find("Attachment") != nil {
		t.Fatal("attachment removal not synchronized", removed)
	}
	denied, err = handler.itemOperations(ctx, user, []byte(request))
	if err != nil || strings.Contains(denied, "<Data>") {
		t.Fatal("removed attachment still downloadable", denied, err)
	}
}

func TestAttachmentRanges(t *testing.T) {
	for _, tc := range []struct {
		input, data, rangeXML string
		status                int
	}{
		{"", "abcdef", "", 1},
		{"<Range>0-0</Range>", "a", "<Range>0-0</Range><Total>6</Total>", 1},
		{"<Range>3-999</Range>", "def", "<Range>3-5</Range><Total>6</Total>", 1},
		{"<Range>6-7</Range>", "", "", 8},
		{"<Range>2-1</Range>", "", "", 8},
		{"<Range>-1-5</Range>", "", "", 8},
		{"<Range>0-99999999999999999999999</Range>", "", "", 8},
		{"<Range>abc</Range>", "", "", 8},
		{"<Range/>", "", "", 8},
		{"<BodyPreference/>", "", "", 2},
		{"<Range>0-1</Range><Range>2-3</Range>", "", "", 2},
	} {
		doc, err := parseProtocolXML([]byte("<Options>" + tc.input + "</Options>"))
		if err != nil {
			t.Fatal(err)
		}
		data, rangeXML, status := attachmentByteRange([]byte("abcdef"), doc.find("Options"))
		if string(data) != tc.data || rangeXML != tc.rangeXML || status != tc.status {
			t.Fatal(tc.input, string(data), rangeXML, status)
		}
	}
}

func TestCalendarAttachmentReferencesCannotEscapeStorage(t *testing.T) {
	ctx, st, user, h, _, _ := syncFixture(t)
	calendar, _ := st.GetCalendarByName(ctx, user.ID, "default")
	first, _ := h.wbCalendar(ctx, user, calendar.ID, calendarOp(t, "add", "", `<Subject>First</Subject><StartTime>20261023T161000Z</StartTime><EndTime>20261023T171000Z</EndTime>`))
	second, _ := h.wbCalendar(ctx, user, calendar.ID, calendarOp(t, "add", "", `<Subject>Second</Subject><StartTime>20261023T161000Z</StartTime><EndTime>20261023T171000Z</EndTime>`))
	root := filepath.Join(h.ms.Root, "calendar-attachments")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "private")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../private", outside, "link"} {
		a, err := st.CreateCalendarAttachment(ctx, &storage.CalendarAttachment{CalendarObjectID: first, UserID: user.ID, Filename: "file", StoragePath: path, Size: 6})
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{first, second} {
			if _, err := h.calendarAttachment(ctx, user, "calendar:"+id+":"+a.ID); err == nil {
				t.Fatal("invalid reference permitted", path, id)
			}
		}
	}
}

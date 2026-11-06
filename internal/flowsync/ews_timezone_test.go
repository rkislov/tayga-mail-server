package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
)

func TestEWSCalendarTimeZoneNamesAndCachedReplay(t *testing.T) {
	for _, tc := range []struct{ name, zone, want string }{
		{"utc", "", "UTC"},
		{"moscow", "Europe/Moscow", "Russian Standard Time"},
		{"berlin", "Europe/Berlin", "W. Europe Standard Time"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, st, u, eas, _, _ := syncFixture(t)
			h := &ewsHandler{store: st, ms: eas.ms}
			cal, err := st.GetCalendarByName(ctx, u.ID, "default")
			if err != nil {
				t.Fatal(err)
			}
			dates := "DTSTART:20261106T100000Z\r\nDTEND:20261106T110000Z\r\n"
			if tc.zone != "" {
				dates = "DTSTART;TZID=" + tc.zone + ":20261106T100000\r\nDTEND;TZID=" + tc.zone + ":20261106T110000\r\n"
			}
			obj, err := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, HrefName: "timezone.ics", UID: "timezone", Component: "VEVENT", Data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:timezone\r\nSUMMARY:Timezone test\r\n" + dates + "END:VEVENT\r\nEND:VCALENDAR\r\n"})
			if err != nil {
				t.Fatal(err)
			}
			check := func(doc *protocolNode) {
				t.Helper()
				for _, field := range []string{"StartTimeZone", "EndTimeZone"} {
					n := doc.find(field)
					if n == nil || nodeAttr(n, "Id") != tc.want || nodeAttr(n, "Name") != tc.want {
						t.Fatalf("Outlook timezone attributes missing for %s: %s", field, doc.render())
					}
				}
			}
			get := calendarSOAP(t, h, ctx, u, `<m:GetItem><m:ItemIds><t:ItemId Id="`+obj.ID+`"/></m:ItemIds></m:GetItem>`)
			check(get)
			initial := calendarSOAP(t, h, ctx, u, `<m:SyncFolderItems><m:SyncFolderId><t:FolderId Id="`+cal.ID+`"/></m:SyncFolderId></m:SyncFolderItems>`)
			check(initial)
			legacy := strings.ReplaceAll(initial.render(), ` Name="`+tc.want+`"`, "")
			if strings.Contains(legacy, ` Name="`+tc.want+`"`) || !strings.Contains(legacy, "StartTimeZone") {
				t.Fatal("legacy fixture invalid")
			}
			body := `<SyncFolderItems><SyncFolderId><FolderId Id="` + cal.ID + `"/></SyncFolderId><SyncState>ews:timezone-replay/old</SyncState></SyncFolderItems>`
			err = saveCollectionState(ctx, st, u.ID, "ews:timezone-replay", cal.ID, collectionState{Key: "next", PreviousKey: "old", RequestHash: digest(body), Response: legacy})
			if err != nil {
				t.Fatal(err)
			}
			result, err := h.syncEWSItems(ctx, u, body)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := parseProtocolXML([]byte(result))
			if err != nil {
				t.Fatal(err)
			}
			check(doc)
			if doc.find("SyncState").Text != initial.find("SyncState").Text {
				t.Fatal("cached replay changed sync token")
			}
		})
	}
}

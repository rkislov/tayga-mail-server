package flowsync

import (
	"context"
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestCalendarRecurrenceUsesCompactDates(t *testing.T) {
	data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:date-fixture\r\nDTSTART:20261106T060000Z\r\nDTEND:20261106T070000Z\r\nRRULE:FREQ=DAILY;UNTIL=20261110T060000Z\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nUID:date-fixture\r\nRECURRENCE-ID:20261107T060000Z\r\nDTSTART:20261107T080000Z\r\nDTEND:20261107T090000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	for _, version := range []string{"14.1", "16.1"} {
		ctx := context.WithValue(context.Background(), easVersionKey{}, version)
		text, err := calendarRecurrenceXML(ctx, data, "")
		if err != nil {
			t.Fatal(err)
		}
		doc, err := parseProtocolXML([]byte("<Event>" + text + "</Event>"))
		if err != nil {
			t.Fatal(err)
		}
		var check func(*protocolNode)
		check = func(n *protocolNode) {
			if n.Name.Space == "Calendar:" {
				switch n.Name.Local {
				case "Until", "StartTime", "EndTime", "ExceptionStartTime":
					if _, e := time.Parse("20060102T150405Z", n.Text); e != nil {
						t.Errorf("%s %s has invalid compact time %q", version, n.Name.Local, n.Text)
					}
				}
			}
			for _, c := range n.Children {
				check(c)
			}
		}
		check(doc)
		binary, err := encodeWire(`<Sync xmlns="AirSync:"><Collections><Collection><CollectionId>calendar</CollectionId><Commands><Add><ServerId>fixture</ServerId><ApplicationData>` + text + `</ApplicationData></Add></Commands></Collection></Collections></Sync>`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = decodeWire(binary); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCalendarReplayRepairsDatesWithoutRepeatingClientAdd(t *testing.T) {
	ctx, st, u, h, dev, _ := syncFixture(t)
	ctx = context.WithValue(ctx, easVersionKey{}, "16.1")
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	_, err := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, UID: "old-series", HrefName: "old-series.ics", Component: "VEVENT", Data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:old-series\r\nDTSTART:20261106T060000Z\r\nDTEND:20261106T070000Z\r\nRRULE:FREQ=DAILY;UNTIL=20261110T060000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	request := func(key, cmd string) string {
		return `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + cal.ID + `</CollectionId>` + cmd + `</Collection></Collections></Sync>`
	}
	init, err := h.syncCollections(ctx, u, dev, []byte(request("0", "")))
	if err != nil {
		t.Fatal(err)
	}
	body := request(extractTag(init, "SyncKey"), `<Commands><Add><ClientId>ics-import</ClientId><ApplicationData xmlns:c="Calendar:"><c:ClientUid>import-id</c:ClientUid><c:Subject>Imported meeting</c:Subject><c:StartTime>20261106T090000Z</c:StartTime><c:EndTime>20261106T100000Z</c:EndTime></ApplicationData></Add></Commands>`)
	first, err := h.syncCollections(ctx, u, dev, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadCollectionState(ctx, st, u.ID, dev.ID, cal.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforeKey := state.Key
	// Replay a legacy response with both the old date representation and an inverted range.
	state.Response = strings.ReplaceAll(state.Response, "20261110T060000Z", "2026-11-06T00:00:00.000Z")
	if err = saveCollectionState(ctx, st, u.ID, dev.ID, cal.ID, state); err != nil {
		t.Fatal(err)
	}
	repaired, err := h.syncCollections(ctx, u, dev, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(repaired, "2026-11-06T00:00:00.000Z") || extractTag(repaired, "Until") != "20261106T060000Z" || extractTag(repaired, "SyncKey") != beforeKey {
		t.Fatalf("invalid repaired replay: %s", repaired)
	}
	again, err := h.syncCollections(ctx, u, dev, []byte(body))
	if err != nil || again != repaired {
		t.Fatal("repaired replay is not stable", err)
	}
	events, err := st.ListCalendarObjects(ctx, cal.ID)
	if err != nil || len(events) != 2 {
		t.Fatalf("replayed client Add: %d %v", len(events), err)
	}
	if extractTag(first, "SyncKey") != beforeKey {
		t.Fatal("fixture key changed")
	}
}

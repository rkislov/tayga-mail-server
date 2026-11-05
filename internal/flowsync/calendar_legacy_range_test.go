package flowsync

import (
	"context"
	"strings"
	"testing"
)

func TestCalendarLegacyUntilBeforeFirstInstance(t *testing.T) {
	for _, until := range []string{"20231002T000000Z", "20231002T060000Z", "20231006T060000Z"} {
		data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:legacy-range\r\nDTSTART:20231002T060000Z\r\nDTEND:20231002T080000Z\r\nRRULE:FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR;UNTIL=" + until + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
		text, err := calendarRecurrenceXML(context.Background(), data, "")
		if err != nil {
			t.Fatal(err)
		}
		doc, err := parseProtocolXML([]byte("<Event>" + text + "</Event>"))
		if err != nil {
			t.Fatal(err)
		}
		want := until
		if until < "20231002T060000Z" {
			want = "20231002T060000Z"
		}
		if got := doc.find("Until").Text; got != want {
			t.Errorf("until %s: got %s, want %s", until, got, want)
		}
	}
}

func TestCalendarReplayRepairsLegacyRangeWithoutChangingOtherDates(t *testing.T) {
	for _, until := range []string{"20231002T000000Z", "2023-10-02T00:00:00.000Z"} {
		response := `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>retained-key</SyncKey><Commands><Change><ServerId>retained-id</ServerId><ApplicationData><StartTime xmlns="Calendar:">20231002T060000Z</StartTime><EndTime xmlns="Calendar:">20231002T080000Z</EndTime><Recurrence xmlns="Calendar:"><Type>1</Type><Until>` + until + `</Until></Recurrence><Exceptions xmlns="Calendar:"><Exception><ExceptionStartTime>20231002T060000Z</ExceptionStartTime><Deleted>1</Deleted></Exception></Exceptions></ApplicationData></Change></Commands></Collection></Collections></Sync>`
		got, err := repairCalendarDateReplay(response)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := parseProtocolXML([]byte(got))
		if err != nil {
			t.Fatal(err)
		}
		for name, want := range map[string]string{"SyncKey": "retained-key", "ServerId": "retained-id", "Until": "20231002T060000Z", "EndTime": "20231002T080000Z", "ExceptionStartTime": "20231002T060000Z", "Deleted": "1"} {
			if n := doc.find(name); n == nil || n.Text != want {
				t.Errorf("%s not preserved/repaired: %s", name, got)
			}
		}
		again, err := repairCalendarDateReplay(got)
		if err != nil || again != got {
			t.Fatal("repair not idempotent", err)
		}
		binary, err := encodeWire(got)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = decodeWire(binary); err != nil {
			t.Fatal(err)
		}
	}
	other := `<Sync xmlns="AirSync:"><ApplicationData><StartTime xmlns="Tasks:">2023-10-02T06:00:00Z</StartTime><Recurrence xmlns="Tasks:"><Until>2023-10-02T00:00:00Z</Until></Recurrence></ApplicationData></Sync>`
	if got, err := repairCalendarDateReplay(other); err != nil || strings.TrimSpace(got) != other {
		t.Fatal("changed task dates", err)
	}
}

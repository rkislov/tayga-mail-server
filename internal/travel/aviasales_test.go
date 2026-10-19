package travel

import (
	"os"
	"strings"
	"testing"
	"time"
)

func aviasalesFixture(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/aviasales-layout.txt")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAviasalesRoundTripAndPassengers(t *testing.T) {
	trips := ParseAll(aviasalesFixture(t))
	if len(trips) != 4 {
		t.Fatalf("got %d trips, want two segments for each passenger: %+v", len(trips), trips)
	}
	for i, trip := range trips {
		if trip.Passenger == "" || strings.Contains(trip.Passenger, "/") || strings.Contains(trip.Passenger, "000000") {
			t.Fatalf("document leaked into passenger: %q", trip.Passenger)
		}
		if trip.Booking != "TEST01" || !trip.Automatic() {
			t.Fatalf("incomplete trip %d: %+v", i, trip)
		}
		start, end, err := trip.Times()
		if err != nil {
			t.Fatal(err)
		}
		if i%2 == 0 {
			if trip.Number != "SU9001" || trip.Departure != "2026-10-23T19:10" || trip.Arrival != "2026-10-23T23:40" || !strings.Contains(trip.Origin, "Москва") || !strings.Contains(trip.Destination, "Екатеринбург") || start.UTC().Format("15:04") != "16:10" || end.Sub(start) != 150*time.Minute || trip.DepartureZone != "Europe/Moscow" || trip.ArrivalZone != "Asia/Yekaterinburg" {
				t.Fatalf("outbound: %+v", trip)
			}
		} else {
			if trip.Number != "SU9002" || trip.Departure != "2026-10-25T22:00" || trip.Arrival != "2026-10-25T22:40" || !strings.Contains(trip.Origin, "Екатеринбург") || !strings.Contains(trip.Destination, "Москва") || start.UTC().Format("15:04") != "17:00" || end.Sub(start) != 160*time.Minute {
				t.Fatalf("return: %+v", trip)
			}
		}
	}
	if trips[0].Passenger == trips[2].Passenger || trips[0].Identity() == trips[2].Identity() {
		t.Fatal("passengers conflated")
	}
	if trips[0].Identity() == trips[1].Identity() {
		t.Fatal("outbound and return conflated")
	}
	for _, trip := range trips {
		if strings.Contains(trip.Origin, "раньше") || strings.Contains(trip.Destination, "2026") {
			t.Fatal("wrong endpoint column", trip)
		}
	}
}

func TestAviasalesUnknownAirportsAndConflictingDuration(t *testing.T) {
	for _, tc := range []struct{ name, from, to string }{
		{"unknown airport", "(SVX)", "(ZZZ)"},
		{"inconsistent duration", "Полёт 2ч 30мин", "Полёт 3ч 30мин"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trips := ParseAll(strings.ReplaceAll(aviasalesFixture(t), tc.from, tc.to))
			if len(trips) != 4 {
				t.Fatal(len(trips))
			}
			if trips[0].Automatic() || !trips[0].ReviewRequired {
				t.Fatal("uncertain trip added automatically", trips[0])
			}
			if tc.name == "unknown airport" && (trips[0].ArrivalOffset != "" || trips[1].DepartureOffset != "") {
				t.Fatal("timezone guessed from duration/issue date")
			}
		})
	}
}

func TestAviasalesStrictTemplateAndDates(t *testing.T) {
	fixture := aviasalesFixture(t)
	for _, text := range []string{
		strings.ReplaceAll(fixture, "Авиасейлс", "Unknown agency"),
		strings.ReplaceAll(fixture, "ПАССАЖИР / ДОКУМЕНТ", "Unknown person"),
		strings.ReplaceAll(fixture, "(SVO)", ""),
		strings.Split(fixture, "\f")[2], // Payment details alone are not a journey.
	} {
		if trips := ParseAll(text); len(trips) != 0 {
			t.Fatalf("invented trips: %d", len(trips))
		}
	}
	for _, date := range []string{"31 фев 2026", "32 окт 2026", "23 XYZ 2026"} {
		if aviasalesWall(date, "19:10") != "" {
			t.Fatal("invalid date accepted", date)
		}
	}
	for _, date := range []string{"3 окт 2026", "3 октября 2026", "3 окт. 2026"} {
		if got := aviasalesWall(date, "9:10"); got != "2026-10-03T09:10" {
			t.Fatal(date, got)
		}
	}
	trips := ParseAll(strings.ReplaceAll(fixture, "23 окт 2026", "31 фев 2026"))
	if len(trips) != 2 || trips[0].Number != "SU9002" {
		t.Fatal("invalid segment mixed with adjacent dates", trips)
	}
}

func TestAviasalesPageIsolationAndLimit(t *testing.T) {
	page := strings.Split(aviasalesFixture(t), "\f")[0]
	trips := ParseAll(strings.Repeat(page+"\f", 12))
	if len(trips) != 16 {
		t.Fatal("unbounded results", len(trips))
	}
	// A missing passenger on a later page must not inherit the first person's name.
	text := page + "\f" + strings.ReplaceAll(page, "ПАССАЖИР / ДОКУМЕНТ", "Missing passenger")
	if len(ParseAll(text)) != 2 {
		t.Fatal("passenger leaked between pages")
	}
}

func TestAviasalesColumnIndentAndFlightSeparator(t *testing.T) {
	text := strings.ReplaceAll(aviasalesFixture(t), "SU-", "SU ")
	text = "    " + strings.ReplaceAll(text, "\n", "\n    ")
	trips := ParseAll(text)
	if len(trips) != 4 {
		t.Fatalf("indented columns: %d trips", len(trips))
	}
	for _, trip := range trips {
		if !trip.Automatic() {
			t.Fatal("indented table lost fields")
		}
	}
}

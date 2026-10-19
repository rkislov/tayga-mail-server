package travel

import "testing"

func TestTripFields(t *testing.T) {
	for _, tc := range []struct{ text, kind, number, seat, coach string }{
		{"Рейс: SU 1234\nОткуда: Москва (SVO)\nКуда: Сочи (AER)\nВылет: 09.10.2026 23:55\nПрилёт: 10.10.2026 03:10\nМесто: 12A\nPNR: ABC123", "flight", "SU1234", "12A", ""},
		{"Поезд № 042А\nСтанция отправления: Москва\nСтанция прибытия: Казань\nОтправление: 2026-10-09 22:15\nПрибытие: 2026-10-10 10:20\nВагон: 05\nМесто: 21", "train", "042А", "21", "05"},
	} {
		got := Parse(tc.text)
		if got == nil || got.Kind != tc.kind || got.Number != tc.number || got.Seat != tc.seat || got.Coach != tc.coach || got.Departure == "" || got.Arrival == "" {
			t.Fatalf("trip: %+v", got)
		}
	}
}
func TestNoInventedDeparture(t *testing.T) {
	for _, text := range []string{"Flight SU1234\nDeparture: tomorrow", "Рейс SU1234\nВылет: 31.02.2026 25:00", "Рейс SU1234\nМесто: 12A", "Распродажа билетов 09.10.2026 12:00"} {
		if Parse(text) != nil {
			t.Fatal("invented trip", text)
		}
	}
}

func TestSegmentsExplicitOffsetsAndUncertainty(t *testing.T) {
	text := "Passenger: TEST PERSON\nBooking reference: TEST123\nFlight: SU1234\nFrom: Moscow\nTo: Delhi\nDeparture: 2026-10-25 23:30 UTC+3\nArrival: 2026-10-26 07:00 +05:30\nFlight: AI0042\nFrom: Delhi\nTo: London\nDeparture: 2026-10-26 09:00 +05:30\nArrival: 2026-10-26 14:00Z"
	trips := ParseAll(text)
	if len(trips) != 2 {
		t.Fatalf("segments: %+v", trips)
	}
	for _, trip := range trips {
		if !trip.Automatic() || trip.Booking != "TEST123" || trip.Passenger != "TEST PERSON" {
			t.Fatalf("incomplete %+v", trip)
		}
	}
	a, b, err := trips[0].Times()
	if err != nil || a.UTC().Format("15:04") != "20:30" || b.UTC().Format("15:04") != "01:30" {
		t.Fatal(a, b, err)
	}
	for _, zone := range []string{"Europe/Moscow", "MSK", "UTC+15", "UTC+03:99", "UTC+030", ""} {
		v := Parse("Flight: SU1234\nFrom:A\nTo:B\nPassenger:Test\nPNR:Test\nDeparture: 2026-10-25 09:00 " + zone + "\nArrival:2026-10-25 11:00 UTC+3")
		if v == nil || v.Automatic() {
			t.Fatalf("invented zone for %q: %+v", zone, v)
		}
	}
	if ParseAll(text + "\nSchedule cancellation information")[1].Automatic() {
		t.Fatal("cancellation text must require review")
	}
}
func TestTripIdentityAndFingerprint(t *testing.T) {
	x := Parse("Train:042A\nDeparture:2026-10-25 23:00 +03:00\nArrival:2026-10-26 10:00 +03:00\nFrom:Moscow\nTo:Kazan\nPassenger:Test Person\nBooking:ABC123\nSeat:21")
	if x == nil {
		t.Fatal("colon without whitespace")
	}
	y := *x
	y.Source = "ticket.pdf"
	y.Passenger = "TEST  PERSON"
	if x.Identity() != y.Identity() || x.Fingerprint() != y.Fingerprint() {
		t.Fatal("source/body duplicate")
	}
	y.Arrival = "2026-10-26T11:00"
	y.Seat = "22"
	if x.Identity() != y.Identity() || x.Fingerprint() == y.Fingerprint() {
		t.Fatal("revision identity")
	}
	y.Departure = "2026-10-26T23:00"
	if x.Identity() == y.Identity() {
		t.Fatal("different service day conflated")
	}
}

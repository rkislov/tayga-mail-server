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

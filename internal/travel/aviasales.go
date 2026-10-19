package travel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // The release must also work without a host zoneinfo database.
	"unicode/utf8"
)

var (
	aviasalesFlight    = regexp.MustCompile(`\b([A-Z0-9]{2})[- ](\d{2,4})\b`)
	aviasalesClock     = regexp.MustCompile(`\b\d{1,2}:\d{2}\b`)
	aviasalesDate      = regexp.MustCompile(`(?i)\b(\d{1,2})[ \t]+(янв(?:аря)?|фев(?:раля)?|мар(?:та)?|апр(?:еля)?|ма[йя]|июн[ья]?|июл[ья]?|авг(?:уста)?|сен(?:тября)?|окт(?:ября)?|ноя(?:бря)?|дек(?:абря)?)\.?[ \t]+(\d{4})\b`)
	aviasalesBooking   = regexp.MustCompile(`(?im)^\s*Номер брони для регистрации:[ \t]*([A-Z0-9]{5,12})\b`)
	aviasalesPassenger = regexp.MustCompile(`(?im)^\s*ПАССАЖИР[ \t]*/[ \t]*ДОКУМЕНТ[ \t]*\n\s*([^\n/]{2,160})[ \t]*/`)
	aviasalesAirport   = regexp.MustCompile(`\(([A-Z]{3})\)`)
	aviasalesCells     = regexp.MustCompile(`\S(?:.*?\S)?(?:[ \t]{2,}|$)`)
	aviasalesDuration  = regexp.MustCompile(`(?i)Пол[её]т[ \t]+(?:(\d{1,2})[ \t]*ч)?[ \t]*(?:(\d{1,2})[ \t]*мин)?`)
	aviasalesRevision  = regexp.MustCompile(`(?i)отмен[её]н|рейс отмен|изменени[ея] расписания|переоформ`)
)

// Explicit airport mappings only. Airport names/cities, the receipt issue time,
// the browser zone and flight duration must never be used to guess an offset.
// Sources: https://www.svo.aero/ru/departure/timetable and
// https://www.rusaero.aero/airports/SVX (airport handling operator's directory).
var airportZones = map[string]string{
	"SVO": "Europe/Moscow",
	"SVX": "Asia/Yekaterinburg",
}

func airportOffset(code, wall string) (string, string) {
	zone := airportZones[code]
	if zone == "" {
		return "", ""
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return "", ""
	}
	t, err := time.ParseInLocation("2006-01-02T15:04", wall, loc)
	if err != nil || t.Format("2006-01-02T15:04") != wall {
		return "", "" // Reject nonexistent wall times at a DST transition.
	}
	// Reject ambiguous repeated wall times rather than choosing one silently.
	for _, shift := range []time.Duration{-2 * time.Hour, -time.Hour, -30 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour} {
		if t.Add(shift).Format("2006-01-02T15:04") == wall {
			return "", ""
		}
	}
	return t.Format("-07:00"), zone
}

func aviasalesWall(date, clock string) string {
	m := aviasalesDate.FindStringSubmatch(date)
	if len(m) != 4 {
		return ""
	}
	months := map[string]int{"янв": 1, "фев": 2, "мар": 3, "апр": 4, "май": 5, "мая": 5, "июн": 6, "июл": 7, "авг": 8, "сен": 9, "окт": 10, "ноя": 11, "дек": 12}
	monthRunes := []rune(strings.ToLower(m[2]))
	if len(monthRunes) < 3 {
		return ""
	}
	month := months[string(monthRunes[:3])]
	day, _ := strconv.Atoi(m[1])
	return wallTime(fmt.Sprintf("%s-%02d-%02d %s", m[3], month, day, clock))
}

type itineraryCell struct {
	column int
	text   string
}

func itineraryCells(line string) []itineraryCell {
	var cells []itineraryCell
	for _, bounds := range aviasalesCells.FindAllStringIndex(line, -1) {
		cells = append(cells, itineraryCell{utf8.RuneCountInString(line[:bounds[0]]), strings.TrimSpace(line[bounds[0]:bounds[1]])})
	}
	return cells
}

// Keep columns in Unicode character coordinates: byte offsets shift whenever
// Cyrillic city names appear to the left of an otherwise ASCII time column.
func parseAviasales(text string) []*Ticket {
	var trips []*Ticket
	for _, page := range strings.Split(text, "\f") {
		lower := strings.ToLower(page)
		if !strings.Contains(lower, "авиасейлс") || !strings.Contains(lower, "маршрутная квитанция / электронный билет") {
			continue
		}
		passenger := aviasalesPassenger.FindStringSubmatch(page)
		if len(passenger) != 2 {
			continue
		}
		lines := strings.Split(page, "\n")
		for i, line := range lines {
			if len(trips) >= 16 {
				return trips
			}
			clocks := aviasalesClock.FindAllStringIndex(line, -1)
			if len(clocks) != 2 {
				continue
			}
			flight := aviasalesFlight.FindAllStringSubmatch(line[clocks[0][1]:clocks[1][0]], -1)
			if len(flight) != 1 {
				continue
			}
			columns := [2]int{utf8.RuneCountInString(line[:clocks[0][0]]), utf8.RuneCountInString(line[:clocks[1][0]])}
			if columns[0] < 4 || columns[1] <= columns[0]+10 {
				continue
			}
			var dates [2]string
			var places [2][]string
			var codes [2][]string
			block := strings.Join(lines[max(0, i-2):min(len(lines), i+4)], "\n")
			for _, row := range lines[max(0, i-2):min(len(lines), i+4)] {
				for _, cell := range itineraryCells(row) {
					for n, column := range columns {
						if cell.column >= column-2 && cell.column <= column+2 && aviasalesDate.MatchString(cell.text) {
							dates[n] = cell.text
						}
					}
					endpoint := -1
					if cell.column < columns[0]-4 {
						endpoint = 0
					}
					if cell.column > columns[1]+5 {
						endpoint = 1
					}
					if endpoint < 0 || len([]rune(cell.text)) > 100 || aviasalesClock.MatchString(cell.text) || strings.Contains(cell.text, "—") {
						continue
					}
					places[endpoint] = append(places[endpoint], cell.text)
					for _, a := range aviasalesAirport.FindAllStringSubmatch(cell.text, -1) {
						codes[endpoint] = append(codes[endpoint], a[1])
					}
				}
			}
			// Both endpoint columns and both dates must be present. Never join
			// separate segments/pages to fill a partially extracted table.
			if len(codes[0]) != 1 || len(codes[1]) != 1 {
				continue
			}
			trip := &Ticket{Kind: "flight", Number: flight[0][1] + flight[0][2], Passenger: strings.TrimSpace(passenger[1]), Origin: strings.Join(places[0], ", "), Destination: strings.Join(places[1], ", ")}
			trip.Departure = aviasalesWall(dates[0], line[clocks[0][0]:clocks[0][1]])
			trip.Arrival = aviasalesWall(dates[1], line[clocks[1][0]:clocks[1][1]])
			if trip.Departure == "" || trip.Arrival == "" {
				continue
			}
			// Booking belongs to this segment, not the first match on the page.
			for j := i + 1; j < len(lines) && j <= i+12; j++ {
				if len(aviasalesClock.FindAllStringIndex(lines[j], -1)) == 2 {
					break
				}
				if booking := aviasalesBooking.FindStringSubmatch(lines[j]); len(booking) == 2 {
					trip.Booking = booking[1]
					break
				}
			}
			trip.DepartureOffset, trip.DepartureZone = airportOffset(codes[0][0], trip.Departure)
			trip.ArrivalOffset, trip.ArrivalZone = airportOffset(codes[1][0], trip.Arrival)
			trip.ReviewRequired = aviasalesRevision.MatchString(page)
			// Duration is a consistency check only; never solve for a missing
			// airport timezone from it.
			start, end, err := trip.Times()
			duration := aviasalesDuration.FindStringSubmatch(block)
			if err != nil || len(duration) != 3 {
				trip.ReviewRequired = true
			} else {
				hours, _ := strconv.Atoi(duration[1])
				minutes, _ := strconv.Atoi(duration[2])
				if minutes > 59 || end.Sub(start) != time.Duration(hours*60+minutes)*time.Minute {
					trip.ReviewRequired = true
				}
			}
			trips = append(trips, trip)
		}
	}
	return trips
}

package travel

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	aeroflotSubject = regexp.MustCompile(`(?i)ваш посадочный талон для перел[её]та [^()]+\((\d{2}\.\d{2}\.\d{4})\)\s*$`)
	aeroflotBooking = regexp.MustCompile(`(?i)Код бронирования\s*:?\s*([A-Z0-9]{6})\b`)
	aeroflotFlight  = regexp.MustCompile(`\bSU\s*(\d{2,4})\s*[·•]`)
	aeroflotPoint   = regexp.MustCompile(`\b(\d{1,2}:\d{2})\s+(\d{1,2} [А-Яа-яЁё]+)\.?\s+([А-Яа-яЁё][А-Яа-яЁё -]{0,60}?)\s+([A-Z]{3})(?:\s+([A-Z0-9]{1,3}))?\s*,`)
	aeroflotName    = regexp.MustCompile(`^([A-Z][A-Z'-]*(?: [A-Z][A-Z'-]*){1,4})\s+(?:Аэрофлот Бонус\s+[0-9]+\s+)?[А-Яа-яЁё]`)
	aeroflotChanges = regexp.MustCompile(`(?i)рейс отмен[её]н|рейс перенес[её]н|изменени[ея] расписания|переоформ`)
)

// ParseAeroflotBoardingPass reads the check-in card, including copies quoted
// in replies. The subject supplies the year and confirms the departure date.
func ParseAeroflotBoardingPass(subject, body string) *Ticket {
	date := aeroflotSubject.FindStringSubmatch(strings.TrimSpace(subject))
	if date == nil || aeroflotChanges.MatchString(subject) {
		return nil
	}
	day, err := time.Parse("02.01.2006", date[1])
	if err != nil {
		return nil
	}
	text := strings.Join(strings.Fields(body), " ")
	_, card, ok := strings.Cut(text, "Вы прошли онлайн-регистрацию на рейс")
	if !ok || strings.Contains(card, "Вы прошли онлайн-регистрацию на рейс") {
		return nil
	}
	bookingText, route, ok := strings.Cut(card, "О маршруте")
	if !ok || !strings.Contains(route, "Время отправления и прибытия местное") {
		return nil
	}
	route, passengerText, ok := strings.Cut(route, "Данные о пассажирах")
	if !ok {
		return nil
	}
	booking := aeroflotBooking.FindStringSubmatch(bookingText)
	flights := aeroflotFlight.FindAllStringSubmatch(route, 2)
	if booking == nil || len(flights) != 1 {
		return nil
	}
	points := aeroflotPoint.FindAllStringSubmatch(route, 3)
	if len(points) != 2 || points[0][4] == points[1][4] {
		return nil
	}
	from, to := points[0], points[1]
	depart := aviasalesWall(fmt.Sprintf("%s %d", from[2], day.Year()), from[1])
	if !strings.HasPrefix(depart, day.Format("2006-01-02")+"T") {
		return nil
	}
	arrive := aviasalesWall(fmt.Sprintf("%s %d", to[2], day.Year()), to[1])
	if day.Month() == time.December && strings.HasPrefix(arrive, fmt.Sprintf("%d-01-", day.Year())) {
		arrive = aviasalesWall(fmt.Sprintf("%s %d", to[2], day.Year()+1), to[1])
	}
	if arrive == "" {
		return nil
	}
	// Each passenger's row ends with the same airport pair. Keep the flight
	// together as one event and give reordered copies the same identity.
	passengerText, _, _ = strings.Cut(passengerText, "Инструкция перед полетом")
	passengerRoute := regexp.MustCompile(`\b` + from[4] + `\s*[—–-]\s*` + to[4] + `\b`)
	rows := passengerRoute.Split(passengerText, 17)
	if len(rows) < 2 || len(rows) == 17 {
		return nil
	}
	var passengers []string
	for _, row := range rows[:len(rows)-1] {
		name := aeroflotName.FindStringSubmatch(strings.TrimSpace(row))
		if name == nil {
			return nil
		}
		passengers = append(passengers, name[1])
	}
	sort.Strings(passengers)
	ticket := &Ticket{
		Kind: "flight", Number: "SU" + flights[0][1], Booking: strings.ToUpper(booking[1]),
		Origin:      strings.TrimSpace(from[3]) + " (" + from[4] + ")",
		Destination: strings.TrimSpace(to[3]) + " (" + to[4] + ")",
		Departure:   depart, Arrival: arrive, Passenger: strings.Join(passengers, ", "),
	}
	if from[5] != "" {
		ticket.Origin += ", терминал " + from[5]
	}
	ticket.DepartureOffset, ticket.DepartureZone = airportOffset(from[4], depart)
	ticket.ArrivalOffset, ticket.ArrivalZone = airportOffset(to[4], arrive)
	start, end, err := ticket.Times()
	ticket.ReviewRequired = err != nil || end.Sub(start) > 24*time.Hour || aeroflotChanges.MatchString(text)
	return ticket
}

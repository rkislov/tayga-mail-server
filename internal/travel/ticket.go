// Package travel extracts explicit trip fields without inventing times or zones.
package travel

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Ticket struct {
	Kind            string `json:"kind"`
	Number          string `json:"number"`
	Origin          string `json:"origin"`
	Destination     string `json:"destination"`
	Departure       string `json:"departure"` // local wall time, no inferred timezone
	Arrival         string `json:"arrival"`
	Seat            string `json:"seat"`
	Coach           string `json:"coach"`
	Booking         string `json:"booking"`
	Passenger       string `json:"passenger"`
	Source          string `json:"source"`
	DepartureOffset string `json:"departure_offset"` // explicit ISO offset only
	ArrivalOffset   string `json:"arrival_offset"`
	Cancelled       bool   `json:"cancelled"`
	ReviewRequired  bool   `json:"review_required"`
}

func field(text, labels string) string {
	re := regexp.MustCompile(`(?im)(?:^|\n)[\t ]*(?:` + labels + `)(?:[\t ]*[:№#][\t ]*|[\t ]+)([^\n\r]{1,160})`)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

var dateTime = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}|\d{2}[./]\d{2}[./]\d{4})[ T,]+(\d{1,2}:\d{2})`)

func wallTime(value string) string {
	m := dateTime.FindStringSubmatch(value)
	if len(m) < 3 {
		return ""
	}
	for _, layout := range []string{"2006-01-02 15:04", "02.01.2006 15:04", "02/01/2006 15:04"} {
		if t, err := time.Parse(layout, m[1]+" "+m[2]); err == nil {
			return t.Format("2006-01-02T15:04")
		}
	}
	return ""
}

// Parse returns a draft only when a flight/train marker and explicit departure
// date/time are available. Other fields may be missing and need user review.
func Parse(text string) *Ticket {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	t := &Ticket{}
	number := regexp.MustCompile(`(?im)(?:рейс|flight)[\t ]*[:№#]?[\t ]*([A-ZА-Я0-9]{2}[\t ]?\d{2,4})\b`).FindStringSubmatch(text)
	if len(number) > 1 {
		t.Kind = "flight"
		t.Number = strings.ReplaceAll(number[1], " ", "")
	} else {
		number = regexp.MustCompile(`(?im)(?:поезд|train)[\t ]*[:№#]?[\t ]*(\d{1,4}[A-ZА-Я]?)`).FindStringSubmatch(text)
		if len(number) < 2 {
			return nil
		}
		t.Kind = "train"
		t.Number = number[1]
	}
	t.Departure = wallTime(field(text, `отправление|вылет|departure|departure date|дата отправления|дата вылета`))
	if t.Departure == "" {
		return nil
	}
	t.Arrival = wallTime(field(text, `прибытие|прилёт|прилет|arrival|arrival date|дата прибытия`))
	t.Origin = field(text, `откуда|from|origin|станция отправления|аэропорт отправления`)
	t.Destination = field(text, `куда|to|destination|станция прибытия|аэропорт прибытия`)
	t.Seat = field(text, `место|seat`)
	t.Coach = field(text, `вагон|coach|carriage`)
	t.Booking = field(text, `бронь|бронирование|номер бронирования|booking reference|booking|pnr`)
	t.Passenger = field(text, `пассажир|passenger`)
	t.DepartureOffset = explicitOffset(field(text, `отправление|вылет|departure date|departure|дата отправления|дата вылета`))
	t.ArrivalOffset = explicitOffset(field(text, `прибытие|прилёт|прилет|arrival date|arrival|дата прибытия`))
	t.Cancelled = regexp.MustCompile(`(?im)^(?:status|статус)[\t ]*:[\t ]*(?:cancelled|canceled|отмен[её]н)`).MatchString(text)
	t.ReviewRequired = regexp.MustCompile(`(?i)cancel|refund|reissu|reschedul|отмен|возврат|переоформ|изменени[ея] расписания`).MatchString(text)
	return t
}

var segmentMarker = regexp.MustCompile(`(?im)^[\t ]*(?:рейс|flight|поезд|train)[\t ]*[:№#]?[\t ]*[A-ZА-Я0-9]`)
var offsetPattern = regexp.MustCompile(`(?i)(?:UTC|GMT)\s*([+-]\d{1,2}(?::\d{2})?)(?:\s|[)]|$)|\d{1,2}:\d{2}\s*(Z|[+-]\d{2}:\d{2})(?:\s|[)]|$)`)

func explicitOffset(value string) string {
	m := offsetPattern.FindStringSubmatch(value)
	if len(m) == 0 {
		return ""
	}
	v := m[1]
	if v == "" {
		v = m[2]
	}
	if strings.EqualFold(v, "Z") {
		return "+00:00"
	}
	var h, min int
	if strings.Contains(v, ":") {
		_, _ = fmt.Sscanf(v[1:], "%d:%d", &h, &min)
	} else {
		_, _ = fmt.Sscanf(v[1:], "%d", &h)
	}
	if h > 14 || min > 59 || (h == 14 && min != 0) {
		return ""
	}
	return fmt.Sprintf("%s%02d:%02d", v[:1], h, min)
}

// ParseAll treats each explicit flight/train heading as a separate segment.
// Unlabelled PDF tables and scans are intentionally not guessed.
func ParseAll(text string) []*Ticket {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	markers := segmentMarker.FindAllStringIndex(text, 17)
	if len(markers) == 0 {
		if t := Parse(text); t != nil {
			return []*Ticket{t}
		}
		return nil
	}
	var out []*Ticket
	header := text[:markers[0][0]]
	for i, marker := range markers {
		if i == 16 {
			break
		}
		end := len(text)
		if i+1 < len(markers) {
			end = markers[i+1][0]
		}
		if t := Parse(header + text[marker[0]:end]); t != nil {
			out = append(out, t)
		}
	}
	return out
}

// Identity excludes times and seat assignments so a revised itinerary can be
// reviewed against its existing event. A different date/route is a new segment.
func (t Ticket) Identity() string {
	norm := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	date := t.Departure
	if len(date) >= 10 {
		date = date[:10]
	}
	parts := []string{t.Kind, t.Number, date, t.Origin, t.Destination, t.Booking, t.Passenger}
	for i := range parts {
		parts[i] = norm(parts[i])
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(parts, "\x00"))))
}
func (t Ticket) Fingerprint() string {
	t.Source = "" // mail body and its PDF attachment can describe the same trip
	t.Number = strings.ToUpper(t.Number)
	for _, v := range []*string{&t.Origin, &t.Destination, &t.Booking, &t.Passenger, &t.Seat, &t.Coach} {
		*v = strings.ToLower(strings.Join(strings.Fields(*v), " "))
	}
	b, _ := json.Marshal(t)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func (t Ticket) Times() (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01-02T15:04Z07:00", t.Departure+t.DepartureOffset)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("departure timezone required")
	}
	end, err := time.Parse("2006-01-02T15:04Z07:00", t.Arrival+t.ArrivalOffset)
	if err != nil || !end.After(start) || end.Sub(start) > 31*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("valid arrival and timezone required")
	}
	return start, end, nil
}
func (t Ticket) Automatic() bool {
	_, _, err := t.Times()
	return err == nil && !t.Cancelled && !t.ReviewRequired && t.Booking != "" && t.Passenger != "" && t.Origin != "" && t.Destination != ""
}

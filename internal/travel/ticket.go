// Package travel extracts explicit trip fields without inventing times or zones.
package travel

import (
	"regexp"
	"strings"
	"time"
)

type Ticket struct {
	Kind        string `json:"kind"`
	Number      string `json:"number"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Departure   string `json:"departure"` // local wall time, no inferred timezone
	Arrival     string `json:"arrival"`
	Seat        string `json:"seat"`
	Coach       string `json:"coach"`
	Booking     string `json:"booking"`
	Passenger   string `json:"passenger"`
	Source      string `json:"source"`
}

func field(text, labels string) string {
	re := regexp.MustCompile(`(?im)(?:^|\n)[\t ]*(?:` + labels + `)[\t ]*[:№#]?[\t ]+([^\n\r]{1,160})`)
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
	t.Booking = field(text, `бронь|бронирование|номер бронирования|booking|booking reference|pnr`)
	t.Passenger = field(text, `пассажир|passenger`)
	return t
}

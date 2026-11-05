package storage

import (
	"github.com/tayga/tms/internal/calutil"
	"strings"
	"time"
)

// CalendarBusyIntervals expands exceptions using the same identities as the web calendar.
func CalendarBusyIntervals(data string, from, to time.Time) ([]BusyInterval, error) {
	return busyIntervalsExceptOccurrence(data, from, to, "")
}

func busyIntervalsExceptOccurrence(data string, from, to time.Time, rid string) ([]BusyInterval, error) {
	return busyIntervalsForUser(data, from, to, rid, "")
}

func busyIntervalsForUser(data string, from, to time.Time, rid, viewer string) ([]BusyInterval, error) {
	occurrences, err := calutil.ExpandOccurrences(data, from, to)
	if err != nil {
		return nil, err
	}
	out := []BusyInterval{}
	for _, o := range occurrences {
		if o.Transparent || rid != "" && o.RecurrenceID == rid {
			continue
		}
		// RSVP exceptions are per attendee; a declined date remains busy for
		// the organizer and other participants, but not for this viewer.
		if viewer != "" && !strings.EqualFold(o.Organizer.Email, viewer) {
			declined := false
			for _, attendee := range o.Attendees {
				if strings.EqualFold(attendee.Email, viewer) && strings.EqualFold(attendee.PartStat, "DECLINED") {
					declined = true
				}
			}
			if declined {
				continue
			}
		}
		if o.Start != nil && o.End != nil && o.End.After(*o.Start) {
			out = append(out, BusyInterval{Start: o.Start.UTC(), End: o.End.UTC()})
		}
	}
	return out, nil
}

func objectBusyIntervals(data string, from, to time.Time) ([]BusyInterval, error) {
	return CalendarBusyIntervals(data, from, to)
}

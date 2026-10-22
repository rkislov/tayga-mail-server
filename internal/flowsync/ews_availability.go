// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

// Availability only exposes busy intervals, never event subjects or locations.
// Access follows the same domain directory boundary as ResolveNames.
func (h *ewsHandler) availabilityEWS(ctx context.Context, u *storage.User, body string) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	req := doc.find("GetUserAvailabilityRequest")
	if req == nil {
		return ewsFault("ErrorInvalidRequest", "Missing availability request"), nil
	}
	mailboxes, options := req.child("MailboxDataArray"), req.child("FreeBusyViewOptions")
	if mailboxes == nil || len(mailboxes.Children) == 0 || len(mailboxes.Children) > 100 || options == nil {
		return availabilityError("ErrorInvalidRequest", "One to 100 mailboxes and free/busy options required"), nil
	}
	zone, err := parseAvailabilityZone(req.child("TimeZone"))
	if err != nil {
		return availabilityError("ErrorInvalidTimeZone", err.Error()), nil
	}
	window := options.child("TimeWindow")
	if window == nil {
		return availabilityError("ErrorInvalidTimeInterval", "Time window required"), nil
	}
	from, e1 := zone.parse(window.value("StartTime"))
	to, e2 := zone.parse(window.value("EndTime"))
	if e1 != nil || e2 != nil || !to.After(from) || to.Sub(from) > 62*24*time.Hour {
		return availabilityError("ErrorInvalidTimeInterval", "Time window must be positive and at most 62 days"), nil
	}
	interval := 30
	if raw := options.value("MergedFreeBusyIntervalInMinutes"); raw != "" {
		interval, err = strconv.Atoi(raw)
	}
	if err != nil || interval < 5 || interval > 1440 {
		return availabilityError("ErrorInvalidMergedFreeBusyInterval", "Interval must be 5 to 1440 minutes"), nil
	}
	view := options.value("RequestedView")
	switch view {
	case "", "FreeBusy", "Detailed":
		view = "FreeBusy"
	case "FreeBusyMerged", "DetailedMerged":
		view = "FreeBusyMerged"
	case "MergedOnly":
	default:
		return availabilityError("ErrorInvalidRequest", "Unsupported free/busy view"), nil
	}
	allowed, err := h.availabilityUsers(ctx, u)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString(`<m:GetUserAvailabilityResponse` + ewsNamespaces + `><m:FreeBusyResponseArray>`)
	for _, mailbox := range mailboxes.Children {
		address := strings.ToLower(strings.TrimSpace(mailbox.findValue("Address")))
		peer := allowed[address]
		if peer == nil {
			out.WriteString(availabilityMailboxError("ErrorMailRecipientNotFound", "Mailbox unavailable"))
			continue
		}
		busy, e := h.ownedCalendarBusy(ctx, peer, from, to)
		if e != nil {
			out.WriteString(availabilityMailboxError("ErrorFreeBusyGenerationFailed", "Availability unavailable"))
			continue
		}
		step := time.Duration(interval) * time.Minute
		merged := []byte(strings.Repeat("0", int((to.Sub(from)+step-1)/step)))
		var events strings.Builder
		for _, b := range busy {
			start, end := b.Start, b.End
			if start.Before(from) {
				start = from
			}
			if end.After(to) {
				end = to
			}
			if !end.After(start) {
				continue
			}
			for i := int(start.Sub(from) / step); i < len(merged) && from.Add(time.Duration(i)*step).Before(end); i++ {
				merged[i] = '2'
			}
			fmt.Fprintf(&events, `<t:CalendarEvent><t:StartTime>%s</t:StartTime><t:EndTime>%s</t:EndTime><t:BusyType>Busy</t:BusyType></t:CalendarEvent>`, zone.format(start), zone.format(end))
		}
		out.WriteString(`<m:FreeBusyResponse><m:ResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode></m:ResponseMessage><m:FreeBusyView><t:FreeBusyViewType>` + view + `</t:FreeBusyViewType>`)
		if view != "FreeBusy" {
			out.WriteString(`<t:MergedFreeBusy>` + string(merged) + `</t:MergedFreeBusy>`)
		}
		if view != "MergedOnly" {
			out.WriteString(`<t:CalendarEventArray>` + events.String() + `</t:CalendarEventArray>`)
		}
		out.WriteString(`</m:FreeBusyView></m:FreeBusyResponse>`)
	}
	out.WriteString(`</m:FreeBusyResponseArray></m:GetUserAvailabilityResponse>`)
	return out.String(), nil
}

// availabilityUsers is the common directory/privacy boundary for EWS and EAS.
func (h *ewsHandler) availabilityUsers(ctx context.Context, u *storage.User) (map[string]*storage.User, error) {
	allowed := map[string]*storage.User{strings.ToLower(u.Email): u}
	if enabled, e := h.galEnabled(ctx, u); e != nil {
		return nil, e
	} else if enabled {
		users, e := h.store.ListUsersByDomain(ctx, u.DomainID)
		if e != nil {
			return nil, e
		}
		resources, e := h.store.ListCalendarResourcesByDomain(ctx, u.DomainID, true)
		if e != nil {
			return nil, e
		}
		activeResource := map[string]bool{}
		for _, r := range resources {
			activeResource[r.UserID] = true
		}
		for _, peer := range users {
			if peer.Enabled && peer.TenantID == u.TenantID && (peer.AuthSource != "resource" || activeResource[peer.ID]) {
				allowed[strings.ToLower(peer.Email)] = peer
			}
		}
	}
	return allowed, nil
}

func availabilityMailboxError(code, message string) string {
	return `<m:FreeBusyResponse><m:ResponseMessage ResponseClass="Error"><m:MessageText>` + xmlEscape(message) + `</m:MessageText><m:ResponseCode>` + code + `</m:ResponseCode></m:ResponseMessage><m:FreeBusyView><t:FreeBusyViewType>None</t:FreeBusyViewType></m:FreeBusyView></m:FreeBusyResponse>`
}
func availabilityError(code, message string) string {
	return `<m:GetUserAvailabilityResponse` + ewsNamespaces + `><m:FreeBusyResponseArray>` + availabilityMailboxError(code, message) + `</m:FreeBusyResponseArray></m:GetUserAvailabilityResponse>`
}

func (h *ewsHandler) ownedCalendarBusy(ctx context.Context, u *storage.User, from, to time.Time) ([]storage.BusyInterval, error) {
	calendars, err := h.store.ListCalendars(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	var busy []storage.BusyInterval
	for _, cal := range calendars {
		// Shared calendars describe other people's schedules, not this mailbox.
		if cal.UserID != u.ID {
			continue
		}
		objects, e := h.store.ListCalendarObjects(ctx, cal.ID)
		if e != nil {
			return nil, e
		}
		for _, obj := range objects {
			intervals, e := storage.CalendarBusyIntervals(obj.Data, from, to)
			if e != nil {
				return nil, e
			}
			busy = append(busy, intervals...)
			if len(busy) > 10000 {
				return nil, fmt.Errorf("too many events")
			}
		}
	}
	sort.Slice(busy, func(i, j int) bool { return busy[i].Start.Before(busy[j].Start) })
	return busy, nil
}

type availabilityTransition struct{ bias, month, order, weekday, seconds int }
type availabilityZone struct {
	bias               int
	standard, daylight availabilityTransition
}

func parseAvailabilityZone(n *protocolNode) (availabilityZone, error) {
	var z availabilityZone
	if n == nil {
		return z, fmt.Errorf("TimeZone required")
	}
	bias, err := strconv.Atoi(n.value("Bias"))
	if err != nil || bias < -1440 || bias > 1440 {
		return z, fmt.Errorf("invalid timezone bias")
	}
	z.bias = bias
	parse := func(n *protocolNode) (availabilityTransition, error) {
		var tr availabilityTransition
		if n == nil {
			return tr, nil
		}
		var e error
		tr.bias, e = strconv.Atoi(n.value("Bias"))
		if e != nil || tr.bias < -1440 || tr.bias > 1440 {
			return tr, fmt.Errorf("invalid transition bias")
		}
		tr.month, e = strconv.Atoi(n.value("Month"))
		if e != nil || tr.month < 0 || tr.month > 12 {
			return tr, fmt.Errorf("invalid transition month")
		}
		if tr.month == 0 {
			return tr, nil
		}
		tr.order, e = strconv.Atoi(n.value("DayOrder"))
		if e != nil || tr.order < 1 || tr.order > 5 {
			return tr, fmt.Errorf("invalid transition order")
		}
		tr.weekday = -1
		for i, name := range []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"} {
			if n.value("DayOfWeek") == name {
				tr.weekday = i
			}
		}
		t, e := time.Parse("15:04:05", n.value("Time"))
		if e != nil || tr.weekday < 0 {
			return tr, fmt.Errorf("invalid transition time")
		}
		tr.seconds = t.Hour()*3600 + t.Minute()*60 + t.Second()
		return tr, nil
	}
	z.standard, err = parse(n.child("StandardTime"))
	if err != nil {
		return z, err
	}
	z.daylight, err = parse(n.child("DaylightTime"))
	if err != nil {
		return z, err
	}
	if (z.standard.month == 0) != (z.daylight.month == 0) {
		return z, fmt.Errorf("incomplete daylight transitions")
	}
	for _, offset := range []int{z.bias + z.standard.bias, z.bias + z.daylight.bias} {
		if offset <= -1440 || offset >= 1440 {
			return z, fmt.Errorf("invalid combined timezone bias")
		}
	}
	return z, nil
}

func (t availabilityTransition) at(year int) time.Time {
	first := time.Date(year, time.Month(t.month), 1, 0, 0, 0, 0, time.UTC)
	day := 1 + (t.weekday-int(first.Weekday())+7)%7 + 7*(t.order-1)
	last := time.Date(year, time.Month(t.month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		day -= 7
	}
	return time.Date(year, time.Month(t.month), day, 0, 0, 0, 0, time.UTC).Add(time.Duration(t.seconds) * time.Second)
}
func (z availabilityZone) biasAt(utc time.Time) int {
	standard := z.bias + z.standard.bias
	if z.daylight.month == 0 {
		return standard
	}
	year := utc.Add(-time.Duration(standard) * time.Minute).Year()
	start := z.daylight.at(year).Add(time.Duration(standard) * time.Minute)
	end := z.standard.at(year).Add(time.Duration(z.bias+z.daylight.bias) * time.Minute)
	dst := !utc.Before(start) && utc.Before(end)
	if start.After(end) {
		dst = !utc.Before(start) || utc.Before(end)
	}
	if dst {
		return z.bias + z.daylight.bias
	}
	return standard
}
func (z availabilityZone) parse(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	wall, err := time.Parse("2006-01-02T15:04:05", raw)
	if err != nil {
		return time.Time{}, err
	}
	// Try both offsets and reject times in a daylight-saving gap.
	for _, bias := range []int{z.bias + z.standard.bias, z.bias + z.daylight.bias} {
		utc := wall.Add(time.Duration(bias) * time.Minute)
		if z.biasAt(utc) == bias {
			return utc, nil
		}
	}
	return time.Time{}, fmt.Errorf("time falls in a daylight-saving gap")
}
func (z availabilityZone) format(t time.Time) string {
	return t.In(time.FixedZone("", -z.biasAt(t)*60)).Format(time.RFC3339)
}

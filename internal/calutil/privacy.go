package calutil

import (
	"bytes"
	"github.com/emersion/go-ical"
	"strings"
)

// BusyOnlyICS preserves scheduling and recurrence data, removing personal details.
func BusyOnlyICS(data, uid string) (string, error) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return "", err
	}
	cal.Props = ical.Props{}
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, ProdID)
	var children []*ical.Component
	allowed := map[string]bool{"DTSTART": true, "DTEND": true, "DURATION": true, "DTSTAMP": true, "RRULE": true, "RDATE": true, "EXDATE": true, "RECURRENCE-ID": true, "STATUS": true, "TRANSP": true, "SEQUENCE": true}
	for _, child := range cal.Children {
		if child.Name == "VTIMEZONE" {
			children = append(children, child)
			continue
		}
		if child.Name != "VEVENT" {
			continue
		}
		for name := range child.Props {
			if !allowed[name] {
				delete(child.Props, name)
			}
		}
		child.Children = nil
		child.Props.SetText("UID", uid+"@tayga-busy")
		child.Props.SetText("SUMMARY", "Busy")
		children = append(children, child)
	}
	cal.Children = children
	var buf bytes.Buffer
	err = ical.NewEncoder(&buf).Encode(cal)
	return buf.String(), err
}

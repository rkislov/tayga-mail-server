package calutil

import (
	"net/url"
	"strings"

	"github.com/emersion/go-ical"
)

// WithDownloadAttachments personalizes a scheduling payload without rebuilding
// recurrence, timezone, alarm or vendor properties. The caller keeps this copy
// out of the organizer's stored event.
func WithDownloadAttachments(data, base string, attachments []Attachment) (string, error) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return "", err
	}
	origin, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	for _, event := range cal.Children {
		if event.Name != "VEVENT" {
			continue
		}
		var retained []ical.Prop
		for _, prop := range event.Props.Values(ical.PropAttach) {
			u, e := url.Parse(prop.Value)
			managed := e == nil && (u.Host == "" || strings.EqualFold(u.Host, origin.Host)) && (strings.HasPrefix(u.Path, strings.TrimRight(origin.Path, "/")+"/api/v1/calendar/attachments/") || strings.HasPrefix(u.Path, strings.TrimRight(origin.Path, "/")+"/calendar-download/"))
			if !managed {
				retained = append(retained, prop)
			}
		}
		event.Props[ical.PropAttach] = retained
		for _, a := range attachments {
			u, err := url.Parse(a.URI)
			if err != nil {
				return "", err
			}
			p := ical.NewProp(ical.PropAttach)
			p.SetURI(u)
			p.Params.Set("FILENAME", a.Filename)
			if a.ContentType != "" {
				p.Params.Set(ical.ParamFormatType, a.ContentType)
			}
			event.Props.Add(p)
		}
		description, _ := event.Props.Text(ical.PropDescription)
		event.Props.SetText(ical.PropDescription, DownloadDescription(description, attachments))
	}
	return encode(cal)
}

func DownloadDescription(description string, attachments []Attachment) string {
	if len(attachments) == 0 {
		return description
	}
	var b strings.Builder
	b.WriteString(description)
	b.WriteString("\n\nВложения встречи:\n")
	for i, a := range attachments {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(a.Filename)
		b.WriteString("\n")
		b.WriteString(a.URI)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

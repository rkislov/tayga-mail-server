package calutil

import (
	"bytes"
	_ "embed"
	"html/template"
	"strings"
)

//go:embed mail_invitation.gohtml
var invitationHTML string

var invitationTemplate = template.Must(template.New("invitation").Parse(invitationHTML))

type invitationView struct {
	Title, Label, Hint, When, Location, Organizer, Comment string
	Description                                            []string
	Attendees                                              []string
}

func attendeeLabel(a Attendee) string {
	if a.Name != "" && a.Email != "" {
		return a.Name + " · " + a.Email
	}
	if a.Name != "" {
		return a.Name
	}
	return a.Email
}

// Keep the event's timezone (including DST) explicit; never infer the reader's
// timezone from the server. The calendar part retains the original dates.
func invitationWhen(ev Event) string {
	if ev.Start == nil {
		return "Время указано в календарном приглашении"
	}
	start := *ev.Start
	if ev.AllDay {
		label := start.Format("02.01.2006")
		if ev.End != nil {
			last := ev.End.AddDate(0, 0, -1) // RFC 5545 exclusive end date
			if last.After(start) {
				label += " — " + last.Format("02.01.2006")
			}
		}
		return label + " · весь день"
	}
	label := start.Format("02.01.2006 · 15:04")
	if ev.End != nil {
		end := ev.End.In(start.Location())
		if start.Format("2006-01-02") == end.Format("2006-01-02") {
			label += " — " + end.Format("15:04")
		} else {
			label += " — " + end.Format("02.01.2006 · 15:04")
		}
		_, startOffset := start.Zone()
		_, endOffset := end.Zone()
		if startOffset != endOffset {
			return start.Format("02.01.2006 · 15:04 (UTC-07:00)") + " — " + end.Format("02.01.2006 · 15:04 (UTC-07:00)")
		}
	}
	return label + " (" + start.Format("UTC-07:00") + ")"
}

func renderInvitationHTML(ics, method, subject, fallback string) string {
	p, _ := ParseICS(ics)
	v := invitationView{Title: p.Summary, Label: "Приглашение на встречу", When: invitationWhen(p.Event), Location: p.Location, Organizer: attendeeLabel(p.Organizer), Comment: p.Comment,
		Hint: "Примите или отклоните приглашение с помощью кнопок вашего почтового приложения. В календаре время отобразится в вашем часовом поясе."}
	if v.Title == "" {
		v.Title = subject
	}
	switch strings.ToUpper(method) {
	case "CANCEL":
		v.Label, v.Hint = "Встреча отменена", "Организатор отменил встречу. Календарное уведомление содержит обновление для вашего календаря."
	case "REPLY":
		v.Label, v.Hint = "Ответ на приглашение", "Ответ участника сохранён в календарном уведомлении."
		if len(p.Attendees) == 1 {
			switch p.Attendees[0].PartStat {
			case "ACCEPTED":
				v.Label = "Приглашение принято"
			case "DECLINED":
				v.Label = "Приглашение отклонено"
			case "TENTATIVE":
				v.Label = "Участие под вопросом"
			}
		}
	case "COUNTER":
		v.Label, v.Hint = "Предложено другое время", "Участник предлагает изменить время встречи."
	case "DECLINECOUNTER":
		v.Label, v.Hint = "Предложение отклонено", "Организатор отклонил предложенное изменение встречи."
	}
	for _, a := range p.Attendees {
		if !strings.EqualFold(a.Email, p.Organizer.Email) {
			v.Attendees = append(v.Attendees, attendeeLabel(a))
		}
	}
	description := p.Description
	if p.UID == "" {
		description = fallback
	}
	if description != "" {
		v.Description = strings.Split(strings.ReplaceAll(description, "\r\n", "\n"), "\n")
	}
	var out bytes.Buffer
	if err := invitationTemplate.Execute(&out, v); err != nil {
		return ""
	}
	return out.String()
}

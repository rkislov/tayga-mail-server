package calutil

import (
	"bytes"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInvitationHTMLMIMEPreservesCalendarAndEscapesContent(t *testing.T) {
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nUID:flight-preview\r\nSEQUENCE:4\r\nSUMMARY:Рейс SU1418 · Москва → Екатеринбург\r\nDTSTART;TZID=Europe/Moscow:20261023T191000\r\nDTEND;TZID=Europe/Moscow:20261023T214000\r\nLOCATION:Шереметьево · терминал B\r\nORGANIZER;CN=Роман Кислов:mailto:roman@example.com\r\nATTENDEE;CN=Юлия;PARTSTAT=NEEDS-ACTION:mailto:juliya@example.com\r\nDESCRIPTION:Встречаемся у стойки регистрации.\\nПожалуйста\\, возьмите документы.\r\nATTACH;FILENAME=ticket.pdf:https://example.com/private/ticket.pdf\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	raw := IMIPMessage("roman@example.com", "Роман", "juliya@example.com", "Приглашение", "Вы приглашены", ics, "REQUEST", "stable-id", time.Unix(100, 0))
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	_, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	r := multipart.NewReader(m.Body, params["boundary"])
	types := []string{}
	for {
		part, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		kind, p, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		types = append(types, kind)
		switch kind {
		case "text/plain":
			if string(body) != "Вы приглашены" {
				t.Fatal(string(body))
			}
		case "text/html":
			for _, want := range []string{"Рейс SU1418", "19:10 — 21:40", "UTC+03:00", "Шереметьево", "Роман Кислов", "juliya@example.com", "Примите или отклоните", "Встречаемся у стойки"} {
				if !strings.Contains(html.UnescapeString(string(body)), want) {
					t.Fatal("HTML missing", want, string(body))
				}
			}
			if strings.Contains(string(body), "private/ticket.pdf") {
				t.Fatal("private download URL exposed as a public invitation link")
			}
			if path := os.Getenv("TAYGA_INVITATION_PREVIEW"); path != "" {
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
		case "text/calendar":
			if p["method"] != "REQUEST" || strings.TrimSpace(string(body)) != strings.TrimSpace(ics) {
				t.Fatal("calendar changed", string(body))
			}
		}
	}
	if strings.Join(types, ",") != "text/plain,text/html,text/calendar" {
		t.Fatal(types)
	}
	if !bytes.Equal(raw, IMIPMessage("roman@example.com", "Роман", "juliya@example.com", "Приглашение", "Вы приглашены", ics, "REQUEST", "stable-id", time.Unix(100, 0))) {
		t.Fatal("snapshot changed")
	}
	malicious := strings.ReplaceAll(ics, "Рейс SU1418", `<img src=x onerror=alert(1)>`)
	malicious = strings.ReplaceAll(malicious, "Шереметьево", `<script>alert(1)</script>`)
	html := renderInvitationHTML(malicious, "REQUEST", "", "")
	if strings.Contains(html, "<img") || strings.Contains(html, "<script") || !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("unescaped event content", html)
	}
}

func TestInvitationStatusAndAllDay(t *testing.T) {
	base := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:all-day\r\nSUMMARY:Встреча\r\nDTSTART;VALUE=DATE:20261023\r\nDTEND;VALUE=DATE:20261024\r\nATTENDEE;PARTSTAT=ACCEPTED:mailto:guest@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	for method, title := range map[string]string{"REQUEST": "Приглашение на встречу", "REPLY": "Приглашение принято", "CANCEL": "Встреча отменена", "COUNTER": "Предложено другое время", "DECLINECOUNTER": "Предложение отклонено"} {
		html := renderInvitationHTML(base, method, "", "")
		if !strings.Contains(html, title) || !strings.Contains(html, "23.10.2026 · весь день") || strings.Contains(html, "24.10.2026") {
			t.Fatal(method, html)
		}
		if method != "REQUEST" && strings.Contains(html, "Примите или отклоните") {
			t.Fatal("wrong action for method", method)
		}
	}
	multiple := strings.Replace(base, "DTEND;VALUE=DATE:20261024", "DTEND;VALUE=DATE:20261026", 1)
	if !strings.Contains(renderInvitationHTML(multiple, "REQUEST", "", ""), "23.10.2026 — 25.10.2026 · весь день") {
		t.Fatal("exclusive end day rendered incorrectly")
	}
}

package httpapi

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMailPreview(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message parsedMsg
		want    string
	}{
		{"plain whitespace", parsedMsg{Text: "Анна,\n\t привет!  Обсудим проект."}, "Анна, привет! Обсудим проект."},
		{"html fallback", parsedMsg{HTML: "<p>Привет &amp; спасибо!</p><p>До встречи.</p>"}, "Привет & спасибо! До встречи."},
		{"prefer plain", parsedMsg{Text: "Plain", HTML: "<p>HTML</p>"}, "Plain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mailPreview(tc.message); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	got := mailPreview(parsedMsg{Text: strings.Repeat("я", 200)})
	if !utf8.ValidString(got) || len([]rune(got)) != 181 || !strings.HasSuffix(got, "…") {
		t.Fatalf("invalid unicode truncation: %q", got)
	}
}

package avatar

import (
	"bytes"
	"image/png"
	"testing"
)

func TestInitialsUnicodeAndFallback(t *testing.T) {
	for _, tt := range []struct{ name, login, want string }{
		{"Роман Кислов", "roman", "РК"}, {"  Алиса   Тестовая ", "alice", "АТ"},
		{"Élodie Durand", "elodie", "ÉD"}, {"", "roman@example.test", "R"},
		{"李 明", "li", "李明"}, {"  ", "", "?"},
	} {
		if got := Initials(tt.name, tt.login); got != tt.want {
			t.Fatalf("initials %q: %q, want %q", tt.name, got, tt.want)
		}
	}
}
func TestInitialsPNG(t *testing.T) {
	raw, err := InitialsPNG("Роман Кислов", "roman")
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 128 || img.Bounds().Dy() != 128 {
		t.Fatal(img.Bounds())
	}
	_, _, _, alpha := img.At(0, 0).RGBA()
	if alpha != 0 {
		t.Fatal("avatar corners must be transparent")
	}
	other, err := InitialsPNG("Алиса Тестовая", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(raw, other) {
		t.Fatal("different Cyrillic initials rendered identically")
	}
}

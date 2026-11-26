package avatar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Initials uses Unicode letters, with the login as fallback for an empty name.
func Initials(name, login string) string {
	if strings.TrimSpace(name) == "" {
		name = login
	}
	if local, _, ok := strings.Cut(name, "@"); ok {
		name = local
	}
	parts := strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	letters := []rune{}
	for _, part := range parts {
		letters = append(letters, unicode.ToUpper([]rune(part)[0]))
		if len(letters) == 2 {
			break
		}
	}
	if len(letters) == 0 {
		return "?"
	}
	return string(letters)
}

var initialsFont = sync.OnceValues(func() (*opentype.Font, error) { return opentype.Parse(gobold.TTF) })

// InitialsPNG works in native Mattermost clients as well as the web products.
// It does not store a generated image as the user's uploaded profile photo.
func InitialsPNG(name, login string) ([]byte, error) {
	const size = 128
	parsed, err := initialsFont()
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: 47, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	defer face.Close()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			distance := math.Hypot(float64(x)+.5-size/2, float64(y)+.5-size/2)
			alpha := uint8(255 * math.Max(0, math.Min(1, size/2-distance)))
			img.SetNRGBA(x, y, color.NRGBA{R: 223, G: 231, B: 214, A: alpha})
		}
	}
	label := Initials(name, login)
	bounds, _ := font.BoundString(face, label)
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y
	d := font.Drawer{Dst: img, Src: image.NewUniform(color.NRGBA{R: 63, G: 90, B: 48, A: 255}), Face: face,
		Dot: fixed.Point26_6{X: fixed.I(size/2) - width/2 - bounds.Min.X, Y: fixed.I(size/2) - height/2 - bounds.Min.Y}}
	d.DrawString(label)
	var result bytes.Buffer
	err = png.Encode(&result, img)
	return result.Bytes(), err
}

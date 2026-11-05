package httpapi

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image/png"
	"io"
	"strings"
	"testing"
)

func motionPatchPart(t *testing.T, data []byte, part string, edit func([]byte) []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, entry := range reader.File {
		r, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name == part {
			body = edit(body)
		}
		w, err := writer.Create(entry.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func motionFocusSizeFixture(t *testing.T, property, operands string, joint bool) []byte {
	timing := motionSingleBehavior(`<p:anim valueType="num" `+operands+`>COMMON</p:anim>`, property)
	if joint {
		timing = strings.Replace(motionJointTiming("rgb"), `fill.angle`, property, 1)
		timing = strings.Replace(timing, `from="0" to="90"`, operands, 1)
	}
	data := motionBehaviorFixture(t, motionRichColorFixture(t), timing)
	return motionPatchPart(t, data, "ppt/slides/slide1.xml", func(body []byte) []byte {
		return bytes.Replace(body, []byte(`<a:lin ang="0" scaled="1"/>`), []byte(`<a:path path="rect"><a:fillToRect l="40000" t="40000" r="40000" b="40000"/></a:path>`), 1)
	})
}

func motionStopsFixture(t *testing.T, ext, operands string, joint bool) []byte {
	if ext == "odp" {
		return []byte(motionODPGradientFixture(t, "tayga:gradient-stops", operands, "linear", joint))
	}
	timing := motionSingleBehavior(`<p:anim valueType="str" `+operands+`>COMMON</p:anim>`, "tayga:gradient-stops")
	if joint {
		timing = strings.Replace(motionJointTiming("rgb"), "fill.angle", "tayga:gradient-stops", 1)
		timing = strings.Replace(timing, `valueType="num" from="0" to="90"`, `valueType="str" `+operands, 1)
	}
	return motionBehaviorFixture(t, motionRichColorFixture(t), timing)
}

func TestMotionFocusSize(t *testing.T) {
	for _, axis := range []string{"x", "y"} {
		for _, joint := range []bool{false, true} {
			data := motionFocusSizeFixture(t, "fill.focussize."+axis, `from="0.2" by="0.4"`, joint)
			deck, parts := motionBuildFixture(t, "size.pptx", string(data))
			slide := deck.Slides[0]
			if slide.Warning != "" || len(slide.Colors) != 1 || (slide.Colors[0].GeometryEffect != nil) != joint {
				t.Fatal(deck)
			}
			body := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.Colors[0].ToWhite)]
			if !strings.Contains(body, "blipFill") || slide.Effects[0].Property != map[string]string{"x": "focus-width", "y": "focus-height"}[axis] {
				t.Fatal("focal size not materialized", body)
			}
			imageName := fmt.Sprintf("ppt/media/taygaMotionFocus%d_0.png", slide.Colors[0].ToWhite)
			img, err := png.Decode(strings.NewReader(parts[imageName]))
			if err != nil || img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
				t.Fatal("invalid bounded focus paint", err)
			}
			fromName := fmt.Sprintf("ppt/media/taygaMotionFocus%d_0.png", slide.Colors[0].FromWhite)
			if parts[fromName] == parts[imageName] {
				t.Fatal("resizing focus did not change paint")
			}

			if !strings.Contains(body, "TEXT") || !strings.Contains(body, `w="80000"`) {
				t.Fatal("text or contour changed", body)
			}
		}
	}
	for _, operands := range []string{`to="-0.1"`, `to="1.1"`, `from="0.4" to="0.6"`, `to="NaN"`, `by="-0.3"`} {
		deck, _ := motionBuildFixture(t, "size.pptx", string(motionFocusSizeFixture(t, "fill.focussize.x", operands, false)))
		if deck.Slides[0].Warning == "" {
			t.Fatal("invalid focal size accepted", operands)
		}
	}
	data := motionFocusSizeFixture(t, "fill.focussize.x", `to="0.6"`, false)
	data = motionPatchPart(t, data, "ppt/slides/slide1.xml", func(body []byte) []byte {
		return bytes.Replace(body, []byte(`l="40000" t="40000" r="40000"`), []byte(`l="10000" t="40000" r="70000"`), 1)
	})
	deck, _ := motionBuildFixture(t, "size.pptx", string(data))
	if deck.Slides[0].Warning == "" {
		t.Fatal("resized focal rectangle escaped fill bounds")
	}
}

func TestMotionGradientStops(t *testing.T) {
	for _, ext := range []string{"pptx", "odp"} {
		for _, joint := range []bool{false, true} {
			operands := `from="0,1" to="0.2,0.8" by="invalid"`
			if ext == "odp" {
				operands = `smil:from="0,1" smil:to="0.2,0.8" smil:by="invalid"`
			}
			deck, parts := motionBuildFixture(t, "stops."+ext, string(motionStopsFixture(t, ext, operands, joint)))
			slide := deck.Slides[0]
			if slide.Warning != "" || slide.Effects[0].Property != "stops" || len(slide.Effects[0].From) != 2 || len(slide.Colors) != 1 || (slide.Colors[0].GeometryEffect != nil) != joint || deck.Frames > 120 {
				t.Fatal(deck)
			}
			if ext == "pptx" {
				body := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.Colors[0].ToWhite)]
				if !strings.Contains(body, `pos="20000"`) || !strings.Contains(body, `pos="80000"`) || !strings.Contains(body, "TEXT") {
					t.Fatal(body)
				}
			} else if !strings.Contains(parts["styles.xml"], `svg:offset="0.200000"`) || !strings.Contains(parts["styles.xml"], `svg:offset="0.800000"`) || !strings.Contains(parts["styles.xml"], `draw:name="G"`) {
				t.Fatal(parts)
			}
		}
		for _, target := range []string{"0.8,0.2", "-0.1,0.9", "0,NaN", "0,0.5,1", "0.2", strings.Repeat("0,", 16) + "1"} {
			operands := `to="` + target + `"`
			if ext == "odp" {
				operands = `smil:to="` + target + `"`
			}
			deck, _ := motionBuildFixture(t, "bad."+ext, string(motionStopsFixture(t, ext, operands, false)))
			if deck.Slides[0].Warning == "" {
				t.Fatal("invalid positions accepted", ext, target)
			}
		}
	}
	// An interior stop moves with both endpoints, without changing its color.
	data := motionStopsFixture(t, "pptx", `by="0.1,0.1,-0.1"`, false)
	data = motionPatchPart(t, data, "ppt/slides/slide1.xml", func(body []byte) []byte {
		return bytes.Replace(body, []byte(`<a:gs pos="100000">`), []byte(`<a:gs pos="50000"><a:srgbClr val="00FF00"/></a:gs><a:gs pos="100000">`), 1)
	})
	deck, parts := motionBuildFixture(t, "three.pptx", string(data))
	slide := deck.Slides[0]
	if slide.Warning != "" {
		t.Fatal(deck)
	}
	body := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.Colors[0].ToWhite)]
	if !strings.Contains(body, `pos="60000"`) || !strings.Contains(body, `val="00FF00"`) {
		t.Fatal(body)
	}
}

func TestMotionFocusPaintGeometry(t *testing.T) {
	palette := [][]float64{{255, 0, 0}, {0, 0, 255}}
	for _, path := range []string{"rect", "circle"} {
		small := motionFocusPixels([4]float64{.4, .4, .4, .4}, path, []float64{0, 1}, palette)
		large := motionFocusPixels([4]float64{.2, .4, .2, .4}, path, []float64{0, 1}, palette)
		a, b := small.NRGBAAt(190, 128), large.NRGBAAt(190, 128)
		if int(b.R) <= int(a.R)+30 || int(b.B) >= int(a.B)-30 || small.NRGBAAt(128, 128) != large.NRGBAAt(128, 128) {
			t.Fatal("focus expansion moved center or failed to expand inner color", path, a, b)
		}
	}
}

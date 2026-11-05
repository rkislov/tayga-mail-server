package httpapi

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strings"
	"testing"
)

func motionODPMultiFixture(t *testing.T, extra string, joint bool) []byte {
	data := []byte(motionODPGradientFixture(t, "tayga:gradient-focus-width", `smil:from="0.2" smil:to="0.6"`, "rectangular", joint))
	data = motionPatchPart(t, data, "styles.xml", func(body []byte) []byte {
		return bytes.Replace(body, []byte(`draw:cy="50%"`), []byte(`draw:cy="50%" xmlns:tg="urn:tayga:gradient:1.0" tg:focus-width="0.2" tg:focus-height="0.2"`), 1)
	})
	if extra != "" {
		data = motionPatchPart(t, data, "content.xml", func(body []byte) []byte {
			return bytes.Replace(body, []byte(`</anim:seq>`), []byte(extra+`</anim:seq>`), 1)
		})
	}
	return data
}

func motionODPExtraGeometry(property, operands string) string {
	return `<anim:par presentation:node-type="with-previous" presentation:preset-class="emphasis" smil:begin="0.25s"><anim:animate smil:attributeName="` + property + `" smil:targetElement="shape" smil:dur="2s" ` + operands + `/></anim:par>`
}

func motionPPTXMultiFixture(t *testing.T, extra string, joint bool) []byte {
	data := motionFocusSizeFixture(t, "fill.focussize.x", `from="0.2" to="0.6"`, joint)
	return motionPatchPart(t, data, "ppt/slides/slide1.xml", func(body []byte) []byte {
		return bytes.Replace(body, []byte(`</p:childTnLst></p:cTn></p:seq>`), []byte(extra+`</p:childTnLst></p:cTn></p:seq>`), 1)
	})
}

func motionPPTXExtraGeometry(property, operands, valueType string) string {
	timing := motionSingleBehavior(`<p:anim valueType="`+valueType+`" `+operands+`>COMMON</p:anim>`, property)
	start := strings.Index(timing, `<p:par><p:cTn presetClass=`)
	end := start + strings.Index(timing[start:], `</p:par>`) + len(`</p:par>`)
	return strings.Replace(timing[start:end], "clickEffect", "withEffect", 1)
}

func TestMotionODPFocusSize(t *testing.T) {
	for _, joint := range []bool{false, true} {
		deck, parts := motionBuildFixture(t, "focus.odp", string(motionODPMultiFixture(t, "", joint)))
		if deck.Slides[0].Warning != "" || len(deck.Slides[0].Colors) != 1 {
			t.Fatal(deck)
		}
		styles := parts["styles.xml"]
		if !strings.Contains(styles, "draw:fill-image") || !strings.Contains(parts["content.xml"], `draw:fill="bitmap"`) || !strings.Contains(parts["content.xml"], "TEXT") {
			t.Fatal("focus bitmap or text missing")
		}
		images := []string{}
		for _, part := range strings.Split(styles, "<office:binary-data")[1:] {
			encoded := strings.SplitN(strings.SplitN(part, ">", 2)[1], "</office:binary-data>", 2)[0]
			pngData, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				t.Fatal(err)
			}
			image, err := png.Decode(bytes.NewReader(pngData))
			if err != nil || image.Bounds().Dx() != 256 || image.Bounds().Dy() != 256 {
				t.Fatal(err)
			}
			images = append(images, string(pngData))
		}
		if len(images) < 2 || images[0] == images[len(images)-1] {
			t.Fatal("focus extent did not change pixels")
		}
		if !strings.Contains(styles, `draw:name="G"`) || !strings.Contains(styles, `tg:focus-width="0.2"`) {
			t.Fatal("shared original gradient was modified")
		}
	}
}

func TestMotionMultipleGeometry(t *testing.T) {
	for _, ext := range []string{"pptx", "odp"} {
		for _, joint := range []bool{false, true} {
			var data []byte
			if ext == "pptx" {
				data = motionPPTXMultiFixture(t, motionPPTXExtraGeometry("fill.focussize.y", `to="0.5"`, "num"), joint)
			} else {
				data = motionODPMultiFixture(t, motionODPExtraGeometry("tayga:gradient-focus-height", `smil:to="0.5"`), joint)
			}
			deck, _ := motionBuildFixture(t, "multi."+ext, string(data))
			slide := deck.Slides[0]
			if slide.Warning != "" || len(slide.Colors) != 1 || len(slide.Colors[0].Axes) != len(slide.Effects) || deck.Frames > 120 {
				t.Fatal(deck)
			}
			paint := slide.Colors[0]
			if len(paint.Samples) > 45 || len(paint.Samples) < 9 {
				t.Fatal("invalid grid budget", deck)
			}
			first, last := paint.Samples[0], paint.Samples[len(paint.Samples)-1]
			for i := range paint.Axes {
				if first.Positions[i] != 0 || last.Positions[i] != 1 {
					t.Fatal("missing grid endpoints")
				}
			}
			// Opposite corners require independent phases, not a single shared clock.
			found := false
			for _, s := range paint.Samples {
				if s.Positions[len(s.Positions)-1] == 1 && s.Positions[len(s.Positions)-2] == 0 {
					found = true
				}
			}
			if !found {
				t.Fatal("independent geometry corners missing")
			}
		}
	}
}

func TestMotionGeometryGroupLimits(t *testing.T) {
	for _, extra := range []string{
		motionODPExtraGeometry("tayga:gradient-focus-width", `smil:to="0.4"`),
		motionODPExtraGeometry("tayga:gradient-center-x", `smil:to="0.8"`),
		motionODPExtraGeometry("tayga:gradient-focus-height", `smil:to="0.4"`) + motionODPExtraGeometry("tayga:gradient-center-x", `smil:to="0.55"`) + motionODPExtraGeometry("tayga:gradient-stops", `smil:to="0.1,0.9"`),
	} {
		deck, _ := motionBuildFixture(t, "bad.odp", string(motionODPMultiFixture(t, extra, false)))
		if deck.Slides[0].Warning == "" || len(deck.Slides[0].Effects) != 0 {
			t.Fatal("duplicate, escaped or excessive geometry accepted", deck)
		}
	}
	data := motionODPMultiFixture(t, "", false)
	data = motionPatchPart(t, data, "styles.xml", func(body []byte) []byte {
		return bytes.Replace(body, []byte(`tg:focus-width="0.2"`), []byte(`tg:focus-width="NaN"`), 1)
	})
	deck, _ := motionBuildFixture(t, "bad.odp", string(data))
	if deck.Slides[0].Warning == "" {
		t.Fatal("invalid initial focal size accepted")
	}
}

func TestMotionThreeGeometryAndColor(t *testing.T) {
	extra := motionODPExtraGeometry("tayga:gradient-focus-height", `smil:to="0.5"`) + motionODPExtraGeometry("tayga:gradient-stops", `smil:to="0.1,0.9"`)
	data := motionODPMultiFixture(t, extra, true)
	deck, _ := motionBuildFixture(t, "four-axes.odp", string(data))
	if deck.Slides[0].Warning != "" || len(deck.Slides[0].Colors[0].Axes) != 4 || len(deck.Slides[0].Colors[0].Samples) != 36 {
		t.Fatal(deck)
	}
	// Three geometry axes together with nine HSL phases exceed the bounded
	// grid. Refuse explicitly instead of collapsing the color cycle.
	data = motionPatchPart(t, data, "content.xml", func(body []byte) []byte {
		return bytes.Replace(body, []byte(`smil:calcMode="linear"`), []byte(`smil:calcMode="linear" anim:color-interpolation="hsl"`), 1)
	})
	deck, _ = motionBuildFixture(t, "too-many.odp", string(data))
	if deck.Slides[0].Warning == "" {
		t.Fatal("cyclic axes exceeded paint budget silently")
	}
}

func motionAngleStopsFixture(t *testing.T, ext string) []byte {
	if ext == "odp" {
		data := []byte(motionODPGradientFixture(t, "tayga:gradient-angle", `smil:from="0" smil:to="90"`, "linear", false))
		return motionPatchPart(t, data, "content.xml", func(body []byte) []byte {
			return bytes.Replace(body, []byte(`</anim:seq>`), []byte(motionODPExtraGeometry("tayga:gradient-stops", `smil:to="0.2,0.8"`)+`</anim:seq>`), 1)
		})
	}
	timing := motionSingleBehavior(`<p:anim valueType="num" from="0" to="90">COMMON</p:anim>`, "fill.angle")
	timing = strings.Replace(timing, `</p:childTnLst></p:cTn></p:seq>`, motionPPTXExtraGeometry("tayga:gradient-stops", `to="0.2,0.8"`, "str")+`</p:childTnLst></p:cTn></p:seq>`, 1)
	return motionBehaviorFixture(t, motionRichColorFixture(t), timing)
}

func TestMotionMultipleAngleAndStops(t *testing.T) {
	for _, ext := range []string{"pptx", "odp"} {
		deck, parts := motionBuildFixture(t, "combined."+ext, string(motionAngleStopsFixture(t, ext)))
		if deck.Slides[0].Warning != "" || len(deck.Slides[0].Colors) != 1 || len(deck.Slides[0].Colors[0].Axes) != 2 {
			t.Fatal(deck)
		}
		if ext == "odp" && (!strings.Contains(parts["styles.xml"], `draw:angle="90.000000deg"`) || !strings.Contains(parts["styles.xml"], `svg:offset="0.200000"`)) {
			t.Fatal("combined angle/stops lost in ODP definitions")
		}
	}
}

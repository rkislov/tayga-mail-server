package httpapi

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func motionPPTXStage(behavior, property, trigger string) string {
	timing := motionSingleBehavior(behavior, property)
	start := strings.Index(timing, `<p:par><p:cTn presetClass=`)
	end := start + strings.Index(timing[start:], `</p:par>`) + len(`</p:par>`)
	return strings.Replace(timing[start:end], "clickEffect", trigger, 1)
}

func motionColorStageFixture(t *testing.T, ext string, reverse bool) []byte {
	start := "00FF00"
	if reverse {
		start = "FF0000"
	}
	if ext == "odp" {
		data := []byte(motionODPGradientFixture(t, "tayga:gradient-angle", `smil:to="90"`, "linear", false))
		data = motionPatchPart(t, data, "content.xml", func(body []byte) []byte {
			a := bytes.Index(body, []byte(`<anim:par><anim:seq>`))
			b := bytes.Index(body[a:], []byte(`</anim:seq></anim:par>`)) + a + len(`</anim:seq></anim:par>`)
			loop := `smil:repeatCount="3"`
			if reverse {
				loop += ` smil:autoReverse="true"`
			}
			stage := func(trigger, to, from, extra string) string {
				return `<anim:par presentation:node-type="` + trigger + `" presentation:preset-class="emphasis"><anim:animateColor smil:attributeName="fill-color" smil:targetElement="shape" smil:dur="1s" smil:calcMode="linear" smil:to="#` + to + `" ` + from + ` ` + extra + `/></anim:par>`
			}
			timing := `<anim:par><anim:seq>` + stage("on-click", "00FF00", "", loop) + stage("after-previous", "0000FF", `smil:from="#`+start+`"`, "") + `</anim:seq></anim:par>`
			body = append(append(append([]byte{}, body[:a]...), []byte(timing)...), body[b:]...)
			body = bytes.Replace(body, []byte(`draw:fill="gradient" draw:fill-gradient-name="G"`), []byte(`draw:fill="solid" draw:fill-color="#ff0000"`), 1)
			return body
		})
		return data
	}
	timing := motionSingleBehavior(`<p:animClr clrSpc="rgb">COMMON<p:to><a:srgbClr val="00FF00"/></p:to></p:animClr>`, "fillcolor")
	loop := `dur="1000" repeatCount="3000"`
	if reverse {
		loop += ` autoRev="1"`
	}
	timing = strings.Replace(timing, `dur="1000"`, loop, 1)
	next := motionPPTXStage(`<p:animClr clrSpc="rgb">COMMON<p:from><a:srgbClr val="`+start+`"/></p:from><p:to><a:srgbClr val="0000FF"/></p:to></p:animClr>`, "fillcolor", "afterEffect")
	timing = strings.Replace(timing, `</p:childTnLst></p:cTn></p:seq>`, next+`</p:childTnLst></p:cTn></p:seq>`, 1)
	return motionBehaviorFixture(t, motionTransformFixture(t), timing)
}

func TestMotionColorStagesAndRepeats(t *testing.T) {
	for _, ext := range []string{"pptx", "odp"} {
		for _, reverse := range []bool{false, true} {
			deck, parts := motionBuildFixture(t, "stages."+ext, string(motionColorStageFixture(t, ext, reverse)))
			slide := deck.Slides[0]
			if slide.Warning != "" || len(slide.Effects) != 2 || slide.Effects[0].Repeat != 3 || slide.Effects[0].AutoReverse != reverse || slide.Effects[1].Previous == nil || *slide.Effects[1].Previous != 0 {
				t.Fatal(deck)
			}
			if motionActiveDuration(slide.Effects[0]) != map[bool]int{false: 3000, true: 6000}[reverse] {
				t.Fatal(slide.Effects[0])
			}
			if len(slide.Colors) != 1 || len(slide.Colors[0].Tracks) != 1 || len(slide.Colors[0].Tracks[0]) != 2 {
				t.Fatal("stages were rendered as independent paint deltas", deck)
			}
			paint := slide.Colors[0]
			start := "00FF00"
			if reverse {
				start = "FF0000"
			}
			if motionHex(slide.Effects[1].From) != start {
				t.Fatal("stage did not inherit previous final", deck)
			}
			if ext == "pptx" {
				for _, sample := range paint.Samples {
					if sample.Positions[0] == 2.0/3 {
						body := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", sample.White)]
						if !strings.Contains(body, `val="`+start+`"`) {
							t.Fatal("second stage initial raster is incorrect", body)
						}
					}
				}
			}
		}
	}
}

func motionGradientStageFixture(t *testing.T, ext string) []byte {
	var data []byte
	if ext == "pptx" {
		timing := motionSingleBehavior(`<p:anim valueType="num" from="0" to="90">COMMON</p:anim>`, "fill.angle")
		next := motionPPTXStage(`<p:anim valueType="num" from="90" to="180">COMMON</p:anim>`, "fill.angle", "afterEffect")
		timing = strings.Replace(timing, `</p:childTnLst></p:cTn></p:seq>`, next+`</p:childTnLst></p:cTn></p:seq>`, 1)
		data = motionBehaviorFixture(t, motionRichColorFixture(t), timing)
	} else {
		data = []byte(motionODPGradientFixture(t, "tayga:gradient-angle", `smil:from="0" smil:to="90"`, "linear", false))
		next := strings.Replace(motionODPExtraGeometry("tayga:gradient-angle", `smil:from="90" smil:to="180"`), "with-previous", "after-previous", 1)
		data = motionPatchPart(t, data, "content.xml", func(body []byte) []byte {
			return bytes.Replace(body, []byte(`</anim:seq>`), []byte(next+`</anim:seq>`), 1)
		})
	}
	return data
}
func TestMotionGradientStages(t *testing.T) {
	for _, ext := range []string{"pptx", "odp"} {
		data := motionGradientStageFixture(t, ext)
		deck, _ := motionBuildFixture(t, "angle."+ext, string(data))
		slide := deck.Slides[0]
		if slide.Warning != "" || slide.Effects[1].From[0] != 90 || slide.Effects[1].To[0] != 180 || len(slide.Colors) != 1 || len(slide.Colors[0].Tracks) == 0 {
			t.Fatal(deck)
		}
	}
}

func TestMotionRotationStageOrigins(t *testing.T) {
	timing := motionSingleBehavior(`<p:animRot by="5400000">COMMON</p:animRot>`, "")
	next := motionPPTXStage(`<p:animRot from="5400000" by="5400000">COMMON</p:animRot>`, "", "afterEffect")
	timing = strings.Replace(timing, `</p:childTnLst></p:cTn></p:seq>`, next+`</p:childTnLst></p:cTn></p:seq>`, 1)
	deck, _ := motionBuildFixture(t, "rotate.pptx", string(motionBehaviorFixture(t, motionTransformFixture(t), timing)))
	if deck.Slides[0].Warning != "" || deck.Slides[0].Effects[1].From[0] != 90 || deck.Slides[0].Effects[1].To[0] != 180 {
		t.Fatal(deck)
	}
}

func TestMotionRepeatLimits(t *testing.T) {
	for _, attrs := range []string{`repeatCount="indefinite"`, `repeatCount="1500"`, `repeatCount="21000"`, `repeatCount="0"`, `repeatDur="1000"`, `autoRev="bad"`, `repeatCount="20000" autoRev="1"`} {
		timing := motionSingleBehavior(`<p:animRot to="5400000">COMMON</p:animRot>`, "")
		duration := "1000"
		if strings.Contains(attrs, `20000`) {
			duration = "30000"
		}
		timing = strings.Replace(timing, `dur="1000"`, `dur="`+duration+`" `+attrs, 1)
		deck, _ := motionBuildFixture(t, "bad.pptx", string(motionBehaviorFixture(t, motionTransformFixture(t), timing)))
		if deck.Slides[0].Warning == "" {
			t.Fatal("unsupported repeat accepted", attrs)
		}
	}
}

func motionFocusStageFixture(t *testing.T, ext string) []byte {
	if ext == "odp" {
		next := strings.Replace(motionODPExtraGeometry("tayga:gradient-focus-width", `smil:from="0.6" smil:to="0.4"`), "with-previous", "after-previous", 1)
		return motionODPMultiFixture(t, next, true)
	}
	next := motionPPTXStage(`<p:anim valueType="num" from="0.6" to="0.4">COMMON</p:anim>`, "fill.focussize.x", "afterEffect")
	return motionPPTXMultiFixture(t, next, true)
}

func TestMotionFocusStagesWithColor(t *testing.T) {
	for _, ext := range []string{"pptx", "odp"} {
		deck, _ := motionBuildFixture(t, "focus."+ext, string(motionFocusStageFixture(t, ext)))
		if deck.Slides[0].Warning != "" || len(deck.Slides[0].Colors) != 1 || len(deck.Slides[0].Colors[0].Tracks) != 2 || deck.Frames > 120 {
			t.Fatal(deck)
		}
	}
}

func TestMotionRepeatDoesNotMultiplyFrames(t *testing.T) {
	data := motionColorStageFixture(t, "pptx", false)
	one := motionPatchPart(t, data, "ppt/slides/slide1.xml", func(body []byte) []byte {
		return bytes.Replace(body, []byte(`repeatCount="3000"`), []byte(`repeatCount="1000"`), 1)
	})
	many := motionPatchPart(t, data, "ppt/slides/slide1.xml", func(body []byte) []byte {
		return bytes.Replace(body, []byte(`repeatCount="3000"`), []byte(`repeatCount="20000"`), 1)
	})
	a, _ := motionBuildFixture(t, "one.pptx", string(one))
	b, _ := motionBuildFixture(t, "many.pptx", string(many))
	if a.Slides[0].Warning != "" || b.Slides[0].Warning != "" || a.Frames != b.Frames {
		t.Fatal(a, b)
	}
}

func TestMotionScaleStagesODP(t *testing.T) {
	data := []byte(motionODPGradientFixture(t, "tayga:gradient-angle", `smil:to="90"`, "linear", false))
	data = motionPatchPart(t, data, "content.xml", func(body []byte) []byte {
		a := bytes.Index(body, []byte(`<anim:par><anim:seq>`))
		b := bytes.Index(body[a:], []byte(`</anim:seq></anim:par>`)) + a + len(`</anim:seq></anim:par>`)
		stage := func(trigger, operands string) string {
			return `<anim:par presentation:node-type="` + trigger + `" presentation:preset-class="emphasis"><anim:animateTransform svg:type="scale" smil:targetElement="shape" smil:dur="1s" ` + operands + `/></anim:par>`
		}
		timing := `<anim:par><anim:seq>` + stage("on-click", `smil:to="2,2"`) + stage("after-previous", `smil:from="2,2" smil:by="0.5,-0.5"`) + `</anim:seq></anim:par>`
		return append(append(append([]byte{}, body[:a]...), []byte(timing)...), body[b:]...)
	})
	deck, _ := motionBuildFixture(t, "scale.odp", string(data))
	slide := deck.Slides[0]
	if slide.Warning != "" || slide.Effects[1].From[0] != 2 || slide.Effects[1].To[0] != 2.5 || slide.Effects[1].To[1] != 1.5 {
		t.Fatal(deck)
	}
}

func TestMotionUnsupportedStageTiming(t *testing.T) {
	data := motionColorStageFixture(t, "pptx", false)
	for _, change := range []struct{ old, new string }{
		{`nodeType="afterEffect"`, `nodeType="withEffect"`},
		{`presetClass="emph"`, `dur="500" presetClass="emph"`},
		{`val="00FF00"/></p:from>`, `val="FFFFFF"/></p:from>`},
	} {
		modified := motionPatchPart(t, data, "ppt/slides/slide1.xml", func(body []byte) []byte { return bytes.Replace(body, []byte(change.old), []byte(change.new), 1) })
		deck, _ := motionBuildFixture(t, "bad.pptx", string(modified))
		if deck.Slides[0].Warning == "" {
			t.Fatal("unsafe stage timing/origin accepted", change, deck)
		}
	}
}

func TestMotionRepeatedPathReturnsToSource(t *testing.T) {
	timing := motionSingleBehavior(`<p:animMotion origin="layout" path="M 0 0 L 0.25 0 E">COMMON</p:animMotion>`, "")
	timing = strings.Replace(timing, `presetClass="emph"`, `presetClass="path"`, 1)
	timing = strings.Replace(timing, `dur="1000"`, `dur="1000" repeatCount="2000" autoRev="1"`, 1)
	deck, parts := motionBuildFixture(t, "path.pptx", string(motionBehaviorFixture(t, motionTransformFixture(t), timing)))
	slide := deck.Slides[0]
	if slide.Warning != "" || motionActiveDuration(slide.Effects[0]) != 4000 {
		t.Fatal(deck)
	}
	last := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.States[len(slide.States)-1])]
	if !strings.Contains(last, `x="1000000"`) {
		t.Fatal("reversed path final state moved the object", last)
	}
}

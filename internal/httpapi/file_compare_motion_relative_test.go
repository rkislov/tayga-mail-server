package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func motionSingleBehavior(behavior, property string) string {
	return `<p:timing><p:tnLst><p:par><p:cTn nodeType="tmRoot"><p:childTnLst><p:seq><p:cTn nodeType="mainSeq"><p:childTnLst><p:par><p:cTn presetClass="emph" presetID="6" nodeType="clickEffect"><p:childTnLst>` + strings.Replace(behavior, "COMMON", `<p:cBhvr><p:cTn dur="1000"/><p:tgtEl><p:spTgt spid="2"/></p:tgtEl><p:attrNameLst><p:attrName>`+property+`</p:attrName></p:attrNameLst></p:cBhvr>`, 1) + `</p:childTnLst></p:cTn></p:par></p:childTnLst></p:cTn></p:seq></p:childTnLst></p:cTn></p:par></p:tnLst></p:timing>`
}
func motionBehaviorFixture(t *testing.T, source []byte, timing string) []byte {
	t.Helper()
	reader, _ := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, entry := range reader.File {
		stream, _ := entry.Open()
		body, _ := io.ReadAll(stream)
		stream.Close()
		if entry.Name == "ppt/slides/slide1.xml" {
			spans, _ := motionSpans(body, "sld", "timing")
			body = motionEdit(body, spans, [][]byte{[]byte(timing)})
		}
		streamOut, _ := writer.Create(entry.Name)
		streamOut.Write(body)
	}
	writer.Close()
	return out.Bytes()
}
func TestMotionRelativeColors(t *testing.T) {
	timing := motionSingleBehavior(`<p:animClr clrSpc="hsl">COMMON<p:by><p:hsl h="21600000" s="0" l="0"/></p:by></p:animClr>`, "fillcolor")
	deck, parts := motionBuildFixture(t, "relative.pptx", string(motionBehaviorFixture(t, motionTransformFixture(t), timing)))
	slide := deck.Slides[0]
	if slide.Warning != "" || !slide.Effects[0].Relative || slide.Effects[0].Delta[0] != 360 || len(slide.Colors[0].Samples) != 9 || motionHex(slide.Effects[0].To) != "FF0000" {
		t.Fatal(deck)
	}
	middle := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.Colors[0].Samples[4].White)]
	if !strings.Contains(middle, `val="00FFFF"`) {
		t.Fatal("full hue cycle was collapsed to identical endpoints")
	}
	e := motionEffect{Relative: true, DeltaSpace: "rgb", Delta: []float64{20, -10, 100}}
	if c := motionRelativeColor([]float64{250, 5, 10}, e, .5); motionHex(c) != "FF003C" {
		t.Fatal(c)
	}
	for _, item := range []struct {
		value, space string
		ok           bool
	}{{"rgb(-10,20,30)", "rgb", true}, {"rgb(-100%,0%,50%)", "rgb", true}, {"hsl(-180,20%,-10%)", "hsl", true}, {"hsl(361,0%,0%)", "hsl", false}, {"hsl(NaN,0%,0%)", "hsl", false}, {"rgb(999,0,0)", "rgb", false}, {"rgb(0,0,0);alert(1)", "rgb", false}} {
		effect := motionEffect{Space: item.space}
		err := motionODPRelative(item.value, &effect)
		if (err == nil) != item.ok {
			t.Fatal(item, err)
		}
	}
	// To takes precedence over By, as required by the Office color animation contract.
	timing = motionSingleBehavior(`<p:animClr clrSpc="rgb">COMMON<p:to><a:srgbClr val="0000FF"/></p:to><p:by><p:rgb r="100000" g="0" b="0"/></p:by></p:animClr>`, "fillcolor")
	deck, _ = motionBuildFixture(t, "relative.pptx", string(motionBehaviorFixture(t, motionTransformFixture(t), timing)))
	if deck.Slides[0].Warning != "" || deck.Slides[0].Effects[0].Relative || motionHex(deck.Slides[0].Effects[0].To) != "0000FF" {
		t.Fatal(deck)
	}
}
func TestMotionGradientGeometry(t *testing.T) {
	timing := motionSingleBehavior(`<p:anim valueType="num" calcmode="lin" from="0" to="90">COMMON</p:anim>`, "fill.angle")
	deck, parts := motionBuildFixture(t, "angle.pptx", string(motionBehaviorFixture(t, motionRichColorFixture(t), timing)))
	slide := deck.Slides[0]
	if slide.Warning != "" || slide.Effects[0].Kind != "gradient" || len(slide.Colors[0].Samples) != 9 {
		t.Fatal(deck)
	}
	end := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.Colors[0].ToWhite)]
	if !strings.Contains(end, `ang="16200000"`) || !strings.Contains(end, "TEXT") {
		t.Fatal("gradient angle or text changed incorrectly")
	}
	bad := strings.Replace(timing, `to="90"`, `to="NaN"`, 1)
	deck, _ = motionBuildFixture(t, "angle.pptx", string(motionBehaviorFixture(t, motionRichColorFixture(t), bad)))
	if deck.Slides[0].Warning == "" {
		t.Fatal("non-finite gradient accepted")
	}
	for _, item := range []struct{ name, behavior, property string }{
		{"multiple turns", `<p:anim valueType="num" to="361">COMMON</p:anim>`, "fill.angle"},
		{"wrong origin", `<p:anim valueType="num" from="45" to="90">COMMON</p:anim>`, "fill.angle"},
		{"formula", `<p:anim valueType="num" to="width/2">COMMON</p:anim>`, "fill.angle"},
		{"keyframes", `<p:anim valueType="num" to="90">COMMON<p:tavLst/></p:anim>`, "fill.angle"},
		{"focus on linear", `<p:anim valueType="num" to="0.25">COMMON</p:anim>`, "fill.focusposition.x"},
		{"unknown geometry", `<p:anim valueType="num" to="0.25">COMMON</p:anim>`, "fill.focussize.x"},
	} {
		t.Run(item.name, func(t *testing.T) {
			deck, _ := motionBuildFixture(t, "angle.pptx", string(motionBehaviorFixture(t, motionRichColorFixture(t), motionSingleBehavior(item.behavior, item.property))))
			if deck.Slides[0].Warning == "" {
				t.Fatal("unsupported gradient was accepted")
			}
		})
	}
	timing = motionSingleBehavior(`<p:anim valueType="num" by="-90">COMMON</p:anim>`, "fill.angle")
	deck, _ = motionBuildFixture(t, "angle.pptx", string(motionBehaviorFixture(t, motionRichColorFixture(t), timing)))
	if deck.Slides[0].Warning != "" || deck.Slides[0].Effects[0].To[0] != -90 {
		t.Fatal("relative gradient angle rejected", deck)
	}
	// A radial focus moves within a fixed focal rectangle; it does not translate the object.
	effect := motionEffect{Kind: "gradient", Property: "focus-x", From: []float64{.5}, To: []float64{.25}, focusRect: [4]float64{.5, .5, .5, .5}}
	body := []byte(`<p:sld xmlns:p="p" xmlns:a="a"><p:cSld><p:spTree><p:sp><p:spPr><a:gradFill><a:path path="circle"><a:fillToRect l="50000" t="50000" r="50000" b="50000"/></a:path></a:gradFill></p:spPr></p:sp></p:spTree></p:cSld></p:sld>`)
	changed, err := motionPPTXGradientBody(body, effect, 9)
	if err != nil || !bytes.Contains(changed, []byte(`l="25000"`)) || !bytes.Contains(changed, []byte(`r="75000"`)) {
		t.Fatal(string(changed), err)
	}
}
func TestMotionRelativeAndGradientRealOffice(t *testing.T) {
	if os.Getenv("TAYGA_TEST_OFFICE_RENDER") != "1" {
		t.Skip("requires local LibreOffice/Poppler")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		out, e := exec.CommandContext(ctx, "soffice", append([]string{"--headless", "-env:UserInstallation=file://" + filepath.Join(dir, "profile")}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("%v %s", e, out)
		}
	}
	relative := motionBehaviorFixture(t, motionTransformFixture(t), motionSingleBehavior(`<p:animClr clrSpc="hsl">COMMON<p:by><p:hsl h="10800000" s="0" l="0"/></p:by></p:animClr>`, "fillcolor"))
	for _, kind := range []string{"relative-pptx", "relative-odp", "angle-pptx", "focus-pptx"} {
		data, ext := relative, "pptx"
		if kind == "relative-odp" {
			os.WriteFile(filepath.Join(dir, "source.pptx"), relative, 0600)
			run("--convert-to", "odp", "--outdir", dir, filepath.Join(dir, "source.pptx"))
			var e error
			data, e = os.ReadFile(filepath.Join(dir, "source.odp"))
			if e != nil {
				t.Fatal(e)
			}
			ext = "odp"
		}
		if kind == "angle-pptx" {
			data = motionBehaviorFixture(t, motionRichColorFixture(t), motionSingleBehavior(`<p:anim valueType="num" calcmode="lin" from="0" to="90">COMMON</p:anim>`, "fill.angle"))
		}
		if kind == "focus-pptx" {
			base := motionBehaviorFixture(t, motionRichColorFixture(t), motionSingleBehavior(`<p:anim valueType="num" calcmode="lin" from="0.5" to="0.25">COMMON</p:anim>`, "fill.focusposition.x"))
			reader, _ := zip.NewReader(bytes.NewReader(base), int64(len(base)))
			var packed bytes.Buffer
			writer := zip.NewWriter(&packed)
			for _, entry := range reader.File {
				stream, _ := entry.Open()
				body, _ := io.ReadAll(stream)
				stream.Close()
				if entry.Name == "ppt/slides/slide1.xml" {
					body = bytes.Replace(body, []byte(`<a:lin ang="0" scaled="1"/>`), []byte(`<a:path path="circle"><a:fillToRect l="50000" t="50000" r="50000" b="50000"/></a:path>`), 1)
				}
				dst, _ := writer.Create(entry.Name)
				dst.Write(body)
			}
			writer.Close()
			data = packed.Bytes()
		}
		source, _ := os.CreateTemp(dir, "input")
		source.Write(data)
		deck, prepared, e := buildMotionStoryboard(source, "test."+ext)
		source.Close()
		if e != nil {
			t.Fatal(kind, e)
		}
		slide := deck.Slides[0]
		if slide.Warning != "" || len(slide.Colors) != 1 {
			t.Fatal(kind, deck)
		}
		file := filepath.Join(dir, kind+"."+ext)
		os.WriteFile(file, prepared, 0600)
		run("--convert-to", "pdf", "--outdir", dir, file)
		hashes := []string{}
		for i, frame := range []int{slide.Colors[0].FromWhite, slide.Colors[0].ToWhite} {
			prefix := filepath.Join(dir, fmt.Sprintf("%s-%d", kind, i))
			out, e := exec.Command("pdftoppm", "-f", fmt.Sprint(frame), "-l", fmt.Sprint(frame), "-scale-to", "600", "-png", "-singlefile", filepath.Join(dir, kind+".pdf"), prefix).CombinedOutput()
			if e != nil {
				t.Fatalf("%v %s", e, out)
			}
			pngFile, _ := os.Open(prefix + ".png")
			img, e := png.Decode(pngFile)
			pngFile.Close()
			if e != nil {
				t.Fatal(e)
			}
			var rgb bytes.Buffer
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					rgb.WriteByte(byte(r >> 8))
					rgb.WriteByte(byte(g >> 8))
					rgb.WriteByte(byte(b >> 8))
				}
			}
			hash := sha256.Sum256(rgb.Bytes())
			hashes = append(hashes, fmt.Sprint(hash))
		}
		if hashes[0] == hashes[1] {
			t.Fatal(kind, "renderer did not change pixels")
		}
		t.Log(kind, "rendered different start/end states")
	}
}

package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func motionRichColorTiming() string {
	effect := func(property, space, to, trigger string) string {
		return `<p:par><p:cTn presetClass="emph" presetID="6" nodeType="` + trigger + `"><p:childTnLst><p:animClr clrSpc="` + space + `" dir="cw"><p:cBhvr><p:cTn dur="1000"/><p:tgtEl><p:spTgt spid="2"/></p:tgtEl><p:attrNameLst><p:attrName>` + property + `</p:attrName></p:attrNameLst></p:cBhvr><p:to>` + to + `</p:to></p:animClr></p:childTnLst></p:cTn></p:par>`
	}
	return `<p:timing><p:tnLst><p:par><p:cTn nodeType="tmRoot"><p:childTnLst><p:seq><p:cTn nodeType="mainSeq"><p:childTnLst>` + effect("fillcolor", "hsl", `<a:schemeClr val="accent2"/>`, "clickEffect") + effect("stroke.color", "rgb", `<a:srgbClr val="FF0000"/>`, "withEffect") + effect("style.color", "rgb", `<a:hslClr hue="7200000" sat="100000" lum="50000"/>`, "withEffect") + `</p:childTnLst></p:cTn></p:seq></p:childTnLst></p:cTn></p:par></p:tnLst></p:timing>`
}
func motionRichColorFixture(t *testing.T) []byte {
	source := motionTransformFixture(t)
	archive, _ := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	for _, entry := range archive.File {
		stream, _ := entry.Open()
		data, _ := io.ReadAll(stream)
		stream.Close()
		if entry.Name == "ppt/slides/slide1.xml" {
			spans, _ := motionSpans(data, "sld", "timing")
			data = motionEdit(data, spans, [][]byte{[]byte(motionRichColorTiming())})
			data = bytes.Replace(data, []byte(`<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>`), []byte(`<a:gradFill><a:gsLst><a:gs pos="0"><a:srgbClr val="FF0000"/></a:gs><a:gs pos="100000"><a:srgbClr val="0000FF"/></a:gs></a:gsLst><a:lin ang="0" scaled="1"/></a:gradFill>`), 1)
			data = bytes.Replace(data, []byte(`w="20000"`), []byte(`w="80000"`), 1)
			data = bytes.Replace(data, []byte(`</p:sld>`), []byte(`<p:clrMapOvr><a:overrideClrMapping bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent3" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:clrMapOvr></p:sld>`), 1)
		}
		if entry.Name == "ppt/slides/slide1.xml" {
			maps, _ := motionSpans(data, "sld", "clrMapOvr")
			if len(maps) > 1 {
				data = motionEdit(data, maps[:len(maps)-1], nil)
			}
		}
		out, _ := writer.Create(entry.Name)
		out.Write(data)
	}
	writer.Close()
	return packed.Bytes()
}
func TestMotionRichColorThemeHSLAndProperties(t *testing.T) {
	deck, parts := motionBuildFixture(t, "color.pptx", string(motionRichColorFixture(t)))
	slide := deck.Slides[0]
	if slide.Warning != "" || len(slide.Colors) != 3 || len(slide.Colors[0].Samples) != 9 {
		t.Fatalf("%+v", deck)
	}
	effect := slide.Effects[0]
	if effect.Space != "hsl" || effect.Direction != "cw" || motionHex(effect.To) != "9BBB59" || !effect.gradient {
		t.Fatal(effect)
	}
	if slide.Effects[1].Property != "stroke" || slide.Effects[2].Property != "text" {
		t.Fatal(slide.Effects)
	}
	fill := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.Colors[0].ToWhite)]
	if !strings.Contains(fill, `<a:gradFill>`) || strings.Count(fill, `val="9BBB59"`) != 2 {
		t.Fatal("gradient geometry/stops were lost")
	}
	text := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.Colors[2].ToWhite)]
	if !strings.Contains(text, `val="00FF00"`) || !strings.Contains(text, "TEXT") || !strings.Contains(text, `<a:gradFill>`) {
		t.Fatal("text color replaced the gradient/body")
	}
	cw := motionColorTween([]float64{255, 0, 0}, []float64{0, 0, 255}, .5, "hsl", "cw")
	ccw := motionColorTween([]float64{255, 0, 0}, []float64{0, 0, 255}, .5, "hsl", "ccw")
	if motionHex(cw) != "00FF00" || motionHex(ccw) != "FF00FF" {
		t.Fatal(cw, ccw)
	}
	tint := &compareNode{name: xml.Name{Local: "srgbClr"}, attrs: []xml.Attr{{Name: xml.Name{Local: "val"}, Value: "000000"}}, children: []*compareNode{{name: xml.Name{Local: "tint"}, attrs: []xml.Attr{{Name: xml.Name{Local: "val"}, Value: "10000"}}}}}
	color, e := motionDrawingColor(tint, nil)
	if e != nil || motionHex(color) != "E6E6E6" {
		t.Fatal(color, e)
	}
}
func TestMotionRichColorRealOffice(t *testing.T) {
	if os.Getenv("TAYGA_TEST_OFFICE_RENDER") != "1" {
		t.Skip("requires local office/Poppler")
	}
	dir := t.TempDir()
	original := motionRichColorFixture(t)
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		out, e := exec.CommandContext(ctx, "soffice", append([]string{"--headless", "-env:UserInstallation=file://" + filepath.Join(dir, "profile")}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("office: %v %s", e, out)
		}
	}
	for _, ext := range []string{"pptx", "odp"} {
		data := original
		if ext == "odp" {
			os.WriteFile(filepath.Join(dir, "original.pptx"), data, 0600)
			run("--convert-to", "odp", "--outdir", dir, filepath.Join(dir, "original.pptx"))
			var e error
			data, e = os.ReadFile(filepath.Join(dir, "original.odp"))
			if e != nil {
				t.Fatal(e)
			}
		}
		file, _ := os.CreateTemp(dir, "source-")
		file.Write(data)
		deck, prepared, e := buildMotionStoryboard(file, "rich."+ext)
		file.Close()
		if e != nil {
			t.Fatal(ext, e)
		}
		slide := deck.Slides[0]
		if slide.Warning != "" || len(slide.Colors) != 3 {
			t.Fatalf("%s %+v", ext, deck)
		}
		source := filepath.Join(dir, "rich-"+ext+"."+ext)
		os.WriteFile(source, prepared, 0600)
		run("--convert-to", "pdf", "--outdir", dir, source)
		pdf := filepath.Join(dir, "rich-"+ext+".pdf")
		for index, palette := range slide.Colors {
			images := []image.Image{}
			for i, frame := range []int{palette.FromWhite, palette.ToWhite} {
				prefix := filepath.Join(dir, fmt.Sprintf("%s-%d-%d", ext, index, i))
				output, e := exec.Command("pdftoppm", "-f", fmt.Sprint(frame), "-l", fmt.Sprint(frame), "-scale-to", "800", "-png", "-singlefile", pdf, prefix).CombinedOutput()
				if e != nil {
					t.Fatalf("%v %s", e, output)
				}
				pngFile, _ := os.Open(prefix + ".png")
				img, e := png.Decode(pngFile)
				pngFile.Close()
				if e != nil {
					t.Fatal(e)
				}
				if index == 0 && i == 1 {
					r, g, b, _ := img.At(int(float64(img.Bounds().Dx())*.14), int(float64(img.Bounds().Dy())*.26)).RGBA()
					expected := slide.Effects[palette.Effect].To
					actual := []float64{float64(r) / 257, float64(g) / 257, float64(b) / 257}
					for k := range actual {
						if math.Abs(actual[k]-expected[k]) > 4 {
							t.Fatalf("%s theme/gradient destination: %v want %v", ext, actual, expected)
						}
					}
				}
				images = append(images, img)
			}
			changed := 0
			for y := 0; y < images[0].Bounds().Dy(); y++ {
				for x := 0; x < images[0].Bounds().Dx(); x++ {
					if images[0].At(x, y) != images[1].At(x, y) {
						changed++
					}
				}
			}
			if changed < 20 {
				t.Fatalf("%s %s did not change (%d)", ext, slide.Effects[palette.Effect].Property, changed)
			}
		}
		t.Logf("%s: actual gradient fill, contour and text palettes differ; %d PDF pages", ext, deck.Frames)
	}
}

func TestMotionColorNamespaceDeclaration(t *testing.T) {
	source := []byte(`<style:style xmlns:style="urn:test" style:name="old"/>`)
	result := motionRootAttr(source, "display-name", "new", "style:display-name", "urn:test")
	if strings.Count(string(result), "xmlns:style=") != 1 || !bytes.Contains(result, []byte(`style:display-name="new"`)) {
		t.Fatal(string(result))
	}
	collision := motionRootAttr(source, "color", "red", "style:color", "urn:other")
	decoder := xml.NewDecoder(bytes.NewReader(collision))
	token, e := decoder.Token()
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, a := range token.(xml.StartElement).Attr {
		if a.Name.Space == "urn:other" && a.Name.Local == "color" {
			found = true
		}
	}
	if !found {
		t.Fatal(string(collision))
	}
}

func TestMotionRichColorWholeTextScope(t *testing.T) {
	for _, end := range []string{"0", "1"} {
		source := motionRichColorFixture(t)
		archive, _ := zip.NewReader(bytes.NewReader(source), int64(len(source)))
		var packed bytes.Buffer
		writer := zip.NewWriter(&packed)
		for _, entry := range archive.File {
			stream, _ := entry.Open()
			body, _ := io.ReadAll(stream)
			stream.Close()
			if entry.Name == "ppt/slides/slide1.xml" {
				old := `<p:tgtEl><p:spTgt spid="2"/></p:tgtEl><p:attrNameLst><p:attrName>style.color`
				next := `<p:tgtEl><p:spTgt spid="2"><p:txEl><p:pRg st="0" end="` + end + `"/></p:txEl></p:spTgt></p:tgtEl><p:attrNameLst><p:attrName>style.color`
				body = bytes.Replace(body, []byte(old), []byte(next), 1)
			}
			out, _ := writer.Create(entry.Name)
			out.Write(body)
		}
		writer.Close()
		deck, _ := motionBuildFixture(t, "scope.pptx", packed.String())
		if end == "0" && deck.Slides[0].Warning != "" {
			t.Fatal(deck)
		}
		if end == "1" && deck.Slides[0].Warning == "" {
			t.Fatal("out-of-range paragraph target accepted")
		}
	}
}

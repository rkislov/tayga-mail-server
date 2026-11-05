package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
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

func motionTransformTiming() string {
	common := `<p:cBhvr><p:cTn dur="1000"/><p:tgtEl><p:spTgt spid="2"/></p:tgtEl></p:cBhvr>`
	behaviors := []string{`<p:animRot by="5400000">` + common + `</p:animRot>`, `<p:animScale>` + common + `<p:by x="150000" y="50000"/></p:animScale>`, `<p:animClr clrSpc="rgb">` + strings.Replace(common, `</p:cBhvr>`, `<p:attrNameLst><p:attrName>fillcolor</p:attrName></p:attrNameLst></p:cBhvr>`, 1) + `<p:from><a:srgbClr val="FF0000"/></p:from><p:to><a:srgbClr val="0000FF"/></p:to></p:animClr>`}
	var effects strings.Builder
	for i, behavior := range behaviors {
		trigger := "withEffect"
		if i == 0 {
			trigger = "clickEffect"
		}
		fmt.Fprintf(&effects, `<p:par><p:cTn nodeType="%s" presetClass="emph" presetID="8"><p:childTnLst>%s</p:childTnLst></p:cTn></p:par>`, trigger, behavior)
	}
	return `<p:timing><p:tnLst><p:par><p:cTn nodeType="tmRoot"><p:childTnLst><p:seq><p:cTn nodeType="mainSeq"><p:childTnLst>` + effects.String() + `</p:childTnLst></p:cTn></p:seq></p:childTnLst></p:cTn></p:par></p:tnLst></p:timing>`
}
func motionTransformFixture(t *testing.T) []byte {
	t.Helper()
	source, e := officeTemplates.ReadFile("file_templates/presentation.pptx")
	if e != nil {
		t.Fatal(e)
	}
	archive, _ := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	tree := `<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="2" name="Color rectangle"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="1000000" y="1000000"/><a:ext cx="2000000" cy="1000000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill><a:ln w="20000"><a:solidFill><a:srgbClr val="000000"/></a:solidFill></a:ln></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr sz="1800"><a:solidFill><a:srgbClr val="000000"/></a:solidFill></a:rPr><a:t>TEXT</a:t></a:r></a:p></p:txBody></p:sp></p:spTree>`
	for _, entry := range archive.File {
		stream, _ := entry.Open()
		body, _ := io.ReadAll(stream)
		stream.Close()
		if entry.Name == "ppt/slides/slide1.xml" {
			spans, _ := motionSpans(body, "cSld", "spTree")
			body = motionEdit(body, spans, [][]byte{[]byte(tree)})
			body = []byte(strings.Replace(string(body), "</p:sld>", motionTransformTiming()+"</p:sld>", 1))
		}
		dest, _ := writer.Create(entry.Name)
		dest.Write(body)
	}
	writer.Close()
	return packed.Bytes()
}
func TestMotionTransformManifest(t *testing.T) {
	deck, parts := motionBuildFixture(t, "transform.pptx", string(motionTransformFixture(t)))
	slide := deck.Slides[0]
	if slide.Warning != "" || len(slide.Effects) != 3 || len(slide.Colors) != 1 || len(slide.Layers) != 1 {
		t.Fatalf("%+v", deck)
	}
	if slide.Effects[0].Kind != "rotate" || slide.Effects[0].To[0] != 90 || slide.Effects[1].Kind != "scale" || slide.Effects[1].To[0] != 1.5 || slide.Effects[1].To[1] != .5 || slide.Effects[2].Kind != "color" {
		t.Fatal(slide.Effects)
	}
	color := slide.Colors[0]
	before := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", color.FromWhite)]
	after := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", color.ToWhite)]
	if !strings.Contains(before, `val="FF0000"`) || !strings.Contains(after, `val="0000FF"`) || !strings.Contains(after, `val="000000"`) || !strings.Contains(after, "TEXT") {
		t.Fatal("fill mask changed text/outline")
	}
	for name, body := range parts {
		if strings.HasSuffix(name, ".xml") {
			decoder := xml.NewDecoder(strings.NewReader(body))
			for {
				_, e := decoder.Token()
				if e == io.EOF {
					break
				}
				if e != nil {
					t.Fatal(name, e)
				}
			}
		}
	}
}
func TestMotionTransformRealOfficePalettes(t *testing.T) {
	if os.Getenv("TAYGA_TEST_OFFICE_RENDER") != "1" {
		t.Skip("requires local LibreOffice/Poppler")
	}
	dir := t.TempDir()
	original := motionTransformFixture(t)
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		output, e := exec.CommandContext(ctx, "soffice", append([]string{"--headless", "-env:UserInstallation=file://" + filepath.Join(dir, "profile")}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("%v %s", e, output)
		}
	}
	for _, ext := range []string{"pptx", "odp"} {
		data := original
		if ext == "odp" {
			os.WriteFile(filepath.Join(dir, "source.pptx"), data, 0600)
			run("--convert-to", "odp", "--outdir", dir, filepath.Join(dir, "source.pptx"))
			var e error
			data, e = os.ReadFile(filepath.Join(dir, "source.odp"))
			if e != nil {
				t.Fatal(e)
			}
		}
		source, _ := os.CreateTemp(dir, "source-")
		source.Write(data)
		deck, storyboard, e := buildMotionStoryboard(source, "transform."+ext)
		source.Close()
		if e != nil {
			t.Fatal(ext, e)
		}
		slide := deck.Slides[0]
		if slide.Warning != "" || len(slide.Colors) != 1 || slide.Effects[0].To[0] != 90 || slide.Effects[1].To[0] != 1.5 || slide.Effects[1].To[1] != .5 {
			t.Fatalf("%s: %+v", ext, deck)
		}
		file := filepath.Join(dir, "palette-"+ext+"."+ext)
		os.WriteFile(file, storyboard, 0600)
		run("--convert-to", "pdf", "--outdir", dir, file)
		pdf := filepath.Join(dir, "palette-"+ext+".pdf")
		color := slide.Colors[0]
		for index, frame := range []int{color.FromWhite, color.ToWhite} {
			prefix := filepath.Join(dir, fmt.Sprintf("palette-%s-%d", ext, index))
			out, e := exec.Command("pdftoppm", "-f", fmt.Sprint(frame), "-l", fmt.Sprint(frame), "-scale-to", "800", "-png", "-singlefile", pdf, prefix).CombinedOutput()
			if e != nil {
				t.Fatalf("%v %s", e, out)
			}
			file, _ := os.Open(prefix + ".png")
			img, e := png.Decode(file)
			file.Close()
			if e != nil {
				t.Fatal(e)
			}
			x, y := int(float64(img.Bounds().Dx())*.14), int(float64(img.Bounds().Dy())*.16)
			r, g, b, _ := img.At(x, y).RGBA()
			if index == 0 && (r < 60000 || g > 4000 || b > 4000) || index == 1 && (b < 60000 || r > 4000 || g > 4000) {
				t.Fatalf("%s palette %d: %d %d %d", ext, index, r, g, b)
			}
		}
		t.Logf("%s: rotation/scale units preserved; real office rendered original red and changed blue fill layers", ext)
	}
}

func TestMotionTransformRejectsUnsupportedValues(t *testing.T) {
	original := motionTransformFixture(t)
	for _, change := range []struct{ old, new string }{{`by="5400000"`, `by="NaN"`}, {`x="150000" y="50000"`, `x="0" y="50000"`}, {`clrSpc="rgb"`, `clrSpc="lab"`}, {`<p:attrName>fillcolor</p:attrName>`, `<p:attrName>unknown.color</p:attrName>`}, {`<p:to><a:srgbClr val="0000FF"/></p:to>`, `<p:to><a:schemeClr val="missing-color"/></p:to>`}} {
		archive, _ := zip.NewReader(bytes.NewReader(original), int64(len(original)))
		var packed bytes.Buffer
		writer := zip.NewWriter(&packed)
		for _, entry := range archive.File {
			stream, _ := entry.Open()
			content, _ := io.ReadAll(stream)
			stream.Close()
			if entry.Name == "ppt/slides/slide1.xml" {
				content = []byte(strings.Replace(string(content), change.old, change.new, 1))
			}
			dst, _ := writer.Create(entry.Name)
			dst.Write(content)
		}
		writer.Close()
		deck, _ := motionBuildFixture(t, "bad.pptx", packed.String())
		slide := deck.Slides[0]
		if slide.Warning == "" || len(slide.Effects) != 0 || len(slide.Colors) != 0 || len(slide.Layers) != 0 {
			t.Fatal(change, deck)
		}
	}
}
func TestMotionODPTransformOperands(t *testing.T) {
	for _, item := range []struct {
		node, kind string
		to         []float64
		discrete   bool
	}{{`<anim:animateTransform smil:dur="1s" smil:targetElement="shape" smil:by="90" xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" svg:type="rotate"/>`, "rotate", []float64{90}, false}, {`<anim:animateTransform smil:dur="1s" smil:targetElement="shape" smil:by="0.5,-0.5" xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" svg:type="scale"/>`, "scale", []float64{1.5, .5}, false}, {`<anim:animateColor smil:dur="1s" smil:targetElement="shape" smil:attributeName="fill-color" smil:to="#0000ff"/>`, "color", []float64{0, 0, 255}, true}} {
		start := strings.Index(motionODPBody, "<anim:transitionFilter")
		end := strings.Index(motionODPBody[start:], "/>") + start + 2
		body := motionODPBody[:start] + item.node + motionODPBody[end:]
		body = strings.Replace(body, `presentation:preset-class="entrance"`, `presentation:preset-class="emphasis"`, 1)
		file, _ := os.CreateTemp(t.TempDir(), "operands")
		file.WriteString(comparisonPackageFixture(t, map[string]string{"content.xml": body}))
		pkg, e := openComparePackage(file)
		if e != nil {
			t.Fatal(e)
		}
		root, e := pkg.xml("content.xml")
		file.Close()
		if e != nil {
			t.Fatal(e)
		}
		effects, e := motionODPEffects(root.child("body").child("presentation").child("page"))
		if e != nil || len(effects) != 1 {
			t.Fatal(effects, e)
		}
		effect := effects[0]
		if effect.Kind != item.kind || effect.Discrete != item.discrete || len(effect.To) != len(item.to) {
			t.Fatal(effect)
		}
		for i, v := range item.to {
			if effect.To[i] != v {
				t.Fatal(effect)
			}
		}
	}
}

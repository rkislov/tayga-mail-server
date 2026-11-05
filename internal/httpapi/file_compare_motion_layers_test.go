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

func motionAdvancedTiming() string {
	effect := func(kind, target, trigger, condition string) string {
		behavior := `<p:animEffect transition="in" filter="fade">`
		end := "</p:animEffect>"
		class := "entr"
		if kind == "path" {
			behavior = `<p:animMotion origin="layout" path="M 0 0 L 0.25 0 C 0.25 0 0.25 0.25 0.5 0.25 E" pathEditMode="relative">`
			end = "</p:animMotion>"
			class = "path"
		}
		return fmt.Sprintf(`<p:par><p:cTn nodeType="%s" presetClass="%s" presetID="10">%s<p:childTnLst>%s<p:cBhvr><p:cTn dur="1000"/><p:tgtEl><p:spTgt spid="%s"/></p:tgtEl></p:cBhvr>%s</p:childTnLst></p:cTn></p:par>`, trigger, class, condition, behavior, target, end)
	}
	return `<p:timing><p:tnLst><p:par><p:cTn nodeType="tmRoot"><p:childTnLst><p:seq><p:cTn nodeType="mainSeq"><p:childTnLst>` + effect("fade", "2", "clickEffect", "") + effect("fade", "3", "withEffect", "") + `</p:childTnLst></p:cTn></p:seq><p:seq><p:cTn nodeType="interactiveSeq"><p:stCondLst><p:cond evt="onClick" delay="0"><p:tgtEl><p:spTgt spid="3"/></p:tgtEl></p:cond></p:stCondLst><p:childTnLst>` + effect("path", "2", "clickEffect", "") + `</p:childTnLst></p:cTn></p:seq></p:childTnLst></p:cTn></p:par></p:tnLst></p:timing>`
}
func TestMotionAdvancedPPTXManifest(t *testing.T) {
	body := motionPPTXFixture(t, motionAdvancedTiming())
	archive, _ := zip.NewReader(strings.NewReader(body), int64(len(body)))
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, entry := range archive.File {
		stream, _ := entry.Open()
		content, _ := io.ReadAll(stream)
		stream.Close()
		switch entry.Name {
		case "ppt/presentation.xml":
			content = bytes.Replace(content, []byte("</p:presentation>"), []byte(`<p:sldSz cx="1000000" cy="1000000"/></p:presentation>`), 1)
		case "ppt/slides/slide1.xml":
			content = bytes.Replace(content, []byte("</p:nvSpPr>"), []byte(`</p:nvSpPr><p:spPr><a:xfrm><a:off x="100000" y="100000"/><a:ext cx="200000" cy="200000"/></a:xfrm></p:spPr>`), -1)
		}
		dest, _ := writer.Create(entry.Name)
		dest.Write(content)
	}
	writer.Close()
	deck, parts := motionBuildFixture(t, "advanced.pptx", out.String())
	slide := deck.Slides[0]
	if slide.Warning != "" || len(slide.Layers) != 2 || slide.Effects[1].Trigger != "with" || slide.Effects[2].Event != "3" || slide.Effects[2].Kind != "path" || len(slide.Effects[2].Path) < 20 || len(slide.Hits) != 2 {
		t.Fatalf("%+v", deck)
	}
	if !strings.Contains(parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.Probe[0])], `val="000000"`) || !strings.Contains(parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.Probe[1])], `val="FFFFFF"`) {
		t.Fatal("missing matte probes")
	}
	if !strings.Contains(parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", slide.Layers[0].Black)], `showMasterSp="0"`) {
		t.Fatal("master graphics not isolated")
	}
}
func TestMotionPathValidation(t *testing.T) {
	for _, value := range []string{"M0 0L.25 .25E", "M0 0c.1 0 .2 .2 .3 .3z", "M0,0L1e-2,2e-2"} {
		p, e := parseMotionPath(value)
		if e != nil || len(p) < 2 {
			t.Fatal(value, p, e)
		}
	}
	for _, value := range []string{"M0 0 A1 1 0 0 0 1 1", "M0 0LNaN 1", "M1 1L2 2", "M0 0L1 1E L2 2", "M0 0M1 1", "M0 0L999 2", "javascript:alert(1)", strings.Repeat("M0 0 ", 2000)} {
		if _, e := parseMotionPath(value); e == nil {
			t.Fatal("accepted unsupported path", value)
		}
	}
}
func TestMotionAdvancedRealOfficeLayers(t *testing.T) {
	if os.Getenv("TAYGA_TEST_OFFICE_RENDER") != "1" {
		t.Skip("requires local LibreOffice/Poppler")
	}
	original, _ := officeTemplates.ReadFile("file_templates/presentation.pptx")
	archive, _ := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	for _, entry := range archive.File {
		stream, _ := entry.Open()
		body, _ := io.ReadAll(stream)
		stream.Close()
		if entry.Name == "ppt/slides/slide1.xml" {
			text := string(body)
			for i := 0; i < 2; i++ {
				xfrm := fmt.Sprintf(`<p:spPr><a:xfrm><a:off x="%d" y="1000000"/><a:ext cx="2000000" cy="1000000"/></a:xfrm></p:spPr>`, 1000000+i*3000000)
				text = strings.Replace(text, `<p:spPr/>`, xfrm, 1)
			}
			body = []byte(strings.Replace(text, "</p:sld>", motionAdvancedTiming()+"</p:sld>", 1))
		}
		dest, _ := writer.Create(entry.Name)
		dest.Write(body)
	}
	writer.Close()
	dir := t.TempDir()
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
		data := packed.Bytes()
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
		deck, storyboard, e := buildMotionStoryboard(source, "layers."+ext)
		source.Close()
		if e != nil {
			t.Fatal(ext, e)
		}
		slide := deck.Slides[0]
		if slide.Warning != "" || len(slide.Layers) == 0 {
			t.Fatalf("%s: %+v", ext, deck)
		}
		file := filepath.Join(dir, "layers-"+ext+"."+ext)
		os.WriteFile(file, storyboard, 0600)
		run("--convert-to", "pdf", "--outdir", dir, file)
		pdf := filepath.Join(dir, "layers-"+ext+".pdf")
		for index, frame := range slide.Probe {
			prefix := filepath.Join(dir, fmt.Sprintf("probe-%s-%d", ext, index))
			output, e := exec.Command("pdftoppm", "-f", fmt.Sprint(frame), "-l", fmt.Sprint(frame), "-scale-to", "400", "-png", "-singlefile", pdf, prefix).CombinedOutput()
			if e != nil {
				t.Fatalf("%v %s", e, output)
			}
			pngFile, _ := os.Open(prefix + ".png")
			img, e := png.Decode(pngFile)
			pngFile.Close()
			if e != nil {
				t.Fatal(e)
			}
			r, g, b, _ := img.At(1, 1).RGBA()
			if index == 0 && (r > 2000 || g > 2000 || b > 2000) || index == 1 && (r < 63000 || g < 63000 || b < 63000) {
				t.Fatalf("%s probe %d: %d %d %d", ext, index, r, g, b)
			}
		}
		t.Logf("%s: %d frames, %d isolated layers, parallel fades + object click path", ext, deck.Frames, len(slide.Layers))
	}
}

func TestMotionAnonymousObjectsKeepZOrder(t *testing.T) {
	body := strings.Replace(motionODPBody, `<draw:frame xml:id="shape">`, `<draw:frame>BEFORE</draw:frame><draw:frame xml:id="shape">`, 1)
	body = strings.Replace(body, `<draw:frame xml:id="static"/>`, `<draw:frame>AFTER</draw:frame>`, 1)
	body = strings.Replace(body, `presentation:node-type="on-click"`, `presentation:node-type="with-previous"`, 1)
	deck, parts := motionBuildFixture(t, "test.odp", comparisonPackageFixture(t, map[string]string{"content.xml": body}))
	slide := deck.Slides[0]
	if slide.Warning != "" || len(slide.Layers) != 3 {
		t.Fatal(deck)
	}
	content := []byte(parts["content.xml"])
	spans, e := motionSpans(content, "presentation", "page")
	if e != nil {
		t.Fatal(e)
	}
	first := content[spans[slide.Layers[0].Black-1].start:spans[slide.Layers[0].Black-1].end]
	last := content[spans[slide.Layers[2].Black-1].start:spans[slide.Layers[2].Black-1].end]
	if !bytes.Contains(first, []byte("BEFORE")) || bytes.Contains(first, []byte("AFTER")) || !bytes.Contains(last, []byte("AFTER")) || bytes.Contains(last, []byte("BEFORE")) {
		t.Fatal("anonymous static layers were duplicated/reordered")
	}
}
func TestMotionOverlappingPropertiesFailClosed(t *testing.T) {
	slide := motionSlide{Effects: []motionEffect{{Target: "2", Kind: "fade", Trigger: "click", Duration: 1000}, {Target: "2", Kind: "fade", Trigger: "with", Duration: 1000}}}
	motionValidateAdvanced(&slide, map[string]bool{"2": true})
	if slide.Warning == "" {
		t.Fatal("overlapping opacity effects accepted")
	}
	event, delay, e := motionODPEvent("constructor.click+0.2s")
	if e != nil || event != "constructor" || delay != 200 {
		t.Fatal(event, delay, e)
	}
}

func TestMotionEmptyStyleRoot(t *testing.T) {
	result, e := motionAppendRoot([]byte(`<office:automatic-styles xmlns:office="urn:office"/>`), "automatic-styles", `<style xmlns="urn:style"/>`)
	if e != nil {
		t.Fatal(e)
	}
	decoder := xml.NewDecoder(bytes.NewReader(result))
	depth, roots := 0, 0
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(string(result), e)
		}
		switch token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
	if roots != 1 || depth != 0 {
		t.Fatal(string(result))
	}
}

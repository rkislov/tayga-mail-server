package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const motionPPTXTiming = `<p:timing><p:tnLst><p:par><p:cTn id="1" dur="indefinite" nodeType="tmRoot"><p:childTnLst><p:seq><p:cTn id="2" nodeType="mainSeq"><p:childTnLst><p:par><p:cTn id="3" nodeType="clickEffect" presetClass="entr" presetID="10"><p:stCondLst><p:cond delay="0"/></p:stCondLst><p:childTnLst><p:animEffect transition="in" filter="fade"><p:cBhvr><p:cTn id="4" dur="1000"/><p:tgtEl><p:spTgt spid="2"/></p:tgtEl></p:cBhvr></p:animEffect></p:childTnLst></p:cTn></p:par></p:childTnLst></p:cTn></p:seq></p:childTnLst></p:cTn></p:par></p:tnLst></p:timing>`

func motionPPTXFixture(t *testing.T, timing string) string {
	return comparisonPackageFixture(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="one"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="one" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld name="Example"><p:spTree><p:nvGrpSpPr/><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="2" name="fade-target"/></p:nvSpPr><p:txBody><a:p><a:r><a:t>Target text</a:t></a:r></a:p></p:txBody></p:sp><p:sp><p:nvSpPr><p:cNvPr id="3" name="static"/></p:nvSpPr></p:sp></p:spTree></p:cSld><p:transition spd="slow"><p:push dir="l"/></p:transition>` + timing + `</p:sld>`,
	})
}
func motionBuildFixture(t *testing.T, name, body string) (motionDeck, map[string]string) {
	t.Helper()
	file, e := os.CreateTemp(t.TempDir(), "motion")
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	file.WriteString(body)
	deck, data, e := buildMotionStoryboard(file, name)
	if e != nil {
		t.Fatal(e)
	}
	reader, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		t.Fatal(e)
	}
	parts := map[string]string{}
	for _, part := range reader.File {
		stream, e := part.Open()
		if e != nil {
			t.Fatal(e)
		}
		content, e := io.ReadAll(stream)
		stream.Close()
		if e != nil {
			t.Fatal(e)
		}
		parts[part.Name] = string(content)
	}
	return deck, parts
}
func TestMotionPPTXKeyStatesAndSafeFallback(t *testing.T) {
	deck, parts := motionBuildFixture(t, "test.pptx", motionPPTXFixture(t, motionPPTXTiming))
	slide := deck.Slides[0]
	if deck.Frames != 2 || len(slide.Effects) != 1 || slide.Effects[0].Kind != "fade" || slide.Effects[0].Duration != 1000 || slide.Transition.Kind != "push" || slide.Transition.Duration != 2000 || slide.Warning != "" {
		t.Fatal(deck)
	}
	if strings.Contains(parts["ppt/slides/taygaMotion1.xml"], "Target text") || !strings.Contains(parts["ppt/slides/taygaMotion2.xml"], "Target text") || strings.Contains(parts["ppt/slides/taygaMotion2.xml"], "<p:timing") || strings.Contains(parts["ppt/slides/taygaMotion2.xml"], "<p:transition") {
		t.Fatal(parts)
	}
	if !strings.Contains(parts["ppt/presentation.xml"], "taygaMotion2") || !strings.Contains(parts["[Content_Types].xml"], "taygaMotion1.xml") {
		t.Fatal("unlinked key states")
	}
	for _, timing := range []string{strings.ReplaceAll(motionPPTXTiming, `filter="fade"`, `filter="blinds"`), strings.ReplaceAll(motionPPTXTiming, `spid="2"`, `spid="99"`), strings.ReplaceAll(motionPPTXTiming, `clickEffect`, `unsupportedEffect`), strings.ReplaceAll(motionPPTXTiming, `dur="1000"`, `dur="indefinite"`)} {
		deck, parts := motionBuildFixture(t, "test.pptx", motionPPTXFixture(t, timing))
		if deck.Frames != 1 || deck.Slides[0].Warning == "" || deck.Slides[0].Effects == nil || len(deck.Slides[0].Effects) != 0 || !strings.Contains(parts["ppt/slides/taygaMotion1.xml"], "Target text") {
			t.Fatal(deck)
		}
	}
}

const motionODPBody = `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:anim="urn:oasis:names:tc:opendocument:xmlns:animation:1.0" xmlns:smil="urn:oasis:names:tc:opendocument:xmlns:smil-compatible:1.0" xmlns:presentation="urn:oasis:names:tc:opendocument:xmlns:presentation:1.0"><office:body><office:presentation><draw:page draw:name="Page"><draw:frame xml:id="shape"><draw:text-box>Target text</draw:text-box></draw:frame><draw:frame xml:id="static"/><anim:par><anim:seq><anim:par smil:begin="next"><anim:par smil:begin="0s" presentation:node-type="on-click" presentation:preset-class="entrance" presentation:preset-id="ooo-entrance-fade-in"><anim:transitionFilter smil:type="fade" smil:mode="in" smil:dur="1s" smil:targetElement="shape"/></anim:par></anim:par></anim:seq></anim:par></draw:page></office:presentation></office:body></office:document-content>`

func TestMotionODPKeyStatesAndIDs(t *testing.T) {
	deck, parts := motionBuildFixture(t, "test.odp", comparisonPackageFixture(t, map[string]string{"content.xml": motionODPBody}))
	if deck.Frames != 2 || deck.Slides[0].Warning != "" || deck.Slides[0].Effects[0].Duration != 1000 {
		t.Fatal(deck)
	}
	content := parts["content.xml"]
	if strings.Count(content, "Target text") != 1 || !strings.Contains(content, `xml:id="static_taygaFrame1"`) || !strings.Contains(content, `xml:id="static_taygaFrame2"`) || strings.Contains(content, "<anim:") {
		t.Fatal(content)
	}
	deck, _ = motionBuildFixture(t, "test.odp", comparisonPackageFixture(t, map[string]string{"content.xml": strings.Replace(motionODPBody, `smil:type="fade"`, `smil:type="barWipe"`, 1)}))
	if deck.Frames != 1 || deck.Slides[0].Warning == "" {
		t.Fatal(deck)
	}
}
func TestMotionComparisonSessionAndRevocation(t *testing.T) {
	f := newFileFixture(t)
	original := motionPPTXFixture(t, motionPPTXTiming)
	rev := makeCompareHistory(t, f, "slides.pptx", original, original+" ")
	request := compareRequest("slides.pptx", rev.ID, "current", "motion")
	f.call(0, "POST", "/api/v1/files/library/compare", request, 503)
	f.s.cfg.Files.OfficeURL = "https://office.example.com"
	f.s.cfg.Files.OfficeProvider = "collabora"
	var launch struct {
		Mode  string
		Left  string `json:"left_timeline"`
		Right string `json:"right_timeline"`
		PDF   string `json:"left_url"`
	}
	json.Unmarshal(f.call(0, "POST", "/api/v1/files/library/compare", request, 200).Body.Bytes(), &launch)
	if launch.Mode != "motion" {
		t.Fatal(launch)
	}
	var deck motionDeck
	json.Unmarshal(f.call(-1, "GET", launch.Left, "", 200).Body.Bytes(), &deck)
	if deck.Frames != 2 {
		t.Fatal(deck)
	}
	parsed, _ := url.Parse(launch.PDF)
	endpoint := "/office/compare/content?" + parsed.RawQuery
	response := f.call(-1, "GET", endpoint, "", 200)
	reader, e := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if e != nil || len(reader.File) == 0 {
		t.Fatal(e)
	}
	f.call(-1, "POST", launch.Left, "", 405)
	f.call(-1, "GET", strings.Replace(endpoint, "&storyboard=1", "", 1), "", 403)
	f.call(1, "POST", "/api/v1/files/library/compare?owner_id="+f.users[0], request, 403)
	f.call(0, "PUT", "/api/v1/files/content?path=slides.pptx", original+"  ", 200)
	f.call(-1, "GET", launch.Left, "", 409)
	if f.call(0, "GET", "/dav/files/slides.pptx", "", 200).Body.String() != original+"  " {
		t.Fatal("original mutated")
	}
	for _, value := range []string{"NaNs", "Infinitys", "-1s", "31s"} {
		if _, e := motionMS(value, true); e == nil {
			t.Fatal(value)
		}
	}
}

// Opt-in acceptance of the generated packages with a real local office renderer.
// Normal CI stays independent of LibreOffice/Poppler installations.
func TestMotionRealOfficeRendering(t *testing.T) {
	if os.Getenv("TAYGA_TEST_OFFICE_RENDER") != "1" {
		t.Skip("set TAYGA_TEST_OFFICE_RENDER=1 for LibreOffice/Poppler acceptance")
	}
	original, e := officeTemplates.ReadFile("file_templates/presentation.pptx")
	if e != nil {
		t.Fatal(e)
	}
	archive, e := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if e != nil {
		t.Fatal(e)
	}
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	for _, entry := range archive.File {
		stream, _ := entry.Open()
		body, _ := io.ReadAll(stream)
		stream.Close()
		if entry.Name == "ppt/slides/slide1.xml" {
			body = []byte(strings.Replace(string(body), "</p:sld>", motionPPTXTiming+"</p:sld>", 1))
		}
		dst, _ := writer.Create(entry.Name)
		dst.Write(body)
	}
	writer.Close()
	dir := t.TempDir()
	profile := "file://" + filepath.Join(dir, "profile")
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		output, e := exec.CommandContext(ctx, "soffice", append([]string{"--headless", "-env:UserInstallation=" + profile}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("office: %v %s", e, output)
		}
	}
	for _, ext := range []string{"pptx", "odp"} {
		data := packed.Bytes()
		if ext == "odp" {
			os.WriteFile(filepath.Join(dir, "original.pptx"), data, 0600)
			run("--convert-to", "odp", "--outdir", dir, filepath.Join(dir, "original.pptx"))
			data, e = os.ReadFile(filepath.Join(dir, "original.odp"))
			if e != nil {
				t.Fatal(e)
			}
		}
		source, _ := os.CreateTemp(dir, "source")
		source.Write(data)
		deck, storyboard, e := buildMotionStoryboard(source, "test."+ext)
		source.Close()
		if e != nil || deck.Frames != 2 || deck.Slides[0].Warning != "" {
			t.Fatalf("%s: %+v %v", ext, deck, e)
		}
		name := filepath.Join(dir, "storyboard-"+ext+"."+ext)
		os.WriteFile(name, storyboard, 0600)
		run("--convert-to", "pdf", "--outdir", dir, name)
		pdf := filepath.Join(dir, "storyboard-"+ext+".pdf")
		prefix := filepath.Join(dir, "raster-"+ext)
		output, e := exec.Command("pdftoppm", "-scale-to", "800", "-png", pdf, prefix).CombinedOutput()
		if e != nil {
			t.Fatalf("pdftoppm: %v %s", e, output)
		}
		images, _ := filepath.Glob(prefix + "-*.png")
		if len(images) != 2 {
			t.Fatalf("%s: expected 2 rendered states, got %d", ext, len(images))
		}
		aFile, _ := os.Open(images[0])
		a, e := png.Decode(aFile)
		aFile.Close()
		if e != nil {
			t.Fatal(e)
		}
		bFile, _ := os.Open(images[1])
		b, e := png.Decode(bFile)
		bFile.Close()
		if e != nil {
			t.Fatal(e)
		}
		changed := 0
		for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
			for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
				if a.At(x, y) != b.At(x, y) {
					changed++
				}
			}
		}
		if changed < 100 {
			t.Fatalf("%s: target did not appear (%d changed pixels)", ext, changed)
		}
		t.Logf("%s: real office rendered two distinct key states (%d changed pixels)", ext, changed)
	}
}

func TestMotionConversionUsesPreparedSource(t *testing.T) {
	for _, provider := range []string{"collabora", "onlyoffice", "r7"} {
		t.Run(provider, func(t *testing.T) {
			f := newFileFixture(t)
			original := motionPPTXFixture(t, motionPPTXTiming)
			rev := makeCompareHistory(t, f, "slides.pptx", original, original+" ")
			var endpoint string
			checked := false
			check := func(body []byte) {
				t.Helper()
				reader, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
				if e != nil {
					t.Error(e)
					return
				}
				hasState := false
				for _, entry := range reader.File {
					if entry.Name == "ppt/slides/taygaMotion2.xml" {
						hasState = true
					}
				}
				if !hasState {
					t.Error("converter received original rather than storyboard")
				}
				checked = true
			}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/cool/convert-to/pdf":
					reader, e := r.MultipartReader()
					if e != nil {
						t.Error(e)
						return
					}
					part, e := reader.NextPart()
					if e != nil {
						t.Error(e)
						return
					}
					body, _ := io.ReadAll(part)
					check(body)
					w.Write([]byte("%PDF-1.4\nmotion"))
				case "/converter":
					var payload map[string]any
					json.NewDecoder(r.Body).Decode(&payload)
					address, _ := url.Parse(payload["url"].(string))
					if address.Query().Get("storyboard") != "1" {
						t.Error("missing storyboard scope")
					}
					check(f.call(-1, "GET", address.RequestURI(), "", 200).Body.Bytes())
					json.NewEncoder(w).Encode(map[string]any{"endConvert": true, "fileUrl": endpoint + "/result"})
				case "/result":
					w.Write([]byte("%PDF-1.4\nmotion"))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			endpoint = server.URL
			old := fileRenderClient
			fileRenderClient = server.Client()
			defer func() { fileRenderClient = old }()
			f.s.cfg.Files.OfficeProvider = provider
			f.s.cfg.Files.OfficeURL = server.URL
			f.s.cfg.Files.OfficeJWTSecret = documentTestSecret
			var launch map[string]string
			json.Unmarshal(f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("slides.pptx", rev.ID, "current", "motion"), 200).Body.Bytes(), &launch)
			f.call(-1, "GET", launch["left_url"], "", 200)
			if !checked {
				t.Fatal("no converted storyboard")
			}
			f.call(-1, "GET", strings.Replace(launch["left_url"], "/pdf?", "/editor?", 1), "", 403)
		})
	}
}
func TestMotionStoryboardFrameLimit(t *testing.T) {
	var steps strings.Builder
	steps.WriteString(`<p:timing><p:tnLst><p:par><p:cTn nodeType="tmRoot"><p:childTnLst>`)
	for i := 0; i < 5; i++ {
		class, mode := "entr", "in"
		if i%2 == 1 {
			class, mode = "exit", "out"
		}
		fmt.Fprintf(&steps, `<p:par><p:cTn nodeType="clickEffect" presetClass="%s" presetID="10"><p:childTnLst><p:animEffect filter="fade" transition="%s"><p:cBhvr><p:cTn dur="100"/><p:tgtEl><p:spTgt spid="2"/></p:tgtEl></p:cBhvr></p:animEffect></p:childTnLst></p:cTn></p:par>`, class, mode)
	}
	steps.WriteString(`</p:childTnLst></p:cTn></p:par></p:tnLst></p:timing>`)
	body := motionPPTXFixture(t, steps.String())
	archive, _ := zip.NewReader(strings.NewReader(body), int64(len(body)))
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	for _, entry := range archive.File {
		reader, _ := entry.Open()
		content, _ := io.ReadAll(reader)
		reader.Close()
		if entry.Name == "ppt/presentation.xml" {
			var list strings.Builder
			for i := 0; i < 25; i++ {
				fmt.Fprintf(&list, `<p:sldId id="%d" r:id="one"/>`, 256+i)
			}
			content = []byte(strings.Replace(string(content), `<p:sldId id="256" r:id="one"/>`, list.String(), 1))
		}
		out, _ := writer.Create(entry.Name)
		out.Write(content)
	}
	writer.Close()
	file, _ := os.CreateTemp(t.TempDir(), "limits")
	defer file.Close()
	file.Write(packed.Bytes())
	_, _, err := buildMotionStoryboard(file, "test.pptx")
	if err == nil || !strings.Contains(err.Error(), "120 frames") {
		t.Fatal(err)
	}
}

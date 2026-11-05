package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func motionJointTiming(space string) string {
	geometry := motionSingleBehavior(`<p:anim valueType="num" from="0" to="90">COMMON</p:anim>`, "fill.angle")
	color := motionSingleBehavior(`<p:animClr clrSpc="`+space+`">COMMON<p:to><a:srgbClr val="00FF00"/></p:to></p:animClr>`, "fillcolor")
	start := strings.Index(color, `<p:par><p:cTn presetClass=`)
	end := start + strings.Index(color[start:], `</p:par>`) + len(`</p:par>`)
	color = strings.ReplaceAll(color[start:end], "clickEffect", "withEffect")
	return strings.Replace(geometry, `</p:childTnLst></p:cTn></p:seq>`, color+`</p:childTnLst></p:cTn></p:seq>`, 1)
}

func motionODPGradientFixture(t *testing.T, property, operands, gradientStyle string, joint bool) string {
	t.Helper()
	ns := ` xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:anim="urn:oasis:names:tc:opendocument:xmlns:animation:1.0" xmlns:smil="urn:oasis:names:tc:opendocument:xmlns:smil-compatible:1.0" xmlns:presentation="urn:oasis:names:tc:opendocument:xmlns:presentation:1.0" office:version="1.3"`
	behavior := `<anim:par presentation:node-type="on-click" presentation:preset-class="emphasis" smil:begin="0s"><anim:animate smil:attributeName="` + property + `" smil:targetElement="shape" smil:dur="1s" ` + operands + `/></anim:par>`
	if joint {
		behavior += `<anim:par presentation:node-type="with-previous" presentation:preset-class="emphasis" smil:begin="0.25s"><anim:animateColor smil:attributeName="fill-color" smil:targetElement="shape" smil:dur="2s" smil:to="#00ff00" smil:calcMode="linear"/></anim:par>`
	}
	return comparisonPackageFixture(t, map[string]string{
		"mimetype":              "application/vnd.oasis.opendocument.presentation",
		"META-INF/manifest.xml": `<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.3"><manifest:file-entry manifest:full-path="/" manifest:media-type="application/vnd.oasis.opendocument.presentation"/><manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/><manifest:file-entry manifest:full-path="styles.xml" manifest:media-type="text/xml"/></manifest:manifest>`,
		"content.xml":           `<office:document-content` + ns + `><office:automatic-styles><style:style style:name="gr" style:family="graphic"><style:graphic-properties draw:fill="gradient" draw:fill-gradient-name="G" draw:stroke="solid" svg:stroke-color="#000000"/><style:text-properties fo:color="#000000" fo:font-size="18pt"/></style:style></office:automatic-styles><office:body><office:presentation><draw:page draw:name="Gradient" draw:master-page-name="Default"><draw:rect xml:id="shape" draw:id="shape" draw:style-name="gr" svg:x="2cm" svg:y="2cm" svg:width="10cm" svg:height="5cm"><text:p>TEXT</text:p></draw:rect><anim:par><anim:seq>` + behavior + `</anim:seq></anim:par></draw:page></office:presentation></office:body></office:document-content>`,
		"styles.xml":            `<office:document-styles` + ns + `><office:styles><draw:gradient draw:name="G" draw:style="` + gradientStyle + `" draw:start-color="#ff0000" draw:end-color="#0000ff" draw:start-intensity="100%" draw:end-intensity="100%" draw:angle="0" draw:cx="50%" draw:cy="50%"/></office:styles><office:automatic-styles><style:page-layout style:name="PM"><style:page-layout-properties fo:page-width="20cm" fo:page-height="10cm"/></style:page-layout></office:automatic-styles><office:master-styles><style:master-page style:name="Default" style:page-layout-name="PM"/></office:master-styles></office:document-styles>`,
	})
}

func TestMotionJointGradientPaint(t *testing.T) {
	for _, space := range []string{"rgb", "hsl"} {
		deck, parts := motionBuildFixture(t, "joint.pptx", string(motionBehaviorFixture(t, motionRichColorFixture(t), motionJointTiming(space))))
		slide := deck.Slides[0]
		if slide.Warning != "" || len(slide.Colors) != 1 || slide.Colors[0].GeometryEffect == nil || *slide.Colors[0].GeometryEffect != 0 {
			t.Fatal(deck)
		}
		color := slide.Colors[0]
		expected := 18
		if space == "hsl" {
			expected = 45
		}
		if len(color.Samples) != expected || deck.Frames > 120 {
			t.Fatal(deck)
		}
		// Off-diagonal states prove geometry and color are independently sampled.
		for _, sample := range color.Samples {
			body := parts[fmt.Sprintf("ppt/slides/taygaMotion%d.xml", sample.White)]
			if sample.GeometryAt == 1 && sample.At == 0 && (!strings.Contains(body, `ang="16200000"`) || !strings.Contains(body, `val="FF0000"`) || !strings.Contains(body, `val="0000FF"`)) {
				t.Fatal(body)
			}
			if sample.GeometryAt == 0 && sample.At == 1 && (!strings.Contains(body, `ang="0"`) || !strings.Contains(body, `val="00FF00"`)) {
				t.Fatal(body)
			}
		}
	}
}

func TestMotionODPGradientGeometry(t *testing.T) {
	for _, item := range []struct{ property, operands, style, fragment string }{
		{"tayga:gradient-angle", `smil:from="0" smil:to="90"`, "linear", `draw:angle="900.0000"`},
		{"tayga:gradient-center-x", `smil:from="0.5" smil:by="-0.25"`, "radial", `draw:cx="25.0000%"`},
		{"tayga:gradient-center-y", `smil:to="0.75" smil:calcMode="discrete"`, "rectangular", `draw:cy="75.0000%"`},
	} {
		for _, joint := range []bool{false, true} {
			deck, parts := motionBuildFixture(t, "gradient.odp", motionODPGradientFixture(t, item.property, item.operands, item.style, joint))
			slide := deck.Slides[0]
			if slide.Warning != "" || len(slide.Colors) != 1 || (slide.Colors[0].GeometryEffect != nil) != joint {
				t.Fatal(deck)
			}
			if !strings.Contains(parts["styles.xml"], item.fragment) || !strings.Contains(parts["content.xml"], "TEXT") {
				t.Fatal("geometry or text missing", parts)
			}
			// Shared definition stays unchanged; all new paints have local styles.
			if !strings.Contains(parts["styles.xml"], `draw:name="G" draw:style="`+item.style+`" draw:start-color="#ff0000"`) {
				t.Fatal("source gradient changed")
			}
		}
	}
	for _, item := range []struct{ property, operands, style string }{
		{"tayga:gradient-angle", `smil:to="NaN"`, "linear"},
		{"tayga:gradient-angle", `smil:to="90" anim:formula="$"`, "linear"},
		{"tayga:gradient-angle", `smil:to="90" smil:values="0;90"`, "linear"},
		{"tayga:gradient-angle", `smil:to="361"`, "linear"},
		{"tayga:gradient-angle", `smil:from="45" smil:to="90"`, "linear"},
		{"tayga:gradient-center-x", `smil:by="0.75"`, "radial"},
		{"tayga:gradient-center-x", `smil:to="0.25"`, "linear"},
		{"fill.angle", `smil:to="90"`, "linear"},
	} {
		deck, _ := motionBuildFixture(t, "bad.odp", motionODPGradientFixture(t, item.property, item.operands, item.style, false))
		if deck.Slides[0].Warning == "" || len(deck.Slides[0].Effects) != 0 {
			t.Fatal("unsupported geometry accepted", item, deck)
		}
	}
}

func TestMotionJointGradientRealOffice(t *testing.T) {
	if os.Getenv("TAYGA_TEST_OFFICE_RENDER") != "1" {
		t.Skip("requires local LibreOffice/Poppler")
	}
	dir := t.TempDir()
	for _, kind := range []string{"joint-pptx", "angle-odp", "focus-odp", "joint-odp", "size-x-pptx", "size-y-pptx", "size-circle-pptx", "joint-size-pptx", "stops-pptx", "stops-odp", "joint-stops-pptx", "joint-stops-odp"} {
		ext, data := "odp", motionODPGradientFixture(t, "tayga:gradient-angle", `smil:to="90"`, "linear", kind == "joint-odp")
		if kind == "focus-odp" {
			data = motionODPGradientFixture(t, "tayga:gradient-center-x", `smil:to="0.25"`, "radial", false)
		}
		if kind == "joint-pptx" {
			ext = "pptx"
			data = string(motionBehaviorFixture(t, motionRichColorFixture(t), motionJointTiming("rgb")))
		}
		if kind == "size-x-pptx" || kind == "size-y-pptx" || kind == "size-circle-pptx" || kind == "joint-size-pptx" {
			ext = "pptx"
			axis := "x"
			if kind == "size-y-pptx" {
				axis = "y"
			}
			data = string(motionFocusSizeFixture(t, "fill.focussize."+axis, `from="0.2" to="0.6"`, kind == "joint-size-pptx"))
		}
		if kind == "size-circle-pptx" {
			data = string(motionPatchPart(t, []byte(data), "ppt/slides/slide1.xml", func(body []byte) []byte {
				return bytes.Replace(body, []byte(`path="rect"`), []byte(`path="circle"`), 1)
			}))
		}
		if strings.Contains(kind, "stops-") {
			ext = "pptx"
			operands := `to="0.2,0.8"`
			if strings.HasSuffix(kind, "odp") {
				ext = "odp"
				operands = `smil:to="0.2,0.8"`
			}
			data = string(motionStopsFixture(t, ext, operands, strings.HasPrefix(kind, "joint-")))
		}
		source, _ := os.CreateTemp(dir, "source")
		source.WriteString(data)
		deck, prepared, err := buildMotionStoryboard(source, "test."+ext)
		source.Close()
		if err != nil || deck.Slides[0].Warning != "" {
			t.Fatal(kind, deck, err)
		}
		file := filepath.Join(dir, kind+"."+ext)
		if err = os.WriteFile(file, prepared, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		out, err := exec.CommandContext(ctx, "soffice", "--headless", "-env:UserInstallation=file://"+filepath.Join(dir, "profile"), "--convert-to", "pdf", "--outdir", dir, file).CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("%s %v %s", kind, err, out)
		}
		palette := deck.Slides[0].Colors[0]
		frames := []int{palette.FromWhite, palette.ToWhite}
		if palette.GeometryEffect != nil {
			frames = []int{palette.Samples[0].White, palette.Samples[1].White, palette.Samples[len(palette.Samples)-2].White, palette.ToWhite}
		}
		images := [][]byte{}
		for i, frame := range frames {
			prefix := filepath.Join(dir, fmt.Sprintf("%s-%d", kind, i))
			out, err := exec.Command("pdftoppm", "-f", fmt.Sprint(frame), "-l", fmt.Sprint(frame), "-scale-to", "600", "-singlefile", "-png", filepath.Join(dir, kind+".pdf"), prefix).CombinedOutput()
			if err != nil {
				t.Fatalf("%s %v %s", kind, err, out)
			}
			f, err := os.Open(prefix + ".png")
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(f)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			var pixels bytes.Buffer
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					pixels.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8)})
				}
			}
			for j, prior := range images {
				// A uniform destination color has no visible gradient direction.
				if palette.GeometryEffect != nil && j == 1 && i == 3 {
					continue
				}
				if bytes.Equal(prior, pixels.Bytes()) {
					t.Fatal(kind, "independent paint states rendered identically")
				}
			}
			images = append(images, pixels.Bytes())
		}
		t.Log(kind, "rendered independent geometry/color states", deck.Frames, "pages")
	}
}

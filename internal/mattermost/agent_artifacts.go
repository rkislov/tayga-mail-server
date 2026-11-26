package mattermost

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/tayga/tms/internal/storage"
)

type agentArtifact struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ExpiresAt int64  `json:"expires_at"`
	mime      string
	data      []byte
}

var agentArtifactTypes = map[string]string{
	"txt":  "text/plain; charset=utf-8",
	"md":   "text/markdown; charset=utf-8",
	"csv":  "text/csv; charset=utf-8",
	"svg":  "image/svg+xml",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

func (f *agentFiles) create(name, format, content string) (any, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	media, ok := agentArtifactTypes[format]
	if len(f.outputs) >= 3 || !ok || strings.TrimSpace(content) == "" || len(content) > 24000 {
		return nil, errors.New("нужны поддерживаемый формат и содержимое до 24 КБ, максимум 3 файла")
	}
	name, err := agentArtifactName(name, format)
	if err != nil {
		return nil, err
	}
	data := []byte(content)
	switch format {
	case "docx":
		data, err = agentDOCX(content)
	case "pptx":
		data, err = agentPPTX(content, f.images)
	case "svg":
		err = validateAgentSVG(content)
	}
	if err != nil {
		return nil, err
	}
	return f.save(name, media, data), nil
}

func agentArtifactName(name, format string) (string, error) {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, name))
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "document"
	}
	if len(name) > 160 {
		return "", errors.New("слишком длинное имя файла")
	}
	if !strings.HasSuffix(strings.ToLower(name), "."+format) {
		name += "." + format
	}
	return name, nil
}

func (f *agentFiles) save(name, mediaType string, data []byte) agentArtifact {
	file := agentArtifact{ID: agentID(), Name: name, ExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli(), mime: mediaType, data: data}
	f.outputs = append(f.outputs, file)
	return file
}

func validateAgentSVG(content string) error {
	elements := strings.Fields("svg g title desc defs marker path rect circle ellipse line polyline polygon text tspan")
	attributes := strings.Fields("viewBox width height x y x1 y1 x2 y2 cx cy r rx ry d points transform fill fill-opacity stroke stroke-width stroke-opacity stroke-linecap stroke-linejoin stroke-dasharray opacity font-size font-family font-weight text-anchor dominant-baseline id markerWidth markerHeight refX refY orient marker-end marker-start")
	contains := func(values []string, value string) bool {
		for _, item := range values {
			if item == value {
				return true
			}
		}
		return false
	}
	decoder := xml.NewDecoder(strings.NewReader(content))
	root, depth := false, 0
	invalid := errors.New("SVG должен содержать только статические геометрические фигуры и текст")
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return invalid
		}
		switch value := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				if root || value.Name.Local != "svg" || value.Name.Space != "http://www.w3.org/2000/svg" {
					return invalid
				}
				root = true
			}
			if depth >= 32 || value.Name.Space != "http://www.w3.org/2000/svg" || !contains(elements, value.Name.Local) {
				return invalid
			}
			for _, attr := range value.Attr {
				if attr.Name.Space == "" && attr.Name.Local == "xmlns" && attr.Value == "http://www.w3.org/2000/svg" {
					continue
				}
				if attr.Name.Space != "" || !contains(attributes, attr.Name.Local) {
					return invalid
				}
				v := strings.ToLower(strings.TrimSpace(attr.Value))
				if strings.ContainsAny(v, "\\") || strings.Contains(v, "/*") ||
					strings.Contains(v, "url") && !((attr.Name.Local == "marker-end" || attr.Name.Local == "marker-start") && strings.HasPrefix(v, "url(#") && strings.HasSuffix(v, ")") && strings.Count(v, "(") == 1) {
					return invalid
				}
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(value)) != "" {
				return invalid
			}
		case xml.Comment:
		default:
			return invalid
		}
	}
	if !root || depth != 0 {
		return invalid
	}
	return nil
}

func (s *API) serveAgentArtifact(w http.ResponseWriter, r *http.Request, u *storage.User, id string) {
	if r.Method != http.MethodGet {
		wrongMethod(w)
		return
	}
	item, err := s.store.GetAgentArtifact(r.Context(), id, u.ID, userScope(u).Key(), time.Now().UnixMilli())
	if err != nil {
		s.failure(w, err)
		return
	}
	if item.MIME == agentArtifactTypes["pptx"] {
		item.Data, err = repairLegacyAgentPPTX(item.Data)
		if err != nil {
			s.failure(w, err)
			return
		}
	}
	w.Header().Set("Content-Type", item.MIME)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.Name}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(item.Data)
}

const agentRelNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
const agentPackageNS = "http://schemas.openxmlformats.org/package/2006"
const agentOOXMLHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`

func agentPackage(parts map[string]string, media map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for name, body := range parts {
		if err := writeAgentXML(archive, name, body); err != nil {
			return nil, err
		}
	}
	for name, data := range media {
		part, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			return nil, err
		}
		if _, err = part.Write(data); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeAgentXML(archive *zip.Writer, name, body string) error {
	part, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(part, agentOOXMLHeader+body)
	return err
}

func agentXML(text string) string {
	var buf strings.Builder
	_ = xml.EscapeText(&buf, []byte(text))
	return buf.String()
}

func agentRels(entries string) string {
	return `<Relationships xmlns="` + agentPackageNS + `/relationships">` + entries + `</Relationships>`
}
func agentRel(id, kind, target string) string {
	return `<Relationship Id="` + id + `" Type="` + agentRelNS + `/` + kind + `" Target="` + target + `"/>`
}
func agentContentTypes(entries string) string {
	return `<Types xmlns="` + agentPackageNS + `/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>` + entries + `</Types>`
}
func agentOverride(part, kind string) string {
	return `<Override PartName="/` + part + `" ContentType="application/vnd.openxmlformats-officedocument.` + kind + `+xml"/>`
}

func agentDOCX(content string) ([]byte, error) {
	var body strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		body.WriteString(`<w:p><w:r><w:t xml:space="preserve">` + agentXML(line) + `</w:t></w:r></w:p>`)
	}
	return agentPackage(map[string]string{
		"[Content_Types].xml": agentContentTypes(agentOverride("word/document.xml", "wordprocessingml.document.main")),
		"_rels/.rels":         agentRels(agentRel("rId1", "officeDocument", "word/document.xml")),
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() +
			`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr></w:body></w:document>`,
	}, nil)
}

type agentSlide struct {
	Title   string   `json:"title"`
	Lines   []string `json:"lines"`
	ImageID string   `json:"image_id,omitempty"`
}

const agentPPTNS = ` xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="` + agentRelNS + `"`
const agentPPTGroup = `<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>`
const agentPPTColors = `<p:clrMap accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" bg1="lt1" bg2="lt2" folHlink="folHlink" hlink="hlink" tx1="dk1" tx2="dk2"/>`

// Office theme from Tayga's bundled presentation template, without the XML declaration.
//
//go:embed agent_pptx_theme.xml
var agentPPTXTheme string

const agentSlideMasterRels = "ppt/slideMasters/_rels/slideMaster1.xml.rels"
const agentPPTXThemePart = "ppt/theme/theme1.xml"

func agentMasterRelationships() string {
	return agentRels(agentRel("rId1", "slideLayout", "../slideLayouts/slideLayout1.xml") +
		agentRel("rId2", "theme", "../theme/theme1.xml"))
}

func agentPPTX(content string, images map[string]agentGeneratedImage) ([]byte, error) {
	var slides []agentSlide
	if json.Unmarshal([]byte(content), &slides) != nil || len(slides) < 1 || len(slides) > 20 {
		return nil, errors.New("для PPTX передайте массив из 1–20 слайдов с полями title и lines в JSON")
	}
	parts := map[string]string{
		"_rels/.rels": agentRels(agentRel("rId1", "officeDocument", "ppt/presentation.xml")),
		"ppt/slideMasters/slideMaster1.xml": `<p:sldMaster` + agentPPTNS + `><p:cSld><p:spTree>` + agentPPTGroup + `</p:spTree></p:cSld>` +
			agentPPTColors + `<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst></p:sldMaster>`,
		agentSlideMasterRels:                           agentMasterRelationships(),
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout` + agentPPTNS + ` type="blank"><p:cSld><p:spTree>` + agentPPTGroup + `</p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": agentRels(agentRel("rId1", "slideMaster", "../slideMasters/slideMaster1.xml")),
		agentPPTXThemePart:                             agentPPTXTheme,
	}
	types := agentOverride("ppt/presentation.xml", "presentationml.presentation.main") +
		agentOverride("ppt/slideMasters/slideMaster1.xml", "presentationml.slideMaster") +
		agentOverride("ppt/slideLayouts/slideLayout1.xml", "presentationml.slideLayout") +
		agentOverride(agentPPTXThemePart, "theme")
	rels := agentRel("rId1", "slideMaster", "slideMasters/slideMaster1.xml")
	media := map[string][]byte{}
	imageNames := map[string]string{}
	var ids strings.Builder
	for i, item := range slides {
		if strings.TrimSpace(item.Title) == "" || len([]rune(item.Title)) > 100 || len(item.Lines) > 10 {
			return nil, errors.New("у каждого слайда нужен заголовок до 100 символов и до 10 строк")
		}
		for _, line := range item.Lines {
			if len([]rune(line)) > 240 {
				return nil, errors.New("строка на слайде длиннее 240 символов")
			}
		}
		name := fmt.Sprintf("slide%d.xml", i+1)
		id := fmt.Sprintf("rId%d", i+2)
		types += agentOverride("ppt/slides/"+name, "presentationml.slide")
		rels += agentRel(id, "slide", "slides/"+name)
		fmt.Fprintf(&ids, `<p:sldId id="%d" r:id="%s"/>`, 256+i, id)
		bodyWidth := 10912000
		picture := ""
		slideRels := agentRel("rId1", "slideLayout", "../slideLayouts/slideLayout1.xml")
		if item.ImageID != "" {
			image, ok := images[item.ImageID]
			if !ok {
				return nil, errors.New("image_id должен указывать на изображение, созданное в этом запросе")
			}
			mediaName := imageNames[item.ImageID]
			if mediaName == "" {
				mediaName = fmt.Sprintf("image%d.png", len(media)+1)
				imageNames[item.ImageID] = mediaName
				media["ppt/media/"+mediaName] = image.data
			}
			slideRels += agentRel("rId2", "image", "../media/"+mediaName)
			bodyWidth = 6080000
			picture = agentSlidePicture(image)
		}
		parts["ppt/slides/"+name] = `<p:sld` + agentPPTNS + `><p:cSld><p:spTree>` + agentPPTGroup +
			agentSlideText(2, 480000, 10912000, 1050000, 3200, "345d30", []string{item.Title}) +
			agentSlideText(3, 1750000, bodyWidth, 4700000, 2100, "202821", item.Lines) + picture +
			`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`
		parts["ppt/slides/_rels/"+name+".rels"] = agentRels(slideRels)
	}
	if len(media) != 0 {
		types = `<Default Extension="png" ContentType="image/png"/>` + types
	}
	parts["[Content_Types].xml"] = agentContentTypes(types)
	parts["ppt/_rels/presentation.xml.rels"] = agentRels(rels)
	parts["ppt/presentation.xml"] = `<p:presentation` + agentPPTNS +
		`><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst><p:sldIdLst>` + ids.String() +
		`</p:sldIdLst><p:sldSz cx="12192000" cy="6858000" type="screen16x9"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`
	return agentPackage(parts, media)
}

func agentSlidePicture(image agentGeneratedImage) string {
	const left, top, boxWidth, boxHeight int64 = 6960000, 1750000, 4552000, 4700000
	width, height := boxWidth, boxHeight
	if boxWidth*int64(image.height) > boxHeight*int64(image.width) {
		width = height * int64(image.width) / int64(image.height)
	} else {
		height = width * int64(image.height) / int64(image.width)
	}
	return fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="4" name="Illustration" descr="%s"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId2"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`,
		agentXML(image.name), left+(boxWidth-width)/2, top+(boxHeight-height)/2, width, height)
}

func agentSlideText(id, top, width, height, size int, color string, lines []string) string {
	var text strings.Builder
	fmt.Fprintf(&text, `<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Text %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="640000" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/></p:spPr><p:txBody><a:bodyPr wrap="square"><a:normAutofit/></a:bodyPr><a:lstStyle/>`, id, id, top, width, height)
	if len(lines) == 0 {
		text.WriteString(`<a:p/>`)
	}
	for _, line := range lines {
		fmt.Fprintf(&text, `<a:p><a:r><a:rPr sz="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:latin typeface="Arial"/></a:rPr><a:t>%s</a:t></a:r></a:p>`, size, color, agentXML(line))
	}
	text.WriteString(`</p:txBody></p:sp>`)
	return text.String()
}

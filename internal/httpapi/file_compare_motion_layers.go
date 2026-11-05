package httpapi

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type motionPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type motionLayer struct {
	Target string `json:"target"`
	Black  int    `json:"black"`
	White  int    `json:"white"`
	Hidden bool   `json:"hidden"`
}
type motionHit struct {
	Target   string     `json:"target"`
	Name     string     `json:"name"`
	Box      [4]float64 `json:"box"`
	Rotation float64    `json:"rotation,omitempty"`
}
type motionLayerPlan struct {
	keep           map[string]bool
	matte          string
	target         string
	probe          bool
	base           bool
	colorEffect    int
	colorPhase     int
	geometryEffect int
	geometryPhase  int
	paintTracks    [][]int
	paintPoints    [][]motionTrackPoint
	paintAxes      []int
	paintPhases    []int
}

func parseMotionPath(value string) ([]motionPoint, error) {
	if len(value) > 8192 {
		return nil, fmt.Errorf("motion path too long")
	}
	// A strict lexer: never pass path strings to SVG, CSS, or JavaScript execution.
	number := regexp.MustCompile(`[-+]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][-+]?[0-9]+)?`)
	tokens := []string{}
	for pos := 0; pos < len(value); {
		c := value[pos]
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == ',' {
			pos++
			continue
		}
		if strings.ContainsRune("MLCZEmlcz", rune(c)) {
			tokens = append(tokens, string(c))
			pos++
			continue
		}
		loc := number.FindStringIndex(value[pos:])
		if loc == nil || loc[0] != 0 {
			return nil, fmt.Errorf("unsupported motion path command")
		}
		tokens = append(tokens, value[pos:pos+loc[1]])
		pos += loc[1]
		if len(tokens) > 2048 {
			return nil, fmt.Errorf("motion path complexity")
		}
	}
	points := []motionPoint{}
	at := motionPoint{}
	command := ""
	first := true
	for i := 0; i < len(tokens); {
		token := tokens[i]
		if len(token) == 1 && strings.Contains("MLCZEmlcz", token) {
			command = token
			i++
			if token == "E" {
				if i != len(tokens) {
					return nil, fmt.Errorf("data after path end")
				}
				break
			}
			if token == "Z" || token == "z" {
				if len(points) == 0 {
					return nil, fmt.Errorf("invalid closed path")
				}
				points = append(points, points[0])
				at = points[0]
				command = ""
				continue
			}
		}
		count := 2
		if command == "C" || command == "c" {
			count = 6
		}
		if command == "" || i+count > len(tokens) {
			return nil, fmt.Errorf("invalid motion path")
		}
		values := make([]float64, count)
		for j := range values {
			v, e := strconv.ParseFloat(tokens[i+j], 64)
			if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 4 {
				return nil, fmt.Errorf("motion path coordinate out of range")
			}
			values[j] = v
		}
		i += count
		relative := command == strings.ToLower(command)
		coords := make([]motionPoint, count/2)
		for j := range coords {
			coords[j] = motionPoint{values[j*2], values[j*2+1]}
			if relative {
				coords[j].X += at.X
				coords[j].Y += at.Y
			}
		}
		switch strings.ToUpper(command) {
		case "M":
			if !first {
				return nil, fmt.Errorf("multiple path subpaths unsupported")
			}
			if math.Abs(coords[0].X) > 0.00001 || math.Abs(coords[0].Y) > 0.00001 {
				return nil, fmt.Errorf("path must start at object origin")
			}
			at = motionPoint{}
			points = append(points, at)
			command = map[bool]string{true: "l", false: "L"}[relative]
			first = false
		case "L":
			if first {
				return nil, fmt.Errorf("path must start with M")
			}
			at = coords[0]
			points = append(points, at)
		case "C":
			if first {
				return nil, fmt.Errorf("path must start with M")
			}
			start := at
			for step := 1; step <= 24; step++ {
				t := float64(step) / 24
				u := 1 - t
				points = append(points, motionPoint{u*u*u*start.X + 3*u*u*t*coords[0].X + 3*u*t*t*coords[1].X + t*t*t*coords[2].X, u*u*u*start.Y + 3*u*u*t*coords[0].Y + 3*u*t*t*coords[1].Y + t*t*t*coords[2].Y})
			}
			at = coords[2]
		default:
			return nil, fmt.Errorf("unsupported path")
		}
		if len(points) > 512 {
			return nil, fmt.Errorf("motion path exceeds 512 points")
		}
	}
	for _, p := range points {
		if math.Abs(p.X) > 16 || math.Abs(p.Y) > 16 {
			return nil, fmt.Errorf("motion path leaves supported range")
		}
	}
	if len(points) < 2 {
		return nil, fmt.Errorf("motion path requires movement")
	}
	return points, nil
}
func motionAdvanced(slide *motionSlide) bool {
	for _, e := range slide.Effects {
		if e.Repeat > 1 || e.AutoReverse || e.Previous != nil || e.Kind == "path" || motionTransformKind(e.Kind) || e.Trigger == "with" || e.Event != "" {
			return true
		}
	}
	return false
}
func motionParents(root *compareNode) map[*compareNode]*compareNode {
	out := map[*compareNode]*compareNode{}
	var walk func(*compareNode)
	walk = func(n *compareNode) {
		if n == nil {
			return
		}
		for _, c := range n.children {
			out[c] = n
			walk(c)
		}
	}
	walk(root)
	return out
}
func motionPPTXEvent(node *compareNode) (string, error) {
	conditions := node.child("stCondLst").all("cond")
	if len(conditions) != 1 {
		return "", fmt.Errorf("interactive sequence requires one click condition")
	}
	cond := conditions[0]
	target := cond.child("tgtEl").child("spTgt")
	if cond.attr("evt") != "onClick" || target == nil || target.attr("spid") == "" || len(target.children) > 0 {
		return "", fmt.Errorf("unsupported interactive event")
	}
	if cond.attr("delay") != "" && cond.attr("delay") != "0" {
		return "", fmt.Errorf("interactive container delay")
	}
	return target.attr("spid"), nil
}
func motionRootAttr(data []byte, local, value, qualified, namespace string) []byte {
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, e := d.Token()
		if e != nil {
			return data
		}
		if _, ok := token.(xml.StartElement); ok {
			break
		}
	}
	end := int(d.InputOffset())
	head := string(data[:end])
	pattern := regexp.MustCompile(`\s+(?:[A-Za-z_][A-Za-z0-9_.-]*:)?` + regexp.QuoteMeta(local) + `\s*=\s*(?:"[^"]*"|'[^']*')`)
	replacement := " " + qualified + `="` + motionXMLAttr(value) + `"`
	if loc := pattern.FindStringIndex(head); loc != nil {
		old := head[loc[0]:loc[1]]
		name := strings.TrimSpace(strings.SplitN(old, "=", 2)[0])
		replacement = " " + name + `="` + motionXMLAttr(value) + `"`
		head = head[:loc[0]] + replacement + head[loc[1]:]
	} else {
		if namespace != "" {
			prefix := strings.Split(qualified, ":")[0]
			declaration := regexp.MustCompile(`\sxmlns:` + regexp.QuoteMeta(prefix) + `\s*=\s*["']([^"']*)["']`).FindStringSubmatch(head)
			if declaration != nil && declaration[1] != namespace {
				for i := 0; ; i++ {
					prefix = fmt.Sprintf("tgMotion%d", i)
					if !strings.Contains(head, "xmlns:"+prefix+"=") {
						break
					}
				}
				local := qualified[strings.Index(qualified, ":")+1:]
				replacement = " " + prefix + ":" + local + `="` + motionXMLAttr(value) + `"`
				declaration = nil
			}
			if declaration == nil {
				replacement = ` xmlns:` + prefix + `="` + namespace + `"` + replacement
			}
		}
		at := strings.LastIndex(head, ">")
		if at > 0 && head[at-1] == '/' {
			at--
		}
		head = head[:at] + replacement + head[at:]
	}
	return append([]byte(head), data[end:]...)
}
func motionLayerPlans(slide *motionSlide, ids []string) []motionLayerPlan {
	targets := map[string]bool{}
	for _, e := range slide.Effects {
		targets[e.Target] = true
	}
	plans := []motionLayerPlan{{base: true}, {matte: "000000", probe: true}, {matte: "FFFFFF", probe: true}}
	group := map[string]bool{}
	flush := func(target string) {
		if len(group) == 0 {
			return
		}
		plans = append(plans, motionLayerPlan{keep: group, matte: "000000", target: target}, motionLayerPlan{keep: group, matte: "FFFFFF", target: target})
		group = map[string]bool{}
	}
	for _, id := range ids {
		if targets[id] {
			flush("")
			group[id] = true
			flush(id)
		} else {
			group[id] = true
		}
	}
	flush("")
	multi := motionMultiPaintGroups(slide)
	pairs := motionJointPaints(slide)
	consumed := map[int]bool{}
	for _, tracks := range motionTrackGroups(slide) {
		points, err := motionTrackPoints(slide, tracks)
		if err != nil {
			continue
		}
		axes := []int{}
		indices := [][]int{}
		for i, track := range tracks {
			axes = append(axes, track[0])
			values := []int{}
			for j := range points[i] {
				values = append(values, j+1)
			}
			indices = append(indices, values)
			for _, index := range track {
				consumed[index] = true
				delete(pairs, index)
			}
		}
		motionWalkPhases(indices, func(values []int) {
			for _, matte := range []string{"000000", "FFFFFF"} {
				plans = append(plans, motionLayerPlan{keep: map[string]bool{slide.Effects[axes[0]].Target: true}, target: slide.Effects[axes[0]].Target, matte: matte, paintAxes: axes, paintTracks: tracks, paintPoints: points, paintPhases: append([]int{}, values...)})
			}
		})
	}

	for _, axes := range multi {
		if consumed[axes[0]] {
			continue
		}
		for _, axis := range axes {
			consumed[axis] = true
			delete(pairs, axis)
		}
		phases, err := motionMultiPhases(slide, axes)
		if err != nil {
			continue
		} // Validation records this before planning.
		motionWalkPhases(phases, func(values []int) {
			for _, matte := range []string{"000000", "FFFFFF"} {
				plans = append(plans, motionLayerPlan{keep: map[string]bool{slide.Effects[axes[0]].Target: true}, matte: matte, target: slide.Effects[axes[0]].Target, paintAxes: axes, paintPhases: append([]int{}, values...)})
			}
		})
	}
	for _, geometry := range pairs {
		consumed[geometry] = true
	}
	for i, e := range slide.Effects {
		if (e.Kind != "color" && e.Kind != "gradient") || consumed[i] {
			continue
		}
		geometry, joint := pairs[i]
		geometryPhases := []int{0}
		if joint {
			geometryPhases = motionJointGeometryPhases(e, slide.Effects[geometry])
		}
		for _, gp := range geometryPhases {
			for phase := 1; phase <= motionSamples(e); phase++ {
				for _, matte := range []string{"000000", "FFFFFF"} {
					plans = append(plans, motionLayerPlan{keep: map[string]bool{e.Target: true}, matte: matte, target: e.Target, colorEffect: i, colorPhase: phase, geometryEffect: geometry, geometryPhase: gp})
				}
			}
		}
	}
	return plans
}
func motionPPTXMatte(body []byte, color string) []byte {
	body = motionRootAttr(body, "showMasterSp", "0", "showMasterSp", "")
	spans, _ := motionSpans(body, "cSld", "bg")
	body = motionEdit(body, spans, nil)
	decoder := xml.NewDecoder(bytes.NewReader(body))
	for {
		token, e := decoder.Token()
		if e != nil {
			return body
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "cSld" {
			pos := int(decoder.InputOffset())
			bg := `<p:bg xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:bgPr><a:solidFill><a:srgbClr val="` + color + `"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>`
			return append(append(append([]byte{}, body[:pos]...), []byte(bg)...), body[pos:]...)
		}
	}
}
func motionRegisterLayer(slide *motionSlide, plan motionLayerPlan, frame int) {
	if len(plan.paintAxes) > 0 {
		motionRegisterMultiPaint(slide, plan, frame)
		return
	}
	if plan.colorPhase > 0 {
		if plan.colorPhase == 1 && plan.geometryPhase <= 1 && plan.matte == "000000" {
			layer := motionColorLayer{Effect: plan.colorEffect}
			if plan.geometryPhase > 0 {
				value := plan.geometryEffect
				layer.GeometryEffect = &value
			}
			slide.Colors = append(slide.Colors, layer)
		}
		color := &slide.Colors[len(slide.Colors)-1]
		count := motionSamples(slide.Effects[plan.colorEffect])
		if plan.colorPhase == 1 && plan.geometryPhase <= 1 {
			if plan.matte == "000000" {
				color.FromBlack = frame
			} else {
				color.FromWhite = frame
			}
		}
		if plan.colorPhase == count && (plan.geometryPhase == 0 || plan.geometryPhase == motionSamples(slide.Effects[plan.geometryEffect])) {
			if plan.matte == "000000" {
				color.ToBlack = frame
			} else {
				color.ToWhite = frame
			}
		}
		if count > 2 || plan.geometryPhase > 0 {
			if plan.matte == "000000" {
				sample := motionColorSample{At: float64(plan.colorPhase-1) / float64(count-1), Black: frame}
				if plan.geometryPhase > 0 {
					sample.GeometryAt = float64(plan.geometryPhase-1) / float64(motionSamples(slide.Effects[plan.geometryEffect])-1)
				}
				color.Samples = append(color.Samples, sample)
			} else {
				color.Samples[len(color.Samples)-1].White = frame
			}
		}

		return
	}
	if plan.base {
		slide.Background = frame
		return
	}
	if plan.probe {
		if plan.matte == "000000" {
			slide.Probe[0] = frame
		} else {
			slide.Probe[1] = frame
		}
		return
	}
	if plan.matte == "000000" {
		hidden := false
		for _, e := range slide.Effects {
			if e.Target == plan.target && !motionAppearanceKind(e.Kind) {
				hidden = !e.Exit
				break
			}
		}
		slide.Layers = append(slide.Layers, motionLayer{Target: plan.target, Black: frame, Hidden: hidden})
	} else {
		slide.Layers[len(slide.Layers)-1].White = frame
	}
}

func motionValidateAdvanced(slide *motionSlide, ids map[string]bool) {
	if slide.Warning != "" || !motionAdvanced(slide) {
		return
	}
	type rangeItem struct {
		start, end      int
		event, property string
	}
	ranges := map[string][]rangeItem{}
	type cursor struct{ start, end, max, count int }
	groups := map[string]cursor{}
	for _, effect := range slide.Effects {
		group := groups[effect.Event]
		start := 0
		switch effect.Trigger {
		case "with":
			start = group.start + effect.Delay
		case "after":
			start = group.end + effect.Delay
		default:
			start = group.max + 500 + effect.Delay
			if effect.Event != "" && group.count == 0 {
				start = effect.Delay
			}
		}
		end := start + motionActiveDuration(effect)
		property := "opacity"
		if effect.Kind == "path" {
			property = "position"
		}
		if motionTransformKind(effect.Kind) {
			property = effect.Kind
			if effect.Kind == "color" || effect.Kind == "gradient" {
				property = effect.Kind + "/" + effect.Property
			}
		}
		for _, prior := range ranges[effect.Target] {
			if prior.property == property && (prior.event != effect.Event || start < prior.end) {
				slide.Warning = "Stages of one property require ordered non-overlapping timing on one clock; static slide"
			}

		}
		ranges[effect.Target] = append(ranges[effect.Target], rangeItem{start, end, effect.Event, property})
		group.start = start
		group.end = end
		if end > group.max {
			group.max = end
		}
		group.count++
		groups[effect.Event] = group
	}
	motionValidateTrackGrids(slide)
	motionValidateGradientGroups(slide)
	clicks := map[string]int{}
	for _, effect := range slide.Effects {
		if effect.Event != "" && !ids[effect.Event] {
			slide.Warning = "Unsupported interactive target; static slide"
		}
		if effect.Event != "" && effect.Trigger == "click" {
			clicks[effect.Event]++
			if clicks[effect.Event] > 1 {
				slide.Warning = "Multi-click interactive sequence unsupported; static slide"
			}
		}

	}
}
func motionPPTXHits(slide *motionSlide, shapes []*compareNode, width, height float64) {
	if width <= 0 || height <= 0 {
		return
	}
	for _, shape := range shapes {
		props := shape.descendants("cNvPr")
		if len(props) == 0 {
			continue
		}
		id := props[0].attr("id")
		xfrm := shape.child("spPr").child("xfrm")
		if xfrm == nil {
			xfrm = shape.child("xfrm")
		}
		box := [4]float64{}
		values := []string{xfrm.child("off").attr("x"), xfrm.child("off").attr("y"), xfrm.child("ext").attr("cx"), xfrm.child("ext").attr("cy")}
		valid := true
		for i, v := range values {
			n, e := strconv.ParseFloat(v, 64)
			if e != nil || math.IsNaN(n) || math.IsInf(n, 0) {
				valid = false
			}
			box[i] = n / []float64{width, height, width, height}[i]
		}
		if valid && motionValidBox(box) {
			if id != "" && len(slide.Hits) < 1000 {
				slide.Hits = append(slide.Hits, motionHit{Target: id, Name: props[0].attr("name"), Box: box, Rotation: math.Mod(motionFloat(xfrm.attr("rot"))/60000, 360)})
			}
		}
	}
}
func motionDefaultHidden(slide *motionSlide, initial map[string]bool) map[string]bool {
	result := map[string]bool{}
	for id, v := range initial {
		result[id] = v
	}
	for _, e := range slide.Effects {
		if e.Event == "" && !motionAppearanceKind(e.Kind) {
			result[e.Target] = e.Exit
		}
	}
	return result
}
func motionPPTXFinal(body []byte, slide *motionSlide, width, height float64) []byte {
	spans, _ := motionSpans(body, "spTree", "sp", "pic", "graphicFrame", "cxnSp", "grpSp")
	replacements := make([][]byte, len(spans))
	for i, span := range spans {
		fragment := body[span.start:span.end]
		for _, hit := range slide.Hits {
			for _, e := range slide.Effects {
				if e.Target != hit.Target || e.Kind != "path" || e.Event != "" {
					continue
				}
				decoder := xml.NewDecoder(bytes.NewReader(fragment))
				id := ""
				for {
					token, err := decoder.Token()
					if err != nil {
						break
					}
					if start, ok := token.(xml.StartElement); ok && start.Name.Local == "cNvPr" {
						for _, a := range start.Attr {
							if a.Name.Local == "id" {
								id = a.Value
							}
						}
						break
					}
				}
				if id != hit.Target {
					continue
				}
				off, _ := motionSpans(fragment, "xfrm", "off")
				if len(off) != 1 {
					continue
				}
				point := e.Path[len(e.Path)-1]
				if e.AutoReverse {
					point = e.Path[0]
				}

				changed := motionRootAttr(fragment[off[0].start:off[0].end], "x", strconv.FormatInt(int64(math.Round((hit.Box[0]+point.X)*width)), 10), "x", "")
				changed = motionRootAttr(changed, "y", strconv.FormatInt(int64(math.Round((hit.Box[1]+point.Y)*height)), 10), "y", "")
				fragment = motionEdit(fragment, off, [][]byte{changed})
			}
		}
		replacements[i] = fragment
	}
	return motionEdit(body, spans, replacements)
}

func motionFloat(value string) float64 {
	v, e := strconv.ParseFloat(value, 64)
	if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func motionLength(value string) float64 {
	for unit, scale := range map[string]float64{"cm": 1, "mm": .1, "in": 2.54, "pt": 2.54 / 72, "px": 2.54 / 96} {
		if strings.HasSuffix(value, unit) {
			return motionFloat(strings.TrimSuffix(value, unit)) * scale
		}
	}
	return 0
}
func motionODPSize(pkg *comparePackage, page *compareNode) (float64, float64) {
	root, e := pkg.xml("styles.xml")
	if e != nil {
		return 0, 0
	}
	layout := ""
	motionWalk(root, func(n *compareNode) {
		if n.name.Local == "master-page" && n.attr("name") == page.attr("master-page-name") {
			layout = n.attr("page-layout-name")
		}
	})
	width, height := 0.0, 0.0
	motionWalk(root, func(n *compareNode) {
		if n.name.Local == "page-layout" && n.attr("name") == layout {
			props := n.child("page-layout-properties")
			width = motionLength(props.attr("page-width"))
			height = motionLength(props.attr("page-height"))
		}
	})
	return width, height
}
func motionODPStyles(content []byte, additions ...string) ([]byte, error) {
	extra := ""
	for _, color := range []string{"000000", "FFFFFF"} {
		extra += `<style:style xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:presentation="urn:oasis:names:tc:opendocument:xmlns:presentation:1.0" style:name="_TaygaMotion` + color + `" style:family="drawing-page"><style:drawing-page-properties draw:fill="solid" draw:fill-color="#` + color + `" presentation:background-visible="true" presentation:background-objects-visible="false"/></style:style>`
	}
	for _, addition := range additions {
		extra += addition
	}
	spans, e := motionSpans(content, "document-content", "automatic-styles")
	if e != nil {
		return nil, e
	}
	if len(spans) == 1 {
		node := content[spans[0].start:spans[0].end]
		if bytes.Contains(node, []byte("_TaygaMotion")) || bytes.Contains(node, []byte("_TaygaColor")) {
			return nil, fmt.Errorf("reserved layer style collision")
		}
		modified, e := motionAppendRoot(node, "automatic-styles", extra)
		if e != nil {
			return nil, e
		}
		return motionEdit(content, spans, [][]byte{modified}), nil
	}
	body, e := motionSpans(content, "document-content", "body")
	if e != nil || len(body) != 1 {
		return nil, fmt.Errorf("invalid ODP body")
	}
	replacement := []byte(`<office:automatic-styles xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0">` + extra + `</office:automatic-styles>`)
	replacement = append(replacement, content[body[0].start:body[0].end]...)
	return motionEdit(content, body, [][]byte{replacement}), nil
}
func motionODPFinal(body []byte, slide *motionSlide, width, height float64) []byte {
	spans, _ := motionSpans(body, "page", "frame", "g", "custom-shape", "rect", "ellipse", "line", "connector", "path", "polygon", "polyline", "circle", "caption")
	replacements := make([][]byte, len(spans))
	for i, span := range spans {
		fragment := body[span.start:span.end]
		id := ""
		for _, a := range span.attrs {
			if a.Name.Local == "id" {
				id = a.Value
				break
			}
		}
		for _, hit := range slide.Hits {
			for _, e := range slide.Effects {
				if e.Target != id || hit.Target != id || e.Kind != "path" || e.Event != "" {
					continue
				}
				point := e.Path[len(e.Path)-1]
				if e.AutoReverse {
					point = e.Path[0]
				}

				fragment = motionRootAttr(fragment, "x", strconv.FormatFloat((hit.Box[0]+point.X)*width, 'f', 6, 64)+"cm", "tgSvg:x", "urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0")
				fragment = motionRootAttr(fragment, "y", strconv.FormatFloat((hit.Box[1]+point.Y)*height, 'f', 6, 64)+"cm", "tgSvg:y", "urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0")
			}
		}
		replacements[i] = fragment
	}
	return motionEdit(body, spans, replacements)
}

var motionODPEventPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_.-]{0,255})\.click(?:\+([0-9.]+(?:ms|s)))?$`)

func motionODPEvent(value string) (string, int, error) {
	match := motionODPEventPattern.FindStringSubmatch(value)
	if match == nil {
		return "", 0, fmt.Errorf("unsupported interactive event")
	}
	delay, e := motionMS(match[2], true)
	return match[1], delay, e
}

func motionValidBox(box [4]float64) bool {
	for _, v := range box {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 16 {
			return false
		}
	}
	return box[2] > 0 && box[3] > 0
}
func motionCheckPathGeometry(slide *motionSlide) {
	for _, e := range slide.Effects {
		if !motionAppearanceKind(e.Kind) {
			continue
		}
		for _, hit := range slide.Hits {
			if hit.Target == e.Target && (hit.Box[0] < 0 || hit.Box[1] < 0 || hit.Box[0]+hit.Box[2] > 1 || hit.Box[1]+hit.Box[3] > 1) {
				slide.Warning = "Motion object starts outside slide bounds; static slide"
			}
		}
	}
}

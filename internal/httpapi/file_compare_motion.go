package httpapi

// Motion comparison renders immutable key states through the configured office
// engine. Only explicitly supported timing is replayed; unsupported slides are
// marked and stay static. No document scripts or macros are executed by the UI.
import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const motionMaxFrames = 120
const motionMaxEffects = 12

type motionEffect struct {
	Kind           string        `json:"kind"`
	Target         string        `json:"target"`
	Exit           bool          `json:"exit"`
	Duration       int           `json:"duration"`
	Trigger        string        `json:"trigger"`
	Delay          int           `json:"delay"`
	Event          string        `json:"event,omitempty"`
	Path           []motionPoint `json:"path,omitempty"`
	From           []float64     `json:"from,omitempty"`
	To             []float64     `json:"to,omitempty"`
	Discrete       bool          `json:"discrete,omitempty"`
	Property       string        `json:"property,omitempty"`
	Space          string        `json:"space,omitempty"`
	Direction      string        `json:"direction,omitempty"`
	Relative       bool          `json:"relative,omitempty"`
	Delta          []float64     `json:"delta,omitempty"`
	DeltaSpace     string        `json:"delta_space,omitempty"`
	byRef          *compareNode
	focusRect      [4]float64
	focusPath      string
	focusStops     []float64
	fromRef        *compareNode
	toRef          *compareNode
	gradient       bool
	sourceColors   [][]float64
	sourceStyles   map[string][]float64
	wholeTextRange bool
	textRangeEnd   int
}
type motionTransition struct {
	Kind      string `json:"kind"`
	Direction string `json:"direction"`
	Duration  int    `json:"duration"`
}
type motionSlide struct {
	Title      string             `json:"title"`
	States     []int              `json:"states"`
	Effects    []motionEffect     `json:"effects"`
	Transition motionTransition   `json:"transition"`
	Warning    string             `json:"warning,omitempty"`
	Layers     []motionLayer      `json:"layers,omitempty"`
	Background int                `json:"background,omitempty"`
	Probe      [2]int             `json:"probe"`
	Hits       []motionHit        `json:"hits,omitempty"`
	Colors     []motionColorLayer `json:"colors,omitempty"`
}
type motionDeck struct {
	Slides []motionSlide `json:"slides"`
	Frames int           `json:"frames"`
}
type motionSpan struct {
	start, end int
	name       string
	attrs      []xml.Attr
}

// Byte ranges preserve original prefixes, extension markup, and inherited namespaces.
func motionSpans(data []byte, parent string, names ...string) ([]motionSpan, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	stack := []motionSpan{}
	out := []motionSpan{}
	for {
		start := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid motion XML")
		}
		switch n := token.(type) {
		case xml.StartElement:
			stack = append(stack, motionSpan{start: start, name: n.Name.Local, attrs: n.Attr})
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("invalid XML nesting")
			}
			item := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			item.end = int(decoder.InputOffset())
			if parent == "" || len(stack) > 0 && stack[len(stack)-1].name == parent {
				for _, name := range names {
					if item.name == name {
						out = append(out, item)
						break
					}
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out, nil
}
func motionEdit(data []byte, spans []motionSpan, replacement [][]byte) []byte {
	var out bytes.Buffer
	offset := 0
	for i, item := range spans {
		if item.start < offset {
			continue
		}
		out.Write(data[offset:item.start])
		if replacement != nil {
			out.Write(replacement[i])
		}
		offset = item.end
	}
	out.Write(data[offset:])
	return out.Bytes()
}
func motionWalk(n *compareNode, visit func(*compareNode)) {
	if n == nil {
		return
	}
	visit(n)
	for _, child := range n.children {
		motionWalk(child, visit)
	}
}
func motionMS(value string, seconds bool) (int, error) {
	if value == "" {
		return 0, nil
	}
	number := value
	if seconds {
		if strings.HasSuffix(value, "ms") {
			number = strings.TrimSuffix(value, "ms")
		} else if strings.HasSuffix(value, "s") {
			v, e := strconv.ParseFloat(strings.TrimSuffix(value, "s"), 64)
			if e != nil || v < 0 || v > 30 || v != v {
				return 0, fmt.Errorf("unsupported duration")
			}
			return int(v * 1000), nil
		} else {
			return 0, fmt.Errorf("unsupported duration")
		}
	}
	v, e := strconv.ParseFloat(number, 64)
	if e != nil || v < 0 || v > 30000 || v != v {
		return 0, fmt.Errorf("unsupported duration")
	}
	return int(v), nil
}
func motionPPTXEffects(slide *compareNode) ([]motionEffect, error) {
	timing := slide.child("timing")
	if timing == nil {
		return []motionEffect{}, nil
	}
	effects := []motionEffect{}
	var failure error
	covered := map[*compareNode]bool{}
	parents := motionParents(timing)
	motionWalk(timing, func(node *compareNode) {
		if failure != nil || node.name.Local != "cTn" || node.attr("nodeType") == "" || node.attr("nodeType") == "tmRoot" || node.attr("nodeType") == "mainSeq" || node.attr("nodeType") == "interactiveSeq" {
			return
		}
		trigger := node.attr("nodeType")
		if trigger != "clickEffect" && trigger != "afterEffect" && trigger != "withEffect" {
			failure = fmt.Errorf("parallel or interactive timing")
			return
		}
		class := node.attr("presetClass")
		if class != "entr" && class != "exit" && class != "path" && class != "emph" {
			failure = fmt.Errorf("motion/emphasis effect")
			return
		}
		item := motionEffect{Kind: "cut", Exit: class == "exit", Trigger: trigger}
		if trigger == "clickEffect" {
			item.Trigger = "click"
		} else {
			item.Trigger = "after"
			if trigger == "withEffect" {
				item.Trigger = "with"
			}
		}
		for ancestor := parents[node]; ancestor != nil; ancestor = parents[ancestor] {
			if ancestor.attr("nodeType") == "interactiveSeq" {
				event, e := motionPPTXEvent(ancestor)
				if e != nil {
					failure = e
					return
				}
				item.Event = event
				break
			}
		}
		for _, cond := range node.child("stCondLst").all("cond") {
			if cond.attr("evt") == "onClick" {
				event, e := motionPPTXEvent(node)
				if e != nil {
					failure = e
					return
				}
				item.Event = event
				continue
			}
			if len(cond.children) > 0 || cond.attr("evt") != "" {
				failure = fmt.Errorf("interactive trigger")
				return
			}
			delay, err := motionMS(cond.attr("delay"), false)
			if err != nil {
				failure = err
				return
			}
			item.Delay = delay
		}
		targets := map[string]bool{}
		behaviors := 0
		motionWalk(node, func(n *compareNode) {
			covered[n] = true
			for _, a := range n.attrs {
				switch a.Name.Local {
				case "autoRev":
					if a.Value != "0" && a.Value != "false" {
						failure = fmt.Errorf("reversing timing")
					}
				case "accel", "decel":
					if a.Value != "0" {
						failure = fmt.Errorf("eased timing")
					}
				case "spd":
					if a.Value != "100000" {
						failure = fmt.Errorf("nonlinear speed")
					}
				case "repeatCount":
					if a.Value != "1000" {
						failure = fmt.Errorf("repeating timing")
					}
				case "repeatDur":
					failure = fmt.Errorf("repeating timing")
				}
			}
			switch n.name.Local {
			case "spTgt":
				if len(n.children) > 0 {
					if item.Kind == "color" && item.Property == "text" && len(n.children) == 1 && n.child("txEl") != nil && len(n.child("txEl").children) == 1 && n.child("txEl").child("pRg") != nil {
						r := n.child("txEl").child("pRg")
						end, e := strconv.Atoi(r.attr("end"))
						if r.attr("st") != "0" || e != nil || end < 0 {
							failure = fmt.Errorf("partial text target unsupported")
						} else {
							item.wholeTextRange = true
							item.textRangeEnd = end
						}
					} else if !((item.Kind == "color" && item.Property == "fill" || item.Kind == "gradient") && len(n.children) == 1 && n.child("bg") != nil) {
						failure = fmt.Errorf("text/sub-object target")
					}
				}
				targets[n.attr("spid")] = true
			case "animEffect":
				behaviors++
				if n.attr("filter") != "fade" || n.attr("transition") != map[bool]string{false: "in", true: "out"}[item.Exit] {
					failure = fmt.Errorf("unsupported animation filter")
				}
				item.Kind = "fade"
				duration, err := motionMS(n.child("cBhvr").child("cTn").attr("dur"), false)
				if err != nil || duration == 0 {
					failure = fmt.Errorf("missing animation duration")
				} else {
					item.Duration = duration
				}
			case "set":
				if n.child("cBhvr").child("attrNameLst").text() != "style.visibility" {
					failure = fmt.Errorf("unsupported set behavior")
				}
				value := n.child("to").child("strVal").attr("val")
				if value != "visible" && value != "hidden" {
					failure = fmt.Errorf("unsupported visibility")
				}
			case "animMotion":
				behaviors++
				item.Kind = "path"
				if class != "path" || (n.attr("origin") != "" && n.attr("origin") != "layout") || (n.attr("rAng") != "" && n.attr("rAng") != "0") || len(n.all("from"))+len(n.all("to"))+len(n.all("by")) > 0 {
					failure = fmt.Errorf("unsupported path origin/rotation")
				}
				var e error
				item.Path, e = parseMotionPath(n.attr("path"))
				if e != nil {
					failure = e
				}
				item.Duration, e = motionMS(n.child("cBhvr").child("cTn").attr("dur"), false)
				if e != nil || item.Duration == 0 {
					failure = fmt.Errorf("missing motion duration")
				}
			case "animRot", "animScale", "animClr":
				behaviors++
				if class != "emph" {
					failure = fmt.Errorf("transform must be emphasis effect")
				}
				if e := motionPPTXTransform(n, &item); e != nil {
					failure = e
				}
			case "anim":
				behaviors++
				if class != "emph" {
					failure = fmt.Errorf("gradient geometry must be emphasis")
				}
				if e := motionGradientAnimation(n, &item); e != nil {
					failure = e
				}
			case "cmd", "audio", "video", "iterate":
				failure = fmt.Errorf("unsupported animation behavior")
			}
		})
		if len(targets) != 1 || targets[""] || behaviors > 1 || (behaviors == 0 && node.attr("presetID") != "1") {
			failure = fmt.Errorf("unsupported animation target/preset")
			return
		}
		for target := range targets {
			item.Target = target
		}
		effects = append(effects, item)
	})
	motionWalk(timing, func(n *compareNode) {
		if !covered[n] && n.name.Local == "cTn" && n.attr("nodeType") != "interactiveSeq" {
			for _, cond := range n.child("stCondLst").all("cond") {
				delay := cond.attr("delay")
				if (delay != "" && delay != "0" && delay != "indefinite") || cond.attr("evt") != "" || len(cond.children) > 0 {
					failure = fmt.Errorf("unsupported container timing")
				}
			}
		}
		switch n.name.Local {
		case "set", "animEffect", "anim", "animClr", "animMotion", "animRot", "animScale", "cmd", "audio", "video", "iterate":
			if !covered[n] {
				failure = fmt.Errorf("animation outside supported sequence")
			}
		}
	})
	if len(effects) > motionMaxEffects {
		return nil, fmt.Errorf("more than 12 effects on slide")
	}
	return effects, failure
}
func motionPPTXTransition(slide *compareNode) (motionTransition, error) {
	result := motionTransition{Kind: "cut"}
	node := slide.child("transition")
	if node == nil {
		return result, nil
	}
	result.Duration = map[string]int{"fast": 500, "med": 1000, "slow": 2000}[node.attr("spd")]
	if result.Duration == 0 {
		result.Duration = 500
	}
	if value := node.attr("dur"); value != "" {
		n, e := motionMS(value, false)
		if e != nil {
			return result, e
		}
		result.Duration = n
	}
	if len(node.children) == 0 {
		return result, nil
	}
	if len(node.children) != 1 {
		return result, fmt.Errorf("transition sound/extensions")
	}
	child := node.children[0]
	result.Kind = child.name.Local
	switch result.Kind {
	case "fade":
		if child.attr("thruBlk") == "1" || child.attr("thruBlk") == "true" {
			return result, fmt.Errorf("fade through black")
		}
	case "wipe", "push":
		result.Direction = child.attr("dir")
		if result.Direction == "" {
			result.Direction = "l"
		}
		if !strings.Contains("lrud", result.Direction) || len(result.Direction) != 1 {
			return result, fmt.Errorf("transition direction")
		}
	case "cut":
		result.Duration = 0
	default:
		return result, fmt.Errorf("unsupported slide transition")
	}
	return result, nil
}

func buildMotionStoryboard(file *os.File, name string) (motionDeck, []byte, error) {
	pkg, err := openComparePackage(file)
	if err != nil {
		return motionDeck{}, nil, err
	}
	if strings.EqualFold(path.Ext(name), ".odp") {
		return buildMotionODP(pkg)
	}
	if !strings.EqualFold(path.Ext(name), ".pptx") {
		return motionDeck{}, nil, fmt.Errorf("motion comparison supports PPTX/ODP")
	}
	return buildMotionPPTX(pkg)
}
func motionStates(slide *motionSlide, ids map[string]bool) []map[string]bool {
	hidden := map[string]bool{}
	seen := map[string]bool{}
	for _, effect := range slide.Effects {
		if !ids[effect.Target] {
			slide.Warning = "Unsupported animation target; static slide"
			slide.Effects = []motionEffect{}
			break
		}
		if !seen[effect.Target] && !motionAppearanceKind(effect.Kind) {
			hidden[effect.Target] = !effect.Exit
			seen[effect.Target] = true
		}
	}
	if slide.Warning != "" {
		slide.Transition = motionTransition{Kind: "cut"}
		slide.Effects = []motionEffect{}
		hidden = map[string]bool{}
	}
	clone := func() map[string]bool {
		out := map[string]bool{}
		for id, value := range hidden {
			out[id] = value
		}
		return out
	}
	states := []map[string]bool{clone()}
	for _, effect := range slide.Effects {
		if motionAppearanceKind(effect.Kind) {
			states = append(states, clone())
			continue
		}
		if hidden[effect.Target] == effect.Exit {
			slide.Warning = "Unsupported visibility sequence; static slide"
			slide.Transition = motionTransition{Kind: "cut"}
			slide.Effects = []motionEffect{}
			return []map[string]bool{{}}
		}
		hidden[effect.Target] = effect.Exit
		states = append(states, clone())
	}
	return states
}
func motionZIP(pkg *comparePackage, changes map[string][]byte) ([]byte, error) {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	names := make([]string, 0, len(pkg.files)+len(changes))
	for name := range pkg.files {
		names = append(names, name)
	}
	for name := range changes {
		if pkg.files[name] == nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var expanded uint64
	for _, name := range names {
		if strings.HasPrefix(name, "_xmlsignatures/") {
			continue
		}
		body, changed := changes[name]
		size := uint64(len(body))
		if !changed {
			size = pkg.files[name].UncompressedSize64
		}
		if size > (64<<20)-expanded {
			return nil, fmt.Errorf("storyboard expansion exceeds 64 MiB")
		}
		expanded += size
		if !changed {
			// Validate even unused copied resources; do not pass a forged ZIP size/CRC to the office engine.
			stream, e := pkg.files[name].Open()
			if e != nil {
				return nil, e
			}
			count, e := io.Copy(io.Discard, io.LimitReader(stream, int64(size)+1))
			stream.Close()
			if e != nil || uint64(count) != size {
				return nil, fmt.Errorf("invalid storyboard package resource")
			}
		}
		if changed {
			stream, e := writer.Create(name)
			if e != nil {
				return nil, e
			}
			if _, e = stream.Write(body); e != nil {
				return nil, e
			}
		} else {
			if e := writer.Copy(pkg.files[name]); e != nil {
				return nil, e
			}
		}
		if data.Len() > maxCompareFileBytes {
			return nil, fmt.Errorf("storyboard exceeds 32 MiB")
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if data.Len() > maxCompareFileBytes {
		return nil, fmt.Errorf("storyboard exceeds 32 MiB")
	}
	return data.Bytes(), nil
}
func buildMotionPPTX(pkg *comparePackage) (motionDeck, []byte, error) {
	deck := motionDeck{}
	presentation, err := pkg.xml("ppt/presentation.xml")
	if err != nil {
		return deck, nil, err
	}
	rels, err := pkg.relationships("ppt/presentation.xml")
	if err != nil {
		return deck, nil, err
	}
	slides := presentation.child("sldIdLst").all("sldId")
	if len(slides) == 0 || len(slides) > 50 {
		return deck, nil, fmt.Errorf("motion comparison requires 1–50 slides")
	}
	changes := map[string][]byte{}
	generatedBytes := 0
	var list, links, types strings.Builder
	for index, item := range slides {
		rel, ok := rels[item.relationshipID()]
		if !ok || rel.external || !strings.HasSuffix(rel.kind, "/slide") {
			return deck, nil, fmt.Errorf("invalid slide relationship")
		}
		if path.Dir(rel.target) != "ppt/slides" {
			return deck, nil, fmt.Errorf("nonstandard slide location unsupported")
		}
		node, e := pkg.xml(rel.target)
		if e != nil {
			return deck, nil, e
		}
		body, e := pkg.read(rel.target)
		if e != nil {
			return deck, nil, e
		}
		slide := motionSlide{Title: node.child("cSld").attr("name"), Effects: []motionEffect{}}
		if slide.Title == "" {
			slide.Title = fmt.Sprintf("Slide %d", index+1)
		}
		slide.Effects, e = motionPPTXEffects(node)
		if e != nil {
			slide.Warning = "Unsupported animation: " + e.Error() + "; static slide"
		}
		slide.Transition, e = motionPPTXTransition(node)
		if e != nil {
			slide.Transition = motionTransition{Kind: "cut"}
			slide.Warning = "Unsupported transition: " + e.Error() + "; static slide"
		}
		ids := map[string]bool{}
		shapes := node.child("cSld").child("spTree").children
		spans, e := motionSpans(body, "spTree", "sp", "pic", "graphicFrame", "cxnSp", "grpSp")
		if e != nil {
			return deck, nil, e
		}
		shapeIDs := []string{}
		for _, shape := range shapes {
			switch shape.name.Local {
			case "sp", "pic", "graphicFrame", "cxnSp", "grpSp":
				props := shape.descendants("cNvPr")
				id := ""
				if len(props) > 0 {
					id = props[0].attr("id")
				}
				if id != "" && ids[id] {
					slide.Warning = "Duplicate object ID; static slide"
				}
				ids[id] = true
				key := id
				if key == "" {
					key = fmt.Sprintf("\x00static:%d", len(shapeIDs))
				}
				shapeIDs = append(shapeIDs, key)
			}
		}
		if len(spans) != len(shapeIDs) {
			return deck, nil, fmt.Errorf("invalid shape tree")
		}
		stripped, e := motionSpans(body, "sld", "timing", "transition")
		if e != nil {
			return deck, nil, e
		}
		motionWalk(node, func(n *compareNode) {
			switch n.name.Local {
			case "videoFile", "audioFile", "wavAudioFile", "media", "oleObj", "control":
				slide.Warning = "Unsupported media or interactive content; static slide"
			}
		})
		motionPPTXHits(&slide, shapes, motionFloat(presentation.child("sldSz").attr("cx")), motionFloat(presentation.child("sldSz").attr("cy")))
		motionPreparePPTXColor(pkg, rel.target, node, &slide, shapes)
		motionPrepareGradient(pkg, rel.target, node, &slide, shapes)
		motionCheckPathGeometry(&slide)
		motionValidateAdvanced(&slide, ids)
		for _, effect := range slide.Effects {
			if effect.Kind == "path" || motionTransformKind(effect.Kind) {
				found := false
				for _, hit := range slide.Hits {
					if hit.Target == effect.Target {
						found = true
					}
				}
				if !found {
					slide.Warning = "Motion requires explicit object geometry; static slide"
				}
			}
		}
		states := motionStates(&slide, ids)
		plans := []motionLayerPlan{}
		if motionAdvanced(&slide) && slide.Warning == "" {
			plans = motionLayerPlans(&slide, shapeIDs)
		}
		for stateIndex := 0; stateIndex < len(states)+len(plans); stateIndex++ {
			hidden := map[string]bool{}
			var plan *motionLayerPlan
			if stateIndex < len(states) {
				hidden = states[stateIndex]
				if len(plans) > 0 && stateIndex == len(states)-1 {
					hidden = motionDefaultHidden(&slide, states[0])
				}
			} else {
				plan = &plans[stateIndex-len(states)]
				for _, id := range shapeIDs {
					hidden[id] = !plan.keep[id]
				}
			}

			deck.Frames++
			if deck.Frames > motionMaxFrames {
				return deck, nil, fmt.Errorf("storyboard exceeds 120 frames")
			}
			removals := append([]motionSpan{}, stripped...)
			for i, span := range spans {
				if hidden[shapeIDs[i]] {
					removals = append(removals, span)
				}
			}
			sort.Slice(removals, func(i, j int) bool { return removals[i].start < removals[j].start })
			part := fmt.Sprintf("ppt/slides/taygaMotion%d.xml", deck.Frames)
			if pkg.files[part] != nil {
				return deck, nil, fmt.Errorf("reserved storyboard part collision")
			}
			changes[part] = motionEdit(body, removals, nil)
			if len(plans) > 0 && stateIndex == len(states)-1 {
				changes[part] = motionPPTXFinal(changes[part], &slide, motionFloat(presentation.child("sldSz").attr("cx")), motionFloat(presentation.child("sldSz").attr("cy")))
			}
			if plan != nil && plan.matte != "" {
				if plan.colorPhase > 0 {
					effect := slide.Effects[plan.colorEffect]
					var modified []byte
					var e error
					if effect.Kind == "gradient" {
						modified, e = motionPPTXGradientBody(changes[part], effect, plan.colorPhase)
					} else {
						modified, e = motionPPTXPaintBody(changes[part], effect, plan.colorPhase)
					}
					if e != nil {
						return deck, nil, e
					}
					changes[part] = modified
					if plan.geometryPhase > 0 {
						changes[part], e = motionPPTXGradientBody(changes[part], slide.Effects[plan.geometryEffect], plan.geometryPhase)
						if e != nil {
							return deck, nil, e
						}
					}
				}
				changes[part] = motionPPTXMatte(changes[part], plan.matte)
			}

			focusAssets := []motionFocusAsset{}
			for effectIndex, effect := range slide.Effects {
				if effect.Property != "focus-width" && effect.Property != "focus-height" {
					continue
				}
				phase := 1
				var paint *motionEffect
				paintPhase := 1
				if plan != nil {
					if plan.geometryPhase > 0 && plan.geometryEffect == effectIndex {
						phase = plan.geometryPhase
					}
					if plan.colorPhase > 0 {
						current := &slide.Effects[plan.colorEffect]
						if current.Kind == "gradient" && plan.colorEffect == effectIndex {
							phase = plan.colorPhase
						}
						if current.Kind == "color" && current.Property == "fill" && current.Target == effect.Target {
							paint, paintPhase = current, plan.colorPhase
						}
					}
				}
				asset := motionFocusAsset{Name: fmt.Sprintf("ppt/media/taygaMotionFocus%d_%d.png", deck.Frames, effectIndex), ID: fmt.Sprintf("taygaFocusPaint%d", effectIndex)}
				var err error
				changes[part], asset.PNG, err = motionPPTXFocusPaint(changes[part], effect, phase, paint, paintPhase, asset.ID)
				if err != nil {
					return deck, nil, err
				}
				if len(asset.PNG) > 0 {
					if pkg.files[asset.Name] != nil {
						return deck, nil, fmt.Errorf("focus paint asset collision")
					}
					changes[asset.Name] = asset.PNG
					generatedBytes += len(asset.PNG)
					focusAssets = append(focusAssets, asset)
					fmt.Fprintf(&types, `<Override xmlns="http://schemas.openxmlformats.org/package/2006/content-types" PartName="/%s" ContentType="image/png"/>`, asset.Name)
				}
			}

			generatedBytes += len(changes[part])
			if generatedBytes > 32<<20 {
				return deck, nil, fmt.Errorf("storyboard XML exceeds 32 MiB")
			}
			originalRels := path.Join(path.Dir(rel.target), "_rels", path.Base(rel.target)+".rels")
			if pkg.files[originalRels] != nil {
				data, e := pkg.read(originalRels)
				if e != nil {
					return deck, nil, e
				}
				generatedBytes += len(data)
				if generatedBytes > 32<<20 {
					return deck, nil, fmt.Errorf("storyboard XML exceeds 32 MiB")
				}
				changes[path.Join("ppt/slides/_rels", path.Base(part)+".rels")] = data
			}
			if len(focusAssets) > 0 {
				relPart := path.Join("ppt/slides/_rels", path.Base(part)+".rels")
				data := changes[relPart]
				if len(data) == 0 {
					data = []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`)
				}
				for _, asset := range focusAssets {
					if strings.Contains(string(data), asset.ID) {
						return deck, nil, fmt.Errorf("focus paint relationship collision")
					}
					extra := fmt.Sprintf(`<Relationship xmlns="http://schemas.openxmlformats.org/package/2006/relationships" Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/%s"/>`, asset.ID, path.Base(asset.Name))
					var err error
					data, err = motionAppendRoot(data, "Relationships", extra)
					if err != nil {
						return deck, nil, err
					}
					generatedBytes += len(extra)
				}
				if generatedBytes > 32<<20 {
					return deck, nil, fmt.Errorf("storyboard exceeds 32 MiB")
				}
				changes[relPart] = data
			}

			id := fmt.Sprintf("taygaMotion%d", deck.Frames)
			if _, exists := rels[id]; exists {
				return deck, nil, fmt.Errorf("storyboard relationship collision")
			}
			fmt.Fprintf(&list, `<p:sldId xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" id="%d" r:id="%s"/>`, 255+deck.Frames, id)
			fmt.Fprintf(&links, `<Relationship xmlns="http://schemas.openxmlformats.org/package/2006/relationships" Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/%s"/>`, id, path.Base(part))
			fmt.Fprintf(&types, `<Override xmlns="http://schemas.openxmlformats.org/package/2006/content-types" PartName="/%s" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, part)
			if plan == nil {
				slide.States = append(slide.States, deck.Frames)
			} else {
				motionRegisterLayer(&slide, *plan, deck.Frames)
			}
		}
		deck.Slides = append(deck.Slides, slide)
	}
	source, _ := pkg.read("ppt/presentation.xml")
	spans, e := motionSpans(source, "presentation", "sldIdLst")
	if e != nil || len(spans) != 1 {
		return deck, nil, fmt.Errorf("invalid slide list")
	}
	changes["ppt/presentation.xml"] = motionEdit(source, spans, [][]byte{[]byte(`<p:sldIdLst xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` + list.String() + `</p:sldIdLst>`)})
	for _, entry := range []struct{ name, closing, addition string }{{"ppt/_rels/presentation.xml.rels", "Relationships", links.String()}, {"[Content_Types].xml", "Types", types.String()}} {
		source, e := pkg.read(entry.name)
		if e != nil {
			return deck, nil, e
		}
		modified, e := motionAppendRoot(source, entry.closing, entry.addition)
		if e != nil {
			return deck, nil, e
		}
		changes[entry.name] = modified
	}
	data, e := motionZIP(pkg, changes)
	return deck, data, e
}
func motionAppendRoot(data []byte, root, extra string) ([]byte, error) {
	// Insert before the actual closing tag, preserving a producer's namespace prefix.
	decoder := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	rootStart, rootEnd := 0, 0
	for {
		offset := int(decoder.InputOffset())
		token, e := decoder.Token()
		if e != nil {
			return nil, fmt.Errorf("invalid package root")
		}
		switch n := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				rootStart = offset
				rootEnd = int(decoder.InputOffset())
			}
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				if n.Name.Local != root {
					return nil, fmt.Errorf("invalid package root")
				}
				if offset == rootEnd && rootEnd >= 2 && string(data[rootEnd-2:rootEnd]) == "/>" {
					opening := string(data[rootStart:rootEnd])
					name := strings.FieldsFunc(strings.TrimPrefix(opening, "<"), func(r rune) bool { return r == ' ' || r == '\n' || r == '\r' || r == '\t' || r == '/' || r == '>' })[0]
					result := append([]byte{}, data[:rootEnd-2]...)
					result = append(result, []byte(">"+extra+"</"+name+">")...)
					return append(result, data[rootEnd:]...), nil
				}
				return append(append(append([]byte{}, data[:offset]...), []byte(extra)...), data[offset:]...), nil
			}
		}
	}
}

var motionIDAttribute = regexp.MustCompile(`((?:[A-Za-z_][A-Za-z0-9_.-]*:)?(?:id|start-shape|end-shape|href)\s*=\s*)(["'])([^"']*)(["'])`)

var motionBuildSlots = make(chan struct{}, 2)
var errMotionBusy = errors.New("storyboard builder busy; retry")

func motionCompareFormat(name string) bool {
	return strings.EqualFold(path.Ext(name), ".pptx") || strings.EqualFold(path.Ext(name), ".odp")
}
func prepareMotionSource(source *os.File, name string, materialize bool) (motionDeck, *os.File, func(), error) {
	select {
	case motionBuildSlots <- struct{}{}:
		defer func() { <-motionBuildSlots }()
	default:
		return motionDeck{}, nil, func() {}, errMotionBusy
	}
	deck, data, err := buildMotionStoryboard(source, name)
	if err != nil {
		return deck, nil, func() {}, err
	}
	if !materialize {
		return deck, nil, func() {}, nil
	}
	temp, err := os.CreateTemp("", "tayga-motion-*")
	if err != nil {
		return deck, nil, func() {}, err
	}
	cleanup := func() { temp.Close(); os.Remove(temp.Name()) }
	if _, err = temp.Write(data); err != nil {
		cleanup()
		return deck, nil, func() {}, err
	}
	if _, err = temp.Seek(0, 0); err != nil {
		cleanup()
		return deck, nil, func() {}, err
	}
	return deck, temp, cleanup, nil
}

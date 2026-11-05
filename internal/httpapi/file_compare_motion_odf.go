package httpapi

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

func motionODPEffects(page *compareNode) ([]motionEffect, error) {
	effects := []motionEffect{}
	var failure error
	covered := map[*compareNode]bool{}
	parents := motionParents(page)
	motionWalk(page, func(node *compareNode) {
		if failure != nil {
			return
		}
		if node.name.Local == "animations" {
			failure = fmt.Errorf("legacy animation")
			return
		}
		if node.name.Local != "par" || node.attr("preset-class") == "" {
			return
		}
		class := node.attr("preset-class")
		if class != "entrance" && class != "exit" && class != "motion-path" && class != "emphasis" {
			failure = fmt.Errorf("motion/emphasis effect")
			return
		}
		trigger := node.attr("node-type")
		if trigger != "on-click" && trigger != "after-previous" && trigger != "with-previous" {
			failure = fmt.Errorf("parallel/interactive timing")
			return
		}
		item := motionEffect{Kind: "cut", Exit: class == "exit", Trigger: "click"}
		if trigger == "with-previous" {
			item.Trigger = "with"
		}
		if trigger == "after-previous" {
			item.Trigger = "after"
		}
		for ancestor := parents[node]; ancestor != nil && ancestor != page; ancestor = parents[ancestor] {
			if strings.Contains(ancestor.attr("begin"), ".click") {
				event, delay, e := motionODPEvent(ancestor.attr("begin"))
				if e != nil || delay != 0 {
					failure = fmt.Errorf("unsupported interactive container delay")
					return
				}
				item.Event = event
				break
			}
		}
		if begin := node.attr("begin"); begin != "" && begin != "next" {
			var e error
			if strings.Contains(begin, ".click") {
				item.Event, item.Delay, e = motionODPEvent(begin)
			} else {
				item.Delay, e = motionMS(begin, true)
			}
			if e != nil {
				failure = e
				return
			}
		}
		count := 0
		targets := map[string]bool{}
		motionWalk(node, func(n *compareNode) {
			covered[n] = true
			if err := motionReadRepeat(n, &item, true); err != nil {
				failure = err
			}
			for _, a := range n.attrs {
				switch a.Name.Local {
				case "accelerate", "decelerate":
					if a.Value != "0" {
						failure = fmt.Errorf("eased timing")
					}
				case "end", "restart", "sub-item":
					failure = fmt.Errorf("unsupported timing or sub-object")
				}
			}
			switch n.name.Local {
			case "transitionFilter":
				mode := n.attr("mode")
				if mode == "" {
					mode = "in"
				}
				if mode != map[bool]string{false: "in", true: "out"}[item.Exit] {
					failure = fmt.Errorf("unsupported transition mode")
				}
				count++
				if n.attr("type") != "fade" || (n.attr("mode") != "" && n.attr("mode") != map[bool]string{false: "in", true: "out"}[item.Exit]) {
					failure = fmt.Errorf("unsupported animation filter")
				}
				if n.attr("subtype") != "" && n.attr("subtype") != "crossfade" {
					failure = fmt.Errorf("unsupported fade subtype")
				}
				item.Kind = "fade"
				duration, e := motionMS(n.attr("dur"), true)
				if e != nil || duration == 0 {
					failure = fmt.Errorf("missing animation duration")
				} else {
					item.Duration = duration
				}
				targets[n.attr("targetElement")] = true
			case "set":
				if n.attr("attributeName") != "visibility" || n.attr("to") != map[bool]string{false: "visible", true: "hidden"}[item.Exit] {
					failure = fmt.Errorf("unsupported set behavior")
				}
				targets[n.attr("targetElement")] = true
			case "animateMotion":
				count++
				item.Kind = "path"
				if class != "motion-path" || (n.attr("calcMode") != "" && n.attr("calcMode") != "paced") || n.attr("rotate") != "" || n.attr("origin") != "" {
					failure = fmt.Errorf("unsupported motion coordinate mode")
				}
				var e error
				item.Path, e = parseMotionPath(n.attr("path"))
				if e != nil {
					failure = e
				}
				item.Duration, e = motionMS(n.attr("dur"), true)
				if e != nil || item.Duration == 0 {
					failure = fmt.Errorf("missing motion duration")
				}
				targets[n.attr("targetElement")] = true
			case "animateTransform", "animateColor":
				count++
				if class != "emphasis" {
					failure = fmt.Errorf("transform must be emphasis effect")
				}
				if e := motionODPTransform(n, &item); e != nil {
					failure = e
				}
				targets[n.attr("targetElement")] = true
			case "animate":
				count++
				if class != "emphasis" {
					failure = fmt.Errorf("gradient geometry must be emphasis")
				}
				if e := motionODPGradientAnimation(n, &item); e != nil {
					failure = e
				}
				targets[n.attr("targetElement")] = true
			case "audio", "iterate":
				failure = fmt.Errorf("unsupported animation behavior")
			}
		})
		if len(targets) != 1 || targets[""] || count > 1 || (count == 0 && !strings.Contains(node.attr("preset-id"), map[bool]string{false: "appear", true: "disappear"}[item.Exit])) {
			failure = fmt.Errorf("unsupported animation target/preset")
			return
		}
		for id := range targets {
			item.Target = id
		}
		if err := motionRepeatValid(item); err != nil {
			failure = err
		}
		effects = append(effects, item)
	})
	motionWalk(page, func(node *compareNode) {
		if !covered[node] && (node.attr("repeatCount") != "" && node.attr("repeatCount") != "1" || node.attr("autoReverse") == "1" || node.attr("autoReverse") == "true" || node.attr("repeatDur") != "") {
			failure = fmt.Errorf("unsupported repeating outer container")
		}
		switch node.name.Local {
		case "transitionFilter", "set", "animate", "animateColor", "animateMotion", "animateTransform", "audio", "iterate":
			if !covered[node] {
				failure = fmt.Errorf("animation outside supported sequence")
			}
		}
	})
	if len(effects) > motionMaxEffects {
		return nil, fmt.Errorf("more than 12 effects on slide")
	}
	return effects, failure
}
func motionODPTransition(page *compareNode, styles map[string]*compareNode) (motionTransition, error) {
	result := motionTransition{Kind: "cut"}
	name := page.attr("style-name")
	style := styles[name]
	if style == nil {
		return result, nil
	}
	props := style.child("drawing-page-properties")
	kind := props.attr("transition-type")
	if props.attr("transition-style") != "" && props.attr("transition-style") != "none" {
		return result, fmt.Errorf("legacy slide transition")
	}
	if style.attr("parent-style-name") != "" {
		return result, fmt.Errorf("inherited transition style")
	}
	if kind == "" || kind == "none" {
		return result, nil
	}
	result.Duration = 1000
	if value := props.attr("transition-duration"); value != "" {
		duration, e := motionMS(value, true)
		if e != nil {
			return result, e
		}
		result.Duration = duration
	}
	switch kind {
	case "fade":
		result.Kind = "fade"
		if sub := props.attr("transition-subtype"); sub != "" && sub != "crossfade" {
			return result, fmt.Errorf("unsupported transition subtype")
		}
	case "barWipe":
		result.Kind = "wipe"
		result.Direction = map[string]string{"leftToRight": "r", "topToBottom": "d"}[props.attr("transition-subtype")]
		if result.Direction == "" {
			return result, fmt.Errorf("unsupported wipe subtype")
		}
		if props.attr("transition-direction") == "reverse" {
			result.Direction = map[string]string{"r": "l", "d": "u"}[result.Direction]
		}
	case "pushWipe":
		result.Kind = "push"
		result.Direction = map[string]string{"fromLeft": "r", "fromRight": "l", "fromTop": "d", "fromBottom": "u"}[props.attr("transition-subtype")]
		if result.Direction == "" {
			return result, fmt.Errorf("unsupported push subtype")
		}
	default:
		return result, fmt.Errorf("unsupported slide transition")
	}
	return result, nil
}
func motionXMLAttr(value string) string {
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}
func motionRenameIDs(data []byte, suffix string) []byte {
	// Animation references have been removed; keep connectors and internal links paired with renamed IDs.
	ids := map[string]string{}
	for _, match := range motionIDAttribute.FindAllSubmatch(data, -1) {
		key := string(match[1])
		if strings.Contains(key, "id") && !strings.Contains(key, "href") {
			ids[string(match[3])] = string(match[3]) + suffix
		}
	}
	return motionIDAttribute.ReplaceAllFunc(data, func(match []byte) []byte {
		parts := motionIDAttribute.FindSubmatch(match)
		value := string(parts[3])
		prefix := ""
		if strings.HasPrefix(value, "#") {
			prefix = "#"
			value = strings.TrimPrefix(value, "#")
		}
		if next, ok := ids[value]; ok {
			return []byte(string(parts[1]) + string(parts[2]) + prefix + next + string(parts[4]))
		}
		return match
	})
}
func buildMotionODP(pkg *comparePackage) (motionDeck, []byte, error) {
	deck := motionDeck{}
	root, err := pkg.xml("content.xml")
	if err != nil {
		return deck, nil, err
	}
	source, _ := pkg.read("content.xml")
	pages := root.child("body").child("presentation").all("page")
	spans, err := motionSpans(source, "presentation", "page")
	if err != nil || len(pages) != len(spans) || len(pages) == 0 || len(pages) > 50 {
		return deck, nil, fmt.Errorf("motion comparison requires 1–50 slides")
	}
	styles := map[string]*compareNode{}
	for _, name := range []string{"styles.xml", "content.xml"} {
		if pkg.files[name] == nil {
			continue
		}
		node, e := pkg.xml(name)
		if e != nil {
			return deck, nil, e
		}
		motionWalk(node, func(n *compareNode) {
			if n.name.Local == "style" && n.attr("family") == "drawing-page" {
				styles[n.attr("name")] = n
			}
		})
	}
	replacements := [][]byte{}
	var colorStyles, gradientStyles strings.Builder
	generatedBytes := 0
	for index, page := range pages {
		slide := motionSlide{Title: page.attr("name"), Effects: []motionEffect{}}
		if slide.Title == "" {
			slide.Title = fmt.Sprintf("Slide %d", index+1)
		}
		slide.Effects, err = motionODPEffects(page)
		if err != nil {
			slide.Warning = "Unsupported animation: " + err.Error() + "; static slide"
		}
		slide.Transition, err = motionODPTransition(page, styles)
		if err != nil {
			slide.Warning = "Unsupported transition: " + err.Error() + "; static slide"
			slide.Transition = motionTransition{Kind: "cut"}
		}
		body := source[spans[index].start:spans[index].end]
		objects, e := motionSpans(body, "page", "frame", "g", "custom-shape", "rect", "ellipse", "line", "connector", "path", "polygon", "polyline", "circle", "caption")
		if e != nil {
			return deck, nil, e
		}
		ids := map[string]bool{}
		shapeIDs := []string{}
		for _, object := range objects {
			id := ""
			for _, attr := range object.attrs {
				if attr.Name.Local == "id" {
					id = attr.Value
					break
				}
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
		stripped, e := motionSpans(body, "page", "par", "seq", "animations")
		if e != nil {
			return deck, nil, e
		}
		var expanded bytes.Buffer
		motionWalk(page, func(n *compareNode) {
			switch n.name.Local {
			case "plugin", "floating-frame", "applet":
				slide.Warning = "Unsupported media or interactive content; static slide"
			}
		})
		width, height := motionODPSize(pkg, page)
		for _, object := range objects {
			hit := motionHit{}
			transformed := false
			for _, a := range object.attrs {
				switch a.Name.Local {
				case "transform":
					transformed = true
				case "id":
					hit.Target = a.Value
				case "name":
					hit.Name = a.Value
				case "x":
					hit.Box[0] = motionLength(a.Value)
				case "y":
					hit.Box[1] = motionLength(a.Value)
				case "width":
					hit.Box[2] = motionLength(a.Value)
				case "height":
					hit.Box[3] = motionLength(a.Value)
				}
			}
			if !transformed && width > 0 && height > 0 && hit.Box[2] > 0 && hit.Box[3] > 0 {
				hit.Box = [4]float64{hit.Box[0] / width, hit.Box[1] / height, hit.Box[2] / width, hit.Box[3] / height}
				if hit.Target != "" && motionValidBox(hit.Box) && len(slide.Hits) < 1000 {
					slide.Hits = append(slide.Hits, hit)
				}
			}
		}
		motionMarkStages(&slide)
		motionPrepareTransformStages(&slide)
		motionPrepareODPColor(pkg, &slide, page)
		motionPrepareODPGradient(pkg, &slide, page)
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
			for i, span := range objects {
				if hidden[shapeIDs[i]] {
					removals = append(removals, span)
				}
			}
			sort.Slice(removals, func(i, j int) bool { return removals[i].start < removals[j].start })
			state := motionEdit(body, removals, nil)
			if len(plans) > 0 && stateIndex == len(states)-1 {
				state = motionODPFinal(state, &slide, width, height)
			}
			state, assets, err := motionODPFramePaint(pkg, state, &slide, plan, deck.Frames)
			if err != nil {
				return deck, nil, err
			}
			generatedBytes += len(assets.Styles) + len(assets.Gradients)
			if generatedBytes > 32<<20 {
				return deck, nil, fmt.Errorf("storyboard XML exceeds 32 MiB")
			}
			colorStyles.WriteString(assets.Styles)
			gradientStyles.WriteString(assets.Gradients)
			if plan != nil && plan.matte != "" {
				state = motionRootAttr(state, "style-name", "_TaygaMotion"+plan.matte, "tgDraw:style-name", "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0")
			}

			state = motionRenameIDs(state, fmt.Sprintf("_taygaFrame%d", deck.Frames))
			generatedBytes += len(state)
			if generatedBytes > 32<<20 {
				return deck, nil, fmt.Errorf("storyboard XML exceeds 32 MiB")
			}
			expanded.Write(state)
			if plan == nil {
				slide.States = append(slide.States, deck.Frames)
			} else {
				motionRegisterLayer(&slide, *plan, deck.Frames)
			}
		}
		replacements = append(replacements, expanded.Bytes())
		deck.Slides = append(deck.Slides, slide)
	}
	content := motionEdit(source, spans, replacements)
	content, e := motionODPStyles(content, colorStyles.String())
	if e != nil {
		return deck, nil, e
	}
	changes := map[string][]byte{"content.xml": content}
	if gradientStyles.Len() > 0 {
		data, e := motionODPAddGradients(pkg, gradientStyles.String())
		if e != nil {
			return deck, nil, e
		}
		changes["styles.xml"] = data
	}
	data, e := motionZIP(pkg, changes)
	return deck, data, e
}

package httpapi

import (
	"fmt"
	"regexp"
	"strings"
)

type motionColorAssets struct{ Styles, Gradients string }

func motionODPStylesMap(pkg *comparePackage) map[string]*compareNode {
	result := map[string]*compareNode{}
	for _, part := range []string{"styles.xml", "content.xml"} {
		if pkg.files[part] == nil {
			continue
		}
		root, e := pkg.xml(part)
		if e != nil {
			continue
		}
		motionWalk(root, func(n *compareNode) {
			if n.name.Local == "style" {
				result[n.attr("name")] = n
			}
		})
	}
	return result
}
func motionStyleValue(styles map[string]*compareNode, name, container, key string) string {
	seen := map[string]bool{}
	for i := 0; i < 16 && name != "" && !seen[name]; i++ {
		seen[name] = true
		n := styles[name]
		if n == nil {
			return ""
		}
		v := n.child(container).attr(key)
		if v != "" {
			return v
		}
		name = n.attr("parent-style-name")
	}
	return ""
}
func motionODPGradient(pkg *comparePackage, name string) (*compareNode, error) {
	for _, part := range []string{"styles.xml", "content.xml"} {
		if pkg.files[part] == nil {
			continue
		}
		root, e := pkg.xml(part)
		if e != nil {
			return nil, e
		}
		for _, n := range root.descendants("gradient") {
			if n.attr("name") == name {
				if _, err := motionODPGradientColors(n); err != nil {
					return nil, err
				}
				if _, e = motionRGB(n.attr("start-color")); e != nil {
					return nil, e
				}
				if _, e = motionRGB(n.attr("end-color")); e != nil {
					return nil, e
				}
				return n, nil
			}
		}
	}
	return nil, fmt.Errorf("gradient definition missing")
}
func motionPrepareODPColor(pkg *comparePackage, slide *motionSlide, page *compareNode) {
	if slide.Warning != "" {
		return
	}
	styles := motionODPStylesMap(pkg)
	for i := range slide.Effects {
		if slide.Warning != "" {
			return
		}
		e := &slide.Effects[i]
		if e.Kind != "color" {
			continue
		}
		found := false
		for _, shape := range page.children {
			if shape.attr("id") != e.Target {
				continue
			}
			switch shape.name.Local {
			case "rect", "ellipse", "custom-shape", "circle", "polygon", "path":
			default:
				continue
			}
			name := shape.attr("style-name")
			colors := [][]float64{}
			switch e.Property {
			case "fill":
				fill := motionStyleValue(styles, name, "graphic-properties", "fill")
				if fill == "gradient" {
					gradient, err := motionODPGradient(pkg, motionStyleValue(styles, name, "graphic-properties", "fill-gradient-name"))
					if err != nil {
						continue
					}
					colors, _ = motionODPGradientColors(gradient)
					e.gradient = true
				} else if fill == "solid" {
					c, err := motionRGB(motionStyleValue(styles, name, "graphic-properties", "fill-color"))
					if err != nil {
						continue
					}
					colors = [][]float64{c}
				} else {
					continue
				}
			case "stroke":
				if value := motionStyleValue(styles, name, "graphic-properties", "stroke"); value != "solid" && value != "dash" {
					continue
				}
				c, err := motionRGB(motionStyleValue(styles, name, "graphic-properties", "stroke-color"))
				if err != nil {
					continue
				}
				colors = [][]float64{c}
			case "text":
				e.sourceStyles = map[string][]float64{}
				styleNames := []string{name, shape.attr("text-style-name")}
				motionWalk(shape, func(n *compareNode) {
					if n.name.Local == "p" || n.name.Local == "span" {
						styleNames = append(styleNames, n.attr("style-name"))
					}
				})
				for _, style := range styleNames {
					if style == "" {
						continue
					}
					value := motionStyleValue(styles, style, "text-properties", "color")
					if value == "" {
						continue
					}
					c, err := motionRGB(value)
					if err != nil {
						colors = nil
						break
					}
					e.sourceStyles[style] = c
					colors = append(colors, c)
				}
				if len(colors) == 0 {
					continue
				}
			}
			if len(colors) == 0 {
				continue
			}
			colors = motionStageColors(slide, *e, colors)
			if e.Previous != nil && e.Property == "text" {
				previous := slide.Effects[*e.Previous]
				for key, source := range previous.sourceStyles {
					e.sourceStyles[key] = motionPaintTween(source, previous, motionFinalPhase(previous))
				}
			}
			found = true
			if len(e.From) > 0 {
				for _, c := range colors {
					if motionHex(c) != motionHex(e.From) {
						found = false
					}
				}
			}
			e.sourceColors = colors
			if len(e.From) == 0 {
				e.From = append([]float64{}, colors[0]...)
			}
			if e.Relative {
				e.To = motionRelativeColor(colors[0], *e, 1)
			}
		}
		if !found {
			slide.Warning = "Unsupported ODP " + e.Property + " color target or source paint; static slide"
		}
	}
}

var motionODPStyleReference = regexp.MustCompile(`((?:[A-Za-z_][A-Za-z0-9_.-]*:)?(?:text-style-name|style-name)\s*=\s*)(["'])([^"']*)(["'])`)

func motionODPColorPaintBody(pkg *comparePackage, body []byte, effect motionEffect, phase int, name string) ([]byte, motionColorAssets, error) {
	return motionODPPaintBody(pkg, body, effect, phase, name, nil, 0)
}
func motionODPPaintBody(pkg *comparePackage, body []byte, effect motionEffect, phase int, name string, geometry *motionEffect, geometryPhase int, rasters ...*motionEffect) ([]byte, motionColorAssets, error) {
	var raster *motionEffect
	if len(rasters) > 0 {
		raster = rasters[0]
	}
	t := float64(phase-1) / float64(motionSamples(effect)-1)
	geometryOnly := effect.Kind == "gradient"
	if geometryOnly {
		copy := effect
		geometry, geometryPhase = &copy, phase
		effect.Kind, effect.Property, effect.Relative, effect.Space = "color", "fill", false, "rgb"
		effect.From, effect.To = effect.sourceColors[0], effect.sourceColors[0]
	}

	assets := motionColorAssets{}
	if raster == nil && effect.Previous == nil && phase == 1 && (geometry == nil || geometryPhase == 1 && geometry.Previous == nil) {
		return body, assets, nil
	}
	color := motionPaintTween(effect.sourceColors[0], effect, t)
	spans, e := motionSpans(body, "page", "rect", "ellipse", "custom-shape", "circle", "polygon", "path")
	if e != nil || len(spans) != 1 {
		return nil, assets, fmt.Errorf("color requires one plain shape")
	}
	span := spans[0]
	shape := body[span.start:span.end]
	styles := motionODPStylesMap(pkg)
	gradientName := ""
	if effect.gradient && raster == nil {
		parent := ""
		for _, a := range span.attrs {
			if a.Name.Local == "style-name" {
				parent = a.Value
			}
		}
		oldName := motionStyleValue(styles, parent, "graphic-properties", "fill-gradient-name")
		gradient, err := motionODPGradient(pkg, oldName)
		if err != nil {
			return nil, assets, err
		}
		gradientName = name + "Gradient"
		values := [][]float64{}
		for _, source := range effect.sourceColors {
			if geometryOnly {
				values = append(values, source)
			} else {
				values = append(values, motionPaintTween(source, effect, t))
			}
		}
		assets.Gradients = motionODPGradientXML(gradient, gradientName, values)
		if geometry != nil {
			modified, err := motionODPGradientGeometry([]byte(assets.Gradients), *geometry, geometryPhase)
			if err != nil {
				return nil, assets, err
			}
			assets.Gradients = string(modified)
		}
	}
	bitmapName := ""
	if raster != nil {
		bitmapName = name + "Image"
		definition, err := motionODPFocusImage(*raster, bitmapName)
		if err != nil {
			return nil, assets, err
		}
		assets.Gradients = definition
	}
	mapping := map[string]string{}
	references := motionODPStyleReference.FindAllSubmatch(shape, -1)
	for _, ref := range references {
		old := string(ref[3])
		if _, ok := mapping[old]; ok {
			continue
		}
		style := styles[old]
		if style == nil {
			continue
		}
		family := style.attr("family")
		if effect.Property != "text" && family != "graphic" && family != "paragraph" {
			continue
		}
		if len(mapping) >= 32 {
			return nil, assets, fmt.Errorf("color target exceeds 32 style references")
		}
		newName := fmt.Sprintf("%sStyle%d", name, len(mapping))
		value := color
		if effect.Property == "text" {
			source := effect.sourceStyles[old]
			if len(source) != 3 {
				source = effect.sourceColors[0]
			}
			value = motionPaintTween(source, effect, t)
		}
		cloned, err := motionCloneODPColorStyle(pkg, old, newName, family, value, effect.Property, gradientName)
		if err != nil {
			return nil, assets, err
		}
		if bitmapName != "" && len(cloned) > 0 {
			cloned, err = motionODPBitmapStyle(cloned, family, bitmapName)
			if err != nil {
				return nil, assets, err
			}
		}
		if len(cloned) == 0 {
			continue
		}
		if len(assets.Styles)+len(assets.Gradients)+len(cloned) > 8<<20 {
			return nil, assets, fmt.Errorf("color styles exceed 8 MiB")
		}
		assets.Styles += string(cloned)
		mapping[old] = newName
	}
	shape = motionODPStyleReference.ReplaceAllFunc(shape, func(value []byte) []byte {
		match := motionODPStyleReference.FindSubmatch(value)
		if next, ok := mapping[string(match[3])]; ok {
			return []byte(string(match[1]) + string(match[2]) + motionXMLAttr(next) + string(match[4]))
		}
		return value
	})
	return motionEdit(body, spans, [][]byte{shape}), assets, nil
}
func motionODPGradientXML(gradient *compareNode, name string, colors [][]float64) string {
	start, end := colors[0], colors[len(colors)-1]
	attrs := ` xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" draw:name="` + name + `"`
	for _, a := range gradient.attrs {
		switch a.Name.Local {
		case "name", "display-name", "start-color", "end-color":
			continue
		}
		if a.Name.Space == "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" {
			attrs += ` draw:` + a.Name.Local + `="` + motionXMLAttr(a.Value) + `"`
		}
	}
	children := ""
	for i, stop := range gradient.children {
		children += `<loext:gradient-stop xmlns:loext="urn:org:documentfoundation:names:experimental:office:xmlns:loext:1.0" xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" svg:offset="` + motionXMLAttr(stop.attr("offset")) + `" loext:color-type="rgb" loext:color-value="#` + motionHex(colors[i]) + `"/>`
	}
	return `<draw:gradient` + attrs + ` draw:start-color="#` + motionHex(start) + `" draw:end-color="#` + motionHex(end) + `">` + children + `</draw:gradient>`
}
func motionCloneODPColorStyle(pkg *comparePackage, parent, name, family string, color []float64, property, gradient string) ([]byte, error) {
	// Reuse namespace-safe style copying, then edit only the requested channel.
	for _, part := range []string{"content.xml", "styles.xml"} {
		if pkg.files[part] == nil {
			continue
		}
		data, err := pkg.read(part)
		if err != nil {
			return nil, err
		}
		root, err := pkg.xml(part)
		if err != nil {
			return nil, err
		}
		spans, err := motionSpans(data, "", "style")
		if err != nil {
			return nil, err
		}
		for _, span := range spans {
			matched, fam := false, ""
			for _, a := range span.attrs {
				if a.Name.Local == "name" && a.Value == parent {
					matched = true
				}
				if a.Name.Local == "family" {
					fam = a.Value
				}
			}
			if !matched || fam != family {
				continue
			}
			style := append([]byte{}, data[span.start:span.end]...)
			count, size := 0, 0
			for _, a := range root.attrs {
				if a.Name.Space != "xmlns" {
					continue
				}
				count++
				size += len(a.Name.Local) + len(a.Value)
				if count > 64 || size > 65536 {
					return nil, fmt.Errorf("style namespace limit exceeded")
				}
				style = motionRootAttr(style, a.Name.Local, a.Value, "xmlns:"+a.Name.Local, "")
			}
			style = motionRootAttr(style, "name", name, "style:name", "urn:oasis:names:tc:opendocument:xmlns:style:1.0")
			style = motionRootAttr(style, "display-name", name, "style:display-name", "urn:oasis:names:tc:opendocument:xmlns:style:1.0")
			container := "graphic-properties"
			if property == "text" {
				container = "text-properties"
			}
			props, err := motionSpans(style, "style", container)
			if err != nil {
				return nil, err
			}
			if len(props) == 0 {
				if property != "text" && family != "graphic" {
					return nil, nil
				}
				empty := `<style:` + container + ` xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"/>`
				style, err = motionAppendRoot(style, "style", empty)
				if err != nil {
					return nil, err
				}
				props, _ = motionSpans(style, "style", container)
			}
			if len(props) != 1 {
				return nil, fmt.Errorf("ambiguous color style properties")
			}
			value := style[props[0].start:props[0].end]
			switch property {
			case "fill":
				if gradient != "" {
					value = motionRootAttr(value, "fill", "gradient", "draw:fill", "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0")
					value = motionRootAttr(value, "fill-gradient-name", gradient, "draw:fill-gradient-name", "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0")
				} else {
					value = motionRootAttr(value, "fill", "solid", "draw:fill", "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0")
					value = motionRootAttr(value, "fill-color", "#"+motionHex(color), "draw:fill-color", "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0")
				}
			case "stroke":
				value = motionRootAttr(value, "stroke-color", "#"+motionHex(color), "svg:stroke-color", "urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0")
			case "text":
				value = motionRootAttr(value, "color", "#"+motionHex(color), "fo:color", "urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0")
			}
			return motionEdit(style, props, [][]byte{value}), nil
		}
	}
	return nil, fmt.Errorf("source color style missing")
}
func motionODPAddGradients(pkg *comparePackage, extra string) ([]byte, error) {
	data, err := pkg.read("styles.xml")
	if err != nil {
		return nil, err
	}
	spans, err := motionSpans(data, "document-styles", "styles")
	if err != nil || len(spans) != 1 {
		return nil, fmt.Errorf("gradient styles root missing")
	}
	for _, reserved := range []string{"_TaygaColor", "_TaygaGradient"} {
		if strings.Contains(string(data), reserved) {
			return nil, fmt.Errorf("reserved gradient name collision")
		}
	}
	node, err := motionAppendRoot(data[spans[0].start:spans[0].end], "styles", extra)
	if err != nil {
		return nil, err
	}
	return motionEdit(data, spans, [][]byte{node}), nil
}

func motionODPGradientColors(gradient *compareNode) ([][]float64, error) {
	if len(gradient.children) == 0 {
		a, e := motionRGB(gradient.attr("start-color"))
		if e != nil {
			return nil, e
		}
		b, e := motionRGB(gradient.attr("end-color"))
		return [][]float64{a, b}, e
	}
	if len(gradient.children) < 2 || len(gradient.children) > 16 {
		return nil, fmt.Errorf("gradient requires 2–16 stops")
	}
	out := [][]float64{}
	last := -1.0
	for _, stop := range gradient.children {
		if stop.name.Local != "gradient-stop" || stop.attr("color-type") != "rgb" {
			return nil, fmt.Errorf("unsupported gradient stop")
		}
		position, e := motionNumber(stop.attr("offset"), 0, 1)
		if e != nil || position < last {
			return nil, fmt.Errorf("invalid gradient offset")
		}
		last = position
		color, e := motionRGB(stop.attr("color-value"))
		if e != nil {
			return nil, e
		}
		out = append(out, color)
	}
	return out, nil
}

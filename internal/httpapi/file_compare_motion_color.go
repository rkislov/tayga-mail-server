package httpapi

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type motionColorSample struct {
	At         float64 `json:"at"`
	GeometryAt float64 `json:"geometry_at,omitempty"`
	Black      int     `json:"black"`
	White      int     `json:"white"`
}
type motionColorStop struct {
	Position int
	Color    []float64
}

func motionRGBToHSL(c []float64) []float64 {
	r, g, b := c[0]/255, c[1]/255, c[2]/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	d := hi - lo
	h, s, l := 0.0, 0.0, (hi+lo)/2
	if d > 1e-9 {
		s = d / (1 - math.Abs(2*l-1))
		switch hi {
		case r:
			h = math.Mod((g-b)/d, 6)
		case g:
			h = (b-r)/d + 2
		default:
			h = (r-g)/d + 4
		}
		h = math.Mod(h*60+360, 360)
	}
	return []float64{h, s, l}
}
func motionHSLToRGB(c []float64) []float64 {
	h, s, l := math.Mod(c[0]+3600, 360)/60, c[1], c[2]
	a := (1 - math.Abs(2*l-1)) * s
	x := a * (1 - math.Abs(math.Mod(h, 2)-1))
	m := l - a/2
	r, g, b := 0.0, 0.0, 0.0
	switch {
	case h < 1:
		r, g = a, x
	case h < 2:
		r, g = x, a
	case h < 3:
		g, b = a, x
	case h < 4:
		g, b = x, a
	case h < 5:
		r, b = x, a
	default:
		r, b = a, x
	}
	return []float64{math.Round((r + m) * 255), math.Round((g + m) * 255), math.Round((b + m) * 255)}
}
func motionColorTween(from, to []float64, t float64, space, direction string) []float64 {
	if t <= 0 {
		return append([]float64{}, from...)
	}
	if t >= 1 {
		return append([]float64{}, to...)
	}
	if space != "hsl" {
		return []float64{math.Round(from[0] + (to[0]-from[0])*t), math.Round(from[1] + (to[1]-from[1])*t), math.Round(from[2] + (to[2]-from[2])*t)}
	}
	a, b := motionRGBToHSL(from), motionRGBToHSL(to)
	if a[1] < 1e-9 {
		a[0] = b[0]
	}
	if b[1] < 1e-9 {
		b[0] = a[0]
	}
	delta := math.Mod(b[0]-a[0]+360, 360)
	if direction == "ccw" && delta > 0 {
		delta -= 360
	}
	return motionHSLToRGB([]float64{a[0] + delta*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t})
}
func motionSamples(e motionEffect) int {
	if (e.Space == "hsl" || e.Relative || e.Kind == "gradient") && !e.Discrete {
		return 9
	}
	return 2
}
func motionSampleValue(e motionEffect, phase int) []float64 {
	return motionColorTween(e.From, e.To, float64(phase-1)/float64(motionSamples(e)-1), e.Space, e.Direction)
}

// Resolve only local appearance parts; never follow external theme resources.
func motionSlideTheme(pkg *comparePackage, part string, slide *compareNode) (map[string][]float64, error) {
	palette := map[string][]float64{}
	mapping := map[string]string{"bg1": "lt1", "tx1": "dk1", "bg2": "lt2", "tx2": "dk2"}
	seen := map[string]bool{}
	roots := []*compareNode{}
	var visit func(string, int) error
	visit = func(current string, depth int) error {
		if depth > 6 || seen[current] {
			return nil
		}
		seen[current] = true
		root, e := pkg.xml(current)
		if e != nil {
			return e
		}
		links, e := pkg.relationships(current)
		if e != nil {
			return e
		}
		ids := []string{}
		for id := range links {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return links[ids[i]].kind < links[ids[j]].kind })
		for _, id := range ids {
			rel := links[id]
			role := rel.kind[strings.LastIndex(rel.kind, "/")+1:]
			if role != "slideLayout" && role != "slideMaster" && role != "theme" && role != "themeOverride" {
				continue
			}
			if role == "slideLayout" && current != part {
				continue
			}
			if rel.external {
				return fmt.Errorf("external color theme unsupported")
			}
			if e = visit(rel.target, depth+1); e != nil {
				return e
			}
		}
		roots = append(roots, root)
		return nil
	}
	if e := visit(part, 0); e != nil {
		return nil, e
	}
	masterMap := map[string]string{"bg1": "lt1", "tx1": "dk1", "bg2": "lt2", "tx2": "dk2"}
	for _, root := range roots {
		scheme := root.child("themeElements").child("clrScheme")
		if scheme == nil {
			scheme = root.child("clrScheme")
		}
		if scheme != nil {
			for _, entry := range scheme.children {
				if len(entry.children) != 1 {
					continue
				}
				color, e := motionDrawingColor(entry.children[0], palette)
				if e == nil {
					palette[entry.name.Local] = color
				}
			}
		}
		if root.child("clrMapOvr").child("masterClrMapping") != nil {
			mapping = map[string]string{}
			for k, v := range masterMap {
				mapping[k] = v
			}
		}
		clrMap := root.child("clrMap")
		if override := root.child("clrMapOvr").child("overrideClrMapping"); override != nil {
			clrMap = override
		}
		if clrMap != nil {
			for _, a := range clrMap.attrs {
				if a.Name.Space == "" {
					mapping[a.Name.Local] = a.Value
				}
			}
		}
		if root.name.Local == "sldMaster" {
			masterMap = map[string]string{}
			for k, v := range mapping {
				masterMap[k] = v
			}
		}
	}
	if override := slide.child("clrMapOvr").child("overrideClrMapping"); override != nil {
		for _, a := range override.attrs {
			if a.Name.Space == "" {
				mapping[a.Name.Local] = a.Value
			}
		}
	}
	resolved := map[string][]float64{}
	for k, v := range palette {
		resolved[k] = v
	}
	for k, v := range mapping {
		if c := palette[v]; len(c) == 3 {
			resolved[k] = c
		}
	}
	return resolved, nil
}
func motionDrawingColor(n *compareNode, palette map[string][]float64) ([]float64, error) {
	if n == nil {
		return nil, fmt.Errorf("color missing")
	}
	var c []float64
	var e error
	switch n.name.Local {
	case "srgbClr":
		c, e = motionRGB(n.attr("val"))
	case "schemeClr":
		c = append([]float64{}, palette[n.attr("val")]...)
		if len(c) != 3 {
			e = fmt.Errorf("theme color missing")
		}
	case "sysClr":
		c, e = motionRGB(n.attr("lastClr"))
	case "hslClr":
		h, e1 := motionNumber(n.attr("hue"), 0, 21600000)
		s, e2 := motionNumber(n.attr("sat"), 0, 100000)
		l, e3 := motionNumber(n.attr("lum"), 0, 100000)
		if e1 != nil || e2 != nil || e3 != nil {
			return nil, fmt.Errorf("invalid HSL color")
		}
		c = motionHSLToRGB([]float64{h / 60000, s / 100000, l / 100000})
	default:
		return nil, fmt.Errorf("unsupported color encoding")
	}
	if e != nil {
		return nil, e
	}
	for _, mod := range n.children {
		v, e := motionNumber(mod.attr("val"), 0, 100000)
		if e != nil {
			return nil, e
		}
		p := v / 100000
		switch mod.name.Local {
		case "tint":
			for i := range c {
				c[i] = 255 - (255-c[i])*p
			}
		case "shade":
			for i := range c {
				c[i] *= p
			}
		case "lumMod", "lumOff", "satMod", "satOff":
			hsl := motionRGBToHSL(c)
			index := 2
			if strings.HasPrefix(mod.name.Local, "sat") {
				index = 1
			}
			if strings.HasSuffix(mod.name.Local, "Mod") {
				hsl[index] *= p
			} else {
				hsl[index] += p
			}
			hsl[index] = math.Min(1, math.Max(0, hsl[index]))
			c = motionHSLToRGB(hsl)
		case "alpha":
			if v != 100000 {
				return nil, fmt.Errorf("animated transparent color unsupported")
			}
		default:
			return nil, fmt.Errorf("unsupported color modifier")
		}
	}
	for i := range c {
		c[i] = math.Round(math.Min(255, math.Max(0, c[i])))
	}
	return c, nil
}
func motionContainerColor(n *compareNode, palette map[string][]float64) ([]float64, error) {
	if n == nil {
		return nil, nil
	}
	if len(n.children) != 1 {
		return nil, fmt.Errorf("one color required")
	}
	return motionDrawingColor(n.children[0], palette)
}
func motionPPTXPaint(shape *compareNode, property string) (*compareNode, error) {
	switch property {
	case "fill":
		return shape.child("spPr"), nil
	case "stroke":
		node := shape.child("spPr").child("ln")
		if node == nil {
			return nil, fmt.Errorf("explicit contour required")
		}
		return node, nil
	case "text":
		return shape.child("txBody"), nil
	}
	return nil, fmt.Errorf("unknown color property")
}
func motionPPTXSourceColors(shape *compareNode, e *motionEffect, palette map[string][]float64) ([][]float64, error) {
	paint, err := motionPPTXPaint(shape, e.Property)
	if err != nil {
		return nil, err
	}
	if e.Property == "text" {
		colors := [][]float64{}
		for _, paragraph := range paint.all("p") {
			fallback := paragraph.child("pPr").child("defRPr").child("solidFill")
			if fallback == nil {
				fallback = shape.child("style").child("fontRef")
			}
			for _, run := range paragraph.children {
				if run.name.Local != "r" && run.name.Local != "fld" {
					continue
				}
				if run.child("t") == nil {
					continue
				}
				fill := run.child("rPr").child("solidFill")
				if fill == nil {
					fill = fallback
				}
				var c []float64
				var err error
				if fill == nil {
					c = palette["tx1"]
					if len(c) != 3 {
						err = fmt.Errorf("inherited text color unresolved")
					}
				} else {
					c, err = motionContainerColor(fill, palette)
				}
				if err != nil {
					return nil, err
				}
				colors = append(colors, c)
				if len(colors) > 1024 {
					return nil, fmt.Errorf("text color target exceeds 1024 runs")
				}
			}
		}
		if len(colors) == 0 {
			return nil, fmt.Errorf("text target empty")
		}
		return colors, nil
	}
	if fill := paint.child("solidFill"); fill != nil {
		c, e := motionContainerColor(fill, palette)
		return [][]float64{c}, e
	}
	grad := paint.child("gradFill")
	if grad != nil && grad.attr("rotWithShape") == "0" {
		return nil, fmt.Errorf("fixed-coordinate gradient unsupported")
	}
	if grad == nil {
		return nil, fmt.Errorf("explicit solid or gradient paint required")
	}
	stops := grad.child("gsLst").all("gs")
	if len(stops) < 2 || len(stops) > 16 {
		return nil, fmt.Errorf("gradient requires 2–16 stops")
	}
	colors := [][]float64{}
	last := -1
	for _, stop := range stops {
		pos, err := strconv.Atoi(stop.attr("pos"))
		if err != nil || pos < last || pos < 0 || pos > 100000 {
			return nil, fmt.Errorf("invalid gradient stop")
		}
		last = pos
		c, err := motionContainerColor(stop, palette)
		if err != nil {
			return nil, err
		}
		colors = append(colors, c)
	}
	e.gradient = true
	return colors, nil
}
func motionPreparePPTXColor(pkg *comparePackage, part string, root *compareNode, slide *motionSlide, shapes []*compareNode) {
	if slide.Warning != "" {
		return
	}
	needs := false
	for _, e := range slide.Effects {
		needs = needs || e.Kind == "color"
	}
	if !needs {
		return
	}
	palette, err := motionSlideTheme(pkg, part, root)
	if err != nil {
		slide.Warning = "Unsupported color theme: " + err.Error() + "; static slide"
		return
	}
	for i := range slide.Effects {
		e := &slide.Effects[i]
		if e.Kind != "color" {
			continue
		}
		if e.fromRef != nil {
			e.From, err = motionContainerColor(e.fromRef, palette)
			if err != nil {
				slide.Warning = "Unsupported source color; static slide"
				continue
			}
		}
		if !e.Relative {
			e.To, err = motionContainerColor(e.toRef, palette)
		}
		if err != nil {
			slide.Warning = "Unsupported destination color; static slide"
			continue
		}
		found := false
		for _, shape := range shapes {
			props := shape.descendants("cNvPr")
			if len(props) == 0 || props[0].attr("id") != e.Target || shape.name.Local != "sp" {
				continue
			}
			if e.wholeTextRange && e.textRangeEnd != len(shape.child("txBody").all("p"))-1 {
				slide.Warning = "Partial text color target unsupported; static slide"
				continue
			}
			colors, err := motionPPTXSourceColors(shape, e, palette)
			if err != nil {
				slide.Warning = "Unsupported paint: " + err.Error() + "; static slide"
				continue
			}
			e.sourceColors = colors
			found = true
			for _, c := range colors {
				if len(e.From) > 0 && motionHex(e.From) != motionHex(c) {
					found = false
				}
			}
			if len(e.From) == 0 {
				e.From = append([]float64{}, colors[0]...)
			}
			if e.Relative {
				e.To = motionRelativeColor(colors[0], *e, 1)
			}
		}
		if !found {
			slide.Warning = "Unsupported color target or mismatched source color; static slide"
		}
	}
}
func motionColorXML(c []float64) []byte {
	return []byte(`<a:srgbClr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" val="` + motionHex(c) + `"/>`)
}
func motionPPTXPaintBody(body []byte, effect motionEffect, phase int) ([]byte, error) {
	if phase == 1 {
		return body, nil
	}
	t := float64(phase-1) / float64(motionSamples(effect)-1)
	shapes, err := motionSpans(body, "spTree", "sp")
	if err != nil || len(shapes) != 1 {
		return nil, fmt.Errorf("one plain shape required")
	}
	shape := body[shapes[0].start:shapes[0].end]
	if effect.Property == "text" {
		spans, err := motionSpans(shape, "p", "r", "fld")
		if err != nil {
			return nil, err
		}
		replacements := make([][]byte, len(spans))
		for i, span := range spans {
			if i >= len(effect.sourceColors) {
				return nil, fmt.Errorf("text color/run mismatch")
			}
			run := shape[span.start:span.end]
			properties, err := motionSpans(run, span.name, "rPr")
			if err != nil {
				return nil, err
			}
			paint := []byte(`<a:solidFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` + string(motionColorXML(motionPaintTween(effect.sourceColors[i], effect, t))) + `</a:solidFill>`)
			if len(properties) == 1 {
				props := run[properties[0].start:properties[0].end]
				fills, _ := motionSpans(props, "rPr", "solidFill", "gradFill", "noFill")
				if len(fills) > 0 {
					replacement := make([][]byte, len(fills))
					replacement[0] = paint
					props = motionEdit(props, fills, replacement)
				} else {
					props, err = motionAppendRoot(props, "rPr", string(paint))
					if err != nil {
						return nil, err
					}
				}
				run = motionEdit(run, properties, [][]byte{props})
			} else {
				decoder := xml.NewDecoder(bytes.NewReader(run))
				_, err = decoder.Token()
				if err != nil {
					return nil, err
				}
				offset := int(decoder.InputOffset())
				insert := []byte(`<a:rPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` + string(paint) + `</a:rPr>`)
				run = append(append(append([]byte{}, run[:offset]...), insert...), run[offset:]...)
			}
			replacements[i] = run
		}
		shape = motionEdit(shape, spans, replacements)
	} else {
		parent := "spPr"
		if effect.Property == "stroke" {
			parent = "ln"
		}
		fills, err := motionSpans(shape, parent, "solidFill", "gradFill")
		if err != nil || len(fills) != 1 {
			return nil, fmt.Errorf("explicit paint required")
		}
		fill := shape[fills[0].start:fills[0].end]
		if effect.gradient {
			stops, _ := motionSpans(fill, "gsLst", "gs")
			if len(stops) != len(effect.sourceColors) {
				return nil, fmt.Errorf("gradient stop mismatch")
			}
			replacements := make([][]byte, len(stops))
			for i, stop := range stops {
				value := fill[stop.start:stop.end]
				colors, _ := motionSpans(value, "gs", "srgbClr", "schemeClr", "hslClr", "sysClr")
				if len(colors) != 1 {
					return nil, fmt.Errorf("invalid stop color")
				}
				replacements[i] = motionEdit(value, colors, [][]byte{motionColorXML(motionPaintTween(effect.sourceColors[i], effect, t))})
			}
			fill = motionEdit(fill, stops, replacements)
		} else {
			fill = []byte(`<a:solidFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` + string(motionColorXML(motionPaintTween(effect.sourceColors[0], effect, t))) + `</a:solidFill>`)
		}
		shape = motionEdit(shape, fills, [][]byte{fill})
	}
	return motionEdit(body, shapes, [][]byte{shape}), nil
}

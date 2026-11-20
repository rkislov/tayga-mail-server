package httpapi

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ODF's presentation animation vocabulary has no gradient geometry properties.
// These literal attributeName values are the documented Tayga profile, not
// native Impress animation names. Unknown properties always remain unsupported.
func motionODPGradientAnimation(node *compareNode, e *motionEffect) error {
	e.Kind = "gradient"
	e.Property = map[string]string{"tayga:gradient-angle": "angle", "tayga:gradient-center-x": "focus-x", "tayga:gradient-center-y": "focus-y", "tayga:gradient-stops": "stops", "tayga:gradient-focus-width": "focus-width", "tayga:gradient-focus-height": "focus-height"}[node.attr("attributeName")]
	if e.Property == "" {
		return fmt.Errorf("unsupported ODP gradient property")
	}
	for _, key := range []string{"values", "keyTimes", "keySplines", "formula", "additive", "accumulate"} {
		if node.attr(key) != "" {
			return fmt.Errorf("unsupported gradient timing or formula")
		}
	}
	mode := node.attr("calcMode")
	if mode != "" && mode != "linear" && mode != "discrete" {
		return fmt.Errorf("unsupported gradient interpolation")
	}
	e.Discrete = mode == "discrete"
	duration, err := motionMS(node.attr("dur"), true)
	if err != nil || duration == 0 {
		return fmt.Errorf("missing gradient duration")
	}
	e.Duration = duration
	return motionGradientOperands(node, e)
}

func motionODPGradientValue(gradient *compareNode, property string) (float64, error) {
	if property == "angle" {
		raw := gradient.attr("angle")
		if raw == "" {
			return 0, nil
		}
		if strings.HasSuffix(raw, "deg") {
			return motionNumber(strings.TrimSuffix(raw, "deg"), 0, 360)
		}
		if strings.ContainsAny(raw, ".eE") {
			return 0, fmt.Errorf("fractional gradient angle requires explicit deg units")
		}
		v, err := motionNumber(raw, 0, 3600)
		return v / 10, err
	}
	if property == "focus-width" || property == "focus-height" {
		key := "focus-width"
		if property == "focus-height" {
			key = "focus-height"
		}
		for _, a := range gradient.attrs {
			if a.Name.Space == motionODPFocusNamespace && a.Name.Local == key {
				return motionNumber(a.Value, 0, 1)
			}
		}
		return 0, nil
	}
	key := "cx"
	if property == "focus-y" {
		key = "cy"
	}
	raw := gradient.attr(key)
	if raw == "" {
		return .5, nil
	}
	if !strings.HasSuffix(raw, "%") {
		return 0, fmt.Errorf("gradient center requires percentage")
	}
	v, err := motionNumber(strings.TrimSuffix(raw, "%"), 0, 100)
	return v / 100, err
}

func motionPrepareODPGradient(pkg *comparePackage, slide *motionSlide, page *compareNode) {
	if slide.Warning != "" {
		return
	}
	styles := motionODPStylesMap(pkg)
	for i := range slide.Effects {
		if slide.Warning != "" {
			return
		}
		e := &slide.Effects[i]
		if e.Kind != "gradient" {
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
			if motionStyleValue(styles, name, "graphic-properties", "fill") != "gradient" {
				continue
			}
			gradient, err := motionODPGradient(pkg, motionStyleValue(styles, name, "graphic-properties", "fill-gradient-name"))
			if err != nil {
				continue
			}
			style := gradient.attr("style")
			if e.Property == "stops" {
				if style != "linear" && style != "axial" && style != "radial" && style != "ellipsoid" && style != "square" && style != "rectangular" {
					continue
				}
				positions := []float64{0, 1}
				if len(gradient.children) > 0 {
					positions = nil
					for _, stop := range gradient.children {
						v, err := motionNumber(stop.attr("offset"), 0, 1)
						if err != nil {
							positions = nil
							break
						}
						positions = append(positions, v)
					}
				}
				positions = motionStageSource(slide, *e, positions)
				if !motionResolveGradientStops(e, positions) {
					continue
				}
				e.sourceColors, err = motionODPGradientColors(gradient)
				if err != nil {
					continue
				}
				if err := motionODPSetupFocus(e, gradient); err != nil {
					continue
				}
				e.gradient, found = true, true
				continue
			} else if e.Property == "angle" {
				if style != "linear" && style != "axial" {
					continue
				}
			} else if style != "radial" && style != "ellipsoid" && style != "square" && style != "rectangular" {
				continue
			}
			if err := motionODPSetupFocus(e, gradient); err != nil {
				continue
			}

			source, err := motionODPGradientValue(gradient, e.Property)
			if err == nil {
				source = motionStageSource(slide, *e, []float64{source})[0]
			}
			if err != nil || (len(e.From) > 0 && math.Abs(e.From[0]-source) > 0.00001) {
				continue
			}
			e.From = []float64{source}
			if e.Relative {
				e.To = []float64{source + e.Delta[0]}
			}
			if e.Property == "angle" {
				if math.Abs(e.To[0]-source) > 360 {
					continue
				}
			} else if e.To[0] < 0 || e.To[0] > 1 {
				continue
			}
			if e.Property != "angle" {
				index := 0
				if e.Property == "focus-y" || e.Property == "focus-height" {
					index = 1
				}
				center := (e.focusRect[index] + 1 - e.focusRect[index+2]) / 2
				half := (1 - e.focusRect[index] - e.focusRect[index+2]) / 2
				if e.Property == "focus-width" || e.Property == "focus-height" {
					half = e.To[0] / 2
				} else {
					center = e.To[0]
				}
				if center-half < -1e-9 || center+half > 1+1e-9 {
					continue
				}
			}
			if motionNeedsFocusRaster(*e) && e.focusPath == "circle" && (e.Property == "focus-x" || e.Property == "focus-y") && math.Abs(e.To[0]-.5) > 0.00001 {
				continue
			}
			e.sourceColors, err = motionODPGradientColors(gradient)
			if err != nil {
				continue
			}

			e.gradient = true
			found = true
		}
		if !found {
			slide.Warning = "Unsupported ODP gradient geometry or mismatched source; static slide"
		}
	}
}

func motionODPGradientGeometry(body []byte, e motionEffect, phase int) ([]byte, error) {
	t := float64(phase-1) / float64(motionSamples(e)-1)
	values := motionGradientPhase(e, t)
	if e.Property == "stops" {
		nodes, err := motionSpans(body, "gradient", "gradient-stop")
		if err != nil {
			return nil, err
		}
		if len(nodes) == 0 {
			if len(values) != 2 {
				return nil, fmt.Errorf("gradient stop count changed")
			}
			for i, key := range []string{"start-color", "end-color"} {
				// The cloned two-color definition has the current joint paint colors.
				roots, err := motionSpans(body, "", "gradient")
				if err != nil || len(roots) != 1 {
					return nil, fmt.Errorf("gradient definition missing")
				}
				color := ""
				for _, a := range roots[0].attrs {
					if a.Name.Local == key {
						color = a.Value
					}
				}
				if _, err := motionRGB(color); err != nil {
					return nil, err
				}
				stop := fmt.Sprintf(`<loext:gradient-stop xmlns:loext="urn:org:documentfoundation:names:experimental:office:xmlns:loext:1.0" xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" svg:offset="%s" loext:color-type="rgb" loext:color-value="%s"/>`, strconv.FormatFloat(values[i], 'f', 6, 64), motionXMLAttr(color))
				body, err = motionAppendRoot(body, "gradient", stop)
				if err != nil {
					return nil, err
				}
			}
			return body, nil
		}
		if len(nodes) != len(values) {
			return nil, fmt.Errorf("gradient stop count changed")
		}
		replacements := make([][]byte, len(nodes))
		for i, node := range nodes {
			replacements[i] = motionRootAttr(body[node.start:node.end], "offset", strconv.FormatFloat(values[i], 'f', 6, 64), "svg:offset", "urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0")
		}
		return motionEdit(body, nodes, replacements), nil
	}
	v := values[0]
	key, value := "angle", ""
	if e.Property == "angle" {
		v = math.Mod(v, 360)
		if v < 0 {
			v += 360
		}
		value = strconv.FormatFloat(v, 'f', 6, 64) + "deg"
	} else {
		key = "cx"
		if e.Property == "focus-y" {
			key = "cy"
		}
		value = strconv.FormatFloat(v*100, 'f', 4, 64) + "%"
	}
	return motionRootAttr(body, key, value, "draw:"+key, "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0"), nil
}

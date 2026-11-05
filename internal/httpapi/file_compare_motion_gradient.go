package httpapi

import (
	"fmt"
	"math"
	"strconv"
)

func motionGradientAnimation(node *compareNode, e *motionEffect) error {
	e.Kind = "gradient"
	e.Property = map[string]string{"fill.angle": "angle", "fill.focusposition.x": "focus-x", "fill.focusposition.y": "focus-y", "fill.focussize.x": "focus-width", "fill.focussize.y": "focus-height", "tayga:gradient-stops": "stops"}[node.child("cBhvr").child("attrNameLst").text()]
	valueType := "num"
	if e.Property == "stops" {
		valueType = "str"
	}
	if e.Property == "" || node.attr("valueType") != valueType || node.child("tavLst") != nil {
		return fmt.Errorf("unsupported gradient geometry animation")
	}
	if node.attr("calcmode") != "" && node.attr("calcmode") != "lin" && node.attr("calcmode") != "discrete" {
		return fmt.Errorf("unsupported gradient interpolation")
	}
	e.Discrete = node.attr("calcmode") == "discrete"
	duration, err := motionMS(node.child("cBhvr").child("cTn").attr("dur"), false)
	if err != nil || duration == 0 {
		return fmt.Errorf("missing gradient duration")
	}
	e.Duration = duration
	return motionGradientOperands(node, e)
}

func motionPrepareGradient(pkg *comparePackage, part string, root *compareNode, slide *motionSlide, shapes []*compareNode) {
	if slide.Warning != "" {
		return
	}
	for i := range slide.Effects {
		e := &slide.Effects[i]
		if e.Kind != "gradient" {
			continue
		}
		found := false
		for _, shape := range shapes {
			props := shape.descendants("cNvPr")
			if len(props) == 0 || props[0].attr("id") != e.Target || shape.name.Local != "sp" {
				continue
			}
			paint := shape.child("spPr").child("gradFill")
			if paint == nil || paint.attr("rotWithShape") == "0" {
				continue
			}
			stops := paint.child("gsLst").all("gs")
			if len(stops) < 2 || len(stops) > 16 {
				continue
			}
			source := 0.0
			if e.Property == "stops" {
				positions := make([]float64, len(stops))
				valid := true
				for i, stop := range stops {
					v, err := motionNumber(stop.attr("pos"), 0, 100000)
					if err != nil {
						valid = false
					}
					positions[i] = v / 100000
				}
				if !valid || !motionResolveGradientStops(e, positions) {
					continue
				}
				found = true
				continue
			} else if e.Property == "angle" {
				lin := paint.child("lin")
				if lin == nil || paint.child("path") != nil {
					continue
				}
				rawAngle := lin.attr("ang")
				if rawAngle == "" {
					rawAngle = "0"
				}
				angle, err := motionNumber(rawAngle, 0, 21600000)
				if err != nil {
					continue
				}
				source = math.Mod(360-angle/60000, 360)
			} else {
				path := paint.child("path")
				if path == nil || (path.attr("path") != "circle" && path.attr("path") != "rect") {
					continue
				}
				rect := path.child("fillToRect")
				if rect == nil {
					continue
				}
				values := []float64{}
				valid := true
				for _, key := range []string{"l", "t", "r", "b"} {
					v := 0.0
					if rect.attr(key) != "" {
						var err error
						v, err = motionNumber(rect.attr(key), 0, 100000)
						if err != nil {
							valid = false
						}
					}
					values = append(values, v/100000)
				}
				if !valid || values[0]+values[2] > 1 || values[1]+values[3] > 1 {
					continue
				}
				copy(e.focusRect[:], values)
				if e.Property == "focus-width" {
					source = 1 - values[0] - values[2]
				} else if e.Property == "focus-height" {
					source = 1 - values[1] - values[3]
				} else if e.Property == "focus-x" {
					source = (values[0] + 1 - values[2]) / 2
				} else {
					source = (values[1] + 1 - values[3]) / 2
				}
			}
			if len(e.From) > 0 && math.Abs(e.From[0]-source) > 0.00001 {
				continue
			}
			e.From = []float64{source}
			if e.Relative {
				e.To = []float64{source + e.Delta[0]}
			}
			if e.Property == "angle" && math.Abs(e.To[0]-e.From[0]) > 360 {
				continue
			}
			if e.Property != "angle" {
				index := 0
				if e.Property == "focus-y" || e.Property == "focus-height" {
					index = 1
				}
				half := (1 - e.focusRect[index] - e.focusRect[index+2]) / 2
				center := e.To[0]
				if e.Property == "focus-width" || e.Property == "focus-height" {
					center = (e.focusRect[index] + 1 - e.focusRect[index+2]) / 2
					half = e.To[0] / 2
				}
				if e.To[0] < 0 || e.To[0] > 1 || center-half < 0 || center+half > 1 {
					continue
				}
			}
			if e.Property == "focus-width" || e.Property == "focus-height" {
				e.focusPath = paint.child("path").attr("path")
				if e.focusPath == "circle" && (math.Abs(e.focusRect[0]-e.focusRect[2]) > 0.00001 || math.Abs(e.focusRect[1]-e.focusRect[3]) > 0.00001) {
					continue
				}
				if paint.child("tileRect") != nil || (paint.attr("flip") != "" && paint.attr("flip") != "none") {
					continue
				}
				palette, err := motionSlideTheme(pkg, part, root)
				if err != nil {
					continue
				}
				colorEffect := motionEffect{Property: "fill"}
				e.sourceColors, err = motionPPTXSourceColors(shape, &colorEffect, palette)
				if err != nil {
					continue
				}
				for _, stop := range stops {
					position, err := motionNumber(stop.attr("pos"), 0, 100000)
					if err != nil {
						break
					}
					e.focusStops = append(e.focusStops, position/100000)
				}
				if len(e.focusStops) != len(stops) || !motionOrderedStops(e.focusStops) {
					continue
				}
			}
			found = true
		}
		if !found {
			slide.Warning = "Unsupported gradient geometry or mismatched initial value; static slide"
		}
	}
}
func motionPPTXGradientBody(body []byte, e motionEffect, phase int) ([]byte, error) {
	if phase == 1 {
		return body, nil
	}
	t := float64(phase-1) / float64(motionSamples(e)-1)
	values := motionGradientPhase(e, t)
	value := values[0]
	fills, err := motionSpans(body, "spPr", "gradFill")
	if err != nil || len(fills) != 1 {
		return nil, fmt.Errorf("explicit fill gradient required")
	}
	fill := body[fills[0].start:fills[0].end]
	if e.Property == "stops" {
		nodes, err := motionSpans(fill, "gsLst", "gs")
		if err != nil || len(nodes) != len(values) {
			return nil, fmt.Errorf("gradient stop count changed")
		}
		replacements := make([][]byte, len(nodes))
		for i, node := range nodes {
			replacements[i] = motionRootAttr(fill[node.start:node.end], "pos", strconv.FormatInt(int64(math.Round(values[i]*100000)), 10), "pos", "")
		}
		fill = motionEdit(fill, nodes, replacements)
	} else if e.Property == "angle" {
		nodes, err := motionSpans(fill, "gradFill", "lin")
		if err != nil || len(nodes) != 1 {
			return nil, fmt.Errorf("linear gradient required")
		}
		angle := math.Mod(-value, 360)
		if angle < 0 {
			angle += 360
		}
		replacement := motionRootAttr(fill[nodes[0].start:nodes[0].end], "ang", strconv.FormatInt(int64(math.Round(angle*60000)), 10), "ang", "")
		fill = motionEdit(fill, nodes, [][]byte{replacement})
	} else {
		nodes, err := motionSpans(fill, "path", "fillToRect")
		if err != nil || len(nodes) != 1 {
			return nil, fmt.Errorf("radial gradient focus required")
		}
		index := 0
		if e.Property == "focus-y" || e.Property == "focus-height" {
			index = 1
		}
		half := (1 - e.focusRect[index] - e.focusRect[index+2]) / 2
		if e.Property == "focus-width" || e.Property == "focus-height" {
			center := (e.focusRect[index] + 1 - e.focusRect[index+2]) / 2
			half, value = value/2, center
		}
		keys := []string{"l", "r"}
		if index == 1 {
			keys = []string{"t", "b"}
		}
		replacement := fill[nodes[0].start:nodes[0].end]
		for i, coordinate := range []float64{value - half, 1 - value - half} {
			replacement = motionRootAttr(replacement, keys[i], strconv.FormatInt(int64(math.Round(coordinate*100000)), 10), keys[i], "")
		}
		fill = motionEdit(fill, nodes, [][]byte{replacement})
	}
	return motionEdit(body, fills, [][]byte{fill}), nil
}

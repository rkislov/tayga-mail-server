package httpapi

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type motionColorLayer struct {
	Effect         int                 `json:"effect"`
	GeometryEffect *int                `json:"geometry_effect,omitempty"`
	FromBlack      int                 `json:"from_black"`
	FromWhite      int                 `json:"from_white"`
	ToBlack        int                 `json:"to_black"`
	ToWhite        int                 `json:"to_white"`
	Samples        []motionColorSample `json:"samples,omitempty"`
}

func motionTransformKind(kind string) bool {
	return kind == "rotate" || kind == "scale" || kind == "color" || kind == "gradient"
}
func motionAppearanceKind(kind string) bool { return kind != "cut" && kind != "fade" }
func motionNumber(value string, min, max float64) (float64, error) {
	v, e := strconv.ParseFloat(value, 64)
	if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
		return 0, fmt.Errorf("unsupported transform value")
	}
	return v, nil
}
func motionRGB(value string) ([]float64, error) {
	value = strings.TrimPrefix(value, "#")
	if len(value) != 6 {
		return nil, fmt.Errorf("explicit RGB color required")
	}
	n, e := strconv.ParseUint(value, 16, 24)
	if e != nil {
		return nil, fmt.Errorf("invalid RGB color")
	}
	return []float64{float64(n >> 16), float64(n >> 8 & 255), float64(n & 255)}, nil
}
func motionHex(color []float64) string {
	return fmt.Sprintf("%02X%02X%02X", int(color[0]), int(color[1]), int(color[2]))
}
func motionPPTXTransform(node *compareNode, effect *motionEffect) error {
	effect.Duration = 0
	duration, e := motionMS(node.child("cBhvr").child("cTn").attr("dur"), false)
	if e != nil || duration == 0 {
		return fmt.Errorf("missing transform duration")
	}
	effect.Duration = duration
	switch node.name.Local {
	case "animRot":
		effect.Kind = "rotate"
		effect.From = []float64{0}
		var angle float64
		if node.attr("by") != "" {
			if node.attr("to") != "" || node.attr("from") != "" {
				return fmt.Errorf("mixed rotation operands")
			}
			angle, e = motionNumber(node.attr("by"), -216000000, 216000000)
		} else {
			if node.attr("from") != "" && node.attr("from") != "0" {
				return fmt.Errorf("nonzero rotation origin")
			}
			angle, e = motionNumber(node.attr("to"), -216000000, 216000000)
		}
		effect.To = []float64{angle / 60000}
		return e
	case "animScale":
		effect.Kind = "scale"
		effect.From = []float64{1, 1}
		target := node.child("to")
		if target == nil {
			target = node.child("by")
		} else if node.child("by") != nil {
			return fmt.Errorf("mixed scale operands")
		}
		if target == nil {
			return fmt.Errorf("missing scale operand")
		}
		if from := node.child("from"); from != nil && (from.attr("x") != "100000" || from.attr("y") != "100000") {
			return fmt.Errorf("nonidentity scale origin")
		}
		x, e := motionNumber(target.attr("x"), 1000, 800000)
		if e != nil {
			return e
		}
		y, e := motionNumber(target.attr("y"), 1000, 800000)
		effect.To = []float64{x / 100000, y / 100000}
		return e
	case "animClr":
		effect.Kind = "color"
		effect.Property = map[string]string{"fillcolor": "fill", "stroke.color": "stroke", "style.color": "text"}[node.child("cBhvr").child("attrNameLst").text()]
		if effect.Property == "" {
			return fmt.Errorf("unsupported color property/relative color")
		}
		effect.Space = node.attr("clrSpc")
		if effect.Space == "" {
			effect.Space = "rgb"
		}
		if effect.Space != "rgb" && effect.Space != "hsl" {
			return fmt.Errorf("unsupported color space")
		}
		effect.Direction = node.attr("dir")
		if effect.Direction == "" {
			effect.Direction = "cw"
		}
		if effect.Direction != "cw" && effect.Direction != "ccw" {
			return fmt.Errorf("unsupported hue direction")
		}
		effect.fromRef = node.child("from")
		effect.toRef = node.child("to")
		if effect.toRef == nil {
			effect.byRef = node.child("by")
			if err := motionPPTXRelative(effect.byRef, effect); err != nil {
				return err
			}
		}
		return nil

	}
	return fmt.Errorf("unsupported transform")
}
func motionODPTransform(node *compareNode, effect *motionEffect) error {
	duration, e := motionMS(node.attr("dur"), true)
	if e != nil || duration == 0 {
		return fmt.Errorf("missing transform duration")
	}
	effect.Duration = duration
	if node.attr("formula") != "" || node.attr("values") != "" || node.attr("keyTimes") != "" || node.attr("keySplines") != "" || (node.attr("calcMode") != "" && node.attr("calcMode") != "linear" && node.attr("calcMode") != "discrete") {
		return fmt.Errorf("unsupported transform interpolation")
	}
	if node.name.Local == "animateColor" {
		effect.Kind = "color"
		effect.Property = map[string]string{"fill-color": "fill", "stroke-color": "stroke", "color": "text"}[node.attr("attributeName")]
		if effect.Property == "" {
			return fmt.Errorf("unsupported color property")
		}
		effect.Space = node.attr("color-interpolation")
		if effect.Space == "" {
			effect.Space = "rgb"
		}
		effect.Direction = map[string]string{"": "cw", "clockwise": "cw", "counter-clockwise": "ccw", "counterclockwise": "ccw"}[node.attr("color-interpolation-direction")]
		if (effect.Space != "rgb" && effect.Space != "hsl") || effect.Direction == "" {
			return fmt.Errorf("unsupported color interpolation")
		}
		effect.Discrete = node.attr("calcMode") == "" || node.attr("calcMode") == "discrete"
		if node.attr("from") != "" {
			effect.From, e = motionRGB(node.attr("from"))
			if e != nil {
				return e
			}
		}
		if node.attr("to") == "" {
			return motionODPRelative(node.attr("by"), effect)
		}
		effect.To, e = motionRGB(node.attr("to"))
		return e
	}
	if node.attr("calcMode") == "discrete" {
		return fmt.Errorf("discrete geometric transform unsupported")
	}
	switch node.attr("type") {
	case "rotate":
		effect.Kind = "rotate"
		effect.From = []float64{0}
		if node.attr("from") != "" && node.attr("from") != "0" {
			return fmt.Errorf("nonzero/custom rotation origin")
		}
		value := node.attr("to")
		if node.attr("by") != "" {
			if value != "" {
				return fmt.Errorf("mixed rotation operands")
			}
			value = node.attr("by")
		}
		to, e := motionNumber(value, -3600, 3600)
		effect.To = []float64{to}
		return e
	case "scale":
		effect.Kind = "scale"
		effect.From = []float64{1, 1}
		parse := func(value string) ([]float64, error) {
			fields := strings.Fields(strings.ReplaceAll(value, ",", " "))
			if len(fields) < 1 || len(fields) > 2 {
				return nil, fmt.Errorf("invalid scale vector")
			}
			x, e := motionNumber(fields[0], .01, 8)
			if e != nil {
				return nil, e
			}
			y := x
			if len(fields) == 2 {
				y, e = motionNumber(fields[1], .01, 8)
			}
			return []float64{x, y}, e
		}
		if node.attr("from") != "" {
			from, e := parse(node.attr("from"))
			if e != nil || from[0] != 1 || from[1] != 1 {
				return fmt.Errorf("nonidentity scale origin")
			}
		}
		if node.attr("by") != "" {
			if node.attr("to") != "" {
				return fmt.Errorf("mixed scale operands")
			}
			parts := strings.Fields(strings.ReplaceAll(node.attr("by"), ",", " "))
			if len(parts) < 1 || len(parts) > 2 {
				return fmt.Errorf("invalid relative scale")
			}
			x, e := motionNumber(parts[0], -.99, 7)
			if e != nil {
				return e
			}
			y := x
			if len(parts) == 2 {
				y, e = motionNumber(parts[1], -.99, 7)
				if e != nil {
					return e
				}
			}
			effect.To = []float64{1 + x, 1 + y}
			return nil
		}
		effect.To, e = parse(node.attr("to"))
		return e
	}
	return fmt.Errorf("unsupported transform type")
}

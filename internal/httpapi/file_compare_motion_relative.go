package httpapi

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

func motionRelativeColor(source []float64, e motionEffect, t float64) []float64 {
	if e.DeltaSpace == "hsl" {
		hsl := motionRGBToHSL(source)
		hsl[0] = math.Mod(hsl[0]+e.Delta[0]*t+36000, 360)
		hsl[1] = math.Max(0, math.Min(1, hsl[1]+e.Delta[1]*t))
		hsl[2] = math.Max(0, math.Min(1, hsl[2]+e.Delta[2]*t))
		return motionHSLToRGB(hsl)
	}
	result := make([]float64, 3)
	for i, v := range source {
		result[i] = math.Round(math.Max(0, math.Min(255, v+e.Delta[i]*t)))
	}
	return result
}
func motionPaintTween(source []float64, e motionEffect, t float64) []float64 {
	if e.Relative {
		return motionRelativeColor(source, e, t)
	}
	return motionColorTween(source, e.To, t, e.Space, e.Direction)
}
func motionPPTXRelative(node *compareNode, e *motionEffect) error {
	if node == nil || len(node.children) != 1 {
		return fmt.Errorf("one relative color operand required")
	}
	operand := node.children[0]
	e.Relative = true
	e.Delta = []float64{0, 0, 0}
	switch operand.name.Local {
	case "rgb":
		e.DeltaSpace = "rgb"
		for i, key := range []string{"r", "g", "b"} {
			v, err := motionNumber(operand.attr(key), -100000, 100000)
			if err != nil {
				return err
			}
			e.Delta[i] = v * 255 / 100000
		}
	case "hsl":
		e.DeltaSpace = "hsl"
		h, err := motionNumber(operand.attr("h"), -21600000, 21600000)
		if err != nil {
			return err
		}
		e.Delta[0] = h / 60000
		for i, key := range []string{"s", "l"} {
			v, err := motionNumber(operand.attr(key), -100000, 100000)
			if err != nil {
				return err
			}
			e.Delta[i+1] = v / 100000
		}
	default:
		return fmt.Errorf("unsupported relative color operand")
	}
	if len(operand.children) > 0 || e.DeltaSpace != e.Space {
		return fmt.Errorf("mixed relative color spaces unsupported")
	}
	return nil
}

var motionRelativeCSS = regexp.MustCompile(`^(rgb|hsl)\(([^()]*)\)$`)

func motionODPRelative(value string, e *motionEffect) error {
	e.Relative = true
	e.DeltaSpace = e.Space
	if strings.HasPrefix(value, "#") {
		if e.Space != "rgb" {
			return fmt.Errorf("relative color space mismatch")
		}
		var err error
		e.Delta, err = motionRGB(value)
		return err
	}
	match := motionRelativeCSS.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil || match[1] != e.Space {
		return fmt.Errorf("unsupported relative color syntax")
	}
	parts := strings.Split(match[2], ",")
	if len(parts) != 3 {
		return fmt.Errorf("relative color requires three components")
	}
	e.Delta = make([]float64, 3)
	for i, raw := range parts {
		part := strings.TrimSpace(raw)
		percent := strings.HasSuffix(part, "%")
		part = strings.TrimSuffix(part, "%")
		var v float64
		var err error
		if e.Space == "hsl" && i == 0 {
			if percent {
				return fmt.Errorf("hue must use degrees")
			}
			v, err = motionNumber(part, -360, 360)
		} else if e.Space == "hsl" {
			if !percent {
				return fmt.Errorf("relative HSL saturation/lightness require percentages")
			}
			v, err = motionNumber(part, -100, 100)
			v /= 100
		} else if percent {
			v, err = motionNumber(part, -100, 100)
			v *= 2.55
		} else {
			v, err = motionNumber(part, -255, 255)
		}
		if err != nil {
			return err
		}
		e.Delta[i] = v
	}
	return nil
}

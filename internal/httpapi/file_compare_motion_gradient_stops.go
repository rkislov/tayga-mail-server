package httpapi

import (
	"fmt"
	"math"
	"strings"
)

func motionGradientOperands(node *compareNode, e *motionEffect) error {
	min, max := -3600.0, 3600.0
	if e.Property != "angle" {
		min, max = 0, 1
	}
	parse := func(raw string, min, max float64) ([]float64, error) {
		parts := strings.Split(raw, ",")
		if e.Property == "stops" {
			if len(parts) < 2 || len(parts) > 16 {
				return nil, fmt.Errorf("gradient requires 2–16 positions")
			}
		} else if len(parts) != 1 {
			return nil, fmt.Errorf("scalar gradient value required")
		}
		values := make([]float64, len(parts))
		for i, part := range parts {
			v, err := motionNumber(strings.TrimSpace(part), min, max)
			if err != nil {
				return nil, err
			}
			values[i] = v
		}
		return values, nil
	}
	var err error
	if raw := node.attr("from"); raw != "" {
		e.From, err = parse(raw, min, max)
		if err != nil {
			return err
		}
	}
	if raw := node.attr("to"); raw != "" {
		e.To, err = parse(raw, min, max)
		return err
	}
	if e.Property != "angle" {
		min, max = -1, 1
	}
	e.Delta, err = parse(node.attr("by"), min, max)
	if err != nil {
		return err
	}
	e.Relative, e.DeltaSpace = true, "geometry"
	return nil
}

func motionOrderedStops(values []float64) bool {
	if len(values) < 2 || len(values) > 16 {
		return false
	}
	for i, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 || i > 0 && v < values[i-1] {
			return false
		}
	}
	return true
}

func motionResolveGradientStops(e *motionEffect, source []float64) bool {
	if !motionOrderedStops(source) {
		return false
	}
	if len(e.From) > 0 {
		if len(e.From) != len(source) {
			return false
		}
		for i, v := range source {
			if math.Abs(e.From[i]-v) > 0.00001 {
				return false
			}
		}
	}
	e.From = append([]float64{}, source...)
	if e.Relative {
		if len(e.Delta) != len(source) {
			return false
		}
		e.To = make([]float64, len(source))
		for i, v := range source {
			e.To[i] = v + e.Delta[i]
		}
	}
	return len(e.To) == len(source) && motionOrderedStops(e.To)
}

func motionGradientPhase(e motionEffect, t float64) []float64 {
	values := make([]float64, len(e.From))
	for i, v := range e.From {
		values[i] = v + (e.To[i]-v)*t
	}
	return values
}

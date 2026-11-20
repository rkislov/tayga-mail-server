package httpapi

import (
	"fmt"
	"math"
)

func motionMultiPaintGroups(slide *motionSlide) [][]int {
	groups := [][]int{}
	seen := map[string]bool{}
	for _, e := range slide.Effects {
		if e.Kind != "gradient" || seen[e.Target] {
			continue
		}
		seen[e.Target] = true
		axes := []int{}
		colors := []int{}
		for i, g := range slide.Effects {
			if g.Target == e.Target && g.Previous == nil {
				if g.Kind == "gradient" {
					axes = append(axes, i)
				} else if g.Kind == "color" && g.Property == "fill" {
					colors = append(colors, i)
				}
			}
		}
		if len(axes) > 1 {
			groups = append(groups, append(colors, axes...))
		}
	}
	return groups
}

func motionMultiPhases(slide *motionSlide, axes []int) ([][]int, error) {
	if len(axes) < 2 || len(axes) > 4 {
		return nil, fmt.Errorf("at most three geometry effects and one fill color are supported")
	}
	counts := make([]int, len(axes))
	product := 1
	for i, axis := range axes {
		e := slide.Effects[axis]
		counts[i] = 2
		if !e.Discrete && (e.Kind == "color" && motionSamples(e) == 9 || e.Property == "angle" && math.Abs(e.To[0]-e.From[0]) >= 180) {
			counts[i] = 9
		}
		product *= counts[i]
	}
	if product > 45 {
		return nil, fmt.Errorf("independent cyclic effects exceed the 45-state paint grid")
	}
	for i, axis := range axes {
		if counts[i] == 2 && motionSamples(slide.Effects[axis]) == 9 && product/2*3 <= 45 {
			product = product / 2 * 3
			counts[i] = 3
		}
	}
	result := make([][]int, len(axes))
	for i, count := range counts {
		last := motionSamples(slide.Effects[axes[i]]) - 1
		for j := 0; j < count; j++ {
			result[i] = append(result[i], 1+j*last/(count-1))
		}
	}
	return result, nil
}

func motionWalkPhases(phases [][]int, visit func([]int)) {
	values := make([]int, len(phases))
	var walk func(int)
	walk = func(axis int) {
		if axis == len(phases) {
			visit(values)
			return
		}
		for _, phase := range phases[axis] {
			values[axis] = phase
			walk(axis + 1)
		}
	}
	walk(0)
}

func motionRegisterMultiPaint(slide *motionSlide, plan motionLayerPlan, frame int) {
	positions, first, last := motionTrackPositions(plan, slide)
	if first && plan.matte == "000000" {
		slide.Colors = append(slide.Colors, motionColorLayer{Effect: plan.paintAxes[0], Axes: plan.paintAxes, Tracks: plan.paintTracks})
	}
	paint := &slide.Colors[len(slide.Colors)-1]
	if plan.matte == "000000" {
		paint.Samples = append(paint.Samples, motionColorSample{At: positions[0], Positions: positions, Black: frame})
	} else {
		paint.Samples[len(paint.Samples)-1].White = frame
	}
	if first {
		if plan.matte == "000000" {
			paint.FromBlack = frame
		} else {
			paint.FromWhite = frame
		}
	}
	if last {
		if plan.matte == "000000" {
			paint.ToBlack = frame
		} else {
			paint.ToWhite = frame
		}
	}
}

func motionPlanPhases(plan *motionLayerPlan) map[int]int {
	values := map[int]int{}
	if plan == nil {
		return values
	}
	if len(plan.paintAxes) > 0 {
		if len(plan.paintPoints) > 0 {
			for i := range plan.paintAxes {
				point := plan.paintPoints[i][plan.paintPhases[i]-1]
				values[point.Effect] = point.Phase
			}
			return values
		}
		for i, axis := range plan.paintAxes {
			values[axis] = plan.paintPhases[i]
		}
		return values
	}
	if plan.colorPhase > 0 {
		values[plan.colorEffect] = plan.colorPhase
	}
	if plan.geometryPhase > 0 {
		values[plan.geometryEffect] = plan.geometryPhase
	}
	return values
}

func motionEffectPhase(e motionEffect, phase int) []float64 {
	if phase == 0 {
		phase = 1
	}
	return motionGradientPhase(e, float64(phase-1)/float64(motionSamples(e)-1))
}

func motionCombinedFocus(slide *motionSlide, target string, phases map[int]int, base motionEffect) motionEffect {
	rect := base.focusRect
	cx, cy := (rect[0]+1-rect[2])/2, (rect[1]+1-rect[3])/2
	w, h := 1-rect[0]-rect[2], 1-rect[1]-rect[3]
	for i, e := range slide.Effects {
		if e.Kind != "gradient" || e.Target != target || !motionSnapshotSelected(slide, i, phases) {
			continue
		}
		v := motionEffectPhase(e, phases[i])
		switch e.Property {
		case "focus-x":
			cx = v[0]
		case "focus-y":
			cy = v[0]
		case "focus-width":
			w = v[0]
		case "focus-height":
			h = v[0]
		case "stops":
			base.focusStops = v
		}
	}
	base.focusRect = [4]float64{cx - w/2, cy - h/2, 1 - cx - w/2, 1 - cy - h/2}
	if base.Property == "focus-height" {
		base.From, base.To = []float64{h}, []float64{h}
	} else {
		base.From, base.To = []float64{w}, []float64{w}
	}
	return base
}

func motionValidateGradientGroups(slide *motionSlide) {
	if slide.Warning != "" {
		return
	}
	for _, axes := range motionMultiPaintGroups(slide) {
		geometry := 0
		for _, axis := range axes {
			if slide.Effects[axis].Kind == "gradient" {
				geometry++
			}
		}
		if geometry > 3 {
			slide.Warning = "More than three geometry effects on one object; static slide"
			return
		}
		phases, err := motionMultiPhases(slide, axes)
		if err != nil {
			slide.Warning = err.Error() + "; static slide"
			return
		}
		var focus *motionEffect
		hasSize := false
		for _, axis := range axes {
			e := slide.Effects[axis]
			if motionNeedsFocusRaster(e) {
				hasSize = true
			}
		}
		for _, axis := range axes {
			e := &slide.Effects[axis]
			if e.Property == "focus-x" || e.Property == "focus-y" || e.Property == "focus-width" || e.Property == "focus-height" {
				focus = e
				break
			}
		}
		if focus == nil {
			continue
		}
		// All combinations of independent endpoints must fit, even if their
		// effects are delayed or belong to different interactive clocks.
		corners := make([][]int, len(axes))
		for i, p := range phases {
			corners[i] = []int{p[0], p[len(p)-1]}
		}
		motionWalkPhases(corners, func(values []int) {
			m := map[int]int{}
			for i, axis := range axes {
				m[axis] = values[i]
			}
			state := motionCombinedFocus(slide, focus.Target, m, *focus)
			for _, v := range state.focusRect {
				if v < -1e-9 || v > 1+1e-9 {
					slide.Warning = "Combined focus center/size escapes the fill; static slide"
				}
			}
			for _, axis := range axes {
				e := slide.Effects[axis]
				if hasSize && e.focusPath == "circle" && (math.Abs(state.focusRect[0]-state.focusRect[2]) > 0.00001 || math.Abs(state.focusRect[1]-state.focusRect[3]) > 0.00001) {
					slide.Warning = "Resized circular focus must remain centered; static slide"
				}
			}
		})
	}
}

func motionPPTXMultiGeometry(body []byte, slide *motionSlide, axes []int, phases map[int]int) ([]byte, error) {
	var focus *motionEffect
	for _, axis := range axes {
		e := slide.Effects[axis]
		if e.Kind != "gradient" {
			continue
		}
		if e.Property == "focus-x" || e.Property == "focus-y" || e.Property == "focus-width" || e.Property == "focus-height" {
			copy := e
			focus = &copy
			continue
		}
		var err error
		body, err = motionPPTXGradientBody(body, e, phases[axis])
		if err != nil {
			return nil, err
		}
	}
	if focus != nil {
		state := motionCombinedFocus(slide, focus.Target, phases, *focus)
		for i, property := range []string{"focus-x", "focus-y"} {
			value := (state.focusRect[i] + 1 - state.focusRect[i+2]) / 2
			state.Property, state.From, state.To = property, []float64{value}, []float64{value}
			var err error
			body, err = motionPPTXGradientBody(body, state, motionSamples(state))
			if err != nil {
				return nil, err
			}
		}
	}
	return body, nil
}

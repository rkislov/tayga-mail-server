package httpapi

import (
	"fmt"
	"math"
)

func motionReadRepeat(node *compareNode, e *motionEffect, odp bool) error {
	wrapper := node.name.Local == "cTn" && node.attr("nodeType") != "" || odp && (node.name.Local == "par" || node.name.Local == "seq")
	if wrapper && node.attr("dur") != "" {
		duration, err := motionMS(node.attr("dur"), odp)
		if err != nil {
			e.wrapperTimingIssue = true
		} else {
			e.wrapperDurations = append(e.wrapperDurations, duration)
		}
	}
	if !wrapper && odp && node.attr("begin") != "" && node.attr("begin") != "0s" && node.attr("begin") != "0ms" && node.attr("begin") != "0" {
		e.wrapperTimingIssue = true
	}
	if !odp && node.name.Local == "cTn" && node.attr("nodeType") == "" {
		for _, cond := range node.child("stCondLst").all("cond") {
			if cond.attr("delay") != "" && cond.attr("delay") != "0" || cond.attr("evt") != "" {
				e.wrapperTimingIssue = true
			}
		}
	}

	if raw := node.attr("repeatCount"); raw != "" {
		v, err := motionNumber(raw, 1, 20000)
		if err != nil {
			return fmt.Errorf("finite repeat count required")
		}
		if !odp {
			v /= 1000
		}
		if v < 1 || v > 20 || math.Trunc(v) != v {
			return fmt.Errorf("repeat count must be an integer from 1 to 20")
		}
		if v != 1 && e.Repeat > 1 {
			return fmt.Errorf("nested repetition unsupported")
		}
		if v != 1 {
			e.Repeat = int(v)
		}
	}
	key := "autoRev"
	if odp {
		key = "autoReverse"
	}
	if raw := node.attr(key); raw != "" {
		if raw != "true" && raw != "1" && raw != "false" && raw != "0" {
			return fmt.Errorf("invalid reverse timing")
		}
		if raw == "true" || raw == "1" {
			if e.AutoReverse {
				return fmt.Errorf("nested reversal unsupported")
			}
			e.AutoReverse = true
		}
	}
	if node.attr("repeatDur") != "" {
		return fmt.Errorf("repeat duration clipping unsupported")
	}
	for _, key := range []string{"additive", "accumulate"} {
		if raw := node.attr(key); raw == "sum" || raw == "always" {
			return fmt.Errorf("additive accumulation unsupported")
		}
	}
	return nil
}

func motionActiveDuration(e motionEffect) int {
	count := e.Repeat
	if count == 0 {
		count = 1
	}
	duration := e.Duration
	if duration == 0 {
		duration = 1
	}
	if e.AutoReverse {
		count *= 2
	}
	return duration * count
}

func motionRepeatValid(e motionEffect) error {
	if e.Repeat > 1 || e.AutoReverse {
		if err := motionWrapperValid(e); err != nil {
			return err
		}
	}
	if (e.Repeat > 1 || e.AutoReverse) && (e.Duration == 0 || e.Kind == "cut") {
		return fmt.Errorf("repetition requires a timed effect")
	}
	if motionActiveDuration(e) > 600000 {
		return fmt.Errorf("repeated effect exceeds ten minutes")
	}
	return nil
}

func motionWrapperValid(e motionEffect) error {
	if e.wrapperTimingIssue {
		return fmt.Errorf("nested or indefinite stage timing unsupported")
	}
	for _, duration := range e.wrapperDurations {
		if duration != e.Duration {
			return fmt.Errorf("container clipping of stage duration unsupported")
		}
	}
	return nil
}

func motionPropertyKey(e motionEffect) string {
	switch e.Kind {
	case "color", "gradient":
		return e.Target + "/" + e.Kind + "/" + e.Property
	case "fade", "cut":
		return e.Target + "/opacity"
	default:
		return e.Target + "/" + e.Kind
	}
}

func motionMarkStages(slide *motionSlide) {
	if slide.Warning != "" {
		return
	}
	counts := map[string]int{}
	for _, e := range slide.Effects {
		counts[motionPropertyKey(e)]++
	}
	previous := map[string]int{}
	for i := range slide.Effects {
		e := &slide.Effects[i]
		key := motionPropertyKey(*e)
		if counts[key] > 1 {
			if err := motionWrapperValid(*e); err != nil {
				slide.Warning = err.Error() + "; static slide"
				return
			}
		}
		if prior, ok := previous[key]; ok {
			value := prior
			e.Previous = &value
		}
		previous[key] = i
	}
}

func motionFinalPhase(e motionEffect) float64 {
	if e.AutoReverse {
		return 0
	}
	return 1
}

func motionStageSource(slide *motionSlide, e motionEffect, source []float64) []float64 {
	if e.Previous == nil {
		return source
	}
	previous := slide.Effects[*e.Previous]
	return motionGradientPhase(previous, motionFinalPhase(previous))
}

func motionStageColors(slide *motionSlide, e motionEffect, source [][]float64) [][]float64 {
	if e.Previous == nil {
		return source
	}
	previous := slide.Effects[*e.Previous]
	colors := make([][]float64, len(previous.sourceColors))
	for i, c := range previous.sourceColors {
		colors[i] = motionPaintTween(c, previous, motionFinalPhase(previous))
	}
	return colors
}

func motionSnapshotSelected(slide *motionSlide, index int, phases map[int]int) bool {
	e := slide.Effects[index]
	if _, ok := phases[index]; ok {
		return true
	}
	if e.Previous != nil {
		return false
	}
	for other := range phases {
		if motionPropertyKey(slide.Effects[other]) == motionPropertyKey(e) {
			return false
		}
	}
	return true
}

func motionPrepareTransformStages(slide *motionSlide) {
	if slide.Warning != "" {
		return
	}
	for i := range slide.Effects {
		e := &slide.Effects[i]
		if e.Kind == "path" {
			if e.Previous != nil {
				slide.Warning = "Multiple path stages require an explicit coordinate model; static slide"
			}
			continue
		}
		if e.Kind != "rotate" && e.Kind != "scale" {
			continue
		}
		source := []float64{0}
		if e.Kind == "scale" {
			source = []float64{1, 1}
		}
		source = motionStageSource(slide, *e, source)
		if e.fromExplicit {
			for j, v := range source {
				if math.Abs(e.From[j]-v) > 1e-6 {
					slide.Warning = "Stage origin does not match previous final value; static slide"
				}
			}
		}
		e.From = append([]float64{}, source...)
		if e.Relative {
			for j, v := range source {
				switch e.DeltaSpace {
				case "rotate", "scale-offset":
					e.To[j] = v + e.Delta[j]
				case "scale-factor":
					e.To[j] = v * e.Delta[j]
				}
			}
		}
		for _, v := range e.To {
			if e.Kind == "rotate" && math.Abs(v) > 3600 || e.Kind == "scale" && (v < .01 || v > 8) {
				slide.Warning = "Staged transform exceeds range; static slide"
			}
		}
	}
}

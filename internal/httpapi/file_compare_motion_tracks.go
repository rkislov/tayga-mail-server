package httpapi

import "fmt"

type motionTrackPoint struct {
	Effect, Phase int
	At            float64
}

// Stage intervals are separated by gaps. Playback jumps across a gap, never
// interpolates through it: an autoreversed stage can end at its initial color
// while the next stage has a distinct atlas interval with the same visual start.
func motionTrackGroups(slide *motionSlide) [][][]int {
	tracks := map[string][]int{}
	order := []string{}
	for i, e := range slide.Effects {
		if e.Kind != "color" && e.Kind != "gradient" {
			continue
		}
		key := motionPropertyKey(e)
		if len(tracks[key]) == 0 {
			order = append(order, key)
		}
		tracks[key] = append(tracks[key], i)
	}
	groups := [][][]int{}
	consumed := map[string]bool{}
	for _, key := range order {
		track := tracks[key]
		e := slide.Effects[track[0]]
		if consumed[key] {
			continue
		}
		if e.Kind == "color" && e.Property != "fill" {
			if len(track) > 1 {
				groups = append(groups, [][]int{track})
			}
			continue
		}
		bundle := [][]int{}
		repeated := false
		for _, other := range order {
			t := tracks[other]
			candidate := slide.Effects[t[0]]
			if candidate.Target == e.Target && (candidate.Kind == "gradient" || candidate.Kind == "color" && candidate.Property == "fill") {
				bundle = append(bundle, t)
				repeated = repeated || len(t) > 1
			}
		}
		if repeated {
			groups = append(groups, bundle)
			for _, t := range bundle {
				consumed[motionPropertyKey(slide.Effects[t[0]])] = true
			}
		}
	}
	return groups
}

func motionTrackPoints(slide *motionSlide, tracks [][]int) ([][]motionTrackPoint, error) {
	if len(tracks) > 4 {
		return nil, fmt.Errorf("stage atlas supports at most four paint properties")
	}
	counts := make([][]int, len(tracks))
	total := 1
	geometry := 0
	for i, track := range tracks {
		if slide.Effects[track[0]].Kind == "gradient" {
			geometry++
		}
		sum := 0
		for _, index := range track {
			e := slide.Effects[index]
			count := 2
			if !e.Discrete && (e.Kind == "color" && motionSamples(e) == 9 || e.Property == "angle" && absMotionSweep(e) >= 180) {
				count = 9
			}
			counts[i] = append(counts[i], count)
			sum += count
		}
		total *= sum
	}
	if geometry > 3 || total > 45 {
		return nil, fmt.Errorf("stage atlas exceeds three geometry properties or 45 paint states")
	}
	for i, track := range tracks {
		for stage, index := range track {
			if counts[i][stage] != 2 || motionSamples(slide.Effects[index]) != 9 {
				continue
			}
			sum := 0
			for _, count := range counts[i] {
				sum += count
			}
			if total/sum*(sum+1) <= 45 {
				total = total / sum * (sum + 1)
				counts[i][stage] = 3
			}
		}
	}
	points := make([][]motionTrackPoint, len(tracks))
	for i, track := range tracks {
		for stage, index := range track {
			count := counts[i][stage]
			for j := 0; j < count; j++ {
				phase := 1 + j*(motionSamples(slide.Effects[index])-1)/(count-1)
				p := float64(phase-1) / float64(motionSamples(slide.Effects[index])-1)
				points[i] = append(points[i], motionTrackPoint{Effect: index, Phase: phase, At: (2*float64(stage) + p) / float64(2*len(track)-1)})
			}
		}
	}
	return points, nil
}

func absMotionSweep(e motionEffect) float64 {
	v := e.To[0] - e.From[0]
	if v < 0 {
		v = -v
	}
	return v
}

func motionTrackAxes(plan *motionLayerPlan) []int {
	if len(plan.paintPoints) == 0 {
		return plan.paintAxes
	}
	axes := make([]int, len(plan.paintAxes))
	for i := range axes {
		axes[i] = plan.paintPoints[i][plan.paintPhases[i]-1].Effect
	}
	return axes
}

func motionTrackPositions(plan motionLayerPlan, slide *motionSlide) ([]float64, bool, bool) {
	positions := make([]float64, len(plan.paintAxes))
	first, last := true, true
	for i, axis := range plan.paintAxes {
		if len(plan.paintPoints) > 0 {
			positions[i] = plan.paintPoints[i][plan.paintPhases[i]-1].At
			first = first && plan.paintPhases[i] == 1
			last = last && plan.paintPhases[i] == len(plan.paintPoints[i])
		} else {
			positions[i] = float64(plan.paintPhases[i]-1) / float64(motionSamples(slide.Effects[axis])-1)
			first = first && positions[i] == 0
			last = last && positions[i] == 1
		}
	}
	return positions, first, last
}

func motionValidateTrackGrids(slide *motionSlide) {
	if slide.Warning != "" {
		return
	}
	for _, tracks := range motionTrackGroups(slide) {
		points, err := motionTrackPoints(slide, tracks)
		if err != nil {
			slide.Warning = err.Error() + "; static slide"
			return
		}
		var focus *motionEffect
		hasSize := false
		for _, track := range tracks {
			for _, index := range track {
				e := &slide.Effects[index]
				if motionNeedsFocusRaster(*e) {
					hasSize = true
				}
				if e.Property == "focus-x" || e.Property == "focus-y" || e.Property == "focus-width" || e.Property == "focus-height" {
					focus = e
				}
			}
		}
		if focus == nil {
			continue
		}
		indices := make([][]int, len(points))
		for i, p := range points {
			for j := range p {
				indices[i] = append(indices[i], j)
			}
		}
		motionWalkPhases(indices, func(values []int) {
			phases := map[int]int{}
			for i, j := range values {
				point := points[i][j]
				phases[point.Effect] = point.Phase
			}
			state := motionCombinedFocus(slide, focus.Target, phases, *focus)
			for _, v := range state.focusRect {
				if v < -1e-9 || v > 1+1e-9 {
					slide.Warning = "Staged focus exceeds shared fill bounds; static slide"
				}
			}
			if hasSize && focus.focusPath == "circle" && (absMotionNumber(state.focusRect[0]-state.focusRect[2]) > 1e-5 || absMotionNumber(state.focusRect[1]-state.focusRect[3]) > 1e-5) {
				slide.Warning = "Staged circular focus must remain centered; static slide"
			}
		})
	}
}

func absMotionNumber(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

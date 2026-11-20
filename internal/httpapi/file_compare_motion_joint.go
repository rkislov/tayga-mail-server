package httpapi

// Joint paints form a bounded tensor of independently timed color and geometry
// phases. Adding independent raster deltas would lose their interaction.
func motionJointPaints(slide *motionSlide) map[int]int {
	pairs := map[int]int{}
	for i, e := range slide.Effects {
		if e.Kind != "color" || e.Property != "fill" {
			continue
		}
		for j, g := range slide.Effects {
			if g.Kind == "gradient" && g.Target == e.Target {
				pairs[i] = j
			}
		}
	}
	return pairs
}

func motionJointGeometryPhases(color, geometry motionEffect) []int {
	count := motionSamples(geometry)
	step := 1
	// 9 color phases × 5 geometry phases × 2 mattes = 90 pages.
	// Keep the existing 120-page limit, including visibility and object layers.
	if motionSamples(color) > 2 && count == 9 {
		step = 2
	}
	phases := []int{}
	for phase := 1; phase <= count; phase += step {
		phases = append(phases, phase)
	}
	return phases
}

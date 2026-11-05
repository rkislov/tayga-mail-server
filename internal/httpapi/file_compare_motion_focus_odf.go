package httpapi

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"math"
)

const motionODPFocusNamespace = "urn:tayga:gradient:1.0"

func motionODPSetupFocus(e *motionEffect, gradient *compareNode) error {
	rect, path, err := motionODPFocusRect(gradient)
	if err != nil {
		return err
	}
	e.focusRect, e.focusPath = rect, path
	e.focusStops = []float64{0, 1}
	if len(gradient.children) > 0 {
		e.focusStops = nil
		for _, stop := range gradient.children {
			v, err := motionNumber(stop.attr("offset"), 0, 1)
			if err != nil {
				return err
			}
			e.focusStops = append(e.focusStops, v)
		}
	}
	if !motionNeedsFocusRaster(*e) {
		return nil
	}
	switch gradient.attr("style") {
	case "radial", "ellipsoid", "square", "rectangular":
	default:
		return fmt.Errorf("focal extent requires radial or rectangular gradient")
	}
	angle, err := motionODPGradientValue(gradient, "angle")
	if err != nil || angle != 0 && angle != 360 {
		return fmt.Errorf("rotated resized ODP focus unsupported")
	}
	for _, key := range []string{"start-intensity", "end-intensity"} {
		if value := gradient.attr(key); value != "" && value != "100%" {
			return fmt.Errorf("resized focus requires full color intensity")
		}
	}
	if value := gradient.attr("border"); value != "" && value != "0%" {
		return fmt.Errorf("resized focus with border unsupported")
	}
	if path == "circle" && (math.Abs(rect[0]-rect[2]) > 0.00001 || math.Abs(rect[1]-rect[3]) > 0.00001) {
		return fmt.Errorf("resized circular focus must be centered")
	}
	return nil
}

func motionODPFocusRect(gradient *compareNode) ([4]float64, string, error) {
	values := make([]float64, 4)
	for i, key := range []string{"focus-x", "focus-y", "focus-width", "focus-height"} {
		v, err := motionODPGradientValue(gradient, key)
		if err != nil {
			return [4]float64{}, "", err
		}
		values[i] = v
	}
	rect := [4]float64{values[0] - values[2]/2, values[1] - values[3]/2, 1 - values[0] - values[2]/2, 1 - values[1] - values[3]/2}
	for _, v := range rect {
		if v < -1e-9 || v > 1+1e-9 {
			return rect, "", fmt.Errorf("ODP focal rectangle escapes fill")
		}
	}
	path := "rect"
	if gradient.attr("style") == "radial" || gradient.attr("style") == "ellipsoid" {
		path = "circle"
	}
	return rect, path, nil
}

func motionODPFocusImage(e motionEffect, name string) (string, error) {
	// ODF's radial/rectangular gradients progress from the outer start color
	// toward the inner end color; the focus rasterizer uses inner-to-outer.
	stops := make([]float64, len(e.focusStops))
	palette := make([][]float64, len(e.sourceColors))
	if len(stops) < 2 || len(stops) != len(palette) {
		return "", fmt.Errorf("focus palette missing")
	}
	for i := range stops {
		stops[i] = 1 - e.focusStops[len(stops)-1-i]
		palette[i] = e.sourceColors[len(palette)-1-i]
	}
	pixels := motionFocusPixels(e.focusRect, e.focusPath, stops, palette)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, pixels); err != nil {
		return "", err
	}
	// ODF allows inline binary data in fill-image; no external links, package
	// resources or manifest entries are necessary for these bounded PNGs.
	return `<draw:fill-image xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" draw:name="` + name + `"><office:binary-data xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0">` + base64.StdEncoding.EncodeToString(encoded.Bytes()) + `</office:binary-data></draw:fill-image>`, nil
}

func motionODPBitmapStyle(style []byte, family, name string) ([]byte, error) {
	if family != "graphic" && family != "paragraph" {
		return style, nil
	}
	props, err := motionSpans(style, "style", "graphic-properties")
	if err != nil {
		return nil, err
	}
	if len(props) == 0 {
		if family != "graphic" {
			return style, nil
		}
		style, err = motionAppendRoot(style, "style", `<style:graphic-properties xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"/>`)
		if err != nil {
			return nil, err
		}
		props, err = motionSpans(style, "style", "graphic-properties")
		if err != nil {
			return nil, err
		}
	}
	if len(props) != 1 {
		return nil, fmt.Errorf("ambiguous focal fill properties")
	}
	value := style[props[0].start:props[0].end]
	for _, a := range [][2]string{{"fill", "bitmap"}, {"fill-image-name", name}} {
		value = motionRootAttr(value, a[0], a[1], "draw:"+a[0], "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0")
	}
	value = motionRootAttr(value, "repeat", "stretch", "style:repeat", "urn:oasis:names:tc:opendocument:xmlns:style:1.0")
	return motionEdit(style, props, [][]byte{value}), nil
}

// Prepare one target while preserving all other objects in ordinary states.
func motionODPScopedPaint(pkg *comparePackage, body []byte, e motionEffect, phase int, name string, raster *motionEffect, geometries []motionEffect) ([]byte, motionColorAssets, error) {
	spans, err := motionSpans(body, "page", "rect", "ellipse", "custom-shape", "circle", "polygon", "path")
	if err != nil {
		return nil, motionColorAssets{}, err
	}
	for _, span := range spans {
		matched := false
		for _, a := range span.attrs {
			if a.Name.Local == "id" && a.Value == e.Target {
				matched = true
			}
		}
		if !matched {
			continue
		}
		removals := []motionSpan{}
		for _, other := range spans {
			if other.start != span.start {
				removals = append(removals, other)
			}
		}
		isolated := motionEdit(body, removals, nil)
		modified, assets, err := motionODPPaintBody(pkg, isolated, e, phase, name, nil, 0, raster)
		if err != nil {
			return nil, assets, err
		}
		if raster == nil && len(geometries) > 0 {
			for _, g := range geometries {
				data, err := motionODPGradientGeometry([]byte(assets.Gradients), g, motionSamples(g))
				if err != nil {
					return nil, assets, err
				}
				assets.Gradients = string(data)
			}
		}
		nodes, err := motionSpans(modified, "page", "rect", "ellipse", "custom-shape", "circle", "polygon", "path")
		if err != nil || len(nodes) != 1 {
			return nil, assets, fmt.Errorf("scoped gradient target missing")
		}
		return motionEdit(body, []motionSpan{span}, [][]byte{modified[nodes[0].start:nodes[0].end]}), assets, nil
	}
	return body, motionColorAssets{}, nil
}

func motionODPFramePaint(pkg *comparePackage, body []byte, slide *motionSlide, plan *motionLayerPlan, frame int) ([]byte, motionColorAssets, error) {
	assets := motionColorAssets{}
	phases := motionPlanPhases(plan)
	handled := map[string]bool{}
	add := func(extra motionColorAssets) error {
		assets.Styles += extra.Styles
		assets.Gradients += extra.Gradients
		if len(assets.Styles)+len(assets.Gradients) > 8<<20 {
			return fmt.Errorf("frame paint assets exceed 8 MiB")
		}
		return nil
	}
	for i, e := range slide.Effects {
		if !motionNeedsFocusRaster(e) || handled[e.Target] {
			continue
		}
		handled[e.Target] = true
		raster := motionCombinedFocus(slide, e.Target, phases, e)
		for j, color := range slide.Effects {
			if color.Kind == "color" && color.Property == "fill" && color.Target == e.Target && motionSnapshotSelected(slide, j, phases) {
				phase := phases[j]
				if phase == 0 {
					phase = 1
				}
				t := float64(phase-1) / float64(motionSamples(color)-1)
				raster.sourceColors = nil
				for _, source := range color.sourceColors {
					raster.sourceColors = append(raster.sourceColors, motionPaintTween(source, color, t))
				}
			}
		}
		paint, phase := e, 1
		if plan != nil && len(plan.paintTracks) > 0 {
			for _, axis := range motionTrackAxes(plan) {
				candidate := slide.Effects[axis]
				if candidate.Kind == "color" && candidate.Property != "fill" && candidate.Target == e.Target {
					paint, phase = candidate, phases[axis]
				}
			}
		}
		if plan != nil && plan.colorPhase > 0 {
			candidate := slide.Effects[plan.colorEffect]
			if candidate.Target == e.Target && candidate.Kind == "color" && candidate.Property != "fill" {
				paint, phase = candidate, plan.colorPhase
			}
		}
		modified, extra, err := motionODPScopedPaint(pkg, body, paint, phase, fmt.Sprintf("_TaygaColor%dFocus%d", frame, i), &raster, nil)
		if err != nil {
			return nil, assets, err
		}
		body = modified
		if err = add(extra); err != nil {
			return nil, assets, err
		}
	}
	if plan == nil {
		return body, assets, nil
	}
	if len(plan.paintAxes) > 0 {
		target := slide.Effects[plan.paintAxes[0]].Target
		if handled[target] {
			return body, assets, nil
		}
		axes := motionTrackAxes(plan)
		if len(axes) == 1 && slide.Effects[axes[0]].Kind == "color" {
			e := slide.Effects[axes[0]]
			modified, extra, err := motionODPScopedPaint(pkg, body, e, phases[axes[0]], fmt.Sprintf("_TaygaColor%dTrack", frame), nil, nil)
			if err != nil {
				return nil, assets, err
			}
			body = modified
			if err = add(extra); err != nil {
				return nil, assets, err
			}
			return body, assets, nil
		}
		geometries := []motionEffect{}
		var carrier motionEffect
		for _, axis := range motionTrackAxes(plan) {
			e := slide.Effects[axis]
			if e.Kind != "gradient" {
				continue
			}
			values := motionEffectPhase(e, phases[axis])
			e.From, e.To = values, values
			e.Relative = false
			geometries = append(geometries, e)
			carrier = e
		}
		for _, axis := range motionTrackAxes(plan) {
			color := slide.Effects[axis]
			if color.Kind != "color" {
				continue
			}
			t := float64(phases[axis]-1) / float64(motionSamples(color)-1)
			carrier.sourceColors = nil
			for _, source := range color.sourceColors {
				carrier.sourceColors = append(carrier.sourceColors, motionPaintTween(source, color, t))
			}
		}
		modified, extra, err := motionODPScopedPaint(pkg, body, carrier, motionSamples(carrier), fmt.Sprintf("_TaygaColor%dMulti", frame), nil, geometries)
		if err != nil {
			return nil, assets, err
		}
		body = modified
		if err = add(extra); err != nil {
			return nil, assets, err
		}
		return body, assets, nil
	}
	if plan.colorPhase > 0 && !handled[slide.Effects[plan.colorEffect].Target] {
		var geometry *motionEffect
		if plan.geometryPhase > 0 {
			geometry = &slide.Effects[plan.geometryEffect]
		}
		modified, extra, err := motionODPPaintBody(pkg, body, slide.Effects[plan.colorEffect], plan.colorPhase, fmt.Sprintf("_TaygaColor%d", frame), geometry, plan.geometryPhase)
		if err != nil {
			return nil, assets, err
		}
		body = modified
		if err = add(extra); err != nil {
			return nil, assets, err
		}
	}
	return body, assets, nil
}

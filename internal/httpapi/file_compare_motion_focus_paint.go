package httpapi

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
)

type motionFocusAsset struct {
	Name, ID string
	PNG      []byte
}

// LibreOffice's DrawingML importer keeps only fillToRect's center, discarding
// its extent. Materialize only this fill as a bounded PNG. Shape clipping,
// contour, text, rotation and mattes still belong to the office renderer.
func motionPPTXFocusPaint(body []byte, e motionEffect, phase int, paint *motionEffect, paintPhase int, id string) ([]byte, []byte, error) {
	shapes, err := motionSpans(body, "spTree", "sp")
	if err != nil {
		return nil, nil, err
	}
	for _, span := range shapes {
		shape := body[span.start:span.end]
		props, err := motionSpans(shape, "nvSpPr", "cNvPr")
		if err != nil {
			return nil, nil, err
		}
		matched := false
		for _, p := range props {
			for _, a := range p.attrs {
				if a.Name.Local == "id" && a.Value == e.Target {
					matched = true
				}
			}
		}
		if !matched {
			continue
		}
		fills, err := motionSpans(shape, "spPr", "gradFill")
		if err != nil || len(fills) != 1 {
			return nil, nil, fmt.Errorf("focus requires explicit gradient fill")
		}
		palette := e.sourceColors
		if paint != nil {
			palette = nil
			t := float64(paintPhase-1) / float64(motionSamples(*paint)-1)
			for _, source := range e.sourceColors {
				palette = append(palette, motionPaintTween(source, *paint, t))
			}
		}
		rect := e.focusRect
		index := 0
		if e.Property == "focus-height" {
			index = 1
		}
		center := (rect[index] + 1 - rect[index+2]) / 2
		t := float64(phase-1) / float64(motionSamples(e)-1)
		half := (e.From[0] + (e.To[0]-e.From[0])*t) / 2
		rect[index], rect[index+2] = center-half, 1-center-half
		pixels := motionFocusPixels(rect, e.focusPath, e.focusStops, palette)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, pixels); err != nil {
			return nil, nil, err
		}
		fill := []byte(`<a:blipFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" rotWithShape="1"><a:blip r:embed="` + id + `"/><a:stretch><a:fillRect/></a:stretch></a:blipFill>`)
		shape = motionEdit(shape, fills, [][]byte{fill})
		return motionEdit(body, []motionSpan{span}, [][]byte{shape}), encoded.Bytes(), nil
	}
	return body, nil, nil // Hidden target: no image or relationship is needed.
}

func motionFocusPixels(rect [4]float64, path string, stops []float64, palette [][]float64) *image.NRGBA {
	const size = 256
	pixels := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px, py := (float64(x)+.5)/size, (float64(y)+.5)/size
			at := motionFocusPosition(rect, path, px, py)
			i := 0
			for i < len(stops)-1 && at >= stops[i+1] {
				i++
			}
			c := palette[i]
			if i < len(stops)-1 && at > stops[i] {
				p := math.Max(0, math.Min(1, (at-stops[i])/(stops[i+1]-stops[i])))
				c = motionColorTween(palette[i], palette[i+1], p, "rgb", "")
			}
			pixels.SetNRGBA(x, y, color.NRGBA{R: uint8(math.Round(c[0])), G: uint8(math.Round(c[1])), B: uint8(math.Round(c[2])), A: 255})
		}
	}
	return pixels
}

func motionFocusPosition(rect [4]float64, path string, x, y float64) float64 {
	if path == "rect" {
		t := 0.0
		if x < rect[0] {
			t = math.Max(t, (rect[0]-x)/rect[0])
		}
		if x > 1-rect[2] {
			t = math.Max(t, (x-1+rect[2])/rect[2])
		}
		if y < rect[1] {
			t = math.Max(t, (rect[1]-y)/rect[1])
		}
		if y > 1-rect[3] {
			t = math.Max(t, (y-1+rect[3])/rect[3])
		}
		return math.Max(0, math.Min(1, t))
	}
	// Centered circle path: interpolate nested ellipses between the focal
	// ellipse and the bounding ellipse. Outside the latter use the last stop.
	rx, ry := (1-rect[0]-rect[2])/2, (1-rect[1]-rect[3])/2
	inside := func(t float64) bool {
		a, b := rx+(.5-rx)*t, ry+(.5-ry)*t
		if a <= 0 || b <= 0 {
			return false
		}
		return math.Pow((x-.5)/a, 2)+math.Pow((y-.5)/b, 2) <= 1
	}
	if inside(0) {
		return 0
	}
	if !inside(1) {
		return 1
	}
	lo, hi := 0.0, 1.0
	for i := 0; i < 18; i++ {
		mid := (lo + hi) / 2
		if inside(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

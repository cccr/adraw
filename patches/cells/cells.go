// Package cells holds the curated cell-based patches.
//
// Every one of them runs the same pipeline — divide the border into convex
// cells, draw something in each — and differs only in which tessellator and
// fill it fixes. They exist as separate registered patches because the patch
// list is how you browse the tool: a style you cannot see in the list is a
// style you will not find.
//
// `tangle` remains the fully parameterized version with tess and style exposed
// as sliders. These are its presets, not replacements.
package cells

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/geom"
	"github.com/cccr/adraw/pkg/patch"
)

// Default page, in millimetres: A4 landscape.
const (
	defaultWidth  = 297
	defaultHeight = 210
)

// preset is one cell patch with its tessellator and fill fixed.
type preset struct {
	name string
	desc string
	tess geom.Tessellator
	fill geom.Fill
	ps   *patch.ParamSet
}

func (p *preset) Name() string              { return p.name }
func (p *preset) Description() string       { return p.desc }
func (p *preset) Params() []patch.ParamSpec { return p.ps.Specs() }

func (p *preset) get(v patch.Values, key string) float64 {
	s, ok := p.ps.Find(key)
	if !ok {
		return 0 // this preset does not expose the control
	}
	return v.Get(s)
}

// has reports whether this preset exposes a given control.
func (p *preset) has(key string) bool {
	_, ok := p.ps.Find(key)
	return ok
}

func (p *preset) setup(req patch.Request) (geom.Polygon, geom.CellOptions) {
	border := geom.Polygon(req.Border)
	if len(border) < 3 {
		w, h := req.PageOr(defaultWidth, defaultHeight)
		border = geom.RectPolygon(w, h, p.get(req.Values, "margin"))
	}
	return border, geom.CellOptions{
		Tessellator: p.tess,
		CellSize:    p.get(req.Values, "cell"),
		MergeProb:   p.get(req.Values, "merge"),
		MaxVerts:    5,
	}
}

func (p *preset) Generate(ctx context.Context, req patch.Request) (drawing.Frame, error) {
	border, cellOpt := p.setup(req)
	rng := rand.New(rand.NewSource(req.Seed))

	cells, err := geom.Cells(border, cellOpt, rng)
	if err != nil {
		return drawing.Frame{}, fmt.Errorf("%s: %w", p.name, err)
	}

	spacing := p.get(req.Values, "spacing")
	fillP := p.get(req.Values, "fill")
	jitter := p.get(req.Values, "jitter")

	// Quantising the hatch angle makes neighbouring cells rhyme rather than
	// each picking an unrelated direction.
	angles := math.Max(1, math.Round(p.get(req.Values, "angles")))

	var paths []drawing.Path
	for _, cell := range cells {
		if err := ctx.Err(); err != nil {
			return drawing.Frame{}, err
		}
		// Draw the die for every cell whatever the outcome, so the random
		// sequence stays stable as the fill probability is tuned.
		draw := rng.Float64() < fillP
		clockwise := rng.Intn(2) == 0
		angle := float64(rng.Intn(int(angles))) / angles * math.Pi
		vertex := rng.Intn(len(cell))
		if !draw {
			continue
		}
		paths = append(paths, geom.ApplyFill(cell, geom.FillOptions{
			Fill:      p.fill,
			Spacing:   spacing,
			Angle:     angle,
			Cross:     math.Pi / 3,
			Clockwise: clockwise,
			Vertex:    vertex,
		}, rng)...)
	}

	if jitter > 0 {
		paths = geom.Jitter(paths, jitter, rng.Float64)
	}

	meta := map[string]string{
		"patch": p.name,
		"seed":  fmt.Sprint(req.Seed),
	}
	for _, s := range p.ps.Specs() {
		meta[s.Key] = fmt.Sprintf("%g", req.Values.Get(s))
	}

	return drawing.Frame{
		Bounds: pageBounds(req, paths),
		Paths:  paths,
		Meta:   meta,
	}, nil
}

// Regions exposes the cells this patch fills, so they can seed another patch.
func (p *preset) Regions(ctx context.Context, req patch.Request) ([]drawing.Vec2Ring, error) {
	border, cellOpt := p.setup(req)
	cells, err := geom.Cells(border, cellOpt, rand.New(rand.NewSource(req.Seed)))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.name, err)
	}
	out := make([]drawing.Vec2Ring, len(cells))
	for i, c := range cells {
		out[i] = drawing.Vec2Ring(c)
	}
	return out, nil
}

// pageBounds returns the bounds of the page rather than of the ink, so the
// margin shows as whitespace instead of being cropped away.
func pageBounds(req patch.Request, paths []drawing.Path) drawing.Rect {
	w, h := req.PageOr(defaultWidth, defaultHeight)
	b := drawing.Rect{Max: drawing.Vec2{X: w, Y: h}}
	if len(req.Border) >= 3 {
		b = drawing.BoundsOf(req.Border)
	}
	for _, p := range paths {
		b = b.Union(drawing.BoundsOf(p.Pts))
	}
	return b
}

// Package topo draws contour lines of a noise field, like a topographic map.
//
// Structurally unlike the cell patches: there is no tessellation and no
// per-cell fill. A scalar elevation field is sampled on a grid and its level
// sets are traced, which yields a handful of long flowing curves rather than
// thousands of short strokes.
package topo

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/geom"
	"github.com/cccr/adraw/pkg/patch"
)

const (
	defaultWidth  = 297
	defaultHeight = 210
)

var params = patch.NewParamSet().
	MM("scale", "Terrain scale",
		"Size of the land features. Larger means broader hills and fewer of them.",
		20, 400, 190).
	Count("levels", "Contour count",
		"How many elevation lines to draw. More means a denser, steeper-reading map.",
		3, 80, 20).
	Count("octaves", "Detail",
		"Layers of noise. 1 is smooth hills; higher adds crinkled small-scale relief.",
		1, 8, 3).
	Unit("ridge", "Ridged",
		"Sharpens rolling hills into crested ridges, like eroded mountains.", 0, 1, 0).
	Unit("warp", "Warp",
		"Distorts the field by another noise field, for swirling, less regular terrain.",
		0, 1, 0.25).
	MM("detail", "Trace precision",
		"Sampling step. Smaller traces smoother curves but costs more time.",
		0.4, 3, 0.9).
	MM("margin", "Page margin",
		"Blank border left around the drawing.", 0, 40, 10)

type Topo struct{}

func New() *Topo { return &Topo{} }

func init() { patch.Register(New()) }

func (t *Topo) Name() string { return "topo" }

func (t *Topo) Description() string {
	return "Contour lines of a noise landscape, drawn like the elevation lines of a topographic map."
}

func (t *Topo) Params() []patch.ParamSpec { return params.Specs() }

func spec(key string) patch.ParamSpec {
	s, ok := params.Find(key)
	if !ok {
		panic("topo: unknown parameter " + key)
	}
	return s
}

func (t *Topo) Generate(ctx context.Context, req patch.Request) (drawing.Frame, error) {
	v := req.Values
	var (
		scale   = v.Get(spec("scale"))
		levels  = int(math.Round(v.Get(spec("levels"))))
		octaves = int(math.Round(v.Get(spec("octaves"))))
		ridge   = v.Get(spec("ridge"))
		warp    = v.Get(spec("warp"))
		detail  = v.Get(spec("detail"))
		margin  = v.Get(spec("margin"))
	)

	border := geom.Polygon(req.Border)
	if len(border) < 3 {
		w, h := req.PageOr(defaultWidth, defaultHeight)
		border = geom.RectPolygon(w, h, margin)
	}

	// Two independent fields: one is the elevation, the other displaces the
	// sampling position. Warping is what stops fBm's isolines reading as
	// obviously procedural blobs — it shears them into the folded, flowing
	// shapes real terrain has.
	rng := rand.New(rand.NewSource(req.Seed))
	elevation := geom.NewNoise(rng.Int63())
	distort := geom.NewNoise(rng.Int63())

	fbm := geom.FBMOptions{Octaves: octaves, Ridge: ridge}
	field := func(x, y float64) float64 {
		u, w := x/scale, y/scale
		if warp > 0 {
			u += warp * distort.At(u+11.3, w+5.7)
			w += warp * distort.At(u-7.1, w-3.9)
		}
		return elevation.FBM(u, w, fbm)
	}

	box := geom.BBox(border)
	paths := geom.Contour(field, geom.ContourOptions{
		Bounds: box,
		Step:   detail,
		Levels: geom.LevelsBetween(-1, 1, levels),
	})

	// The field is sampled over the bounding box, so trim to the border itself.
	// Contours are open curves, not areas, so they are clipped by walking each
	// one and splitting wherever it leaves the shape.
	paths = clipPaths(paths, border)

	return drawing.Frame{
		Bounds: pageBounds(req),
		Paths:  paths,
		Meta: map[string]string{
			"patch":   t.Name(),
			"seed":    fmt.Sprint(req.Seed),
			"scale":   fmt.Sprintf("%g", scale),
			"levels":  fmt.Sprint(levels),
			"octaves": fmt.Sprint(octaves),
			"ridge":   fmt.Sprintf("%g", ridge),
			"warp":    fmt.Sprintf("%g", warp),
			"detail":  fmt.Sprintf("%g", detail),
			"margin":  fmt.Sprintf("%g", margin),
		},
	}, nil
}

// clipPaths keeps the portions of each path that lie inside the border,
// splitting a path into several wherever it leaves and re-enters.
func clipPaths(paths []drawing.Path, border geom.Polygon) []drawing.Path {
	out := make([]drawing.Path, 0, len(paths))
	for _, p := range paths {
		pts := p.Pts
		if p.Closed && len(pts) > 0 {
			pts = append(append([]geom.Vec2(nil), pts...), pts[0])
		}

		var run []geom.Vec2
		flush := func() {
			if len(run) >= 2 {
				out = append(out, drawing.Path{Pts: run})
			}
			run = nil
		}
		for _, pt := range pts {
			if geom.Contains(border, pt) {
				run = append(run, pt)
			} else {
				flush()
			}
		}
		// A closed path wholly inside stays closed; anything trimmed is open.
		if p.Closed && len(run) == len(pts) && len(run) >= 3 {
			out = append(out, drawing.Path{Pts: run[:len(run)-1], Closed: true})
			run = nil
		}
		flush()
	}
	return out
}

func pageBounds(req patch.Request) drawing.Rect {
	if len(req.Border) >= 3 {
		return drawing.BoundsOf(req.Border)
	}
	w, h := req.PageOr(defaultWidth, defaultHeight)
	return drawing.Rect{Max: drawing.Vec2{X: w, Y: h}}
}

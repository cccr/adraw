// Package flow draws streamlines through a noise-driven vector field.
//
// Like topo it abandons tessellation entirely, but where topo traces the level
// sets of a scalar field, flow traces its flow direction — the two are
// perpendicular views of the same kind of landscape.
package flow

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
	MM("scale", "Field scale",
		"Size of the swirls. Larger means broader, calmer currents.", 20, 400, 150).
	MM("separation", "Line spacing",
		"Minimum distance between neighbouring lines. Set it from your pen width.",
		0.8, 8, 2).
	MM("length", "Line length",
		"How far a single line may run before stopping.", 10, 600, 250).
	Count("octaves", "Detail",
		"Layers of noise. 1 is smooth laminar flow; higher adds turbulence.", 1, 6, 2).
	Unit("curl", "Curl",
		"Rotates the field toward its perpendicular, turning outward flow into "+
			"circulating eddies.", 0, 1, 0.5).
	MM("margin", "Page margin",
		"Blank border left around the drawing.", 0, 40, 10)

type Flow struct{}

func New() *Flow { return &Flow{} }

func init() { patch.Register(New()) }

func (f *Flow) Name() string { return "flow" }

func (f *Flow) Description() string {
	return "Long curving lines following a noise flow field, like iron filings or contour currents."
}

func (f *Flow) Params() []patch.ParamSpec { return params.Specs() }

func spec(key string) patch.ParamSpec {
	s, ok := params.Find(key)
	if !ok {
		panic("flow: unknown parameter " + key)
	}
	return s
}

func (f *Flow) Generate(ctx context.Context, req patch.Request) (drawing.Frame, error) {
	v := req.Values
	var (
		scale   = v.Get(spec("scale"))
		sep     = v.Get(spec("separation"))
		length  = v.Get(spec("length"))
		octaves = int(math.Round(v.Get(spec("octaves"))))
		curl    = v.Get(spec("curl"))
		margin  = v.Get(spec("margin"))
	)

	border := geom.Polygon(req.Border)
	if len(border) < 3 {
		w, h := req.PageOr(defaultWidth, defaultHeight)
		border = geom.RectPolygon(w, h, margin)
	}

	rng := rand.New(rand.NewSource(req.Seed))
	n := geom.NewNoise(rng.Int63())
	fbm := geom.FBMOptions{Octaves: octaves}

	// The noise gives an angle rather than a vector, so the field has uniform
	// magnitude everywhere and lines advance at a steady rate. Curl then rotates
	// that angle toward the perpendicular, which turns flow that mostly runs
	// off the page into flow that circulates within it.
	field := func(x, y float64) geom.Vec2 {
		a := n.FBM(x/scale, y/scale, fbm) * math.Pi * 2
		a += curl * math.Pi / 2
		return geom.Vec2{X: math.Cos(a), Y: math.Sin(a)}
	}

	// Blue-noise seeds spaced at the line separation: tried in order, each is
	// skipped if an earlier line already runs nearby, which gives an even
	// covering without the clumping random seeding would produce.
	seeds := geom.PoissonDisk(border, geom.SampleOptions{
		MinDist:      sep,
		SkipBoundary: true,
	}, rng)

	paths := geom.Streamlines(field, seeds, geom.StreamOptions{
		Border:     border,
		Step:       math.Max(sep/2, 0.4),
		MaxLength:  length,
		Separation: sep,
	})

	if err := ctx.Err(); err != nil {
		return drawing.Frame{}, err
	}

	pw, ph := req.PageOr(defaultWidth, defaultHeight)
	bounds := drawing.Rect{Max: drawing.Vec2{X: pw, Y: ph}}
	if len(req.Border) >= 3 {
		bounds = drawing.BoundsOf(req.Border)
	}
	return drawing.Frame{
		Bounds: bounds,
		Paths:  paths,
		Meta: map[string]string{
			"patch":      f.Name(),
			"seed":       fmt.Sprint(req.Seed),
			"scale":      fmt.Sprintf("%g", scale),
			"separation": fmt.Sprintf("%g", sep),
			"length":     fmt.Sprintf("%g", length),
			"octaves":    fmt.Sprint(octaves),
			"curl":       fmt.Sprintf("%g", curl),
			"margin":     fmt.Sprintf("%g", margin),
		},
	}, nil
}

// Package tangle draws nested rotating polygons across a tessellated area.
//
// The area is divided into convex cells by blue-noise sampling plus Delaunay
// triangulation, then each cell is filled with rings that shrink and twist into
// a vortex. It is the pen-and-paper "spiral triangles" pattern.
package tangle

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

var params = patch.NewParamSet().
	MM("cell", "Cell size",
		"Typical width of one cell. Smaller means more, finer cells.", 5, 60, 26).
	MM("spacing", "Ring gap",
		"Distance between nested rings. Set it from your pen width.", 0.3, 5, 1.2).
	Unit("merge", "Merge probability",
		"Chance of fusing neighbouring triangles into quads and pentagons. 0 leaves pure triangles.", 0, 1, 0.25).
	Unit("fill", "Fill probability",
		"Chance a cell is drawn at all. Below 1 leaves bare patches; 1 fills everything.", 0, 1, 1).
	MM("jitter", "Hand wobble",
		"Random displacement of every point, for a less mechanical line.", 0, 2, 0).
	MM("margin", "Page margin",
		"Blank border left around the drawing.", 0, 40, 10).
	Count("tess", "Cell shape",
		"0 angular Delaunay triangles, 1 organic Voronoi cells.",
		0, float64(geom.TessellatorCount-1), 0).
	Count("style", "Fill style",
		"0 spiral, 1 contour, 2 fan, 3 hatch, 4 crosshatch, 5 dots.",
		0, float64(geom.FillCount-1), 0)

// Tangle is the patch. It holds no state; all configuration arrives in the
// Request, which is what keeps it deterministic and drivable from any UI.
// spec looks up a declared parameter. A missing key is a programming error in
// this file, not a runtime condition.
func spec(key string) patch.ParamSpec {
	s, ok := params.Find(key)
	if !ok {
		panic("tangle: unknown parameter " + key)
	}
	return s
}

type Tangle struct{}

func New() *Tangle { return &Tangle{} }

func init() { patch.Register(New()) }

func (t *Tangle) Name() string { return "tangle" }

func (t *Tangle) Description() string {
	return "Divides the page into convex cells and fills each with nested rings " +
		"that shrink and rotate into a vortex."
}

func (t *Tangle) Params() []patch.ParamSpec { return params.Specs() }

func (t *Tangle) Generate(ctx context.Context, req patch.Request) (drawing.Frame, error) {
	v := req.Values
	var (
		cellSize = v.Get(spec("cell"))
		spacing  = v.Get(spec("spacing"))
		mergeP   = v.Get(spec("merge"))
		fillP    = v.Get(spec("fill"))
		jitter   = v.Get(spec("jitter"))
		margin   = v.Get(spec("margin"))
		style    = geom.Fill(int(v.Get(spec("style"))))
	)

	border, cellOpt := t.setup(req)
	rng := rand.New(rand.NewSource(req.Seed))

	cells, err := geom.Cells(border, cellOpt, rng)
	if err != nil {
		return drawing.Frame{}, fmt.Errorf("tangle: %w", err)
	}

	var paths []drawing.Path
	for _, cell := range cells {
		if err := ctx.Err(); err != nil {
			return drawing.Frame{}, err
		}
		// Every cell is filled by default. Lowering fill leaves some empty,
		// which can read as hand-drawn, but a skipped cell is conspicuous —
		// especially a large merged one — so it is opt-in. The die is drawn for
		// every cell regardless of the outcome, so the random sequence stays
		// stable as fill is tuned.
		draw := rng.Float64() < fillP
		// Varying twist direction, hatch angle and fan apex per cell is what
		// stops a field of identical cells looking machine-stamped.
		clockwise := rng.Intn(2) == 0
		angle := rng.Float64() * math.Pi
		vertex := rng.Intn(len(cell))
		if !draw {
			continue
		}
		paths = append(paths, geom.ApplyFill(cell, geom.FillOptions{
			Fill:      style,
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

	return drawing.Frame{
		Bounds: pageBounds(req, paths),
		Paths:  paths,
		Meta: map[string]string{
			"patch":   t.Name(),
			"seed":    fmt.Sprint(req.Seed),
			"cell":    fmt.Sprintf("%g", cellSize),
			"spacing": fmt.Sprintf("%g", spacing),
			"merge":   fmt.Sprintf("%g", mergeP),
			"fill":    fmt.Sprintf("%g", fillP),
			"jitter":  fmt.Sprintf("%g", jitter),
			"margin":  fmt.Sprintf("%g", margin),
			"tess":    fmt.Sprintf("%g", v.Get(spec("tess"))),
			"style":   fmt.Sprintf("%g", v.Get(spec("style"))),
		},
	}, nil
}

// pageBounds returns the bounds of the page, not of the ink.
//
// For the default page that is the full sheet, so the margin shows up as
// whitespace on every side instead of being cropped away. For a caller-supplied
// border it is that border's extent. Either way the paths are unioned in last,
// so jitter can never push ink outside the declared bounds.
func pageBounds(req patch.Request, paths []drawing.Path) drawing.Rect {
	var b drawing.Rect
	if len(req.Border) >= 3 {
		b = drawing.BoundsOf(req.Border)
	} else {
		w, h := req.PageOr(defaultWidth, defaultHeight)
		b = drawing.Rect{Max: drawing.Vec2{X: w, Y: h}}
	}
	for _, p := range paths {
		b = b.Union(drawing.BoundsOf(p.Pts))
	}
	return b
}

// Regions exposes the cells this patch fills, so they can seed another patch.
func (t *Tangle) Regions(ctx context.Context, req patch.Request) ([]drawing.Vec2Ring, error) {
	border, opt := t.setup(req)
	cells, err := geom.Cells(border, opt, rand.New(rand.NewSource(req.Seed)))
	if err != nil {
		return nil, fmt.Errorf("tangle: %w", err)
	}
	out := make([]drawing.Vec2Ring, len(cells))
	for i, c := range cells {
		out[i] = drawing.Vec2Ring(c)
	}
	return out, nil
}

// setup resolves the border and cell options from a request. Generate and
// Regions share it so the two can never disagree about what the cells are.
func (t *Tangle) setup(req patch.Request) (geom.Polygon, geom.CellOptions) {
	border := geom.Polygon(req.Border)
	if len(border) < 3 {
		w, h := req.PageOr(defaultWidth, defaultHeight)
		border = geom.RectPolygon(w, h, req.Values.Get(spec("margin")))
	}
	return border, geom.CellOptions{
		Tessellator: geom.Tessellator(int(req.Values.Get(spec("tess")))),
		CellSize:    req.Values.Get(spec("cell")),
		MergeProb:   req.Values.Get(spec("merge")),
		MaxVerts:    5,
	}
}

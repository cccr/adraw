package geom

import (
	"math"

	"github.com/cccr/adraw/pkg/drawing"
)

// VectorField gives a direction at every point. Streamlines follow it.
type VectorField func(x, y float64) Vec2

// StreamOptions configures streamline tracing.
type StreamOptions struct {
	// Border confines the lines; tracing stops on leaving it.
	Border Polygon

	// Step is the integration step length, in world units.
	Step float64

	// MaxLength caps how far one line may run.
	MaxLength float64

	// Separation is the minimum distance between lines. It is what keeps a
	// flow field legible: without it every line converges onto the same
	// attractors and the drawing turns to mush.
	Separation float64
}

// Streamlines traces a set of curves that follow a vector field.
//
// Seeds are tried in the order given and rejected if they fall too close to an
// existing line, so passing blue-noise seed points gives an even covering.
// Each accepted seed is traced both forward and backward, so a line is not
// arbitrarily cut short at the point it happened to start from.
func Streamlines(f VectorField, seeds []Vec2, opt StreamOptions) []drawing.Path {
	if opt.Step <= 0 || opt.Separation <= 0 || len(opt.Border) < 3 {
		return nil
	}
	maxLen := opt.MaxLength
	if maxLen <= 0 {
		maxLen = 1e9
	}
	maxSteps := int(maxLen / opt.Step)
	if maxSteps < 2 {
		maxSteps = 2
	}

	// Every point of every accepted line goes into the grid, so the separation
	// test is a constant-time neighbourhood query rather than a scan.
	occupied := NewGrid(BBox(opt.Border), math.Max(opt.Separation, 1e-6))

	var out []drawing.Path
	for _, seed := range seeds {
		if !Contains(opt.Border, seed) || occupied.AnyWithin(seed, opt.Separation) {
			continue
		}

		fwd := trace(f, seed, 1, opt, maxSteps, occupied)
		bwd := trace(f, seed, -1, opt, maxSteps, occupied)

		// Reverse the backward half and join, so the seed sits in the middle of
		// one continuous curve.
		pts := make([]Vec2, 0, len(fwd)+len(bwd))
		for i := len(bwd) - 1; i > 0; i-- {
			pts = append(pts, bwd[i])
		}
		pts = append(pts, fwd...)

		if len(pts) < 2 {
			continue
		}
		for _, p := range pts {
			occupied.Add(p)
		}
		out = append(out, drawing.Path{Pts: pts})
	}
	return out
}

// trace integrates from a point until it leaves the border, runs too long, or
// approaches an existing line.
//
// Midpoint (RK2) rather than Euler: with a plain Euler step a line curving
// steadily drifts outward and spirals where it should close, which is visible
// as a systematic widening of every curve.
func trace(f VectorField, start Vec2, dir int, opt StreamOptions, maxSteps int, occupied *Grid) []Vec2 {
	pts := []Vec2{start}
	p := start

	for i := 0; i < maxSteps; i++ {
		v := Norm(f(p.X, p.Y))
		if v == (Vec2{}) {
			break // a stationary point; there is no direction to follow
		}
		mid := Add(p, Scale(v, float64(dir)*opt.Step/2))
		vm := Norm(f(mid.X, mid.Y))
		if vm == (Vec2{}) {
			break
		}
		next := Add(p, Scale(vm, float64(dir)*opt.Step))

		if !Contains(opt.Border, next) {
			break
		}
		// Stop short of other lines. No exemption is needed for this line's own
		// points: they are not added to the grid until the trace finishes, so
		// the grid only ever holds other lines.
		if occupied.AnyWithin(next, opt.Separation) {
			break
		}
		pts = append(pts, next)
		p = next
	}
	return pts
}

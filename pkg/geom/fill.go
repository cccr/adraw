package geom

import (
	"math"
	"math/rand"

	"github.com/cccr/adraw/pkg/drawing"
)

// SpiralOptions configures the nested-rotating fill.
type SpiralOptions struct {
	// Spacing is the target perpendicular gap between consecutive rings, in
	// millimetres. Set it from the pen width.
	Spacing float64

	// Clockwise selects the twist direction. Randomizing this per cell is what
	// keeps a field of spirals from looking mechanical.
	Clockwise bool

	// MinArea stops the spiral once a ring gets this small. Defaults to a
	// fraction of Spacing squared.
	MinArea float64

	// MaxRings bounds the output regardless of geometry. Defaults to 400.
	MaxRings int
}

// Spiral fills a convex polygon with nested rings that shrink *and rotate*,
// producing the vortex that gives this pattern its character.
//
// Each ring is inscribed in the previous one by walking a fraction t along
// every edge, always in the same rotational direction. That offset walk is what
// distinguishes this from a plain inward inset: an inset produces flat nested
// outlines, whereas advancing along the edges twists each ring relative to its
// parent, and the accumulated twist sweeps the interior into a fan converging
// on an off-centre point.
//
// t is solved for per ring rather than fixed. The naive choice, t = Spacing/h
// with h = 2*Area/Perimeter, is wrong by a wide margin: because each ring also
// rotates, the perpendicular gap it opens is considerably larger than t*h, so
// the rings converge far too fast and leave sweeping voids. Instead t is found
// by bisection on the ring's own inradius, which is exact for triangles, quads
// and pentagons alike and keeps Spacing a genuine physical quantity tied to the
// pen. Once a ring is too small for a full gap the bisection saturates at tMax
// and the remaining rings converge geometrically, which is the dense core the
// pattern wants.
//
// The polygon must be convex; Tessellate guarantees that for its output.
func Spiral(p Polygon, opt SpiralOptions) []drawing.Path {
	const (
		tMin = 0.01 // below this the spiral barely advances and wastes rings
		tMax = 0.45 // above this a ring can overshoot its parent's centre
	)
	if len(p) < 3 || opt.Spacing <= 0 {
		return nil
	}
	maxRings := opt.MaxRings
	if maxRings <= 0 {
		maxRings = 400
	}
	minArea := opt.MinArea
	if minArea <= 0 {
		minArea = opt.Spacing * opt.Spacing
	}

	dir := 1
	if opt.Clockwise {
		dir = -1
	}

	ring := make(Polygon, len(p))
	copy(ring, p)

	paths := make([]drawing.Path, 0, 16)
	for k := 0; k < maxRings; k++ {
		if Area(ring) < minArea {
			break
		}
		paths = append(paths, ToPath(ring))

		t := solveT(ring, opt.Spacing, tMin, tMax)
		next := inscribe(ring, t, dir)

		// Guard against a degenerate cell where the inscribed ring fails to
		// shrink; without this the loop would spin to MaxRings emitting
		// near-identical paths.
		if Area(next) >= Area(ring) {
			break
		}
		ring = next
	}
	return paths
}

// inscribe returns the ring formed by walking fraction t along every edge, in
// the rotational direction dir. This single step is both the shrink and the
// twist.
func inscribe(ring Polygon, t float64, dir int) Polygon {
	n := len(ring)
	out := make(Polygon, n)
	for i := range ring {
		j := ((i+dir)%n + n) % n
		out[i] = Lerp(ring[i], ring[j], t)
	}
	return out
}

// solveT finds the edge fraction whose inscribed ring sits gap millimetres
// inside ring, measured as the drop in inradius.
//
// The inradius of the inscribed ring falls monotonically as t rises over
// [0, 0.5], so plain bisection converges. Doing this numerically rather than in
// closed form is what lets one Spacing parameter behave identically across
// triangles, quads and pentagons — a closed form would need a separate
// derivation per vertex count.
func solveT(ring Polygon, gap, tMin, tMax float64) float64 {
	target := InRadius(ring) - gap
	if target <= 0 {
		return tMax // the ring is thinner than one gap; converge and finish
	}
	// Direction is irrelevant here: the two twist directions are reflections of
	// each other and share an inradius.
	if InRadius(inscribe(ring, tMax, 1)) > target {
		return tMax // even the tightest step cannot open a full gap
	}
	if InRadius(inscribe(ring, tMin, 1)) < target {
		return tMin // even the gentlest step already overshoots
	}
	lo, hi := tMin, tMax
	for i := 0; i < 24; i++ {
		mid := (lo + hi) / 2
		if InRadius(inscribe(ring, mid, 1)) > target {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// Jitter displaces every point of every path by a random offset up to amount
// millimetres, giving a mechanically exact drawing a little hand-drawn wobble.
// It takes a function rather than an *rand.Rand so callers keep control of the
// seeded sequence.
func Jitter(paths []drawing.Path, amount float64, rnd func() float64) []drawing.Path {
	if amount <= 0 {
		return paths
	}
	for _, path := range paths {
		for i, pt := range path.Pts {
			ang := rnd() * 2 * math.Pi
			mag := rnd() * amount
			path.Pts[i] = Vec2{pt.X + math.Cos(ang)*mag, pt.Y + math.Sin(ang)*mag}
		}
	}
	return paths
}

// Hatch fills a convex polygon with parallel lines spaced spacing millimetres
// apart, running at angle radians from the x axis.
//
// It is the plain counterpart to Spiral: no twist, no nesting, just ruled
// lines. Patches combine the two, or pick between them per cell.
func Hatch(p Polygon, spacing, angle float64) []drawing.Path {
	if len(p) < 3 || spacing <= 0 {
		return nil
	}
	dir := Vec2{math.Cos(angle), math.Sin(angle)}
	normal := Perp(dir)

	// Project every vertex onto the normal to find the band of offsets the
	// polygon spans, then walk that band in steps of spacing.
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range p {
		d := Dot(v, normal)
		lo, hi = math.Min(lo, d), math.Max(hi, d)
	}

	c := Centroid(p)
	span := Len(Sub(BBox(p).Max, BBox(p).Min))

	var out []drawing.Path
	// Start at the first multiple of spacing inside the band so the hatch
	// pattern is stable under translation rather than tied to the polygon.
	for d := math.Ceil(lo/spacing) * spacing; d <= hi; d += spacing {
		// A point on this line: shift the centroid onto the right offset.
		base := Add(c, Scale(normal, d-Dot(c, normal)))
		a, b, ok := clipLineToConvex(p, Sub(base, Scale(dir, span)), Add(base, Scale(dir, span)))
		if ok {
			out = append(out, drawing.Path{Pts: []Vec2{a, b}})
		}
	}
	return out
}

// clipLineToConvex trims the segment a-b to the part inside the convex polygon
// p, returning false if it misses entirely.
//
// For a convex polygon the line can cross the boundary at most twice, so
// collecting every edge intersection and keeping the extremes is both correct
// and simpler than a general clipper.
func clipLineToConvex(p Polygon, a, b Vec2) (Vec2, Vec2, bool) {
	d := Sub(b, a)
	tLo, tHi := math.Inf(1), math.Inf(-1)

	for i := range p {
		e0, e1 := p[i], p[(i+1)%len(p)]
		edge := Sub(e1, e0)
		den := Cross(d, edge)
		if math.Abs(den) < 1e-12 {
			continue // parallel to this edge
		}
		w := Sub(e0, a)
		t := Cross(w, edge) / den // along a-b
		u := Cross(w, d) / den    // along the edge
		if u < -1e-9 || u > 1+1e-9 || t < -1e-9 || t > 1+1e-9 {
			continue
		}
		tLo, tHi = math.Min(tLo, t), math.Max(tHi, t)
	}

	if tHi-tLo < 1e-9 {
		return Vec2{}, Vec2{}, false
	}
	return Add(a, Scale(d, tLo)), Add(a, Scale(d, tHi)), true
}

// Inset fills a convex polygon with concentric rings that shrink without
// rotating — the topographic-contour counterpart to Spiral.
//
// This is genuinely different geometry, not Spiral with the twist removed:
// Spiral walks a fraction along each edge, whereas Inset offsets every edge
// inward along its own normal and re-intersects the neighbours. That keeps the
// gap between rings exactly Spacing everywhere, including at sharp corners
// where an edge-walk would bunch up.
func Inset(p Polygon, opt SpiralOptions) []drawing.Path {
	if len(p) < 3 || opt.Spacing <= 0 {
		return nil
	}
	maxRings := opt.MaxRings
	if maxRings <= 0 {
		maxRings = 400
	}
	minArea := opt.MinArea
	if minArea <= 0 {
		minArea = opt.Spacing * opt.Spacing
	}

	ring := append(Polygon(nil), p...)
	paths := make([]drawing.Path, 0, 16)

	for k := 0; k < maxRings; k++ {
		if Area(ring) < minArea {
			break
		}
		paths = append(paths, ToPath(ring))

		// A convex polygon can only be inset by less than its inradius. Past
		// that the offset edges cross over each other and the "polygon" becomes
		// a self-intersecting bowtie — whose shoelace area is small and
		// meaningless, so an area comparison alone would happily march on
		// emitting garbage that sprawls outside the cell.
		if InRadius(ring) <= opt.Spacing {
			break
		}

		next := insetRing(ring, opt.Spacing)
		if len(next) < 3 || !IsConvex(next) || Area(next) >= Area(ring) {
			break
		}
		ring = next
	}
	return paths
}

// insetRing offsets every edge of a convex polygon inward by d and returns the
// polygon formed by the offset edges' intersections.
func insetRing(p Polygon, d float64) Polygon {
	n := len(p)
	// Interior lies to the left of each directed edge for counter-clockwise
	// winding, and to the right for clockwise.
	side := 1.0
	if SignedArea(p) < 0 {
		side = -1
	}

	type line struct{ a, b Vec2 }
	offset := make([]line, n)
	for i := range p {
		a, b := p[i], p[(i+1)%n]
		normal := Scale(Norm(Perp(Sub(b, a))), side*d)
		offset[i] = line{Add(a, normal), Add(b, normal)}
	}

	out := make(Polygon, 0, n)
	for i := range p {
		prev := offset[(i-1+n)%n]
		cur := offset[i]
		x, ok := lineIntersect(prev.a, prev.b, cur.a, cur.b)
		if !ok {
			return nil // adjacent edges are parallel; the ring has collapsed
		}
		out = append(out, x)
	}
	return out
}

// Fan fills a convex polygon with rays from one vertex to points spaced along
// the far side of its perimeter.
func Fan(p Polygon, spacing float64, vertex int) []drawing.Path {
	if len(p) < 3 || spacing <= 0 {
		return nil
	}
	n := len(p)
	apex := p[((vertex%n)+n)%n]
	start := ((vertex%n)+n)%n + 1

	var out []drawing.Path
	// Walk the edges that do not touch the apex; the two adjacent edges are
	// skipped because a ray along them would just retrace the outline.
	for e := 0; e < n-2; e++ {
		i := (start + e) % n
		a, b := p[i], p[(i+1)%n]
		steps := int(math.Max(1, math.Floor(Dist(a, b)/spacing)))
		for s := 0; s <= steps; s++ {
			t := float64(s) / float64(steps)
			if e > 0 && s == 0 {
				continue // this point ended the previous edge
			}
			out = append(out, drawing.Path{Pts: []Vec2{apex, Lerp(a, b, t)}})
		}
	}
	return out
}

// CrossHatch fills a convex polygon with two sets of ruled lines crossing at
// the given angle to each other.
func CrossHatch(p Polygon, spacing, angle, cross float64) []drawing.Path {
	out := Hatch(p, spacing, angle)
	return append(out, Hatch(p, spacing, angle+cross)...)
}

// Dots stipples a convex polygon with short marks at blue-noise positions.
//
// A plotter cannot draw a zero-length move, so each dot is a very short stroke.
// Reusing PoissonDisk keeps the stipple evenly spread rather than clumping the
// way uniform random placement would.
func Dots(p Polygon, spacing float64, rng *rand.Rand) []drawing.Path {
	if len(p) < 3 || spacing <= 0 {
		return nil
	}
	dot := math.Min(spacing/4, 0.3)

	// Sample inside a shrunken copy, not the cell itself. Each mark is centred
	// on its sample point and has real extent, so sampling right up to the edge
	// would leave half of every boundary dot sticking out of the cell.
	region := p
	if inner := insetRing(p, dot/2); len(inner) >= 3 && IsConvex(inner) &&
		Area(inner) > 0 && Area(inner) < Area(p) {
		region = inner
	}

	pts := PoissonDisk(region, SampleOptions{MinDist: spacing, SkipBoundary: true}, rng)

	out := make([]drawing.Path, 0, len(pts))
	for _, pt := range pts {
		out = append(out, drawing.Path{
			Pts: []Vec2{{X: pt.X - dot/2, Y: pt.Y}, {X: pt.X + dot/2, Y: pt.Y}},
		})
	}
	return out
}

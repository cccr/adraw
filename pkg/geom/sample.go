package geom

import (
	"math"
	"math/rand"
)

// bridsonAttempts is the number of candidate points tried around an active
// point before it is retired. 30 is Bridson's recommendation.
const bridsonAttempts = 30

// SampleOptions configures blue-noise sampling.
type SampleOptions struct {
	// MinDist is the minimum distance between any two points, in millimetres.
	// It is the dominant control over cell size in the resulting tessellation.
	MinDist float64

	// BoundarySpacing is the spacing of the points seeded along the border. If
	// zero it defaults to MinDist. See PoissonDisk for why these matter.
	BoundarySpacing float64

	// SkipBoundary suppresses the seeded border points entirely.
	//
	// Delaunay wants them: they make the convex hull equal the border, so the
	// triangulation tiles it exactly. Voronoi wants the opposite — a seeded
	// border point owns a thin sliver cell hugging the edge, so a Voronoi
	// tessellation samples the interior only and clips its cells to the border
	// afterwards instead.
	SkipBoundary bool
}

// PoissonDisk returns points spread through border with a guaranteed minimum
// separation, using Bridson's algorithm over a background grid of cell size
// MinDist/sqrt(2), which makes the neighbour check O(1) per candidate.
//
// Before sampling the interior it seeds points along the border's edges and
// marks them as already placed. This is what makes the result usable for
// tessellation, for two reasons:
//
//   - The convex hull of the point set becomes the border itself, so a Delaunay
//     triangulation tiles the border exactly with no clipping step at all.
//   - No interior point can land just barely inside an edge, which is the
//     classic source of sliver triangles along the boundary.
//
// The returned points are deterministic for a given rng sequence. Boundary
// points come first, in border order.
func PoissonDisk(border Polygon, opt SampleOptions, rng *rand.Rand) []Vec2 {
	r := opt.MinDist
	if r <= 0 || len(border) < 3 {
		return nil
	}
	spacing := opt.BoundarySpacing
	if spacing <= 0 {
		spacing = r
	}

	// Cell size r/sqrt(2) guarantees at most one point per cell, which is what
	// makes the neighbour check a fixed-size window rather than a scan.
	box := BBox(border)
	grid := NewGrid(box, r/math.Sqrt2)

	var pts []Vec2
	place := func(p Vec2) {
		grid.Add(p)
		pts = append(pts, p)
	}

	// Seed the border. Each edge contributes its start vertex plus evenly
	// spaced points along it; the end vertex belongs to the next edge, so it is
	// not emitted twice.
	for i := 0; i < len(border) && !opt.SkipBoundary; i++ {
		a, b := border[i], border[(i+1)%len(border)]
		// Floor, not round: rounding to nearest can land on a step count that
		// divides the edge into gaps *below* the minimum distance, which would
		// reintroduce the slivers this seeding exists to prevent. Flooring can
		// only ever overshoot the spacing, never undershoot it.
		steps := int(math.Max(1, math.Floor(Dist(a, b)/spacing)))
		for s := 0; s < steps; s++ {
			place(Lerp(a, b, float64(s)/float64(steps)))
		}
	}

	// With a seeded border every boundary point is active, so the interior
	// fills inward from the edges. Without one there is nothing to grow from,
	// so start at the centroid.
	if len(pts) == 0 {
		c := Centroid(border)
		if !Contains(border, c) {
			return nil // a border whose centroid falls outside it is not convex
		}
		place(c)
	}
	active := make([]int, len(pts))
	for i := range active {
		active[i] = i
	}

	for len(active) > 0 {
		ai := rng.Intn(len(active))
		origin := pts[active[ai]]
		placed := false

		for t := 0; t < bridsonAttempts; t++ {
			// Candidate in the annulus [r, 2r) around the active point.
			ang := rng.Float64() * 2 * math.Pi
			rad := r * (1 + rng.Float64())
			c := Vec2{origin.X + math.Cos(ang)*rad, origin.Y + math.Sin(ang)*rad}

			if !box.Contains(c) || !Contains(border, c) || grid.AnyWithin(c, r) {
				continue
			}
			place(c)
			active = append(active, len(pts)-1)
			placed = true
			break
		}

		if !placed {
			active[ai] = active[len(active)-1]
			active = active[:len(active)-1]
		}
	}
	return pts
}

package geom

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/fogleman/delaunay"
)

// TessOptions configures how a point set is turned into cells.
type TessOptions struct {
	// MergeProb is the chance that any given interior edge is dissolved,
	// fusing the two cells either side of it. 0 leaves pure triangles.
	MergeProb float64

	// MaxVerts caps the vertex count of a merged cell, keeping the output to
	// triangles, quads and pentagons rather than growing blobs. Values below 3
	// are treated as 3.
	MaxVerts int
}

// Tessellate triangulates pts and returns the resulting cells, optionally
// fusing neighbours into larger convex polygons.
//
// Delaunay is the right triangulation here for one specific reason: it
// maximizes the minimum angle over all possible triangulations of the point
// set. Fed blue-noise points from PoissonDisk, sliver cells are not merely
// unlikely, they are structurally excluded — the "no small areas" requirement
// is satisfied by construction rather than by filtering afterwards.
//
// Every returned cell is convex, which the spiral fill relies on.
func Tessellate(pts []Vec2, opt TessOptions, rng *rand.Rand) ([]Polygon, error) {
	if len(pts) < 3 {
		return nil, fmt.Errorf("geom: need at least 3 points to tessellate, got %d", len(pts))
	}

	dpts := make([]delaunay.Point, len(pts))
	for i, p := range pts {
		dpts[i] = delaunay.Point{X: p.X, Y: p.Y}
	}
	// Delaunator fails on fully collinear input. With interior points present
	// this cannot happen, but surface it rather than panicking downstream.
	tri, err := delaunay.Triangulate(dpts)
	if err != nil {
		return nil, fmt.Errorf("geom: triangulation failed: %w", err)
	}

	nTri := len(tri.Triangles) / 3
	if nTri == 0 {
		return nil, fmt.Errorf("geom: triangulation produced no triangles")
	}

	// Each triangle starts as its own cell. Merging rewrites cells in place and
	// repoints the owner of every triangle it absorbs.
	cells := make([][]int, nTri)
	members := make([][]int, nTri)
	owner := make([]int, nTri)
	for t := 0; t < nTri; t++ {
		cells[t] = []int{tri.Triangles[3*t], tri.Triangles[3*t+1], tri.Triangles[3*t+2]}
		members[t] = []int{t}
		owner[t] = t
	}

	if opt.MergeProb > 0 {
		maxVerts := opt.MaxVerts
		if maxVerts < 3 {
			maxVerts = 3
		}
		mergeCells(tri, cells, members, owner, pts, opt.MergeProb, maxVerts, rng)
	}

	out := make([]Polygon, 0, len(cells))
	for _, c := range cells {
		if len(c) < 3 {
			continue // absorbed by a merge
		}
		poly := make(Polygon, len(c))
		for i, idx := range c {
			poly[i] = pts[idx]
		}
		out = append(out, poly)
	}
	return out, nil
}

// mergeCells dissolves interior edges at random, fusing the cells either side.
func mergeCells(tri *delaunay.Triangulation, cells, members [][]int, owner []int,
	pts []Vec2, prob float64, maxVerts int, rng *rand.Rand) {

	order := rng.Perm(len(tri.Halfedges))
	for _, e := range order {
		o := tri.Halfedges[e]
		if o < 0 {
			continue // border edge, nothing on the far side
		}
		c1, c2 := owner[e/3], owner[o/3]
		if c1 == c2 {
			continue // already the same cell
		}
		if len(cells[c1])+len(cells[c2])-2 > maxVerts {
			continue
		}
		if rng.Float64() >= prob {
			continue
		}

		a, b := tri.Triangles[e], tri.Triangles[nextHalfedge(e)]
		merged, ok := mergeAlongEdge(cells[c1], cells[c2], a, b)
		if !ok {
			continue
		}

		// The fill assumes convex cells, so a merge that creates a reflex
		// vertex is rejected outright.
		poly := make(Polygon, len(merged))
		for i, idx := range merged {
			poly[i] = pts[idx]
		}
		if !IsConvex(poly) {
			continue
		}

		cells[c1] = merged
		for _, t := range members[c2] {
			owner[t] = c1
		}
		members[c1] = append(members[c1], members[c2]...)
		cells[c2], members[c2] = nil, nil
	}
}

// nextHalfedge returns the following halfedge within the same triangle.
func nextHalfedge(e int) int {
	if e%3 == 2 {
		return e - 2
	}
	return e + 1
}

// mergeAlongEdge fuses rings p and q, which share the edge a->b in p and b->a
// in q, into one ring with that edge removed.
//
// The result starts at b and walks all of p back round to a, then continues
// through q's remaining vertices back to b — so every boundary edge of both
// rings survives, in order, and only the shared edge disappears.
func mergeAlongEdge(p, q []int, a, b int) ([]int, bool) {
	i := findEdge(p, a, b)
	j := findEdge(q, b, a)
	if i < 0 || j < 0 {
		return nil, false
	}
	n, m := len(p), len(q)

	out := make([]int, 0, n+m-2)
	for k := 1; k <= n; k++ { // p[i+1]==b through p[i+n]==a
		out = append(out, p[(i+k)%n])
	}
	for k := 2; k <= m-1; k++ { // q's vertices strictly between a and b
		out = append(out, q[(j+k)%m])
	}
	return out, true
}

// findEdge returns the index i where ring[i]==from and ring[i+1]==to.
func findEdge(ring []int, from, to int) int {
	for i := range ring {
		if ring[i] == from && ring[(i+1)%len(ring)] == to {
			return i
		}
	}
	return -1
}

// MinAngle returns the smallest interior angle of p, in radians. It is the
// standard measure of how sliver-like a cell is.
func MinAngle(p Polygon) float64 {
	if len(p) < 3 {
		return 0
	}
	min := 2 * math.Pi
	for i := range p {
		prev := p[(i-1+len(p))%len(p)]
		next := p[(i+1)%len(p)]
		u, v := Norm(Sub(prev, p[i])), Norm(Sub(next, p[i]))
		ang := math.Acos(clamp(Dot(u, v), -1, 1))
		if ang < min {
			min = ang
		}
	}
	return min
}

// Tessellator selects how a border is divided into cells.
type Tessellator int

const (
	// Delaunay triangulates blue-noise points: angular, mostly triangular cells
	// with a guaranteed minimum angle.
	Delaunay Tessellator = iota
	// Voronoi takes the dual: organic five- to seven-sided cells.
	Voronoi
	numTessellators
)

// TessellatorCount is how many tessellators exist, for patches declaring a
// selector parameter.
const TessellatorCount = int(numTessellators)

// CellOptions configures the full border-to-cells pipeline.
type CellOptions struct {
	// Tessellator picks the subdivision scheme.
	Tessellator Tessellator

	// CellSize is the minimum distance between sample points, and so the
	// dominant control over how big the resulting cells are.
	CellSize float64

	// MergeProb and MaxVerts are passed through to Tessellate.
	MergeProb float64
	MaxVerts  int
}

// Cells divides a convex border into convex cells: blue-noise sampling followed
// by Delaunay triangulation and optional merging.
//
// This is the pipeline every cell-based patch starts from, extracted so the
// patches themselves stay thin and so a patch can expose its regions without
// duplicating the steps that produced them.
func Cells(border Polygon, opt CellOptions, rng *rand.Rand) ([]Polygon, error) {
	if opt.Tessellator == Voronoi {
		// Merging is a Delaunay-only idea: Voronoi cells are already many-sided,
		// and fusing them would routinely produce concave shapes the fills
		// cannot handle.
		return VoronoiIn(border, opt, rng)
	}
	pts := PoissonDisk(border, SampleOptions{MinDist: opt.CellSize}, rng)
	if len(pts) < 3 {
		return nil, fmt.Errorf("geom: cell size %.1f is too large for the border", opt.CellSize)
	}
	maxVerts := opt.MaxVerts
	if maxVerts < 3 {
		maxVerts = 5
	}
	return Tessellate(pts, TessOptions{MergeProb: opt.MergeProb, MaxVerts: maxVerts}, rng)
}

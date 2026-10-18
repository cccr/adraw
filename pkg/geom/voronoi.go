package geom

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/fogleman/delaunay"
)

// VoronoiCells returns the Voronoi cell of every point in pts, clipped to the
// border.
//
// The Voronoi diagram is the dual of the Delaunay triangulation: a cell's
// corners are the circumcenters of the triangles around its site. Since
// Tessellate already computes a triangulation, this reuses the same machinery
// to produce organically shaped five- to seven-sided cells instead of triangles.
//
// Points on the convex hull would otherwise have unbounded cells. Rather than
// special-casing infinite rays, a ring of far-away ghost points is added so that
// every real site ends up interior and therefore bounded; the ghosts are then
// discarded and every cell is clipped to the border.
func VoronoiCells(border Polygon, pts []Vec2) ([]Polygon, error) {
	if len(pts) < 3 || len(border) < 3 {
		return nil, fmt.Errorf("geom: voronoi needs at least 3 points and a border")
	}

	box := BBox(border)
	center := Centroid(border)
	radius := 4 * math.Hypot(box.Width(), box.Height())

	all := make([]delaunay.Point, 0, len(pts)+8)
	for _, p := range pts {
		all = append(all, delaunay.Point{X: p.X, Y: p.Y})
	}
	const ghosts = 8
	for i := 0; i < ghosts; i++ {
		a := float64(i) / ghosts * 2 * math.Pi
		all = append(all, delaunay.Point{
			X: center.X + math.Cos(a)*radius,
			Y: center.Y + math.Sin(a)*radius,
		})
	}

	tri, err := delaunay.Triangulate(all)
	if err != nil {
		return nil, fmt.Errorf("geom: voronoi triangulation failed: %w", err)
	}

	// Which triangles touch each site, in one pass.
	incident := make([][]int, len(pts))
	for t := 0; t < len(tri.Triangles)/3; t++ {
		for k := 0; k < 3; k++ {
			if v := tri.Triangles[3*t+k]; v < len(pts) {
				incident[v] = append(incident[v], t)
			}
		}
	}

	out := make([]Polygon, 0, len(pts))
	for site, tris := range incident {
		if len(tris) < 3 {
			continue
		}
		corners := make(Polygon, 0, len(tris))
		for _, t := range tris {
			c, ok := circumcenter(
				vecOf(all[tri.Triangles[3*t]]),
				vecOf(all[tri.Triangles[3*t+1]]),
				vecOf(all[tri.Triangles[3*t+2]]),
			)
			if ok {
				corners = append(corners, c)
			}
		}
		if len(corners) < 3 {
			continue
		}

		// The circumcenters surround the site, so sorting them by angle about it
		// puts the cell's corners in ring order.
		s := pts[site]
		sort.Slice(corners, func(i, j int) bool {
			return math.Atan2(corners[i].Y-s.Y, corners[i].X-s.X) <
				math.Atan2(corners[j].Y-s.Y, corners[j].X-s.X)
		})

		if cell := ClipToConvex(corners, border); len(cell) >= 3 {
			out = append(out, cell)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("geom: voronoi produced no cells")
	}
	return out, nil
}

func vecOf(p delaunay.Point) Vec2 { return Vec2{p.X, p.Y} }

// circumcenter returns the point equidistant from a, b and c, or false when
// they are collinear.
func circumcenter(a, b, c Vec2) (Vec2, bool) {
	d := 2 * (a.X*(b.Y-c.Y) + b.X*(c.Y-a.Y) + c.X*(a.Y-b.Y))
	if math.Abs(d) < 1e-12 {
		return Vec2{}, false
	}
	aa := a.X*a.X + a.Y*a.Y
	bb := b.X*b.X + b.Y*b.Y
	cc := c.X*c.X + c.Y*c.Y
	return Vec2{
		X: (aa*(b.Y-c.Y) + bb*(c.Y-a.Y) + cc*(a.Y-b.Y)) / d,
		Y: (aa*(c.X-b.X) + bb*(a.X-c.X) + cc*(b.X-a.X)) / d,
	}, true
}

// VoronoiIn samples the border's interior and returns the Voronoi cells of the
// resulting points — the Voronoi counterpart to Cells.
func VoronoiIn(border Polygon, opt CellOptions, rng *rand.Rand) ([]Polygon, error) {
	pts := PoissonDisk(border, SampleOptions{
		MinDist:      opt.CellSize,
		SkipBoundary: true,
	}, rng)
	if len(pts) < 3 {
		return nil, fmt.Errorf("geom: cell size %.1f is too large for the border", opt.CellSize)
	}
	return VoronoiCells(border, pts)
}

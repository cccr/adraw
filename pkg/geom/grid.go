package geom

import (
	"math"

	"github.com/cccr/adraw/pkg/drawing"
)

// Grid is a uniform spatial hash over a bounded region.
//
// It answers "is anything near this point" and "what is the nearest point" in
// roughly constant time, which is what keeps blue-noise sampling, self-avoiding
// growth, and plotter travel ordering from degrading into O(n^2) scans.
//
// Points can be removed, so it also serves as a shrinking pool: travel ordering
// consumes each path as it visits it.
type Grid struct {
	bounds  drawing.Rect
	cell    float64
	cols    int
	rows    int
	buckets [][]int
	pts     []Vec2
	live    []bool
	count   int
}

// NewGrid creates a grid covering bounds with the given cell size. Choose a cell
// size near the typical query radius: much smaller wastes time walking empty
// cells, much larger degrades toward a linear scan.
func NewGrid(bounds drawing.Rect, cell float64) *Grid {
	if cell <= 0 {
		cell = 1
	}
	cols := int(math.Ceil(bounds.Width()/cell)) + 1
	rows := int(math.Ceil(bounds.Height()/cell)) + 1
	return &Grid{
		bounds:  bounds,
		cell:    cell,
		cols:    cols,
		rows:    rows,
		buckets: make([][]int, cols*rows),
	}
}

// Len reports how many points are currently live.
func (g *Grid) Len() int { return g.count }

// At returns the point stored at index i.
func (g *Grid) At(i int) Vec2 { return g.pts[i] }

// Add stores a point and returns its index.
func (g *Grid) Add(p Vec2) int {
	i := len(g.pts)
	g.pts = append(g.pts, p)
	g.live = append(g.live, true)
	g.count++
	if b := g.bucket(p); b >= 0 {
		g.buckets[b] = append(g.buckets[b], i)
	}
	return i
}

// Remove marks a point dead. Its index stays valid for At, so callers can keep
// referring to it, but it will not be returned by any query again.
func (g *Grid) Remove(i int) {
	if i >= 0 && i < len(g.live) && g.live[i] {
		g.live[i] = false
		g.count--
	}
}

// AnyWithin reports whether any live point lies strictly within d of p.
func (g *Grid) AnyWithin(p Vec2, d float64) bool {
	reach := int(math.Ceil(d/g.cell)) + 1
	cx, cy := g.coords(p)
	for y := cy - reach; y <= cy+reach; y++ {
		for x := cx - reach; x <= cx+reach; x++ {
			for _, i := range g.at(x, y) {
				if g.live[i] && Dist(g.pts[i], p) < d {
					return true
				}
			}
		}
	}
	return false
}

// Within appends the indices of all live points within d of p to dst and
// returns it. Passing a reused slice avoids an allocation per call, which
// matters in the inner loop of an iterative patch.
func (g *Grid) Within(p Vec2, d float64, dst []int) []int {
	reach := int(math.Ceil(d/g.cell)) + 1
	cx, cy := g.coords(p)
	for y := cy - reach; y <= cy+reach; y++ {
		for x := cx - reach; x <= cx+reach; x++ {
			for _, i := range g.at(x, y) {
				if g.live[i] && Dist(g.pts[i], p) <= d {
					dst = append(dst, i)
				}
			}
		}
	}
	return dst
}

// Nearest returns the index of the closest live point to p, or false if none
// remain.
//
// It searches outward a ring of cells at a time and, once it has a candidate,
// continues far enough to cover that distance: a point two cells away
// diagonally can be nearer than one in the adjacent cell, so stopping at the
// first hit would silently return the wrong answer.
func (g *Grid) Nearest(p Vec2) (int, bool) {
	if g.count == 0 {
		return 0, false
	}
	cx, cy := g.coords(p)
	best, bestDist := -1, math.Inf(1)
	maxRing := g.cols + g.rows

	for ring := 0; ring <= maxRing; ring++ {
		if best >= 0 && float64(ring-1)*g.cell > bestDist {
			break
		}
		for _, i := range g.ring(cx, cy, ring) {
			if !g.live[i] {
				continue
			}
			if d := Dist(g.pts[i], p); d < bestDist {
				best, bestDist = i, d
			}
		}
	}
	return best, best >= 0
}

// ring collects the indices in the square shell of cells at the given radius.
func (g *Grid) ring(cx, cy, r int) []int {
	if r == 0 {
		return g.at(cx, cy)
	}
	var out []int
	for x := cx - r; x <= cx+r; x++ {
		out = append(out, g.at(x, cy-r)...)
		out = append(out, g.at(x, cy+r)...)
	}
	for y := cy - r + 1; y <= cy+r-1; y++ {
		out = append(out, g.at(cx-r, y)...)
		out = append(out, g.at(cx+r, y)...)
	}
	return out
}

func (g *Grid) at(x, y int) []int {
	if x < 0 || y < 0 || x >= g.cols || y >= g.rows {
		return nil
	}
	return g.buckets[y*g.cols+x]
}

func (g *Grid) coords(p Vec2) (int, int) {
	return int(math.Floor((p.X - g.bounds.Min.X) / g.cell)),
		int(math.Floor((p.Y - g.bounds.Min.Y) / g.cell))
}

func (g *Grid) bucket(p Vec2) int {
	x, y := g.coords(p)
	if x < 0 || y < 0 || x >= g.cols || y >= g.rows {
		return -1
	}
	return y*g.cols + x
}

// Package optimize contains Frame-to-Frame passes that improve a drawing for
// plotting without changing what it looks like.
package optimize

import (
	"fmt"
	"math"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/geom"
)

// Reorder rearranges paths to shorten the distance the pen travels lifted.
//
// A patch emits paths in whatever order it generated them, which is close to
// worst case for a plotter: `tangle` draws every ring of one cell, then jumps
// across the page to the next. The drawn strokes are fixed, but the order they
// are drawn in, and where each one starts, are free.
//
// Two freedoms are exploited beyond plain ordering:
//
//   - An open path can be entered from either end, so it may be reversed.
//   - A closed path can start at any of its vertices, so it may be rotated.
//     This is the dominant win here, because `tangle` output is almost entirely
//     closed rings.
//
// The result contains exactly the same strokes; only their order, direction and
// starting vertex change.
func Reorder(f drawing.Frame) drawing.Frame {
	if len(f.Paths) < 2 {
		return f
	}

	before := TravelDistance(f)

	// Index every path by both endpoints. A grid cell near the typical gap
	// between paths keeps the nearest-endpoint lookup close to constant time.
	cell := math.Max(f.Bounds.Width(), f.Bounds.Height()) / 64
	grid := geom.NewGrid(f.Bounds, cell)

	// Each grid entry maps back to a path and which end it represents.
	type endpoint struct {
		path int
		tail bool // true if this is the path's last point
	}
	ends := make([]endpoint, 0, len(f.Paths)*2)

	for i, p := range f.Paths {
		if len(p.Pts) == 0 {
			continue
		}
		grid.Add(p.Pts[0])
		ends = append(ends, endpoint{i, false})

		if p.Closed {
			// A closed ring can be entered anywhere, so index every vertex.
			// Storing only the two endpoints would throw away the freedom that
			// matters most for ring-heavy output.
			for j := 1; j < len(p.Pts); j++ {
				grid.Add(p.Pts[j])
				ends = append(ends, endpoint{i, false})
			}
		} else if len(p.Pts) > 1 {
			grid.Add(p.Pts[len(p.Pts)-1])
			ends = append(ends, endpoint{i, true})
		}
	}

	// Which grid entries belong to each path, so taking a path retires all of
	// its other entries at once.
	entries := make([][]int, len(f.Paths))
	for idx, e := range ends {
		entries[e.path] = append(entries[e.path], idx)
	}

	out := make([]drawing.Path, 0, len(f.Paths))
	taken := make([]bool, len(f.Paths))
	pen := drawing.Vec2{X: f.Bounds.Min.X, Y: f.Bounds.Min.Y}

	for len(out) < len(f.Paths) {
		idx, ok := grid.Nearest(pen)
		if !ok {
			break
		}
		e := ends[idx]
		if taken[e.path] {
			grid.Remove(idx)
			continue
		}

		path := f.Paths[e.path]
		start := grid.At(idx)

		switch {
		case path.Closed:
			path = rotateToStart(path, start)
		case e.tail:
			path = reverse(path)
		}

		out = append(out, path)
		taken[e.path] = true
		for _, other := range entries[e.path] {
			grid.Remove(other)
		}
		pen = path.Pts[len(path.Pts)-1]
		if path.Closed {
			pen = path.Pts[0] // a closed path returns to where it began
		}
	}

	// Anything with no indexed endpoint (an empty path) is appended unchanged,
	// so this pass can never drop a stroke.
	for i, p := range f.Paths {
		if !taken[i] {
			out = append(out, p)
		}
	}

	f.Paths = out
	after := TravelDistance(f)
	if f.Meta == nil {
		f.Meta = map[string]string{}
	}
	f.Meta["travel_before_mm"] = fmt.Sprintf("%.0f", before)
	f.Meta["travel_after_mm"] = fmt.Sprintf("%.0f", after)
	return f
}

// TravelDistance is the total distance the pen moves lifted: from the origin to
// the first path, then between the end of each path and the start of the next.
func TravelDistance(f drawing.Frame) float64 {
	pen := drawing.Vec2{X: f.Bounds.Min.X, Y: f.Bounds.Min.Y}
	var total float64
	for _, p := range f.Paths {
		if len(p.Pts) == 0 {
			continue
		}
		total += geom.Dist(pen, p.Pts[0])
		if p.Closed {
			pen = p.Pts[0]
		} else {
			pen = p.Pts[len(p.Pts)-1]
		}
	}
	return total
}

// rotateToStart returns a closed path beginning at the vertex nearest to start,
// preserving the ring's order and direction.
func rotateToStart(p drawing.Path, start drawing.Vec2) drawing.Path {
	best, bestDist := 0, math.Inf(1)
	for i, pt := range p.Pts {
		if d := geom.Dist(pt, start); d < bestDist {
			best, bestDist = i, d
		}
	}
	if best == 0 {
		return p
	}
	pts := make([]drawing.Vec2, 0, len(p.Pts))
	pts = append(pts, p.Pts[best:]...)
	pts = append(pts, p.Pts[:best]...)
	return drawing.Path{Pts: pts, Closed: true}
}

// reverse returns the path drawn from its other end.
func reverse(p drawing.Path) drawing.Path {
	pts := make([]drawing.Vec2, len(p.Pts))
	for i, pt := range p.Pts {
		pts[len(p.Pts)-1-i] = pt
	}
	return drawing.Path{Pts: pts, Closed: p.Closed}
}

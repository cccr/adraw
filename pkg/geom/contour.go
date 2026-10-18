package geom

import (
	"math"

	"github.com/cccr/adraw/pkg/drawing"
)

// Field is a scalar function over the plane. Contour traces its level sets.
type Field func(x, y float64) float64

// ContourOptions configures isoline extraction.
type ContourOptions struct {
	// Bounds is the region to sample.
	Bounds drawing.Rect

	// Step is the sampling grid spacing, in world units. Smaller gives smoother
	// curves at quadratic cost.
	Step float64

	// Levels are the field values to trace.
	Levels []float64
}

// Contour extracts the isolines of a field at the given levels, using marching
// squares.
//
// Marching squares naturally emits one or two short segments per grid cell. Left
// like that a single contour would be thousands of disconnected stubs: correct
// on screen, but hours of pen lifts on a plotter and useless for any later pass
// that wants whole curves. So the segments are stitched back into long
// polylines, and closed loops are marked as such.
func Contour(f Field, opt ContourOptions) []drawing.Path {
	if opt.Step <= 0 || len(opt.Levels) == 0 {
		return nil
	}
	cols := int(opt.Bounds.Width()/opt.Step) + 2
	rows := int(opt.Bounds.Height()/opt.Step) + 2
	if cols < 2 || rows < 2 {
		return nil
	}

	// Sample once and reuse across every level: the field is usually far more
	// expensive than the marching itself.
	grid := make([]float64, cols*rows)
	for j := 0; j < rows; j++ {
		y := opt.Bounds.Min.Y + float64(j)*opt.Step
		for i := 0; i < cols; i++ {
			grid[j*cols+i] = f(opt.Bounds.Min.X+float64(i)*opt.Step, y)
		}
	}

	// Nudge levels off the sampled values.
	//
	// When a level lands exactly on a grid sample — which is not exotic, a
	// circle of radius 30 on a half-millimetre grid does it at four points —
	// the cell around it is degenerate and marching squares emits segments that
	// do not quite meet, tearing the curve. Displacing the level by a hair
	// makes exact coincidence effectively impossible, and the shift is nine
	// orders of magnitude below anything a pen could render.
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range grid {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	nudge := (hi - lo) * 1e-9

	var out []drawing.Path
	for _, level := range opt.Levels {
		segs := marchingSquares(grid, cols, rows, opt, level+nudge)
		out = append(out, stitch(segs, opt.Step)...)
	}
	return out
}

type segment struct{ a, b Vec2 }

func marchingSquares(grid []float64, cols, rows int, opt ContourOptions, level float64) []segment {
	var segs []segment

	at := func(i, j int) float64 { return grid[j*cols+i] }
	pos := func(i, j int) Vec2 {
		return Vec2{
			X: opt.Bounds.Min.X + float64(i)*opt.Step,
			Y: opt.Bounds.Min.Y + float64(j)*opt.Step,
		}
	}
	// Linear interpolation along an edge to where the field crosses the level.
	// This is what makes the curve smooth rather than stair-stepped.
	lerpEdge := func(i0, j0, i1, j1 int) Vec2 {
		v0, v1 := at(i0, j0), at(i1, j1)
		p0, p1 := pos(i0, j0), pos(i1, j1)
		if d := v1 - v0; math.Abs(d) > 1e-12 {
			return Lerp(p0, p1, (level-v0)/d)
		}
		return Lerp(p0, p1, 0.5)
	}

	for j := 0; j < rows-1; j++ {
		for i := 0; i < cols-1; i++ {
			// Corners, counter-clockwise from bottom-left.
			bl, br := at(i, j), at(i+1, j)
			tr, tl := at(i+1, j+1), at(i, j+1)

			var code int
			if bl > level {
				code |= 1
			}
			if br > level {
				code |= 2
			}
			if tr > level {
				code |= 4
			}
			if tl > level {
				code |= 8
			}
			if code == 0 || code == 15 {
				continue
			}

			bottom := func() Vec2 { return lerpEdge(i, j, i+1, j) }
			right := func() Vec2 { return lerpEdge(i+1, j, i+1, j+1) }
			top := func() Vec2 { return lerpEdge(i, j+1, i+1, j+1) }
			left := func() Vec2 { return lerpEdge(i, j, i, j+1) }

			switch code {
			case 1, 14:
				segs = append(segs, segment{left(), bottom()})
			case 2, 13:
				segs = append(segs, segment{bottom(), right()})
			case 3, 12:
				segs = append(segs, segment{left(), right()})
			case 4, 11:
				segs = append(segs, segment{right(), top()})
			case 6, 9:
				segs = append(segs, segment{bottom(), top()})
			case 7, 8:
				segs = append(segs, segment{left(), top()})
			case 5, 10:
				// Ambiguous saddle: the two diagonal corners are above the level
				// and the other two below, so the cell can be cut two ways. The
				// cell's average decides which, which keeps the choice
				// consistent with neighbours and stops contours crossing.
				avg := (bl + br + tr + tl) / 4
				if (code == 5) == (avg > level) {
					segs = append(segs, segment{left(), top()}, segment{bottom(), right()})
				} else {
					segs = append(segs, segment{left(), bottom()}, segment{right(), top()})
				}
			}
		}
	}
	return segs
}

// stitch joins segments sharing an endpoint into long polylines.
//
// Matching is very nearly exact, and deliberately so. Two cells sharing an edge
// interpolate the crossing from identical inputs, so the endpoint they produce
// is bit-for-bit the same; a generous tolerance buys nothing and actively
// breaks the walk. Where a contour runs nearly along a grid line, many
// unrelated segment ends fall within a fraction of a cell of each other, and a
// loose match will hop onto the wrong one, stranding the rest of the curve as
// orphan fragments.
func stitch(segs []segment, step float64) []drawing.Path {
	if len(segs) == 0 {
		return nil
	}
	tol := step * 1e-6

	// Index every endpoint. Entry 2k is segment k's start, 2k+1 its end.
	var box drawing.Rect
	for i, s := range segs {
		r := drawing.BoundsOf([]Vec2{s.a, s.b})
		if i == 0 {
			box = r
		} else {
			box = box.Union(r)
		}
	}
	grid := NewGrid(box, math.Max(step, 1e-9))
	for _, s := range segs {
		grid.Add(s.a)
		grid.Add(s.b)
	}

	used := make([]bool, len(segs))
	var scratch []int

	// findNext returns the unused segment whose endpoint is closest to p, along
	// with that segment's far end. Nearest rather than first-found: at a place
	// where several curves pass close together, taking any candidate within
	// tolerance is how a walk jumps tracks.
	findNext := func(p Vec2) (int, Vec2, bool) {
		scratch = grid.Within(p, tol, scratch[:0])
		best, bestDist := -1, math.Inf(1)
		for _, idx := range scratch {
			if used[idx/2] {
				continue
			}
			if d := Dist(grid.At(idx), p); d < bestDist {
				best, bestDist = idx, d
			}
		}
		if best < 0 {
			return 0, Vec2{}, false
		}
		seg := best / 2
		if best%2 == 0 {
			return seg, segs[seg].b, true
		}
		return seg, segs[seg].a, true
	}

	var out []drawing.Path
	for start := range segs {
		if used[start] {
			continue
		}
		used[start] = true
		pts := []Vec2{segs[start].a, segs[start].b}

		// Extend forward from the tail, then backward from the head, so an open
		// curve found from its middle still comes out whole.
		for {
			seg, far, ok := findNext(pts[len(pts)-1])
			if !ok {
				break
			}
			used[seg] = true
			pts = append(pts, far)
		}
		for {
			seg, far, ok := findNext(pts[0])
			if !ok {
				break
			}
			used[seg] = true
			pts = append([]Vec2{far}, pts...)
		}

		closed := len(pts) > 3 && Dist(pts[0], pts[len(pts)-1]) <= tol
		if closed {
			pts = pts[:len(pts)-1] // the closing point is implied
		}
		if len(pts) >= 2 {
			out = append(out, drawing.Path{Pts: pts, Closed: closed})
		}
	}
	return out
}

// LevelsBetween returns count evenly spaced values strictly inside (lo,hi),
// which is what a contour map wants: a level exactly at an extreme traces
// nothing.
func LevelsBetween(lo, hi float64, count int) []float64 {
	if count < 1 {
		return nil
	}
	out := make([]float64, count)
	for i := range out {
		out[i] = lo + (hi-lo)*float64(i+1)/float64(count+1)
	}
	return out
}

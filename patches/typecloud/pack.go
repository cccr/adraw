package typecloud

import (
	"math"
	"sort"

	"github.com/cccr/adraw/pkg/drawing"
)

// grid is the page as a bitmap of cells, true where ink may not go: outside the
// drawing area, or already taken by a placed chunk (plus its gap).
type grid struct {
	ox, oy float64 // page position of cell (0,0)
	cell   float64 // cell side in mm
	cols   int
	rows   int
	occ    []bool

	free     int // cells that were free at the start
	occupied int // free cells taken since
}

func newGrid(bounds drawing.Rect, cell float64, inside func(drawing.Vec2) bool) *grid {
	g := &grid{ox: bounds.Min.X, oy: bounds.Min.Y, cell: cell}
	g.cols = int(math.Ceil(bounds.Width() / cell))
	g.rows = int(math.Ceil(bounds.Height() / cell))
	g.occ = make([]bool, g.cols*g.rows)
	for r := 0; r < g.rows; r++ {
		for c := 0; c < g.cols; c++ {
			centre := drawing.Vec2{X: g.ox + (float64(c)+0.5)*cell, Y: g.oy + (float64(r)+0.5)*cell}
			if inside(centre) {
				g.free++
			} else {
				g.occ[r*g.cols+c] = true
			}
		}
	}
	return g
}

// coverage is the share of the drawing area now taken, 0 to 1.
func (g *grid) coverage() float64 {
	if g.free == 0 {
		return 1
	}
	return float64(g.occupied) / float64(g.free)
}

// offset is a cell relative to a chunk's own origin cell.
type offset struct{ dx, dy int }

// mask is the set of cells a chunk's ink takes up, thickened by half the gap so
// that two masks which do not touch leave a full gap between their ink.
type mask struct {
	cells []offset
	// bounding box of the cells, relative to the chunk origin
	minX, minY, maxX, maxY int
}

// rasterise turns paths (in mm, relative to the chunk origin) into a mask. The
// ink is sampled twice per cell and each sample stamps a disc of radius
// radius plus three quarters of a cell, which covers a sample sitting anywhere
// in its cell (up to 0.71 cell from the centre). The disc is never under 1.5
// cells: when the grid is coarse next to the gap, the neighbouring cells must
// still count as touching, or two chunks could sit with ink a hair apart.
func rasterise(paths []drawing.Path, cell, radius float64) mask {
	rc := math.Max(radius/cell+0.75, 1.5)
	var disc []offset
	reach := int(math.Ceil(rc))
	for dy := -reach; dy <= reach; dy++ {
		for dx := -reach; dx <= reach; dx++ {
			if float64(dx*dx+dy*dy) <= rc*rc {
				disc = append(disc, offset{dx, dy})
			}
		}
	}

	var samples []offset
	for _, p := range paths {
		for i := 0; i < len(p.Pts); i++ {
			a := p.Pts[i]
			samples = append(samples, offset{int(math.Floor(a.X / cell)), int(math.Floor(a.Y / cell))})
			if i == 0 {
				continue
			}
			b := p.Pts[i-1]
			n := int(math.Hypot(a.X-b.X, a.Y-b.Y)/(cell/2)) + 1
			for k := 1; k < n; k++ {
				f := float64(k) / float64(n)
				x, y := b.X+(a.X-b.X)*f, b.Y+(a.Y-b.Y)*f
				samples = append(samples, offset{int(math.Floor(x / cell)), int(math.Floor(y / cell))})
			}
		}
	}
	if len(samples) == 0 {
		return mask{}
	}

	m := mask{minX: math.MaxInt32, minY: math.MaxInt32, maxX: math.MinInt32, maxY: math.MinInt32}
	for _, s := range samples {
		m.minX, m.maxX = min(m.minX, s.dx-reach), max(m.maxX, s.dx+reach)
		m.minY, m.maxY = min(m.minY, s.dy-reach), max(m.maxY, s.dy+reach)
	}
	w, h := m.maxX-m.minX+1, m.maxY-m.minY+1
	bits := make([]bool, w*h)
	for _, s := range samples {
		for _, d := range disc {
			bits[(s.dy+d.dy-m.minY)*w+(s.dx+d.dx-m.minX)] = true
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if bits[y*w+x] {
				m.cells = append(m.cells, offset{x + m.minX, y + m.minY})
			}
		}
	}
	return m
}

// rasteriseFilled is rasterise for solid letters: the inside of every contour
// counts as ink too (nonzero winding, as fonts define it, so a letter's
// counters stay open to the eye but belong to the letter for packing, which
// leaves them unusable: a small word inside an "o" would be hard to read).
//
// The shape thickened by the gap is the shape itself plus the outline thickened
// by the gap, so the outline's mask is reused and the fill added to it.
func rasteriseFilled(paths []drawing.Path, cell, radius float64) mask {
	m := rasterise(paths, cell, radius)
	if len(m.cells) == 0 {
		return m
	}
	w, h := m.maxX-m.minX+1, m.maxY-m.minY+1
	bits := make([]bool, w*h)
	for _, c := range m.cells {
		bits[(c.dy-m.minY)*w+(c.dx-m.minX)] = true
	}

	// Scanline each cell row through the contours' edges, at the row's centre.
	type crossing struct {
		x   float64
		dir int
	}
	for row := 0; row < h; row++ {
		y := (float64(row+m.minY) + 0.5) * cell
		var xs []crossing
		for _, p := range paths {
			n := len(p.Pts)
			for i := 0; i < n; i++ {
				a, b := p.Pts[i], p.Pts[(i+1)%n]
				if (a.Y <= y) == (b.Y <= y) {
					continue
				}
				x := a.X + (y-a.Y)/(b.Y-a.Y)*(b.X-a.X)
				dir := 1
				if b.Y < a.Y {
					dir = -1
				}
				xs = append(xs, crossing{x, dir})
			}
		}
		sort.Slice(xs, func(i, j int) bool { return xs[i].x < xs[j].x })
		wind := 0
		for i := 0; i+1 < len(xs); i++ {
			wind += xs[i].dir
			if wind == 0 {
				continue
			}
			// cells whose centre lies between this crossing and the next
			from := int(math.Ceil(xs[i].x/cell-0.5)) - m.minX
			to := int(math.Floor(xs[i+1].x/cell-0.5)) - m.minX
			for col := max(from, 0); col <= min(to, w-1); col++ {
				bits[row*w+col] = true
			}
		}
	}

	m.cells = m.cells[:0]
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if bits[y*w+x] {
				m.cells = append(m.cells, offset{x + m.minX, y + m.minY})
			}
		}
	}
	return m
}

// fits reports whether m placed with its origin at cell (px,py) overlaps
// nothing, and how many cells it looked at, so callers can keep a budget.
func (g *grid) fits(m mask, px, py int) (ok bool, looked int) {
	if px+m.minX < 0 || py+m.minY < 0 || px+m.maxX >= g.cols || py+m.maxY >= g.rows {
		return false, 1
	}
	for _, c := range m.cells {
		looked++
		if g.occ[(py+c.dy)*g.cols+px+c.dx] {
			return false, looked
		}
	}
	return true, looked
}

func (g *grid) take(m mask, px, py int) {
	for _, c := range m.cells {
		i := (py+c.dy)*g.cols + px + c.dx
		if !g.occ[i] {
			g.occ[i] = true
			g.occupied++
		}
	}
}

// centres is every lattice position (in cells, stride apart) of the grid,
// nearest the middle first. A chunk is tried with its own centre on each in
// turn, so the cloud grows outward from the middle of the page.
func (g *grid) centres(stride int) []offset {
	var out []offset
	for y := 0; y < g.rows; y += stride {
		for x := 0; x < g.cols; x += stride {
			out = append(out, offset{x, y})
		}
	}
	// Sort on a precomputed squared distance: computing it inside the comparator
	// would repeat it n log n times.
	cx, cy := float64(g.cols)/2, float64(g.rows)/2
	d2 := make([]float64, len(out))
	idx := make([]int, len(out))
	for i, o := range out {
		dx, dy := float64(o.dx)-cx, float64(o.dy)-cy
		d2[i], idx[i] = dx*dx+dy*dy, i
	}
	sort.SliceStable(idx, func(i, j int) bool { return d2[idx[i]] < d2[idx[j]] })
	sorted := make([]offset, len(out))
	for i, k := range idx {
		sorted[i] = out[k]
	}
	out = sorted
	return out
}

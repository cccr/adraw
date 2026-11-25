// Package typecloud fills the page with the user's own words.
//
// The text is split at ";" into chunks. Each chunk gets a stroke font and a
// size, and the chunks are packed like a word cloud: biggest first, each one at
// the free spot nearest the middle where its real letter shapes (not just a box)
// clear everything already placed. Chunks then repeat at smaller and smaller
// sizes to use up the gaps. All text is horizontal.
package typecloud

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/font"
	"github.com/cccr/adraw/pkg/geom"
	"github.com/cccr/adraw/pkg/outlinefont"
	"github.com/cccr/adraw/pkg/patch"
)

const (
	defaultWidth  = 297
	defaultHeight = 210

	defaultText = "WORDS; fill the page; in many; fonts; and sizes; ink; lines; typecloud"

	// budget caps how many grid cells all placement tests together may look at.
	// It is a count, not a clock, so a seed always draws the same picture.
	budget = 120_000_000

	// maxCells caps the grid so a huge sheet does not need a huge bitmap.
	maxCells = 1_000_000
)

var params = patch.NewParamSet().
	Text("text", "Text",
		"What to write. Separate chunks with ;  and write \\; for a literal semicolon.",
		defaultText).
	MM("min", "Smallest text",
		"Height of a capital in the smallest repeats. Smaller fills more gaps.", 3, 20, 5).
	MM("max", "Largest text",
		"Height of a capital in the biggest chunk. Larger makes a bolder contrast.", 10, 120, 45).
	MM("gap", "Gap",
		"Clear space between neighbouring chunks. Set it from your pen width.", 0.5, 10, 2).
	Unit("fill", "Fill",
		"How much of the page to cover before the repeats stop. Higher packs tighter.", 0, 1, 0.85).
	Count("fonts", "Fonts",
		"How many different fonts to use. 1 keeps a single style.", 1, 11, 4).
	Count("style", "Letters",
		"0 single-line plotter fonts, 1 regular solid fonts. Solid letters cannot be "+
			"plotted as lines: use the PNG download (SVG and G-code give their outlines).",
		0, 1, 0).
	MM("margin", "Page margin",
		"Blank border left around the drawing.", 0, 40, 10)

type Typecloud struct{}

func New() *Typecloud { return &Typecloud{} }

func init() { patch.Register(New()) }

func (t *Typecloud) Name() string { return "typecloud" }

func (t *Typecloud) Description() string {
	return "Your own words packed edge to edge in different plotter fonts and sizes, like a word cloud."
}

func (t *Typecloud) Params() []patch.ParamSpec { return params.Specs() }

func spec(key string) patch.ParamSpec {
	s, ok := params.Find(key)
	if !ok {
		panic("typecloud: unknown parameter " + key)
	}
	return s
}

// typeface sets a line of text: contours or strokes in mm, with the origin at the
// top-left of the box and a box w by h. Both the plotter line fonts (pkg/font)
// and the regular solid ones (pkg/outlinefont) satisfy it.
type typeface interface {
	Layout(text string, size float64) (paths []drawing.Path, w, h float64)
}

// face is a typeface and its name.
type face struct {
	name string
	typeface
}

// placement is one chunk set down on the page.
type placement struct {
	chunk      int
	name       string // the face's name
	size       float64
	x, y, w, h float64 // the chunk's line box, in page mm
	paths      []drawing.Path
}

func (t *Typecloud) Generate(ctx context.Context, req patch.Request) (drawing.Frame, error) {
	pl, bounds, err := t.place(ctx, req)
	if err != nil {
		return drawing.Frame{}, err
	}
	var paths []drawing.Path
	for _, p := range pl {
		paths = append(paths, p.paths...)
	}
	v := req.Values
	return drawing.Frame{
		Bounds: bounds,
		Paths:  paths,
		Meta: map[string]string{
			"patch":  t.Name(),
			"seed":   fmt.Sprint(req.Seed),
			"text":   req.Strings.Get(spec("text")),
			"min":    fmt.Sprintf("%g", v.Get(spec("min"))),
			"max":    fmt.Sprintf("%g", v.Get(spec("max"))),
			"gap":    fmt.Sprintf("%g", v.Get(spec("gap"))),
			"fill":   fmt.Sprintf("%g", v.Get(spec("fill"))),
			"fonts":  fmt.Sprint(int(math.Round(v.Get(spec("fonts"))))),
			"margin": fmt.Sprintf("%g", v.Get(spec("margin"))),
			"style":  fmt.Sprint(int(math.Round(v.Get(spec("style"))))),
		},
	}, nil
}

// place decides where every chunk goes. Generate only turns the result into paths.
func (t *Typecloud) place(ctx context.Context, req patch.Request) ([]placement, drawing.Rect, error) {
	v := req.Values
	var (
		minSize = v.Get(spec("min"))
		maxSize = math.Max(v.Get(spec("max")), minSize)
		gap     = v.Get(spec("gap"))
		fill    = v.Get(spec("fill"))
		nFonts  = int(math.Round(v.Get(spec("fonts"))))
		margin  = v.Get(spec("margin"))
		solid   = math.Round(v.Get(spec("style"))) >= 1
	)

	// The drawing area: a border polygon if given, else the page less its margin.
	var bounds drawing.Rect
	var inside func(drawing.Vec2) bool
	if len(req.Border) >= 3 {
		poly := geom.Polygon(req.Border)
		bounds = drawing.BoundsOf(req.Border)
		inside = func(p drawing.Vec2) bool { return geom.Contains(poly, p) }
	} else {
		w, h := req.PageOr(defaultWidth, defaultHeight)
		bounds = drawing.Rect{Max: drawing.Vec2{X: w, Y: h}}
		m := math.Min(margin, math.Min(w, h)/4)
		inside = func(p drawing.Vec2) bool {
			return p.X >= m && p.X <= w-m && p.Y >= m && p.Y <= h-m
		}
	}

	rng := rand.New(rand.NewSource(req.Seed))

	// Fonts in play, and which chunk gets which. Chunks that cannot be drawn at
	// all (no glyph the fonts know) are dropped before anything else.
	// Solid letters need the regular faces; if none loaded, fall back to lines.
	var all []face
	if solid {
		for _, f := range outlinefont.All {
			all = append(all, face{f.Name, f})
		}
	}
	if len(all) == 0 {
		solid = false
		for _, f := range font.All {
			all = append(all, face{f.Name, f})
		}
	}
	perm := rng.Perm(len(all))
	play := make([]face, 0, nFonts)
	for _, i := range perm[:min(nFonts, len(perm))] {
		play = append(play, all[i])
	}
	chunks := drawable(splitChunks(req.Strings.Get(spec("text"))), play[0])
	if len(chunks) == 0 {
		chunks = splitChunks(defaultText)
	}
	order := rng.Perm(len(chunks)) // order[r] is the chunk with size rank r

	cell := math.Max(0.5, math.Min(1, gap/2))
	cell = math.Max(cell, math.Sqrt(bounds.Width()*bounds.Height()/maxCells))
	g := newGrid(bounds, cell, inside)
	// Candidate spots are a lattice over the grid, about 2 mm apart on a normal
	// sheet and never more than about 65 thousand of them on a huge one.
	stride := max(1, int(math.Round(2/cell)), int(math.Ceil(math.Sqrt(float64(g.cols*g.rows)/65536))))
	centres := g.centres(stride)
	radius := gap / 2

	var placed []placement
	left := budget
	// try places chunk c at the given size at the free spot nearest the middle.
	try := func(c int, f face, size float64) bool {
		paths, w, h := f.Layout(chunks[c], size)
		if w <= 0 || len(paths) == 0 || w > bounds.Width() || h > bounds.Height() {
			return false
		}
		var m mask
		if solid {
			m = rasteriseFilled(paths, cell, radius)
		} else {
			m = rasterise(paths, cell, radius)
		}
		hw, hh := int(w/cell/2), int(h/cell/2)
		for _, ctr := range centres {
			if left <= 0 {
				return false
			}
			px, py := ctr.dx-hw, ctr.dy-hh
			ok, looked := g.fits(m, px, py)
			left -= looked
			if !ok {
				continue
			}
			g.take(m, px, py)
			x, y := g.ox+float64(px)*cell, g.oy+float64(py)*cell
			moved := make([]drawing.Path, len(paths))
			for i, p := range paths {
				pts := make([]drawing.Vec2, len(p.Pts))
				for j, q := range p.Pts {
					pts[j] = drawing.Vec2{X: q.X + x, Y: q.Y + y}
				}
				moved[i] = drawing.Path{Pts: pts, Closed: p.Closed, Fill: p.Fill}
			}
			placed = append(placed, placement{chunk: c, name: f.name, size: size, x: x, y: y, w: w, h: h, paths: moved})
			return true
		}
		return false
	}

	// Pass one: every chunk once, biggest first, shrinking until it fits.
	n := len(chunks)
	lo := math.Min(maxSize, math.Max(minSize, maxSize/4))
	fontOf := make([]face, n)
	sizeOf := make([]float64, n)
	for rank, c := range order {
		fontOf[c] = play[rank%len(play)]
		sizeOf[c] = maxSize
		if n > 1 {
			sizeOf[c] = maxSize * math.Pow(lo/maxSize, float64(rank)/float64(n-1))
		}
	}
	smallest := maxSize
	for _, c := range order {
		if err := ctx.Err(); err != nil {
			return nil, bounds, err
		}
		for s := sizeOf[c]; s >= minSize-1e-9 && left > 0; s *= 0.92 {
			if try(c, fontOf[c], s) {
				smallest = math.Min(smallest, s)
				break
			}
		}
	}

	if len(placed) == 0 {
		placed = append(placed, forced(chunks[order[0]], play[0], bounds, inside))
	}

	// Pass two: repeats at falling sizes until the page is as full as asked.
	failedAt := make([]float64, n)
	for i := range failedAt {
		failedAt[i] = math.Inf(1)
	}
	for s := smallest * 0.85; s >= minSize-1e-9 && g.coverage() < fill && left > 0; s *= 0.85 {
		for progress := true; progress && g.coverage() < fill && left > 0; {
			progress = false
			for _, c := range order {
				if err := ctx.Err(); err != nil {
					return nil, bounds, err
				}
				if s >= failedAt[c] || g.coverage() >= fill {
					continue
				}
				if try(c, fontOf[c], s) {
					progress = true
				} else {
					failedAt[c] = s
				}
			}
		}
	}
	return placed, bounds, ctx.Err()
}

// drawable drops chunks none of whose characters the typeface can draw.
func drawable(chunks []string, f typeface) []string {
	var out []string
	for _, c := range chunks {
		// A space has width but no ink, so ask for strokes, not for width.
		if paths, _, _ := f.Layout(c, 10); len(paths) > 0 {
			out = append(out, c)
		}
	}
	return out
}

// forced puts one chunk in the middle, scaled to fit, for when nothing fitted at
// the smallest size. A page is never left blank.
func forced(text string, f face, bounds drawing.Rect, inside func(drawing.Vec2) bool) placement {
	size := 10.0
	cx, cy := (bounds.Min.X+bounds.Max.X)/2, (bounds.Min.Y+bounds.Max.Y)/2
	for i := 0; i < 80; i++ {
		_, w, h := f.Layout(text, size)
		x, y := cx-w/2, cy-h/2
		corners := []drawing.Vec2{{X: x, Y: y}, {X: x + w, Y: y}, {X: x, Y: y + h}, {X: x + w, Y: y + h}}
		ok := w <= bounds.Width() && h <= bounds.Height()
		for _, c := range corners {
			ok = ok && inside(c)
		}
		if ok || i == 79 {
			paths, w, h := f.Layout(text, size)
			x, y = cx-w/2, cy-h/2
			for _, p := range paths {
				for j := range p.Pts {
					p.Pts[j].X += x
					p.Pts[j].Y += y
				}
			}
			return placement{name: f.name, size: size, x: x, y: y, w: w, h: h, paths: paths}
		}
		size *= 0.8
	}
	return placement{}
}

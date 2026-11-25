// Package outlinefont sets text in ordinary typefaces and returns the letters
// as solid shapes: closed contours flagged Fill, which a raster renderer fills
// and a plotter draws as outlines.
//
// The faces are open-licence TrueType files embedded in the binary (see
// README.md for each licence). Characters a face lacks are skipped.
package outlinefont

import (
	"embed"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/cccr/adraw/pkg/drawing"
)

//go:embed fonts/*.ttf
var files embed.FS

// flatness is how far a flattened curve may stray from the true one, in mm.
const flatness = 0.1

// Font is one typeface.
type Font struct {
	Name string

	face     *sfnt.Font
	ppem     fixed.Int26_6 // one pixel per font unit, so coordinates come out in units
	capUnits float64       // height of a capital H, in font units
}

var faces = []struct{ name, file string }{
	{"sans", "Lato-Regular.ttf"},
	{"serif", "CrimsonText-Regular.ttf"},
	{"slab", "Arvo-Regular.ttf"},
	{"fat", "AbrilFatface-Regular.ttf"},
	{"script", "Pacifico-Regular.ttf"},
	{"condensed", "BebasNeue-Regular.ttf"},
	{"typewriter", "CourierPrime-Regular.ttf"},
	{"lobster", "Lobster-Regular.ttf"},
	{"blackletter", "UnifrakturCook-Bold.ttf"},
	{"comic", "Bangers-Regular.ttf"},
	{"deco", "Righteous-Regular.ttf"},
}

// All is every face that loaded, in a stable order.
var All = load()

func load() []*Font {
	var out []*Font
	for _, fc := range faces {
		data, err := files.ReadFile("fonts/" + fc.file)
		if err != nil {
			continue
		}
		face, err := sfnt.Parse(data)
		if err != nil {
			continue // a face that will not parse is left out, not fatal
		}
		f := &Font{Name: fc.name, face: face, ppem: fixed.I(int(face.UnitsPerEm()))}
		var b sfnt.Buffer
		h, err := face.GlyphIndex(&b, 'H')
		if err != nil || h == 0 {
			continue
		}
		bounds, _, err := face.GlyphBounds(&b, h, f.ppem, font.HintingNone)
		if err != nil || bounds.Min.Y >= 0 || bounds.Max.Y <= bounds.Min.Y {
			continue
		}
		// The H's whole height, so a face whose H dips below the baseline (a
		// blackletter) still draws an H of the size asked for.
		f.capUnits = float64(bounds.Max.Y-bounds.Min.Y) / 64
		out = append(out, f)
	}
	return out
}

// Layout sets text on one line. size is the cap height in mm. The contours are
// in mm with the origin at the top-left of the ink and y down; w and h are the
// size of that box, so every point lies in [0,w]x[0,h]. Characters the face
// lacks are skipped; text with none returns no contours and w == 0.
func (f *Font) Layout(text string, size float64) (paths []drawing.Path, w, h float64) {
	k := size / f.capUnits
	var b sfnt.Buffer
	penX := 0.0
	any := false
	for _, r := range text {
		idx, err := f.face.GlyphIndex(&b, r)
		if err != nil || idx == 0 {
			continue
		}
		adv, err := f.face.GlyphAdvance(&b, idx, f.ppem, font.HintingNone)
		if err != nil {
			continue
		}
		any = true
		segs, err := f.face.LoadGlyph(&b, idx, f.ppem, nil)
		if err == nil {
			paths = append(paths, contours(segs, penX, k)...)
		}
		penX += float64(adv) / 64
	}
	if !any {
		return nil, 0, 0
	}

	// Box the ink, widened to the advance width so spacing is kept.
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range paths {
		for _, pt := range p.Pts {
			minX, maxX = math.Min(minX, pt.X), math.Max(maxX, pt.X)
			minY, maxY = math.Min(minY, pt.Y), math.Max(maxY, pt.Y)
		}
	}
	if len(paths) == 0 { // only spaces: width, no ink
		return nil, penX * k, 0
	}
	minX = math.Min(minX, 0)
	maxX = math.Max(maxX, penX*k)
	for _, p := range paths {
		for i := range p.Pts {
			p.Pts[i].X -= minX
			p.Pts[i].Y -= minY
		}
	}
	return paths, maxX - minX, maxY - minY
}

// contours turns a glyph's segments into closed polygons, x offset by penX
// font units and everything scaled by k (mm per font unit).
func contours(segs sfnt.Segments, penX, k float64) []drawing.Path {
	pt := func(p fixed.Point26_6) drawing.Vec2 {
		return drawing.Vec2{X: (penX + float64(p.X)/64) * k, Y: float64(p.Y) / 64 * k}
	}
	var out []drawing.Path
	var cur []drawing.Vec2
	flush := func() {
		if n := len(cur); n > 1 && cur[0] == cur[n-1] {
			cur = cur[:n-1]
		}
		if len(cur) >= 3 {
			out = append(out, drawing.Path{Pts: cur, Closed: true, Fill: true})
		}
		cur = nil
	}
	for _, s := range segs {
		if s.Op != sfnt.SegmentOpMoveTo && len(cur) == 0 {
			continue // some faces' space glyph is a lone curve with no start
		}
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			flush()
			cur = []drawing.Vec2{pt(s.Args[0])}
		case sfnt.SegmentOpLineTo:
			cur = append(cur, pt(s.Args[0]))
		case sfnt.SegmentOpQuadTo:
			p0, p1, p2 := cur[len(cur)-1], pt(s.Args[0]), pt(s.Args[1])
			dev := math.Hypot(p0.X-2*p1.X+p2.X, p0.Y-2*p1.Y+p2.Y)
			n := steps(dev / 4)
			for i := 1; i <= n; i++ {
				t := float64(i) / float64(n)
				u := 1 - t
				cur = append(cur, drawing.Vec2{
					X: u*u*p0.X + 2*u*t*p1.X + t*t*p2.X,
					Y: u*u*p0.Y + 2*u*t*p1.Y + t*t*p2.Y})
			}
		case sfnt.SegmentOpCubeTo:
			p0, p1, p2, p3 := cur[len(cur)-1], pt(s.Args[0]), pt(s.Args[1]), pt(s.Args[2])
			d1 := math.Hypot(p0.X-2*p1.X+p2.X, p0.Y-2*p1.Y+p2.Y)
			d2 := math.Hypot(p1.X-2*p2.X+p3.X, p1.Y-2*p2.Y+p3.Y)
			n := steps(0.75 * math.Max(d1, d2))
			for i := 1; i <= n; i++ {
				t := float64(i) / float64(n)
				u := 1 - t
				cur = append(cur, drawing.Vec2{
					X: u*u*u*p0.X + 3*u*u*t*p1.X + 3*u*t*t*p2.X + t*t*t*p3.X,
					Y: u*u*u*p0.Y + 3*u*u*t*p1.Y + 3*u*t*t*p2.Y + t*t*t*p3.Y})
			}
		}
	}
	flush()
	return out
}

// steps is how many pieces a curve needs so that cutting it into straight
// pieces strays at most flatness. err is the curve's own deviation measure
// (its error halves with every doubling of pieces squared), in mm.
func steps(err float64) int {
	n := int(math.Ceil(math.Sqrt(err / flatness)))
	return max(1, min(n, 48))
}

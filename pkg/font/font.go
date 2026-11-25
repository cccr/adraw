// Package font draws text as single-stroke lines, which is what a pen plotter
// wants: one pen pass per stroke, no outlines to hatch.
//
// The glyphs are Hershey fonts (see the notice in the data files), embedded as
// their original text records and parsed once at start-up. Only printable ASCII
// is covered; any other rune is skipped.
package font

import (
	"math"

	"github.com/cccr/adraw/pkg/drawing"
)

// capUnits is the cap height of a Hershey glyph in font units (y runs from -12
// at the top of a capital to +9 on the baseline). Layout's size is scaled so a
// capital is size mm tall, whichever font.
const capUnits = 21.0

// Font is one set of stroke glyphs.
type Font struct {
	Name string

	glyphs     [95]glyph // ASCII 32..126
	minY, maxY float64   // vertical extent over all glyphs, in font units
}

type glyph struct {
	advance float64 // right bearing minus left bearing
	strokes [][]point
}

type point struct{ x, y float64 } // relative to the glyph's left bearing

// All is every font, in a stable order.
var All = []*Font{
	parse("simplex", &rawSimplex),
	parse("roman", &rawRoman),
	parse("script", &rawScript),
	parse("gothic", &rawGothic),
	parse("duplex", &rawDuplex),
	parse("triplex", &rawTriplex),
	parse("cursive", &rawCursive),
	parse("italic", &rawItalic),
	parse("bolditalic", &rawBolditalic),
	parse("fraktur", &rawFraktur),
	parse("chancery", &rawChancery),
}

// parse reads Hershey records: two characters (left, right bearing) then
// coordinate pairs, each character being its offset from 'R'. " R" lifts the pen.
func parse(name string, raw *[95]string) *Font {
	f := &Font{Name: name, minY: math.Inf(1), maxY: math.Inf(-1)}
	val := func(c byte) float64 { return float64(int(c) - 'R') }
	for i, rec := range raw {
		if len(rec) < 2 {
			continue
		}
		left, right := val(rec[0]), val(rec[1])
		g := glyph{advance: right - left}
		var cur []point
		flush := func() {
			if len(cur) > 1 {
				g.strokes = append(g.strokes, cur)
			} else if len(cur) == 1 { // a lone point is a dot
				g.strokes = append(g.strokes, []point{cur[0], {cur[0].x + 0.01, cur[0].y}})
			}
			cur = nil
		}
		for j := 2; j+1 < len(rec); j += 2 {
			if rec[j] == ' ' && rec[j+1] == 'R' {
				flush()
				continue
			}
			p := point{val(rec[j]) - left, val(rec[j+1])}
			cur = append(cur, p)
			f.minY = math.Min(f.minY, p.y)
			f.maxY = math.Max(f.maxY, p.y)
		}
		flush()
		f.glyphs[i] = g
	}
	return f
}

// Layout sets text on one line. size is the cap height in mm. Strokes are in
// mm with the origin at the top-left of the line box, y down, and the returned
// w and h are the box's width and height, so every point lies in [0,w]x[0,h].
// Runes the font lacks are skipped; text with none returns no strokes and w == 0.
func (f *Font) Layout(text string, size float64) (paths []drawing.Path, w, h float64) {
	k := size / capUnits
	h = (f.maxY - f.minY) * k
	penX := 0.0
	any := false
	for _, r := range text {
		if r < 32 || r > 126 {
			continue
		}
		any = true
		g := f.glyphs[r-32]
		for _, s := range g.strokes {
			pts := make([]drawing.Vec2, len(s))
			for i, p := range s {
				pts[i] = drawing.Vec2{X: (penX + p.x) * k, Y: (p.y - f.minY) * k}
			}
			paths = append(paths, drawing.Path{Pts: pts})
		}
		penX += g.advance
	}
	if !any {
		return nil, 0, 0
	}
	return paths, penX * k, h
}

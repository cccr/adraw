// Package png renders a frame as a raster image: white paper, black ink.
//
// This is the output for solid letters, which a pen plotter cannot draw. Paths
// flagged Fill are filled (nonzero winding, so a letter's counters stay open);
// every other path is drawn as a thin line, so any pattern can be saved as a PNG.
package png

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	stdpng "image/png"
	"io"
	"math"

	"golang.org/x/image/vector"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/render"
)

const (
	scale     = 10.0 // pixels per millimetre
	maxSide   = 6000 // pixels: scale is reduced so a huge sheet stays this size
	lineMM    = 0.3  // width of an unfilled line, in mm
	minLinePx = 1.5
)

// PNG implements render.Renderer.
type PNG struct{}

func New() *PNG { return &PNG{} }

func init() { render.Register(New()) }

func (*PNG) Name() string { return "png" }
func (*PNG) Ext() string  { return "png" }

func (*PNG) Render(w io.Writer, f drawing.Frame) error {
	bw, bh := f.Bounds.Width(), f.Bounds.Height()
	if bw <= 0 || bh <= 0 {
		return fmt.Errorf("png: frame has no extent (%gx%g)", bw, bh)
	}
	k := math.Min(scale, maxSide/math.Max(bw, bh))
	W, H := max(1, int(math.Round(bw*k))), max(1, int(math.Round(bh*k)))

	img := image.NewGray(image.Rect(0, 0, W, H))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	black := image.NewUniform(color.Black)

	px := func(p drawing.Vec2) (float32, float32) {
		return float32((p.X - f.Bounds.Min.X) * k), float32((p.Y - f.Bounds.Min.Y) * k)
	}

	// Filled shapes: one pass, so counters wound the other way cancel the fill.
	fill := vector.NewRasterizer(W, H)
	anyFill := false
	for _, p := range f.Paths {
		if !p.Fill || len(p.Pts) < 3 {
			continue
		}
		anyFill = true
		x, y := px(p.Pts[0])
		fill.MoveTo(x, y)
		for _, q := range p.Pts[1:] {
			x, y = px(q)
			fill.LineTo(x, y)
		}
		fill.ClosePath()
	}
	if anyFill {
		fill.Draw(img, img.Bounds(), black, image.Point{})
	}

	// Everything else as thin lines: each segment is a small quad.
	lines := vector.NewRasterizer(W, H)
	anyLine := false
	half := float32(math.Max(lineMM*k, minLinePx) / 2)
	seg := func(a, b drawing.Vec2) {
		ax, ay := px(a)
		bx, by := px(b)
		dx, dy := bx-ax, by-ay
		n := float32(math.Hypot(float64(dx), float64(dy)))
		if n == 0 {
			dx, dy, n = 1, 0, 1
		}
		nx, ny := -dy/n*half, dx/n*half
		ex, ey := dx/n*half, dy/n*half // cap the ends so dots and joins show
		lines.MoveTo(ax-ex+nx, ay-ey+ny)
		lines.LineTo(bx+ex+nx, by+ey+ny)
		lines.LineTo(bx+ex-nx, by+ey-ny)
		lines.LineTo(ax-ex-nx, ay-ey-ny)
		lines.ClosePath()
		anyLine = true
	}
	for _, p := range f.Paths {
		if p.Fill {
			continue
		}
		for i := 1; i < len(p.Pts); i++ {
			seg(p.Pts[i-1], p.Pts[i])
		}
		if p.Closed && len(p.Pts) > 2 {
			seg(p.Pts[len(p.Pts)-1], p.Pts[0])
		}
	}
	if anyLine {
		lines.Draw(img, img.Bounds(), black, image.Point{})
	}

	enc := stdpng.Encoder{CompressionLevel: stdpng.BestSpeed}
	return enc.Encode(w, img)
}

package geom

import (
	"math/rand"

	"github.com/cccr/adraw/pkg/drawing"
)

// Fill selects how a single cell is drawn.
type Fill int

const (
	FillSpiral Fill = iota
	FillInset
	FillFan
	FillHatch
	FillCrossHatch
	FillDots
	numFills
)

// FillCount is how many fills exist, for patches declaring a selector
// parameter.
const FillCount = int(numFills)

// FillOptions carries everything any fill might need. A fill ignores what does
// not apply to it, which keeps the selector a single switch rather than a
// per-fill parameter set the UI would have to show and hide.
type FillOptions struct {
	Fill    Fill
	Spacing float64
	Angle   float64
	Cross   float64
	// Clockwise and Vertex vary per cell to stop a field of cells looking
	// mechanically identical.
	Clockwise bool
	Vertex    int
}

// ApplyFill draws one cell with the selected fill.
func ApplyFill(p Polygon, opt FillOptions, rng *rand.Rand) []drawing.Path {
	switch opt.Fill {
	case FillInset:
		return Inset(p, SpiralOptions{Spacing: opt.Spacing})
	case FillFan:
		return Fan(p, opt.Spacing, opt.Vertex)
	case FillHatch:
		return Hatch(p, opt.Spacing, opt.Angle)
	case FillCrossHatch:
		return CrossHatch(p, opt.Spacing, opt.Angle, opt.Cross)
	case FillDots:
		return Dots(p, opt.Spacing, rng)
	default:
		return Spiral(p, SpiralOptions{Spacing: opt.Spacing, Clockwise: opt.Clockwise})
	}
}

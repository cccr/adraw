// Package gcode renders a Frame as G-code for a pen plotter.
package gcode

import (
	"bufio"
	"fmt"
	"io"
	"sort"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/render"
)

// Defaults in millimetres and mm/min. They suit a small hobby pen plotter and
// are the values to change first for different hardware.
const (
	DefaultPenUp   = 5
	DefaultPenDown = 0
	DefaultFeed    = 3000
	DefaultPlunge  = 1000
)

type Renderer struct {
	PenUp   float64 // Z height clear of the paper
	PenDown float64 // Z height in contact with the paper
	Feed    float64 // drawing feedrate, mm/min
	Plunge  float64 // pen raise and lower feedrate, mm/min
}

func New() *Renderer {
	return &Renderer{
		PenUp:   DefaultPenUp,
		PenDown: DefaultPenDown,
		Feed:    DefaultFeed,
		Plunge:  DefaultPlunge,
	}
}

func init() { render.Register(New()) }

func (r *Renderer) Name() string { return "gcode" }
func (r *Renderer) Ext() string  { return "nc" }

// Render writes f as G-code. Paths are emitted in generation order: travel
// between them is not optimized, which belongs in a separate Frame-to-Frame
// pass rather than hidden inside a renderer.
func (r *Renderer) Render(w io.Writer, f drawing.Frame) error {
	bw := bufio.NewWriter(w)

	writeMeta(bw, f.Meta)
	fmt.Fprintf(bw, "G21 ; millimetres\nG90 ; absolute positioning\n")
	fmt.Fprintf(bw, "G0 Z%.3f ; pen up\n", r.PenUp)

	for _, p := range f.Paths {
		if len(p.Pts) < 2 {
			continue
		}
		first := p.Pts[0]
		fmt.Fprintf(bw, "G0 X%.3f Y%.3f\n", first.X, first.Y)
		fmt.Fprintf(bw, "G1 Z%.3f F%.0f ; pen down\n", r.PenDown, r.Plunge)

		for _, pt := range p.Pts[1:] {
			fmt.Fprintf(bw, "G1 X%.3f Y%.3f F%.0f\n", pt.X, pt.Y, r.Feed)
		}
		if p.Closed {
			fmt.Fprintf(bw, "G1 X%.3f Y%.3f F%.0f\n", first.X, first.Y, r.Feed)
		}
		fmt.Fprintf(bw, "G0 Z%.3f ; pen up\n", r.PenUp)
	}

	fmt.Fprintf(bw, "G0 X0 Y0\nM2 ; end\n")
	return bw.Flush()
}

func writeMeta(w io.Writer, meta map[string]string) {
	if len(meta) == 0 {
		return
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprint(w, "; adraw:")
	for _, k := range keys {
		fmt.Fprintf(w, " %s=%s", k, meta[k])
	}
	fmt.Fprintln(w)
}

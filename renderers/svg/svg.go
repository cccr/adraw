// Package svg renders a Frame as SVG.
package svg

import (
	"bufio"
	"fmt"
	"io"
	"sort"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/render"
)

// StrokeWidth is the rendered line width in millimetres.
const StrokeWidth = 0.3

type Renderer struct{}

func New() *Renderer { return &Renderer{} }

func init() { render.Register(New()) }

func (r *Renderer) Name() string { return "svg" }
func (r *Renderer) Ext() string  { return "svg" }

// Render writes f as SVG. The viewBox is set in the frame's own millimetre
// units and width/height carry the mm suffix, so one unit on screen is one
// millimetre on the plotter and no scaling factor has to be tracked anywhere.
func (r *Renderer) Render(w io.Writer, f drawing.Frame) error {
	bw := bufio.NewWriter(w)

	width, height := f.Bounds.Width(), f.Bounds.Height()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("svg: frame bounds %+v have no extent", f.Bounds)
	}

	fmt.Fprintf(bw, `<svg xmlns="http://www.w3.org/2000/svg" `+
		`width="%gmm" height="%gmm" viewBox="%g %g %g %g">`+"\n",
		width, height, f.Bounds.Min.X, f.Bounds.Min.Y, width, height)

	writeMeta(bw, f.Meta)

	fmt.Fprintf(bw, `<g fill="none" stroke="black" stroke-width="%g" `+
		`stroke-linejoin="round" stroke-linecap="round">`+"\n", StrokeWidth)

	for _, p := range f.Paths {
		if len(p.Pts) < 2 {
			continue
		}
		bw.WriteString(`<path d="`)
		for i, pt := range p.Pts {
			verb := 'L'
			if i == 0 {
				verb = 'M'
			}
			fmt.Fprintf(bw, "%c%.3f %.3f", verb, pt.X, pt.Y)
			if i < len(p.Pts)-1 {
				bw.WriteByte(' ')
			}
		}
		if p.Closed {
			bw.WriteString("Z")
		}
		bw.WriteString("\"/>\n")
	}

	bw.WriteString("</g>\n</svg>\n")
	return bw.Flush()
}

// writeMeta emits the frame's metadata as an XML comment, so an output file
// records the exact parameters that produced it.
func writeMeta(w io.Writer, meta map[string]string) {
	if len(meta) == 0 {
		return
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Fprint(w, "<!-- adraw:")
	for _, k := range keys {
		fmt.Fprintf(w, " %s=%s", k, meta[k])
	}
	fmt.Fprint(w, " -->\n")
}

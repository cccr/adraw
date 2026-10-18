package cells

import (
	"github.com/cccr/adraw/pkg/geom"
	"github.com/cccr/adraw/pkg/patch"
)

// build assembles a preset's parameter list.
//
// Controls are included only when they mean something for that combination:
// merge is a Delaunay idea (Voronoi cells are already many-sided, and fusing
// them would make concave shapes the fills cannot take), and an angle only
// applies to the ruled fills. Showing a control that does nothing is worse than
// not showing it, because a UI gives no hint that dragging it is pointless.
// The spacing range is per-preset rather than shared because what counts as a
// sensible stroke gap depends entirely on the fill. Nested rings look right at
// a pen width apart; a stipple at the same gap is 367,000 dots that take three
// seconds to generate, plot for hours and come out as solid grey. Declaring an
// honest range per fill keeps every slider position worth visiting.
func build(tess geom.Tessellator, fill geom.Fill, gap string, spMin, spMax, spDef float64) *patch.ParamSet {
	ps := patch.NewParamSet().
		MM("cell", "Cell size",
			"Typical width of one cell. Smaller means more, finer cells.", 5, 60, 26).
		MM("spacing", gap,
			"Distance between strokes. Set it from your pen width.", spMin, spMax, spDef)

	if tess == geom.Delaunay {
		ps.Unit("merge", "Merge probability",
			"Chance of fusing neighbouring triangles into quads and pentagons. "+
				"0 leaves pure triangles.", 0, 1, 0.25)
	}
	if fill == geom.FillHatch || fill == geom.FillCrossHatch {
		ps.Count("angles", "Distinct angles",
			"How many directions the hatching may take. Fewer makes neighbouring "+
				"cells rhyme.", 1, 24, 6)
	}

	return ps.
		Unit("fill", "Fill probability",
			"Chance a cell is drawn at all. Below 1 leaves bare patches.", 0, 1, 1).
		MM("jitter", "Hand wobble",
			"Random displacement of every point, for a less mechanical line.", 0, 2, 0).
		MM("margin", "Page margin",
			"Blank border left around the drawing.", 0, 40, 10)
}

func init() {
	for _, p := range []*preset{
		{
			name: "contour", tess: geom.Delaunay, fill: geom.FillInset,
			desc: "Nested outlines stepping inward from each cell's edges, like a contour map.",
			ps:   build(geom.Delaunay, geom.FillInset, "Ring gap", 0.3, 5, 1.2),
		},
		{
			name: "cellwall", tess: geom.Voronoi, fill: geom.FillInset,
			desc: "Organic Voronoi cells filled with nested outlines, reading as thick cell walls.",
			ps:   build(geom.Voronoi, geom.FillInset, "Ring gap", 0.3, 5, 1.2),
		},
		{
			name: "vortex", tess: geom.Voronoi, fill: geom.FillSpiral,
			desc: "Spiralling vortices in organic Voronoi cells, rounder than tangle's triangles.",
			ps:   build(geom.Voronoi, geom.FillSpiral, "Ring gap", 0.3, 5, 1.2),
		},
		{
			name: "fan", tess: geom.Delaunay, fill: geom.FillFan,
			desc: "Rays from one corner of each cell, reading as shaded gradient wedges.",
			ps:   build(geom.Delaunay, geom.FillFan, "Ray gap", 0.8, 8, 2.5),
		},
		{
			name: "hatch", tess: geom.Delaunay, fill: geom.FillHatch,
			desc: "Each cell filled with parallel ruled lines at its own angle.",
			ps:   build(geom.Delaunay, geom.FillHatch, "Line gap", 0.4, 6, 1.5),
		},
		{
			name: "crosshatch", tess: geom.Delaunay, fill: geom.FillCrossHatch,
			desc: "Two sets of ruled lines crossing in each cell, for a denser tone.",
			ps:   build(geom.Delaunay, geom.FillCrossHatch, "Line gap", 0.8, 8, 2.5),
		},
		{
			name: "stipple", tess: geom.Voronoi, fill: geom.FillDots,
			desc: "Blue-noise dots stippled through organic cells.",
			ps:   build(geom.Voronoi, geom.FillDots, "Dot gap", 1.2, 8, 2.5),
		},
	} {
		patch.Register(p)
	}
}

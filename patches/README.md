# Writing a patch

A patch is a generator: it turns a seed and a set of numeric parameters into a
`drawing.Frame` of polylines. It never knows what will render it or what is
driving it.

## The contract

```go
type Patch interface {
    Name() string
    Params() []ParamSpec
    Generate(ctx context.Context, req Request) (drawing.Frame, error)
}
```

Three rules, all enforced by `patchtest.Verify`:

1. **Deterministic.** The same `Request` must always produce the same `Frame`.
   Seed your own `*rand.Rand` from `req.Seed`; never touch the global source.
2. **All configuration comes from `req.Values`.** This is what lets one control
   surface drive every patch. A patch that reads settings from anywhere else
   silently breaks the CLI, MIDI, CV, and web UIs at once.
3. **Every parameter is a `float64` in a declared `[Min, Max]`.** A MIDI CC maps
   in as `spec.FromNormalized(cc/127)`, a CV input as `FromNormalized(v/10)`.
   Declare an honest range and the patch is playable from anything.

## Steps

**1. Declare parameters** with the `ParamSet` builder:

```go
var params = patch.NewParamSet().
    MM("cell", "Cell size",
        "Typical width of one cell. Smaller means more, finer cells.",
        5, 60, 26).                                   // millimetres
    Unit("merge", "Merge probability",
        "Chance of fusing neighbouring cells.", 0, 1, 0.25).  // dimensionless
    Count("style", "Fill style",
        "0 spiral, 1 contour, 2 fan.", 0, 2, 0)       // integer-valued
```

**2. Implement the interface** and register in `init()`:

```go
func init() { patch.Register(New()) }

func (p *Mine) Name() string              { return "mine" }
func (p *Mine) Params() []patch.ParamSpec { return params.Specs() }

func (p *Mine) Description() string {
    return "One or two sentences on what this draws."
}
```

The description and the per-parameter `desc` strings are not decoration: they
are the only text any UI has to explain your patch. Write the parameter ones to
say which way the control goes ("smaller means more, finer cells"), since a
slider shows a number but not its consequence.

Integer `Count` parameters make good mode selectors — a fill style, a
tessellator. They stay `float64` in a declared range, so a MIDI CC switches mode
the way a modular synth switches waveform. See `tangle`'s `tess` and `style`.

**3. Build on `pkg/geom`** rather than rolling your own maths. What is already
there:

| Function | Use |
|---|---|
| `RectPolygon(w, h, margin)` | a default page |
| `Cells(border, opts, rng)` | border to convex cells in one call — start here |
| `PoissonDisk(border, opts, rng)` | points evenly spread, with a guaranteed minimum separation |
| `Tessellate(pts, opts, rng)` | convex cells covering the border exactly |
| `NewGrid(bounds, cell)` | spatial hash for nearest / within queries |
| `Spiral(poly, opts)` | nested rings that shrink and twist into a vortex |
| `Inset(poly, opts)` | concentric rings with no twist; contour-map look |
| `Hatch(poly, spacing, angle)` | parallel ruled lines clipped to a polygon |
| `CrossHatch`, `Fan`, `Dots` | further cell fills |
| `ApplyFill(poly, opts, rng)` | all of the above behind one selector |
| `VoronoiIn(border, opts, rng)` | organic cells instead of triangles |
| `ClipToConvex(subject, clip)` | trim a polygon to a convex region |
| `NewNoise(seed)`, `FBM` | simplex noise for fields and variation |
| `Contour(field, opts)` | isolines of a scalar field, stitched into long curves |
| `Streamlines(field, seeds, opts)` | curves following a vector field, kept apart |
| `Jitter(paths, amount, rnd)` | hand-drawn wobble |
| `Area`, `Centroid`, `IsConvex`, `Contains`, `InRadius`, `MinAngle` | polygon queries |

**If you find yourself writing general-purpose geometry inside a patch, it
belongs in `pkg/geom` instead** — then the next patch inherits it for free. That
is the whole reason `geom` is public rather than internal.

**4. Fall back to the default page** when no border is supplied, and record
`Meta` so any output can be reproduced from the file it was written to:

```go
border := geom.Polygon(req.Border)
if len(border) < 3 {
    w, h := req.PageOr(297, 210)      // honour a requested page size
    border = geom.RectPolygon(w, h, margin)
}
```

Route your default sheet through `req.PageOr` rather than reading your own
constants, and set `Frame.Bounds` from the same numbers. Page size is a request
field rather than a parameter, so `patchtest` checks this separately — a patch
that keeps its own hard-coded A4 will fail the `page_size` case.

**5. Register it** by adding one line to `patches/patches.go`:

```go
_ "github.com/cccr/adraw/patches/mine"
```

That is the only shared file you touch. Both the CLI and the web UI discover the
patch and build its controls from `Params()` automatically — there is no switch
statement to update and no frontend code to write. Run `adraw serve` and your
patch is there with working sliders.

**6. Test it** in three lines:

```go
func TestConformance(t *testing.T) {
    patchtest.Verify(t, New())
}
```

If your patch divides space into regions, also implement the optional
`patch.Regioner` interface so its cells can seed another patch. Share one setup
helper between `Generate` and `Regions` so the two can never disagree about what
the cells are.

`Verify` checks determinism, seed sensitivity, that points stay within the
declared bounds, that no coordinate is NaN or infinite, that parameter defaults
sit inside their ranges, and that `Generate` survives the extreme of every
declared range — the values a control surface will actually send. Add your own
tests for what is specific to your patch.

## Worked examples

- **`tangle/`** — the full pipeline: sample, tessellate, fill each cell, with
  the tessellator and fill exposed as parameters. Read this one first.
- **`topo/`** and **`flow/`** — field-based rather than cell-based. Read these
  if your idea is not about subdividing space at all.
- **`cells/`** — seven presets sharing one generic implementation. Read this if
  your idea is a variation on cells rather than a new algorithm: you may only
  need a handful of lines in `presets.go`.

**Preset or package?** If your idea is an existing patch with different
constants, add a preset. If it is a genuinely different algorithm, give it its
own directory.

Whichever you write, declare parameter ranges that are honest for *your* fill
rather than copying another patch's. A stipple at the ring-gap default is
367,000 dots that take seconds to generate and plot for hours — every slider
position should be worth visiting.

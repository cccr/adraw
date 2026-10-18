# Plan 002 — patch library expansion + infrastructure

> Plan 001 (foundation + `tangle`) is complete and shipped. This plan supersedes it.
> Its output lives at `plans/001-foundation-and-tangle.md`.

## Context

`adraw` has three working layers, two patches, two renderers, a CLI and a web UI.
But a survey of the current state turned up a problem the green test suite does
not show:

**Both patches are the same algorithm wearing different clothes.** `tangle` and
`hatch` are both *sample → tessellate → fill each cell*, differing only in the
fill function. The foundation has been exercised twice with one structural idea,
so its generality is assumed rather than demonstrated, and the visual range of
the tool is much narrower than the patch count suggests.

Two concrete gaps came out of the same survey:

- **`Request.Border` is dead code.** No UI can set it (`grep` over `cmd/`, `ui/`,
  `internal/` finds nothing), so the "feed one patch into another" claim in the
  README is unproven.
- **814 pen lifts per drawing** (814 paths averaging 3.9 points). A plotter
  spends most of its time travelling with the pen up, and every denser patch
  makes it worse.

This plan adds patches that each break the mould in a *different* direction —
so each one stress-tests a different assumption — and closes both gaps.

### Scope warning

This is a large body of work: three new patches, a second tessellator, four new
fills, two new geometry primitives, and two infrastructure pieces. It is staged
so **every stage ships something usable on its own**. Stages 3 and 4 are
naturally separate sessions; do not treat this as one sitting.

---

## Stage 1 — Infrastructure

Everything downstream benefits, and neither piece depends on the new patches.

### 1a. Travel optimization — `pkg/optimize`

```go
package optimize
func Reorder(f drawing.Frame) drawing.Frame   // also records before/after in Meta
```

Greedy nearest-neighbour from the current pen position. Two details that matter
more than the ordering heuristic itself:

- **Paths may be reversed.** An open path can be entered from either end, so
  both endpoints are candidates. This alone typically beats ordering-only.
- **Closed paths may start at any vertex.** `tangle` output is almost entirely
  closed rings, so this is the dominant win for the flagship patch.

Use the spatial grid from 1c for the nearest lookup — O(n²) is fine at 800 paths
but the flow and growth patches will push path counts far higher.

**Where it runs:** the CLI's `render` command and `/api/export`. Deliberately
**not** `/api/render` — the canvas preview does not care about path order, and
paying for it on every slider tick would spoil the live feel that makes the web
UI worth having.

- **Check:** total travel distance drops substantially on `tangle` (measure
  before/after, expect a large multiple); path count and every path's point set
  are unchanged — this pass must reorder and reverse only, never redraw.

### 1b. Border input — making `Request.Border` live

The field exists and `tangle` already honours it (`tangle_test.go` covers a
custom border); nothing can *supply* one. Wire it through both UIs:

- CLI: `--border <file>` accepting a simple JSON point array
- API: `border=x0,y0,x1,y1,…` on `/api/render` and `/api/export`

That makes "draw into an arbitrary shape" work immediately, and is the
prerequisite for true patch-to-patch composition.

**On full chaining:** composing patch A's cells as patch B's borders needs
patches to expose their regions, not just their strokes. The clean shape is an
*optional* interface, so the core `Patch` contract does not change:

```go
// Implemented by patches that divide space. Optional.
type Regioner interface {
    Regions(ctx context.Context, req Request) ([]geom.Polygon, error)
}
```

`tangle` and `hatch` both implement it trivially (they already compute cells).
**Build the interface and have the cell patches satisfy it, but stop there** — a
`compose` patch has to solve parameter namespacing for two patches through one
flat `Values` map, and designing that before a second implementor exists is
guessing. Revisit once Stage 3 or 4 lands a patch that wants it.

- **Check:** a non-rectangular border drawn from the CLI and from a URL; a
  `Regioner` round-trip test asserting the regions tile the border.

### 1c. Extract `geom.Grid`

`PoissonDisk` already contains a background spatial grid (`pkg/geom/sample.go`).
Differential growth needs the same structure for self-avoidance and travel
optimization needs it for nearest-path lookup. Extract it rather than writing it
a third time — this is exactly the "if you write it twice it belongs in geom"
rule from `patches/README.md`.

- **Check:** `PoissonDisk` behaviour is bit-identical after the refactor (its
  existing determinism test already pins this).

---

## Stage 2 — The cheap multiplier

Most distinct looks per line of code. No new structural idea, but it multiplies
what already exists.

### 2a. `geom.VoronoiCells`

The Voronoi diagram is the dual of the Delaunay triangulation already being
computed: cell corners are triangle circumcenters, walked via the `Halfedges`
array `Tessellate` already uses.

**One real complication, worth knowing before starting.** `PoissonDisk` seeds
the border densely on purpose — that is what makes Delaunay tile the border
exactly. For Voronoi it backfires: every boundary point owns a thin sliver cell
hugging the edge, which looks terrible. Voronoi therefore needs interior-only
sampling plus explicit clipping of each cell to the border. Add a
`SkipBoundary` option to `SampleOptions`, and clip with Sutherland–Hodgman
(valid here because v1 borders are convex).

- **Check:** cells tile the border (areas sum to the border area); every cell is
  convex; no cell is a sliver against the edge — the specific failure this
  design avoids.

### 2b. New fills in `pkg/geom/fill.go`

Alongside the existing `Spiral` and `Hatch`:

| Fill | Look |
|---|---|
| `Inset` | concentric rings with no twist — the topographic/contour look |
| `Fan` | rays from one vertex to the opposite edge |
| `CrossHatch` | two hatch passes at an angle to each other |
| `Dots` | stippled interior, density as a parameter |

`Inset` is `Spiral` with the rotation removed, so it should share the
ring-stepping code rather than copy it.

### 2c. Expose tessellator and fill as parameters

Rather than a third near-identical patch, extract the shared *sample →
tessellate* pipeline (currently duplicated between `patches/tangle/tangle.go`
and `patches/hatch/hatch.go`) into one helper, and give both patches:

- `tess` — Count param: 0 = Delaunay, 1 = Voronoi
- `fill` — Count param selecting among the fills that suit that patch

Integer selector parameters fit the existing model exactly: they are still
`float64` in a declared range, so a MIDI CC or CV input switches fill style the
way a modular synth switches waveform. Two tessellators × the fill set gives a
large jump in range for a small amount of code.

- **Check:** `patchtest.Verify` still passes for both patches — note its
  *parameter extremes* check will now exercise every tessellator/fill
  combination automatically, which is precisely the coverage this needs.

---

## Stage 3 — Flow field

The biggest visual departure: no cells at all. Particles traced through a noise
field become long flowing streamlines.

### 3a. `geom.Noise` — 2D simplex

Prefer simplex over Perlin: Perlin's axis-aligned directional artifacts are
mild in a texture but become visible, regular stripes once you trace streamlines
through the field. Add fBm octave summing alongside it. Roughly 150 lines from
the standard reference, no dependency.

### 3b. `geom.Streamline`

Trace a point through a vector field with fixed-step RK2 integration, stopping
on leaving the border, on exceeding a length cap, or on approaching an existing
line (using `geom.Grid` from 1c — this is what stops the drawing turning into
mush).

### 3c. `patches/flow`

Seed points, trace, keep what survives. Parameters: field scale, octaves, step
length, line separation, max length, seeding density.

This patch produces *few paths with many points each* — the exact inverse of
`tangle`'s profile, which is why it is worth building: it tests the `Frame`
contract and both renderers at an extreme nothing has hit yet.

- **Check:** `patchtest.Verify`; streamlines stay inside the border; no
  streamline crosses another closer than the separation parameter.

---

## Stage 4 — Truchet and growth

### 4a. `patches/truchet`

A grid of tiles, each carrying arcs that meet at edge midpoints, so strokes join
across tile boundaries into long wandering meanders. Closest of all of these to
the original Zentangle reference, and very plotter-friendly.

Needs `geom.Arc` (flatten a circular arc to a polyline at a given tolerance) —
small, and reusable by any later curve work. Joining arcs across tiles into
single continuous paths is what makes this worth doing; emitting each arc as its
own path would waste the whole benefit.

### 4b. `patches/growth` — differential growth

A closed curve that lengthens, repels itself, and subdivides, convoluting into
coral and brain-fold forms. Uses `geom.Grid` for self-avoidance.

The slowest patch here by some margin — expect it to need an iteration cap tuned
against the live-preview budget, and check whether it stays inside the web UI's
interactive feel or needs its own handling.

- **Check:** both pass `patchtest.Verify`; growth terminates within its cap at
  every parameter extreme (the likeliest place for a runaway loop).

---

## Files

| Path | Change |
|---|---|
| `pkg/optimize/optimize.go` | new — travel reordering |
| `pkg/geom/grid.go` | new — extracted from `sample.go` |
| `pkg/geom/voronoi.go` | new — dual of the existing Delaunay |
| `pkg/geom/noise.go`, `streamline.go`, `arc.go` | new primitives |
| `pkg/geom/fill.go` | add `Inset`, `Fan`, `CrossHatch`, `Dots` |
| `pkg/geom/sample.go` | add `SkipBoundary`; extract the grid |
| `pkg/patch/patch.go` | add optional `Regioner` interface |
| `patches/cells.go` | new — shared sample/tessellate pipeline |
| `patches/{tangle,hatch}/` | use the shared pipeline; add `tess` + `fill` params |
| `patches/{flow,truchet,growth}/` | new patches |
| `cmd/adraw/main.go`, `ui/web/api.go` | `--border` / `border=`; run the optimizer on export |
| `patches/patches.go` | one line per new patch |

---

## Verification

Per-stage checks are inline above. Throughout:

```bash
make check                     # vet + full suite, including patchtest for every patch
make serve                     # every new patch appears with working sliders, no UI code
```

The web UI is the fastest judge of whether a patch is any good — it needs no
changes for any of this, and that staying true is itself the test that the
`ParamSpec` design holds up under six patches rather than two.

Two measurements worth recording rather than assuming:

- **Travel optimization:** total pen-up distance before and after, on `tangle`
  at default settings. State the real number.
- **Generation time per patch** at its densest settings, against the 1–8 ms the
  current patches manage. If `growth` or `flow` blows past the live-preview
  budget, that is a finding to report, not something to quietly absorb.

## Out of scope

- **A `compose` patch.** `Regioner` lands in Stage 1; the patch that uses it
  waits for a real second implementor, per 1b.
- **Non-convex borders.** Stage 2's Voronoi clipping assumes convex, as v1 does.
- **Real-time/animation.** Unchanged from Plan 001.

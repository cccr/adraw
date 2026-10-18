# Plan 001 — `adraw` foundation + first patch (`tangle`)

## Context

`abstract_drawing` is a greenfield Go project (Go 1.22.5, empty except the README). The goal is a
three-layer system for controllable generated graphics: **UI → Engine (patches) → Renderer**.

`tangle` is the **first of many patches**. So this plan is 80% about building a foundation that
makes patch #2 through #20 cheap to write, and 20% about the first patch — which exists mainly to
prove the foundation is the right shape. The reference image (nested rotating polygons filling a
tessellated rectangle — the Zentangle "spiral triangles" pattern) is the target for `tangle`.

The critical constraint from the user, in their words: *"important that we won't stay with small
areas — consider points pseudo random equally spread in initial border."* Every tessellation
decision below is driven by that.

**Decisions already made:**

| Decision | Choice |
|---|---|
| Module path | `github.com/cccr/adraw` |
| Layout | `pkg/` public API, `internal/` private |
| Tessellation | Poisson-disk (blue noise) + Delaunay |
| First renderer | SVG, world units in **millimetres** |

### One revision to the agreed layout

The agreed layout put geometry in `internal/geom`. **With many patches coming, that is wrong.**
Poisson sampling, tessellation, polygon operations, and spiral fills are the *shared toolkit patch
authors build with* — not implementation details of one patch. `internal/` would bar any
out-of-repo patch from importing them and would push each new patch toward copy-pasting geometry.

So: **`pkg/geom` is public**, and `internal/` holds only genuinely private things (CLI table
formatting). The `pkg/` vs `internal/` split is preserved; the line just falls in the right place.

---

## Target structure

```
plans/
  001-foundation-and-tangle.md   this document

pkg/                     ← stable public API: everything a patch author touches
  drawing/  frame.go     Vec2, Path, Frame, Rect — the engine↔renderer contract
  patch/    patch.go     Patch iface, Request, registry
            param.go     ParamSpec, Values, ParamSet builder
            patchtest/   shared conformance suite every patch runs
  render/   render.go    Renderer iface, registry
  geom/     vec.go       Vec2 arithmetic, lerp, rotate, perp
            poly.go      area, centroid, perimeter, convexity, contains, bbox, inset
            sample.go    Bridson blue-noise (Poisson-disk) sampling
            tess.go      Delaunay wrapper, convex cell merging
            fill.go      spiral/nested fills, hatching  ← grows with each patch
internal/
  cli/      table.go     flag parsing + table output, private to cmd/
patches/
  tangle/   tangle.go    patch #1
            tangle_test.go
  patches.go             blank imports — the single registration point
renderers/
  svg/      svg.go
  gcode/    gcode.go
  renderers.go           blank imports
cmd/adraw/  main.go      CLI: list / params / render
```

### What makes patch #2 cheap

Five deliberate choices, each paying off per-patch rather than once:

1. **`pkg/geom` is a growing toolkit.** Patch #2 reuses sampling, tessellation, and polygon ops on
   day one. New primitives land in `geom`, not in the patch, so patch #3 inherits them.
2. **`patch.ParamSet` builder** collapses parameter declaration to a few readable lines, so the
   boilerplate tax per patch stays near zero.
3. **`patchtest.Verify(t, p)`** — a shared conformance suite. Each new patch gets real test coverage
   from a three-line test file: determinism, bounds, NaN-freedom, parameter sanity.
4. **Self-registration via `init()` + one blank-import file** (`patches/patches.go`). Adding a patch
   is: one new directory, one line in that file. No switch statement, no CLI change, no UI change.
5. **`ParamSpec` drives every UI.** A patch declares its parameters once and is immediately
   controllable from the CLI, and later from MIDI/CV/web, with no UI code written per patch.

---

## Step-by-step

Each step is independently verifiable. Do not start a step before its predecessor's check passes.

### Step 1 — Repo skeleton and this plan

- `mkdir plans` and save this document as `plans/001-foundation-and-tangle.md`.
  (Plan mode restricts edits to the plan file, so this is the first implementation action.)
- `go mod init github.com/cccr/adraw`
- `.gitignore`: `/adraw`, `*.svg`, `*.nc`, `/out/`
- **Check:** `go build ./...` succeeds on an empty module.

### Step 2 — `pkg/drawing`: the data contract

Everything the engine produces is **polylines in millimetres** — the lowest common denominator
serving all three renderer categories the user named: image (stroke the paths), G-code (pen-up
travel, pen-down along each path), and input to another patch (a `Path` *is* a border polygon).

```go
type Vec2 struct{ X, Y float64 }        // millimetres

type Path struct {
    Pts    []Vec2
    Closed bool
}

type Rect struct{ Min, Max Vec2 }

type Frame struct {
    Bounds Rect
    Paths  []Path
    Meta   map[string]string   // patch name, seed, resolved params — reproducibility
}
```

No colour, no stroke width: a pen plotter has one pen, and renderers that want style apply their
own. **Resist growing this struct** — it is the contract that is expensive to change once several
patches and renderers depend on it.

- **Check:** compiles; `Rect` helpers (`Width`, `Height`, `Union`) unit-tested.

### Step 3 — `pkg/patch`: interface, parameters, registry

```go
type Patch interface {
    Name() string
    Params() []ParamSpec
    Generate(ctx context.Context, req Request) (drawing.Frame, error)
}

type Request struct {
    Seed   int64
    Border []drawing.Vec2   // empty ⇒ patch default (A4 landscape)
    Values Values
}

type Values map[string]float64

type ParamSpec struct {
    Key, Label string
    Min, Max   float64
    Default    float64
    Step       float64   // 0 ⇒ continuous
    Unit       string    // "mm", "", "×"
}

func (s ParamSpec) FromNormalized(n float64) float64  // n∈[0,1] → [Min,Max]
func (s ParamSpec) Normalize(v float64) float64
func (v Values) Get(s ParamSpec) float64              // clamped; Default if absent
```

**The load-bearing design decision:** every parameter is a `float64` inside a declared `[Min,Max]`.
That is what makes a MIDI CC (0–127) or CV input (0–10 V) map onto *any* patch mechanically —
`spec.FromNormalized(cc/127)` — with no per-patch UI code, and it is why this holds for patch #20
as much as patch #1. A web UI generates its sliders from the same list. **Patches must read all
configuration from `Values`**; the moment one reads config from elsewhere, that property is lost for
every UI.

`Seed` and `Border` are dedicated `Request` fields, not `Values` entries: an `int64` seed does not
survive a `float64` round-trip cleanly, and a border is a polygon, not a scalar.

Add the `ParamSet` builder so declaring parameters stays terse:

```go
var params = patch.NewParamSet().
    MM("cell",    "Cell size",   5, 60, 18).
    MM("spacing", "Ring gap",    0.3, 5, 1.2).
    Unit("merge", "Merge prob.", 0, 1, 0.15)
```

**Registry:** `patch.Register(p Patch)` from each patch's `init()`. Do **not** use Go's `plugin`
package — it is effectively unusable on macOS, demands exact toolchain matching, and cannot be
cross-compiled. Compile-time registration is the correct "pluggable" mechanism in Go.

- **Check:** unit tests — `Get` clamps out-of-range input and returns `Default` when absent;
  `FromNormalized(0)`/`(1)` hit exactly `Min`/`Max`; duplicate `Register` panics loudly.

### Step 4 — `pkg/render`: renderer interface

```go
type Renderer interface {
    Name() string
    Ext() string                                 // "svg", "nc"
    Render(w io.Writer, f drawing.Frame) error
}
```

Same registry pattern. `io.Writer` covers file, stdout, and network with no further abstraction.

- **Check:** compiles; registry test mirrors Step 3's.

### Step 5 — `pkg/geom`: vectors and polygons

`vec.go`: `Add`, `Sub`, `Scale`, `Lerp`, `Len`, `Perp`, `Rotate`, `Dist`.
`poly.go`: `Area` (shoelace, signed), `Centroid`, `Perimeter`, `IsConvex`, `Contains`, `BBox`,
`Rect(w,h,margin)`.

- **Check:** `Area` of a unit square is 1 and sign flips with winding; `IsConvex` correctly rejects
  a chevron; `Contains` is right on edges and corners.

### Step 6 — `pkg/geom/sample.go`: blue-noise sampling

Bridson's algorithm: grid-accelerated O(n), 30 candidate attempts per active point, background grid
of cell size `r/√2`.

**The step that makes or breaks the "no small areas" requirement:** before sampling the interior,
walk the border polygon and place points along its edges at spacing ≈ `r`, corners included. Insert
these into the Poisson grid as *already-placed* points, so interior samples respect the minimum
distance from them too.

Two consequences, both essential:
- The convex hull of the point set becomes exactly the border, so Delaunay tiles the border with
  **no clipping step at all**.
- No interior point can land just barely inside an edge — the classic source of edge slivers.

RNG is `math/rand.New(rand.NewSource(seed))`, never the global source.

- **Check:** no two points closer than `r`; every point inside the border; boundary points present
  at the expected spacing; identical output for identical seed.

### Step 7 — `pkg/geom/tess.go`: Delaunay + cell merging

Add `github.com/fogleman/delaunay` (MIT, zero dependencies, a Go port of Delaunator). Verify the
exact API on implementation; expected shape:

```go
tri, err := delaunay.Triangulate(pts)
// tri.Triangles []int  — flat, 3 vertex indices per triangle
// tri.Halfedges []int  — adjacency; -1 on the hull
```

Delaunay is the right choice specifically because it **maximizes the minimum angle** over all
triangulations of the point set. Combined with blue-noise input, sliver triangles are not merely
unlikely — they are structurally excluded. The user's constraint is satisfied by construction rather
than by filtering afterwards.

Guard: Delaunator errors on fully-collinear input — with interior points present this cannot happen,
but return the error rather than panicking.

**Cell merging** gives the *"triangle, or rectangle or pentagon"* variety the user asked for, since
Delaunay alone yields only triangles. Using `tri.Halfedges`:

- Walk half-edges in seeded-shuffled order.
- With probability `merge`, fuse the two triangles sharing that edge into a quad — **only if the
  result is convex** (all four cross-products share a sign). Convexity is required by the fill.
- Mark both consumed so nothing merges twice.
- An optional second pass fuses a quad with an adjacent triangle into a pentagon.

`merge = 0` ⇒ pure triangles, matching the reference image's dominant look.

- **Check:** Σ cell areas ≈ border area within 1e-6 (proves full coverage: no gaps, no overlaps);
  no triangle's minimum angle below a threshold (proves the no-slivers claim rather than assuming
  it); every merged cell passes `IsConvex`.

### Step 8 — `pkg/geom/fill.go`: the spiral fill

The visual signature of the reference image, and **not** a plain inward offset. Each ring is
*inscribed in the previous by walking a fixed fraction along each edge*, so the polygon both shrinks
*and rotates* — producing the vortex.

For a convex polygon `v[0..n-1]`:

```go
ring := v
for k := 0; k < maxRings; k++ {
    emit(ring)                                  // closed drawing.Path
    t := clamp(spacing/h(ring), 0.02, 0.45)
    next := make([]Vec2, n)
    for i := range ring {
        j := ((i+dir)%n + n) % n                // dir ∈ {+1,-1}
        next[i] = Lerp(ring[i], ring[j], t)
    }
    ring = next
    if Area(ring) < minArea { break }
}
```

Two details that matter:

- **`dir` is randomized per cell.** This produces the mix of left- and right-twisting vortexes
  visible in the reference image. A fixed direction looks conspicuously mechanical.
- **`t` is derived per ring, not fixed.** With `h(ring) = 2·Area/Perimeter` (the inradius for a
  tangential polygon), `t = spacing/h` holds the perpendicular gap between rings ≈ `spacing`
  millimetres — so `spacing` is a *physical* quantity set from pen width. As the ring shrinks, `h`
  shrinks, `t` rises, and once it clamps at `0.45` the rings go geometric and converge fast — which
  is exactly the dense core the reference image shows. **This formula is a heuristic, not an
  identity**; expect to tune the clamp bounds by eye in Step 11.

Terminate on `Area < minArea` or `maxRings` (safety bound ~200).

- **Check:** ring areas strictly decreasing; each ring contained in its parent; terminates on
  degenerate input (near-zero-area and near-collinear triangles) rather than spinning.

### Step 9 — `patches/tangle`

Compose Steps 5–8. The patch itself should be thin — sampling, tessellation, and filling all live in
`geom`; `tangle` declares parameters and wires the stages together. **If this file grows large,
something belongs in `geom` instead.** That is the test of whether the foundation works.

| Key | Range | Default | Meaning |
|---|---|---|---|
| `cell` | 5–60 mm | 18 | Poisson minimum distance — the dominant size control |
| `spacing` | 0.3–5 mm | 1.2 | gap between nested rings; set from pen width |
| `merge` | 0–1 | 0.15 | probability of merging triangles into quads/pentagons |
| `fill` | 0–1 | 0.95 | probability a cell is filled at all — a few blanks read as hand-drawn |
| `jitter` | 0–2 mm | 0 | per-point random offset; a touch of marker wobble |
| `margin` | 0–40 mm | 10 | inset from the page edge |

Populate `Frame.Meta` with patch name, seed, and resolved parameter values so any output can be
reproduced from the file itself.

- **Check:** `patchtest.Verify(t, tangle.New())` passes.

### Step 10 — `pkg/patch/patchtest`: shared conformance suite

Write this *now*, not later — it is what makes every subsequent patch cheap to trust.
`patchtest.Verify(t, p)` asserts:

- **Determinism:** same `Request` ⇒ byte-identical `Frame`.
- **Seed sensitivity:** different seed ⇒ different output (catches an ignored seed).
- **Bounds:** every point lies within `Frame.Bounds`.
- **Sanity:** no NaN or Inf coordinates; no empty or single-point paths.
- **Parameter hygiene:** `Default` lies within `[Min,Max]`; keys are unique and non-empty.
- **Range robustness:** `Generate` succeeds at every parameter's `Min` and `Max` — the extremes MIDI
  and CV will actually send.

- **Check:** temporarily break `tangle` (ignore the seed) and confirm the suite fails.

### Step 11 — `renderers/svg` + `cmd/adraw` → first visual output

SVG: `width="297mm" height="210mm" viewBox="0 0 297 210"` so on-screen millimetres equal plotted
millimetres. One `<path>` per `drawing.Path` (`M`/`L`, `Z` when `Closed`), under a single `<g>`
carrying `fill="none" stroke="black" stroke-width="0.3"` — small files, trivially restyled after.

CLI:

```
adraw list                       # registered patches and renderers
adraw params tangle              # ParamSpecs as a table, with ranges and defaults
adraw render --patch tangle --seed 42 \
             --set cell=14 --set spacing=0.9 --set merge=0.3 \
             --out out.svg
```

`--set key=value` is repeatable and validated against the patch's `ParamSpec` list, so **the CLI
needs no per-patch knowledge** and never changes as patches are added. Renderer is inferred from the
output file extension.

- **Check:** the visual loop below. **Expect to tune Step 8's clamp bounds and the `spacing` default
  here** — this is the step where the thing either looks right or doesn't.

### Step 12 — `renderers/gcode`

Cheap (~80 lines) now that the data format was chosen with it in mind: `G21 G90` preamble, then per
path — pen-up (`G0 Z<up>`), rapid to first point, pen-down (`G1 Z<down> F<plunge>`), `G1` along the
points, closing to the start when `Closed`. Configurable Z heights and feedrate.

Explicit non-goal: **travel-order optimization.** Paths come out in generation order, which is not
plot-efficient. That belongs in a later `Frame → Frame` optimizer pass, not smuggled into a renderer.

- **Check:** output parses in a G-code previewer; pen-up/pen-down pairs balance.

### Step 13 — Documentation for patch authors

- `patches/README.md`: how to add a patch — the interface, the `ParamSet` builder, registering in
  `patches/patches.go`, running `patchtest.Verify`. Use `tangle` as the worked example.
- Rewrite `README.md`: replace the speculative structure with the real one; document the three
  layers as built.

- **Check:** follow your own `patches/README.md` to stub a trivial second patch (say, plain
  hatching) and confirm it takes only a few minutes. **If it doesn't, the foundation needs fixing
  now** — before there are many patches depending on its shape.

---

## Out of scope for this pass

Called out so they are choices, not oversights:

- **Non-convex / SVG-path borders.** v1 accepts a convex polygon; the rectangle default covers the
  reference image. Non-convex needs constrained Delaunay or a Greiner–Hormann clip.
- **MIDI / CV / web UI.** `ParamSpec` exists so these are purely additive; building one now would be
  guessing at an interface with no patches to drive.
- **Real-time / streaming.** `Generate` returns a complete `Frame`. Animation needs a different
  signature and there is no evidence yet which one.
- **Patch chaining.** The data format supports it (a `Path` is a border); no wiring until a second
  patch exists to chain to.

---

## Verification

Per-step checks are listed inline above. End to end:

```bash
go vet ./... && go test ./...
go build -o adraw ./cmd/adraw
./adraw list
./adraw params tangle
./adraw render --patch tangle --seed 1 --out /tmp/a.svg
./adraw render --patch tangle --seed 2 --set cell=10 --out /tmp/b.svg
./adraw render --patch tangle --seed 3 --set merge=0.5 --set spacing=0.6 --out /tmp/c.svg
open /tmp/a.svg /tmp/b.svg /tmp/c.svg
```

**The tests cannot tell you whether it looks like the reference image — only your eye can.** Compare
against it: cells vary in size but never go tiny; roughly half the vortexes twist each way; ring
density rises toward each cell's core. Tuning in Step 11 is expected work, not a sign of a mistake.

package geom

import (
	"math"
	"math/rand"
	"testing"
)

func voronoiRect(t *testing.T, cellSize float64, seed int64) (Polygon, []Polygon) {
	t.Helper()
	border := RectPolygon(297, 210, 0)
	cells, err := VoronoiIn(border, CellOptions{CellSize: cellSize},
		rand.New(rand.NewSource(seed)))
	if err != nil {
		t.Fatalf("VoronoiIn: %v", err)
	}
	return border, cells
}

func TestVoronoiCoversBorder(t *testing.T) {
	border, cells := voronoiRect(t, 16, 1)
	var sum float64
	for _, c := range cells {
		sum += Area(c)
	}
	// Clipping is exact, so coverage should be near-perfect. Allow a whisker
	// for floating-point accumulation over a few hundred clipped polygons.
	want := Area(border)
	if math.Abs(sum-want)/want > 1e-6 {
		t.Errorf("cells cover %.3f of %.3f (%.4f%% off)", sum, want,
			math.Abs(sum-want)/want*100)
	}
}

func TestVoronoiCellsAreConvex(t *testing.T) {
	_, cells := voronoiRect(t, 16, 2)
	if len(cells) < 50 {
		t.Fatalf("only %d cells", len(cells))
	}
	for i, c := range cells {
		if !IsConvex(c) {
			t.Errorf("cell %d is not convex: %v", i, c)
		}
	}
}

func TestVoronoiHasNoEdgeSlivers(t *testing.T) {
	// The specific failure that SkipBoundary exists to prevent: sampling the
	// border would give every boundary point a paper-thin cell along the edge.
	const cellSize = 16
	_, cells := voronoiRect(t, cellSize, 3)

	ideal := cellSize * cellSize * math.Sqrt(3) / 2
	var tiny int
	for _, c := range cells {
		if Area(c) < 0.1*ideal {
			tiny++
		}
	}
	if tiny > 0 {
		t.Errorf("%d of %d cells are slivers; boundary seeding has leaked back in",
			tiny, len(cells))
	}
}

func TestVoronoiCellCountIsSane(t *testing.T) {
	_, cells := voronoiRect(t, 16, 4)
	sides := map[int]int{}
	for _, c := range cells {
		sides[len(c)]++
	}
	t.Logf("cell sides: %v", sides)
	// Voronoi cells of blue noise average six sides; a distribution centred on
	// triangles would mean the dual was built wrong.
	var total, n int
	for s, count := range sides {
		total += s * count
		n += count
	}
	if avg := float64(total) / float64(n); avg < 4.5 || avg > 7 {
		t.Errorf("average cell has %.1f sides, want near 6", avg)
	}
}

func TestVoronoiDeterministic(t *testing.T) {
	_, a := voronoiRect(t, 20, 9)
	_, b := voronoiRect(t, 20, 9)
	if len(a) != len(b) {
		t.Fatalf("same seed gave %d then %d cells", len(a), len(b))
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			t.Fatalf("cell %d differs between identical seeds", i)
		}
	}
}

func TestVoronoiRejectsBadInput(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	if _, err := VoronoiCells(RectPolygon(100, 100, 0), []Vec2{{1, 1}}); err == nil {
		t.Error("too few points should error")
	}
	if _, err := VoronoiIn(RectPolygon(50, 50, 0), CellOptions{CellSize: 500}, rng); err == nil {
		t.Error("an oversized cell size should error")
	}
}

func TestClipToConvex(t *testing.T) {
	clip := RectPolygon(10, 10, 0)

	// Fully inside: unchanged area.
	inside := Polygon{{2, 2}, {8, 2}, {8, 8}, {2, 8}}
	if got := Area(ClipToConvex(inside, clip)); math.Abs(got-36) > 1e-9 {
		t.Errorf("inside polygon area = %v, want 36", got)
	}
	// Straddling: trimmed to the overlap.
	straddle := Polygon{{5, 5}, {15, 5}, {15, 15}, {5, 15}}
	if got := Area(ClipToConvex(straddle, clip)); math.Abs(got-25) > 1e-9 {
		t.Errorf("straddling polygon area = %v, want 25", got)
	}
	// Entirely outside: nothing survives.
	if got := ClipToConvex(Polygon{{20, 20}, {30, 20}, {30, 30}}, clip); got != nil {
		t.Errorf("outside polygon should clip to nil, got %v", got)
	}
	// Works regardless of clip winding.
	reversed := Polygon{clip[3], clip[2], clip[1], clip[0]}
	if got := Area(ClipToConvex(straddle, reversed)); math.Abs(got-25) > 1e-9 {
		t.Errorf("clockwise clip gave area %v, want 25", got)
	}
}

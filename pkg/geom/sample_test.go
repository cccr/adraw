package geom

import (
	"math"
	"math/rand"
	"testing"
)

func TestPoissonDiskMinimumDistance(t *testing.T) {
	const r = 12
	border := RectPolygon(297, 210, 10)
	pts := PoissonDisk(border, SampleOptions{MinDist: r}, rand.New(rand.NewSource(1)))

	if len(pts) < 50 {
		t.Fatalf("got only %d points, expected a dense fill", len(pts))
	}
	// The whole point of blue noise: no two points closer than r. A violation
	// here means the tessellation can produce slivers.
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			if d := Dist(pts[i], pts[j]); d < r-1e-9 {
				t.Fatalf("points %d and %d are %v apart, want >= %v", i, j, d, r)
			}
		}
	}
}

func TestPoissonDiskStaysInsideBorder(t *testing.T) {
	border := RectPolygon(100, 80, 5)
	pts := PoissonDisk(border, SampleOptions{MinDist: 8}, rand.New(rand.NewSource(7)))
	for i, p := range pts {
		if !Contains(border, p) {
			t.Errorf("point %d at %+v is outside the border", i, p)
		}
	}
}

func TestPoissonDiskSeedsBoundary(t *testing.T) {
	const r = 10
	border := RectPolygon(100, 100, 0)
	pts := PoissonDisk(border, SampleOptions{MinDist: r}, rand.New(rand.NewSource(3)))

	// Every corner must be present, otherwise the convex hull is not the
	// border and Delaunay would not tile it exactly.
	for _, corner := range border {
		found := false
		for _, p := range pts {
			if Dist(p, corner) < 1e-9 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("corner %+v was not seeded", corner)
		}
	}

	var onEdge int
	for _, p := range pts {
		if OnBoundary(border, p, 1e-9) {
			onEdge++
		}
	}
	// Four edges of length 100 at spacing 10 give 10 points each.
	if want := 40; onEdge != want {
		t.Errorf("got %d boundary points, want %d", onEdge, want)
	}
}

func TestPoissonDiskDeterministic(t *testing.T) {
	border := RectPolygon(200, 150, 10)
	run := func() []Vec2 {
		return PoissonDisk(border, SampleOptions{MinDist: 14}, rand.New(rand.NewSource(42)))
	}
	a, b := run(), run()
	if len(a) != len(b) {
		t.Fatalf("same seed gave %d then %d points", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("point %d differs between identical seeds: %+v vs %+v", i, a[i], b[i])
		}
	}

	c := PoissonDisk(border, SampleOptions{MinDist: 14}, rand.New(rand.NewSource(43)))
	if len(c) == len(a) && pointsEqual(a, c) {
		t.Error("a different seed produced identical output")
	}
}

func pointsEqual(a, b []Vec2) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPoissonDiskDegenerateInput(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	if got := PoissonDisk(RectPolygon(100, 100, 0), SampleOptions{MinDist: 0}, rng); got != nil {
		t.Errorf("zero MinDist should yield nil, got %d points", len(got))
	}
	if got := PoissonDisk(Polygon{{0, 0}, {1, 1}}, SampleOptions{MinDist: 1}, rng); got != nil {
		t.Errorf("degenerate border should yield nil, got %d points", len(got))
	}
}

func TestPoissonDiskFillsDensely(t *testing.T) {
	// Sanity check on coverage: the count should be within a reasonable factor
	// of the theoretical maximum packing, not an order of magnitude below it.
	const r = 10
	border := RectPolygon(200, 200, 0)
	pts := PoissonDisk(border, SampleOptions{MinDist: r}, rand.New(rand.NewSource(5)))
	maxPack := 2 * 200 * 200 / (math.Sqrt(3) * r * r)
	if ratio := float64(len(pts)) / maxPack; ratio < 0.5 {
		t.Errorf("packing ratio %.2f is too sparse (%d points)", ratio, len(pts))
	}
}

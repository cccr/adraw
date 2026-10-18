package geom

import (
	"math"
	"math/rand"
	"testing"
)

func tessellateRect(t *testing.T, w, h, minDist, mergeProb float64, seed int64) (Polygon, []Polygon) {
	t.Helper()
	border := RectPolygon(w, h, 0)
	rng := rand.New(rand.NewSource(seed))
	pts := PoissonDisk(border, SampleOptions{MinDist: minDist}, rng)
	cells, err := Tessellate(pts, TessOptions{MergeProb: mergeProb, MaxVerts: 5}, rng)
	if err != nil {
		t.Fatalf("Tessellate: %v", err)
	}
	return border, cells
}

func TestTessellateCoversBorderExactly(t *testing.T) {
	// Full coverage with no gaps and no overlaps shows up as the cell areas
	// summing to the border area. This is the payoff of seeding the boundary
	// in PoissonDisk: the convex hull is the border, so no clipping is needed.
	border, cells := tessellateRect(t, 297, 210, 14, 0, 11)

	var sum float64
	for _, c := range cells {
		sum += Area(c)
	}
	want := Area(border)
	if math.Abs(sum-want) > 1e-6 {
		t.Errorf("cell areas sum to %v, want %v (diff %v)", sum, want, math.Abs(sum-want))
	}
}

func TestTessellateProducesNoSlivers(t *testing.T) {
	// The structural claim behind choosing Delaunay over blue noise: with a
	// guaranteed minimum point separation, no cell can be a sliver. Assert it
	// rather than assuming it.
	_, cells := tessellateRect(t, 297, 210, 14, 0, 3)

	const floor = 20 * math.Pi / 180
	worst := math.Pi
	for _, c := range cells {
		if a := MinAngle(c); a < worst {
			worst = a
		}
	}
	if worst < floor {
		t.Errorf("worst cell angle is %.1f deg, want >= %.0f deg",
			worst*180/math.Pi, floor*180/math.Pi)
	}
}

func TestTessellateNoTinyCells(t *testing.T) {
	// The user's constraint, stated directly: no small areas left over.
	const minDist = 14
	_, cells := tessellateRect(t, 297, 210, minDist, 0, 5)

	// An equilateral triangle of side minDist has this area; real cells should
	// be a decent fraction of it.
	ideal := math.Sqrt(3) / 4 * minDist * minDist
	for i, c := range cells {
		if a := Area(c); a < 0.15*ideal {
			t.Errorf("cell %d has area %.2f, well under the %.2f expected floor", i, a, 0.15*ideal)
		}
	}
}

func TestTessellateMergingStaysConvexAndBounded(t *testing.T) {
	_, cells := tessellateRect(t, 297, 210, 14, 0.5, 21)

	counts := map[int]int{}
	for i, c := range cells {
		if !IsConvex(c) {
			t.Errorf("merged cell %d is not convex: %v", i, c)
		}
		if len(c) > 5 {
			t.Errorf("merged cell %d has %d vertices, want <= 5", i, len(c))
		}
		counts[len(c)]++
	}
	if counts[4] == 0 {
		t.Error("merging produced no quads")
	}
	t.Logf("cell shapes: %d triangles, %d quads, %d pentagons", counts[3], counts[4], counts[5])
}

func TestTessellateMergingPreservesArea(t *testing.T) {
	// Merging must only dissolve interior edges, never lose or duplicate area.
	border, cells := tessellateRect(t, 297, 210, 14, 0.6, 9)
	var sum float64
	for _, c := range cells {
		sum += Area(c)
	}
	if want := Area(border); math.Abs(sum-want) > 1e-6 {
		t.Errorf("after merging, areas sum to %v, want %v", sum, want)
	}
}

func TestTessellateRejectsDegenerateInput(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	if _, err := Tessellate([]Vec2{{0, 0}, {1, 1}}, TessOptions{}, rng); err == nil {
		t.Error("too few points should error")
	}
	// Fully collinear input must return an error, not panic.
	collinear := []Vec2{{0, 0}, {1, 0}, {2, 0}, {3, 0}}
	if _, err := Tessellate(collinear, TessOptions{}, rng); err == nil {
		t.Error("collinear points should error")
	}
}

func TestMergeAlongEdge(t *testing.T) {
	// Two triangles of a unit square sharing the diagonal a-b.
	// t1 = a,b,c and t2 = b,a,d should fuse into the square b,c,a,d.
	got, ok := mergeAlongEdge([]int{0, 1, 2}, []int{1, 0, 3}, 0, 1)
	if !ok {
		t.Fatal("mergeAlongEdge reported failure")
	}
	want := []int{1, 2, 0, 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	if _, ok := mergeAlongEdge([]int{0, 1, 2}, []int{1, 0, 3}, 7, 8); ok {
		t.Error("merging on a non-existent edge should fail")
	}
}

func TestMinAngle(t *testing.T) {
	eq := Polygon{{0, 0}, {1, 0}, {0.5, math.Sqrt(3) / 2}}
	if got := MinAngle(eq); math.Abs(got-math.Pi/3) > 1e-9 {
		t.Errorf("equilateral MinAngle = %v, want %v", got, math.Pi/3)
	}
	sliver := Polygon{{0, 0}, {100, 0}, {50, 0.1}}
	if MinAngle(sliver) > 5*math.Pi/180 {
		t.Error("sliver should report a tiny minimum angle")
	}
}

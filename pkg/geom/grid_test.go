package geom

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
)

func testGrid(cell float64) *Grid {
	return NewGrid(drawing.Rect{Max: Vec2{X: 100, Y: 100}}, cell)
}

func TestGridAddAndQuery(t *testing.T) {
	g := testGrid(10)
	g.Add(Vec2{10, 10})
	g.Add(Vec2{50, 50})
	g.Add(Vec2{90, 90})

	if g.Len() != 3 {
		t.Errorf("Len = %d, want 3", g.Len())
	}
	if !g.AnyWithin(Vec2{12, 12}, 5) {
		t.Error("AnyWithin missed a point 2.8 away")
	}
	if g.AnyWithin(Vec2{30, 30}, 5) {
		t.Error("AnyWithin found a point that is far away")
	}
	// The boundary is exclusive: a point exactly d away is not "within".
	if g.AnyWithin(Vec2{20, 10}, 10) {
		t.Error("AnyWithin should be strict at exactly d")
	}
}

func TestGridNearestIsExact(t *testing.T) {
	// The subtle case: a point two cells away diagonally can be closer than one
	// in an adjacent cell. A ring search that stops at its first hit gets this
	// wrong, so check against brute force over random data.
	rng := rand.New(rand.NewSource(4))
	g := testGrid(7)
	var pts []Vec2
	for i := 0; i < 300; i++ {
		p := Vec2{rng.Float64() * 100, rng.Float64() * 100}
		pts = append(pts, p)
		g.Add(p)
	}

	for trial := 0; trial < 200; trial++ {
		q := Vec2{rng.Float64() * 100, rng.Float64() * 100}

		want, wantDist := -1, math.Inf(1)
		for i, p := range pts {
			if d := Dist(p, q); d < wantDist {
				want, wantDist = i, d
			}
		}
		got, ok := g.Nearest(q)
		if !ok {
			t.Fatalf("Nearest found nothing among %d points", len(pts))
		}
		if math.Abs(Dist(g.At(got), q)-wantDist) > 1e-12 {
			t.Fatalf("Nearest returned index %d at %v, brute force says %d at %v",
				got, Dist(g.At(got), q), want, wantDist)
		}
	}
}

func TestGridRemove(t *testing.T) {
	g := testGrid(10)
	a := g.Add(Vec2{10, 10})
	g.Add(Vec2{90, 90})

	g.Remove(a)
	if g.Len() != 1 {
		t.Errorf("Len = %d after removal, want 1", g.Len())
	}
	if g.AnyWithin(Vec2{10, 10}, 1) {
		t.Error("a removed point was still found")
	}
	// The index stays readable so callers can keep referring to it.
	if g.At(a) != (Vec2{10, 10}) {
		t.Error("At should still return a removed point's position")
	}

	got, _ := g.Nearest(Vec2{10, 10})
	if g.At(got) != (Vec2{90, 90}) {
		t.Error("Nearest returned a removed point")
	}

	g.Remove(a) // idempotent
	if g.Len() != 1 {
		t.Errorf("double Remove changed Len to %d", g.Len())
	}
}

func TestGridEmptyAndOutOfBounds(t *testing.T) {
	g := testGrid(10)
	if _, ok := g.Nearest(Vec2{5, 5}); ok {
		t.Error("Nearest on an empty grid should report false")
	}
	// Points outside the grid must not panic; they simply are not indexed.
	g.Add(Vec2{-500, -500})
	g.Add(Vec2{50, 50})
	if got, ok := g.Nearest(Vec2{50, 50}); !ok || g.At(got) != (Vec2{50, 50}) {
		t.Error("an out-of-bounds point disturbed the query")
	}
}

func TestGridWithin(t *testing.T) {
	g := testGrid(5)
	for x := 0.0; x < 100; x += 10 {
		g.Add(Vec2{x, 50})
	}
	// Points sit at x = 0,10,...,90 on y=50, so within 15 of x=50 there are
	// exactly three: 40, 50 and 60.
	got := g.Within(Vec2{50, 50}, 15, nil)
	if len(got) != 3 {
		t.Errorf("Within returned %d points, want 3: %v", len(got), got)
	}
	for _, i := range got {
		if Dist(g.At(i), Vec2{50, 50}) > 15+1e-9 {
			t.Errorf("Within returned a point %v away", Dist(g.At(i), Vec2{50, 50}))
		}
	}
}

package geom

import (
	"math"
	"math/rand"

	"github.com/cccr/adraw/pkg/drawing"
	"testing"
	"time"
)

var bigTriangle = Polygon{{0, 0}, {60, 0}, {30, 52}}

func toPoly(pts []Vec2) Polygon { return Polygon(pts) }

func TestSpiralRingsShrinkMonotonically(t *testing.T) {
	paths := Spiral(bigTriangle, SpiralOptions{Spacing: 1.2})
	if len(paths) < 10 {
		t.Fatalf("got %d rings, expected a dense fill", len(paths))
	}
	prev := math.Inf(1)
	for i, p := range paths {
		a := Area(toPoly(p.Pts))
		if a >= prev {
			t.Fatalf("ring %d has area %v, not smaller than the previous %v", i, a, prev)
		}
		if !p.Closed {
			t.Errorf("ring %d is not closed", i)
		}
		if len(p.Pts) != len(bigTriangle) {
			t.Errorf("ring %d has %d points, want %d", i, len(p.Pts), len(bigTriangle))
		}
		prev = a
	}
}

func TestSpiralRingsNest(t *testing.T) {
	paths := Spiral(bigTriangle, SpiralOptions{Spacing: 2})
	for i := 1; i < len(paths); i++ {
		parent := toPoly(paths[i-1].Pts)
		for _, pt := range paths[i].Pts {
			if !Contains(parent, pt) {
				t.Fatalf("ring %d escapes its parent at %+v", i, pt)
			}
		}
	}
}

func TestSpiralTwists(t *testing.T) {
	// The signature of this fill: rings rotate as they shrink. Compare the
	// direction of the first edge between the outer ring and a later one; a
	// plain inward inset would leave it unchanged.
	paths := Spiral(bigTriangle, SpiralOptions{Spacing: 1})
	if len(paths) < 5 {
		t.Fatal("not enough rings to measure twist")
	}
	angle := func(p []Vec2) float64 {
		d := Sub(p[1], p[0])
		return math.Atan2(d.Y, d.X)
	}
	if delta := math.Abs(angle(paths[4].Pts) - angle(paths[0].Pts)); delta < 0.05 {
		t.Errorf("rings barely rotated (%.4f rad); this is an inset, not a spiral", delta)
	}
}

func TestSpiralDirectionReverses(t *testing.T) {
	ccw := Spiral(bigTriangle, SpiralOptions{Spacing: 2, Clockwise: false})
	cw := Spiral(bigTriangle, SpiralOptions{Spacing: 2, Clockwise: true})
	if len(ccw) < 3 || len(cw) < 3 {
		t.Fatal("not enough rings")
	}
	if ccw[2].Pts[0] == cw[2].Pts[0] {
		t.Error("clockwise and counter-clockwise spirals are identical")
	}
}

func TestSpiralSpacingTracksParameter(t *testing.T) {
	// Spacing is meant to be physical: doubling it should roughly halve the
	// ring count, not change it arbitrarily.
	fine := len(Spiral(bigTriangle, SpiralOptions{Spacing: 1}))
	coarse := len(Spiral(bigTriangle, SpiralOptions{Spacing: 2}))
	if ratio := float64(fine) / float64(coarse); ratio < 1.4 || ratio > 2.6 {
		t.Errorf("ring counts %d vs %d give ratio %.2f, want near 2", fine, coarse, ratio)
	}
}

func TestSpiralTerminatesOnDegenerateInput(t *testing.T) {
	cases := []struct {
		name string
		p    Polygon
	}{
		{"near-zero area", Polygon{{0, 0}, {0.01, 0}, {0, 0.01}}},
		{"near collinear", Polygon{{0, 0}, {100, 0}, {50, 0.001}}},
		{"duplicate points", Polygon{{0, 0}, {0, 0}, {10, 10}}},
		{"too few points", Polygon{{0, 0}, {1, 1}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			done := make(chan int, 1)
			go func() { done <- len(Spiral(c.p, SpiralOptions{Spacing: 1})) }()
			select {
			case n := <-done:
				if n > 400 {
					t.Errorf("emitted %d rings, exceeding MaxRings", n)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Spiral did not terminate")
			}
		})
	}
}

func TestSpiralRejectsBadOptions(t *testing.T) {
	if got := Spiral(bigTriangle, SpiralOptions{Spacing: 0}); got != nil {
		t.Errorf("zero spacing should yield nil, got %d rings", len(got))
	}
}

func TestSpiralHandlesPentagon(t *testing.T) {
	var pent Polygon
	for i := 0; i < 5; i++ {
		a := float64(i) / 5 * 2 * math.Pi
		pent = append(pent, Vec2{30 * math.Cos(a), 30 * math.Sin(a)})
	}
	paths := Spiral(pent, SpiralOptions{Spacing: 1.5})
	if len(paths) < 5 {
		t.Fatalf("pentagon produced only %d rings", len(paths))
	}
	for i, p := range paths {
		if len(p.Pts) != 5 {
			t.Errorf("ring %d has %d points, want 5", i, len(p.Pts))
		}
	}
}

func TestJitter(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	paths := Spiral(bigTriangle, SpiralOptions{Spacing: 3})
	orig := append([]Vec2(nil), paths[0].Pts...)

	Jitter(paths, 0, rng.Float64)
	for i := range orig {
		if paths[0].Pts[i] != orig[i] {
			t.Fatal("zero amount should leave points untouched")
		}
	}

	Jitter(paths, 1.5, rng.Float64)
	var moved bool
	for i := range orig {
		d := Dist(paths[0].Pts[i], orig[i])
		if d > 1.5+1e-9 {
			t.Errorf("point %d moved %v, beyond the %v limit", i, d, 1.5)
		}
		if d > 0 {
			moved = true
		}
	}
	if !moved {
		t.Error("jitter moved nothing")
	}
}

func TestHatchStaysInsidePolygon(t *testing.T) {
	paths := Hatch(bigTriangle, 2, 0)
	if len(paths) < 10 {
		t.Fatalf("got %d hatch lines, expected more", len(paths))
	}
	for i, p := range paths {
		if len(p.Pts) != 2 {
			t.Fatalf("hatch line %d has %d points, want 2", i, len(p.Pts))
		}
		if p.Closed {
			t.Errorf("hatch line %d should not be closed", i)
		}
		for _, pt := range p.Pts {
			if !Contains(bigTriangle, pt) {
				t.Errorf("hatch line %d escapes the polygon at %+v", i, pt)
			}
		}
	}
}

func TestHatchSpacingAndAngle(t *testing.T) {
	fine := len(Hatch(bigTriangle, 1, 0))
	coarse := len(Hatch(bigTriangle, 2, 0))
	if ratio := float64(fine) / float64(coarse); ratio < 1.6 || ratio > 2.4 {
		t.Errorf("line counts %d vs %d give ratio %.2f, want near 2", fine, coarse, ratio)
	}

	flat := Hatch(bigTriangle, 3, 0)
	steep := Hatch(bigTriangle, 3, math.Pi/2)
	if len(flat) == 0 || len(steep) == 0 {
		t.Fatal("hatching produced nothing")
	}
	angleOf := func(p drawing.Path) float64 {
		d := Sub(p.Pts[1], p.Pts[0])
		return math.Abs(math.Atan2(d.Y, d.X))
	}
	if math.Abs(angleOf(flat[0])-angleOf(steep[0])) < 1 {
		t.Error("changing the angle did not rotate the hatch")
	}
}

func TestHatchRejectsBadInput(t *testing.T) {
	if got := Hatch(bigTriangle, 0, 0); got != nil {
		t.Error("zero spacing should yield nil")
	}
	if got := Hatch(Polygon{{0, 0}, {1, 1}}, 1, 0); got != nil {
		t.Error("degenerate polygon should yield nil")
	}
}

func TestInsetRingsDoNotRotate(t *testing.T) {
	paths := Inset(bigTriangle, SpiralOptions{Spacing: 2})
	if len(paths) < 5 {
		t.Fatalf("got %d rings", len(paths))
	}
	// The defining difference from Spiral: edge directions stay put.
	angle := func(p []Vec2) float64 {
		d := Sub(p[1], p[0])
		return math.Atan2(d.Y, d.X)
	}
	if delta := math.Abs(angle(paths[3].Pts) - angle(paths[0].Pts)); delta > 1e-6 {
		t.Errorf("rings rotated by %v; Inset must not twist", delta)
	}
	prev := math.Inf(1)
	for i, p := range paths {
		a := Area(toPoly(p.Pts))
		if a >= prev {
			t.Fatalf("ring %d did not shrink", i)
		}
		prev = a
	}
}

func TestInsetHoldsSpacingAtCorners(t *testing.T) {
	// The reason Inset offsets edges rather than walking them: at a sharp
	// corner an edge-walk bunches rings together, while a true offset keeps the
	// perpendicular gap equal to Spacing everywhere.
	const spacing = 2
	sharp := Polygon{{0, 0}, {100, 0}, {40, 25}}
	paths := Inset(sharp, SpiralOptions{Spacing: spacing})
	if len(paths) < 3 {
		t.Fatalf("got %d rings", len(paths))
	}
	for i := 1; i < len(paths); i++ {
		got := InRadius(toPoly(paths[i-1].Pts)) - InRadius(toPoly(paths[i].Pts))
		if math.Abs(got-spacing) > 0.05 {
			t.Errorf("gap between rings %d and %d is %.3f, want %v", i-1, i, got, spacing)
		}
	}
}

func TestFan(t *testing.T) {
	paths := Fan(bigTriangle, 3, 0)
	if len(paths) < 10 {
		t.Fatalf("got %d rays", len(paths))
	}
	apex := bigTriangle[0]
	for i, p := range paths {
		if len(p.Pts) != 2 {
			t.Fatalf("ray %d has %d points, want 2", i, len(p.Pts))
		}
		if p.Pts[0] != apex {
			t.Errorf("ray %d does not start at the apex", i)
		}
		if !Contains(bigTriangle, p.Pts[1]) {
			t.Errorf("ray %d ends outside the polygon at %+v", i, p.Pts[1])
		}
	}
}

func TestCrossHatchIsTwoPasses(t *testing.T) {
	single := len(Hatch(bigTriangle, 3, 0))
	both := len(CrossHatch(bigTriangle, 3, 0, math.Pi/3))
	if both <= single {
		t.Errorf("crosshatch produced %d lines, single hatch %d", both, single)
	}
}

func TestDots(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	paths := Dots(bigTriangle, 4, rng)
	if len(paths) < 20 {
		t.Fatalf("got %d dots", len(paths))
	}
	for i, p := range paths {
		if len(p.Pts) != 2 {
			t.Fatalf("dot %d has %d points; a plotter needs a short stroke", i, len(p.Pts))
		}
		if d := Dist(p.Pts[0], p.Pts[1]); d <= 0 || d > 1 {
			t.Errorf("dot %d is %v long, want a short mark", i, d)
		}
	}
	// Blue-noise spacing, not clumping.
	for i := range paths {
		for j := i + 1; j < len(paths); j++ {
			if Dist(paths[i].Pts[0], paths[j].Pts[0]) < 4-0.5 {
				t.Fatalf("dots %d and %d are clumped", i, j)
			}
		}
	}
}

func TestFillsRejectBadInput(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	degenerate := Polygon{{0, 0}, {1, 1}}
	if Inset(degenerate, SpiralOptions{Spacing: 1}) != nil {
		t.Error("Inset should reject a degenerate polygon")
	}
	if Fan(degenerate, 1, 0) != nil {
		t.Error("Fan should reject a degenerate polygon")
	}
	if Dots(degenerate, 1, rng) != nil {
		t.Error("Dots should reject a degenerate polygon")
	}
	if Inset(bigTriangle, SpiralOptions{Spacing: 0}) != nil {
		t.Error("Inset should reject zero spacing")
	}
}

func TestInsetRingsStayNested(t *testing.T) {
	// The property that actually matters and that shrinking-area alone does not
	// imply: once the offset edges cross, the result is a self-intersecting
	// bowtie whose shoelace area is small but whose geometry sprawls far
	// outside the cell.
	shapes := []struct {
		name string
		p    Polygon
	}{
		{"triangle", bigTriangle},
		{"sharp sliver", Polygon{{0, 0}, {100, 0}, {40, 12}}},
		{"quad", Polygon{{0, 0}, {50, 5}, {45, 40}, {5, 35}}},
		{"pentagon", Polygon{{0, 0}, {40, 0}, {52, 30}, {20, 48}, {-10, 28}}},
	}
	for _, sh := range shapes {
		t.Run(sh.name, func(t *testing.T) {
			paths := Inset(sh.p, SpiralOptions{Spacing: 1.5})
			for i := 1; i < len(paths); i++ {
				parent := toPoly(paths[i-1].Pts)
				for _, pt := range paths[i].Pts {
					if !Contains(parent, pt) {
						t.Fatalf("ring %d escapes its parent at %+v", i, pt)
					}
				}
			}
			// And nothing may leave the original cell.
			for i, p := range paths {
				for _, pt := range p.Pts {
					if !Contains(sh.p, pt) {
						t.Fatalf("ring %d leaves the cell at %+v", i, pt)
					}
				}
			}
		})
	}
}

func TestDotsStayInsideIncludingTheirWidth(t *testing.T) {
	// A dot is a short stroke, not a point: sampling up to the edge would leave
	// half of every boundary mark hanging outside the cell.
	rng := rand.New(rand.NewSource(3))
	for _, sh := range []struct {
		name string
		p    Polygon
	}{
		{"triangle", bigTriangle},
		{"small quad", Polygon{{0, 0}, {14, 0}, {14, 12}, {0, 12}}},
	} {
		t.Run(sh.name, func(t *testing.T) {
			for _, p := range Dots(sh.p, 2, rng) {
				for _, pt := range p.Pts {
					if !Contains(sh.p, pt) {
						t.Fatalf("a dot's end lands outside the cell at %+v", pt)
					}
				}
			}
		})
	}
}

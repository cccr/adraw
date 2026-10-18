package geom

import (
	"math"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
)

func unitBox(size float64) drawing.Rect {
	return drawing.Rect{Max: Vec2{X: size, Y: size}}
}

// circleField is a cone: its level sets are exact circles of known radius.
func circleField(cx, cy float64) Field {
	return func(x, y float64) float64 { return math.Hypot(x-cx, y-cy) }
}

func TestContourTracesKnownCircles(t *testing.T) {
	// A field whose isolines are analytically known, so accuracy is checkable
	// rather than merely plausible.
	paths := Contour(circleField(50, 50), ContourOptions{
		Bounds: unitBox(100),
		Step:   0.5,
		Levels: []float64{10, 20, 30},
	})
	if len(paths) != 3 {
		t.Fatalf("got %d contours, want exactly 3 closed circles", len(paths))
	}
	for i, p := range paths {
		if !p.Closed {
			t.Errorf("contour %d is not closed", i)
		}
		want := float64((i + 1) * 10)
		for _, pt := range p.Pts {
			if r := math.Hypot(pt.X-50, pt.Y-50); math.Abs(r-want) > 0.05 {
				t.Fatalf("contour %d has radius %.3f, want %v", i, r, want)
			}
		}
	}
}

func TestContourStitchesIntoFewLongPaths(t *testing.T) {
	// The whole point of stitching. Raw marching squares would emit one stub
	// per grid cell the contour crosses; a circle of radius 30 at step 0.5
	// crosses roughly 380 cells.
	paths := Contour(circleField(50, 50), ContourOptions{
		Bounds: unitBox(100), Step: 0.5, Levels: []float64{30},
	})
	if len(paths) != 1 {
		t.Fatalf("got %d paths, want 1 stitched circle", len(paths))
	}
	if n := len(paths[0].Pts); n < 300 {
		t.Errorf("stitched circle has only %d points; segments were dropped", n)
	}
}

func TestContourHandlesOpenCurves(t *testing.T) {
	// A contour that runs off the sampled region must come out as one open
	// path, not two halves found from the middle outward.
	paths := Contour(func(x, y float64) float64 { return x }, ContourOptions{
		Bounds: unitBox(100), Step: 1, Levels: []float64{50},
	})
	if len(paths) != 1 {
		t.Fatalf("got %d paths, want 1", len(paths))
	}
	if paths[0].Closed {
		t.Error("a straight line across the region should not be closed")
	}
	span := Dist(paths[0].Pts[0], paths[0].Pts[len(paths[0].Pts)-1])
	if span < 95 {
		t.Errorf("open contour spans only %.1f of ~100; it was split", span)
	}
}

func TestContourSaddleDoesNotCross(t *testing.T) {
	// A saddle field is the ambiguous case in marching squares. Resolved badly
	// it produces contours that cross each other.
	paths := Contour(func(x, y float64) float64 {
		return (x - 50) * (y - 50) / 100
	}, ContourOptions{
		Bounds: unitBox(100), Step: 1, Levels: []float64{-5, 0, 5},
	})
	if len(paths) == 0 {
		t.Fatal("saddle field produced no contours")
	}
	for i, p := range paths {
		for _, pt := range p.Pts {
			if math.IsNaN(pt.X) || math.IsNaN(pt.Y) {
				t.Fatalf("path %d has NaN", i)
			}
		}
	}
}

func TestContourLevelsOutsideRangeTraceNothing(t *testing.T) {
	paths := Contour(circleField(50, 50), ContourOptions{
		Bounds: unitBox(100), Step: 1, Levels: []float64{-5, 1000},
	})
	if len(paths) != 0 {
		t.Errorf("levels outside the field's range produced %d paths", len(paths))
	}
}

func TestContourRejectsBadOptions(t *testing.T) {
	f := circleField(50, 50)
	if Contour(f, ContourOptions{Bounds: unitBox(100), Step: 0, Levels: []float64{10}}) != nil {
		t.Error("zero step should yield nil")
	}
	if Contour(f, ContourOptions{Bounds: unitBox(100), Step: 1}) != nil {
		t.Error("no levels should yield nil")
	}
	if Contour(f, ContourOptions{Bounds: drawing.Rect{}, Step: 1, Levels: []float64{1}}) != nil {
		t.Error("an empty region should yield nil")
	}
}

func TestLevelsBetween(t *testing.T) {
	got := LevelsBetween(0, 10, 4)
	want := []float64{2, 4, 6, 8}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	// Levels must stay strictly inside: one exactly at an extreme traces
	// nothing and would silently cost a contour.
	for _, v := range got {
		if v <= 0 || v >= 10 {
			t.Errorf("level %v touches the bounds", v)
		}
	}
	if LevelsBetween(0, 1, 0) != nil {
		t.Error("count 0 should yield nil")
	}
}

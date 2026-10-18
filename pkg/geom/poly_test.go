package geom

import (
	"math"
	"testing"
)

var (
	unitSquareCCW = Polygon{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	unitSquareCW  = Polygon{{0, 1}, {1, 1}, {1, 0}, {0, 0}}
	chevron       = Polygon{{0, 0}, {4, 0}, {2, 1}, {4, 4}, {0, 4}} // reflex at {2,1}
)

func TestSignedAreaWinding(t *testing.T) {
	if got := SignedArea(unitSquareCCW); math.Abs(got-1) > 1e-12 {
		t.Errorf("CCW signed area = %v, want +1", got)
	}
	if got := SignedArea(unitSquareCW); math.Abs(got+1) > 1e-12 {
		t.Errorf("CW signed area = %v, want -1", got)
	}
	if got := Area(unitSquareCW); math.Abs(got-1) > 1e-12 {
		t.Errorf("Area = %v, want 1 regardless of winding", got)
	}
	if got := SignedArea(Polygon{{0, 0}, {1, 1}}); got != 0 {
		t.Errorf("degenerate polygon area = %v, want 0", got)
	}
}

func TestPerimeterAndCentroid(t *testing.T) {
	if got := Perimeter(unitSquareCCW); math.Abs(got-4) > 1e-12 {
		t.Errorf("Perimeter = %v, want 4", got)
	}
	c := Centroid(unitSquareCCW)
	if math.Abs(c.X-0.5) > 1e-12 || math.Abs(c.Y-0.5) > 1e-12 {
		t.Errorf("Centroid = %+v, want {0.5,0.5}", c)
	}
	// A zero-area polygon must still produce a usable point, not NaN.
	c = Centroid(Polygon{{0, 0}, {2, 0}, {4, 0}})
	if math.IsNaN(c.X) || math.IsNaN(c.Y) {
		t.Errorf("degenerate centroid = %+v, want finite", c)
	}
}

func TestIsConvex(t *testing.T) {
	cases := []struct {
		name string
		p    Polygon
		want bool
	}{
		{"ccw square", unitSquareCCW, true},
		{"cw square", unitSquareCW, true},
		{"chevron is reflex", chevron, false},
		{"triangle", Polygon{{0, 0}, {1, 0}, {0, 1}}, true},
		{"collinear vertex tolerated", Polygon{{0, 0}, {1, 0}, {2, 0}, {2, 2}, {0, 2}}, true},
		{"fully collinear", Polygon{{0, 0}, {1, 0}, {2, 0}}, false},
		{"too few points", Polygon{{0, 0}, {1, 0}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsConvex(c.p); got != c.want {
				t.Errorf("IsConvex = %v, want %v", got, c.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	cases := []struct {
		name string
		pt   Vec2
		want bool
	}{
		{"interior", Vec2{0.5, 0.5}, true},
		{"corner", Vec2{0, 0}, true},
		{"edge midpoint", Vec2{0.5, 0}, true},
		{"outside right", Vec2{1.5, 0.5}, false},
		{"outside below", Vec2{0.5, -0.5}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Contains(unitSquareCCW, c.pt); got != c.want {
				t.Errorf("Contains(%v) = %v, want %v", c.pt, got, c.want)
			}
		})
	}
	// The reflex notch of the chevron must read as outside.
	if Contains(chevron, Vec2{3, 1.2}) {
		t.Error("point in the chevron notch reported as inside")
	}
}

func TestInRadius(t *testing.T) {
	// For a square of side s the inradius is s/2.
	if got := InRadius(unitSquareCCW); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("InRadius = %v, want 0.5", got)
	}
	if got := InRadius(Polygon{}); got != 0 {
		t.Errorf("InRadius of empty = %v, want 0", got)
	}
}

func TestRectPolygon(t *testing.T) {
	p := RectPolygon(100, 50, 10)
	if got := Area(p); math.Abs(got-80*30) > 1e-9 {
		t.Errorf("Area = %v, want %v", got, 80*30)
	}
	if SignedArea(p) < 0 {
		t.Error("RectPolygon should wind counter-clockwise")
	}
}

package drawing

import "testing"

func TestRectDimensions(t *testing.T) {
	r := Rect{Min: Vec2{1, 2}, Max: Vec2{11, 8}}
	if r.Width() != 10 {
		t.Errorf("Width() = %v, want 10", r.Width())
	}
	if r.Height() != 6 {
		t.Errorf("Height() = %v, want 6", r.Height())
	}
}

func TestRectContains(t *testing.T) {
	r := Rect{Min: Vec2{0, 0}, Max: Vec2{10, 10}}
	cases := []struct {
		p    Vec2
		want bool
	}{
		{Vec2{5, 5}, true},
		{Vec2{0, 0}, true},   // corner is inclusive
		{Vec2{10, 10}, true}, // corner is inclusive
		{Vec2{0, 5}, true},   // edge is inclusive
		{Vec2{-0.001, 5}, false},
		{Vec2{5, 10.001}, false},
	}
	for _, c := range cases {
		if got := r.Contains(c.p); got != c.want {
			t.Errorf("Contains(%v) = %v, want %v", c.p, got, c.want)
		}
	}
}

func TestRectUnion(t *testing.T) {
	a := Rect{Min: Vec2{0, 0}, Max: Vec2{5, 5}}
	b := Rect{Min: Vec2{-2, 3}, Max: Vec2{4, 9}}
	got := a.Union(b)
	want := Rect{Min: Vec2{-2, 0}, Max: Vec2{5, 9}}
	if got != want {
		t.Errorf("Union = %+v, want %+v", got, want)
	}
}

func TestBoundsOf(t *testing.T) {
	if got := BoundsOf(nil); (got != Rect{}) {
		t.Errorf("BoundsOf(nil) = %+v, want zero", got)
	}
	got := BoundsOf([]Vec2{{3, 1}, {-1, 7}, {5, 2}})
	want := Rect{Min: Vec2{-1, 1}, Max: Vec2{5, 7}}
	if got != want {
		t.Errorf("BoundsOf = %+v, want %+v", got, want)
	}
}

package optimize

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/patch"

	"github.com/cccr/adraw/patches/tangle"
)

func TestReorderShortensTravel(t *testing.T) {
	f, err := tangle.New().Generate(context.Background(), patch.Request{Seed: 1})
	if err != nil {
		t.Fatal(err)
	}

	before := TravelDistance(f)
	got := Reorder(f)
	after := TravelDistance(got)

	t.Logf("travel: %.0f mm -> %.0f mm (%.1fx shorter) over %d paths",
		before, after, before/after, len(got.Paths))

	if after >= before {
		t.Errorf("travel did not improve: %.0f -> %.0f", before, after)
	}
	if got.Meta["travel_before_mm"] == "" || got.Meta["travel_after_mm"] == "" {
		t.Error("Meta did not record the travel distances")
	}
}

// pathKey identifies a stroke independently of where it starts or which
// direction it runs, so the test can assert the drawing is unchanged.
func pathKey(p drawing.Path) string {
	pts := make([]string, len(p.Pts))
	for i, pt := range p.Pts {
		pts[i] = formatPt(pt)
	}
	if p.Closed {
		// Rotation-invariant: start from the lexically smallest vertex, and
		// take whichever direction sorts lower.
		best := 0
		for i, s := range pts {
			if s < pts[best] {
				best = i
			}
		}
		fwd := append(append([]string{}, pts[best:]...), pts[:best]...)
		rev := make([]string, len(fwd))
		rev[0] = fwd[0]
		for i := 1; i < len(fwd); i++ {
			rev[i] = fwd[len(fwd)-i]
		}
		return "C" + minJoin(fwd, rev)
	}
	rev := make([]string, len(pts))
	for i, s := range pts {
		rev[len(pts)-1-i] = s
	}
	return "O" + minJoin(pts, rev)
}

func formatPt(p drawing.Vec2) string {
	return fmt.Sprintf("%.4f,%.4f;", p.X, p.Y)
}

func minJoin(a, b []string) string {
	ja, jb := strings.Join(a, ""), strings.Join(b, "")
	if ja < jb {
		return ja
	}
	return jb
}

func TestReorderPreservesEveryStroke(t *testing.T) {
	// The whole point of this pass is that it must not change the drawing. Any
	// path that is lost, gained or altered here would be an invisible
	// corruption of the artwork.
	f, err := tangle.New().Generate(context.Background(), patch.Request{Seed: 5})
	if err != nil {
		t.Fatal(err)
	}

	want := make([]string, len(f.Paths))
	for i, p := range f.Paths {
		want[i] = pathKey(p)
	}

	got := Reorder(f)
	if len(got.Paths) != len(want) {
		t.Fatalf("path count changed: %d -> %d", len(want), len(got.Paths))
	}

	have := make([]string, len(got.Paths))
	for i, p := range got.Paths {
		have[i] = pathKey(p)
	}
	sort.Strings(want)
	sort.Strings(have)
	for i := range want {
		if want[i] != have[i] {
			t.Fatalf("stroke %d differs after reordering", i)
		}
	}
}

func TestReorderRotatesClosedPaths(t *testing.T) {
	// A ring far from the pen should be entered at its nearest vertex, not
	// always at vertex zero.
	f := drawing.Frame{
		Bounds: drawing.Rect{Max: drawing.Vec2{X: 100, Y: 100}},
		Paths: []drawing.Path{
			{Pts: []drawing.Vec2{{X: 1, Y: 1}, {X: 2, Y: 1}}},
			{Pts: []drawing.Vec2{
				{X: 90, Y: 90}, {X: 95, Y: 90}, {X: 95, Y: 95}, {X: 10, Y: 5},
			}, Closed: true},
		},
	}
	got := Reorder(f)

	ring := got.Paths[1]
	if !ring.Closed {
		t.Fatal("expected the closed ring second")
	}
	if ring.Pts[0] != (drawing.Vec2{X: 10, Y: 5}) {
		t.Errorf("ring starts at %+v, want the vertex nearest the pen {10,5}", ring.Pts[0])
	}
}

func TestReorderReversesOpenPaths(t *testing.T) {
	f := drawing.Frame{
		Bounds: drawing.Rect{Max: drawing.Vec2{X: 100, Y: 100}},
		Paths: []drawing.Path{
			{Pts: []drawing.Vec2{{X: 1, Y: 1}, {X: 2, Y: 2}}},
			// Its far end is at {3,3}; entering there and drawing backwards is
			// much shorter than starting at {99,99}.
			{Pts: []drawing.Vec2{{X: 99, Y: 99}, {X: 3, Y: 3}}},
		},
	}
	got := Reorder(f)
	if got.Paths[1].Pts[0] != (drawing.Vec2{X: 3, Y: 3}) {
		t.Errorf("open path starts at %+v, want it reversed to {3,3}", got.Paths[1].Pts[0])
	}
}

func TestReorderHandlesDegenerateFrames(t *testing.T) {
	for _, c := range []struct {
		name string
		f    drawing.Frame
	}{
		{"empty", drawing.Frame{}},
		{"single path", drawing.Frame{Paths: []drawing.Path{{Pts: []drawing.Vec2{{}, {X: 1}}}}}},
		{"path with no points", drawing.Frame{
			Bounds: drawing.Rect{Max: drawing.Vec2{X: 10, Y: 10}},
			Paths:  []drawing.Path{{}, {Pts: []drawing.Vec2{{}, {X: 1}}}},
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Reorder(c.f)
			if len(got.Paths) != len(c.f.Paths) {
				t.Errorf("path count changed from %d to %d", len(c.f.Paths), len(got.Paths))
			}
		})
	}
}

func TestTravelDistance(t *testing.T) {
	f := drawing.Frame{
		Bounds: drawing.Rect{Max: drawing.Vec2{X: 100, Y: 100}},
		Paths: []drawing.Path{
			{Pts: []drawing.Vec2{{X: 0, Y: 0}, {X: 10, Y: 0}}},
			{Pts: []drawing.Vec2{{X: 20, Y: 0}, {X: 30, Y: 0}}},
		},
	}
	// Origin to (0,0) is 0, then (10,0) to (20,0) is 10.
	if got := TravelDistance(f); math.Abs(got-10) > 1e-9 {
		t.Errorf("TravelDistance = %v, want 10", got)
	}
}

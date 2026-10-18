package svg

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
)

func renderOut(t *testing.T, f drawing.Frame) string {
	t.Helper()
	var buf bytes.Buffer
	if err := New().Render(&buf, f); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return buf.String()
}

func TestViewBoxIsInMillimetres(t *testing.T) {
	f := drawing.Frame{
		Bounds: drawing.Rect{Max: drawing.Vec2{X: 297, Y: 210}},
		Paths:  []drawing.Path{{Pts: []drawing.Vec2{{X: 0, Y: 0}, {X: 10, Y: 10}}}},
	}
	out := renderOut(t, f)
	// width/height in mm alongside a viewBox in the same numbers is what makes
	// one user unit equal one millimetre on the plotter.
	if !strings.Contains(out, `width="297mm" height="210mm"`) {
		t.Error("physical dimensions are not in millimetres")
	}
	if !strings.Contains(out, `viewBox="0 0 297 210"`) {
		t.Error("viewBox does not match the frame bounds")
	}
}

func TestPathsAndClosure(t *testing.T) {
	f := drawing.Frame{
		Bounds: drawing.Rect{Max: drawing.Vec2{X: 100, Y: 100}},
		Paths: []drawing.Path{
			{Pts: []drawing.Vec2{{X: 1, Y: 2}, {X: 3, Y: 4}}, Closed: true},
			{Pts: []drawing.Vec2{{X: 5, Y: 6}, {X: 7, Y: 8}}, Closed: false},
			{Pts: []drawing.Vec2{{X: 9, Y: 9}}}, // degenerate, must be skipped
		},
	}
	out := renderOut(t, f)

	if !strings.Contains(out, `d="M1.000 2.000 L3.000 4.000Z"`) {
		t.Error("closed path is missing its Z")
	}
	if !strings.Contains(out, `d="M5.000 6.000 L7.000 8.000"`) {
		t.Error("open path is malformed")
	}
	if strings.Contains(out, "M9.000") {
		t.Error("a single-point path should not be emitted")
	}
	if got := strings.Count(out, "<path"); got != 2 {
		t.Errorf("got %d paths, want 2", got)
	}
}

func TestMetaRecorded(t *testing.T) {
	f := drawing.Frame{
		Bounds: drawing.Rect{Max: drawing.Vec2{X: 10, Y: 10}},
		Paths:  []drawing.Path{{Pts: []drawing.Vec2{{X: 0, Y: 0}, {X: 1, Y: 1}}}},
		Meta:   map[string]string{"seed": "42", "patch": "tangle"},
	}
	if !strings.Contains(renderOut(t, f), "<!-- adraw: patch=tangle seed=42 -->") {
		t.Error("metadata comment missing or unsorted")
	}
}

func TestZeroBoundsRejected(t *testing.T) {
	var buf bytes.Buffer
	err := New().Render(&buf, drawing.Frame{})
	if err == nil {
		t.Error("a frame with no extent should be rejected, not written as a broken SVG")
	}
}

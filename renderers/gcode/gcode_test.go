package gcode

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
)

func testFrame() drawing.Frame {
	return drawing.Frame{
		Bounds: drawing.Rect{Max: drawing.Vec2{X: 100, Y: 100}},
		Paths: []drawing.Path{
			{Pts: []drawing.Vec2{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 5, Y: 8}}, Closed: true},
			{Pts: []drawing.Vec2{{X: 20, Y: 20}, {X: 30, Y: 25}}, Closed: false},
			{Pts: []drawing.Vec2{{X: 1, Y: 1}}}, // degenerate, must be skipped
		},
		Meta: map[string]string{"patch": "tangle", "seed": "7"},
	}
}

func renderOut(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := New().Render(&buf, testFrame()); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return buf.String()
}

func TestPreambleAndMeta(t *testing.T) {
	out := renderOut(t)
	for _, want := range []string{"G21", "G90", "M2", "; adraw: patch=tangle seed=7"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q", want)
		}
	}
}

func TestPenUpDownPairsBalance(t *testing.T) {
	out := renderOut(t)
	// Two drawable paths: one pen-down each, plus the initial pen-up.
	if got, want := strings.Count(out, "pen down"), 2; got != want {
		t.Errorf("got %d pen-down moves, want %d", got, want)
	}
	if got, want := strings.Count(out, "pen up"), 3; got != want {
		t.Errorf("got %d pen-up moves, want %d (one initial plus one per path)", got, want)
	}
}

func TestClosedPathReturnsToStart(t *testing.T) {
	out := renderOut(t)
	lines := strings.Split(out, "\n")

	var moves []string
	for _, l := range lines {
		if strings.HasPrefix(l, "G1 X") {
			moves = append(moves, l[:strings.Index(l, " F")])
		}
	}
	// The closed triangle emits 3 moves: two edges plus the closing edge back
	// to the first point.
	if len(moves) < 4 {
		t.Fatalf("got only %d cutting moves: %v", len(moves), moves)
	}
	if moves[2] != "G1 X0.000 Y0.000" {
		t.Errorf("closed path did not return to its start, got %q", moves[2])
	}
	// The open path must not close.
	if last := moves[len(moves)-1]; last != "G1 X30.000 Y25.000" {
		t.Errorf("open path ended at %q, want its final point", last)
	}
}

func TestDegeneratePathSkipped(t *testing.T) {
	if strings.Contains(renderOut(t), "X1.000 Y1.000") {
		t.Error("a single-point path should not be emitted")
	}
}

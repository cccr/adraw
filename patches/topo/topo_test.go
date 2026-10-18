package topo

import (
	"context"
	"testing"

	"github.com/cccr/adraw/pkg/geom"
	"github.com/cccr/adraw/pkg/patch"
	"github.com/cccr/adraw/pkg/patch/patchtest"
)

func TestConformance(t *testing.T) {
	patchtest.Verify(t, New())
}

func TestProducesFewLongPaths(t *testing.T) {
	// The signature of this patch, and the inverse of tangle's profile: a
	// handful of long curves rather than thousands of short strokes. If this
	// flips, stitching has regressed.
	f, err := New().Generate(context.Background(), patch.Request{Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	var total int
	for _, p := range f.Paths {
		total += len(p.Pts)
	}
	avg := float64(total) / float64(len(f.Paths))
	t.Logf("%d paths, %d points, %.0f points/path", len(f.Paths), total, avg)
	if avg < 20 {
		t.Errorf("average path has only %.1f points; contours are fragmenting", avg)
	}
}

func TestLevelsControlDensity(t *testing.T) {
	count := func(levels float64) int {
		f, err := New().Generate(context.Background(), patch.Request{
			Seed: 2, Values: patch.Values{"levels": levels},
		})
		if err != nil {
			t.Fatal(err)
		}
		return len(f.Paths)
	}
	if few, many := count(5), count(50); many <= few {
		t.Errorf("50 levels gave %d paths, 5 gave %d", many, few)
	}
}

func TestStaysInsideCustomBorder(t *testing.T) {
	tri := geom.Polygon{{X: 20, Y: 20}, {X: 200, Y: 30}, {X: 100, Y: 160}}
	f, err := New().Generate(context.Background(), patch.Request{Seed: 3, Border: tri})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Paths) == 0 {
		t.Fatal("custom border produced nothing")
	}
	for i, p := range f.Paths {
		for _, pt := range p.Pts {
			if !geom.Contains(tri, pt) {
				t.Fatalf("path %d escapes the border at %+v", i, pt)
			}
		}
	}
}

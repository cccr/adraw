package flow

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

func TestLinesRespectSeparation(t *testing.T) {
	// The property that decides whether a flow field is legible or mush.
	const sep = 4
	f, err := New().Generate(context.Background(), patch.Request{
		Seed: 1, Values: patch.Values{"separation": sep},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Paths) < 10 {
		t.Fatalf("only %d lines", len(f.Paths))
	}

	// No tolerance is warranted: a point is appended only after passing the
	// separation test, so the guarantee should hold exactly. A slack here would
	// have hidden the real bug that this caught.
	const tol = 1e-9
	for i := range f.Paths {
		for j := i + 1; j < len(f.Paths); j++ {
			for _, a := range f.Paths[i].Pts {
				for _, b := range f.Paths[j].Pts {
					if geom.Dist(a, b) < sep-tol {
						t.Fatalf("lines %d and %d come within %.2f, want >= %v",
							i, j, geom.Dist(a, b), sep-tol)
					}
				}
			}
		}
	}
}

func TestProducesLongLines(t *testing.T) {
	f, err := New().Generate(context.Background(), patch.Request{Seed: 2})
	if err != nil {
		t.Fatal(err)
	}
	var total int
	for _, p := range f.Paths {
		total += len(p.Pts)
	}
	avg := float64(total) / float64(len(f.Paths))
	t.Logf("%d lines, %.0f points/line", len(f.Paths), avg)
	if avg < 10 {
		t.Errorf("average line has %.1f points; tracing is stopping too early", avg)
	}
}

func TestStaysInsideBorder(t *testing.T) {
	tri := geom.Polygon{{X: 20, Y: 20}, {X: 220, Y: 30}, {X: 110, Y: 170}}
	f, err := New().Generate(context.Background(), patch.Request{
		Seed: 3, Border: tri, Values: patch.Values{"separation": 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Paths) == 0 {
		t.Fatal("custom border produced nothing")
	}
	for i, p := range f.Paths {
		for _, pt := range p.Pts {
			if !geom.Contains(tri, pt) {
				t.Fatalf("line %d escapes the border at %+v", i, pt)
			}
		}
	}
}

func TestSeparationControlsDensity(t *testing.T) {
	count := func(sep float64) int {
		fr, err := New().Generate(context.Background(), patch.Request{
			Seed: 4, Values: patch.Values{"separation": sep},
		})
		if err != nil {
			t.Fatal(err)
		}
		return len(fr.Paths)
	}
	if tight, loose := count(1.5), count(6); tight <= loose {
		t.Errorf("separation 1.5 gave %d lines, 6 gave %d; tighter should give more", tight, loose)
	}
}

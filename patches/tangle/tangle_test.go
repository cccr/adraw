package tangle

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/geom"
	"github.com/cccr/adraw/pkg/patch"
	"github.com/cccr/adraw/pkg/patch/patchtest"
)

func TestConformance(t *testing.T) {
	patchtest.Verify(t, New())
}

func TestCellSizeControlsDensity(t *testing.T) {
	gen := func(cell float64) int {
		f, err := New().Generate(context.Background(), patch.Request{
			Seed:   1,
			Values: patch.Values{"cell": cell},
		})
		if err != nil {
			t.Fatalf("cell=%v: %v", cell, err)
		}
		return len(f.Paths)
	}
	// Smaller cells mean more of them; each is smaller so holds fewer rings,
	// but the count of cells grows quadratically and wins.
	if small, large := gen(10), gen(40); small <= large {
		t.Errorf("cell=10 gave %d paths, cell=40 gave %d; smaller cells should yield more", small, large)
	}
}

func TestFillProbabilityLeavesCellsEmpty(t *testing.T) {
	gen := func(fill float64) int {
		f, err := New().Generate(context.Background(), patch.Request{
			Seed:   3,
			Values: patch.Values{"fill": fill},
		})
		if err != nil {
			t.Fatal(err)
		}
		return len(f.Paths)
	}
	if none := gen(0); none != 0 {
		t.Errorf("fill=0 produced %d paths, want none", none)
	}
	if half, all := gen(0.5), gen(1); half >= all {
		t.Errorf("fill=0.5 gave %d paths, fill=1 gave %d; expected fewer", half, all)
	}
}

func TestCustomBorderIsRespected(t *testing.T) {
	// A caller-supplied border must override the default page entirely.
	border := []drawing.Vec2{{X: 0, Y: 0}, {X: 80, Y: 0}, {X: 80, Y: 60}, {X: 0, Y: 60}}
	f, err := New().Generate(context.Background(), patch.Request{
		Seed:   5,
		Border: border,
		Values: patch.Values{"cell": 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Paths) == 0 {
		t.Fatal("custom border produced no paths")
	}
	for i, path := range f.Paths {
		for _, pt := range path.Pts {
			if pt.X < -1e-9 || pt.X > 80+1e-9 || pt.Y < -1e-9 || pt.Y > 60+1e-9 {
				t.Fatalf("path %d escapes the custom border at %+v", i, pt)
			}
		}
	}
}

func TestMetaRecordsReproductionInfo(t *testing.T) {
	f, err := New().Generate(context.Background(), patch.Request{Seed: 77})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"patch", "seed", "cell", "spacing", "merge", "fill"} {
		if f.Meta[key] == "" {
			t.Errorf("Meta[%q] is empty; output cannot be reproduced from the file", key)
		}
	}
	if f.Meta["seed"] != "77" {
		t.Errorf("Meta[seed] = %q, want 77", f.Meta["seed"])
	}
}

func TestEveryTessellatorFillCombination(t *testing.T) {
	// patchtest varies one parameter at a time, so it never tries e.g. a fan
	// fill on a Voronoi cell. The combinations are where the geometry actually
	// interacts, so check the whole grid.
	for tess := 0; tess < geom.TessellatorCount; tess++ {
		for style := 0; style < geom.FillCount; style++ {
			name := fmt.Sprintf("tess%d_style%d", tess, style)
			t.Run(name, func(t *testing.T) {
				f, err := New().Generate(context.Background(), patch.Request{
					Seed: 3,
					Values: patch.Values{
						"tess": float64(tess), "style": float64(style), "cell": 30,
					},
				})
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if len(f.Paths) == 0 {
					t.Fatalf("%s produced no paths", name)
				}
				for i, p := range f.Paths {
					if len(p.Pts) < 2 {
						t.Fatalf("%s path %d has %d points", name, i, len(p.Pts))
					}
					for _, pt := range p.Pts {
						if math.IsNaN(pt.X) || math.IsNaN(pt.Y) {
							t.Fatalf("%s produced NaN coordinates", name)
						}
						if !f.Bounds.Contains(pt) {
							t.Fatalf("%s path %d escapes bounds at %+v", name, i, pt)
						}
					}
				}
				t.Logf("%s: %d paths", name, len(f.Paths))
			})
		}
	}
}

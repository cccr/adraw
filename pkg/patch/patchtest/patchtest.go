// Package patchtest provides a conformance suite that every patch runs.
//
// The point is leverage: a new patch gets meaningful test coverage from a
// three-line test file, so the marginal cost of adding patch number twenty is
// no higher than patch number two. Add a check here and every existing patch is
// held to it immediately.
package patchtest

import (
	"context"
	"math"
	"testing"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/patch"
)

// Verify exercises p against every invariant a well-behaved patch must hold.
func Verify(t *testing.T, p patch.Patch) {
	t.Helper()

	t.Run("params", func(t *testing.T) { verifyParams(t, p) })
	t.Run("output", func(t *testing.T) { verifyOutput(t, p) })
	t.Run("determinism", func(t *testing.T) { verifyDeterminism(t, p) })
	t.Run("seed_sensitivity", func(t *testing.T) { verifySeedSensitivity(t, p) })
	t.Run("parameter_extremes", func(t *testing.T) { verifyExtremes(t, p) })
	t.Run("page_size", func(t *testing.T) { verifyPageSize(t, p) })
}

func verifyParams(t *testing.T, p patch.Patch) {
	t.Helper()
	if p.Name() == "" {
		t.Error("Name() is empty")
	}
	if p.Description() == "" {
		t.Error("Description() is empty; UIs have nothing to show alongside the name")
	}
	seen := map[string]bool{}
	for i, s := range p.Params() {
		switch {
		case s.Key == "":
			t.Errorf("param %d has an empty Key", i)
		case seen[s.Key]:
			t.Errorf("param %q is declared twice", s.Key)
		}
		seen[s.Key] = true

		if s.Label == "" {
			t.Errorf("param %q has no Label; UIs have nothing to show", s.Key)
		}
		if s.Desc == "" {
			t.Errorf("param %q has no Desc; a UI cannot explain what it does", s.Key)
		}
		if s.Min > s.Max {
			t.Errorf("param %q has Min %v above Max %v", s.Key, s.Min, s.Max)
		}
		if s.Default < s.Min || s.Default > s.Max {
			t.Errorf("param %q default %v is outside [%v,%v]", s.Key, s.Default, s.Min, s.Max)
		}
	}
}

func verifyOutput(t *testing.T, p patch.Patch) {
	t.Helper()
	f, err := p.Generate(context.Background(), patch.Request{Seed: 1})
	if err != nil {
		t.Fatalf("Generate with defaults: %v", err)
	}
	if len(f.Paths) == 0 {
		t.Fatal("Generate produced no paths")
	}
	if f.Bounds.Width() <= 0 || f.Bounds.Height() <= 0 {
		t.Errorf("Bounds %+v has no extent", f.Bounds)
	}

	for i, path := range f.Paths {
		if len(path.Pts) < 2 {
			t.Errorf("path %d has %d points; a path needs at least 2", i, len(path.Pts))
			continue
		}
		for j, pt := range path.Pts {
			if math.IsNaN(pt.X) || math.IsNaN(pt.Y) || math.IsInf(pt.X, 0) || math.IsInf(pt.Y, 0) {
				t.Fatalf("path %d point %d is not finite: %+v", i, j, pt)
			}
			if !f.Bounds.Contains(pt) {
				t.Errorf("path %d point %d at %+v escapes Bounds %+v", i, j, pt, f.Bounds)
			}
		}
	}
}

func verifyDeterminism(t *testing.T, p patch.Patch) {
	t.Helper()
	req := patch.Request{Seed: 99}
	a, err := p.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	b, err := p.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("second Generate: %v", err)
	}

	if len(a.Paths) != len(b.Paths) {
		t.Fatalf("same request gave %d then %d paths", len(a.Paths), len(b.Paths))
	}
	for i := range a.Paths {
		if len(a.Paths[i].Pts) != len(b.Paths[i].Pts) {
			t.Fatalf("path %d differs in length between identical requests", i)
		}
		for j := range a.Paths[i].Pts {
			if a.Paths[i].Pts[j] != b.Paths[i].Pts[j] {
				t.Fatalf("path %d point %d differs between identical requests", i, j)
			}
		}
	}
}

func verifySeedSensitivity(t *testing.T, p patch.Patch) {
	t.Helper()
	a, err := p.Generate(context.Background(), patch.Request{Seed: 1})
	if err != nil {
		t.Fatalf("Generate seed 1: %v", err)
	}
	b, err := p.Generate(context.Background(), patch.Request{Seed: 2})
	if err != nil {
		t.Fatalf("Generate seed 2: %v", err)
	}

	if len(a.Paths) != len(b.Paths) {
		return // clearly different
	}
	for i := range a.Paths {
		if len(a.Paths[i].Pts) != len(b.Paths[i].Pts) {
			return
		}
		for j := range a.Paths[i].Pts {
			if a.Paths[i].Pts[j] != b.Paths[i].Pts[j] {
				return
			}
		}
	}
	t.Error("different seeds produced identical output; the seed is being ignored")
}

// verifyPageSize checks that a patch draws on the sheet it was given.
//
// Page size is a request field rather than a parameter, so nothing in the
// parameter checks would catch a patch that quietly keeps using its own
// hard-coded A4.
func verifyPageSize(t *testing.T, p patch.Patch) {
	t.Helper()
	for _, size := range []drawing.Size{
		{W: 100, H: 100}, // square
		{W: 420, H: 297}, // A3 landscape
		{W: 148, H: 210}, // A5 portrait
	} {
		f, err := p.Generate(context.Background(), patch.Request{Seed: 6, Page: size})
		if err != nil {
			t.Errorf("page %gx%g: %v", size.W, size.H, err)
			continue
		}
		if f.Bounds.Width() != size.W || f.Bounds.Height() != size.H {
			t.Errorf("page %gx%g produced bounds %gx%g; the request was ignored",
				size.W, size.H, f.Bounds.Width(), f.Bounds.Height())
		}
		if len(f.Paths) == 0 {
			t.Errorf("page %gx%g produced no paths", size.W, size.H)
		}
		for _, path := range f.Paths {
			for _, pt := range path.Pts {
				if !f.Bounds.Contains(pt) {
					t.Errorf("page %gx%g: ink at %+v escapes the sheet", size.W, size.H, pt)
					return
				}
			}
		}
	}
}

func verifyExtremes(t *testing.T, p patch.Patch) {
	t.Helper()
	// A control surface will send the ends of every declared range, so the
	// patch must survive them. This is the check that catches divide-by-zero
	// and runaway loops at a parameter's boundary.
	for _, s := range p.Params() {
		for _, v := range []float64{s.Min, s.Max} {
			req := patch.Request{Seed: 4, Values: patch.Values{s.Key: v}}
			f, err := p.Generate(context.Background(), req)
			if err != nil {
				t.Errorf("Generate with %s=%v: %v", s.Key, v, err)
				continue
			}
			for _, path := range f.Paths {
				for _, pt := range path.Pts {
					if math.IsNaN(pt.X) || math.IsNaN(pt.Y) {
						t.Errorf("%s=%v produced NaN coordinates", s.Key, v)
						return
					}
				}
			}
		}
	}
}

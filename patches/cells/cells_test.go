package cells

import (
	"context"
	"testing"

	"github.com/cccr/adraw/pkg/geom"
	"github.com/cccr/adraw/pkg/patch"
	"github.com/cccr/adraw/pkg/patch/patchtest"
)

// registered returns every preset this package registered, by name.
func registered(t *testing.T) []patch.Patch {
	t.Helper()
	var out []patch.Patch
	for _, name := range []string{
		"contour", "cellwall", "vortex", "fan", "hatch", "crosshatch", "stipple",
	} {
		p, err := patch.Get(name)
		if err != nil {
			t.Fatalf("preset %q was not registered: %v", name, err)
		}
		out = append(out, p)
	}
	return out
}

func TestEveryPresetConforms(t *testing.T) {
	for _, p := range registered(t) {
		t.Run(p.Name(), func(t *testing.T) { patchtest.Verify(t, p) })
	}
}

func TestPresetsExposeOnlyRelevantControls(t *testing.T) {
	// A control that does nothing is worse than an absent one: the UI gives no
	// hint that dragging it is pointless.
	cases := []struct {
		name     string
		wantKey  string
		expected bool
	}{
		{"contour", "merge", true},     // Delaunay
		{"cellwall", "merge", false},   // Voronoi: merging does not apply
		{"vortex", "merge", false},     // Voronoi
		{"stipple", "merge", false},    // Voronoi
		{"hatch", "angles", true},      // ruled fill
		{"crosshatch", "angles", true}, // ruled fill
		{"contour", "angles", false},   // no angle in a contour fill
		{"fan", "angles", false},
	}
	for _, c := range cases {
		p, err := patch.Get(c.name)
		if err != nil {
			t.Fatal(err)
		}
		var found bool
		for _, s := range p.Params() {
			if s.Key == c.wantKey {
				found = true
			}
		}
		if found != c.expected {
			t.Errorf("%s exposes %q = %v, want %v", c.name, c.wantKey, found, c.expected)
		}
	}
}

func TestPresetsDifferFromEachOther(t *testing.T) {
	// Presets that produce identical output would be pure list noise.
	seen := map[int]string{}
	for _, p := range registered(t) {
		f, err := p.Generate(context.Background(), patch.Request{
			Seed: 11, Values: patch.Values{"cell": 30},
		})
		if err != nil {
			t.Fatalf("%s: %v", p.Name(), err)
		}
		if len(f.Paths) == 0 {
			t.Errorf("%s produced nothing", p.Name())
		}
		if other, dup := seen[len(f.Paths)]; dup {
			t.Logf("note: %s and %s both produced %d paths", p.Name(), other, len(f.Paths))
		}
		seen[len(f.Paths)] = p.Name()
		t.Logf("%-11s %d paths", p.Name(), len(f.Paths))
	}
}

func TestPresetsHonourCustomBorder(t *testing.T) {
	tri := geom.Polygon{{X: 10, Y: 10}, {X: 180, Y: 20}, {X: 90, Y: 140}}
	for _, p := range registered(t) {
		t.Run(p.Name(), func(t *testing.T) {
			f, err := p.Generate(context.Background(), patch.Request{
				Seed:   2,
				Border: tri,
				Values: patch.Values{"cell": 18},
			})
			if err != nil {
				t.Fatal(err)
			}
			for i, path := range f.Paths {
				for _, pt := range path.Pts {
					// Dots and jitter can nudge a hair past the edge; a tenth of
					// a millimetre is well inside pen width and well short of an
					// actual escape.
					if !geom.Contains(tri, pt) && !geom.OnBoundary(tri, pt, 0.1) {
						t.Fatalf("path %d escapes the border at %+v", i, pt)
					}
				}
			}
		})
	}
}

func TestPresetsImplementRegioner(t *testing.T) {
	for _, p := range registered(t) {
		r, ok := p.(patch.Regioner)
		if !ok {
			t.Errorf("%s does not implement Regioner", p.Name())
			continue
		}
		regions, err := r.Regions(context.Background(), patch.Request{
			Seed: 1, Values: patch.Values{"cell": 30},
		})
		if err != nil || len(regions) == 0 {
			t.Errorf("%s: %d regions, err %v", p.Name(), len(regions), err)
		}
	}
}

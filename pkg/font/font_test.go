package font

import (
	"math"
	"reflect"
	"testing"
)

func TestElevenFontsWithDistinctNames(t *testing.T) {
	if len(All) != 11 {
		t.Fatalf("%d fonts, want 11", len(All))
	}
	seen := map[string]bool{}
	for _, f := range All {
		if f.Name == "" || seen[f.Name] {
			t.Errorf("bad or duplicate font name %q", f.Name)
		}
		seen[f.Name] = true
	}
}

func TestEveryFontHasASCII(t *testing.T) {
	for _, f := range All {
		for r := rune('!'); r <= '~'; r++ {
			paths, w, _ := f.Layout(string(r), 10)
			if len(paths) == 0 {
				t.Errorf("%s: %q has no strokes", f.Name, r)
			}
			if w <= 0 {
				t.Errorf("%s: %q has no width", f.Name, r)
			}
		}
	}
}

func TestLayoutScalesLinearly(t *testing.T) {
	for _, f := range All {
		_, w1, h1 := f.Layout("Hello, world", 10)
		_, w2, h2 := f.Layout("Hello, world", 20)
		if math.Abs(w2-2*w1) > 1e-9 || math.Abs(h2-2*h1) > 1e-9 {
			t.Errorf("%s: 10 -> %gx%g, 20 -> %gx%g", f.Name, w1, h1, w2, h2)
		}
	}
}

func TestUnknownRunesSkipped(t *testing.T) {
	for _, f := range All {
		a, aw, ah := f.Layout("a😀b", 10)
		b, bw, bh := f.Layout("ab", 10)
		if !reflect.DeepEqual(a, b) || aw != bw || ah != bh {
			t.Errorf("%s: unknown rune changed the layout", f.Name)
		}
	}
}

func TestNoKnownGlyphsHasNoWidth(t *testing.T) {
	for _, f := range All {
		paths, w, _ := f.Layout("😀世界", 10)
		if len(paths) != 0 || w != 0 {
			t.Errorf("%s: got %d paths, w=%g for unknown-only text", f.Name, len(paths), w)
		}
	}
}

func TestLayoutBoundsMatchStrokes(t *testing.T) {
	for _, f := range All {
		paths, w, h := f.Layout("The quick brown fox, jumps over: Q(g)j|~", 12)
		for _, p := range paths {
			for _, pt := range p.Pts {
				if pt.X < -1e-9 || pt.X > w+1e-9 || pt.Y < -1e-9 || pt.Y > h+1e-9 {
					t.Fatalf("%s: point %+v outside [0,%g]x[0,%g]", f.Name, pt, w, h)
				}
			}
		}
	}
}

func TestFontsDiffer(t *testing.T) {
	var prev []interface{}
	for _, f := range All {
		p, _, _ := f.Layout("Typecloud", 10)
		for _, q := range prev {
			if reflect.DeepEqual(p, q) {
				t.Errorf("%s draws identically to an earlier font", f.Name)
			}
		}
		prev = append(prev, p)
	}
}

func TestCapHeightIsSize(t *testing.T) {
	// "H" spans the cap height, so a 10 mm font draws an H about 10 mm tall.
	for _, f := range All {
		paths, _, _ := f.Layout("H", 10)
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, p := range paths {
			for _, pt := range p.Pts {
				lo, hi = math.Min(lo, pt.Y), math.Max(hi, pt.Y)
			}
		}
		if got := hi - lo; got < 8 || got > 12 {
			t.Errorf("%s: H is %g mm tall at size 10", f.Name, got)
		}
	}
}

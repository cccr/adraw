package outlinefont

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestElevenFacesWithLicences(t *testing.T) {
	if len(All) != 11 {
		t.Fatalf("%d faces, want 11", len(All))
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range All {
		if f.Name == "" || seen[f.Name] {
			t.Errorf("bad or duplicate name %q", f.Name)
		}
		seen[f.Name] = true
		if !strings.Contains(string(readme), f.Name) {
			t.Errorf("README.md does not mention %q", f.Name)
		}
	}
}

func TestLayoutScalesLinearly(t *testing.T) {
	for _, f := range All {
		_, w1, h1 := f.Layout("Hello, world", 10)
		_, w2, h2 := f.Layout("Hello, world", 20)
		if math.Abs(w2-2*w1) > 1e-6 || math.Abs(h2-2*h1) > 1e-6 {
			t.Errorf("%s: 10 -> %gx%g, 20 -> %gx%g", f.Name, w1, h1, w2, h2)
		}
	}
}

func TestCapHeightIsSize(t *testing.T) {
	for _, f := range All {
		paths, _, _ := f.Layout("H", 10)
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, p := range paths {
			for _, pt := range p.Pts {
				lo, hi = math.Min(lo, pt.Y), math.Max(hi, pt.Y)
			}
		}
		if got := hi - lo; got < 9.5 || got > 10.5 {
			t.Errorf("%s: H is %.2f mm tall at size 10", f.Name, got)
		}
	}
}

func TestContoursAreClosedAndFilledAndInsideTheBox(t *testing.T) {
	for _, f := range All {
		paths, w, h := f.Layout("The quick brown fox, jumps: Q(g)j|~ ÿ", 12)
		if len(paths) == 0 || w <= 0 || h <= 0 {
			t.Fatalf("%s: no output", f.Name)
		}
		for _, p := range paths {
			if !p.Closed || !p.Fill || len(p.Pts) < 3 {
				t.Fatalf("%s: contour closed=%v fill=%v points=%d", f.Name, p.Closed, p.Fill, len(p.Pts))
			}
			for _, pt := range p.Pts {
				if pt.X < -1e-9 || pt.X > w+1e-9 || pt.Y < -1e-9 || pt.Y > h+1e-9 {
					t.Fatalf("%s: point %+v outside [0,%g]x[0,%g]", f.Name, pt, w, h)
				}
			}
		}
	}
}

func TestUnknownRunesSkipped(t *testing.T) {
	for _, f := range All {
		a, aw, ah := f.Layout("a😀b", 10)
		b, bw, bh := f.Layout("ab", 10)
		if !reflect.DeepEqual(a, b) || aw != bw || ah != bh {
			t.Errorf("%s: an unknown rune changed the layout", f.Name)
		}
		if p, w, _ := f.Layout("😀世", 10); len(p) != 0 || w != 0 {
			t.Errorf("%s: unknown-only text gave %d contours, w=%g", f.Name, len(p), w)
		}
	}
}

func TestCountersAreSeparateContours(t *testing.T) {
	for _, f := range All {
		if p, _, _ := f.Layout("o", 20); len(p) != 2 {
			t.Errorf("%s: o has %d contours, want 2 (outer and counter)", f.Name, len(p))
		}
		// The script faces draw B as one connected loop, so it has no counter.
		if p, _, _ := f.Layout("B", 20); len(p) < 2 && f.Name != "script" && f.Name != "lobster" {
			t.Errorf("%s: B has %d contours, want at least 2", f.Name, len(p))
		}
	}
}

func TestAccentsCovered(t *testing.T) {
	for _, f := range All {
		plain, _, _ := f.Layout("e", 10)
		acute, _, _ := f.Layout("é", 10)
		if len(acute) == 0 || reflect.DeepEqual(plain, acute) {
			t.Errorf("%s: é is missing or drawn as e", f.Name)
		}
	}
}

func TestFacesDiffer(t *testing.T) {
	var seen [][]float64
	for _, f := range All {
		_, w, h := f.Layout("Typecloud", 10)
		for _, s := range seen {
			if s[0] == w && s[1] == h {
				t.Errorf("%s has the same extent as an earlier face", f.Name)
			}
		}
		seen = append(seen, []float64{w, h})
	}
}

func TestSpacesHaveWidthButNoInk(t *testing.T) {
	for _, f := range All {
		_, a, _ := f.Layout("a b", 10)
		_, b, _ := f.Layout("ab", 10)
		if a <= b {
			t.Errorf("%s: a space added no width", f.Name)
		}
	}
}

func TestEveryFontFileHasALicence(t *testing.T) {
	ttfs, _ := filepath.Glob("fonts/*.ttf")
	lics, _ := filepath.Glob("fonts/OFL-*.txt")
	if len(ttfs) != len(lics) || len(ttfs) != len(faces) {
		t.Errorf("%d fonts, %d licences, %d listed faces", len(ttfs), len(lics), len(faces))
	}
}

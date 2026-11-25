package typecloud

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/outlinefont"
	"github.com/cccr/adraw/pkg/patch"
	"github.com/cccr/adraw/pkg/patch/patchtest"
)

func TestConformance(t *testing.T) {
	patchtest.Verify(t, New())
}

func gen(t *testing.T, req patch.Request) drawing.Frame {
	t.Helper()
	f, err := New().Generate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDeterministicAndSeeded(t *testing.T) {
	req := patch.Request{Seed: 7, Strings: patch.Strings{"text": "alpha;beta;gamma"}}
	a, b := gen(t, req), gen(t, req)
	if !reflect.DeepEqual(a.Paths, b.Paths) {
		t.Error("same request, different drawing")
	}
	req.Seed = 8
	if reflect.DeepEqual(a.Paths, gen(t, req).Paths) {
		t.Error("seed ignored")
	}
}

func TestEmptyTextUsesDefault(t *testing.T) {
	for _, in := range []string{"", "  ", ";;;", "😀", "😀 😀", "日本 語", "é é", "😀;日本 語"} {
		f := gen(t, patch.Request{Seed: 1, Strings: patch.Strings{"text": in}})
		if len(f.Paths) == 0 {
			t.Errorf("text %q gave a blank page", in)
		}
	}
}

func TestNoOverlapAndInsidePage(t *testing.T) {
	const gap = 3.0
	req := patch.Request{Seed: 3,
		Strings: patch.Strings{"text": "Typecloud;pen plotter;ink;lines of text"},
		Values:  patch.Values{"gap": gap}}
	pl, bounds, err := New().place(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl) < 4 {
		t.Fatalf("only %d placements for 4 chunks", len(pl))
	}
	type pt = drawing.Vec2
	sample := func(p placement) []pt {
		var out []pt
		for _, path := range p.paths {
			for i := 1; i < len(path.Pts); i++ {
				a, b := path.Pts[i-1], path.Pts[i]
				n := int(math.Hypot(b.X-a.X, b.Y-a.Y)/0.5) + 1
				for k := 0; k <= n; k++ {
					f := float64(k) / float64(n)
					out = append(out, pt{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f})
				}
			}
		}
		return out
	}
	pts := make([][]pt, len(pl))
	for i, p := range pl {
		pts[i] = sample(p)
		for _, q := range pts[i] {
			if !bounds.Contains(q) {
				t.Fatalf("placement %d ink %+v outside page %+v", i, q, bounds)
			}
		}
	}
	// Mask packing works on a grid, so allow one cell of slack on the gap.
	want := gap - 1.0
	for i := range pl {
		for j := i + 1; j < len(pl); j++ {
			a, b := pl[i], pl[j]
			if a.x > b.x+b.w+gap || b.x > a.x+a.w+gap || a.y > b.y+b.h+gap || b.y > a.y+a.h+gap {
				continue // far apart: boxes alone prove it
			}
			for _, p := range pts[i] {
				for _, q := range pts[j] {
					if math.Hypot(p.X-q.X, p.Y-q.Y) < want {
						t.Fatalf("placements %d and %d have ink %.2f mm apart, want >= %.2f", i, j, math.Hypot(p.X-q.X, p.Y-q.Y), want)
					}
				}
			}
		}
	}
}

func TestChunksDifferInFontAndSize(t *testing.T) {
	pl, _, err := New().place(context.Background(), patch.Request{Seed: 2,
		Strings: patch.Strings{"text": "one;two;three;four;five"}})
	if err != nil {
		t.Fatal(err)
	}
	fonts, sizes := map[string]bool{}, map[float64]bool{}
	for _, p := range pl {
		fonts[p.name] = true
		sizes[math.Round(p.size)] = true
	}
	if len(fonts) < 3 {
		t.Errorf("only %d fonts used: %v", len(fonts), fonts)
	}
	if len(sizes) < 3 {
		t.Errorf("only %d distinct sizes", len(sizes))
	}
}

func TestFontsParamLimitsFonts(t *testing.T) {
	pl, _, _ := New().place(context.Background(), patch.Request{Seed: 2,
		Strings: patch.Strings{"text": "one;two;three;four;five"},
		Values:  patch.Values{"fonts": 1}})
	for _, p := range pl {
		if p.name != pl[0].name {
			t.Fatal("fonts=1 still used several fonts")
		}
	}
}

func TestLargestFirstChunkAtMaxSize(t *testing.T) {
	pl, _, _ := New().place(context.Background(), patch.Request{Seed: 2,
		Strings: patch.Strings{"text": "ab;cd;ef"}, Values: patch.Values{"max": 30, "min": 5}})
	biggest := 0.0
	for _, p := range pl {
		biggest = math.Max(biggest, p.size)
	}
	if biggest < 29 || biggest > 30.0001 {
		t.Errorf("largest placement is %g mm, want max (30)", biggest)
	}
}

func TestRepeatsFillLeftoverSpace(t *testing.T) {
	few := func(fill float64) int {
		pl, _, _ := New().place(context.Background(), patch.Request{Seed: 2,
			Strings: patch.Strings{"text": "one;two"}, Values: patch.Values{"fill": fill}})
		return len(pl)
	}
	if low, high := few(0.1), few(0.9); high <= low {
		t.Errorf("fill 0.9 gave %d placements, fill 0.1 gave %d; repeats should add more", high, low)
	}
}

func TestHugeInputsAreBounded(t *testing.T) {
	long := ""
	for i := 0; i < 5000; i++ {
		long += "x"
	}
	many := ""
	for i := 0; i < 1500; i++ {
		many += "word;"
	}
	for name, req := range map[string]patch.Request{
		"one 5000-char chunk": {Seed: 1, Strings: patch.Strings{"text": long}},
		"1500 chunks":         {Seed: 1, Strings: patch.Strings{"text": many}},
		"tiny page":           {Seed: 1, Page: drawing.Size{W: 10, H: 10}},
	} {
		start := time.Now()
		f := gen(t, req)
		if d := time.Since(start); d > 3*time.Second {
			t.Errorf("%s took %v", name, d)
		}
		if len(f.Paths) == 0 {
			t.Errorf("%s: no paths", name)
		}
	}
}

func TestMinEqualsMax(t *testing.T) {
	f := gen(t, patch.Request{Seed: 1, Values: patch.Values{"min": 10, "max": 10}})
	if len(f.Paths) == 0 {
		t.Error("no paths")
	}
}

func TestBorderConfinesInk(t *testing.T) {
	border := []drawing.Vec2{{X: 50, Y: 50}, {X: 150, Y: 50}, {X: 100, Y: 150}}
	f := gen(t, patch.Request{Seed: 1, Border: border})
	for _, p := range f.Paths {
		for _, q := range p.Pts {
			if !pointInTri(q, border) {
				t.Fatalf("ink %+v outside the triangular border", q)
			}
		}
	}
}

func pointInTri(p drawing.Vec2, tri []drawing.Vec2) bool {
	sign := func(a, b, c drawing.Vec2) float64 { return (a.X-c.X)*(b.Y-c.Y) - (b.X-c.X)*(a.Y-c.Y) }
	d1, d2, d3 := sign(p, tri[0], tri[1]), sign(p, tri[1], tri[2]), sign(p, tri[2], tri[0])
	neg := d1 < 0 || d2 < 0 || d3 < 0
	pos := d1 > 0 || d2 > 0 || d3 > 0
	return !(neg && pos)
}

func TestContextCancelStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().Generate(ctx, patch.Request{Seed: 1}); err == nil {
		t.Error("cancelled context should return an error")
	}
}

// The gap is a promise to the pen: ink of two chunks never closer than gap. It
// must hold on a big sheet too, where the grid gets coarser to stay small.
func TestGapHoldsOnLargePages(t *testing.T) {
	for _, c := range []struct {
		page drawing.Size
		gap  float64
	}{{drawing.Size{W: 297, H: 210}, 0.5}, {drawing.Size{W: 1200, H: 1200}, 0.5}, {drawing.Size{W: 2000, H: 2000}, 0.5}} {
		req := patch.Request{Seed: 3, Page: c.page,
			Strings: patch.Strings{"text": "Typecloud;pen plotter;ink;lines of text"},
			Values:  patch.Values{"gap": c.gap, "fill": 0.3, "min": 10, "max": 60}}
		pl, _, err := New().place(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		type pt = drawing.Vec2
		sample := func(p placement) []pt {
			var out []pt
			for _, path := range p.paths {
				for i := 1; i < len(path.Pts); i++ {
					a, b := path.Pts[i-1], path.Pts[i]
					n := int(math.Hypot(b.X-a.X, b.Y-a.Y)/0.25) + 1
					for k := 0; k <= n; k++ {
						f := float64(k) / float64(n)
						out = append(out, pt{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f})
					}
				}
			}
			return out
		}
		for i := range pl {
			for j := i + 1; j < len(pl); j++ {
				a, b := pl[i], pl[j]
				if a.x > b.x+b.w+1 || b.x > a.x+a.w+1 || a.y > b.y+b.h+1 || b.y > a.y+a.h+1 {
					continue
				}
				// Only ink inside the other chunk's box (plus the gap) can be too close.
				near := func(pts []pt, o placement) []pt {
					var out []pt
					for _, p := range pts {
						if p.X >= o.x-c.gap && p.X <= o.x+o.w+c.gap && p.Y >= o.y-c.gap && p.Y <= o.y+o.h+c.gap {
							out = append(out, p)
						}
					}
					return out
				}
				pa, pb := near(sample(a), b), near(sample(b), a)
				for _, p := range pa {
					for _, q := range pb {
						if d := math.Hypot(p.X-q.X, p.Y-q.Y); d < c.gap-1e-6 {
							t.Fatalf("page %v gap %g: placements %d and %d have ink %.3f mm apart", c.page, c.gap, i, j, d)
						}
					}
				}
			}
		}
	}
}

func solid(seed int64, extra patch.Values) patch.Request {
	v := patch.Values{"style": 1}
	for k, x := range extra {
		v[k] = x
	}
	return patch.Request{Seed: seed, Values: v,
		Strings: patch.Strings{"text": "Typecloud;pen plotter;ink;lines of text"}}
}

func TestStyleOneUsesRegularFontsAndFillsPaths(t *testing.T) {
	pl, _, err := New().place(context.Background(), solid(2, nil))
	if err != nil {
		t.Fatal(err)
	}
	regular := map[string]bool{}
	for _, f := range outlinefont.All {
		regular[f.Name] = true
	}
	if len(pl) < 4 {
		t.Fatalf("only %d placements", len(pl))
	}
	for _, p := range pl {
		if !regular[p.name] {
			t.Errorf("placement uses %q, not a regular font", p.name)
		}
		for _, path := range p.paths {
			if !path.Closed || !path.Fill {
				t.Fatalf("a %s contour is closed=%v fill=%v", p.name, path.Closed, path.Fill)
			}
		}
	}
}

func TestStyleOneDeterministicAndInsidePage(t *testing.T) {
	a, err := New().Generate(context.Background(), solid(5, nil))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := New().Generate(context.Background(), solid(5, nil))
	if !reflect.DeepEqual(a.Paths, b.Paths) {
		t.Error("same request, different drawing")
	}
	if len(a.Paths) == 0 {
		t.Fatal("no paths")
	}
	for _, p := range a.Paths {
		for _, q := range p.Pts {
			if !a.Bounds.Contains(q) {
				t.Fatalf("ink %+v outside the page", q)
			}
		}
	}
}

// Solid letters keep the same promise to the pen as line letters: ink of two
// chunks is never closer than the gap.
func TestSolidShapesKeepGap(t *testing.T) {
	const gap = 2.0
	pl, _, err := New().place(context.Background(), solid(3, patch.Values{"gap": gap, "fill": 0.4}))
	if err != nil {
		t.Fatal(err)
	}
	type pt = drawing.Vec2
	sample := func(p placement) []pt {
		var out []pt
		for _, path := range p.paths {
			n := len(path.Pts)
			for i := 0; i < n; i++ {
				a, b := path.Pts[i], path.Pts[(i+1)%n]
				k := int(math.Hypot(b.X-a.X, b.Y-a.Y)/0.25) + 1
				for j := 0; j < k; j++ {
					f := float64(j) / float64(k)
					out = append(out, pt{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f})
				}
			}
		}
		return out
	}
	for i := range pl {
		for j := i + 1; j < len(pl); j++ {
			a, b := pl[i], pl[j]
			if a.x > b.x+b.w+gap || b.x > a.x+a.w+gap || a.y > b.y+b.h+gap || b.y > a.y+a.h+gap {
				continue
			}
			for _, p := range sample(a) {
				if p.X < b.x-gap || p.X > b.x+b.w+gap || p.Y < b.y-gap || p.Y > b.y+b.h+gap {
					continue
				}
				for _, q := range sample(b) {
					if d := math.Hypot(p.X-q.X, p.Y-q.Y); d < gap-1e-6 {
						t.Fatalf("placements %d and %d: ink %.3f mm apart, gap is %g", i, j, d, gap)
					}
				}
			}
		}
	}
}

// Packing on the real letter shapes, not boxes, is the point of a word cloud.
func TestStyleOnePacksDense(t *testing.T) {
	pl, bounds, err := New().place(context.Background(), solid(1, nil))
	if err != nil {
		t.Fatal(err)
	}
	box := 0.0
	for _, p := range pl {
		box += p.w * p.h
	}
	if cover := box / (bounds.Width() * bounds.Height()); cover < 0.35 {
		t.Errorf("line boxes cover only %.0f%% of the page", cover*100)
	}
}

func TestSolidMaskIncludesTheInsideOfLetters(t *testing.T) {
	// An "I" in a heavy face is a filled bar: its mask must contain the cell at
	// its centre, which an outline-only mask would leave free.
	paths, w, h := outlinefont.All[3].Layout("I", 40) // "fat"
	m := rasteriseFilled(paths, 1, 0.5)
	has := func(dx, dy int) bool {
		for _, c := range m.cells {
			if c.dx == dx && c.dy == dy {
				return true
			}
		}
		return false
	}
	if !has(int(w/2), int(h/2)) {
		t.Error("the centre of a solid I is not in its mask")
	}
}

func TestStyleOneFallsBackWithoutRegularFonts(t *testing.T) {
	saved := outlinefont.All
	outlinefont.All = nil
	defer func() { outlinefont.All = saved }()
	f, err := New().Generate(context.Background(), solid(1, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Paths) == 0 {
		t.Error("no drawing when no regular font loaded")
	}
	for _, p := range f.Paths {
		if p.Fill {
			t.Fatal("fell back but still produced filled paths")
		}
	}
}

func TestFontsSliderGoesToEleven(t *testing.T) {
	if s := spec("fonts"); s.Max != 11 || s.Min != 1 {
		t.Errorf("fonts range is %g..%g, want 1..11", s.Min, s.Max)
	}
}

// With eleven fonts asked for and enough chunks, all eleven are used, in either
// mode, and no two chunks of the first eleven share one.
func TestElevenFontsAreAllUsable(t *testing.T) {
	text := "one;two;three;four;five;six;seven;eight;nine;ten;eleven"
	for _, style := range []float64{0, 1} {
		pl, _, err := New().place(context.Background(), patch.Request{Seed: 4,
			Strings: patch.Strings{"text": text},
			Values:  patch.Values{"fonts": 11, "style": style, "fill": 0, "max": 20, "min": 4}})
		if err != nil {
			t.Fatal(err)
		}
		used := map[string]bool{}
		for _, p := range pl {
			used[p.name] = true
		}
		if len(used) != 11 {
			t.Errorf("style %g: %d fonts used, want 11: %v", style, len(used), used)
		}
	}
}

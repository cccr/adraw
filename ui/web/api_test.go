package web

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	_ "github.com/cccr/adraw/patches"
	_ "github.com/cccr/adraw/renderers"
)

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestPatchesDescribeThemselves(t *testing.T) {
	rec := get(t, "/api/patches")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}

	var got []PatchInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("got %d patches, want the registered ones", len(got))
	}

	// A client builds its entire UI from this payload, so every field it needs
	// must be present for every parameter of every patch.
	for _, p := range got {
		if p.Name == "" || p.Description == "" {
			t.Errorf("patch %+v is missing name or description", p)
		}
		if len(p.Params) == 0 {
			t.Errorf("patch %q exposes no parameters", p.Name)
		}
		for _, s := range p.Params {
			if s.Key == "" || s.Label == "" || s.Description == "" {
				t.Errorf("patch %q param %+v is missing text", p.Name, s)
			}
			if s.Min > s.Max || s.Default < s.Min || s.Default > s.Max {
				t.Errorf("patch %q param %q has an incoherent range", p.Name, s.Key)
			}
		}
	}
}

func TestRenderReturnsGeometry(t *testing.T) {
	rec := get(t, "/api/render?patch=tangle&seed=3&cell=40")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	var f FrameJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &f); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if f.Width <= 0 || f.Height <= 0 {
		t.Errorf("frame has no extent: %gx%g", f.Width, f.Height)
	}
	if len(f.Paths) == 0 {
		t.Fatal("no paths returned")
	}
	if len(f.Closed) != len(f.Paths) {
		t.Errorf("%d paths but %d closed flags", len(f.Paths), len(f.Closed))
	}
	for i, p := range f.Paths {
		if len(p)%2 != 0 {
			t.Fatalf("path %d has an odd coordinate count %d", i, len(p))
		}
		if len(p) < 4 {
			t.Errorf("path %d has fewer than 2 points", i)
		}
	}
	if f.Meta["cell"] != "40" {
		t.Errorf("meta did not record the request: %v", f.Meta)
	}
}

func TestNormalizedParametersMapOntoRange(t *testing.T) {
	// This is the contract a MIDI CC or CV input relies on: send 0..1 and the
	// server maps it onto whatever range the patch declared, so the client
	// never needs to know a patch's units or bounds.
	cases := []struct{ query, wantCell string }{
		{"n.cell=0", "5"},
		{"n.cell=1", "60"},
		{"n.cell=0.5", "32.5"},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			rec := get(t, "/api/render?patch=tangle&seed=1&"+c.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			var f FrameJSON
			json.Unmarshal(rec.Body.Bytes(), &f)
			if f.Meta["cell"] != c.wantCell {
				t.Errorf("cell = %s, want %s", f.Meta["cell"], c.wantCell)
			}
		})
	}
}

func TestExportUsesRealRenderers(t *testing.T) {
	svg := get(t, "/api/export?patch=tangle&seed=2&format=svg")
	if svg.Code != http.StatusOK {
		t.Fatalf("svg status %d: %s", svg.Code, svg.Body)
	}
	if !strings.HasPrefix(svg.Body.String(), "<svg") {
		t.Error("svg export is not an SVG document")
	}
	if ct := svg.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("svg content type = %q", ct)
	}
	if cd := svg.Header().Get("Content-Disposition"); !strings.Contains(cd, "tangle-2.svg") {
		t.Errorf("download filename = %q", cd)
	}

	nc := get(t, "/api/export?patch=tangle&seed=2&format=gcode")
	if nc.Code != http.StatusOK {
		t.Fatalf("gcode status %d", nc.Code)
	}
	if !strings.Contains(nc.Body.String(), "G21") {
		t.Error("gcode export is missing its preamble")
	}
}

func TestBadRequestsAreRejected(t *testing.T) {
	cases := []struct{ name, path string }{
		{"missing patch", "/api/render"},
		{"unknown patch", "/api/render?patch=nope"},
		{"unknown parameter", "/api/render?patch=tangle&bogus=1"},
		{"non-numeric value", "/api/render?patch=tangle&cell=big"},
		{"non-integer seed", "/api/render?patch=tangle&seed=1.5"},
		{"unknown format", "/api/export?patch=tangle&format=pdf"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, c.path)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status %d, want 400", rec.Code)
			}
			var body map[string]string
			json.Unmarshal(rec.Body.Bytes(), &body)
			if body["error"] == "" {
				t.Error("no error message returned")
			}
		})
	}
}

func TestOutOfRangeValuesAreClamped(t *testing.T) {
	// A slider cannot exceed its range but a hand-edited URL can. Clamping
	// keeps a shared link working rather than failing it.
	rec := get(t, "/api/render?patch=tangle&seed=1&cell=99999")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var f FrameJSON
	json.Unmarshal(rec.Body.Bytes(), &f)
	if f.Meta["cell"] != "60" {
		t.Errorf("cell = %s, want it clamped to 60", f.Meta["cell"])
	}
}

func TestUIIsServed(t *testing.T) {
	for _, path := range []string{"/", "/app.js", "/style.css"} {
		rec := get(t, path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s returned %d", path, rec.Code)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s is cacheable; edits would serve stale", path)
		}
	}
}

func TestBorderParameter(t *testing.T) {
	// A triangle well inside the default page: all ink must land within it.
	const border = "20,20,200,30,100,150"
	rec := get(t, "/api/render?patch=tangle&seed=4&cell=20&border="+border)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	var f FrameJSON
	json.Unmarshal(rec.Body.Bytes(), &f)
	if len(f.Paths) == 0 {
		t.Fatal("custom border produced no paths")
	}
	tri := [][2]float64{{20, 20}, {200, 30}, {100, 150}}
	for i, p := range f.Paths {
		for j := 0; j < len(p); j += 2 {
			if !inTriangle(p[j], p[j+1], tri) {
				t.Fatalf("path %d point %d at (%g,%g) is outside the border", i, j/2, p[j], p[j+1])
			}
		}
	}
}

// inTriangle reports whether (x,y) is inside t, tolerating points that sit on
// an edge.
//
// The tolerance has to be a real distance, not a raw cross product: the cross
// product scales with edge length, so on a 150mm edge the API's 0.001mm
// coordinate rounding shows up as 0.15. Dividing by the edge length converts it
// back to millimetres, where a 0.01mm slack is obviously generous and
// obviously not hiding a genuine escape. Boundary points are expected here —
// PoissonDisk seeds the border deliberately.
func inTriangle(x, y float64, t [][2]float64) bool {
	const tolMM = 0.01

	dist := func(a, b [2]float64) float64 {
		cross := (x-b[0])*(a[1]-b[1]) - (a[0]-b[0])*(y-b[1])
		edge := math.Hypot(a[0]-b[0], a[1]-b[1])
		if edge == 0 {
			return cross
		}
		return cross / edge
	}
	d1, d2, d3 := dist(t[0], t[1]), dist(t[1], t[2]), dist(t[2], t[0])
	neg := d1 < -tolMM || d2 < -tolMM || d3 < -tolMM
	pos := d1 > tolMM || d2 > tolMM || d3 > tolMM
	return !(neg && pos)
}

func TestMalformedBorderIsRejected(t *testing.T) {
	for _, c := range []struct{ name, border string }{
		{"odd coordinate count", "1,2,3"},
		{"too few points", "1,2,3,4"},
		{"non-numeric", "1,2,3,4,five,6"},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, "/api/render?patch=tangle&border="+c.border)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status %d, want 400", rec.Code)
			}
		})
	}
}

func TestExportOptimizesTravelButRenderDoesNot(t *testing.T) {
	// Downloads are for plotting and should be reordered; the live preview is
	// not, because paying for the pass on every slider tick would cost the
	// interactivity that makes the web UI worth having.
	exp := get(t, "/api/export?patch=tangle&seed=1&format=svg")
	if exp.Code != http.StatusOK {
		t.Fatalf("export status %d", exp.Code)
	}
	if !strings.Contains(exp.Body.String(), "travel_after_mm") {
		t.Error("export was not travel-optimized")
	}

	prev := get(t, "/api/render?patch=tangle&seed=1")
	var f FrameJSON
	json.Unmarshal(prev.Body.Bytes(), &f)
	if _, ok := f.Meta["travel_after_mm"]; ok {
		t.Error("preview was travel-optimized; that cost belongs only on export")
	}
}

// patchNames lists every registered patch, via the public API.
func patchNames(t *testing.T) []string {
	t.Helper()
	var infos []PatchInfo
	if err := json.Unmarshal(get(t, "/api/patches").Body.Bytes(), &infos); err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(infos))
	for i, p := range infos {
		out[i] = p.Name
	}
	return out
}

func TestPrintRulesAreServed(t *testing.T) {
	// A light guard on the print behaviour, which cannot be exercised headlessly:
	// if these rules are dropped, printing silently reverts to putting the whole
	// interface on the page.
	css := get(t, "/style.css").Body.String()
	for _, want := range []string{
		"@media print",
		"#print-area",
		"#header,", // chrome must be hidden
		"max-width: 100%",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("style.css is missing %q", want)
		}
	}

	js := get(t, "/app.js").Body.String()
	for _, want := range []string{
		"beforeprint", // builds the SVG lazily
		"afterprint",  // and tears it down
		"frameToSVG",
		"@page",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("app.js is missing %q", want)
		}
	}
}

func TestPageSizeParameter(t *testing.T) {
	cases := []struct {
		page string
		w, h float64
	}{
		{"297x210", 297, 210}, // A4 landscape
		{"210x297", 210, 297}, // rotated
		{"210x210", 210, 210}, // square
		{"420x297", 420, 297}, // A3
		{"297X210", 297, 210}, // case-insensitive separator
	}
	for _, c := range cases {
		t.Run(c.page, func(t *testing.T) {
			rec := get(t, "/api/render?patch=tangle&seed=1&page="+c.page)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			var f FrameJSON
			json.Unmarshal(rec.Body.Bytes(), &f)
			if f.Width != c.w || f.Height != c.h {
				t.Errorf("got %gx%g, want %gx%g", f.Width, f.Height, c.w, c.h)
			}
			for i, p := range f.Paths {
				for j := 0; j < len(p); j += 2 {
					if p[j] < 0 || p[j] > c.w || p[j+1] < 0 || p[j+1] > c.h {
						t.Fatalf("path %d point %d at (%g,%g) escapes the sheet",
							i, j/2, p[j], p[j+1])
					}
				}
			}
		})
	}
}

func TestPageSizeIsNotAPatchParameter(t *testing.T) {
	// Page belongs to the request, not to the parameter list: it is a property
	// of the paper, and it must never appear as something a control surface
	// could map a knob onto.
	var infos []PatchInfo
	json.Unmarshal(get(t, "/api/patches").Body.Bytes(), &infos)
	for _, p := range infos {
		for _, s := range p.Params {
			if s.Key == "page" || s.Key == "width" || s.Key == "height" {
				t.Errorf("patch %q exposes %q as a parameter", p.Name, s.Key)
			}
		}
	}
}

func TestPageSizeRespectsMargin(t *testing.T) {
	// The reason Page exists alongside Border: supplying a border overrides the
	// patch's margin, so routing page size through Border would have silently
	// killed the margin control.
	const page, margin = "200x200", 25.0
	rec := get(t, fmt.Sprintf("/api/render?patch=tangle&seed=1&page=%s&margin=%g&cell=20",
		page, margin))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var f FrameJSON
	json.Unmarshal(rec.Body.Bytes(), &f)

	// Bounds are the full sheet; the ink sits inside the margin.
	if f.Width != 200 || f.Height != 200 {
		t.Fatalf("bounds %gx%g, want the full 200x200 sheet", f.Width, f.Height)
	}
	minX, minY := math.Inf(1), math.Inf(1)
	for _, p := range f.Paths {
		for j := 0; j < len(p); j += 2 {
			minX, minY = math.Min(minX, p[j]), math.Min(minY, p[j+1])
		}
	}
	if minX < margin-0.01 || minY < margin-0.01 {
		t.Errorf("ink starts at (%.2f,%.2f); the %gmm margin was not applied", minX, minY, margin)
	}
}

func TestMalformedPageIsRejected(t *testing.T) {
	for _, c := range []struct{ name, page string }{
		{"no separator", "297"},
		{"non-numeric", "wide x tall"},
		{"too small", "5x5"},
		{"too large", "9000x9000"},
		{"negative", "-100x200"},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, "/api/render?patch=tangle&page="+url.QueryEscape(c.page))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status %d, want 400", rec.Code)
			}
		})
	}
}

func TestPageAppliesToEveryPatch(t *testing.T) {
	for _, name := range patchNames(t) {
		t.Run(name, func(t *testing.T) {
			rec := get(t, "/api/render?patch="+name+"&seed=2&page=180x240")
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			var f FrameJSON
			json.Unmarshal(rec.Body.Bytes(), &f)
			if f.Width != 180 || f.Height != 240 {
				t.Errorf("%s produced %gx%g, want 180x240", name, f.Width, f.Height)
			}
		})
	}
}

// Package web serves the HTTP API and the browser UI.
//
// The JSON API is the universal contract: every future control surface — MIDI,
// CV, another program — speaks it, and the browser page is simply its first
// client. Two rules follow from that:
//
//   - The API returns data, never a rendered format. Turning a Frame into SVG
//     or G-code is the renderer layer's job, exposed separately at /api/export.
//     Baking one renderer's output into /api/render would hand a MIDI client
//     XML it has no use for.
//   - Patches describe themselves. /api/patches is assembled from each patch's
//     own ParamSpec list, so the server holds no per-patch knowledge and a new
//     patch appears in every client the moment it is registered.
package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/optimize"
	"github.com/cccr/adraw/pkg/patch"
	"github.com/cccr/adraw/pkg/render"
)

// PatchInfo is one patch as the API describes it.
type PatchInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Params      []ParamInfo `json:"params"`
}

// ParamInfo is one control. Min, Max and Step are what a client needs to build
// a slider; Unit and Desc are what it needs to label one.
type ParamInfo struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Default     float64 `json:"default"`
	Step        float64 `json:"step"`
	Unit        string  `json:"unit"`
}

// RendererInfo is one output format.
type RendererInfo struct {
	Name string `json:"name"`
	Ext  string `json:"ext"`
}

// FrameJSON is a generated drawing.
//
// Paths are flat [x0,y0,x1,y1,...] arrays rather than arrays of objects: for a
// dense drawing that is tens of thousands of numbers, and {"x":..,"y":..} per
// point would roughly triple the payload for no gain.
type FrameJSON struct {
	Width  float64           `json:"width"`
	Height float64           `json:"height"`
	MinX   float64           `json:"minX"`
	MinY   float64           `json:"minY"`
	Paths  [][]float64       `json:"paths"`
	Closed []bool            `json:"closed"`
	Meta   map[string]string `json:"meta"`
}

// Handler returns the mux serving the API and, if assets are present, the UI.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/gallery", handleGallery)
	mux.HandleFunc("/api/patches", handlePatches)
	mux.HandleFunc("/api/renderers", handleRenderers)
	mux.HandleFunc("/api/render", handleRender)
	mux.HandleFunc("/api/export", handleExport)
	mux.Handle("/", staticHandler())
	return mux
}

func handlePatches(w http.ResponseWriter, r *http.Request) {
	out := make([]PatchInfo, 0, len(patch.Names()))
	for _, name := range patch.Names() {
		p, err := patch.Get(name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		info := PatchInfo{Name: p.Name(), Description: p.Description()}
		for _, s := range p.Params() {
			info.Params = append(info.Params, ParamInfo{
				Key: s.Key, Label: s.Label, Description: s.Desc,
				Min: s.Min, Max: s.Max, Default: s.Default,
				Step: s.Step, Unit: s.Unit,
			})
		}
		out = append(out, info)
	}
	writeJSON(w, out)
}

func handleRenderers(w http.ResponseWriter, r *http.Request) {
	out := make([]RendererInfo, 0)
	for _, name := range render.Names() {
		rd, err := render.Get(name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		out = append(out, RendererInfo{Name: rd.Name(), Ext: rd.Ext()})
	}
	writeJSON(w, out)
}

func handleRender(w http.ResponseWriter, r *http.Request) {
	frame, _, err := generate(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	out := FrameJSON{
		Width:  frame.Bounds.Width(),
		Height: frame.Bounds.Height(),
		MinX:   frame.Bounds.Min.X,
		MinY:   frame.Bounds.Min.Y,
		Paths:  make([][]float64, 0, len(frame.Paths)),
		Closed: make([]bool, 0, len(frame.Paths)),
		Meta:   frame.Meta,
	}
	for _, p := range frame.Paths {
		flat := make([]float64, 0, len(p.Pts)*2)
		for _, pt := range p.Pts {
			flat = append(flat, round(pt.X), round(pt.Y))
		}
		out.Paths = append(out.Paths, flat)
		out.Closed = append(out.Closed, p.Closed)
	}
	writeJSON(w, out)
}

// handleExport runs a real renderer, so a download is byte-identical to what
// the CLI would have produced for the same parameters.
//
// Travel is optimized here but deliberately not in handleRender: the canvas
// preview does not care what order the strokes arrive in, and paying for the
// pass on every slider tick would spoil the live feel that makes the web UI
// worth having.
func handleExport(w http.ResponseWriter, r *http.Request) {
	frame, name, err := generate(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	frame = optimize.Reorder(frame)

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "svg"
	}
	rd, err := render.Get(format)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	seed := r.URL.Query().Get("seed")
	if seed == "" {
		seed = "1"
	}
	filename := fmt.Sprintf("%s-%s.%s", name, seed, rd.Ext())
	w.Header().Set("Content-Type", contentType(rd.Ext()))
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	if err := rd.Render(w, frame); err != nil {
		writeError(w, http.StatusInternalServerError, err)
	}
}

// generate parses a request into a patch.Request and runs it.
//
// Parameters arrive either as their real value (cell=26) or normalized
// (n.cell=0.4). The normalized form is what a MIDI CC or a CV input naturally
// produces, and ParamSpec.FromNormalized maps it onto the declared range, so
// such a client never needs to know a patch's units or bounds.
func generate(r *http.Request) (drawing.Frame, string, error) {
	q := r.URL.Query()

	name := q.Get("patch")
	if name == "" {
		return drawing.Frame{}, "", fmt.Errorf("missing required parameter: patch")
	}
	p, err := patch.Get(name)
	if err != nil {
		return drawing.Frame{}, "", err
	}

	var seed int64 = 1
	if s := q.Get("seed"); s != "" {
		seed, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return drawing.Frame{}, "", fmt.Errorf("seed %q is not an integer", s)
		}
	}

	specs := make(map[string]patch.ParamSpec, len(p.Params()))
	for _, s := range p.Params() {
		specs[s.Key] = s
	}

	border, err := parseBorder(q.Get("border"))
	if err != nil {
		return drawing.Frame{}, "", err
	}
	page, err := parsePage(q.Get("page"))
	if err != nil {
		return drawing.Frame{}, "", err
	}

	values := patch.Values{}
	for key, raw := range q {
		switch key {
		case "patch", "seed", "format", "border", "page":
			continue
		}
		normalized := strings.HasPrefix(key, "n.")
		lookup := strings.TrimPrefix(key, "n.")

		spec, ok := specs[lookup]
		if !ok {
			return drawing.Frame{}, "", fmt.Errorf("patch %q has no parameter %q", name, lookup)
		}
		f, err := strconv.ParseFloat(raw[0], 64)
		if err != nil {
			return drawing.Frame{}, "", fmt.Errorf("value for %q is not a number: %q", key, raw[0])
		}
		if normalized {
			f = spec.FromNormalized(f)
		}
		values[lookup] = f
	}

	frame, err := p.Generate(r.Context(), patch.Request{
		Seed: seed, Page: page, Border: border, Values: values,
	})
	return frame, name, err
}

// parsePage reads a "WxH" page size in millimetres. Empty means the patch's
// own default.
func parsePage(raw string) (drawing.Size, error) {
	if raw == "" {
		return drawing.Size{}, nil
	}
	wStr, hStr, ok := strings.Cut(strings.ToLower(raw), "x")
	if !ok {
		return drawing.Size{}, fmt.Errorf("page %q must look like 297x210", raw)
	}
	w, err := strconv.ParseFloat(strings.TrimSpace(wStr), 64)
	if err != nil {
		return drawing.Size{}, fmt.Errorf("page width %q is not a number", wStr)
	}
	h, err := strconv.ParseFloat(strings.TrimSpace(hStr), 64)
	if err != nil {
		return drawing.Size{}, fmt.Errorf("page height %q is not a number", hStr)
	}
	// An upper bound as much as a lower one: nothing rejects a 50-metre page
	// otherwise, and a patch asked for one will happily try to fill it.
	const minMM, maxMM = 10, 2000
	if w < minMM || h < minMM || w > maxMM || h > maxMM {
		return drawing.Size{}, fmt.Errorf(
			"page %gx%g is outside the supported range %g-%gmm", w, h, float64(minMM), float64(maxMM))
	}
	return drawing.Size{W: w, H: h}, nil
}

// parseBorder reads a "x0,y0,x1,y1,..." polygon in millimetres. Empty means the
// patch uses its own default page.
func parseBorder(raw string) ([]drawing.Vec2, error) {
	if raw == "" {
		return nil, nil
	}
	fields := strings.Split(raw, ",")
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("border needs an even number of coordinates, got %d", len(fields))
	}
	if len(fields) < 6 {
		return nil, fmt.Errorf("border needs at least 3 points, got %d", len(fields)/2)
	}
	out := make([]drawing.Vec2, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		x, err := strconv.ParseFloat(strings.TrimSpace(fields[i]), 64)
		if err != nil {
			return nil, fmt.Errorf("border coordinate %q is not a number", fields[i])
		}
		y, err := strconv.ParseFloat(strings.TrimSpace(fields[i+1]), 64)
		if err != nil {
			return nil, fmt.Errorf("border coordinate %q is not a number", fields[i+1])
		}
		out = append(out, drawing.Vec2{X: x, Y: y})
	}
	return out, nil
}

func contentType(ext string) string {
	if ext == "svg" {
		return "image/svg+xml"
	}
	return "text/plain; charset=utf-8"
}

// round trims coordinates to micrometre precision. Full float64 digits would
// inflate the payload substantially and no pen can hit them anyway.
func round(v float64) float64 {
	return float64(int64(v*1000+copysign(0.5, v))) / 1000
}

func copysign(v, sign float64) float64 {
	if sign < 0 {
		return -v
	}
	return v
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

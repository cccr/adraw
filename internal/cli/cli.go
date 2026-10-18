// Package cli holds presentation helpers private to the adraw command.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/cccr/adraw/pkg/drawing"
	"github.com/cccr/adraw/pkg/patch"
)

// ValueFlag collects repeated --set key=value flags.
type ValueFlag struct {
	Values patch.Values
}

func (f *ValueFlag) String() string { return "" }

func (f *ValueFlag) Set(s string) error {
	key, raw, ok := strings.Cut(s, "=")
	if !ok {
		return fmt.Errorf("expected key=value, got %q", s)
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fmt.Errorf("value for %q is not a number: %w", key, err)
	}
	if f.Values == nil {
		f.Values = patch.Values{}
	}
	f.Values[strings.TrimSpace(key)] = v
	return nil
}

// Validate rejects keys the patch does not declare, and reports values outside
// a declared range rather than silently clamping them.
func Validate(vals patch.Values, specs []patch.ParamSpec) error {
	byKey := make(map[string]patch.ParamSpec, len(specs))
	known := make([]string, len(specs))
	for i, s := range specs {
		byKey[s.Key] = s
		known[i] = s.Key
	}
	for k, v := range vals {
		s, ok := byKey[k]
		if !ok {
			return fmt.Errorf("unknown parameter %q (known: %s)", k, strings.Join(known, ", "))
		}
		if v < s.Min || v > s.Max {
			return fmt.Errorf("%s=%g is outside the valid range [%g, %g]", k, v, s.Min, s.Max)
		}
	}
	return nil
}

// ParsePage reads a "WxH" page size in millimetres. An empty string means the
// patch's own default.
func ParsePage(raw string) (drawing.Size, error) {
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
	if w <= 0 || h <= 0 {
		return drawing.Size{}, fmt.Errorf("page %gx%g must be positive", w, h)
	}
	return drawing.Size{W: w, H: h}, nil
}

// LoadBorder reads a polygon from a JSON file of [[x,y],...] point pairs, in
// millimetres. An empty path yields nil, meaning the patch uses its own page.
func LoadBorder(path string) ([]drawing.Vec2, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading border: %w", err)
	}
	var pairs [][2]float64
	if err := json.Unmarshal(raw, &pairs); err != nil {
		return nil, fmt.Errorf("border file must be JSON [[x,y],...]: %w", err)
	}
	if len(pairs) < 3 {
		return nil, fmt.Errorf("border needs at least 3 points, got %d", len(pairs))
	}
	out := make([]drawing.Vec2, len(pairs))
	for i, p := range pairs {
		out[i] = drawing.Vec2{X: p[0], Y: p[1]}
	}
	return out, nil
}

// PrintParams writes a patch's parameters as an aligned table. It works off
// ParamSpec alone, so it never needs updating when a patch is added.
func PrintParams(w io.Writer, p patch.Patch) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "KEY\tRANGE\tDEFAULT\tDESCRIPTION\n")
	for _, s := range p.Params() {
		unit := s.Unit
		if unit != "" {
			unit = " " + unit
		}
		fmt.Fprintf(tw, "%s\t%g .. %g%s\t%g\t%s\n", s.Key, s.Min, s.Max, unit, s.Default, s.Desc)
	}
	tw.Flush()
}

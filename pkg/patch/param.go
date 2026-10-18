package patch

import "math"

// ParamSpec describes one tunable control of a patch.
//
// Every parameter is a float64 inside a declared [Min,Max]. That uniformity is
// what lets any control surface drive any patch mechanically: a MIDI CC maps in
// as FromNormalized(cc/127), a CV input as FromNormalized(volts/10), and a web
// UI generates a slider straight from the range — none of them needing code
// written per patch.
//
// A patch must therefore read all of its configuration from Values. Reading
// config from anywhere else silently breaks every UI at once.
type ParamSpec struct {
	Key     string  // stable identifier, e.g. "cell"
	Label   string  // short human-readable name, e.g. "Cell size"
	Desc    string  // one line on what the control does and which way it goes
	Min     float64 // inclusive lower bound
	Max     float64 // inclusive upper bound
	Default float64
	Step    float64 // 0 means continuous
	Unit    string  // "mm", "", "x"
}

// Clamp constrains v to [Min,Max].
func (s ParamSpec) Clamp(v float64) float64 {
	return math.Max(s.Min, math.Min(s.Max, v))
}

// FromNormalized maps n in [0,1] onto [Min,Max]. n is clamped first, so
// FromNormalized(0) is exactly Min and FromNormalized(1) exactly Max.
func (s ParamSpec) FromNormalized(n float64) float64 {
	n = math.Max(0, math.Min(1, n))
	return s.Min + n*(s.Max-s.Min)
}

// Normalize maps v in [Min,Max] onto [0,1]. A degenerate range yields 0.
func (s ParamSpec) Normalize(v float64) float64 {
	if s.Max == s.Min {
		return 0
	}
	return math.Max(0, math.Min(1, (s.Clamp(v)-s.Min)/(s.Max-s.Min)))
}

// Values holds the parameter settings for one Generate call. Missing keys fall
// back to the spec's default, so a caller may supply only what it wants to
// change.
type Values map[string]float64

// Get returns the value for s, clamped to its range, or s.Default if unset.
func (v Values) Get(s ParamSpec) float64 {
	raw, ok := v[s.Key]
	if !ok {
		return s.Default
	}
	return s.Clamp(raw)
}

// ParamSet builds a parameter list fluently, keeping the per-patch declaration
// boilerplate to a few readable lines.
type ParamSet struct {
	specs []ParamSpec
}

func NewParamSet() *ParamSet { return &ParamSet{} }

// Add appends a fully specified parameter.
func (p *ParamSet) Add(s ParamSpec) *ParamSet {
	p.specs = append(p.specs, s)
	return p
}

// MM adds a parameter measured in millimetres.
func (p *ParamSet) MM(key, label, desc string, min, max, def float64) *ParamSet {
	return p.Add(ParamSpec{Key: key, Label: label, Desc: desc,
		Min: min, Max: max, Default: def, Unit: "mm"})
}

// Unit adds a dimensionless parameter, typically a 0..1 ratio or probability.
func (p *ParamSet) Unit(key, label, desc string, min, max, def float64) *ParamSet {
	return p.Add(ParamSpec{Key: key, Label: label, Desc: desc,
		Min: min, Max: max, Default: def})
}

// Count adds an integer-valued parameter.
func (p *ParamSet) Count(key, label, desc string, min, max, def float64) *ParamSet {
	return p.Add(ParamSpec{Key: key, Label: label, Desc: desc,
		Min: min, Max: max, Default: def, Step: 1})
}

// Specs returns a copy of the declared parameters, so a caller cannot mutate
// the patch's own list.
func (p *ParamSet) Specs() []ParamSpec {
	out := make([]ParamSpec, len(p.specs))
	copy(out, p.specs)
	return out
}

// Find returns the spec with the given key.
func (p *ParamSet) Find(key string) (ParamSpec, bool) {
	for _, s := range p.specs {
		if s.Key == key {
			return s, true
		}
	}
	return ParamSpec{}, false
}

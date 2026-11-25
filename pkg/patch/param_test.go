package patch

import "testing"

var spec = ParamSpec{Key: "cell", Min: 5, Max: 60, Default: 18}

func TestValuesGetClampsAndDefaults(t *testing.T) {
	cases := []struct {
		name string
		v    Values
		want float64
	}{
		{"absent falls back to default", Values{}, 18},
		{"in range passes through", Values{"cell": 30}, 30},
		{"above max clamps", Values{"cell": 1e6}, 60},
		{"below min clamps", Values{"cell": -4}, 5},
		{"nil map falls back", nil, 18},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.v.Get(spec); got != c.want {
				t.Errorf("Get = %v, want %v", got, c.want)
			}
		})
	}
}

func TestNormalizedRoundTrip(t *testing.T) {
	if got := spec.FromNormalized(0); got != spec.Min {
		t.Errorf("FromNormalized(0) = %v, want %v", got, spec.Min)
	}
	if got := spec.FromNormalized(1); got != spec.Max {
		t.Errorf("FromNormalized(1) = %v, want %v", got, spec.Max)
	}
	// Out-of-range normalized input (a misbehaving control surface) must clamp
	// rather than produce a value outside the declared range.
	if got := spec.FromNormalized(2); got != spec.Max {
		t.Errorf("FromNormalized(2) = %v, want %v", got, spec.Max)
	}
	if got := spec.FromNormalized(-1); got != spec.Min {
		t.Errorf("FromNormalized(-1) = %v, want %v", got, spec.Min)
	}
	for _, v := range []float64{5, 18, 42, 60} {
		if got := spec.FromNormalized(spec.Normalize(v)); got != v {
			t.Errorf("round trip of %v = %v", v, got)
		}
	}
}

func TestNormalizeDegenerateRange(t *testing.T) {
	flat := ParamSpec{Key: "x", Min: 3, Max: 3}
	if got := flat.Normalize(3); got != 0 {
		t.Errorf("Normalize on zero-width range = %v, want 0", got)
	}
}

func TestParamSetBuilder(t *testing.T) {
	ps := NewParamSet().
		MM("cell", "Cell size", "width of a cell", 5, 60, 18).
		Unit("merge", "Merge", "chance of fusing cells", 0, 1, 0.15)

	specs := ps.Specs()
	if len(specs) != 2 {
		t.Fatalf("got %d specs, want 2", len(specs))
	}
	if specs[0].Unit != "mm" {
		t.Errorf("MM did not set unit, got %q", specs[0].Unit)
	}
	if _, ok := ps.Find("merge"); !ok {
		t.Error("Find(merge) failed")
	}
	if _, ok := ps.Find("nope"); ok {
		t.Error("Find returned a spec for an unknown key")
	}
	// Specs must hand back a copy; mutating it must not affect the patch.
	specs[0].Default = 999
	if again := ps.Specs(); again[0].Default != 18 {
		t.Error("Specs returned an aliased slice")
	}
}

func TestTextParamDefaultsAndOverride(t *testing.T) {
	s := NewParamSet().Text("text", "Text", "What to write.", "hello").Specs()[0]
	if s.Kind != KindText {
		t.Fatalf("Kind = %v, want KindText", s.Kind)
	}
	if got := (Strings{}).Get(s); got != "hello" {
		t.Errorf("absent key = %q, want default", got)
	}
	if got := (Strings{"text": "a;b"}).Get(s); got != "a;b" {
		t.Errorf("present key = %q, want a;b", got)
	}
}

func TestNumericSpecIsKindNumber(t *testing.T) {
	for _, s := range NewParamSet().MM("a", "A", "d", 0, 1, 0).Unit("b", "B", "d", 0, 1, 0).
		Count("c", "C", "d", 0, 1, 0).Specs() {
		if s.Kind != KindNumber {
			t.Errorf("%s has Kind %v, want KindNumber", s.Key, s.Kind)
		}
	}
}

func TestTextParamNilStringsFallsBack(t *testing.T) {
	s := NewParamSet().Text("text", "Text", "d", "dflt").Specs()[0]
	var nilStrings Strings
	if got := nilStrings.Get(s); got != "dflt" {
		t.Errorf("nil Strings = %q, want dflt", got)
	}
}

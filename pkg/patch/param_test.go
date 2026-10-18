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

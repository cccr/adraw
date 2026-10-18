package geom

import (
	"math"
	"testing"
)

func TestNoiseRange(t *testing.T) {
	n := NewNoise(1)
	lo, hi := math.Inf(1), math.Inf(-1)
	for i := 0; i < 20000; i++ {
		x := float64(i%200) * 0.137
		y := float64(i/200) * 0.191
		v := n.At(x, y)
		lo, hi = math.Min(lo, v), math.Max(hi, v)
		if math.IsNaN(v) {
			t.Fatalf("NaN at (%v,%v)", x, y)
		}
	}
	if lo < -1.05 || hi > 1.05 {
		t.Errorf("range [%v,%v] escapes [-1,1]", lo, hi)
	}
	// A field that never gets near its bounds is badly scaled.
	if hi < 0.5 || lo > -0.5 {
		t.Errorf("range [%v,%v] is suspiciously narrow", lo, hi)
	}
}

func TestNoiseDeterministicAndSeedDependent(t *testing.T) {
	a, b, c := NewNoise(7), NewNoise(7), NewNoise(8)
	var differs bool
	for i := 0; i < 500; i++ {
		x, y := float64(i)*0.31, float64(i)*0.17
		if a.At(x, y) != b.At(x, y) {
			t.Fatalf("same seed differs at (%v,%v)", x, y)
		}
		if a.At(x, y) != c.At(x, y) {
			differs = true
		}
	}
	if !differs {
		t.Error("different seeds produced an identical field")
	}
}

func TestNoiseIsContinuous(t *testing.T) {
	// Level-set tracing depends on smoothness; a discontinuity would show up
	// as a torn contour.
	n := NewNoise(3)
	const step = 1e-4
	for i := 0; i < 2000; i++ {
		x, y := float64(i)*0.07, float64(i)*0.13
		if d := math.Abs(n.At(x, y) - n.At(x+step, y)); d > 0.01 {
			t.Fatalf("jump of %v over %v at (%v,%v)", d, step, x, y)
		}
	}
}

func TestNoiseHasNoAxisAlignedBias(t *testing.T) {
	// The reason for choosing simplex over Perlin. Compare how much the field
	// varies along the axes against the diagonals; a lattice-aligned field is
	// measurably flatter along its axes.
	n := NewNoise(5)
	vary := func(dx, dy float64) float64 {
		var sum float64
		for i := 0; i < 4000; i++ {
			x, y := float64(i%80)*0.25, float64(i/80)*0.25
			sum += math.Abs(n.At(x, y) - n.At(x+dx, y+dy))
		}
		return sum
	}
	const d = 0.2
	axis := (vary(d, 0) + vary(0, d)) / 2
	diag := (vary(d*0.707, d*0.707) + vary(-d*0.707, d*0.707)) / 2
	if ratio := axis / diag; ratio < 0.8 || ratio > 1.25 {
		t.Errorf("axis/diagonal variation ratio %.2f suggests lattice bias", ratio)
	}
}

func TestFBMOctavesAddDetail(t *testing.T) {
	n := NewNoise(2)
	roughness := func(oct int) float64 {
		var sum float64
		for i := 0; i < 3000; i++ {
			x, y := float64(i)*0.01, float64(i)*0.013
			o := FBMOptions{Octaves: oct}
			sum += math.Abs(n.FBM(x, y, o) - n.FBM(x+0.01, y, o))
		}
		return sum
	}
	if one, five := roughness(1), roughness(5); five <= one {
		t.Errorf("5 octaves (%v) are no rougher than 1 (%v)", five, one)
	}
}

func TestFBMStaysInRange(t *testing.T) {
	n := NewNoise(4)
	for _, opt := range []FBMOptions{
		{Octaves: 1},
		{Octaves: 6},
		{Octaves: 6, Ridge: 1},
		{Octaves: 4, Ridge: 0.5, Lacunarity: 2.7, Gain: 0.6},
		{}, // zero value must not divide by zero
	} {
		for i := 0; i < 3000; i++ {
			x, y := float64(i%60)*0.3, float64(i/60)*0.3
			v := n.FBM(x, y, opt)
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("%+v produced %v", opt, v)
			}
			if v < -1.2 || v > 1.2 {
				t.Fatalf("%+v produced %v, outside [-1,1]", opt, v)
			}
		}
	}
}

func TestFBMRidgeSharpensCrests(t *testing.T) {
	// Ridged noise should concentrate values near the top of the range, since
	// folding about zero turns troughs into peaks.
	n := NewNoise(6)
	mean := func(ridge float64) float64 {
		var sum float64
		const count = 5000
		for i := 0; i < count; i++ {
			x, y := float64(i%100)*0.2, float64(i/100)*0.2
			sum += n.FBM(x, y, FBMOptions{Octaves: 4, Ridge: ridge})
		}
		return sum / count
	}
	if plain, ridged := mean(0), mean(1); ridged <= plain {
		t.Errorf("ridged mean %v is not above plain mean %v", ridged, plain)
	}
}

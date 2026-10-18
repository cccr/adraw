package geom

import (
	"math"
	"math/rand"
)

// Noise is a seeded 2D simplex noise field.
//
// Simplex rather than Perlin: Perlin's gradients live on a square lattice,
// which leaves faint axis-aligned streaks. They are invisible in a texture but
// become regular, obviously artificial stripes once you trace level sets or
// streamlines through the field — exactly what the topographic and flow patches
// do.
type Noise struct {
	perm [512]int
}

// NewNoise builds a noise field from a seed. The same seed always gives the
// same field.
func NewNoise(seed int64) *Noise {
	n := &Noise{}
	p := make([]int, 256)
	for i := range p {
		p[i] = i
	}
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(256, func(i, j int) { p[i], p[j] = p[j], p[i] })
	// Doubled so lookups can index up to 511 without a modulo.
	for i := 0; i < 512; i++ {
		n.perm[i] = p[i&255]
	}
	return n
}

// Skew factors taking the square lattice to the simplex (triangular) one.
var (
	f2 = 0.5 * (math.Sqrt(3) - 1)
	g2 = (3 - math.Sqrt(3)) / 6
)

var grad2 = [12][2]float64{
	{1, 1}, {-1, 1}, {1, -1}, {-1, -1},
	{1, 0}, {-1, 0}, {1, 0}, {-1, 0},
	{0, 1}, {0, -1}, {0, 1}, {0, -1},
}

// At returns the noise value at (x,y), in roughly [-1,1].
func (n *Noise) At(x, y float64) float64 {
	s := (x + y) * f2
	i := math.Floor(x + s)
	j := math.Floor(y + s)

	t := (i + j) * g2
	x0 := x - (i - t)
	y0 := y - (j - t)

	// Which of the two triangles in this lattice cell the point falls into.
	var i1, j1 float64
	if x0 > y0 {
		i1, j1 = 1, 0
	} else {
		i1, j1 = 0, 1
	}

	x1, y1 := x0-i1+g2, y0-j1+g2
	x2, y2 := x0-1+2*g2, y0-1+2*g2

	ii, jj := int(i)&255, int(j)&255

	corner := func(dx, dy float64, gi int) float64 {
		t := 0.5 - dx*dx - dy*dy
		if t < 0 {
			return 0
		}
		t *= t
		g := grad2[gi%12]
		return t * t * (g[0]*dx + g[1]*dy)
	}

	total := corner(x0, y0, n.perm[ii+n.perm[jj]]) +
		corner(x1, y1, n.perm[ii+int(i1)+n.perm[jj+int(j1)]]) +
		corner(x2, y2, n.perm[ii+1+n.perm[jj+1]])

	return 70 * total
}

// FBMOptions configures fractional Brownian motion: several octaves of noise
// summed at rising frequency and falling amplitude.
type FBMOptions struct {
	Octaves    int     // how many layers; 1 is plain noise
	Lacunarity float64 // frequency multiplier per octave, conventionally 2
	Gain       float64 // amplitude multiplier per octave, conventionally 0.5

	// Ridge folds each octave about zero and inverts it, turning rolling hills
	// into sharp crests. 0 is off, 1 is fully ridged.
	Ridge float64
}

// FBM sums octaves of noise at (x,y), returning roughly [-1,1].
func (n *Noise) FBM(x, y float64, opt FBMOptions) float64 {
	octaves := opt.Octaves
	if octaves < 1 {
		octaves = 1
	}
	lac := opt.Lacunarity
	if lac == 0 {
		lac = 2
	}
	gain := opt.Gain
	if gain == 0 {
		gain = 0.5
	}

	var sum, amp, norm float64 = 0, 1, 0
	freq := 1.0
	for o := 0; o < octaves; o++ {
		v := n.At(x*freq, y*freq)
		if opt.Ridge > 0 {
			// 1-|v| peaks where plain noise crosses zero, so crests become
			// sharp lines instead of smooth domes.
			ridged := 2*(1-math.Abs(v)) - 1
			v = v*(1-opt.Ridge) + ridged*opt.Ridge
		}
		sum += v * amp
		norm += amp
		amp *= gain
		freq *= lac
	}
	if norm == 0 {
		return 0
	}
	return sum / norm
}

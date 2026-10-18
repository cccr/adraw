// Package geom is the shared toolkit patches are built from: vector and
// polygon arithmetic, blue-noise sampling, tessellation, and fills.
//
// It is public rather than internal on purpose. These are the primitives every
// patch composes, not implementation details of any one of them — a patch
// living outside this repository must be able to import them, and new
// primitives belong here so the next patch inherits them for free.
package geom

import (
	"math"

	"github.com/cccr/adraw/pkg/drawing"
)

// Vec2 is re-exported from drawing so patch code can work in one namespace.
type Vec2 = drawing.Vec2

func Add(a, b Vec2) Vec2           { return Vec2{a.X + b.X, a.Y + b.Y} }
func Sub(a, b Vec2) Vec2           { return Vec2{a.X - b.X, a.Y - b.Y} }
func Scale(a Vec2, k float64) Vec2 { return Vec2{a.X * k, a.Y * k} }

// Lerp returns the point t of the way from a to b.
func Lerp(a, b Vec2, t float64) Vec2 {
	return Vec2{a.X + (b.X-a.X)*t, a.Y + (b.Y-a.Y)*t}
}

func Len(a Vec2) float64     { return math.Hypot(a.X, a.Y) }
func Dist(a, b Vec2) float64 { return math.Hypot(b.X-a.X, b.Y-a.Y) }
func Dot(a, b Vec2) float64  { return a.X*b.X + a.Y*b.Y }

// Cross is the z component of the 3D cross product. Its sign gives the turn
// direction from a to b, which is how convexity and winding are decided.
func Cross(a, b Vec2) float64 { return a.X*b.Y - a.Y*b.X }

// Perp returns a rotated a quarter turn counter-clockwise.
func Perp(a Vec2) Vec2 { return Vec2{-a.Y, a.X} }

// Rotate turns a about the origin by radians.
func Rotate(a Vec2, radians float64) Vec2 {
	s, c := math.Sincos(radians)
	return Vec2{a.X*c - a.Y*s, a.X*s + a.Y*c}
}

// Norm returns a unit vector in the direction of a, or the zero vector if a has
// no length.
func Norm(a Vec2) Vec2 {
	l := Len(a)
	if l == 0 {
		return Vec2{}
	}
	return Vec2{a.X / l, a.Y / l}
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

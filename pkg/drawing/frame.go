// Package drawing defines the data contract between the engine (patches) and
// renderers.
//
// Everything a patch produces is a set of polylines in millimetres. That is the
// lowest common denominator serving every renderer category: an image renderer
// strokes the paths, a G-code renderer travels pen-up between them and draws
// pen-down along them, and a chaining renderer can feed a Path straight back in
// as another patch's border.
package drawing

import "math"

// Vec2 is a point in millimetres.
type Vec2 struct {
	X, Y float64
}

// Path is a polyline. When Closed, the last point connects back to the first;
// the closing point is implied and must not be repeated in Pts.
type Path struct {
	Pts    []Vec2
	Closed bool
}

// Vec2Ring is a closed ring of points: a polygon boundary with the closing edge
// implied. It lives here rather than in geom so the patch contract does not have
// to depend on the geometry toolkit.
type Vec2Ring []Vec2

// Size is a page's physical dimensions in millimetres. The zero value means
// unspecified.
type Size struct {
	W, H float64
}

// IsZero reports whether no size was given.
func (s Size) IsZero() bool { return s.W <= 0 || s.H <= 0 }

// Rect is an axis-aligned bounding box.
type Rect struct {
	Min, Max Vec2
}

// Frame is one complete generated drawing.
//
// Deliberately carries no colour or stroke width: a pen plotter has one pen,
// and renderers that want style apply their own. Resist growing this struct —
// every patch and renderer depends on its shape.
type Frame struct {
	Bounds Rect
	Paths  []Path

	// Meta records patch name, seed and resolved parameters so any output can
	// be reproduced from the file it was written to.
	Meta map[string]string
}

func (r Rect) Width() float64  { return r.Max.X - r.Min.X }
func (r Rect) Height() float64 { return r.Max.Y - r.Min.Y }

// Contains reports whether p lies within r, inclusive of the edges.
func (r Rect) Contains(p Vec2) bool {
	return p.X >= r.Min.X && p.X <= r.Max.X && p.Y >= r.Min.Y && p.Y <= r.Max.Y
}

// Union returns the smallest rect containing both r and o.
func (r Rect) Union(o Rect) Rect {
	return Rect{
		Min: Vec2{math.Min(r.Min.X, o.Min.X), math.Min(r.Min.Y, o.Min.Y)},
		Max: Vec2{math.Max(r.Max.X, o.Max.X), math.Max(r.Max.Y, o.Max.Y)},
	}
}

// BoundsOf returns the bounding box of pts. The zero Rect is returned for an
// empty slice.
func BoundsOf(pts []Vec2) Rect {
	if len(pts) == 0 {
		return Rect{}
	}
	r := Rect{Min: pts[0], Max: pts[0]}
	for _, p := range pts[1:] {
		r.Min.X = math.Min(r.Min.X, p.X)
		r.Min.Y = math.Min(r.Min.Y, p.Y)
		r.Max.X = math.Max(r.Max.X, p.X)
		r.Max.Y = math.Max(r.Max.Y, p.Y)
	}
	return r
}

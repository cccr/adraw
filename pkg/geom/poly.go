package geom

import (
	"math"

	"github.com/cccr/adraw/pkg/drawing"
)

// Polygon is a closed ring of vertices. The closing edge from the last vertex
// back to the first is implied and never stored.
type Polygon []Vec2

// SignedArea returns the shoelace area: positive for counter-clockwise
// winding, negative for clockwise.
func SignedArea(p Polygon) float64 {
	if len(p) < 3 {
		return 0
	}
	var sum float64
	for i := range p {
		a, b := p[i], p[(i+1)%len(p)]
		sum += a.X*b.Y - b.X*a.Y
	}
	return sum / 2
}

// Area returns the unsigned area.
func Area(p Polygon) float64 { return math.Abs(SignedArea(p)) }

// Perimeter returns the total edge length including the closing edge.
func Perimeter(p Polygon) float64 {
	if len(p) < 2 {
		return 0
	}
	var sum float64
	for i := range p {
		sum += Dist(p[i], p[(i+1)%len(p)])
	}
	return sum
}

// Centroid returns the area centroid, falling back to the vertex mean for
// degenerate (zero-area) polygons.
func Centroid(p Polygon) Vec2 {
	if len(p) == 0 {
		return Vec2{}
	}
	a := SignedArea(p)
	if a == 0 {
		var m Vec2
		for _, v := range p {
			m = Add(m, v)
		}
		return Scale(m, 1/float64(len(p)))
	}
	var cx, cy float64
	for i := range p {
		u, v := p[i], p[(i+1)%len(p)]
		w := u.X*v.Y - v.X*u.Y
		cx += (u.X + v.X) * w
		cy += (u.Y + v.Y) * w
	}
	return Vec2{cx / (6 * a), cy / (6 * a)}
}

// IsConvex reports whether p is convex. Every turn must go the same way;
// collinear vertices are tolerated. The spiral fill requires convex input, so
// this gates the cell-merging step.
func IsConvex(p Polygon) bool {
	if len(p) < 3 {
		return false
	}
	const eps = 1e-12
	var sign float64
	for i := range p {
		a := p[i]
		b := p[(i+1)%len(p)]
		c := p[(i+2)%len(p)]
		cr := Cross(Sub(b, a), Sub(c, b))
		if math.Abs(cr) < eps {
			continue // collinear vertex, no information about the turn
		}
		if sign == 0 {
			sign = math.Copysign(1, cr)
		} else if math.Copysign(1, cr) != sign {
			return false
		}
	}
	return sign != 0
}

// Contains reports whether pt is inside p, using the even-odd ray crossing
// rule. Points exactly on an edge are reported as inside.
func Contains(p Polygon, pt Vec2) bool {
	if len(p) < 3 {
		return false
	}
	if OnBoundary(p, pt, 1e-9) {
		return true
	}
	in := false
	for i := range p {
		a, b := p[i], p[(i+1)%len(p)]
		if (a.Y > pt.Y) != (b.Y > pt.Y) {
			x := a.X + (pt.Y-a.Y)/(b.Y-a.Y)*(b.X-a.X)
			if pt.X < x {
				in = !in
			}
		}
	}
	return in
}

// OnBoundary reports whether pt lies within tol of any edge of p.
func OnBoundary(p Polygon, pt Vec2, tol float64) bool {
	for i := range p {
		if distToSegment(pt, p[i], p[(i+1)%len(p)]) <= tol {
			return true
		}
	}
	return false
}

func distToSegment(p, a, b Vec2) float64 {
	ab := Sub(b, a)
	l2 := Dot(ab, ab)
	if l2 == 0 {
		return Dist(p, a)
	}
	t := clamp(Dot(Sub(p, a), ab)/l2, 0, 1)
	return Dist(p, Add(a, Scale(ab, t)))
}

// BBox returns the bounding box of p.
func BBox(p Polygon) drawing.Rect { return drawing.BoundsOf(p) }

// InRadius returns 2*Area/Perimeter — the inradius for a tangential polygon,
// and a good stand-in elsewhere for "how thick is this shape". The spiral fill
// uses it to convert a desired ring gap in millimetres into an edge fraction.
func InRadius(p Polygon) float64 {
	per := Perimeter(p)
	if per == 0 {
		return 0
	}
	return 2 * Area(p) / per
}

// RectPolygon returns a w by h rectangle inset by margin on every side, with
// its lower-left corner at the origin plus margin. Vertices wind
// counter-clockwise.
func RectPolygon(w, h, margin float64) Polygon {
	return Polygon{
		{margin, margin},
		{w - margin, margin},
		{w - margin, h - margin},
		{margin, h - margin},
	}
}

// ToPath converts a polygon to a closed drawing path.
func ToPath(p Polygon) drawing.Path {
	pts := make([]Vec2, len(p))
	copy(pts, p)
	return drawing.Path{Pts: pts, Closed: true}
}

// ClipToConvex trims subject to the part inside the convex polygon clip, using
// Sutherland-Hodgman. It returns nil if nothing survives.
//
// The clip region must be convex; the subject need not be. Borders are convex
// throughout v1, which is what makes this sufficient.
func ClipToConvex(subject, clip Polygon) Polygon {
	if len(subject) < 3 || len(clip) < 3 {
		return nil
	}
	// Work with an outward-consistent winding so the inside test has one sign.
	inward := 1.0
	if SignedArea(clip) < 0 {
		inward = -1
	}

	out := append(Polygon(nil), subject...)
	for i := range clip {
		if len(out) == 0 {
			return nil
		}
		a, b := clip[i], clip[(i+1)%len(clip)]
		edge := Sub(b, a)

		inside := func(p Vec2) bool {
			return Cross(edge, Sub(p, a))*inward >= -1e-12
		}

		in := out
		out = out[:0:0]
		for j := range in {
			cur, prev := in[j], in[(j-1+len(in))%len(in)]
			curIn, prevIn := inside(cur), inside(prev)

			if curIn != prevIn {
				if x, ok := lineIntersect(prev, cur, a, b); ok {
					out = append(out, x)
				}
			}
			if curIn {
				out = append(out, cur)
			}
		}
	}
	if len(out) < 3 {
		return nil
	}
	return out
}

// lineIntersect returns where the infinite lines through p1-p2 and p3-p4 meet.
func lineIntersect(p1, p2, p3, p4 Vec2) (Vec2, bool) {
	d1, d2 := Sub(p2, p1), Sub(p4, p3)
	den := Cross(d1, d2)
	if math.Abs(den) < 1e-12 {
		return Vec2{}, false
	}
	t := Cross(Sub(p3, p1), d2) / den
	return Add(p1, Scale(d1, t)), true
}

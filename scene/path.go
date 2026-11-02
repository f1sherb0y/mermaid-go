package scene

import "math"

// Seg is a path segment: Op with its points (one for MoveTo and LineTo,
// two for QuadTo, three for CubeTo, none for Close).
type Seg struct {
	Op Op
	P  [3]Pt
}

// Op is a segment's kind.
type Op uint8

const (
	MoveTo Op = iota
	LineTo
	QuadTo
	CubeTo
	Close
)

// Pt is a point.
type Pt struct{ X, Y float64 }

// PathBuilder makes a path's segments; arcs become cubic curves.
type PathBuilder struct {
	Segs       []Seg
	start, cur Pt
}

func (b *PathBuilder) MoveTo(x, y float64) *PathBuilder {
	b.Segs = append(b.Segs, Seg{Op: MoveTo, P: [3]Pt{{x, y}}})
	b.start, b.cur = Pt{x, y}, Pt{x, y}
	return b
}

func (b *PathBuilder) LineTo(x, y float64) *PathBuilder {
	b.Segs = append(b.Segs, Seg{Op: LineTo, P: [3]Pt{{x, y}}})
	b.cur = Pt{x, y}
	return b
}

func (b *PathBuilder) QuadTo(x1, y1, x, y float64) *PathBuilder {
	b.Segs = append(b.Segs, Seg{Op: QuadTo, P: [3]Pt{{x1, y1}, {x, y}}})
	b.cur = Pt{x, y}
	return b
}

func (b *PathBuilder) CubeTo(x1, y1, x2, y2, x, y float64) *PathBuilder {
	b.Segs = append(b.Segs, Seg{Op: CubeTo, P: [3]Pt{{x1, y1}, {x2, y2}, {x, y}}})
	b.cur = Pt{x, y}
	return b
}

func (b *PathBuilder) Close() *PathBuilder {
	b.Segs = append(b.Segs, Seg{Op: Close})
	b.cur = b.start
	return b
}

// Current is the pen's position.
func (b *PathBuilder) Current() Pt { return b.cur }

// ArcTo is SVG's elliptical arc command (A) to (x, y).
func (b *PathBuilder) ArcTo(rx, ry, rotation float64, large, sweep bool, x, y float64) *PathBuilder {
	x0, y0 := b.cur.X, b.cur.Y
	if x0 == x && y0 == y {
		return b
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		return b.LineTo(x, y)
	}
	// SVG's endpoint to center parameterization (SVG 1.1, F.6.5).
	sinPhi, cosPhi := math.Sincos(rotation * math.Pi / 180)
	dx, dy := (x0-x)/2, (y0-y)/2
	x1 := cosPhi*dx + sinPhi*dy
	y1 := -sinPhi*dx + cosPhi*dy
	if l := x1*x1/(rx*rx) + y1*y1/(ry*ry); l > 1 {
		s := math.Sqrt(l)
		rx, ry = rx*s, ry*s
	}
	num := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	den := rx*rx*y1*y1 + ry*ry*x1*x1
	co := math.Sqrt(math.Max(0, num/den))
	if large == sweep {
		co = -co
	}
	cx1, cy1 := co*rx*y1/ry, -co*ry*x1/rx
	cx := cosPhi*cx1 - sinPhi*cy1 + (x0+x)/2
	cy := sinPhi*cx1 + cosPhi*cy1 + (y0+y)/2
	angle := func(ux, uy, vx, vy float64) float64 {
		return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	}
	t1 := angle(1, 0, (x1-cx1)/rx, (y1-cy1)/ry)
	dt := angle((x1-cx1)/rx, (y1-cy1)/ry, (-x1-cx1)/rx, (-y1-cy1)/ry)
	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	} else if sweep && dt < 0 {
		dt += 2 * math.Pi
	}
	// At most a quarter turn per cubic.
	n := int(math.Ceil(math.Abs(dt) / (math.Pi / 2)))
	step := dt / float64(n)
	k := 4.0 / 3 * math.Tan(step/4)
	pt := func(t float64) (float64, float64, float64, float64) {
		s, c := math.Sincos(t)
		px, py := rx*c, ry*s
		tx, ty := -rx*s, ry*c
		return cosPhi*px - sinPhi*py + cx, sinPhi*px + cosPhi*py + cy, cosPhi*tx - sinPhi*ty, sinPhi*tx + cosPhi*ty
	}
	t := t1
	ax, ay, adx, ady := pt(t)
	for i := range n {
		bx, by, bdx, bdy := pt(t + step)
		if i == n-1 {
			bx, by = x, y
		}
		b.CubeTo(ax+k*adx, ay+k*ady, bx-k*bdx, by-k*bdy, bx, by)
		ax, ay, adx, ady = bx, by, bdx, bdy
		t += step
	}
	return b
}

// RectPath is a rectangle, its corners rounded by rx, ry when not zero.
func RectPath(x, y, w, h, rx, ry float64) []Seg {
	var b PathBuilder
	if rx <= 0 && ry <= 0 {
		b.MoveTo(x, y).LineTo(x+w, y).LineTo(x+w, y+h).LineTo(x, y+h).Close()
		return b.Segs
	}
	if rx <= 0 {
		rx = ry
	}
	if ry <= 0 {
		ry = rx
	}
	rx, ry = math.Min(rx, w/2), math.Min(ry, h/2)
	b.MoveTo(x+rx, y).LineTo(x+w-rx, y).ArcTo(rx, ry, 0, false, true, x+w, y+ry).
		LineTo(x+w, y+h-ry).ArcTo(rx, ry, 0, false, true, x+w-rx, y+h).
		LineTo(x+rx, y+h).ArcTo(rx, ry, 0, false, true, x, y+h-ry).
		LineTo(x, y+ry).ArcTo(rx, ry, 0, false, true, x+rx, y).Close()
	return b.Segs
}

// Ellipse is an ellipse about (cx, cy).
func Ellipse(cx, cy, rx, ry float64) []Seg {
	var b PathBuilder
	b.MoveTo(cx+rx, cy).ArcTo(rx, ry, 0, false, true, cx-rx, cy).ArcTo(rx, ry, 0, false, true, cx+rx, cy).Close()
	return b.Segs
}

// Polygon is a closed polygon.
func Polygon(pts ...Pt) []Seg {
	var b PathBuilder
	for i, p := range pts {
		if i == 0 {
			b.MoveTo(p.X, p.Y)
		} else {
			b.LineTo(p.X, p.Y)
		}
	}
	return b.Close().Segs
}

// Bounds is the box of a path's points (control points included: a little
// generous for curves).
func Bounds(segs []Seg) Rect {
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	n := map[Op]int{MoveTo: 1, LineTo: 1, QuadTo: 2, CubeTo: 3}
	for _, s := range segs {
		for _, p := range s.P[:n[s.Op]] {
			minX, minY, maxX, maxY = math.Min(minX, p.X), math.Min(minY, p.Y), math.Max(maxX, p.X), math.Max(maxY, p.Y)
		}
	}
	if minX > maxX {
		return Rect{}
	}
	return Rect{minX, minY, maxX - minX, maxY - minY}
}

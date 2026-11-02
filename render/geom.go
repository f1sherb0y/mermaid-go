package render

import (
	"math"
	"sort"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// P is a point.
type P = scene.Pt

// box is a node's placed box, as mermaid's intersect functions read it:
// centre and size.
type box struct{ X, Y, W, H float64 }

// intersectRect is where the ray from the box's centre to p leaves it.
func intersectRect(b box, p P) P {
	dx, dy := p.X-b.X, p.Y-b.Y
	w, h := b.W/2, b.H/2
	var sx, sy float64
	if math.Abs(dy)*w > math.Abs(dx)*h {
		if dy < 0 {
			h = -h
		}
		if dy != 0 {
			sx = h * dx / dy
		}
		sy = h
	} else {
		if dx < 0 {
			w = -w
		}
		sx = w
		if dx != 0 {
			sy = w * dy / dx
		}
	}
	return P{X: b.X + sx, Y: b.Y + sy}
}

// intersectEllipse is where the ray from the centre to p leaves an ellipse.
func intersectEllipse(b box, rx, ry float64, p P) P {
	px, py := b.X-p.X, b.Y-p.Y
	det := math.Sqrt(rx*rx*py*py + ry*ry*px*px)
	dx := math.Abs(rx * ry * px / det)
	if p.X < b.X {
		dx = -dx
	}
	dy := math.Abs(rx * ry * py / det)
	if p.Y < b.Y {
		dy = -dy
	}
	return P{X: b.X + dx, Y: b.Y + dy}
}

// intersectPolygon is where the segment from the box's centre to p crosses
// a polygon given relative to the box's top left (shifted by its smallest
// coordinates), nearest p; the centre when it crosses none.
func intersectPolygon(b box, poly []P, p P) P {
	minX, minY := math.Inf(1), math.Inf(1)
	for _, q := range poly {
		minX, minY = math.Min(minX, q.X), math.Min(minY, q.Y)
	}
	left := b.X - b.W/2 - minX
	top := b.Y - b.H/2 - minY
	c := P{X: b.X, Y: b.Y}
	var hits []P
	for i, p1 := range poly {
		p2 := poly[(i+1)%len(poly)]
		if q, ok := intersectLine(c, p, P{X: left + p1.X, Y: top + p1.Y}, P{X: left + p2.X, Y: top + p2.Y}); ok {
			hits = append(hits, q)
		}
	}
	if len(hits) == 0 {
		return c
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return math.Hypot(hits[i].X-p.X, hits[i].Y-p.Y) < math.Hypot(hits[j].X-p.X, hits[j].Y-p.Y)
	})
	return hits[0]
}

// intersectLine is where segment p1-p2 meets segment q1-q2.
func intersectLine(p1, p2, q1, q2 P) (P, bool) {
	a1 := p2.Y - p1.Y
	b1 := p1.X - p2.X
	c1 := p2.X*p1.Y - p1.X*p2.Y
	r3 := a1*q1.X + b1*q1.Y + c1
	r4 := a1*q2.X + b1*q2.Y + c1
	if r3 != 0 && r4 != 0 && r3*r4 > 0 {
		return P{}, false
	}
	a2 := q2.Y - q1.Y
	b2 := q1.X - q2.X
	c2 := q2.X*q1.Y - q1.X*q2.Y
	r1 := a2*p1.X + b2*p1.Y + c2
	r2 := a2*p2.X + b2*p2.Y + c2
	if math.Abs(r1) < 1e-6 && math.Abs(r2) < 1e-6 && r1*r2 > 0 {
		return P{}, false
	}
	denom := a1*b2 - a2*b1
	if denom == 0 {
		return P{}, false
	}
	return P{X: (b1*c2 - b2*c1) / denom, Y: (a2*c1 - a1*c2) / denom}, true
}

// rect is a bounding box being grown.
type rect struct{ x0, y0, x1, y1 float64 }

func emptyRect() rect { return rect{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)} }

func (r *rect) add(x, y float64) {
	r.x0, r.y0 = math.Min(r.x0, x), math.Min(r.y0, y)
	r.x1, r.y1 = math.Max(r.x1, x), math.Max(r.y1, y)
}

func (r *rect) addRect(o scene.Rect) {
	r.add(o.X, o.Y)
	r.add(o.X+o.W, o.Y+o.H)
}

func (r rect) empty() bool { return r.x0 > r.x1 }

func (r rect) scene() scene.Rect {
	if r.empty() {
		return scene.Rect{}
	}
	return scene.Rect{X: r.x0, Y: r.y0, W: r.x1 - r.x0, H: r.y1 - r.y0}
}

// pathBounds is a path's tight box, curves' extremes included (what
// getBBox gives).
func pathBounds(segs []scene.Seg) scene.Rect {
	r := emptyRect()
	var cur P
	for _, s := range segs {
		switch s.Op {
		case scene.MoveTo, scene.LineTo:
			cur = s.P[0]
			r.add(cur.X, cur.Y)
		case scene.QuadTo:
			p0, p1, p2 := cur, s.P[0], s.P[1]
			r.add(p2.X, p2.Y)
			for _, t := range []float64{quadT(p0.X, p1.X, p2.X), quadT(p0.Y, p1.Y, p2.Y)} {
				if t > 0 && t < 1 {
					u := 1 - t
					r.add(u*u*p0.X+2*u*t*p1.X+t*t*p2.X, u*u*p0.Y+2*u*t*p1.Y+t*t*p2.Y)
				}
			}
			cur = p2
		case scene.CubeTo:
			p0, p1, p2, p3 := cur, s.P[0], s.P[1], s.P[2]
			r.add(p3.X, p3.Y)
			ts := append(cubicT(p0.X, p1.X, p2.X, p3.X), cubicT(p0.Y, p1.Y, p2.Y, p3.Y)...)
			for _, t := range ts {
				if t > 0 && t < 1 {
					u := 1 - t
					a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
					r.add(a*p0.X+b*p1.X+c*p2.X+d*p3.X, a*p0.Y+b*p1.Y+c*p2.Y+d*p3.Y)
				}
			}
			cur = p3
		}
	}
	return r.scene()
}

func quadT(a, b, c float64) float64 {
	d := a - 2*b + c
	if d == 0 {
		return -1
	}
	return (a - b) / d
}

func cubicT(a, b, c, d float64) []float64 {
	// Derivative: 3[(b-a)(1-t)^2 + 2(c-b)t(1-t) + (d-c)t^2].
	qa := (b - a) - 2*(c-b) + (d - c)
	qb := 2 * ((c - b) - (b - a))
	qc := b - a
	if math.Abs(qa) < 1e-12 {
		if qb == 0 {
			return nil
		}
		return []float64{-qc / qb}
	}
	disc := qb*qb - 4*qa*qc
	if disc < 0 {
		return nil
	}
	s := math.Sqrt(disc)
	return []float64{(-qb + s) / (2 * qa), (-qb - s) / (2 * qa)}
}

// translate moves segments.
func translate(segs []scene.Seg, dx, dy float64) []scene.Seg {
	out := make([]scene.Seg, len(segs))
	n := map[scene.Op]int{scene.MoveTo: 1, scene.LineTo: 1, scene.QuadTo: 2, scene.CubeTo: 3}
	for i, s := range segs {
		out[i] = s
		for j := range n[s.Op] {
			out[i].P[j].X += dx
			out[i].P[j].Y += dy
		}
	}
	return out
}

// poly is a closed polygon's segments, moved by (dx, dy).
func poly(pts []P, dx, dy float64) []scene.Seg {
	moved := make([]P, len(pts))
	for i, p := range pts {
		moved[i] = P{X: p.X + dx, Y: p.Y + dy}
	}
	return scene.Polygon(moved...)
}

// polyline is an open path through the points.
func polyline(pts []P) []scene.Seg {
	var b scene.PathBuilder
	for i, p := range pts {
		if i == 0 {
			b.MoveTo(p.X, p.Y)
		} else {
			b.LineTo(p.X, p.Y)
		}
	}
	return b.Segs
}

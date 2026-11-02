package render

import (
	"math"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// d3-shape's curves, as mermaid's edges use them (edges.js: the
// interpolation by name, basis by default).

// curvePath is the path d3.line() with the named curve draws through pts.
func curvePath(name string, pts []P) []scene.Seg {
	if len(pts) == 0 {
		return nil
	}
	var b scene.PathBuilder
	switch name {
	case "linear", "rounded":
		return polyline(pts)
	case "step":
		stepCurve(&b, pts, 0.5)
	case "stepBefore":
		stepCurve(&b, pts, 0)
	case "stepAfter":
		stepCurve(&b, pts, 1)
	case "bumpX":
		bumpCurve(&b, pts, true)
	case "bumpY":
		bumpCurve(&b, pts, false)
	case "cardinal":
		cardinalCurve(&b, pts, 0)
	case "catmullRom":
		catmullRomCurve(&b, pts, 0.5)
	case "natural":
		naturalCurve(&b, pts)
	case "monotoneX":
		monotoneCurve(&b, pts, false)
	case "monotoneY":
		monotoneCurve(&b, pts, true)
	default:
		basisCurve(&b, pts)
	}
	return b.Segs
}

func basisCurve(b *scene.PathBuilder, pts []P) {
	if len(pts) == 1 {
		b.MoveTo(pts[0].X, pts[0].Y)
		return
	}
	var x0, y0, x1, y1 float64
	state := 0
	point := func(x, y float64) {
		b.CubeTo((2*x0+x1)/3, (2*y0+y1)/3, (x0+2*x1)/3, (y0+2*y1)/3, (x0+4*x1+x)/6, (y0+4*y1+y)/6)
	}
	for _, p := range pts {
		x, y := p.X, p.Y
		switch state {
		case 0:
			state = 1
			b.MoveTo(x, y)
		case 1:
			state = 2
		case 2:
			state = 3
			b.LineTo((5*x0+x1)/6, (5*y0+y1)/6)
			point(x, y)
		default:
			point(x, y)
		}
		x0, x1 = x1, x
		y0, y1 = y1, y
	}
	switch state {
	case 3:
		point(x1, y1)
		b.LineTo(x1, y1)
	case 2:
		b.LineTo(x1, y1)
	}
}

func stepCurve(b *scene.PathBuilder, pts []P, t float64) {
	for i, p := range pts {
		if i == 0 {
			b.MoveTo(p.X, p.Y)
			continue
		}
		q := pts[i-1]
		switch {
		case t <= 0:
			b.LineTo(q.X, p.Y)
			b.LineTo(p.X, p.Y)
		case t >= 1:
			b.LineTo(p.X, q.Y)
			b.LineTo(p.X, p.Y)
		default:
			x := q.X*(1-t) + p.X*t
			b.LineTo(x, q.Y)
			b.LineTo(x, p.Y)
			b.LineTo(p.X, p.Y)
		}
	}
}

func bumpCurve(b *scene.PathBuilder, pts []P, x bool) {
	for i, p := range pts {
		if i == 0 {
			b.MoveTo(p.X, p.Y)
			continue
		}
		q := pts[i-1]
		if x {
			mx := (q.X + p.X) / 2
			b.CubeTo(mx, q.Y, mx, p.Y, p.X, p.Y)
		} else {
			my := (q.Y + p.Y) / 2
			b.CubeTo(q.X, my, p.X, my, p.X, p.Y)
		}
	}
}

func cardinalCurve(b *scene.PathBuilder, pts []P, tension float64) {
	k := (1 - tension) / 6
	n := len(pts)
	if n == 2 {
		b.MoveTo(pts[0].X, pts[0].Y)
		b.LineTo(pts[1].X, pts[1].Y)
		return
	}
	get := func(i int) P {
		if i < 0 {
			return pts[0]
		}
		if i >= n {
			return pts[n-1]
		}
		return pts[i]
	}
	b.MoveTo(pts[0].X, pts[0].Y)
	for i := 1; i < n; i++ {
		p0, p1, p2, p3 := get(i-2), get(i-1), get(i), get(i+1)
		if i == 1 {
			p0 = p1
		}
		if i == n-1 {
			p3 = p2
		}
		b.CubeTo(p1.X+k*(p2.X-p0.X), p1.Y+k*(p2.Y-p0.Y), p2.X+k*(p1.X-p3.X), p2.Y+k*(p1.Y-p3.Y), p2.X, p2.Y)
	}
}

func catmullRomCurve(b *scene.PathBuilder, pts []P, alpha float64) {
	n := len(pts)
	if n == 2 {
		b.MoveTo(pts[0].X, pts[0].Y)
		b.LineTo(pts[1].X, pts[1].Y)
		return
	}
	b.MoveTo(pts[0].X, pts[0].Y)
	for i := 1; i < n; i++ {
		p1, p2 := pts[i-1], pts[i]
		p0, p3 := p1, p2
		if i >= 2 {
			p0 = pts[i-2]
		}
		if i+1 < n {
			p3 = pts[i+1]
		}
		l01 := math.Pow(math.Hypot(p1.X-p0.X, p1.Y-p0.Y), alpha)
		l12 := math.Pow(math.Hypot(p2.X-p1.X, p2.Y-p1.Y), alpha)
		l23 := math.Pow(math.Hypot(p3.X-p2.X, p3.Y-p2.Y), alpha)
		c1, c2 := p1, p2
		const eps = 1e-12
		if l01 > eps {
			a := 2*l01*l01 + 3*l01*l12 + l12*l12
			nn := 3 * l01 * (l01 + l12)
			c1 = P{X: (p1.X*a - p0.X*l12*l12 + p2.X*l01*l01) / nn, Y: (p1.Y*a - p0.Y*l12*l12 + p2.Y*l01*l01) / nn}
		}
		if l23 > eps {
			bb := 2*l23*l23 + 3*l23*l12 + l12*l12
			m := 3 * l23 * (l23 + l12)
			c2 = P{X: (p2.X*bb + p1.X*l23*l23 - p3.X*l12*l12) / m, Y: (p2.Y*bb + p1.Y*l23*l23 - p3.Y*l12*l12) / m}
		}
		b.CubeTo(c1.X, c1.Y, c2.X, c2.Y, p2.X, p2.Y)
	}
}

func naturalCurve(b *scene.PathBuilder, pts []P) {
	n := len(pts)
	b.MoveTo(pts[0].X, pts[0].Y)
	if n == 2 {
		b.LineTo(pts[1].X, pts[1].Y)
		return
	}
	if n < 2 {
		return
	}
	ctrl := func(x []float64) ([]float64, []float64) {
		m := len(x) - 1
		a, bb, r := make([]float64, m), make([]float64, m), make([]float64, m)
		a[0], bb[0], r[0] = 0, 2, x[0]+2*x[1]
		for i := 1; i < m-1; i++ {
			a[i], bb[i], r[i] = 1, 4, 4*x[i]+2*x[i+1]
		}
		a[m-1], bb[m-1], r[m-1] = 2, 7, 8*x[m-1]+x[m]
		for i := 1; i < m; i++ {
			t := a[i] / bb[i-1]
			bb[i] -= t
			r[i] -= t * r[i-1]
		}
		c1 := make([]float64, m)
		c1[m-1] = r[m-1] / bb[m-1]
		for i := m - 2; i >= 0; i-- {
			c1[i] = (r[i] - c1[i+1]) / bb[i]
		}
		c2 := make([]float64, m)
		c2[m-1] = (x[m] + c1[m-1]) / 2
		for i := 0; i < m-1; i++ {
			c2[i] = 2*x[i+1] - c1[i+1]
		}
		return c1, c2
	}
	xs, ys := make([]float64, n), make([]float64, n)
	for i, p := range pts {
		xs[i], ys[i] = p.X, p.Y
	}
	ax, bx := ctrl(xs)
	ay, by := ctrl(ys)
	for i := 0; i < n-1; i++ {
		b.CubeTo(ax[i], ay[i], bx[i], by[i], xs[i+1], ys[i+1])
	}
}

func monotoneCurve(b *scene.PathBuilder, pts []P, swap bool) {
	get := func(p P) (float64, float64) {
		if swap {
			return p.Y, p.X
		}
		return p.X, p.Y
	}
	put := func(x, y float64) (float64, float64) {
		if swap {
			return y, x
		}
		return x, y
	}
	n := len(pts)
	x0, y0 := get(pts[0])
	b.MoveTo(put(x0, y0))
	if n == 2 {
		x1, y1 := get(pts[1])
		b.LineTo(put(x1, y1))
		return
	}
	sgn := func(f float64) float64 {
		if f < 0 {
			return -1
		}
		return 1
	}
	slope3 := func(x0, y0, x1, y1, x2, y2 float64) float64 {
		h0, h1 := x1-x0, x2-x1
		s0, s1 := (y1-y0)/h0, (y2-y1)/h1
		if h0 == 0 && h1 < 0 || h1 == 0 && h0 < 0 {
			s0 = (y1 - y0) / h0
		}
		p := (s0*h1 + s1*h0) / (h0 + h1)
		v := (sgn(s0) + sgn(s1)) * math.Min(math.Min(math.Abs(s0), math.Abs(s1)), 0.5*math.Abs(p))
		if math.IsNaN(v) {
			return 0
		}
		return v
	}
	slope2 := func(x0, y0, x1, y1, t float64) float64 {
		h := x1 - x0
		if h == 0 {
			return t
		}
		return (3*(y1-y0)/h - t) / 2
	}
	seg := func(xa, ya, xb, yb, t0, t1 float64) {
		dx := (xb - xa) / 3
		c1x, c1y := put(xa+dx, ya+dx*t0)
		c2x, c2y := put(xb-dx, yb-dx*t1)
		ex, ey := put(xb, yb)
		b.CubeTo(c1x, c1y, c2x, c2y, ex, ey)
	}
	xs, ys := make([]float64, n), make([]float64, n)
	for i, p := range pts {
		xs[i], ys[i] = get(p)
	}
	t := make([]float64, n)
	for i := 1; i < n-1; i++ {
		t[i] = slope3(xs[i-1], ys[i-1], xs[i], ys[i], xs[i+1], ys[i+1])
	}
	t[0] = slope2(xs[0], ys[0], xs[1], ys[1], t[1])
	t[n-1] = slope2(xs[n-2], ys[n-2], xs[n-1], ys[n-1], t[n-2])
	for i := 0; i < n-1; i++ {
		seg(xs[i], ys[i], xs[i+1], ys[i+1], t[i], t[i+1])
	}
}

// pathLength is a path's arc length.
func pathLength(segs []scene.Seg) float64 {
	l := 0.0
	var cur, start P
	for _, s := range segs {
		switch s.Op {
		case scene.MoveTo:
			cur, start = s.P[0], s.P[0]
		case scene.LineTo:
			l += math.Hypot(s.P[0].X-cur.X, s.P[0].Y-cur.Y)
			cur = s.P[0]
		case scene.QuadTo:
			p0, p1, p2 := cur, s.P[0], s.P[1]
			l += curveLen(func(t float64) P {
				u := 1 - t
				return P{X: u*u*p0.X + 2*u*t*p1.X + t*t*p2.X, Y: u*u*p0.Y + 2*u*t*p1.Y + t*t*p2.Y}
			})
			cur = p2
		case scene.CubeTo:
			p0, p1, p2, p3 := cur, s.P[0], s.P[1], s.P[2]
			l += curveLen(func(t float64) P {
				u := 1 - t
				a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
				return P{X: a*p0.X + b*p1.X + c*p2.X + d*p3.X, Y: a*p0.Y + b*p1.Y + c*p2.Y + d*p3.Y}
			})
			cur = p3
		case scene.Close:
			l += math.Hypot(start.X-cur.X, start.Y-cur.Y)
			cur = start
		}
	}
	return l
}

func curveLen(f func(t float64) P) float64 {
	const n = 64
	l := 0.0
	prev := f(0)
	for i := 1; i <= n; i++ {
		p := f(float64(i) / n)
		l += math.Hypot(p.X-prev.X, p.Y-prev.Y)
		prev = p
	}
	return l
}

// segAngle is the direction a path leaves its start or reaches its end.
func segAngle(segs []scene.Seg, start bool) float64 {
	var cur, sub P
	type tan struct{ a, b P } // tangent at start: a->b ; at end: a->b
	var starts, ends []tan
	for _, s := range segs {
		switch s.Op {
		case scene.MoveTo:
			cur, sub = s.P[0], s.P[0]
		case scene.LineTo:
			starts = append(starts, tan{cur, s.P[0]})
			ends = append(ends, tan{cur, s.P[0]})
			cur = s.P[0]
		case scene.QuadTo:
			starts = append(starts, tan{cur, s.P[0]}, tan{cur, s.P[1]})
			ends = append(ends, tan{cur, s.P[1]}, tan{s.P[0], s.P[1]})
			cur = s.P[1]
		case scene.CubeTo:
			starts = append(starts, tan{cur, s.P[0]}, tan{cur, s.P[1]}, tan{cur, s.P[2]})
			ends = append(ends, tan{cur, s.P[2]}, tan{s.P[0], s.P[2]}, tan{s.P[1], s.P[2]})
			cur = s.P[2]
		case scene.Close:
			starts = append(starts, tan{cur, sub})
			ends = append(ends, tan{cur, sub})
			cur = sub
		}
	}
	if start {
		for _, t := range starts {
			if t.a != t.b {
				return math.Atan2(t.b.Y-t.a.Y, t.b.X-t.a.X)
			}
		}
		return 0
	}
	for i := len(ends) - 1; i >= 0; i-- {
		t := ends[i]
		if t.a != t.b {
			return math.Atan2(t.b.Y-t.a.Y, t.b.X-t.a.X)
		}
	}
	return 0
}

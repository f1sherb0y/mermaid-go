package scene

import "math"

// TextBox gives a text's box in its own coordinates (before Rotate), as
// SVG's getBBox would: the runs' advance width placed by the anchor, the
// line's ascent and descent placed by the baseline.
type TextBox func(t *Text) Rect

// BoundsOf is the box of items as SVG's getBBox: geometry only (no stroke
// widths), curves to their extremes, transforms applied.
func BoundsOf(items []Item, text TextBox) (Rect, bool) {
	b := box{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	b.items(items, Identity, text)
	if b.minX > b.maxX {
		return Rect{}, false
	}
	return Rect{b.minX, b.minY, b.maxX - b.minX, b.maxY - b.minY}, true
}

type box struct{ minX, minY, maxX, maxY float64 }

func (b *box) add(m Matrix, x, y float64) {
	x, y = m.Apply(x, y)
	b.minX, b.minY = math.Min(b.minX, x), math.Min(b.minY, y)
	b.maxX, b.maxY = math.Max(b.maxX, x), math.Max(b.maxY, y)
}

func (b *box) items(items []Item, m Matrix, text TextBox) {
	for _, it := range items {
		switch it := it.(type) {
		case *Group:
			b.items(it.Items, it.Transform.orIdentity().Then(m), text)
		case *Path:
			b.path(it.Segs, m)
		case *Text:
			if text == nil {
				continue
			}
			r := text(it)
			tm := m
			if it.Rotate != 0 {
				tm = Translate(-it.X, -it.Y).Then(Rotate(it.Rotate)).Then(Translate(it.X, it.Y)).Then(m)
			}
			b.add(tm, r.X, r.Y)
			b.add(tm, r.X+r.W, r.Y)
			b.add(tm, r.X, r.Y+r.H)
			b.add(tm, r.X+r.W, r.Y+r.H)
		}
	}
}

func (m Matrix) orIdentity() Matrix {
	if m == (Matrix{}) {
		return Identity
	}
	return m
}

func (b *box) path(segs []Seg, m Matrix) {
	var cur Pt
	for _, s := range segs {
		switch s.Op {
		case MoveTo, LineTo:
			cur = s.P[0]
			b.add(m, cur.X, cur.Y)
		case QuadTo:
			// As a cubic.
			c1 := Pt{cur.X + 2.0/3*(s.P[0].X-cur.X), cur.Y + 2.0/3*(s.P[0].Y-cur.Y)}
			c2 := Pt{s.P[1].X + 2.0/3*(s.P[0].X-s.P[1].X), s.P[1].Y + 2.0/3*(s.P[0].Y-s.P[1].Y)}
			b.cubic(m, cur, c1, c2, s.P[1])
			cur = s.P[1]
		case CubeTo:
			b.cubic(m, cur, s.P[0], s.P[1], s.P[2])
			cur = s.P[2]
		}
	}
}

// cubic adds a cubic's end points and its extremes in x and y (in the
// curve's own space; a rotation would want them in the transformed space,
// close enough for the boxes diagrams use).
func (b *box) cubic(m Matrix, p0, p1, p2, p3 Pt) {
	b.add(m, p3.X, p3.Y)
	at := func(t float64) Pt {
		u := 1 - t
		return Pt{
			u*u*u*p0.X + 3*u*u*t*p1.X + 3*u*t*t*p2.X + t*t*t*p3.X,
			u*u*u*p0.Y + 3*u*u*t*p1.Y + 3*u*t*t*p2.Y + t*t*t*p3.Y,
		}
	}
	for _, axis := range [2][4]float64{{p0.X, p1.X, p2.X, p3.X}, {p0.Y, p1.Y, p2.Y, p3.Y}} {
		// Roots of the derivative: a t² + b t + c.
		a := -axis[0] + 3*axis[1] - 3*axis[2] + axis[3]
		bb := 2 * (axis[0] - 2*axis[1] + axis[2])
		c := axis[1] - axis[0]
		for _, t := range quadRoots(a, bb, c) {
			if t > 0 && t < 1 {
				p := at(t)
				b.add(m, p.X, p.Y)
			}
		}
	}
}

func quadRoots(a, b, c float64) []float64 {
	if math.Abs(a) < 1e-12 {
		if math.Abs(b) < 1e-12 {
			return nil
		}
		return []float64{-c / b}
	}
	d := b*b - 4*a*c
	if d < 0 {
		return nil
	}
	s := math.Sqrt(d)
	return []float64{(-b + s) / (2 * a), (-b - s) / (2 * a)}
}

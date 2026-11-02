package scene

import (
	"math"
	"strings"
	"testing"
)

// The cubics an arc becomes stay on its circle.
func TestArcsStayOnTheCircle(t *testing.T) {
	segs := Ellipse(10, 20, 30, 30)
	for _, s := range segs {
		if s.Op != CubeTo {
			continue
		}
		// The curve's midpoint, from its control points.
		p0 := s.P[0]
		_ = p0
	}
	var b PathBuilder
	b.MoveTo(40, 20)
	b.ArcTo(30, 30, 0, false, true, 10, 50) // a quarter turn, clockwise on screen
	c := b.Segs[1]
	if c.Op != CubeTo || c.P[2] != (Pt{10, 50}) {
		t.Fatalf("segs %v", b.Segs)
	}
	start := b.Segs[0].P[0]
	mid := func(t float64) Pt {
		u := 1 - t
		x := u*u*u*start.X + 3*u*u*t*c.P[0].X + 3*u*t*t*c.P[1].X + t*t*t*c.P[2].X
		y := u*u*u*start.Y + 3*u*u*t*c.P[0].Y + 3*u*t*t*c.P[1].Y + t*t*t*c.P[2].Y
		return Pt{x, y}
	}
	for _, tt := range []float64{0.25, 0.5, 0.75} {
		p := mid(tt)
		if r := math.Hypot(p.X-10, p.Y-20); math.Abs(r-30) > 0.05 {
			t.Errorf("t=%v radius %v", tt, r)
		}
	}
}

func TestSVG(t *testing.T) {
	s := &Scene{ViewBox: Rect{0, 0, 100, 50}, Items: []Item{
		&Path{Segs: RectPath(1, 1, 98, 48, 5, 5), Fill: RGB(255, 255, 255), Stroke: RGB(40, 37, 61), StrokeWidth: 2, Shadow: &Shadow{DX: 4, DY: 4, Color: Color{0, 0, 0, 0.06}}},
		&Text{X: 50, Y: 25, Anchor: Middle, Baseline: Central, Family: `"Recursive Variable", arial`, Size: 14, Color: RGB(40, 37, 61), Runs: []Span{{Text: "a < b"}, {Text: " bold", Weight: 700}}},
	}}
	svg := s.SVG()
	for _, want := range []string{`viewBox="0 0 100 50"`, `feDropShadow dx="4" dy="4"`, `a &lt; b`, `font-weight="700"`, `font-family="&quot;Recursive Variable&quot;, arial"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("no %s in %s", want, svg)
		}
	}
}

func TestParsePath(t *testing.T) {
	for d, want := range map[string]string{
		"M10 10h5v5H10z":           "M10,10 L15,10 L15,15 L10,15 Z",
		"m1,1 2,0 0,2":             "M1,1 L3,1 L3,3",
		"M0 0L1.5.5-.5-1.5e1":      "M0,0 L1.5,0.5 L-0.5,-15",
		"M0 0c1 0 2 1 2 2s1 2 2 2": "M0,0 C1,0 2,1 2,2 C2,3 3,4 4,4",
		"M0 0q1 0 1 1t1 1":         "M0,0 Q1,0 1,1 Q1,2 2,2",
		"M0 0V5 x":                 "M0,0 L0,5",
		"M2 0a2 2 0 012 2":         "",
	} {
		got := PathData(ParsePath(d))
		if want == "" {
			// A compact arc: flags without separators, a quarter circle.
			segs := ParsePath(d)
			if len(segs) != 2 || segs[1].Op != CubeTo || segs[1].P[2] != (Pt{4, 2}) {
				t.Errorf("%q: %s", d, got)
			}
			continue
		}
		if got != want {
			t.Errorf("%q: %s, want %s", d, got, want)
		}
	}
}

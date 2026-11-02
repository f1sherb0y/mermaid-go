package render

import (
	"math"
	"strings"

	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

// Drawing edges (edges.js insertEdge, positionEdgeLabel).

// edgePathPoints are the points the curve goes through: rounded paths get
// their ends pulled back for the markers; others are pre-rounded at right
// angles (fixCorners) and offset by getLineFunctionsWithOffset.
func edgeLinePath(e *estate) []scene.Seg {
	pts := e.points
	if e.curve == "rounded" {
		return roundedPath(applyMarkerOffsets(pts, e), 5)
	}
	pts = fixCorners(pts)
	return curvePath(e.curve, lineWithOffset(pts, e))
}

// applyMarkerOffsets pulls a path's first and last points back along their
// segments by the markers' offsets.
func applyMarkerOffsets(pts []P, e *estate) []P {
	out := append([]P(nil), pts...)
	n := len(pts)
	if n >= 2 {
		if off := markerOffsets[e.arrowStart]; off != 0 {
			a := math.Atan2(pts[1].Y-pts[0].Y, pts[1].X-pts[0].X)
			out[0] = P{X: pts[0].X + off*math.Cos(a), Y: pts[0].Y + off*math.Sin(a)}
		}
		if off := markerOffsets[e.arrowEnd]; off != 0 {
			a := math.Atan2(pts[n-1].Y-pts[n-2].Y, pts[n-1].X-pts[n-2].X)
			out[n-1] = P{X: pts[n-1].X - off*math.Cos(a), Y: pts[n-1].Y - off*math.Sin(a)}
		}
	}
	return out
}

// lineWithOffset applies getLineFunctionsWithOffset's x and y accessors.
func lineWithOffset(pts []P, e *estate) []P {
	n := len(pts)
	if n == 0 {
		return pts
	}
	out := make([]P, n)
	first, last := pts[0], pts[n-1]
	so, sHas := markerOffsets[e.arrowStart]
	eo, eHas := markerOffsets[e.arrowEnd]
	angle := func(a, b P) (float64, float64, float64) {
		dx, dy := b.X-a.X, b.Y-a.Y
		return math.Atan(dy / dx), dx, dy
	}
	sgn := func(v float64) float64 {
		if v >= 0 {
			return 1
		}
		return -1
	}
	right := !(first.X < last.X)
	up := !(first.Y < last.Y)
	for i, d := range pts {
		ox, oy := 0.0, 0.0
		if i == 0 && sHas && n > 1 {
			a, dx, dy := angle(pts[0], pts[1])
			ox = so * math.Cos(a) * sgn(dx)
			oy = so * math.Abs(math.Sin(a)) * sgn(dy)
		} else if i == n-1 && eHas && n > 1 {
			a, dx, dy := angle(pts[n-1], pts[n-2])
			ox = eo * math.Cos(a) * sgn(dx)
			oy = eo * math.Abs(math.Sin(a)) * sgn(dy)
		}
		if math.IsNaN(ox) {
			ox = 0
		}
		if math.IsNaN(oy) {
			oy = 0
		}
		// Points closer to an end than its marker are pushed clear of it.
		if eHas {
			dxEnd, dyEnd := math.Abs(d.X-last.X), math.Abs(d.Y-last.Y)
			if dxEnd < eo && dxEnd > 0 && dyEnd < eo {
				adj := eo + 1 - dxEnd
				if right {
					adj = -adj
				}
				ox -= adj
			}
			if dyEnd < eo && dyEnd > 0 && dxEnd < eo {
				adj := eo + 1 - dyEnd
				if up {
					adj = -adj
				}
				oy -= adj
			}
		}
		if sHas {
			dxS, dyS := math.Abs(d.X-first.X), math.Abs(d.Y-first.Y)
			if dxS < so && dxS > 0 && dyS < so {
				adj := so + 1 - dxS
				if right {
					adj = -adj
				}
				ox += adj
			}
			if dyS < so && dyS > 0 && dxS < so {
				adj := so + 1 - dyS
				if up {
					adj = -adj
				}
				oy += adj
			}
		}
		out[i] = P{X: d.X + ox, Y: d.Y + oy}
	}
	return out
}

// roundedPath is generateRoundedPath: straight segments with each corner
// rounded by a quadratic curve of the given radius (at most half of either
// segment).
func roundedPath(pts []P, radius float64) []scene.Seg {
	if len(pts) < 2 {
		return nil
	}
	var b scene.PathBuilder
	const eps = 1e-5
	for i, c := range pts {
		switch {
		case i == 0:
			b.MoveTo(c.X, c.Y)
		case i == len(pts)-1:
			b.LineTo(c.X, c.Y)
		default:
			p, n := pts[i-1], pts[i+1]
			dx1, dy1 := c.X-p.X, c.Y-p.Y
			dx2, dy2 := n.X-c.X, n.Y-c.Y
			l1, l2 := math.Hypot(dx1, dy1), math.Hypot(dx2, dy2)
			if l1 < eps || l2 < eps {
				b.LineTo(c.X, c.Y)
				continue
			}
			nx1, ny1, nx2, ny2 := dx1/l1, dy1/l1, dx2/l2, dy2/l2
			ang := math.Acos(math.Max(-1, math.Min(1, nx1*nx2+ny1*ny2)))
			if ang < eps || math.Abs(math.Pi-ang) < eps {
				b.LineTo(c.X, c.Y)
				continue
			}
			cut := math.Min(radius/math.Sin(ang/2), math.Min(l1/2, l2/2))
			b.LineTo(c.X-nx1*cut, c.Y-ny1*cut)
			b.QuadTo(c.X, c.Y, c.X+nx2*cut, c.Y+ny2*cut)
		}
	}
	return b.Segs
}

// fixCorners pre-rounds right-angle corners for the non-rounded curves.
func fixCorners(pts []P) []P {
	corner := map[int]bool{}
	for i := 1; i+1 < len(pts); i++ {
		p, c, n := pts[i-1], pts[i], pts[i+1]
		if p.X == c.X && c.Y == n.Y && math.Abs(c.X-n.X) > 5 && math.Abs(c.Y-p.Y) > 5 ||
			p.Y == c.Y && c.X == n.X && math.Abs(c.X-p.X) > 5 && math.Abs(c.Y-n.Y) > 5 {
			corner[i] = true
		}
	}
	if len(corner) == 0 {
		return pts
	}
	adj := func(a, b P, d float64) P {
		dx, dy := b.X-a.X, b.Y-a.Y
		l := math.Hypot(dx, dy)
		r := d / l
		return P{X: b.X - r*dx, Y: b.Y - r*dy}
	}
	var out []P
	for i, c := range pts {
		if !corner[i] {
			out = append(out, c)
			continue
		}
		p, n := pts[i-1], pts[i+1]
		np, nn := adj(p, c, 5), adj(n, c, 5)
		xd, yd := nn.X-np.X, nn.Y-np.Y
		out = append(out, np)
		a := math.Sqrt2 * 2
		nc := c
		if math.Abs(n.X-p.X) > 10 && math.Abs(n.Y-p.Y) >= 10 {
			const r = 5.0
			if c.X == np.X {
				nc = P{X: np.X + r - a, Y: np.Y + a}
				if xd < 0 {
					nc.X = np.X - r + a
				}
				if yd < 0 {
					nc.Y = np.Y - a
				}
			} else {
				nc = P{X: np.X + a, Y: np.Y + r - a}
				if xd < 0 {
					nc.X = np.X - a
				}
				if yd < 0 {
					nc.Y = np.Y - r + a
				}
			}
		}
		out = append(out, nc, nn)
	}
	return out
}

// edgeCSS is the edge path's inline style: its classes' styles, then its
// own.
func edgeCSS(e *estate) []string {
	var out []string
	for _, s := range e.CSSCompiledStyles {
		k, _, _ := strings.Cut(s, ":")
		if !isLabelStyle(strings.TrimSpace(k)) {
			out = append(out, s)
		}
	}
	return append(out, e.style...)
}

// drawEdge draws an edge's path and markers.
func (r *renderer) drawEdge(e *estate) []scene.Item {
	if len(e.points) == 0 || e.IsLayoutOnly {
		return nil
	}
	segs := edgeLinePath(e)
	if len(segs) == 0 {
		return nil
	}
	css := edgeCSS(e)
	s := r.sheet
	p := paint{Stroke: s.edgeStrokeFor(e), Width: s.edgeWidthFor(e)}
	switch e.thickness {
	case "thick":
		p.Width = 3.5
	case "invisible":
		p.Width = 0
	}
	switch e.pattern {
	case "dashed":
		p.Dash = []float64{3}
	case "dotted":
		p.Dash = []float64{2}
	}
	classStyles := strings.Join(css[:len(css)-len(e.style)], ";")
	animated := e.Animate || e.Animation != "" || strings.Contains(classStyles, "animation")
	if e.Look == "neo" && !animated {
		l := pathLength(segs)
		os, oe := markerOffsets2[e.arrowStart], markerOffsets2[e.arrowEnd]
		if e.pattern == "dotted" || e.pattern == "dashed" {
			pairs := math.Floor((l - os - oe) / 4)
			if math.IsNaN(pairs) || pairs < 0 {
				pairs = 0
			}
			p.Dash = []float64{0, os}
			for range int(pairs) {
				p.Dash = append(p.Dash, 2, 2)
			}
			p.Dash = append(p.Dash, oe)
		} else {
			p.Dash = []float64{0, os, l - os - oe, oe}
		}
	}
	p = p.withCSS(css...)
	p.Fill = scene.None
	path := p.path(segs)
	if animated {
		path.Dash = []float64{9, 5}
		path.Cap = scene.RoundCap
	}
	if p.Width == 0 || p.Stroke.IsNone() {
		path.Stroke = scene.None
	}
	items := []scene.Item{path}
	// The edge's own stroke color colors its markers.
	color := scene.None
	if v, ok := style.Parse(css...)["stroke"]; ok {
		color = style.Color(v)
	}
	margin := !animated && e.Look == "neo"
	for _, end := range []struct {
		t     string
		start bool
	}{{e.arrowStart, true}, {e.arrowEnd, false}} {
		m, filled, name := r.marker(end.t, end.start, margin)
		if m == nil {
			continue
		}
		pt := e.markerPoint(segs, end.start)
		items = append(items, r.drawMarker(m, name, pt, segAngle(segs, end.start), p.Width, color, filled))
	}
	return items
}

// markerPoint is where a marker sits: the drawn path's first or last point.
func (e *estate) markerPoint(segs []scene.Seg, start bool) P {
	if start {
		return segs[0].P[0]
	}
	last := segs[len(segs)-1]
	switch last.Op {
	case scene.QuadTo:
		return last.P[1]
	case scene.CubeTo:
		return last.P[2]
	}
	return last.P[0]
}

// drawEdgeLabel draws an edge's label centred at its position, on the
// label background, and its end labels.
func (r *renderer) drawEdgeLabel(e *estate) []scene.Item {
	var items []scene.Item
	if e.label != nil && e.Label != "" {
		x, y := e.x, e.y+r.subGraphTitleTotal()/2
		l := e.label
		if bg := r.sheet.edgeLabelBgFor(e); !bg.IsNone() && l.H > 0 {
			if r.svgLabels {
				bg.A *= 0.5 // .edgeLabel rect { opacity: 0.5 }
			}
			items = append(items, &scene.Path{Segs: scene.RectPath(x-e.w/2, y-e.h/2, e.w, e.h, 0, 0), Fill: bg})
		}
		items = append(items, r.drawLabel(l, x-l.W/2, y-l.H/2)...)
	}
	for _, k := range []string{"startLeft", "startRight", "endLeft", "endRight"} {
		t := e.terminals[k]
		if t == nil {
			continue
		}
		c := t.center
		if !t.placed {
			c = r.terminalPosition(e, k)
		}
		items = append(items, r.drawLabel(t.label, c.X-t.w/2, c.Y-t.h/2)...)
	}
	return items
}

func (r *renderer) subGraphTitleTotal() float64 {
	return r.cfg.Num("flowchart.subGraphTitleMargin.top", 0) + r.cfg.Num("flowchart.subGraphTitleMargin.bottom", 0)
}

// terminalPosition is calcTerminalLabelPosition: beside the path, a little
// in from the end.
func (r *renderer) terminalPosition(e *estate, key string) P {
	pts := e.points
	if len(pts) == 0 {
		return P{X: e.x, Y: e.y}
	}
	atStart := strings.HasPrefix(key, "start")
	marker := e.arrowEnd
	if atStart {
		marker = e.arrowStart
	}
	off := 0.0
	if marker != "" {
		off = 10
	}
	if !atStart {
		pts = reversed(pts)
	}
	// Walk 25 + offset along the path.
	dist := 25 + off
	prev := pts[0]
	center := prev
	total := 0.0
	for _, p := range pts {
		seg := math.Hypot(p.X-prev.X, p.Y-prev.Y)
		if seg > 0 && total+seg >= dist {
			t := (dist - total) / seg
			center = P{X: prev.X + t*(p.X-prev.X), Y: prev.Y + t*(p.Y-prev.Y)}
			break
		}
		total += seg
		prev = p
		center = p
	}
	d := 10 + off*0.5
	a := math.Atan2(pts[0].Y-center.Y, pts[0].X-center.X)
	x, y := 0.0, 0.0
	switch key {
	case "startLeft", "endRight":
		// Not symmetric in mermaid; see utils.calcTerminalLabelPosition.
		x = math.Sin(a+math.Pi)*d + (pts[0].X+center.X)/2
		y = -math.Cos(a+math.Pi)*d + (pts[0].Y+center.Y)/2
	default:
		x = math.Sin(a)*d + (pts[0].X+center.X)/2
		y = -math.Cos(a)*d + (pts[0].Y+center.Y)/2
	}
	_ = x
	return P{X: x, Y: y}
}

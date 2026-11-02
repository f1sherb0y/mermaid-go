package architecture

import (
	"math"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// Two layouts are made: mermaid's (fcose, fcose.go), so that a diagram
// looks as on mermaid.live, and ours on a grid (grid.go, pixels.go), tidy
// where fcose is not. The diagram takes mermaid's unless ours is clearly
// better, judged on the edges as mermaid would draw them (before any go
// around what is in the way, route.go):
//
//  1. faults. Overlaps first: nodes (icon and label) or sibling groups
//     that overlap, a member outside its group; mermaid's layout with any
//     loses (ours has none). Then edges through another node, label or
//     group title (which then go around): fewer wins. fcose failing
//     (contradicting constraints, where mermaid throws) loses.
//  2. readability: crossings between edges, edges whose sides would run
//     straight but that are not in line, edges that leave or enter against
//     their side (2 each). Ours wins with at least 2 less.
//  3. size: ours wins if mermaid's needs more than 3 times the area (a
//     diagram fcose blew apart).
//
// Ours alone is used where mermaid's is not made: fcose throws, its drawing
// grows past 10 times the area of ours while it runs (it would lose on
// size; stopping saves the time), or mermaid's constraints run into the
// thousands (maxRelative, fcose.go).

// quality is a layout's score.
type quality struct {
	overlaps, faults, crossings, offline, against int
	area                                          float64
}

func (q quality) readability() int { return q.crossings + q.offline + 2*q.against }

// gridBetter says that the grid layout (g) is clearly better than fcose's (f).
func gridBetter(f, g quality) bool {
	if f.overlaps != g.overlaps {
		return g.overlaps < f.overlaps
	}
	if f.faults != g.faults {
		return g.faults < f.faults
	}
	if f.readability()-g.readability() >= 2 {
		return true
	}
	return f.area > 3*g.area
}

// quality scores the laid-out diagram (nodes placed, boxes made).
func (l *layout) quality() quality {
	var q quality
	q.overlaps = l.overlaps()
	obs := l.obstacles()
	courses := make([][]scene.Pt, len(l.d.Edges))
	for i, e := range l.d.Edges {
		pts := l.course(e)
		courses[i] = pts
		n := len(pts)
		u, v := unit(e.FromSide), unit(e.ToSide)
		first, last := sub(pts[1], pts[0]), sub(pts[n-1], pts[n-2])
		if first.X*u.x+first.Y*u.y <= 0 || last.X*v.x+last.Y*v.y >= 0 {
			q.against++
		}
	through:
		for _, o := range obs {
			if o.node == e.From || o.node == e.To {
				continue
			}
			for k := 1; k < n; k++ {
				if crosses(pts[k-1], pts[k], o.r) {
					q.faults++
					break through
				}
			}
		}
		if isX(e.FromSide) == isX(e.ToSide) && (len(pts) != 2 || math.Abs(pts[0].X-pts[1].X) > 0.5 && math.Abs(pts[0].Y-pts[1].Y) > 0.5) {
			q.offline++
		}
	}
	for i := range courses {
		for j := i + 1; j < len(courses); j++ {
			if coursesCross(courses[i], courses[j]) {
				q.crossings++
			}
		}
	}
	q.area = l.extent()
	return q
}

// extent is the area of the box around the nodes and groups.
func (l *layout) extent() float64 {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	grow := func(r scene.Rect) {
		x0, y0, x1, y1 = math.Min(x0, r.X), math.Min(y0, r.Y), math.Max(x1, r.X+r.W), math.Max(y1, r.Y+r.H)
	}
	for _, n := range l.nodes {
		grow(l.itemBox(item{node: n}))
	}
	for _, g := range l.d.Groups {
		grow(l.box[g])
	}
	return (x1 - x0) * (y1 - y0)
}

// overlaps counts what check (render_test.go) forbids: siblings' boxes that
// overlap, members not strictly inside their group, nodes that overlap.
func (l *layout) overlaps() int {
	inner := func(a, b scene.Rect) bool {
		return a.X > b.X && a.Y > b.Y && a.X+a.W < b.X+b.W && a.Y+a.H < b.Y+b.H
	}
	apart := func(a, b scene.Rect) bool {
		return a.X+a.W <= b.X+1e-6 || b.X+b.W <= a.X+1e-6 || a.Y+a.H <= b.Y+1e-6 || b.Y+b.H <= a.Y+1e-6
	}
	k := 0
	for g, ks := range l.kids {
		for i, a := range ks {
			ra := l.itemBox(a)
			if g != nil && !inner(ra, l.box[g]) {
				k++
			}
			for _, b := range ks[i+1:] {
				if a.node != nil && b.node != nil {
					continue // counted below
				}
				if !apart(ra, l.itemBox(b)) {
					k++
				}
			}
		}
	}
	for i, a := range l.nodes {
		ra := l.itemBox(item{node: a})
		for _, b := range l.nodes[i+1:] {
			if !apart(ra, l.itemBox(item{node: b})) {
				k++
			}
		}
	}
	return k
}

// coursesCross reports that two edges' courses cross (not merely meet at
// an end they share).
func coursesCross(a, b []scene.Pt) bool {
	for i := 1; i < len(a); i++ {
		for j := 1; j < len(b); j++ {
			if segmentsCross(a[i-1], a[i], b[j-1], b[j]) {
				return true
			}
		}
	}
	return false
}

// segmentsCross reports a proper crossing: each segment strictly between
// the other's ends.
func segmentsCross(p, q, r, s scene.Pt) bool {
	d := func(a, b, c scene.Pt) float64 { return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X) }
	d1, d2 := d(r, s, p), d(r, s, q)
	d3, d4 := d(p, q, r), d(p, q, s)
	const eps = 1e-6
	return (d1 > eps && d2 < -eps || d1 < -eps && d2 > eps) && (d3 > eps && d4 < -eps || d3 < -eps && d4 > eps)
}

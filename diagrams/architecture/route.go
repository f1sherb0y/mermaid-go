package architecture

import (
	"math"
	"slices"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// Edges are drawn as mermaid draws them, straight between opposite sides
// and with one bend between perpendicular ones, where that course leaves
// and enters the nodes by their sides and passes no other node, label or
// group title. Elsewhere (sides that contradict each other, as a cycle of
// directions or an edge from a node's right side to its right side, or a
// course through another node) the edge goes around: out of its side, then
// along the fewest, shortest lines around what is in the way, into the
// other side.

const (
	stub      = 20 // how far an edge goes straight out of a side before it turns
	clearance = 20 // room around what an edge goes around
)

// course is an edge's course as mermaid draws it, except that a straight
// edge whose ends are not in line, and that shares neither side with
// another edge (a fan, where mermaid's slants read well), goes out of its
// sides and crosses over halfway, square.
func (l *layout) course(e *Edge) []scene.Pt {
	from := l.endpoint(e.From, e.FromSide, e.FromGroup, e.To)
	to := l.endpoint(e.To, e.ToSide, e.ToGroup, e.From)
	if isX(e.FromSide) == isX(e.ToSide) {
		if isX(e.FromSide) && math.Abs(from.Y-to.Y) > 0.5 && !l.fan(e) {
			m := (from.X + to.X) / 2
			return []scene.Pt{from, {X: m, Y: from.Y}, {X: m, Y: to.Y}, to}
		}
		if !isX(e.FromSide) && math.Abs(from.X-to.X) > 0.5 && !l.fan(e) {
			m := (from.Y + to.Y) / 2
			return []scene.Pt{from, {X: from.X, Y: m}, {X: to.X, Y: m}, to}
		}
		return []scene.Pt{from, to}
	}
	if isX(e.FromSide) {
		return []scene.Pt{from, {X: to.X, Y: from.Y}, to}
	}
	return []scene.Pt{from, {X: from.X, Y: to.Y}, to}
}

// obstacle is what an edge must not cross: a node's icon or label, a
// junction, a group's title.
type obstacle struct {
	r    scene.Rect
	node *Node
}

func (l *layout) obstacles() []obstacle {
	var out []obstacle
	for _, n := range l.nodes {
		p := l.at[n]
		switch {
		case l.stand[n]:
		case n.Junction:
			out = append(out, obstacle{scene.Rect{X: p.X - 4, Y: p.Y - 4, W: 8, H: 8}, n})
		default:
			// The icon and the label under it.
			h, w, b := l.iconSize/2, l.iconSize/2, l.iconSize/2
			if tb, ok := l.labels[n]; ok {
				w, b = math.Max(w, tb.w/2), l.textB[n]
			}
			out = append(out, obstacle{scene.Rect{X: p.X - w, Y: p.Y - h, W: 2 * w, H: h + b}, n})
		}
	}
	for _, g := range l.d.Groups {
		if r, ok := l.titleBox(g); ok {
			out = append(out, obstacle{r, nil})
		}
	}
	return out
}

// titleBox is the box of a group's icon and title.
func (l *layout) titleBox(g *Group) (scene.Rect, bool) {
	if g.Title == "" && g.Icon == "" {
		return scene.Rect{}, false
	}
	b := l.box[g]
	w, h := l.titleX(g)+l.titles[g], 18+l.desc
	if g.Icon != "" {
		h = math.Max(h, 23+l.desc)
		w = math.Max(w, 0.75*l.padding+1)
	}
	return scene.Rect{X: b.X, Y: b.Y, W: w + 2, H: h}, true
}

// routeBudget bounds the points that routing looks at in all, so that a
// diagram with hundreds of contradictory edges still lays out in time;
// past it, edges keep mermaid's course.
const routeBudget = 1 << 20

// route sets every edge's course.
func (l *layout) route() {
	l.routes = map[*Edge][]scene.Pt{}
	obs := l.obstacles()
	budget := routeBudget
	for _, e := range l.d.Edges {
		pts := l.course(e)
		if !l.fits(e, pts, obs) {
			if r := l.around(e, pts[0], pts[len(pts)-1], obs, &budget); r != nil {
				pts = r
			}
		}
		l.routes[e] = pts
	}
}

// fits reports that a course leaves and enters by the edge's sides and
// crosses nothing but its own ends.
func (l *layout) fits(e *Edge, pts []scene.Pt, obs []obstacle) bool {
	n := len(pts)
	u, v := unit(e.FromSide), unit(e.ToSide)
	first, last := sub(pts[1], pts[0]), sub(pts[n-1], pts[n-2])
	if first.X*u.x+first.Y*u.y <= 0 || last.X*v.x+last.Y*v.y >= 0 {
		return false
	}
	for _, o := range obs {
		if o.node == e.From || o.node == e.To {
			continue
		}
		for i := 1; i < n; i++ {
			if crosses(pts[i-1], pts[i], o.r) {
				return false
			}
		}
	}
	return true
}

func sub(a, b scene.Pt) scene.Pt { return scene.Pt{X: a.X - b.X, Y: a.Y - b.Y} }

// crosses reports that segment a-b passes through r's inside.
func crosses(a, b scene.Pt, r scene.Rect) bool {
	// Liang-Barsky clipping against r shrunk by a hair.
	const eps = 0.5
	x0, y0, x1, y1 := r.X+eps, r.Y+eps, r.X+r.W-eps, r.Y+r.H-eps
	if x0 >= x1 || y0 >= y1 {
		return false
	}
	dx, dy := b.X-a.X, b.Y-a.Y
	t0, t1 := 0.0, 1.0
	for _, c := range [4][2]float64{{-dx, a.X - x0}, {dx, x1 - a.X}, {-dy, a.Y - y0}, {dy, y1 - a.Y}} {
		p, q := c[0], c[1]
		if p == 0 {
			if q < 0 {
				return false
			}
			continue
		}
		t := q / p
		if p < 0 {
			t0 = math.Max(t0, t)
		} else {
			t1 = math.Min(t1, t)
		}
		if t0 > t1 {
			return false
		}
	}
	return true
}

// around finds a course from s, out of e's from side, to t, into its to
// side, on lines along the obstacles: short, with few turns; nil if there
// is none.
func (l *layout) around(e *Edge, s, t scene.Pt, obs []obstacle, budget *int) []scene.Pt {
	ds, dt := unit(e.FromSide), unit(e.ToSide)
	s1 := scene.Pt{X: s.X + ds.x*stub, Y: s.Y + ds.y*stub}
	t1 := scene.Pt{X: t.X + dt.x*stub, Y: t.Y + dt.y*stub}
	// Near the ends first, then further.
	for _, reach := range []float64{l.pitch, 4 * l.pitch, math.Inf(1)} {
		if p := search(s1, t1, ds, dt.neg(), obs, l.pitch/2, reach, budget); p != nil {
			return simplify(append(append([]scene.Pt{s}, p...), t))
		}
	}
	return nil
}

// search is A* from s1 heading ds to t1 arriving heading dt, over the
// points where the lines through the ends and along the obstacles' edges
// (clearance out) cross, within reach of the box around both ends; a turn
// costs as much as turn of line. It spends budget by the points.
func search(s1, t1 scene.Pt, ds, dt gp, obs []obstacle, turn, reach float64, budget *int) []scene.Pt {
	lo := scene.Pt{X: math.Min(s1.X, t1.X) - reach, Y: math.Min(s1.Y, t1.Y) - reach}
	hi := scene.Pt{X: math.Max(s1.X, t1.X) + reach, Y: math.Max(s1.Y, t1.Y) + reach}
	var rs []scene.Rect
	xs, ys := []float64{s1.X, t1.X}, []float64{s1.Y, t1.Y}
	if !math.IsInf(reach, 1) {
		xs, ys = append(xs, lo.X, hi.X), append(ys, lo.Y, hi.Y)
	}
	for _, o := range obs {
		r := scene.Rect{X: o.r.X - clearance, Y: o.r.Y - clearance, W: o.r.W + 2*clearance, H: o.r.H + 2*clearance}
		if r.X > hi.X || r.Y > hi.Y || r.X+r.W < lo.X || r.Y+r.H < lo.Y {
			continue
		}
		// The ends may be close to something; they stay reachable.
		if within(s1, r) || within(t1, r) {
			r = o.r
			if within(s1, r) || within(t1, r) {
				continue
			}
		}
		rs = append(rs, r)
		xs = append(xs, r.X, r.X+r.W)
		ys = append(ys, r.Y, r.Y+r.H)
	}
	clip := func(vs []float64, a, b float64) []float64 {
		slices.Sort(vs)
		return slices.DeleteFunc(slices.Compact(vs), func(v float64) bool { return v < a || v > b })
	}
	xs, ys = clip(xs, lo.X, hi.X), clip(ys, lo.Y, hi.Y)
	nx, ny := len(xs), len(ys)
	if nx*ny > *budget {
		*budget = 0
		return nil
	}
	*budget -= nx * ny
	find := func(vs []float64, v float64) int { k, _ := slices.BinarySearch(vs, v); return k }
	// The obstacles across each line (in) and across the middle of each
	// gap to the next line (mid); the lines include every obstacle's
	// edges, so a gap is inside one or not at all.
	colIn, colMid := make([][]int32, nx), make([][]int32, nx)
	rowIn, rowMid := make([][]int32, ny), make([][]int32, ny)
	for k, r := range rs {
		for i := find(xs, r.X); i < nx && xs[i] < r.X+r.W; i++ {
			if xs[i] > r.X {
				colIn[i] = append(colIn[i], int32(k))
			}
			colMid[i] = append(colMid[i], int32(k))
		}
		for j := find(ys, r.Y); j < ny && ys[j] < r.Y+r.H; j++ {
			if ys[j] > r.Y {
				rowIn[j] = append(rowIn[j], int32(k))
			}
			rowMid[j] = append(rowMid[j], int32(k))
		}
	}
	inY := func(list []int32, y float64) bool {
		for _, k := range list {
			if r := rs[k]; r.Y < y && y < r.Y+r.H {
				return true
			}
		}
		return false
	}
	inX := func(list []int32, x float64) bool {
		for _, k := range list {
			if r := rs[k]; r.X < x && x < r.X+r.W {
				return true
			}
		}
		return false
	}
	blocked := func(i, j int) bool { return inY(colIn[i], ys[j]) }
	dirs := [4]gp{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	// step is the point one step from (i, j) heading d, if it is clear.
	step := func(i, j, d int) (int, int, bool) {
		ni, nj := i+int(dirs[d].x), j+int(dirs[d].y)
		if ni < 0 || nj < 0 || ni >= nx || nj >= ny || blocked(ni, nj) {
			return 0, 0, false
		}
		switch d {
		case 0:
			return ni, nj, !inY(colMid[i], ys[j])
		case 1:
			return ni, nj, !inY(colMid[ni], ys[j])
		case 2:
			return ni, nj, !inX(rowMid[j], xs[i])
		}
		return ni, nj, !inX(rowMid[nj], xs[i])
	}
	si, sj, ti, tj := find(xs, s1.X), find(ys, s1.Y), find(xs, t1.X), find(ys, t1.Y)
	dirOf := func(d gp) int { return slices.Index(dirs[:], d) }
	// The estimate: the distance left, and a turn unless the target is
	// straight ahead and entered heading this way.
	h := func(i, j, d int) float64 {
		dx, dy := t1.X-xs[i], t1.Y-ys[j]
		v := math.Abs(dx) + math.Abs(dy)
		if dirs[d] != dt || dx*dirs[d].y != 0 || dy*dirs[d].x != 0 || dx*dirs[d].x+dy*dirs[d].y < 0 {
			v += turn
		}
		return v
	}
	key := func(i, j, d int) int { return (i*ny+j)*4 + d }
	cost, prev := make([]float64, nx*ny*4), make([]int32, nx*ny*4)
	for k := range cost {
		cost[k] = math.Inf(1)
	}
	st := key(si, sj, dirOf(ds))
	cost[st], prev[st] = 0, -1
	q := &queue{{st, h(si, sj, dirOf(ds)), 0}}
	end, best := -1, math.Inf(1)
	for len(*q) > 0 {
		c := q.pop()
		p, d := c.k/4, c.k%4
		i, j := p/ny, p%ny
		if c.cost >= best {
			break
		}
		if c.g > cost[c.k] {
			continue
		}
		if i == ti && j == tj {
			extra := 0.0
			switch {
			case dirs[d] == dt:
			case dirs[d] == dt.neg():
				extra = math.Inf(1)
			default:
				extra = turn
			}
			if c.g+extra < best {
				best, end = c.g+extra, c.k
			}
		}
		for nd := range dirs {
			if dirs[nd] == dirs[d].neg() {
				continue
			}
			ni, nj, ok := step(i, j, nd)
			if !ok {
				continue
			}
			w := math.Abs(xs[ni]-xs[i]) + math.Abs(ys[nj]-ys[j])
			if nd != d {
				w += turn
			}
			k := key(ni, nj, nd)
			if c.g+w < cost[k] {
				cost[k], prev[k] = c.g+w, int32(c.k)
				q.push(state{k, c.g + w + h(ni, nj, nd), c.g + w})
			}
		}
	}
	if end < 0 {
		return nil
	}
	var out []scene.Pt
	for k := end; k >= 0; k = int(prev[k]) {
		p := k / 4
		out = append(out, scene.Pt{X: xs[p/ny], Y: ys[p%ny]})
	}
	slices.Reverse(out)
	return out
}

func within(p scene.Pt, r scene.Rect) bool {
	return p.X > r.X && p.X < r.X+r.W && p.Y > r.Y && p.Y < r.Y+r.H
}

// simplify drops repeated points and the middle of straight runs.
func simplify(pts []scene.Pt) []scene.Pt {
	out := pts[:1]
	for _, p := range pts[1:] {
		if p == out[len(out)-1] {
			continue
		}
		if n := len(out); n >= 2 {
			a, b := out[n-2], out[n-1]
			if (a.X == b.X && b.X == p.X) || (a.Y == b.Y && b.Y == p.Y) {
				out[n-1] = p
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

// state is a point and heading in the search: its estimated total cost
// and its cost so far.
type state struct {
	k       int
	cost, g float64
}

// less orders states by estimated cost, then the furthest along first.
func (a state) less(b state) bool { return a.cost < b.cost || a.cost == b.cost && a.g > b.g }

// queue is a binary min-heap of states by cost.
type queue []state

func (q *queue) push(x state) {
	*q = append(*q, x)
	h := *q
	for i := len(h) - 1; i > 0; {
		p := (i - 1) / 2
		if !h[i].less(h[p]) {
			break
		}
		h[p], h[i] = h[i], h[p]
		i = p
	}
}

func (q *queue) pop() state {
	h := *q
	top := h[0]
	n := len(h) - 1
	h[0] = h[n]
	h = h[:n]
	for i := 0; ; {
		c := 2*i + 1
		if c >= n {
			break
		}
		if c+1 < n && h[c+1].less(h[c]) {
			c++
		}
		if !h[c].less(h[i]) {
			break
		}
		h[i], h[c] = h[c], h[i]
		i = c
	}
	*q = h
	return top
}

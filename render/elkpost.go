package render

import (
	"math"
	"strings"
)

// The ELK adapter's post-processing of edge routes (elk/render.ts and
// elk/geometry.ts).

const (
	terminalJogMax = 16
	terminalRunMax = 30
	jogEps         = 0.01
	borderEps      = 1
	pushOut        = 10
)

// markerOffsets: how far a path end is pulled back for its marker
// (utils/lineWithOffset.ts).
var markerOffsets = map[string]float64{
	"aggregation": 17.25, "extension": 17.25, "composition": 17.25, "dependency": 6,
	"lollipop": 13.5, "arrow_point": 4, "arrow_barb": 0, "arrow_barb_neo": 5.5,
}

// markerOffsets2: how much of a path's end the neo look leaves undrawn.
var markerOffsets2 = map[string]float64{"arrow_point": 4, "arrow_cross": 12.5, "arrow_circle": 12.5}

func markerPathOffset(t string) float64 { return math.Max(markerOffsets[t], markerOffsets2[t]) }

func onBorder(b box, p P, tol float64) bool {
	l, r, t, bt := b.X-b.W/2, b.X+b.W/2, b.Y-b.H/2, b.Y+b.H/2
	onL := math.Abs(p.X-l) <= tol && p.Y >= t-tol && p.Y <= bt+tol
	onR := math.Abs(p.X-r) <= tol && p.Y >= t-tol && p.Y <= bt+tol
	onT := math.Abs(p.Y-t) <= tol && p.X >= l-tol && p.X <= r+tol
	onB := math.Abs(p.Y-bt) <= tol && p.X >= l-tol && p.X <= r+tol
	return onL || onR || onT || onB
}

func outsideNode(b box, p P) bool {
	return math.Abs(p.X-b.X) >= b.W/2 || math.Abs(p.Y-b.Y) >= b.H/2
}

// borderIntersection is geometry.ts's intersection: where the segment from
// inside to outside crosses the box.
func borderIntersection(b box, outside, inside P) P {
	x, y := b.X, b.Y
	w, h := b.W/2, b.H/2
	Q := math.Abs(outside.Y - inside.Y)
	R := math.Abs(outside.X - inside.X)
	if math.Abs(y-outside.Y)*w > math.Abs(x-outside.X)*h {
		var q float64
		if inside.Y < outside.Y {
			q = outside.Y - h - y
		} else {
			q = y - h - outside.Y
		}
		r := R * q / Q
		res := P{}
		if inside.X < outside.X {
			res.X = inside.X + r
		} else {
			res.X = inside.X - R + r
		}
		if inside.Y < outside.Y {
			res.Y = inside.Y + Q - q
		} else {
			res.Y = inside.Y - Q + q
		}
		if R == 0 {
			res.X = outside.X
		}
		if Q == 0 {
			res.Y = outside.Y
		}
		return res
	}
	var r float64
	if inside.X < outside.X {
		r = outside.X - w - x
	} else {
		r = x - w - outside.X
	}
	q := Q * r / R
	res := P{}
	if inside.X < outside.X {
		res.X = inside.X + R - r
	} else {
		res.X = inside.X - R + r
	}
	if inside.Y < outside.Y {
		res.Y = inside.Y + q
	} else {
		res.Y = inside.Y - q
	}
	if R == 0 {
		res.X = outside.X
	}
	if Q == 0 {
		res.Y = outside.Y
	}
	return res
}

func ensureTrulyOutside(b box, p P) P {
	dx, dy := math.Abs(p.X-b.X), math.Abs(p.Y-b.Y)
	if math.Abs(dx-b.W/2) < borderEps || math.Abs(dy-b.H/2) < borderEps {
		dirX, dirY := p.X-b.X, p.Y-b.Y
		l := math.Hypot(dirX, dirY)
		if l > 0 {
			return P{X: b.X + dirX/l*(l+pushOut), Y: b.Y + dirY/l*(l+pushOut)}
		}
	}
	return p
}

func makeInsidePoint(b box, outside, center P) P {
	vertical := math.Abs(outside.X-b.X) < borderEps
	horizontal := math.Abs(outside.Y-b.Y) < borderEps
	p := P{X: outside.X, Y: center.Y}
	if !vertical {
		if outside.X < b.X {
			p.X = b.X - b.W/4
		} else {
			p.X = b.X + b.W/4
		}
	}
	if horizontal {
		p.Y = outside.Y
	}
	return p
}

// nodeIntersect is the node's own intersect at its placed box, if any.
func (n *nstate) nodeIntersect(p P) (P, bool) {
	if n.IsGroup || n.shape == nil || n.shape.intersect == nil {
		return P{}, false
	}
	return n.shape.intersect(box{X: n.x, Y: n.y, W: n.w, H: n.h}, p), true
}

func tryNodeIntersect(n *nstate, b box, outside P) (P, bool) {
	res, ok := n.nodeIntersect(outside)
	if !ok {
		return P{}, false
	}
	if (outside.X < b.X && res.X > b.X) || (outside.X > b.X && res.X < b.X) {
		return P{}, false
	}
	if math.Hypot(outside.X-res.X, outside.Y-res.Y) <= borderEps {
		return P{}, false
	}
	return res, true
}

func computeNodeIntersection(n *nstate, b box, outside, center P) P {
	o2 := ensureTrulyOutside(b, outside)
	if p, ok := tryNodeIntersect(n, b, o2); ok {
		return p
	}
	return borderIntersection(b, o2, makeInsidePoint(b, o2, center))
}

func insideOutline(n *nstate, centre, probe P) bool {
	c, ok := n.nodeIntersect(probe)
	if !ok {
		return false
	}
	return math.Hypot(probe.X-centre.X, probe.Y-centre.Y) <= math.Hypot(c.X-centre.X, c.Y-centre.Y)+1e-9
}

// outlineAttachPoint is where the outline meets the line along the edge's
// departure axis through the port: keeps ELK's straight stubs straight on
// non-rectangular shapes.
func outlineAttachPoint(n *nstate, b box, port, next P) (P, bool) {
	if n.IsGroup || n.shape == nil || n.shape.intersect == nil {
		return P{}, false
	}
	dx, dy := next.X-port.X, next.Y-port.Y
	if dx == 0 && dy == 0 {
		return P{}, false
	}
	if math.Abs(dx) > 1e-6 && math.Abs(dy) > 1e-6 {
		return P{}, false
	}
	centre := P{X: b.X, Y: b.Y}
	horizontal := math.Abs(dx) > math.Abs(dy)
	along := func(t float64) P {
		if horizontal {
			return P{X: t, Y: port.Y}
		}
		return P{X: port.X, Y: t}
	}
	inner, outer := centre.Y, port.Y
	if horizontal {
		inner, outer = centre.X, port.X
	}
	if !insideOutline(n, centre, along(inner)) {
		return P{}, false
	}
	if insideOutline(n, centre, along(outer)) {
		return port, true
	}
	for range 20 {
		mid := (inner + outer) / 2
		if insideOutline(n, centre, along(mid)) {
			inner = mid
		} else {
			outer = mid
		}
	}
	return along(inner), true
}

func replaceEndpoint(pts []P, start bool, v P) []P {
	const tol = 0.1
	if len(pts) == 0 {
		return pts
	}
	if start {
		if math.Abs(pts[0].X-v.X) < tol && math.Abs(pts[0].Y-v.Y) < tol {
			return pts[1:]
		}
		pts[0] = v
		return pts
	}
	last := len(pts) - 1
	if math.Abs(pts[last].X-v.X) < tol && math.Abs(pts[last].Y-v.Y) < tol {
		return pts[:last]
	}
	pts[last] = v
	return pts
}

func approxEq(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// sanitizeElkPoints clips a route (node centres at its ends) to the two
// nodes' outlines.
func (r *renderer) sanitizeElkPoints(points []P, sn, tn *nstate) []P {
	prev := append([]P(nil), points...)
	sb, tb := sn.boundsBox(), tn.boundsBox()
	candidate := func(start bool, n *nstate) (P, bool) {
		if len(prev) == 0 {
			return P{X: n.x, Y: n.y}, true
		}
		if start {
			c := approxEq(prev[0].X, n.x) && approxEq(prev[0].Y, n.y)
			if c && len(prev) > 1 {
				return prev[1], true
			}
			return prev[0], c
		}
		last := prev[len(prev)-1]
		c := approxEq(last.X, n.x) && approxEq(last.Y, n.y)
		if c && len(prev) > 1 {
			return prev[len(prev)-2], true
		}
		return last, c
	}
	sc, sCenter := candidate(true, sn)
	tc, tCenter := candidate(false, tn)
	skipStart := sn.IsGroup && onBorder(sb, sc, 0.5)
	skipEnd := tn.IsGroup && onBorder(tb, tc, 0.5)
	if skipStart && sCenter && len(prev) > 0 {
		prev = prev[1:]
	}
	if skipEnd && tCenter && len(prev) > 0 {
		prev = prev[:len(prev)-1]
	}
	if sn.IsGroup && !skipStart {
		prev, skipStart = clipGroupEndpoint(prev, sb, true)
	}
	if tn.IsGroup && !skipEnd {
		prev, skipEnd = clipGroupEndpoint(prev, tb, false)
	}
	var out []P
	if skipStart || skipEnd {
		if !skipStart {
			for i, p := range prev {
				if outsideNode(sb, p) {
					prev = replaceEndpoint(prev, true, computeNodeIntersection(sn, sb, prev[i], prev[0]))
					break
				}
			}
		}
		if !skipEnd {
			for i := len(prev) - 1; i >= 0; i-- {
				if outsideNode(tb, prev[i]) {
					prev = replaceEndpoint(prev, false, computeNodeIntersection(tn, tb, prev[i], prev[len(prev)-1]))
					break
				}
			}
		}
		out = prev
	} else {
		out = cutter2(sn, tn, prev)
	}
	if len(out) < 2 || hasNaN(out) {
		var clean []P
		for _, p := range prev {
			if !math.IsNaN(p.X) && !math.IsNaN(p.Y) && !math.IsInf(p.X, 0) && !math.IsInf(p.Y, 0) {
				clean = append(clean, p)
			}
		}
		if len(clean) >= 2 {
			out = clean
		} else {
			out = prev
		}
	}
	return dedupe(out)
}

func hasNaN(pts []P) bool {
	for _, p := range pts {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			return true
		}
	}
	return false
}

func dedupe(pts []P) []P {
	var out []P
	for i, p := range pts {
		if i > 0 {
			q := pts[i-1]
			if math.Abs(p.X-q.X) <= 1e-6 && math.Abs(p.Y-q.Y) <= 1e-6 {
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

// clipGroupEndpoint cuts the route where it crosses a (moved) frame.
func clipGroupEndpoint(pts []P, b box, start bool) ([]P, bool) {
	step, idx := 1, 0
	if !start {
		step, idx = -1, len(pts)-1
	}
	term := idx
	for idx >= 0 && idx < len(pts) && !outsideNode(b, pts[idx]) {
		idx += step
	}
	if idx == term || idx < 0 || idx >= len(pts) {
		return pts, false
	}
	out, in := pts[idx], pts[idx-step]
	dx, dy := out.X-in.X, out.Y-in.Y
	tx, ty := math.Inf(1), math.Inf(1)
	if dx != 0 {
		tx = (b.X + sign(dx)*b.W/2 - in.X) / dx
	}
	if dy != 0 {
		ty = (b.Y + sign(dy)*b.H/2 - in.Y) / dy
	}
	t := math.Min(tx, ty)
	cross := P{X: in.X + t*dx, Y: in.Y + t*dy}
	if start {
		return append([]P{cross}, pts[idx:]...), true
	}
	return append(append([]P(nil), pts[:idx+1]...), cross), true
}

func sign(f float64) float64 {
	switch {
	case f > 0:
		return 1
	case f < 0:
		return -1
	}
	return 0
}

// cutter2 replaces the route's ends with where it leaves the start node and
// enters the end node.
func cutter2(sn, tn *nstate, orig []P) []P {
	sb, tb := sn.boundsBox(), tn.boundsBox()
	if len(orig) == 0 {
		return nil
	}
	pts := append([]P(nil), orig...)
	startCenter, endCenter := pts[0], pts[len(pts)-1]
	for i, p := range pts {
		if outsideNode(sb, p) {
			var v P
			ok := false
			if i+1 < len(pts) {
				v, ok = outlineAttachPoint(sn, sb, pts[i], pts[i+1])
			}
			if !ok {
				v = computeNodeIntersection(sn, sb, p, startCenter)
			}
			pts = replaceEndpoint(pts, true, v)
			break
		}
	}
	oi := -1
	for i := len(pts) - 1; i >= 0; i-- {
		if outsideNode(tb, pts[i]) {
			oi = i
			break
		}
	}
	if oi < 0 && len(pts) > 1 {
		oi = len(pts) - 2
	}
	if oi >= 0 {
		var v P
		ok := false
		if oi-1 >= 0 {
			v, ok = outlineAttachPoint(tn, tb, pts[oi], pts[oi-1])
		}
		if !ok {
			v = computeNodeIntersection(tn, tb, pts[oi], endCenter)
		}
		pts = replaceEndpoint(pts, false, v)
	}
	if n := len(pts); n > 1 && math.Hypot(pts[n-1].X-pts[n-2].X, pts[n-1].Y-pts[n-2].Y) < 2 {
		pts = pts[:n-1]
	}
	return pts
}

func ensureEndMarkerSegment(pts []P, b box, off float64) []P {
	if off <= 0 || len(pts) < 3 {
		return pts
	}
	end, entry := pts[len(pts)-1], pts[len(pts)-2]
	if math.Hypot(end.X-entry.X, end.Y-entry.Y) >= math.Max(minEndMarkerSegment, off*2) || !onBorder(b, entry, 1) {
		return pts
	}
	return append(append([]P(nil), pts[:len(pts)-2]...), end)
}

func ensureStartMarkerSegment(pts []P, b box, off float64) []P {
	if off <= 0 || len(pts) < 3 {
		return pts
	}
	start, exit := pts[0], pts[1]
	if math.Hypot(exit.X-start.X, exit.Y-start.Y) >= math.Max(minEndMarkerSegment, off*2) || !onBorder(b, exit, 1) {
		return pts
	}
	return append([]P{start}, pts[2:]...)
}

// Terminal jog straightening.

type stRun struct{ oldA, oldB, newA, newB P }

func axisOf(a, b P) byte {
	dx, dy := math.Abs(b.X-a.X), math.Abs(b.Y-a.Y)
	if dx > jogEps && dy <= jogEps {
		return 'h'
	}
	if dy > jogEps && dx <= jogEps {
		return 'v'
	}
	return 0
}

func straightenFront(pts []P) ([]P, stRun, bool) {
	if len(pts) < 5 {
		return nil, stRun{}, false
	}
	p0, p1, p2, p3 := pts[0], pts[1], pts[2], pts[3]
	ax := axisOf(p0, p1)
	other := byte('h')
	if ax == 'h' {
		other = 'v'
	}
	if ax == 0 || axisOf(p2, p3) != ax || axisOf(p1, p2) != other {
		return nil, stRun{}, false
	}
	if math.Hypot(p1.X-p0.X, p1.Y-p0.Y) > terminalRunMax {
		return nil, stRun{}, false
	}
	jog := math.Abs(p2.X - p1.X)
	if ax == 'h' {
		jog = math.Abs(p2.Y - p1.Y)
	}
	if jog < jogEps || jog > terminalJogMax {
		return nil, stRun{}, false
	}
	var fwd bool
	if ax == 'h' {
		fwd = sign(p1.X-p0.X) == sign(p3.X-p2.X)
	} else {
		fwd = sign(p1.Y-p0.Y) == sign(p3.Y-p2.Y)
	}
	if !fwd {
		return nil, stRun{}, false
	}
	last := 3
	for last+1 < len(pts) && axisOf(pts[last], pts[last+1]) == ax {
		last++
	}
	if last == len(pts)-1 {
		return nil, stRun{}, false
	}
	moved := append([]P(nil), pts...)
	for i := 2; i <= last; i++ {
		if ax == 'h' {
			moved[i] = P{X: pts[i].X, Y: p0.Y}
		} else {
			moved[i] = P{X: p0.X, Y: pts[i].Y}
		}
	}
	run := stRun{pts[2], pts[last], moved[2], moved[last]}
	moved = append(moved[:1], moved[3:]...)
	return moved, run, true
}

func straightenJogs(pts []P) ([]P, []stRun, bool) {
	var runs []stRun
	changed := false
	if f, run, ok := straightenFront(pts); ok {
		pts, changed = f, true
		runs = append(runs, run)
	}
	rev := reversed(pts)
	if b, run, ok := straightenFront(rev); ok {
		pts, changed = reversed(b), true
		runs = append(runs, run)
	}
	return pts, runs, changed
}

func reversed(pts []P) []P {
	out := make([]P, len(pts))
	for i, p := range pts {
		out[len(pts)-1-i] = p
	}
	return out
}

func segmentsCrossStrict(a1, a2, b1, b2 P) bool {
	side := func(o, p, q P) float64 { return (p.X-o.X)*(q.Y-o.Y) - (p.Y-o.Y)*(q.X-o.X) }
	d1, d2 := side(b1, b2, a1), side(b1, b2, a2)
	d3, d4 := side(a1, a2, b1), side(a1, a2, b2)
	return ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) && ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0))
}

func crossingCount(a, b []P) int {
	n := 0
	for i := 0; i+1 < len(a); i++ {
		for j := 0; j+1 < len(b); j++ {
			if segmentsCrossStrict(a[i], a[i+1], b[j], b[j+1]) {
				n++
			}
		}
	}
	return n
}

func paramOnSegment(p, a, b P) float64 {
	abx, aby := b.X-a.X, b.Y-a.Y
	l := abx*abx + aby*aby
	if l == 0 {
		return 0
	}
	return math.Max(0, math.Min(1, ((p.X-a.X)*abx+(p.Y-a.Y)*aby)/l))
}

func pointAt(a, b P, t float64) P { return P{X: a.X + t*(b.X-a.X), Y: a.Y + t*(b.Y-a.Y)} }

func projectLabelOntoRun(l P, runs []stRun) (P, bool) {
	best, found := math.Inf(1), false
	var out P
	for _, r := range runs {
		t := paramOnSegment(l, r.oldA, r.oldB)
		o := pointAt(r.oldA, r.oldB, t)
		d := (o.X-l.X)*(o.X-l.X) + (o.Y-l.Y)*(o.Y-l.Y)
		if !found || d < best {
			best, out, found = d, pointAt(r.newA, r.newB, t), true
		}
	}
	return out, found
}

type lbox struct{ x1, y1, x2, y2 float64 }

func segmentHitsBox(a, b P, bx lbox) bool {
	t0, t1 := 0.0, 1.0
	dx, dy := b.X-a.X, b.Y-a.Y
	for _, pq := range [][2]float64{{-dx, a.X - bx.x1}, {dx, bx.x2 - a.X}, {-dy, a.Y - bx.y1}, {dy, bx.y2 - a.Y}} {
		p, q := pq[0], pq[1]
		switch {
		case p == 0:
			if q < 0 {
				return false
			}
		case p < 0:
			t0 = math.Max(t0, q/p)
		default:
			t1 = math.Min(t1, q/p)
		}
	}
	return t0 < t1
}

func labelBox(c P, w, h float64) lbox { return lbox{c.X - w/2, c.Y - h/2, c.X + w/2, c.Y + h/2} }

type straightened struct {
	edge *estate
	runs []stRun
}

// straightenEdgeTerminals straightens the port-to-channel step of every
// edge where that buys no crossing and no label over an edge.
func straightenEdgeTerminals(edges []*estate) []straightened {
	routes := make([][]P, len(edges))
	type lab struct {
		c    P
		w, h float64
		ok   bool
	}
	labels := make([]lab, len(edges))
	for i, e := range edges {
		routes[i] = e.points
		if e.hasXY && e.Label != "" && e.w != 0 && e.h != 0 {
			labels[i] = lab{P{X: e.x, Y: e.y}, e.w, e.h, true}
		}
	}
	covers := func(route []P, l lab) bool {
		if !l.ok {
			return false
		}
		b := labelBox(l.c, l.w, l.h)
		for i := 0; i+1 < len(route); i++ {
			if segmentHitsBox(route[i], route[i+1], b) {
				return true
			}
		}
		return false
	}
	var changed []straightened
	for i, e := range edges {
		orig := routes[i]
		if len(orig) < 5 {
			continue
		}
		cand, runs, ok := straightenJogs(orig)
		if !ok {
			continue
		}
		l := labels[i]
		cl := l
		if l.ok {
			if m, ok := projectLabelOntoRun(l.c, runs); ok {
				cl.c = m
			}
		}
		before, after, cb, ca := 0, 0, 0, 0
		for j, route := range routes {
			if j == i || len(route) < 2 {
				continue
			}
			before += crossingCount(orig, route)
			after += crossingCount(cand, route)
			cb += b2i(covers(route, l)) + b2i(covers(orig, labels[j]))
			ca += b2i(covers(route, cl)) + b2i(covers(cand, labels[j]))
		}
		if after > before || ca > cb {
			continue
		}
		e.points = cand
		routes[i] = cand
		labels[i] = cl
		changed = append(changed, straightened{e, runs})
	}
	return changed
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// separateOppositeEdgeLabels pushes apart the labels of the two edges
// between one pair of nodes, one each way, when they overlap.
func separateOppositeEdgeLabels(edges []*estate) {
	type pair struct{ a, b string }
	byPair := map[pair][]*estate{}
	var order []pair
	for _, e := range edges {
		if !e.hasXY || e.Label == "" || e.w == 0 {
			continue
		}
		k := pair{e.Start, e.End}
		if k.b < k.a {
			k.a, k.b = k.b, k.a
		}
		if _, ok := byPair[k]; !ok {
			order = append(order, k)
		}
		byPair[k] = append(byPair[k], e)
	}
	for _, k := range order {
		bucket := byPair[k]
		if len(bucket) != 2 {
			continue
		}
		a, b := bucket[0], bucket[1]
		if math.Abs(a.y-b.y) > math.Max(a.h, b.h) {
			continue
		}
		required := a.w/2 + b.w/2 + 4
		cur := b.x - a.x
		deficit := required - math.Abs(cur)
		if deficit <= 0 {
			continue
		}
		s := sign(cur)
		if s == 0 {
			s = 1
		}
		a.x -= s * deficit / 2
		b.x += s * deficit / 2
	}
}

// followMovedEndpoints moves terminal labels with their path ends.
func followMovedEndpoints(e *estate, start, end P) {
	if len(e.points) == 0 {
		return
	}
	ms := P{X: e.points[0].X - start.X, Y: e.points[0].Y - start.Y}
	me := P{X: e.points[len(e.points)-1].X - end.X, Y: e.points[len(e.points)-1].Y - end.Y}
	for k, t := range e.terminals {
		if !t.placed {
			continue
		}
		d := me
		if strings.HasPrefix(k, "start") {
			d = ms
		}
		t.center = P{X: t.center.X + d.X, Y: t.center.Y + d.Y}
	}
}

// terminal-label obstacles.
type obstacles struct {
	nodes  []lbox
	frames [][2]P
	labels []lbox
	edges  []*estate
}

func (r *renderer) terminalObstacles() *obstacles {
	o := &obstacles{edges: r.edges}
	for _, n := range r.nodes {
		if !n.laidOut {
			continue
		}
		b := labelBox(P{X: n.x, Y: n.y}, n.w, n.h)
		if !n.IsGroup {
			o.nodes = append(o.nodes, b)
			continue
		}
		o.frames = append(o.frames,
			[2]P{{X: b.x1, Y: b.y1}, {X: b.x2, Y: b.y1}}, [2]P{{X: b.x2, Y: b.y1}, {X: b.x2, Y: b.y2}},
			[2]P{{X: b.x2, Y: b.y2}, {X: b.x1, Y: b.y2}}, [2]P{{X: b.x1, Y: b.y2}, {X: b.x1, Y: b.y1}})
	}
	for _, e := range r.edges {
		if e.Label != "" && e.w != 0 && e.h != 0 && e.hasXY {
			o.labels = append(o.labels, labelBox(P{X: e.x, Y: e.y}, e.w, e.h))
		}
	}
	return o
}

func boxesOverlap(a, b lbox) bool { return a.x1 < b.x2 && b.x1 < a.x2 && a.y1 < b.y2 && b.y1 < a.y2 }

func (o *obstacles) hitsFrame(b lbox, clearance float64) bool {
	pb := lbox{b.x1 - clearance, b.y1 - clearance, b.x2 + clearance, b.y2 + clearance}
	for _, f := range o.frames {
		if segmentHitsBox(f[0], f[1], pb) {
			return true
		}
	}
	return false
}

func (o *obstacles) blocked(b lbox, e *estate, key string, segment int, clearance float64) bool {
	for _, n := range o.nodes {
		if boxesOverlap(b, n) {
			return true
		}
	}
	for _, l := range o.labels {
		if boxesOverlap(b, l) {
			return true
		}
	}
	if o.hitsFrame(b, clearance) {
		return true
	}
	for _, other := range o.edges {
		for i := 0; i+1 < len(other.points); i++ {
			if other == e && i == segment {
				continue
			}
			if segmentHitsBox(other.points[i], other.points[i+1], b) {
				return true
			}
		}
	}
	for _, other := range o.edges {
		for k, t := range other.terminals {
			if !t.placed || (other == e && k == key) {
				continue
			}
			if boxesOverlap(b, labelBox(t.center, t.w, t.h)) {
				return true
			}
		}
	}
	return false
}

type placedTerminal struct {
	e        *estate
	key      string
	t        *terminal
	atStart  bool
	segment  int
	from, to P
	length   float64
}

func placedTerminals(edges []*estate) []placedTerminal {
	var out []placedTerminal
	for _, e := range edges {
		if len(e.points) < 2 {
			continue
		}
		for _, k := range []string{"startLeft", "startRight", "endLeft", "endRight"} {
			t := e.terminals[k]
			if t == nil || !t.placed {
				continue
			}
			atStart := strings.HasPrefix(k, "start")
			seg := len(e.points) - 2
			if atStart {
				seg = 0
			}
			from, to := e.points[seg], e.points[seg+1]
			l := math.Hypot(to.X-from.X, to.Y-from.Y)
			if l == 0 {
				continue
			}
			out = append(out, placedTerminal{e, k, t, atStart, seg, from, to, l})
		}
	}
	return out
}

func (r *renderer) slideTerminalLabelsOffFrames() {
	o := r.terminalObstacles()
	for _, pt := range placedTerminals(r.edges) {
		t := pt.t
		if !o.hitsFrame(labelBox(t.center, t.w, t.h), 0) {
			continue
		}
		far, end := pt.from, pt.to
		if pt.atStart {
			far, end = pt.to, pt.from
		}
		dir := P{X: (end.X - far.X) / pt.length, Y: (end.Y - far.Y) / pt.length}
		half := (math.Abs(dir.X)*t.w + math.Abs(dir.Y)*t.h) / 2
		along := func(c P) float64 { return (c.X-end.X)*dir.X + (c.Y-end.Y)*dir.Y }
		onSeg := func(c P) bool { return along(c)+half <= 0 && along(c)-half >= -pt.length }
	search:
		for step := 1.0; step <= pt.length; step++ {
			for _, s := range []float64{step, -step} {
				m := P{X: t.center.X + s*dir.X, Y: t.center.Y + s*dir.Y}
				if onSeg(m) && !o.blocked(labelBox(m, t.w, t.h), pt.e, pt.key, pt.segment, 2) {
					t.center = m
					break search
				}
			}
		}
	}
}

func (r *renderer) putTerminalLabelsOnTheirSide() {
	o := r.terminalObstacles()
	for _, pt := range placedTerminals(r.edges) {
		t := pt.t
		nrm := P{X: -(pt.to.Y - pt.from.Y) / pt.length, Y: (pt.to.X - pt.from.X) / pt.length}
		off := (t.center.X-pt.from.X)*nrm.X + (t.center.Y-pt.from.Y)*nrm.Y
		if off == 0 || (off > 0) == strings.HasSuffix(pt.key, "Right") {
			continue
		}
		m := P{X: t.center.X - 2*off*nrm.X, Y: t.center.Y - 2*off*nrm.Y}
		if !o.blocked(labelBox(m, t.w, t.h), pt.e, pt.key, pt.segment, 0) {
			t.center = m
		}
	}
}

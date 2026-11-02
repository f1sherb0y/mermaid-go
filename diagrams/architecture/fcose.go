package architecture

import (
	"errors"
	"math"
	"slices"
	"strconv"

	"github.com/f1sherb0y/mermaid-go/fcose"
	"github.com/f1sherb0y/mermaid-go/scene"
)

// mermaid's layout: the graph and constraints architectureRenderer.ts
// gives cytoscape-fcose (package fcose), run twice as mermaid runs it
// (layout.run(), then again on 'layoutstop'), seeded as mermaid seeds it.
// The constraints come from mermaid's spatial maps (getDataStructures),
// as mermaid computes them, bends upside down included: getAlignments
// and getRelativeConstraints.

// placeFcose sets every node's center from fcose; it fails where fcose
// throws, and gives up on a drawing that grows past 10 times the area
// of ours (area, if not 0), which would not be taken (choose.go).
func (l *layout) placeFcose(area float64) error {
	cfg := l.ctx.Config
	d := l.d
	g := &fcose.Graph{}
	idx := map[any]int{}
	hasKids := map[*Group]bool{}
	for _, n := range d.Nodes {
		if n.Parent != nil {
			hasKids[n.Parent] = true
		}
	}
	for _, gr := range d.Groups {
		if gr.Parent != nil {
			hasKids[gr.Parent] = true
		}
	}
	parent := func(gr *Group) int {
		if gr == nil {
			return -1
		}
		return idx[gr]
	}
	// cytoscape's order: groups, services, junctions. An empty group is a
	// plain node of cytoscape's default size (30) plus its padding.
	empty := map[int]*Node{} // fcose index → the group's stand-in
	stands := map[*Group]*Node{}
	for n := range l.stand {
		stands[n.Parent] = n
	}
	for _, gr := range d.Groups {
		idx[gr] = len(g.Nodes)
		nd := fcose.Node{ID: gr.ID, Parent: parent(gr.Parent), Padding: float64(int(l.padding))}
		if !hasKids[gr] {
			nd.W, nd.H = 30+2*l.padding, 30+2*l.padding
			empty[len(g.Nodes)] = stands[gr]
		}
		g.Nodes = append(g.Nodes, nd)
	}
	for _, n := range d.Nodes {
		idx[n] = len(g.Nodes)
		g.Nodes = append(g.Nodes, fcose.Node{ID: n.ID, Parent: parent(n.Parent), W: l.iconSize, H: l.iconSize})
	}
	same := l.iconSize * cfg.Num("architecture.idealEdgeLengthMultiplier", 1.5)
	elasticity := cfg.Num("architecture.edgeElasticity", 0.45)
	for _, e := range d.Edges {
		fe := fcose.Edge{Source: idx[e.From], Target: idx[e.To], IdealLength: 0.5 * l.iconSize, Elasticity: 0.001}
		if e.From.Parent == e.To.Parent {
			fe.IdealLength, fe.Elasticity = same, elasticity
		}
		g.Edges = append(g.Edges, fe)
	}
	cons, err := l.fcoseConstraints(idx)
	if err != nil {
		return err
	}
	o := fcose.DefaultOptions()
	o.NumIter = int(cfg.Num("architecture.numIter", 2500))
	seed := uint32(cfg.Num("architecture.seed", 1))
	if seed == 0 {
		seed = 1 // mermaid's unseeded Math.random; we stay deterministic
	}

	// The first run starts with every node at the origin and recenters on
	// the elements' bounding box there (cytoscape's, with labels and
	// edges; ours, close enough: it only moves the drawing).
	if area > 0 {
		o.Stop = func(w, h float64) bool { return w*h > 10*area }
	}
	o.Random = fcose.Mulberry32(seed)
	r1, err := fcose.Layout(g, cons, o)
	if err != nil {
		return err
	}
	cx, cy := l.originCenter()
	pos := fcose.Recenter(r1.Rects, cx, cy)
	for i := range g.Nodes {
		g.Nodes[i].X, g.Nodes[i].Y = pos[i][0], pos[i][1]
	}
	o.Random = fcose.Mulberry32(seed)
	r2, err := fcose.Layout(g, cons, o)
	if err != nil {
		return err
	}
	l.at = map[*Node]scene.Pt{}
	for _, n := range d.Nodes {
		x, y := r2.Rects[idx[n]].Center()
		l.at[n] = scene.Pt{X: x, Y: y}
	}
	for i, n := range empty {
		x, y := r2.Rects[i].Center()
		l.at[n] = scene.Pt{X: x, Y: y}
	}
	return nil
}

// originCenter approximates the center of cytoscape's bounding box of
// the elements when every node is at the origin, where fcose's first run
// puts the drawing back: nodes (an icon and 1px, a service's label 18px
// under it, as wide as it is), groups (their members and 41.5px; 40.5 under
// a label), and edges (their endpoints, at the sides' offsets from the
// center, and a bend's first point 0.5px off the line, 2.5px around). Only
// the rounding of the run depends on it; edge labels are left out.
func (l *layout) originCenter() (float64, float64) {
	type bb struct {
		x1, y1, x2, y2 float64
		label          bool // the bottom is a service's label
	}
	union := func(a, b bb) bb {
		r := bb{min(a.x1, b.x1), min(a.y1, b.y1), max(a.x2, b.x2), max(a.y2, b.y2), a.label}
		if b.y2 > a.y2 {
			r.label = b.label
		} else if b.y2 == a.y2 {
			r.label = a.label || b.label
		}
		return r
	}
	empty := bb{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1), false}
	boxes := map[*Group]bb{}
	var walk func(g *Group) bb
	walk = func(g *Group) bb {
		b := empty
		for _, it := range l.kids[g] {
			switch {
			case it.group != nil:
				b = union(b, walk(it.group))
			case l.stand[it.node]:
				// An empty group: a plain node, 30px and its padding.
				h := 15 + l.padding + 1
				return bb{-h, -h, h, h, false}
			default:
				n := it.node
				w := l.iconSize/2 + 1
				nb := bb{-w, -w, w, w, false}
				if tb, ok := l.labels[n]; ok {
					nb.x1, nb.x2 = min(nb.x1, -tb.w/2-1), max(nb.x2, tb.w/2+1)
					nb.y2, nb.label = w+18, true
				}
				b = union(b, nb)
			}
		}
		if g != nil && len(l.kids[g]) > 0 {
			bottom := 41.5
			if b.label {
				bottom = 40.5
			}
			b = bb{b.x1 - 41.5, b.y1 - 41.5, b.x2 + 41.5, b.y2 + bottom, false}
			boxes[g] = b
		}
		return b
	}
	all := walk(nil)
	s := l.iconSize
	ep := func(sd Side) scene.Pt {
		switch sd {
		case 'L':
			return scene.Pt{X: 0, Y: s / 2}
		case 'R':
			return scene.Pt{X: s, Y: s / 2}
		case 'T':
			return scene.Pt{X: s / 2, Y: 0}
		}
		return scene.Pt{X: s / 2, Y: s}
	}
	for _, e := range l.d.Edges {
		a, b := ep(e.FromSide), ep(e.ToSide)
		pts := []scene.Pt{a, b}
		if isX(e.FromSide) != isX(e.ToSide) {
			dx, dy := b.X-a.X, b.Y-a.Y
			d := math.Hypot(dx, dy)
			pts = append(pts, scene.Pt{X: a.X - 0.5*dy/d, Y: a.Y + 0.5*dx/d})
		}
		for _, p := range pts {
			all = union(all, bb{p.X - 2.5, p.Y - 2.5, p.X + 2.5, p.Y + 2.5, false})
		}
	}
	return all.x1 + (all.x2-all.x1)/2, all.y1 + (all.y2-all.y1)/2
}

// spatialMaps is getDataStructures' spatial maps: from each part's first
// node a walk over the edges, each putting its far end a cell off by its
// sides (y up), in mermaid's order.
func (l *layout) spatialMaps() []*posMap {
	type link struct {
		pair string
		to   *Node
	}
	adj := map[*Node][]link{} // a Map from pair to node, in insertion order
	edgesOf := map[*Node][]*Edge{}
	for _, e := range l.d.Edges {
		edgesOf[e.From] = append(edgesOf[e.From], e)
		edgesOf[e.To] = append(edgesOf[e.To], e)
	}
	for _, n := range l.d.Nodes {
		var m []link
		put := func(pair string, to *Node) {
			if pair == "LL" || pair == "RR" || pair == "TT" || pair == "BB" {
				return
			}
			for i := range m {
				if m[i].pair == pair {
					m[i].to = to
					return
				}
			}
			m = append(m, link{pair, to})
		}
		for _, e := range edgesOf[n] {
			if e.From == n {
				put(string([]byte{byte(e.FromSide), byte(e.ToSide)}), e.To)
			} else {
				put(string([]byte{byte(e.ToSide), byte(e.FromSide)}), e.From)
			}
		}
		adj[n] = m
	}
	visited := map[*Node]bool{}
	var maps []*posMap
	for _, start := range l.d.Nodes {
		if visited[start] {
			continue
		}
		sm := newPosMap()
		sm.set(start, [2]int{0, 0})
		queue := []*Node{start}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			visited[id] = true
			p := sm.at[id]
			for _, lk := range adj[id] {
				if !visited[lk.to] {
					sm.set(lk.to, shiftPair(p, lk.pair))
					queue = append(queue, lk.to)
				}
			}
		}
		maps = append(maps, sm)
	}
	return maps
}

// shiftPair is shiftPositionByArchitectureDirectionPair.
func shiftPair(p [2]int, pair string) [2]int {
	lhs, rhs := Side(pair[0]), Side(pair[1])
	x, y := p[0], p[1]
	dx := func(s Side, neg Side) int {
		if s == neg {
			return -1
		}
		return 1
	}
	if isX(lhs) {
		if !isX(rhs) {
			return [2]int{x + dx(lhs, 'L'), y + -dx(rhs, 'T')}
		}
		return [2]int{x + dx(lhs, 'L'), y}
	}
	if isX(rhs) {
		return [2]int{x - dx(rhs, 'L'), y - dx(lhs, 'T')}
	}
	return [2]int{x, y - dx(lhs, 'T')}
}

// posMap is a spatial map: node → cell, in insertion order.
type posMap struct {
	nodes []*Node
	at    map[*Node][2]int
}

func newPosMap() *posMap { return &posMap{at: map[*Node][2]int{}} }

func (m *posMap) set(n *Node, p [2]int) {
	if _, ok := m.at[n]; !ok {
		m.nodes = append(m.nodes, n)
	}
	m.at[n] = p
}

// maxRelative bounds the relative placements. mermaid's walk over a
// spatial map marks a cell visited when it leaves the queue, not when it
// enters, so a cell is queued once per way to reach it and its placements
// repeat as often: a grid of 10 by 10 services makes 184754 of them, 10 by
// 15 three million (mermaid all but hangs). Past this many, mermaid's
// layout is not made and ours is used.
const maxRelative = 5000

var errTooManyConstraints = errors.New("architecture: too many fcose constraints")

// fcoseConstraints is getAlignments and getRelativeConstraints.
func (l *layout) fcoseConstraints(idx map[any]int) (*fcose.Constraints, error) {
	maps := l.spatialMaps()
	// groupAlignments: how groups an edge joins line up (the last edge
	// says), by the sorted pair of group ids.
	groupAlign := map[[2]string]bool{} // true: horizontal
	for _, n := range l.d.Nodes {
		for _, e := range l.d.Edges {
			if e.From != n && e.To != n {
				continue
			}
			a, b := e.From.Parent, e.To.Parent
			if a != nil && b != nil && a != b && isX(e.FromSide) == isX(e.ToSide) {
				groupAlign[groupKey(a.ID, b.ID)] = isX(e.FromSide)
			}
		}
	}
	groupOf := func(n *Node) string {
		if n.Parent == nil {
			return "default"
		}
		return n.Parent.ID
	}
	// flatten merges, per row or column, the nodes of groups that line up
	// that way (or are at the top level); others get a list each.
	flatten := func(lines *strMap[*strMap[[]*Node]], horizontal bool) [][]*Node {
		out := newStrMap[[]*Node]()
		for i, dir := range lines.keys {
			groups := lines.vals[i]
			if len(groups.keys) == 1 {
				out.set(dir, groups.vals[0])
				continue
			}
			cnt := 0
			for a := 0; a < len(groups.keys)-1; a++ {
				for b := a + 1; b < len(groups.keys); b++ {
					ag, an := groups.keys[a], groups.vals[a]
					bg, bn := groups.keys[b], groups.vals[b]
					h, ok := groupAlign[groupKey(ag, bg)]
					if ok && h == horizontal || ag == "default" || bg == "default" {
						cur, _ := out.get(dir)
						out.set(dir, append(append(slices.Clone(cur), an...), bn...))
					} else {
						out.set(dir+"-"+strconv.Itoa(cnt), an)
						cnt++
						out.set(dir+"-"+strconv.Itoa(cnt), bn)
						cnt++
					}
				}
			}
		}
		var res [][]*Node
		for _, v := range out.vals {
			if len(v) > 1 {
				res = append(res, v)
			}
		}
		return res
	}
	var horizontal, vertical [][]*Node
	for _, sm := range maps {
		rows, cols := newStrMap[*strMap[[]*Node]](), newStrMap[*strMap[[]*Node]]()
		for _, n := range sm.nodes {
			p := sm.at[n]
			for k, lines := range []*strMap[*strMap[[]*Node]]{rows, cols} {
				key := strconv.Itoa(p[1-k])
				line, ok := lines.get(key)
				if !ok {
					line = newStrMap[[]*Node]()
					lines.set(key, line)
				}
				list, _ := line.get(groupOf(n))
				line.set(groupOf(n), append(list, n))
			}
		}
		horizontal = append(horizontal, flatten(rows, true)...)
		vertical = append(vertical, flatten(cols, false)...)
	}
	declared := map[*Node]bool{}
	for _, h := range l.d.Hints {
		for _, m := range h.Members {
			declared[m] = true
		}
	}
	keep := func(lists [][]*Node) [][]*Node {
		var out [][]*Node
		for _, list := range lists {
			if !slices.ContainsFunc(list, func(n *Node) bool { return declared[n] }) {
				out = append(out, list)
			}
		}
		return out
	}
	horizontal, vertical = keep(horizontal), keep(vertical)
	for _, h := range l.d.Hints {
		if len(h.Members) < 2 {
			continue
		}
		if h.Column {
			vertical = append(vertical, h.Members)
		} else {
			horizontal = append(horizontal, h.Members)
		}
	}
	c := &fcose.Constraints{}
	ids := func(list []*Node) []int {
		out := make([]int, len(list))
		for i, n := range list {
			out[i] = idx[n]
		}
		return out
	}
	for _, list := range horizontal {
		c.Horizontal = append(c.Horizontal, ids(list))
	}
	for _, list := range vertical {
		c.Vertical = append(c.Vertical, ids(list))
	}

	// Relative placements: the hints' chains, then a walk over each
	// spatial map's cells from (0, 0), each neighbor a gap off.
	gap := l.iconSize * l.ctx.Config.Num("architecture.idealEdgeLengthMultiplier", 1.5)
	pairs := map[[2]*Node]bool{}
	for _, h := range l.d.Hints {
		for i := 0; i+1 < len(h.Members); i++ {
			a, b := h.Members[i], h.Members[i+1]
			pairs[[2]*Node{a, b}], pairs[[2]*Node{b, a}] = true, true
			c.Relative = append(c.Relative, fcose.Relative{A: idx[a], B: idx[b], Gap: gap, Vertical: h.Column})
		}
	}
	dirs := [4]struct {
		side Side
		d    [2]int
	}{{'L', [2]int{-1, 0}}, {'R', [2]int{1, 0}}, {'T', [2]int{0, 1}}, {'B', [2]int{0, -1}}}
	for _, sm := range maps {
		inv := map[[2]int]*Node{}
		for _, n := range sm.nodes {
			inv[sm.at[n]] = n
		}
		queue := [][2]int{{0, 0}}
		visited := map[[2]int]bool{}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			visited[cur] = true
			curN := inv[cur]
			if curN == nil {
				continue
			}
			if len(c.Relative) > maxRelative {
				return nil, errTooManyConstraints
			}
			for _, dd := range dirs {
				np := [2]int{cur[0] + dd.d[0], cur[1] + dd.d[1]}
				nn := inv[np]
				if nn == nil || visited[np] {
					continue
				}
				queue = append(queue, np)
				if pairs[[2]*Node{curN, nn}] {
					continue
				}
				// {[side]: new, [opposite]: current}
				switch dd.side {
				case 'L':
					c.Relative = append(c.Relative, fcose.Relative{A: idx[nn], B: idx[curN], Gap: gap})
				case 'R':
					c.Relative = append(c.Relative, fcose.Relative{A: idx[curN], B: idx[nn], Gap: gap})
				case 'T':
					c.Relative = append(c.Relative, fcose.Relative{A: idx[nn], B: idx[curN], Gap: gap, Vertical: true})
				default:
					c.Relative = append(c.Relative, fcose.Relative{A: idx[curN], B: idx[nn], Gap: gap, Vertical: true})
				}
			}
		}
	}
	return c, nil
}

// groupKey is architectureGroupAlignmentKey: the two ids, sorted.
func groupKey(a, b string) [2]string {
	if b < a {
		a, b = b, a
	}
	return [2]string{a, b}
}

// strMap is an insertion-ordered map with string keys, as a JS Map.
type strMap[V any] struct {
	keys []string
	at   map[string]int
	vals []V
}

func newStrMap[V any]() *strMap[V] { return &strMap[V]{at: map[string]int{}} }

func (m *strMap[V]) get(k string) (V, bool) {
	if i, ok := m.at[k]; ok {
		return m.vals[i], true
	}
	var z V
	return z, false
}

func (m *strMap[V]) set(k string, v V) {
	if i, ok := m.at[k]; ok {
		m.vals[i] = v
		return
	}
	m.at[k] = len(m.keys)
	m.keys = append(m.keys, k)
	m.vals = append(m.vals, v)
}

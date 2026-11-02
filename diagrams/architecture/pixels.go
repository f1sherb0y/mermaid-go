package architecture

import (
	"cmp"
	"math"
	"slices"
	"sort"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// From cells to pixels, one axis at a time (x, then y once the columns
// are known), as fcose ends up with mermaid's constraints:
//
//   - Nodes line up only where mermaid aligns them: a column is the nodes
//     that edges join into one part of the diagram, that share the x of
//     mermaid's spatial map (and of the cell), and that are in one group,
//     in groups a vertical edge joins, or at the top level (getAlignments);
//     and an `align column`. Elsewhere a cell's column says only what is
//     left of what.
//   - The ends of an edge along the axis are at least a pitch apart (the
//     ideal edge length between icons), a bend's at least the compact
//     distance on both axes; two siblings side by side keep apart: nodes at
//     the compact distance (unconnected neighbors; `align` neighbors a
//     little further), a group and its neighbor nodeSeparation and a
//     quarter icon between bodies (labels as cytoscape sizes them, the
//     padding of every group around each); an edge's label fits between
//     its ends.
//   - The least coordinates that keep these; then each line moves, within
//     what they allow, to the median of where its edges want it: a pitch
//     from the other end along the edge, in line with it across (so a fan's
//     hub centers on its fan), and `align` neighbors at their distance.

// place lays the diagram out: mermaid's way and ours, and keeps the better
// (choose.go); then the edges' courses.
func (l *layout) place() {
	f := l.fork()
	l.placeGrid()
	l.choice = "grid"
	fok := f.placeFcose(l.extent()) == nil
	if fok {
		f.fitBoxes()
	}
	if fok {
		qf, qg := f.quality(), l.quality()
		l.qFcose, l.qGrid, f.qFcose, f.qGrid = qf, qg, qf, qg
		if !gridBetter(qf, qg) {
			*l = *f
			l.choice = "fcose"
		}
	}
	l.route()
}

// fork is a copy of the layout before placing, to place another way.
func (l *layout) fork() *layout {
	f := *l
	f.pad = map[*Group]*extent{}
	for g, p := range l.pad {
		q := *p
		f.pad[g] = &q
	}
	return &f
}

// fitBoxes makes the groups' boxes around the placed nodes and widens them
// for their titles.
func (l *layout) fitBoxes() {
	for range 8 {
		l.boxes()
		if !l.widen() {
			return
		}
	}
}

// placeGrid lays the diagram out on the grid: cells, then pixels.
func (l *layout) placeGrid() {
	l.comp = l.components()
	l.spatial = l.spatialMap()
	l.cell = l.grid(nil).pos
	for range 8 {
		l.at = map[*Node]scene.Pt{}
		xs := l.solve(0)
		for i, n := range l.nodes {
			l.at[n] = scene.Pt{X: xs[i]}
		}
		l.boxes()
		ys := l.solve(1)
		for i, n := range l.nodes {
			l.at[n] = scene.Pt{X: xs[i], Y: ys[i]}
		}
		l.boxes()
		if !l.widen() {
			break
		}
	}
}

// components numbers the parts of the diagram that edges join.
func (l *layout) components() []int {
	uf := newUnion(len(l.nodes))
	for _, e := range l.d.Edges {
		uf.join(l.idx[e.From], l.idx[e.To])
	}
	return uf.labels()
}

// spatialMap is where mermaid's spatial map puts each node: a walk over
// the edges from each part's first node, each edge putting its far end a
// cell off as its sides say (getDataStructures; the grid's directions,
// with bends the right way up).
func (l *layout) spatialMap() map[*Node]gp {
	adj := map[*Node][]*Edge{}
	for _, e := range l.d.Edges {
		adj[e.From] = append(adj[e.From], e)
		adj[e.To] = append(adj[e.To], e)
	}
	sp := map[*Node]gp{}
	for _, s := range l.nodes {
		if _, ok := sp[s]; ok {
			continue
		}
		sp[s] = gp{}
		queue := []*Node{s}
		for len(queue) > 0 {
			u := queue[0]
			queue = queue[1:]
			for _, e := range adj[u] {
				v, d := e.To, dir(e.FromSide, e.ToSide)
				if v == u {
					v, d = e.From, d.neg()
				}
				if _, ok := sp[v]; !ok {
					sp[v] = sp[u].add(d)
					queue = append(queue, v)
				}
			}
		}
	}
	return sp
}

// classes groups the nodes that share a column (axis 0) or row (1); it
// returns each node's line, by index, and the number of lines.
func (l *layout) classes(axis int) ([]int, int) {
	uf := newUnion(len(l.nodes))
	// Groups that a straight edge across the axis joins line up.
	type pair [2]*Group
	aligned := map[pair]bool{}
	for _, e := range l.d.Edges {
		a, b := e.From.Parent, e.To.Parent
		if a != nil && b != nil && a != b && isX(e.FromSide) == isX(e.ToSide) && isX(e.FromSide) == (axis == 1) {
			aligned[pair{a, b}], aligned[pair{b, a}] = true, true
		}
	}
	type bucket struct{ comp, at, sp int }
	buckets := map[bucket][]int{}
	var keys []bucket
	for i, n := range l.nodes {
		if l.stand[n] {
			continue
		}
		sp := l.spatial[n]
		k := bucket{l.comp[i], key(l.coord(n, axis)), key([2]float64{sp.x, sp.y}[axis])}
		if _, ok := buckets[k]; !ok {
			keys = append(keys, k)
		}
		buckets[k] = append(buckets[k], i)
	}
	for _, k := range keys {
		first := map[*Group]int{}
		var groups []*Group
		for _, i := range buckets[k] {
			g := l.nodes[i].Parent
			if f, ok := first[g]; ok {
				uf.join(f, i)
			} else {
				first[g] = i
				groups = append(groups, g)
			}
		}
		for i, a := range groups {
			for _, b := range groups[i+1:] {
				if a == nil || b == nil || aligned[pair{a, b}] {
					uf.join(first[a], first[b])
				}
			}
		}
	}
	for _, h := range l.d.Hints {
		if h.Column != (axis == 0) {
			continue
		}
		for k := 1; k < len(h.Members); k++ {
			a, b := h.Members[k-1], h.Members[k]
			if key(l.coord(a, axis)) == key(l.coord(b, axis)) {
				uf.join(l.idx[a], l.idx[b])
			}
		}
	}
	cls := uf.labels()
	return cls, slices.Max(append(cls, -1)) + 1
}

// coord is n's cell's x (axis 0) or y.
func (l *layout) coord(n *Node, axis int) float64 {
	return [2]float64{l.cell[n].x, l.cell[n].y}[axis]
}

// arc says that a line is at least w from another.
type arc struct {
	to int
	w  float64
}

// wish says where a line wants another: at its coordinate plus off.
type wish struct {
	from      int
	off, mass float64
}

// solve places the columns (axis 0) or rows (1); it returns each node's
// coordinate, by index.
func (l *layout) solve(axis int) []float64 {
	cls, nc := l.classes(axis)
	at := make([]float64, nc)
	for i, n := range l.nodes {
		at[cls[i]] = l.coord(n, axis)
	}
	line := func(n *Node) int { return cls[l.idx[n]] }
	least := map[[2]int]float64{}
	lines := func(a, b int, w float64) {
		if at[a] < at[b] {
			k := [2]int{a, b}
			least[k] = math.Max(least[k], w)
		}
	}
	add := func(m, n *Node, w float64) { lines(line(m), line(n), w) }
	// Siblings side by side.
	groups := append([]*Group{nil}, l.d.Groups...)
	for _, g := range groups {
		ks := l.kids[g]
		spans := make([]span, len(ks))
		for i, it := range ks {
			spans[i] = l.span(it)
		}
		// A group's reach past each of its lines, toward the next sibling
		// (far) and the one before (near), so that two groups are kept
		// apart line by line, not node by node.
		far, near := make([]map[int]reachOf, len(ks)), make([]map[int]reachOf, len(ks))
		for i, it := range ks {
			if it.group != nil {
				far[i], near[i] = l.reach(it, spans[i].nodes, axis, line)
			}
		}
		for i := range ks {
			for j := range ks {
				a, b := spans[i], spans[j]
				if i == j || [2]float64{b.lo.x, b.lo.y}[axis] <= [2]float64{a.hi.x, a.hi.y}[axis] || !l.beside(ks[i], ks[j], a, b, axis) {
					continue
				}
				if ks[i].node != nil && ks[j].node != nil {
					add(ks[i].node, ks[j].node, l.leafSep(ks[i].node, ks[j].node, axis))
					continue
				}
				fi, nj := far[i], near[j]
				if fi == nil {
					fi, _ = l.reach(ks[i], a.nodes, axis, line)
				}
				if nj == nil {
					_, nj = l.reach(ks[j], b.nodes, axis, line)
				}
				// Beside a group fcose leaves more than nodeSeparation:
				// about 95px between bodies on mermaid.live (edges between
				// groups 255 to 264px long, one into a group 216).
				gap := l.gap + l.iconSize/4
				for c, rc := range fi {
					for d, rd := range nj {
						lines(c, d, math.Max(rc.body+gap+rd.body, rc.label+labelGap+rd.label))
					}
				}
			}
		}
	}
	// A straight edge across the axis keeps its line clear of the nodes
	// between its ends: along x by their pixels (known when the rows are
	// placed), along y by their cells.
	for _, e := range l.d.Edges {
		if isX(e.FromSide) != isX(e.ToSide) || isX(e.FromSide) != (axis == 1) || line(e.From) != line(e.To) {
			continue
		}
		c := line(e.From)
		lo, hi := l.coord(e.From, 1-axis), l.coord(e.To, 1-axis)
		if axis == 1 {
			lo, hi = l.at[e.From].X, l.at[e.To].X
		}
		lo, hi = math.Min(lo, hi), math.Max(lo, hi)
		for _, n := range l.nodes {
			if n == e.From || n == e.To || l.stand[n] || line(n) == c {
				continue
			}
			between := false
			if axis == 1 {
				x, ex := l.at[n].X, l.ext[n]
				between = x+ex.l < hi && x+ex.r > lo
			} else {
				between = lo < l.coord(n, 1) && l.coord(n, 1) < hi
			}
			if !between {
				continue
			}
			const margin = 20
			e := l.ext[n]
			if at[line(n)] < at[c] {
				lines(line(n), c, [2]float64{e.r, e.b}[axis]+margin)
			} else {
				lines(c, line(n), -[2]float64{e.l, e.t}[axis]+margin)
			}
		}
	}
	// Edges, with room for their labels.
	for _, e := range l.d.Edges {
		along := [2]bool{isX(e.FromSide) == (axis == 0), isX(e.ToSide) == (axis == 0)}
		if !along[0] && !along[1] || e.FromSide == e.ToSide {
			continue
		}
		a, b := e.From, e.To
		if l.coord(a, axis) > l.coord(b, axis) {
			a, b = b, a
		}
		w := l.pitch
		if along[0] != along[1] {
			w = l.compact
		}
		if tb, ok := l.edgeLabels[e]; ok && along[0] && along[1] && l.coord(a, 1-axis) == l.coord(b, 1-axis) {
			const margin = 10
			if axis == 0 {
				w = math.Max(w, l.iconSize+tb.w+2*margin)
			} else {
				w = math.Max(w, l.textB[a]+tb.w+2*margin+l.iconSize/2)
			}
		}
		add(a, b, w)
	}
	for _, h := range l.d.Hints {
		if h.Column == (axis == 1) {
			for k := 1; k < len(h.Members); k++ {
				a, b := h.Members[k-1], h.Members[k]
				if l.coord(a, axis) > l.coord(b, axis) {
					a, b = b, a
				}
				add(a, b, l.compact)
			}
		}
	}
	keys := make([][2]int, 0, len(least))
	for k := range least {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b [2]int) int { return cmp.Or(a[0]-b[0], a[1]-b[1]) })
	before, after := make([][]arc, nc), make([][]arc, nc)
	for _, k := range keys {
		after[k[0]] = append(after[k[0]], arc{k[1], least[k]})
		before[k[1]] = append(before[k[1]], arc{k[0], least[k]})
	}

	// Where the edges and hints want each line.
	wishes := make([][]wish, nc)
	want := func(m, n *Node, off, mass float64) { // n at m + off
		a, b := line(m), line(n)
		if a == b {
			return
		}
		wishes[b] = append(wishes[b], wish{a, off, mass})
		wishes[a] = append(wishes[a], wish{b, -off, mass})
	}
	for _, e := range l.d.Edges {
		along := [2]bool{isX(e.FromSide) == (axis == 0), isX(e.ToSide) == (axis == 0)}
		d := sign(l.coord(e.To, axis) - l.coord(e.From, axis))
		switch {
		case !along[0] && !along[1]:
			want(e.From, e.To, 0, 1)
		case e.FromSide == e.ToSide:
		case along[0] && along[1]:
			want(e.From, e.To, d*l.pitch, 1)
		default:
			want(e.From, e.To, d*l.compact, 0.5)
		}
	}
	for _, h := range l.d.Hints {
		if h.Column == (axis == 1) {
			for k := 1; k < len(h.Members); k++ {
				a, b := h.Members[k-1], h.Members[k]
				if d := sign(l.coord(b, axis) - l.coord(a, axis)); d > 0 {
					want(a, b, l.spread(a, axis), 1)
				} else if d < 0 {
					want(b, a, l.spread(b, axis), 1)
				}
			}
		}
	}

	// The least coordinates, then the medians.
	order := make([]int, nc)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return at[order[i]] < at[order[j]] })
	pos := make([]float64, nc)
	for _, c := range order {
		for _, a := range before[c] {
			pos[c] = math.Max(pos[c], pos[a.to]+a.w)
		}
	}
	var targets []wish
	for round := range 60 {
		moved := 0.0
		for k := range order {
			c := order[k]
			if round%2 == 1 {
				c = order[len(order)-1-k]
			}
			if len(wishes[c]) == 0 {
				continue
			}
			lo, hi := math.Inf(-1), math.Inf(1)
			for _, a := range before[c] {
				lo = math.Max(lo, pos[a.to]+a.w)
			}
			for _, a := range after[c] {
				hi = math.Min(hi, pos[a.to]-a.w)
			}
			targets = targets[:0]
			for _, w := range wishes[c] {
				targets = append(targets, wish{off: pos[w.from] + w.off, mass: w.mass})
			}
			p := math.Max(lo, math.Min(hi, wmedian(targets)))
			moved = math.Max(moved, math.Abs(p-pos[c]))
			pos[c] = p
		}
		if moved < 0.01 {
			break
		}
	}
	// Parts that no edge places center on what they were packed beside,
	// as far as the others let them.
	for _, p := range l.packs {
		if p.axis != axis {
			continue
		}
		in := map[int]bool{}
		for _, n := range p.part {
			in[line(n)] = true
		}
		span := func(ns []*Node) (float64, float64) {
			lo, hi := math.Inf(1), math.Inf(-1)
			for _, n := range ns {
				e, c := l.ext[n], pos[line(n)]
				lo, hi = math.Min(lo, c+[2]float64{e.l, e.t}[axis]), math.Max(hi, c+[2]float64{e.r, e.b}[axis])
			}
			return lo, hi
		}
		a0, a1 := span(p.part)
		b0, b1 := span(p.ref)
		delta := (b0+b1)/2 - (a0+a1)/2
		for c := range in {
			for _, a := range before[c] {
				if !in[a.to] {
					delta = math.Max(delta, pos[a.to]+a.w-pos[c])
				}
			}
			for _, a := range after[c] {
				if !in[a.to] {
					delta = math.Min(delta, pos[a.to]-a.w-pos[c])
				}
			}
		}
		for c := range in {
			pos[c] += delta
		}
	}
	out := make([]float64, len(l.nodes))
	for i := range l.nodes {
		out[i] = pos[cls[i]]
	}
	return out
}

// centered is a part of the diagram, placed by nothing but pack, to center
// on ref along axis.
type centered struct {
	part, ref []*Node
	axis      int
}

// spread is how far an `align` neighbor is after a: about an icon between
// the icons, as fcose leaves them, and a's label under it.
func (l *layout) spread(a *Node, axis int) float64 {
	s := 2 * l.iconSize
	if axis == 1 {
		s += l.ext[a].b - l.iconSize/2
	}
	return s
}

// wmedian is the weighted median of the targets (in off); between two
// middle ones, halfway.
func wmedian(ts []wish) float64 {
	slices.SortFunc(ts, func(a, b wish) int { return cmp.Compare(a.off, b.off) })
	total := 0.0
	for _, t := range ts {
		total += t.mass
	}
	acc := 0.0
	for i, t := range ts {
		acc += t.mass
		switch {
		case math.Abs(acc-total/2) < 1e-9 && i+1 < len(ts):
			return (t.off + ts[i+1].off) / 2
		case acc > total/2:
			return t.off
		}
	}
	return ts[len(ts)-1].off
}

// leafSep is how far apart the centers of two sibling nodes, m first, must
// be along axis: labels side by side nearly touch; a label over an icon
// keeps clear of it.
func (l *layout) leafSep(m, n *Node, axis int) float64 {
	em, en := l.ext[m], l.ext[n]
	far, near := [2]float64{em.r, em.b}[axis], -[2]float64{en.l, en.t}[axis]
	gap := [2]float64{10, 30}[axis]
	return math.Max(l.compact, far+gap+near)
}

// labelGap is the least room between labels (with the groups' padding)
// where the bodies keep nodeSeparation.
const labelGap = 40

// reachOf is how far past a line a sibling's bodies reach (fcose keeps
// nodeSeparation between them, labels left out) and its labels (which
// keep labelGap).
type reachOf struct{ body, label float64 }

// reach is how far a sibling reaches past each of its lines along axis,
// toward the next sibling (far) and the one before (near), counting the
// padding of the groups around its nodes up to it.
func (l *layout) reach(it item, nodes []*Node, axis int, line func(*Node) int) (far, near map[int]reachOf) {
	far, near = map[int]reachOf{}, map[int]reachOf{}
	half := l.iconSize / 2
	for _, m := range nodes {
		e := l.ext[m]
		f, n := [2]float64{e.r, e.b}[axis], -[2]float64{e.l, e.t}[axis]
		pf, pn := l.pads(m, it, 2+axis), l.pads(m, it, axis)
		c := line(m)
		r := far[c]
		far[c] = reachOf{math.Max(r.body, pf+math.Min(f, half)), math.Max(r.label, pf+f)}
		r = near[c]
		near[c] = reachOf{math.Max(r.body, pn+math.Min(n, half)), math.Max(r.label, pn+n)}
	}
	return far, near
}

// union is a union-find over 0..n-1.
type union []int

func newUnion(n int) union {
	u := make(union, n)
	for i := range u {
		u[i] = i
	}
	return u
}

func (u union) find(i int) int {
	for u[i] != i {
		u[i] = u[u[i]]
		i = u[i]
	}
	return i
}

func (u union) join(a, b int) {
	if ra, rb := u.find(a), u.find(b); ra != rb {
		u[max(ra, rb)] = min(ra, rb)
	}
}

// labels numbers the sets 0, 1, ... in order of their first member.
func (u union) labels() []int {
	out := make([]int, len(u))
	num := map[int]int{}
	for i := range u {
		r := u.find(i)
		if _, ok := num[r]; !ok {
			num[r] = len(num)
		}
		out[i] = num[r]
	}
	return out
}

package fcose

import "math"

// The constraints: ConstraintHandler.handleConstraints enforces them on the
// start positions (fcose with randomize false does not transform the
// drawing first), and CoSELayout keeps them during the iterations
// (initConstraintVariables, updateDisplacements, with
// RELAX_MOVEMENT_ON_CONSTRAINTS). Constraint members are leaf nodes.
//
// In the relative placement graphs a node that is in an alignment stands
// for the whole alignment ("dummy" + i in fcose): keys here are node
// indices, and -1-i for the i-th alignment across the axis.

// omap is an insertion-ordered map, as a JS Map.
type omap[V any] struct {
	keys []int
	at   map[int]int
	vals []V
}

func newOmap[V any]() *omap[V] { return &omap[V]{at: map[int]int{}} }

func (m *omap[V]) get(k int) (V, bool) {
	if i, ok := m.at[k]; ok {
		return m.vals[i], true
	}
	var z V
	return z, false
}

func (m *omap[V]) set(k int, v V) {
	if i, ok := m.at[k]; ok {
		m.vals[i] = v
		return
	}
	m.at[k] = len(m.keys)
	m.keys = append(m.keys, k)
	m.vals = append(m.vals, v)
}

func (m *omap[V]) has(k int) bool { _, ok := m.at[k]; return ok }

// oset is an insertion-ordered set, as a JS Set.
type oset struct {
	items []int
	in    map[int]bool
}

func newOset(items ...int) *oset {
	s := &oset{in: map[int]bool{}}
	for _, k := range items {
		s.add(k)
	}
	return s
}

func (s *oset) add(k int) {
	if !s.in[k] {
		s.in[k] = true
		s.items = append(s.items, k)
	}
}

type adj struct {
	id         int
	gap        float64
	horizontal bool
}

// alignment maps a node to its alignment's key and back.
type alignment struct {
	toDummy map[int]int   // node → -1-i (the last alignment it is in)
	members map[int][]int // -1-i → nodes
}

func newAlignment(lists [][]int) alignment {
	a := alignment{toDummy: map[int]int{}, members: map[int][]int{}}
	for i, list := range lists {
		d := -1 - i
		a.members[d] = []int{}
		for _, n := range list {
			a.toDummy[n] = d
			a.members[d] = append(a.members[d], n)
		}
	}
	return a
}

func (a alignment) key(n int) int {
	if d, ok := a.toDummy[n]; ok {
		return d
	}
	return n
}

func (l *layout) handleConstraints() {
	c := l.c
	if c == nil {
		c = &Constraints{}
	}
	nodeIndexes := map[int]int{}
	var xs, ys []float64
	for _, n := range l.nodes {
		if n.child == nil {
			nodeIndexes[n.index] = len(xs)
			xs = append(xs, n.centerX())
			ys = append(ys, n.centerY())
		}
	}
	coord := func(cs []float64, n int) float64 {
		if i, ok := nodeIndexes[n]; ok {
			return cs[i]
		}
		return math.NaN() // xCoords[undefined]
	}
	setCoord := func(cs []float64, n int, v float64) {
		if i, ok := nodeIndexes[n]; ok {
			cs[i] = v
		}
	}

	// The DAG of relative placements, in constraint order.
	dag := newOmap[[]adj]()
	for _, r := range c.Relative {
		list, _ := dag.get(r.A)
		dag.set(r.A, append(list, adj{r.B, r.Gap, !r.Vertical}))
		if !dag.has(r.B) {
			dag.set(r.B, nil)
		}
	}

	// Alignments: every member to the members' mean.
	align := func(lists [][]int, cs []float64) {
		for _, list := range lists {
			set := newOset(list...)
			sum := 0.0
			for _, n := range set.items {
				sum += coord(cs, n)
			}
			pos := sum / float64(len(set.items))
			for _, n := range set.items {
				setCoord(cs, n, pos)
			}
		}
	}
	align(c.Vertical, xs)
	align(c.Horizontal, ys)

	if len(c.Relative) > 0 {
		vAlign, hAlign := newAlignment(c.Vertical), newAlignment(c.Horizontal)
		dummyPosV, dummyPosH := map[int]float64{}, map[int]float64{}
		for i, list := range c.Vertical {
			dummyPosV[-1-i] = coord(xs, list[0])
		}
		for i, list := range c.Horizontal {
			dummyPosH[-1-i] = coord(ys, list[0])
		}
		dagH, dagV := newOmap[[]adj](), newOmap[[]adj]()
		for i, id := range dag.keys {
			for _, a := range dag.vals[i] {
				d, al := dagV, hAlign
				if a.horizontal {
					d, al = dagH, vAlign
				}
				src := al.key(id)
				t := adj{al.key(a.id), a.gap, a.horizontal}
				list, _ := d.get(src)
				d.set(src, append(list, t))
				if !d.has(t.id) {
					d.set(t.id, nil)
				}
			}
		}
		posH := placeRelative(dagH, func(k int) float64 {
			if k >= 0 {
				return coord(xs, k)
			}
			return dummyPosV[k]
		})
		posV := placeRelative(dagV, func(k int) float64 {
			if k >= 0 {
				return coord(ys, k)
			}
			return dummyPosH[k]
		})
		for i, k := range posH.keys {
			if k < 0 {
				for _, n := range vAlign.members[k] {
					setCoord(xs, n, posH.vals[i])
				}
			} else {
				setCoord(xs, k, posH.vals[i])
			}
		}
		for i, k := range posV.keys {
			if k < 0 {
				for _, n := range hAlign.members[k] {
					setCoord(ys, n, posV.vals[i])
				}
			} else {
				setCoord(ys, k, posV.vals[i])
			}
		}
	}

	for _, n := range l.nodes {
		if n.child == nil {
			i := nodeIndexes[n.index]
			n.setCenter(xs[i], ys[i])
		}
	}
}

// placeRelative is findAppropriatePositionForRelativePlacement in the
// enforcement phase (with an empty set of fixed nodes, which JS takes as
// given): each component's sources start at their mean, the others a gap
// after their predecessors (longest path), then each component is moved
// back to be centered where it was.
func placeRelative(graph *omap[[]adj], before func(int) float64) *omap[float64] {
	// Components of the undirected graph and their sources.
	und := newOmap[[]int]()
	for _, k := range graph.keys {
		und.set(k, nil)
	}
	indeg := newOmap[int]()
	for _, k := range graph.keys {
		indeg.set(k, 0)
	}
	for i, k := range graph.keys {
		for _, a := range graph.vals[i] {
			l1, _ := und.get(k)
			und.set(k, append(l1, a.id))
			l2, _ := und.get(a.id)
			und.set(a.id, append(l2, k))
			d, _ := indeg.get(a.id)
			indeg.set(a.id, d+1)
		}
	}
	visited := map[int]bool{}
	var sources [][]int
	for _, k := range und.keys {
		if visited[k] {
			continue
		}
		comp := []int{k}
		visited[k] = true
		for q := 0; q < len(comp); q++ {
			nb, _ := und.get(comp[q])
			for _, n := range nb {
				if !visited[n] {
					visited[n] = true
					comp = append(comp, n)
				}
			}
		}
		var src []int
		for _, n := range comp {
			if d, _ := indeg.get(n); d == 0 {
				src = append(src, n)
			}
		}
		sources = append(sources, src)
	}

	// fcose keeps each node's predecessors (pastMap: itself, then the
	// union of its processed predecessors', the latest first). Only their
	// first element and, for the sinks, their members are used: first is
	// tracked, and a sink's members found by walking back.
	pos := newOmap[float64]()
	first := map[int]int{}
	processed := map[int]bool{}
	preds := map[int][]int{}
	var queue []int
	for i, k := range indeg.keys {
		if indeg.vals[i] == 0 {
			queue = append(queue, k)
		} else {
			pos.set(k, math.Inf(-1))
		}
		first[k] = k
	}
	for _, src := range sources {
		p := 0.0
		for _, n := range src {
			p += before(n)
		}
		p /= float64(len(src))
		for _, n := range src {
			pos.set(n, p)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		processed[cur] = true
		nbs, _ := graph.get(cur)
		for _, a := range nbs {
			pc, _ := pos.get(cur)
			if pn, _ := pos.get(a.id); pn < pc+a.gap {
				pos.set(a.id, pc+a.gap)
			}
			d, _ := indeg.get(a.id)
			indeg.set(a.id, d-1)
			if d-1 == 0 {
				queue = append(queue, a.id)
			}
			first[a.id] = first[cur]
			preds[a.id] = append(preds[a.id], cur)
		}
	}
	past := func(k int) []int {
		seen := map[int]bool{k: true}
		out := []int{k}
		for i := 0; i < len(out); i++ {
			for _, p := range preds[out[i]] {
				if processed[p] && !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
		}
		return out
	}

	// Recenter each part that ends in a sink, by the first node of its
	// predecessors as fcose looks it up.
	var comps []*oset
	for _, k := range indeg.keys { // pastMap's order
		if list, _ := graph.get(k); len(list) != 0 {
			continue
		}
		at := -1
		for i, c := range comps {
			if c.in[first[k]] {
				at = i
			}
		}
		if at < 0 {
			comps = append(comps, newOset(past(k)...))
		} else {
			for _, n := range past(k) {
				comps[at].add(n)
			}
		}
	}
	for _, c := range comps {
		minB, minA := math.Inf(1), math.Inf(1)
		maxB, maxA := math.Inf(-1), math.Inf(-1)
		for _, n := range c.items {
			b := before(n)
			a, _ := pos.get(n)
			if b < minB {
				minB = b
			}
			if b > maxB {
				maxB = b
			}
			if a < minA {
				minA = a
			}
			if a > maxA {
				maxA = a
			}
		}
		diff := (minB+maxB)/2 - (minA+maxA)/2
		for _, n := range c.items {
			p, _ := pos.get(n)
			pos.set(n, p+diff)
		}
	}
	return pos
}

// relaxation is the state initConstraintVariables sets up for the
// relative placements, one per axis (x: nodesInRelativeHorizontal and
// its maps; y: the vertical ones). Keys are numbered in order of first
// appearance.
type relaxation struct{ h, v axisRelax }

type axisRelax struct {
	order   []int       // the keys, in processing order (shuffled)
	nodes   [][]*lnode  // by key: the node, or the alignment's members
	cons    [][]relCons // by key
	temp    []float64   // by key: the tracked position
	numbers map[int]int // constraint key → number
}

type relCons struct {
	other int  // key number
	after bool // the other is right of (below) this one
	gap   float64
}

func (a *axisRelax) number(l *layout, al alignment, k int, center func(*lnode) float64) int {
	if i, ok := a.numbers[k]; ok {
		return i
	}
	i := len(a.order)
	a.numbers[k] = i
	a.order = append(a.order, i)
	var ns []*lnode
	if k < 0 {
		for _, n := range al.members[k] {
			ns = append(ns, l.byIndex[n])
		}
	} else {
		ns = []*lnode{l.byIndex[k]}
	}
	a.nodes = append(a.nodes, ns)
	a.cons = append(a.cons, nil)
	a.temp = append(a.temp, center(ns[0]))
	return i
}

func (l *layout) initConstraintVariables() {
	c := l.c
	if c == nil || len(c.Relative) == 0 {
		return
	}
	vAlign, hAlign := newAlignment(c.Vertical), newAlignment(c.Horizontal)
	r := &relaxation{h: axisRelax{numbers: map[int]int{}}, v: axisRelax{numbers: map[int]int{}}}
	cx := func(n *lnode) float64 { return n.centerX() }
	cy := func(n *lnode) float64 { return n.centerY() }
	for _, rc := range c.Relative {
		ax, al, center := &r.h, vAlign, cx
		if rc.Vertical {
			ax, al, center = &r.v, hAlign, cy
		}
		a := ax.number(l, al, al.key(rc.A), center)
		b := ax.number(l, al, al.key(rc.B), center)
		ax.cons[a] = append(ax.cons[a], relCons{b, true, rc.Gap})
		ax.cons[b] = append(ax.cons[b], relCons{a, false, rc.Gap})
	}
	l.rel = r
}

func (l *layout) shuffle(a []int) {
	for i := len(a) - 1; float64(i) >= 2*float64(len(a))/3; i-- {
		j := int(math.Floor(l.o.Random() * float64(i+1)))
		a[i], a[j] = a[j], a[i]
	}
}

// updateDisplacements keeps the constraints: an alignment's members move
// by their mean displacement; relative placements are kept on tracked
// positions, in an order shuffled every tenth iteration.
func (l *layout) updateDisplacements() {
	c := l.c
	if c == nil {
		return
	}
	for _, list := range c.Vertical {
		t := 0.0
		for _, n := range list {
			t += l.byIndex[n].dispX
		}
		avg := t / float64(len(list))
		for _, n := range list {
			l.byIndex[n].dispX = avg
		}
	}
	for _, list := range c.Horizontal {
		t := 0.0
		for _, n := range list {
			t += l.byIndex[n].dispY
		}
		avg := t / float64(len(list))
		for _, n := range list {
			l.byIndex[n].dispY = avg
		}
	}
	r := l.rel
	if r == nil {
		return
	}
	if l.totalIterations%10 == 0 {
		l.shuffle(r.h.order)
		l.shuffle(r.v.order)
	}
	relax(&r.h, func(n *lnode) *float64 { return &n.dispX })
	relax(&r.v, func(n *lnode) *float64 { return &n.dispY })
}

func relax(a *axisRelax, disp func(*lnode) *float64) {
	for _, k := range a.order {
		d := *disp(a.nodes[k][0])
		for _, rc := range a.cons[k] {
			if rc.after {
				if diff := (a.temp[rc.other] - a.temp[k]) - d; diff < rc.gap {
					d -= rc.gap - diff
				}
			} else if diff := (a.temp[k] - a.temp[rc.other]) + d; diff < rc.gap {
				d += rc.gap - diff
			}
		}
		a.temp[k] += d
		for _, n := range a.nodes[k] {
			*disp(n) = d
		}
	}
}

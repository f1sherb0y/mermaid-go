package architecture

import (
	"cmp"
	"math"
	"slices"
	"sort"
)

// The grid: every node gets a cell (in units of cells; multiples of 0.5,
// as a node can sit between two others), group by group from the innermost
// out. A group's members are laid out first and then move as one block
// among their siblings, so a group's cells never overlap a sibling's and
// nesting is unlimited. Within a level the items (nodes and member groups)
// are placed by a breadth-first walk over the edges (each edge puts its far
// node one cell off in the direction its sides say: R to L is one to the
// right), `align` hints first; an item whose cell is taken moves aside,
// across the edge's direction. Then each item that is not in a hint moves
// toward where its edges want it (the median, so fans center on their hub).
// Parts of a level that no edge inside it joins go where two edges from
// outside nodes (one, or two in a sibling group laid out before), or else
// mermaid's spatial map, put them; the rest are packed as fcose packs
// disconnected parts. Last, items move where their edges come out
// straighter.

// gp is a point on the grid.
type gp struct{ x, y float64 }

func (a gp) add(b gp) gp                  { return gp{a.x + b.x, a.y + b.y} }
func (a gp) sub(b gp) gp                  { return gp{a.x - b.x, a.y - b.y} }
func (a gp) mul(k float64) gp             { return gp{a.x * k, a.y * k} }
func (a gp) neg() gp                      { return gp{-a.x, -a.y} }
func (a gp) sign() gp                     { return gp{sign(a.x), sign(a.y)} }
func (a gp) straight() bool               { return (a.x == 0) != (a.y == 0) }
func half(v float64) float64              { return math.Round(v*2) / 2 }
func overlap(a0, a1, b0, b1 float64) bool { return a0 < b1-1e-9 && b0 < a1-1e-9 }

// unit is the direction out of side s.
func unit(s Side) gp {
	switch s {
	case 'L':
		return gp{-1, 0}
	case 'R':
		return gp{1, 0}
	case 'T':
		return gp{0, -1}
	}
	return gp{0, 1}
}

func sign(v float64) float64 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// dir is where an edge's far node lies from its near one: one cell out of
// the near side and one into the far side (R to L: right; R to T: right and
// down). Same sides (R to R) say nothing.
func dir(near, far Side) gp { return unit(near).sub(unit(far)).sign() }

// item is a node or a group, as a child of a group (or of the diagram).
type item struct {
	node  *Node
	group *Group
}

// block is a laid-out item: its nodes' cells relative to its origin, and
// the box of those cells.
type block struct {
	pos    map[*Node]gp
	lo, hi gp
}

func leafBlock(n *Node) block { return block{pos: map[*Node]gp{n: {}}} }

// link is where another item's origin goes, from an edge, a hint, or
// (weak) two edges from an outside node or mermaid's spatial map.
type link struct {
	to   int
	off  gp // to's origin minus this item's
	d    gp // the direction it says
	at   gp // where in this item the link's node is
	kind int
	// fan: an edge that shares a side of a node with another edge, which
	// may well run aslant.
	fan bool
}

const (
	hintLink = iota
	edgeLink
	weakLink
)

// grid places the nodes under g.
func (l *layout) grid(g *Group) block {
	items := l.kids[g]
	n := len(items)
	blocks := make([]block, n)
	at := map[*Node]int{}
	for i, it := range items {
		if it.group != nil {
			blocks[i] = l.grid(it.group)
		} else {
			blocks[i] = leafBlock(it.node)
		}
		for m := range blocks[i].pos {
			at[m] = i
		}
	}
	links := make([][]link, n)
	// link puts b at a + d; along is the direction the link says.
	link2 := func(a, b *Node, d, along gp, kind int, fan bool) {
		i, ok1 := at[a]
		j, ok2 := at[b]
		if !ok1 || !ok2 || i == j || along == (gp{}) {
			return
		}
		off := blocks[i].pos[a].add(d).sub(blocks[j].pos[b])
		links[i] = append(links[i], link{j, off, along, blocks[i].pos[a], kind, fan})
		links[j] = append(links[j], link{i, off.neg(), along.neg(), blocks[j].pos[b], kind, fan})
	}
	add := func(a, b *Node, d gp, kind int) { link2(a, b, d, d, kind, false) }
	for _, h := range l.d.Hints {
		d := gp{1, 0}
		if h.Column {
			d = gp{0, 1}
		}
		for k := 1; k < len(h.Members); k++ {
			add(h.Members[k-1], h.Members[k], d, hintLink)
		}
	}
	for _, e := range l.d.Edges {
		d := dir(e.FromSide, e.ToSide)
		link2(e.From, e.To, d, d, edgeLink, l.fan(e))
	}
	// Two edges from one outside node, or from two outside nodes already
	// laid out together (in a sibling group done before this one), say
	// where their inside ends lie from each other.
	outside := map[*Node][]struct {
		n *Node
		d gp
	}{}
	for _, e := range l.d.Edges {
		d := dir(e.FromSide, e.ToSide)
		for _, p := range [][2]*Node{{e.From, e.To}, {e.To, e.From}} {
			if _, in := at[p[0]]; !in {
				if _, in := at[p[1]]; in {
					outside[p[0]] = append(outside[p[0]], struct {
						n *Node
						d gp
					}{p[1], d})
				}
			}
			d = d.neg()
		}
	}
	type end struct {
		n     *Node
		at, d gp // where n goes in the frame; the edge's direction
	}
	byFrame := map[int][]end{}
	var frames []int
	for _, u := range l.nodes {
		f, ok := l.frame[u]
		if !ok {
			f = frame{-1 - l.idx[u], gp{}}
		}
		for _, o := range outside[u] {
			if _, seen := byFrame[f.id]; !seen {
				frames = append(frames, f.id)
			}
			byFrame[f.id] = append(byFrame[f.id], end{o.n, f.at.add(o.d), o.d})
		}
	}
	for _, f := range frames {
		ends := byFrame[f]
		for k := 1; k < len(ends); k++ {
			if d := ends[k].at.sub(ends[0].at); d != (gp{}) {
				add(ends[0].n, ends[k].n, d, weakLink)
			} else {
				// A fan: on one cell, so stacked across the edges.
				link2(ends[0].n, ends[k].n, d, ends[0].d, weakLink, true)
			}
		}
	}
	// Failing that, where mermaid's spatial map puts them: nodes that
	// edges join through other levels.
	// Each from the part's first node there (or the one before, where the
	// map puts the two on one cell).
	first, last := map[int]*Node{}, map[int]*Node{}
	for _, m := range l.nodes {
		if _, in := at[m]; !in || l.stand[m] {
			continue
		}
		c := l.comp[l.idx[m]]
		for _, p := range []*Node{first[c], last[c]} {
			if p != nil && l.spatial[m] != l.spatial[p] {
				add(p, m, l.spatial[m].sub(l.spatial[p]), weakLink)
				break
			}
		}
		if first[c] == nil {
			first[c] = m
		}
		last[c] = m
	}
	// Which way each item's edges leave the level.
	rays := make([][]ray, n)
	for _, u := range l.nodes {
		for _, end := range outside[u] {
			if end.d.straight() {
				i := at[end.n]
				rays[i] = append(rays[i], ray{blocks[i].pos[end.n], end.d.neg()})
			}
		}
	}
	for i := range links {
		slices.SortStableFunc(links[i], func(a, b link) int { return a.kind - b.kind })
	}

	pl := placer{blocks: blocks, links: links, rays: rays, pos: make([]gp, n), comp: make([]int, n)}
	for i := range pl.comp {
		pl.comp[i] = -1
	}
	// Parts joined by edges and hints, each in its own frame.
	var comps [][]int
	for s := range n {
		if pl.comp[s] < 0 {
			comps = append(comps, pl.walk(s, len(comps)))
		}
	}
	for _, c := range comps {
		pl.relax(c)
	}
	// Parts joined by weak links move next to each other; the rest are
	// packed in rows.
	parts := pl.join(comps)
	pl.pack(parts)
	pl.straighten()
	nodes := func(items []int) []*Node {
		var out []*Node
		for _, i := range items {
			for m := range blocks[i].pos {
				out = append(out, m)
			}
		}
		slices.SortFunc(out, func(a, b *Node) int { return l.idx[a] - l.idx[b] })
		return out
	}
	for _, p := range pl.packs {
		l.packs = append(l.packs, centered{nodes(p.part), nodes(p.ref), p.axis})
	}

	b := block{pos: map[*Node]gp{}, lo: gp{math.Inf(1), math.Inf(1)}, hi: gp{math.Inf(-1), math.Inf(-1)}}
	for i, bl := range blocks {
		for m, p := range bl.pos {
			p = p.add(pl.pos[i])
			b.pos[m] = p
			b.lo = gp{math.Min(b.lo.x, p.x), math.Min(b.lo.y, p.y)}
			b.hi = gp{math.Max(b.hi.x, p.x), math.Max(b.hi.y, p.y)}
		}
	}
	l.frames++
	for m, p := range b.pos {
		l.frame[m] = frame{l.frames, p}
	}
	return b
}

// frame is where a node is in the last block laid out around it.
type frame struct {
	id int
	at gp
}

// fan reports that e shares a side of one of its ends with another edge.
func (l *layout) fan(e *Edge) bool {
	return l.sideUse[sideOf{e.From, e.FromSide}] > 1 || l.sideUse[sideOf{e.To, e.ToSide}] > 1
}

// sideOf is a side of a node.
type sideOf struct {
	n *Node
	s Side
}

// placer places one level's items.
type placer struct {
	blocks []block
	links  [][]link
	rays   [][]ray // by item: its edges out of the level
	packs  []packed
	pos    []gp
	comp   []int // the item's part; -1 before it is placed
}

// packed is a part (or a column of them) that pack put beside the items
// in ref, centered on them along axis.
type packed struct {
	part, ref []int
	axis      int
}

// ray is an edge out of the level: from the node at at, toward d.
type ray struct{ at, d gp }

// box is item i's cells at p, half a cell out around its nodes.
func (pl *placer) box(i int, p gp) (lo, hi gp) {
	b := pl.blocks[i]
	return p.add(b.lo).sub(gp{0.5, 0.5}), p.add(b.hi).add(gp{0.5, 0.5})
}

// free reports that item i at p overlaps none of others (but itself).
func (pl *placer) free(i int, p gp, others []int) bool {
	lo, hi := pl.box(i, p)
	for _, j := range others {
		if j == i {
			continue
		}
		lo2, hi2 := pl.box(j, pl.pos[j])
		if overlap(lo.x, hi.x, lo2.x, hi2.x) && overlap(lo.y, hi.y, lo2.y, hi2.y) {
			return false
		}
	}
	return true
}

// spot is where item i goes when u's link lk wants it at p: p, or further
// along the link if the straight edge stays clear of the others (a block
// pushed out of the way), or the first free cell stepping across the link
// (a fan of nodes on one side); hints only move along.
func (pl *placer) spot(i, u int, lk link, others []int) gp {
	p, d := pl.pos[u].add(lk.off), lk.d
	if pl.free(i, p, others) {
		return p
	}
	if d.straight() {
		for k := 1; k < 1000; k++ {
			q := p.add(d.mul(float64(k)))
			if !pl.free(i, q, others) {
				continue
			}
			if lk.kind == hintLink || pl.clear(pl.pos[u].add(lk.at), q.add(lk.at.add(d).sub(lk.off)), u, i, others) {
				return q
			}
			break
		}
	}
	step := d
	switch {
	case lk.kind == hintLink || !d.straight():
	case d.y == 0:
		step = gp{0, 1}
	default:
		step = gp{1, 0}
	}
	for k := 1; k < 1000; k++ {
		if q := p.add(step.mul(float64(k))); pl.free(i, q, others) {
			return q
		}
	}
	return p.add(step.mul(1000))
}

// clear reports that the straight line a-b crosses no item of others but
// u and i.
func (pl *placer) clear(a, b gp, u, i int, others []int) bool {
	lo := gp{math.Min(a.x, b.x), math.Min(a.y, b.y)}
	hi := gp{math.Max(a.x, b.x), math.Max(a.y, b.y)}
	for _, j := range others {
		if j == u || j == i {
			continue
		}
		blo, bhi := pl.box(j, pl.pos[j])
		if blo.x < hi.x-1e-9 && lo.x < bhi.x-1e-9 && blo.y < hi.y-1e-9 && lo.y < bhi.y-1e-9 {
			return false
		}
	}
	return true
}

// walk places the items reachable from s by edges and hints.
func (pl *placer) walk(s, c int) []int {
	part := []int{s}
	pl.comp[s] = c
	for q := 0; q < len(part); q++ {
		u := part[q]
		for _, lk := range pl.links[u] {
			if lk.kind == weakLink || pl.comp[lk.to] >= 0 {
				continue
			}
			pl.pos[lk.to] = pl.spot(lk.to, u, lk, part)
			pl.comp[lk.to] = c
			part = append(part, lk.to)
		}
	}
	return part
}

// relax moves each item that no hint holds to where its edges want it:
// across a row of edges to their median row (the middle of a fan of edges
// that all come from one side), and the same for columns, keeping every
// edge's ends on the sides it says.
func (pl *placer) relax(part []int) {
	held := map[int]bool{}
	for _, i := range part {
		for _, lk := range pl.links[i] {
			if lk.kind == hintLink {
				held[i] = true
			}
		}
	}
	for range 20 {
		moved := false
		for _, v := range part {
			if held[v] {
				continue
			}
			cur := pl.pos[v]
			next := cur
			// x from the vertical edges, y from the horizontal ones.
			for axis := range 2 {
				var want []float64
				sides := map[float64]bool{}
				for _, lk := range pl.links[v] {
					if lk.kind != edgeLink || !lk.d.straight() {
						continue
					}
					w := pl.pos[lk.to].sub(lk.off)
					if axis == 0 && lk.d.x == 0 {
						want, sides[lk.d.y] = append(want, w.x), true
					} else if axis == 1 && lk.d.y == 0 {
						want, sides[lk.d.x] = append(want, w.y), true
					}
				}
				if len(want) == 0 {
					continue
				}
				c := median(want, []float64{cur.x, cur.y}[axis], len(sides) == 1)
				if axis == 0 {
					next.x = c
				} else {
					next.y = c
				}
			}
			for _, p := range []gp{next, {next.x, cur.y}, {cur.x, next.y}} {
				if p != cur && pl.cost(v, p) < pl.cost(v, cur)-1e-9 && pl.keepsSides(v, p) && pl.free(v, p, part) {
					pl.pos[v], moved = p, true
					break
				}
			}
		}
		if !moved {
			return
		}
	}
}

// median is the median of vs; between the two middle values it is the
// middle of them when the edges come from one side (a fan), else the one
// nearest cur.
func median(vs []float64, cur float64, fan bool) float64 {
	sort.Float64s(vs)
	lo, hi := vs[(len(vs)-1)/2], vs[len(vs)/2]
	if fan {
		return half((lo + hi) / 2)
	}
	return math.Max(lo, math.Min(hi, cur))
}

// cost is how far item v at p is from where its straight edges want it,
// across them.
func (pl *placer) cost(v int, p gp) float64 {
	c := 0.0
	for _, lk := range pl.links[v] {
		if lk.kind == edgeLink && lk.d.straight() {
			w := pl.pos[lk.to].sub(lk.off)
			c += math.Abs(w.x-p.x)*math.Abs(lk.d.y) + math.Abs(w.y-p.y)*math.Abs(lk.d.x)
		}
	}
	return c
}

// keepsSides reports that v at p keeps each of its edges' far ends on the
// side the edge says.
func (pl *placer) keepsSides(v int, p gp) bool {
	for _, lk := range pl.links[v] {
		if lk.kind == weakLink {
			continue
		}
		// The far node minus the near one is (want - p) + d.
		w := pl.pos[lk.to].sub(lk.off)
		if lk.d.x != 0 && (w.x-p.x+lk.d.x)*lk.d.x <= 0 || lk.d.y != 0 && (w.y-p.y+lk.d.y)*lk.d.y <= 0 {
			return false
		}
	}
	return true
}

// join moves parts that weak links tie together next to each other, as the
// links say; it returns the joined parts.
func (pl *placer) join(comps [][]int) [][]int {
	var parts [][]int
	done := make([]bool, len(comps))
	for c := range comps {
		if done[c] {
			continue
		}
		done[c] = true
		part := slices.Clone(comps[c])
		for q := 0; q < len(part); q++ {
			for _, lk := range pl.links[part[q]] {
				oc := pl.comp[lk.to]
				if lk.kind != weakLink || done[oc] {
					continue
				}
				done[oc] = true
				// Shift the other part so lk holds, stepping across it while
				// it overlaps, to either side: the first place where its
				// edges out of the level have a clear way, or else the
				// first free one.
				other := comps[oc]
				shift := pl.pos[part[q]].add(lk.off).sub(pl.pos[lk.to])
				shift = pl.shift(other, part, shift, lk.d)
				for _, i := range other {
					pl.pos[i] = pl.pos[i].add(shift)
				}
				part = append(part, other...)
			}
		}
		parts = append(parts, part)
	}
	return parts
}

// shift finds where other goes near shift (by the cells in part),
// stepping across d.
func (pl *placer) shift(other, part []int, shift, d gp) gp {
	step := gp{0, 1}
	if math.Abs(d.x) < math.Abs(d.y) {
		step = gp{1, 0}
	}
	fits := func(s gp) bool {
		for _, i := range other {
			if !pl.free(i, pl.pos[i].add(s), part) {
				return false
			}
		}
		return true
	}
	first, found := gp{}, false
	for k := 0; k < 1000; k++ {
		for _, sg := range []float64{1, -1} {
			if k == 0 && sg < 0 {
				continue
			}
			s := shift.add(step.mul(sg * float64(k)))
			if !fits(s) {
				continue
			}
			if pl.clearRays(other, part, s) {
				return s
			}
			if !found {
				first, found = s, true
			}
		}
		if found && k >= 3 {
			return first
		}
	}
	if found {
		return first
	}
	return shift.add(step.mul(1000))
}

// clearRays reports that no item of part is in the way of the edges out
// of the level from other's items shifted by s.
func (pl *placer) clearRays(other, part []int, s gp) bool {
	for _, i := range other {
		for _, r := range pl.rays[i] {
			p := pl.pos[i].add(s).add(r.at)
			for _, j := range part {
				if lo, hi := pl.box(j, pl.pos[j]); rayCrosses(p, r.d, lo, hi) {
					return false
				}
			}
		}
	}
	return true
}

// pack places the parts that nothing ties together as fcose packs
// disconnected components: the first stays; each other one, largest
// first, goes to its right (stacked in a column, the column centered on
// it) or under it (centered), whichever keeps the whole box smaller, the
// right on a tie (mermaid.live puts 08's Client to the right of the rest
// and 12's group under its row). A part with a group counts half a cell
// bigger all round, for the group's padding and title.
func (pl *placer) pack(parts [][]int) {
	if len(parts) < 2 {
		return
	}
	type rect struct{ lo, hi gp }
	area := func(r rect) float64 { return (r.hi.x - r.lo.x) * (r.hi.y - r.lo.y) }
	union := func(a, b rect) rect {
		return rect{gp{math.Min(a.lo.x, b.lo.x), math.Min(a.lo.y, b.lo.y)}, gp{math.Max(a.hi.x, b.hi.x), math.Max(a.hi.y, b.hi.y)}}
	}
	boxes := make([]rect, len(parts))
	for k, part := range parts {
		r := rect{gp{math.Inf(1), math.Inf(1)}, gp{math.Inf(-1), math.Inf(-1)}}
		grouped := false
		for _, i := range part {
			lo, hi := pl.box(i, pl.pos[i])
			r = union(r, rect{lo, hi})
			grouped = grouped || len(pl.blocks[i].pos) > 1
		}
		if grouped {
			r = rect{r.lo.sub(gp{0.5, 0.5}), r.hi.add(gp{0.5, 0.5})}
		}
		boxes[k] = r
	}
	order := make([]int, len(parts))
	for k := range order {
		order[k] = k
	}
	slices.SortStableFunc(order[1:], func(a, b int) int { return cmp.Compare(area(boxes[b]), area(boxes[a])) })
	move := func(k int, lo gp) {
		shift := lo.sub(boxes[k].lo)
		for _, i := range parts[k] {
			pl.pos[i] = pl.pos[i].add(shift)
		}
		boxes[k] = rect{boxes[k].lo.add(shift), boxes[k].hi.add(shift)}
	}
	size := func(k int) gp { return boxes[k].hi.sub(boxes[k].lo) }
	placed := boxes[order[0]]
	done := slices.Clone(parts[order[0]]) // the items in placed
	var col []int                         // the column being filled, right of placed
	// items lists the items of parts.
	items := func(ks ...int) []int {
		var out []int
		for _, k := range ks {
			out = append(out, parts[k]...)
		}
		return out
	}
	colX := 0.0
	// stack lays the column out, centered on placed.
	stack := func(col []int) rect {
		h, w := 0.0, 0.0
		for _, k := range col {
			h += size(k).y
			w = math.Max(w, size(k).x)
		}
		y := half((placed.lo.y + placed.hi.y - h) / 2)
		r := rect{gp{colX, y}, gp{colX + w, y + h}}
		for _, k := range col {
			move(k, gp{colX + half((w-size(k).x)/2), y})
			y += size(k).y
		}
		return r
	}
	total := func() rect {
		if len(col) == 0 {
			return placed
		}
		return union(placed, stack(col))
	}
	for _, k := range order[1:] {
		sz := size(k)
		// In the column.
		var inCol rect
		inColOK := false
		if len(col) > 0 {
			h := sz.y
			for _, c := range col {
				h += size(c).y
			}
			if h <= placed.hi.y-placed.lo.y {
				inCol, inColOK = union(placed, stack(append(slices.Clone(col), k))), true
				stack(col)
			}
		}
		// A new column, or under.
		all := total()
		right := union(all, rect{gp{all.hi.x, 0}, gp{all.hi.x + sz.x, 0}})
		right.hi.y = math.Max(all.hi.y, half((all.lo.y+all.hi.y+sz.y)/2))
		right.lo.y = math.Min(all.lo.y, half((all.lo.y+all.hi.y-sz.y)/2))
		under := union(all, rect{gp{half((all.lo.x + all.hi.x - sz.x) / 2), all.hi.y}, gp{half((all.lo.x + all.hi.x + sz.x) / 2), all.hi.y + sz.y}})
		switch {
		case inColOK && area(inCol) <= area(right) && area(inCol) <= area(under):
			col = append(col, k)
			stack(col)
			pl.packs[len(pl.packs)-1].part = items(col...)
		case area(right) <= area(under):
			placed = all
			done = append(done, items(col...)...)
			col, colX = []int{k}, all.hi.x
			stack(col)
			pl.packs = append(pl.packs, packed{items(k), slices.Clone(done), 1})
		default:
			placed = all
			done = append(done, items(col...)...)
			col = nil
			move(k, gp{half((all.lo.x + all.hi.x - sz.x) / 2), all.hi.y})
			placed = union(placed, boxes[k])
			pl.packs = append(pl.packs, packed{items(k), slices.Clone(done), 0})
			done = append(done, parts[k]...)
		}
	}
}

// straighten moves items, one at a time, where their edges come out
// straighter: an edge aslant (but in a fan) or a bend the wrong way costs
// 1, a straight edge the wrong way 3,
// and a line through another item, or an edge out of the level whose way
// out another item blocks, 2. An item moves to where one of its edges is
// straight, or a cell over, or swaps with a neighboring node, when that
// lowers the cost; hints hold theirs.
func (pl *placer) straighten() {
	held := make([]bool, len(pl.pos))
	all := make([]int, len(pl.pos))
	for i := range pl.pos {
		all[i] = i
		for _, lk := range pl.links[i] {
			if lk.kind == hintLink {
				held[i] = true
			}
		}
	}
	for range 6 {
		moved := false
		for v := range pl.pos {
			if held[v] {
				continue
			}
			cur := pl.pos[v]
			best, bestCost := cur, pl.local(v, cur)
			if bestCost == 0 {
				continue
			}
			var cands []gp
			for _, lk := range pl.links[v] {
				if lk.kind == edgeLink {
					cands = append(cands, pl.pos[lk.to].sub(lk.off))
				}
			}
			cands = append(cands, cur.add(gp{1, 0}), cur.add(gp{-1, 0}), cur.add(gp{0, 1}), cur.add(gp{0, -1}))
			for _, p := range cands {
				if p == cur || !pl.free(v, p, all) {
					continue
				}
				if c := pl.local(v, p); c < bestCost-1e-9 {
					best, bestCost = p, c
				}
			}
			if best != cur {
				pl.pos[v], moved = best, true
				continue
			}
			// Swapping two neighboring nodes.
			if len(pl.blocks[v].pos) != 1 {
				continue
			}
			// (What the two touch is all that changes; a term they share
			// counts twice both ways.)
			for w := range pl.pos {
				pw := pl.pos[w]
				if w == v || held[w] || len(pl.blocks[w].pos) != 1 || math.Abs(pw.x-cur.x) > 1 || math.Abs(pw.y-cur.y) > 1 {
					continue
				}
				before := bestCost + pl.local(w, pw)
				pl.pos[v], pl.pos[w] = pw, cur
				if pl.local(v, pw)+pl.local(w, cur) < before-1e-9 {
					moved = true
					break
				}
				pl.pos[v], pl.pos[w] = cur, pw
			}
		}
		if !moved {
			return
		}
	}
}

// local is the cost of what item v at p touches: its edges, its ways out
// of the level, and the others' lines and ways out through its box.
func (pl *placer) local(v int, p gp) float64 {
	c := 0.0
	lo, hi := pl.box(v, p)
	for _, lk := range pl.links[v] {
		if lk.kind != edgeLink {
			continue
		}
		q := pl.pos[lk.to]
		c += misfit(lk, p, q)
		a := p.add(lk.at)
		b := q.add(lk.at).add(lk.d).sub(lk.off)
		if pl.blocked(a, b, v, lk.to, v, p) {
			c += 2
		}
	}
	for _, r := range pl.rays[v] {
		if pl.rayBlocked(p.add(r.at), r.d, v, v, p) {
			c += 2
		}
	}
	for i := range pl.pos {
		if i == v {
			continue
		}
		for _, lk := range pl.links[i] {
			if lk.kind != edgeLink || lk.to == v || lk.to < i {
				continue
			}
			a := pl.pos[i].add(lk.at)
			b := pl.pos[lk.to].add(lk.at).add(lk.d).sub(lk.off)
			if segCrosses(a, b, lo, hi) {
				c += 2
			}
		}
		for _, r := range pl.rays[i] {
			if rayCrosses(pl.pos[i].add(r.at), r.d, lo, hi) {
				c += 2
			}
		}
	}
	return c
}

// misfit is how far a link from an item at p to one at q is from what it
// says: 0 as it says, 1 aslant (0 in a fan) or a bend the wrong way (it
// goes around), 3 a straight edge the wrong way.
func misfit(lk link, p, q gp) float64 {
	delta := q.sub(p).add(lk.d).sub(lk.off) // the far node from the near one
	d := lk.d
	if !d.straight() {
		if delta.x*d.x <= 0 || delta.y*d.y <= 0 {
			return 1
		}
		return 0
	}
	switch {
	case delta.x*d.x+delta.y*d.y <= 0:
		return 3
	case delta.x*math.Abs(d.y)+delta.y*math.Abs(d.x) != 0 && !lk.fan:
		return 1
	}
	return 0
}

// blocked reports that the line a-b crosses an item but i and j, with item
// v at p.
func (pl *placer) blocked(a, b gp, i, j, v int, p gp) bool {
	for k := range pl.pos {
		if k == i || k == j {
			continue
		}
		at := pl.pos[k]
		if k == v {
			at = p
		}
		lo, hi := pl.box(k, at)
		if segCrosses(a, b, lo, hi) {
			return true
		}
	}
	return false
}

// rayBlocked reports that an item but i is in the way from a toward d,
// with item v at p.
func (pl *placer) rayBlocked(a, d gp, i, v int, p gp) bool {
	for k := range pl.pos {
		if k == i {
			continue
		}
		at := pl.pos[k]
		if k == v {
			at = p
		}
		lo, hi := pl.box(k, at)
		if rayCrosses(a, d, lo, hi) {
			return true
		}
	}
	return false
}

// segCrosses reports that segment a-b passes through the inside of the box
// lo-hi.
func segCrosses(a, b, lo, hi gp) bool {
	const eps = 1e-6
	t0, t1 := 0.0, 1.0
	dx, dy := b.x-a.x, b.y-a.y
	for _, c := range [4][2]float64{{-dx, a.x - lo.x - eps}, {dx, hi.x - eps - a.x}, {-dy, a.y - lo.y - eps}, {dy, hi.y - eps - a.y}} {
		if c[0] == 0 {
			if c[1] < 0 {
				return false
			}
			continue
		}
		t := c[1] / c[0]
		if c[0] < 0 {
			t0 = math.Max(t0, t)
		} else {
			t1 = math.Min(t1, t)
		}
		if t0 >= t1 {
			return false
		}
	}
	return true
}

// rayCrosses reports that the ray from a toward d (straight) passes
// through the inside of the box lo-hi.
func rayCrosses(a, d, lo, hi gp) bool {
	if d.x != 0 {
		return lo.y < a.y && a.y < hi.y && math.Max((lo.x-a.x)*d.x, (hi.x-a.x)*d.x) > 0
	}
	return lo.x < a.x && a.x < hi.x && math.Max((lo.y-a.y)*d.y, (hi.y-a.y)*d.y) > 0
}

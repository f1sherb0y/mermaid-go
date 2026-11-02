package architecture

import (
	"math"
	"slices"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

// The layout's sizes: nodes' boxes (an icon, and its label as cytoscape
// sizes it for fcose: a line height a line), labels wrapped as mermaid
// wraps them, and the groups' boxes, their members' plus padding, widened
// for a title wider than the box or that an edge would cross. The grid is
// in grid.go, the pixels in pixels.go, the edges' courses in route.go.

// extent is a box around a node's center.
type extent struct{ l, t, r, b float64 }

// textBlock is wrapped text.
type textBlock struct {
	lines []string
	w     float64
}

type layout struct {
	d   *Diagram
	ctx *diagram.Context
	// nodes are the diagram's, then a stand-in for each empty group.
	nodes []*Node
	stand map[*Node]bool
	kids  map[*Group][]item
	first map[item]int // the item's first node, by index in nodes

	iconSize, padding, pitch, compact, gap float64
	font                                   text.Style
	asc, desc, xh, lineH                   float64
	labels                                 map[*Node]textBlock
	edgeLabels                             map[*Edge]textBlock
	titles                                 map[*Group]float64 // title widths
	ext                                    map[*Node]extent
	// textB is how far under a node's center its drawn label ends.
	textB map[*Node]float64
	idx   map[*Node]int // index in nodes
	comp  []int         // by index: the part of the diagram edges join
	// spatial is mermaid's spatial map.
	spatial map[*Node]gp
	frame   map[*Node]frame
	frames  int
	sideUse map[sideOf]int     // edges by side of a node
	packs   []centered         // parts that no edge places, centered on others
	pad     map[*Group]*extent // padding: l, t, r, b all positive

	cell   map[*Node]gp
	at     map[*Node]scene.Pt
	box    map[*Group]scene.Rect
	routes map[*Edge][]scene.Pt

	// choice is the layout kept, "fcose" or "grid", and the scores.
	choice        string
	qFcose, qGrid quality
}

func newLayout(d *Diagram, ctx *diagram.Context) *layout {
	cfg, th := ctx.Config, ctx.Theme
	l := &layout{d: d, ctx: ctx, stand: map[*Node]bool{}, kids: map[*Group][]item{}, first: map[item]int{},
		labels: map[*Node]textBlock{}, edgeLabels: map[*Edge]textBlock{}, titles: map[*Group]float64{},
		ext: map[*Node]extent{}, pad: map[*Group]*extent{}, textB: map[*Node]float64{}, idx: map[*Node]int{},
		frame: map[*Node]frame{}, sideUse: map[sideOf]int{}}
	for _, e := range d.Edges {
		l.sideUse[sideOf{e.From, e.FromSide}]++
		l.sideUse[sideOf{e.To, e.ToSide}]++
	}
	l.iconSize = cfg.Num("architecture.iconSize", 80)
	l.padding = cfg.Num("architecture.padding", 40)
	l.pitch = l.iconSize * (1 + cfg.Num("architecture.idealEdgeLengthMultiplier", 1.5))
	l.compact = l.iconSize + 0.4*(l.pitch-l.iconSize)
	l.gap = cfg.Num("architecture.nodeSeparation", 75)
	l.font = text.Style{Family: th.Get("fontFamily"), Size: style.Px(th.Get("fontSize"), 16)}
	l.asc, l.desc = ctx.Fonts.Extents(l.font)
	l.xh = ctx.Fonts.XHeight(l.font)
	l.lineH = 1.1 * l.font.Size

	l.nodes = slices.Clone(d.Nodes)
	hasKids := map[*Group]bool{}
	for _, n := range d.Nodes {
		if n.Parent != nil {
			hasKids[n.Parent] = true
		}
	}
	for _, g := range d.Groups {
		if g.Parent != nil {
			hasKids[g.Parent] = true
		}
	}
	for _, g := range d.Groups {
		if !hasKids[g] {
			n := &Node{Junction: true, Parent: g}
			l.stand[n] = true
			l.nodes = append(l.nodes, n)
		}
	}
	// The tree, each level ordered by first node.
	for i, n := range l.nodes {
		l.idx[n] = i
		it := item{node: n}
		l.first[it] = i
		l.kids[n.Parent] = append(l.kids[n.Parent], it)
		for g := n.Parent; g != nil; g = g.Parent {
			gi := item{group: g}
			if _, ok := l.first[gi]; !ok {
				l.first[gi] = i
				l.kids[g.Parent] = append(l.kids[g.Parent], gi)
			}
		}
	}
	for g, ks := range l.kids {
		slices.SortStableFunc(ks, func(a, b item) int { return l.first[a] - l.first[b] })
		l.kids[g] = ks
	}
	l.measure()
	return l
}

// measure sizes the labels and the nodes' boxes.
func (l *layout) measure() {
	half := l.iconSize / 2
	for _, n := range l.nodes {
		switch {
		case l.stand[n]:
			l.ext[n] = extent{-half / 2, -half / 2, half / 2, half / 2}
		case n.Title != "":
			tb := l.wrap(n.Title, 1.5*l.iconSize)
			l.labels[n] = tb
			w := math.Max(half, tb.w/2)
			// The label takes a line height per line under the icon, as
			// cytoscape sizes it for fcose and the groups; the text is drawn
			// a little lower (its first line's middle an em under the icon),
			// into the padding, as in mermaid.
			lines := float64(len(tb.lines))
			l.ext[n] = extent{-w, -half, w, half + lines*l.lineH}
			l.textB[n] = half + l.font.Size + (lines-1)*l.lineH + l.xh/2 + l.desc
		default:
			l.ext[n] = extent{-half, -half, half, half}
		}
	}
	for _, e := range l.d.Edges {
		if e.Label == "" {
			continue
		}
		// mermaid wraps a label to the edge's length (along a column, two
		// thirds of it); the default distance between icons, here.
		width := l.pitch - l.iconSize
		if !isX(e.FromSide) && !isX(e.ToSide) {
			width /= 1.5
		}
		l.edgeLabels[e] = l.wrap(e.Label, width)
	}
	for _, g := range l.d.Groups {
		p := l.padding + 2.5
		l.pad[g] = &extent{p, p, p, p}
		if g.Title != "" {
			l.titles[g] = l.ctx.Fonts.Width(g.Title, l.font)
		}
	}
}

func isX(s Side) bool { return s == 'L' || s == 'R' }

// wrap breaks text into lines no wider than width, as mermaid's createText:
// at spaces, and inside a word that is too wide on its own.
func (l *layout) wrap(s string, width float64) textBlock {
	fs := l.ctx.Fonts
	var tb textBlock
	put := func(line string) {
		tb.lines = append(tb.lines, line)
		tb.w = math.Max(tb.w, fs.Width(line, l.font))
	}
	cur := ""
	for _, word := range strings.Fields(s) {
		if cur != "" && fs.Width(cur+" "+word, l.font) <= width {
			cur += " " + word
			continue
		}
		if cur != "" {
			put(cur)
			cur = ""
		}
		for fs.Width(word, l.font) > width {
			rs := []rune(word)
			k := 1
			for k < len(rs) && fs.Width(string(rs[:k+1]), l.font) <= width {
				k++
			}
			put(string(rs[:k]))
			word = string(rs[k:])
		}
		cur = word
	}
	if cur != "" || len(tb.lines) == 0 {
		put(cur)
	}
	return tb
}

// key is a grid coordinate as an exact map key.
func key(v float64) int { return int(math.Round(v * 2)) }

// span is an item's nodes and the range of their cells.
type span struct {
	nodes  []*Node
	lo, hi gp
}

func (l *layout) span(it item) span {
	var s span
	if it.node != nil {
		s.nodes = []*Node{it.node}
	} else {
		for _, n := range l.nodes {
			if inside(n, it.group) {
				s.nodes = append(s.nodes, n)
			}
		}
	}
	s.lo, s.hi = gp{math.Inf(1), math.Inf(1)}, gp{math.Inf(-1), math.Inf(-1)}
	for _, n := range s.nodes {
		c := l.cell[n]
		s.lo = gp{math.Min(s.lo.x, c.x), math.Min(s.lo.y, c.y)}
		s.hi = gp{math.Max(s.hi.x, c.x), math.Max(s.hi.y, c.y)}
	}
	return s
}

// inside reports that n is in g, at any depth.
func inside(n *Node, g *Group) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p == g {
			return true
		}
	}
	return false
}

// pads is the padding of the groups around n up to and including it's
// item (none for a node), on one side (0 l, 1 t, 2 r, 3 b).
func (l *layout) pads(n *Node, it item, side int) float64 {
	if it.group == nil {
		return 0
	}
	sum := 0.0
	for g := n.Parent; g != nil; g = g.Parent {
		p := l.pad[g]
		sum += [4]float64{p.l, p.t, p.r, p.b}[side]
		if g == it.group {
			break
		}
	}
	return sum
}

// beside reports that two siblings must be kept apart along axis: their
// rows overlap (for columns), or their boxes' columns do (for rows).
func (l *layout) beside(a, b item, sa, sb span, axis int) bool {
	if axis == 0 {
		return overlap(sa.lo.y-0.5, sa.hi.y+0.5, sb.lo.y-0.5, sb.hi.y+0.5)
	}
	ra, rb := l.itemBox(a), l.itemBox(b)
	return overlap(ra.X, ra.X+ra.W+l.gap/2, rb.X, rb.X+rb.W+l.gap/2)
}

func (l *layout) itemBox(it item) scene.Rect {
	if it.group != nil {
		return l.box[it.group]
	}
	c, e := l.at[it.node], l.ext[it.node]
	return scene.Rect{X: c.X + e.l, Y: c.Y + e.t, W: e.r - e.l, H: e.b - e.t}
}

// boxes computes the groups' boxes from their members, innermost first.
func (l *layout) boxes() {
	l.box = map[*Group]scene.Rect{}
	var walk func(g *Group) scene.Rect
	walk = func(g *Group) scene.Rect {
		x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, it := range l.kids[g] {
			r := l.itemBox(it)
			if it.group != nil {
				// cytoscape's bounds are a pixel out around a node, not
				// around a group: a nested group adds a pixel less.
				r = walk(it.group)
				r = scene.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}
			}
			x0, y0, x1, y1 = math.Min(x0, r.X), math.Min(y0, r.Y), math.Max(x1, r.X+r.W), math.Max(y1, r.Y+r.H)
		}
		p := l.pad[g]
		r := scene.Rect{X: x0 - p.l, Y: y0 - p.t, W: x1 - x0 + p.l + p.r, H: y1 - y0 + p.t + p.b}
		l.box[g] = r
		return r
	}
	for _, it := range l.kids[nil] {
		if it.group != nil {
			walk(it.group)
		}
	}
}

// widen pads the groups whose title is wider than their box, evenly on
// both sides, and the groups whose title an edge would cross: on the left
// so that the title ends before the edge (or on top, for an edge across
// it). It reports a change.
func (l *layout) widen() bool {
	changed := false
	for g, w := range l.titles {
		need := l.titleX(g) + w + 10
		if b := l.box[g]; b.W < need-0.5 {
			extra := (need - b.W) / 2
			l.pad[g].l += extra
			l.pad[g].r += extra
			changed = true
		}
	}
	const margin = 12
	courses := make([][]scene.Pt, len(l.d.Edges))
	for i, e := range l.d.Edges {
		courses[i] = l.course(e)
	}
	for _, g := range l.d.Groups {
		r, ok := l.titleBox(g)
		if !ok {
			continue
		}
		left, top := 0.0, 0.0
		for _, pts := range courses {
			for i := 1; i < len(pts); i++ {
				a, b := pts[i-1], pts[i]
				if !crosses(a, b, scene.Rect{X: r.X, Y: r.Y, W: r.W + margin, H: r.H + margin}) {
					continue
				}
				if math.Abs(b.X-a.X) <= math.Abs(b.Y-a.Y) {
					// Where it leaves the title's band.
					y := r.Y + r.H
					x := a.X
					if b.Y != a.Y {
						x += (b.X - a.X) * (y - a.Y) / (b.Y - a.Y)
					}
					left = math.Max(left, r.X+r.W+margin-x)
				} else {
					top = math.Max(top, r.Y+r.H+margin-math.Min(a.Y, b.Y))
				}
			}
		}
		if left > 0.5 || top > 0.5 {
			l.pad[g].l += left
			l.pad[g].t += top
			changed = true
		}
	}
	return changed
}

// titleX is where a group's title starts, after its icon.
func (l *layout) titleX(g *Group) float64 {
	if g.Icon != "" {
		return 0.75*l.padding + 4
	}
	return 4
}

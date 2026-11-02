package dagre

import (
	"math"
	"slices"
	"strings"

	"github.com/f1sherb0y/mermaid-go/graph"
)

// position assigns y by rank and x by Brandes-Köpf, on a copy of the graph
// without clusters (whose adjacency order is the copy's own, see
// asNonCompoundGraph).
func (l *layout) position() error {
	g := asNonCompoundGraph(l.g)
	layering := buildLayerMatrix(g)
	if hasHoles(layering) {
		return errSameOrder
	}
	// Coordinates start undefined in dagre; a node it fails to place keeps
	// them so, and they turn into NaN.
	g.EachNode(func(_ string, n *node) { n.x, n.y = math.NaN(), math.NaN() })
	l.positionY(g, layering)
	for v, x := range l.positionX(g, layering) {
		g.Node(v).x = x
	}
	return nil
}

func (l *layout) positionY(g *lgraph, layering [][]string) {
	prevY := 0.0
	for _, layer := range layering {
		maxHeight := math.NaN() // lodash's max of nothing is undefined
		for i, v := range layer {
			if h := g.Node(v).height; i == 0 || h > maxHeight {
				maxHeight = h
			}
		}
		for _, v := range layer {
			g.Node(v).y = prevY + maxHeight/2
		}
		prevY += maxHeight + l.rankSep
	}
}

// Brandes and Köpf, "Fast and Simple Horizontal Coordinate Assignment",
// with dagre's changes.

// conflicts is a set of unordered node pairs.
type conflicts map[[2]string]struct{}

func (c conflicts) add(v, w string) {
	if w < v {
		v, w = w, v
	}
	c[[2]string{v, w}] = struct{}{}
}

func (c conflicts) has(v, w string) bool {
	if w < v {
		v, w = w, v
	}
	_, ok := c[[2]string{v, w}]
	return ok
}

// findType1Conflicts marks non-inner segments that cross inner segments
// (edges between two dummies).
func findType1Conflicts(g *lgraph, layering [][]string, c conflicts) {
	for li := 1; li < len(layering); li++ {
		prev, layer := layering[li-1], layering[li]
		k0, scanPos := 0, 0
		for i, v := range layer {
			w := otherInnerSegmentNode(g, v)
			k1 := len(prev)
			if w != "" {
				k1 = g.Node(w).order
			}
			if w == "" && i != len(layer)-1 {
				continue
			}
			for _, sn := range layer[scanPos : i+1] {
				snDummy := g.Node(sn).dummy != notDummy
				for _, u := range g.Predecessors(sn) {
					un := g.Node(u)
					if (un.order < k0 || k1 < un.order) && !(un.dummy != notDummy && snDummy) {
						c.add(u, sn)
					}
				}
			}
			scanPos = i + 1
			k0 = k1
		}
	}
}

func otherInnerSegmentNode(g *lgraph, v string) string {
	if g.Node(v).dummy != notDummy {
		for _, u := range g.Predecessors(v) {
			if g.Node(u).dummy != notDummy {
				return u
			}
		}
	}
	return ""
}

// findType2Conflicts marks inner segments crossing cluster border segments.
func findType2Conflicts(g *lgraph, layering [][]string, c conflicts) {
	scan := func(south []string, from, to int, prevBorder, nextBorder float64) {
		for _, v := range south[from:to] {
			if g.Node(v).dummy == notDummy {
				continue
			}
			for _, u := range g.Predecessors(v) {
				un := g.Node(u)
				if o := float64(un.order); un.dummy != notDummy && (o < prevBorder || o > nextBorder) {
					c.add(u, v)
				}
			}
		}
	}
	for li := 1; li < len(layering); li++ {
		north, south := layering[li-1], layering[li]
		// nextNorthPos starts undefined: NaN compares false like it.
		prevNorthPos, nextNorthPos := -1.0, math.NaN()
		southPos := 0
		for lookahead, v := range south {
			if g.Node(v).dummy == dummyBorder {
				if preds := g.Predecessors(v); len(preds) > 0 {
					nextNorthPos = float64(g.Node(preds[0]).order)
					scan(south, southPos, lookahead, prevNorthPos, nextNorthPos)
					southPos = lookahead
					prevNorthPos = nextNorthPos
				}
			}
			// dagre-d3-es runs this inside the loop, once per node.
			scan(south, southPos, len(south), nextNorthPos, float64(len(north)))
		}
	}
}

// verticalAlignment aligns each node with a median neighbour where no
// conflict or earlier alignment forbids it, forming blocks. root maps a node
// to its block's root; align links each block into a cycle.
func verticalAlignment(layering [][]string, c conflicts, neighbors func(string) []string) (root, align map[string]string) {
	root, align = map[string]string{}, map[string]string{}
	pos := map[string]int{}
	for _, layer := range layering {
		for i, v := range layer {
			root[v], align[v], pos[v] = v, v, i
		}
	}
	for _, layer := range layering {
		prevIdx := -1
		for _, v := range layer {
			ws := neighbors(v)
			if len(ws) == 0 {
				continue
			}
			slices.SortStableFunc(ws, func(a, b string) int { return pos[a] - pos[b] })
			for i := (len(ws) - 1) / 2; i <= len(ws)/2; i++ {
				w := ws[i]
				if align[v] == v && prevIdx < pos[w] && !c.has(v, w) {
					align[w] = v
					root[v] = root[w]
					align[v] = root[v]
					prevIdx = pos[w]
				}
			}
		}
	}
	return root, align
}

// horizontalCompaction places blocks: first as far left as separation
// allows, then each as far right as its right neighbours allow.
func (l *layout) horizontalCompaction(g *lgraph, layering [][]string, root, align map[string]string, reverseSep bool) map[string]float64 {
	bg := l.buildBlockGraph(g, layering, root, reverseSep)
	border := borderRight
	if reverseSep {
		border = borderLeft
	}
	xs := map[string]float64{}
	x := func(v string) float64 {
		if x, ok := xs[v]; ok {
			return x
		}
		return math.NaN() // undefined in dagre
	}
	iterate := func(set func(string), next func(string) []string) {
		stack := bg.Nodes()
		visited := map[string]bool{}
		for len(stack) > 0 {
			v := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if v == "" {
				break // dagre's loop ends on a falsy id
			}
			if visited[v] {
				set(v)
			} else {
				visited[v] = true
				stack = append(stack, v)
				stack = append(stack, next(v)...)
			}
		}
	}
	iterate(func(v string) {
		acc := 0.0
		for _, e := range bg.InEdges(v, "") {
			acc = max(acc, x(e.V)+bg.Edge(e))
		}
		xs[v] = acc
	}, bg.Predecessors)
	iterate(func(v string) {
		m := math.Inf(1)
		for _, e := range bg.OutEdges(v, "") {
			m = min(m, x(e.W)-bg.Edge(e))
		}
		if m != math.Inf(1) && g.Node(v).borderType != border {
			xs[v] = max(x(v), m)
		}
	}, bg.Successors)
	// Every node takes its block root's x. This visits align in JS key
	// order, which matters only when a root is not its own root (possible
	// when edges within a rank or upward survive a greedy acyclicer).
	var keys []string
	for _, layer := range layering {
		keys = append(keys, layer...)
	}
	for _, k := range graph.KeyOrder(keys) {
		v := align[k]
		xs[v] = x(root[v])
	}
	return xs
}

func (l *layout) buildBlockGraph(g *lgraph, layering [][]string, root map[string]string, reverseSep bool) *graph.Graph[struct{}, float64] {
	bg := graph.New[struct{}, float64](graph.Options{})
	for _, layer := range layering {
		u := ""
		for _, v := range layer {
			vRoot := root[v]
			bg.EnsureNode(vRoot)
			if u != "" {
				k := graph.Edge{V: root[u], W: vRoot}
				bg.SetEdge(k, max(l.sep(g, v, u, reverseSep), bg.Edge(k)))
			}
			u = v
		}
	}
	return bg
}

// sep is the least distance between the centers of v and its left
// neighbour w.
func (l *layout) sep(g *lgraph, v, w string, reverseSep bool) float64 {
	vl, wl := g.Node(v), g.Node(w)
	sum := vl.width / 2
	shift := func(delta float64) {
		if truthy(delta) {
			if reverseSep {
				sum += delta
			} else {
				sum -= delta
			}
		}
	}
	if vl.dummy == dummyEdgeLabel {
		switch strings.ToLower(vl.labelPos) {
		case "l":
			shift(-vl.width / 2)
		case "r":
			shift(vl.width / 2)
		}
	}
	sum += l.halfSep(vl)
	sum += l.halfSep(wl)
	sum += wl.width / 2
	if wl.dummy == dummyEdgeLabel {
		switch strings.ToLower(wl.labelPos) {
		case "l":
			shift(wl.width / 2)
		case "r":
			shift(-wl.width / 2)
		}
	}
	return sum
}

func (l *layout) halfSep(n *node) float64 {
	if n.dummy != notDummy {
		return l.opt.EdgeSep / 2
	}
	return l.opt.NodeSep / 2
}

// positionX runs the four alignments (up/down, left/right), aligns them to
// the narrowest and takes the mean of the middle two (or the one asked for).
func (l *layout) positionX(g *lgraph, layering [][]string) map[string]float64 {
	c := conflicts{}
	findType1Conflicts(g, layering, c)
	findType2Conflicts(g, layering, c)

	var xss [4]map[string]float64 // ul, ur, dl, dr
	for vi, vert := range "ud" {
		adjusted := layering
		if vert == 'd' {
			adjusted = slices.Clone(layering)
			slices.Reverse(adjusted)
		}
		neighbors := g.Predecessors
		if vert == 'd' {
			neighbors = g.Successors
		}
		for hi := range 2 {
			right := hi == 1
			if right {
				r := make([][]string, len(adjusted))
				for i, layer := range adjusted {
					r[i] = slices.Clone(layer)
					slices.Reverse(r[i])
				}
				adjusted = r
			}
			root, align := verticalAlignment(adjusted, c, neighbors)
			xs := l.horizontalCompaction(g, adjusted, root, align, right)
			if right {
				for v, x := range xs {
					xs[v] = -x
				}
			}
			xss[vi*2+hi] = xs
			if traceBK != nil {
				traceBK(string(vert)+[]string{"l", "r"}[hi], adjusted, root, xs)
			}
		}
	}

	// The narrowest alignment, first wins (lodash minBy skips NaN).
	smallest, best := -1, 0.0
	for i, xs := range xss {
		hi, lo := math.Inf(-1), math.Inf(1)
		for v, x := range xs {
			hw := g.Node(v).width / 2
			hi = math.Max(x+hw, hi)
			lo = math.Min(x-hw, lo)
		}
		if w := hi - lo; !math.IsNaN(w) && (smallest < 0 || w < best) {
			smallest, best = i, w
		}
	}

	// Align the others to it: left alignments by their minimum, right ones
	// by their maximum.
	alignLo, alignHi := math.NaN(), math.NaN()
	if smallest >= 0 {
		alignLo, alignHi = extremes(xss[smallest])
	}
	for i, xs := range xss {
		if i == smallest {
			continue
		}
		lo, hi := extremes(xs)
		delta := alignLo - lo
		if i%2 == 1 {
			delta = alignHi - hi
		}
		if truthy(delta) {
			for v, x := range xs {
				xs[v] = x + delta
			}
		}
	}

	out := make(map[string]float64, len(xss[0]))
	pick := -1
	switch strings.ToLower(l.opt.Align) {
	case "ul":
		pick = 0
	case "ur":
		pick = 1
	case "dl":
		pick = 2
	case "dr":
		pick = 3
	}
	for v := range xss[0] {
		if pick >= 0 {
			out[v] = xss[pick][v]
			continue
		}
		s := [4]float64{xss[0][v], xss[1][v], xss[2][v], xss[3][v]}
		sortByNumber(s[:])
		out[v] = (s[1] + s[2]) / 2
	}
	return out
}

// extremes is lodash's min and max of the values: NaN is skipped, nothing
// gives NaN (undefined).
func extremes(xs map[string]float64) (lo, hi float64) {
	lo, hi = math.NaN(), math.NaN()
	for _, x := range xs {
		if math.IsNaN(x) {
			continue
		}
		if math.IsNaN(lo) || x < lo {
			lo = x
		}
		if math.IsNaN(hi) || x > hi {
			hi = x
		}
	}
	return lo, hi
}

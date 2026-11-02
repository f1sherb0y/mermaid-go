package dagre

import "github.com/f1sherb0y/mermaid-go/graph"

// rank assigns ranks that respect every edge's minlen (Gansner et al., "A
// Technique for Drawing Directed Graphs"). g must be a connected DAG; ranks
// may start anywhere, normalizeRanks fixes that later.
func (l *layout) rank(g *lgraph) {
	switch l.opt.Ranker {
	case "tight-tree":
		longestPath(g)
		feasibleTree(g)
	case "longest-path":
		longestPath(g)
	default:
		networkSimplex(g)
	}
}

type minLener interface{ minlen() int }

// longestPath puts every node as low as its out-edges allow (sinks at 0,
// the rest below), searching from the sources in graph order.
//
// The graph can still have a cycle (the greedy acyclicer ignores zero-weight
// edges). dagre then meets a node still being searched, whose rank is
// undefined; the NaN it computes is skipped by lodash's min. done mirrors
// that: false means "no rank yet".
func longestPath[E minLener](g *graph.Graph[*node, E]) {
	state := map[string]bool{} // false: being searched, true: ranked
	var dfs func(v string) (int, bool)
	dfs = func(v string) (int, bool) {
		n := g.Node(v)
		if done, seen := state[v]; seen {
			return n.rank, done
		}
		state[v] = false
		r, any := 0, false
		for _, e := range g.OutEdges(v, "") {
			x, ok := dfs(e.W)
			if x -= g.Edge(e).minlen(); ok && (!any || x < r) {
				r, any = x, true
			}
		}
		n.rank = r
		state[v] = true
		return r, true
	}
	for _, v := range g.Sources() {
		dfs(v)
	}
}

func slack[E minLener](g *graph.Graph[*node, E], e graph.Edge) int {
	return g.Node(e.W).rank - g.Node(e.V).rank - g.Edge(e).minlen()
}

// tnode is a node label of the spanning tree.
type tnode struct {
	low, lim int
	parent   string // "" at the root
}

// tree is the spanning tree; edge labels are cut values. It is undirected,
// so its neighbor lists, and the low/lim numbering, follow JS string order
// of the node names (dummy names included).
type tree = graph.Graph[*tnode, float64]

// feasibleTree grows a spanning tree of tight edges from the first node,
// shifting the tree's ranks toward the closest outside node whenever it
// runs out of tight edges.
func feasibleTree[E minLener](g *graph.Graph[*node, E]) *tree {
	t := graph.New[*tnode, float64](graph.Options{Undirected: true})
	t.SetNode(g.Nodes()[0], &tnode{})
	for size := g.NodeCount(); tightTree(t, g) < size; {
		e := findMinSlackEdge(t, g)
		delta := slack(g, e)
		if !t.HasNode(e.V) {
			delta = -delta
		}
		for _, v := range t.Nodes() {
			g.Node(v).rank += delta
		}
	}
	return t
}

func tightTree[E minLener](t *tree, g *graph.Graph[*node, E]) int {
	var dfs func(v string)
	dfs = func(v string) {
		for _, e := range g.NodeEdges(v, "") {
			w := e.V
			if v == e.V {
				w = e.W
			}
			if !t.HasNode(w) && slack(g, e) == 0 {
				t.SetNode(w, &tnode{})
				t.SetEdge(graph.Edge{V: v, W: w}, 0)
				dfs(w)
			}
		}
	}
	for _, v := range t.Nodes() {
		dfs(v)
	}
	return t.NodeCount()
}

// findMinSlackEdge returns the first edge, in graph order, of least slack
// among those with exactly one end in the tree (lodash minBy).
func findMinSlackEdge[E minLener](t *tree, g *graph.Graph[*node, E]) graph.Edge {
	var best graph.Edge
	bestSlack, found := 0, false
	for _, e := range g.Edges() {
		if t.HasNode(e.V) != t.HasNode(e.W) {
			if s := slack(g, e); !found || s < bestSlack {
				best, bestSlack, found = e, s, true
			}
		}
	}
	return best
}

type nsGraph = graph.Graph[*node, *nsEdge]

// networkSimplex ranks g optimally: longest path, then a feasible tight
// tree, then exchanging tree edges with negative cut values (first in tree
// edge order) for the entering edge of least slack (first in graph order).
func networkSimplex(lg *lgraph) {
	g := simplify(lg)
	longestPath(g)
	t := feasibleTree(g)
	initLowLimValues(t)
	initCutValues(t, g)
	for {
		e, ok := leaveEdge(t)
		if !ok {
			break
		}
		f := enterEdge(t, g, e)
		t.RemoveEdge(e)
		t.SetEdge(graph.Edge{V: f.V, W: f.W}, 0)
		initLowLimValues(t)
		initCutValues(t, g)
		updateRanks(t, g)
	}
}

func initCutValues(t *tree, g *nsGraph) {
	vs := graph.Postorder(t, t.Nodes()...)
	for _, v := range vs[:len(vs)-1] {
		t.SetEdge(graph.Edge{V: v, W: t.Node(v).parent}, calcCutValue(t, g, v))
	}
}

// calcCutValue returns the cut value of the tree edge between child and its
// parent; the cut values of child's other tree edges must be known.
func calcCutValue(t *tree, g *nsGraph, child string) float64 {
	parent := t.Node(child).parent
	childIsTail := true
	ge, ok := g.EdgeOK(graph.Edge{V: child, W: parent})
	if !ok {
		childIsTail = false
		ge = g.Edge(graph.Edge{V: parent, W: child})
	}
	cut := ge.weight
	for _, e := range g.NodeEdges(child, "") {
		isOut := e.V == child
		other := e.V
		if isOut {
			other = e.W
		}
		if other == parent {
			continue
		}
		pointsToHead := isOut == childIsTail
		w := g.Edge(e).weight
		if pointsToHead {
			cut += w
		} else {
			cut -= w
		}
		if oc, ok := t.EdgeOK(graph.Edge{V: child, W: other}); ok {
			if pointsToHead {
				cut -= oc
			} else {
				cut += oc
			}
		}
	}
	return cut
}

func initLowLimValues(t *tree) {
	visited := make(map[string]bool, t.NodeCount())
	dfsAssignLowLim(t, visited, 1, t.Nodes()[0], "")
}

func dfsAssignLowLim(t *tree, visited map[string]bool, nextLim int, v, parent string) int {
	low := nextLim
	visited[v] = true
	for _, w := range t.Neighbors(v) {
		if !visited[w] {
			nextLim = dfsAssignLowLim(t, visited, nextLim, w, v)
		}
	}
	n := t.Node(v)
	n.low, n.lim, n.parent = low, nextLim, parent
	return nextLim + 1
}

func leaveEdge(t *tree) (graph.Edge, bool) {
	for _, e := range t.Edges() {
		if t.Edge(e) < 0 {
			return e, true
		}
	}
	return graph.Edge{}, false
}

func enterEdge(t *tree, g *nsGraph, e graph.Edge) graph.Edge {
	v, w := e.V, e.W
	// Orient the tree edge as the graph has it.
	if !g.HasEdge(graph.Edge{V: v, W: w}) {
		v, w = w, v
	}
	vl, wl := t.Node(v), t.Node(w)
	tail, flip := vl, false
	// If the root is on the tail side, flip the head/tail tests.
	if vl.lim > wl.lim {
		tail, flip = wl, true
	}
	var best graph.Edge
	bestSlack, found := 0, false
	for _, f := range g.Edges() {
		if flip == isDescendant(t.Node(f.V), tail) && flip != isDescendant(t.Node(f.W), tail) {
			if s := slack(g, f); !found || s < bestSlack {
				best, bestSlack, found = f, s, true
			}
		}
	}
	return best
}

func isDescendant(v, root *tnode) bool {
	return root.low <= v.lim && v.lim <= root.lim
}

// updateRanks recomputes ranks down the tree from its first node.
func updateRanks(t *tree, g *nsGraph) {
	vs := graph.Preorder(t, t.Nodes()[0])
	for _, v := range vs[1:] {
		parent := t.Node(v).parent
		if e, ok := g.EdgeOK(graph.Edge{V: v, W: parent}); ok {
			g.Node(v).rank = g.Node(parent).rank - e.minLen
		} else {
			e := g.Edge(graph.Edge{V: parent, W: v})
			g.Node(v).rank = g.Node(parent).rank + e.minLen
		}
	}
}

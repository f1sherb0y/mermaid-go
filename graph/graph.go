// Package graph is a port of graphlib (as bundled in dagre-d3-es 7.0.14): a
// directed or undirected multigraph with optional compound (parent/child)
// structure, node labels of type N and edge labels of type E.
//
// Everything that lists nodes or edges returns them in the same order as
// graphlib does (see keys.go): dagre's layouts depend on it.
//
// Node ids are strings; the empty string stands for graphlib's undefined
// (no parent, the root of the compound hierarchy) and cannot be a node id
// usable as a parent.
package graph

// Edge identifies an edge by its tail V, head W and, in a multigraph, its
// name. In an undirected graph V and W are stored in JS string order.
type Edge struct{ V, W, Name string }

// Options selects the kind of graph. The zero value is a simple directed
// graph, graphlib's default.
type Options struct {
	Undirected bool
	Multigraph bool
	Compound   bool
}

// Graph is graphlib's Graph.
type Graph[N, E any] struct {
	opt       Options
	nodes     map[string]*node[N, E]
	order     nodeOrder[N, E]
	edges     map[Edge]*edge[N, E]
	edgeList  []*edge[N, E] // insertion order; removed edges are nil
	deadEdges int
	root      keySet[N, E] // children of the root (compound graphs)

	defaultNode func(v string) N
	defaultEdge func(e Edge) E
}

type node[N, E any] struct {
	id       string
	idx      int64 // arrayIndex(id)
	seq      int   // position in nodeOrder.strs
	label    N
	in, out  []*edge[N, E] // insertion order (edge ids are never array indices)
	preds    keySet[N, E]
	sucs     keySet[N, E]
	parent   *node[N, E] // nil: the root
	children keySet[N, E]
}

type edge[N, E any] struct {
	key   Edge
	label E
	seq   int
	v, w  *node[N, E]
}

// New returns an empty graph.
func New[N, E any](opt Options) *Graph[N, E] {
	return &Graph[N, E]{
		opt:   opt,
		nodes: make(map[string]*node[N, E]),
		edges: make(map[Edge]*edge[N, E]),
	}
}

func (g *Graph[N, E]) IsDirected() bool   { return !g.opt.Undirected }
func (g *Graph[N, E]) IsMultigraph() bool { return g.opt.Multigraph }
func (g *Graph[N, E]) IsCompound() bool   { return g.opt.Compound }

// SetDefaultNodeLabel sets the label given to nodes created implicitly (by
// EnsureNode, SetParent or SetEdge). Without it they get the zero N.
func (g *Graph[N, E]) SetDefaultNodeLabel(fn func(v string) N) { g.defaultNode = fn }

// SetDefaultEdgeLabel sets the label EnsureEdge gives new edges.
func (g *Graph[N, E]) SetDefaultEdgeLabel(fn func(e Edge) E) { g.defaultEdge = fn }

// --- Nodes ---

func (g *Graph[N, E]) NodeCount() int { return len(g.nodes) }

// Nodes returns the node ids.
func (g *Graph[N, E]) Nodes() []string {
	out := make([]string, 0, len(g.nodes))
	g.order.each(func(n *node[N, E]) { out = append(out, n.id) })
	return out
}

// EachNode calls fn for every node in order. fn must not add or remove nodes.
func (g *Graph[N, E]) EachNode(fn func(v string, label N)) {
	g.order.each(func(n *node[N, E]) { fn(n.id, n.label) })
}

// Sources returns the nodes without in-edges.
func (g *Graph[N, E]) Sources() []string {
	var out []string
	g.order.each(func(n *node[N, E]) {
		if len(n.in) == 0 {
			out = append(out, n.id)
		}
	})
	return out
}

// Sinks returns the nodes without out-edges.
func (g *Graph[N, E]) Sinks() []string {
	var out []string
	g.order.each(func(n *node[N, E]) {
		if len(n.out) == 0 {
			out = append(out, n.id)
		}
	})
	return out
}

// SetNode adds v or replaces its label.
func (g *Graph[N, E]) SetNode(v string, label N) {
	if n := g.nodes[v]; n != nil {
		n.label = label
		return
	}
	g.addNode(v, label)
}

// EnsureNode adds v with the default label if it is missing.
func (g *Graph[N, E]) EnsureNode(v string) {
	g.ensure(v)
}

func (g *Graph[N, E]) ensure(v string) *node[N, E] {
	if n := g.nodes[v]; n != nil {
		return n
	}
	var label N
	if g.defaultNode != nil {
		label = g.defaultNode(v)
	}
	return g.addNode(v, label)
}

func (g *Graph[N, E]) addNode(v string, label N) *node[N, E] {
	n := &node[N, E]{id: v, idx: arrayIndex(v), label: label}
	g.nodes[v] = n
	g.order.add(n)
	if g.opt.Compound {
		g.root.add(n)
	}
	return n
}

// Node returns v's label (the zero N if v is missing).
func (g *Graph[N, E]) Node(v string) N {
	if n := g.nodes[v]; n != nil {
		return n.label
	}
	var zero N
	return zero
}

func (g *Graph[N, E]) HasNode(v string) bool { return g.nodes[v] != nil }

// RemoveNode removes v and its edges; v's children move to the root.
func (g *Graph[N, E]) RemoveNode(v string) {
	n := g.nodes[v]
	if n == nil {
		return
	}
	delete(g.nodes, v)
	g.order.remove(n)
	if g.opt.Compound {
		g.childSet(n.parent).remove(n)
		for _, c := range n.children.items {
			c.n.parent = nil
			g.root.add(c.n)
		}
	}
	// v's own adjacency goes away with it; only the other ends are updated.
	for _, e := range n.in {
		g.dropEdge(e, n)
	}
	for _, e := range n.out {
		g.dropEdge(e, n)
	}
}

// SetParent makes parent ("" for the root) the parent of v, adding either if
// missing. It panics if that would create a cycle or the graph is not
// compound.
func (g *Graph[N, E]) SetParent(v, parent string) {
	if !g.opt.Compound {
		panic("graph: cannot set parent in a non-compound graph")
	}
	var p *node[N, E]
	if parent != "" {
		for a := parent; a != ""; a = g.Parent(a) {
			if a == v {
				panic("graph: setting " + parent + " as parent of " + v + " would create a cycle")
			}
		}
		p = g.ensure(parent)
	}
	n := g.ensure(v)
	g.childSet(n.parent).remove(n)
	n.parent = p
	g.childSet(p).add(n)
}

func (g *Graph[N, E]) childSet(p *node[N, E]) *keySet[N, E] {
	if p == nil {
		return &g.root
	}
	return &p.children
}

// Parent returns v's parent, "" if none.
func (g *Graph[N, E]) Parent(v string) string {
	if n := g.nodes[v]; n != nil && n.parent != nil {
		return n.parent.id
	}
	return ""
}

// Children returns v's children; v == "" lists the root's. In a
// non-compound graph the root's children are all nodes.
func (g *Graph[N, E]) Children(v string) []string {
	if !g.opt.Compound {
		if v == "" {
			return g.Nodes()
		}
		if g.HasNode(v) {
			return []string{}
		}
		return nil
	}
	if v == "" {
		return g.root.ids()
	}
	if n := g.nodes[v]; n != nil {
		return n.children.ids()
	}
	return nil
}

// HasChildren reports whether v has children (v == "": whether the graph has
// top-level nodes).
func (g *Graph[N, E]) HasChildren(v string) bool {
	if !g.opt.Compound {
		return v == "" && len(g.nodes) > 0
	}
	if v == "" {
		return len(g.root.items) > 0
	}
	n := g.nodes[v]
	return n != nil && len(n.children.items) > 0
}

func (g *Graph[N, E]) Predecessors(v string) []string {
	if n := g.nodes[v]; n != nil {
		return n.preds.ids()
	}
	return nil
}

func (g *Graph[N, E]) Successors(v string) []string {
	if n := g.nodes[v]; n != nil {
		return n.sucs.ids()
	}
	return nil
}

// Neighbors returns the predecessors then the successors not already listed.
func (g *Graph[N, E]) Neighbors(v string) []string {
	n := g.nodes[v]
	if n == nil {
		return nil
	}
	out := n.preds.ids()
	for _, s := range n.sucs.items {
		if n.preds.find(s.n) < 0 {
			out = append(out, s.n.id)
		}
	}
	return out
}

func (g *Graph[N, E]) IsLeaf(v string) bool {
	if g.IsDirected() {
		return len(g.Successors(v)) == 0
	}
	return len(g.Neighbors(v)) == 0
}

// FilterNodes returns a copy holding the nodes for which keep returns true,
// the edges between them and, in a compound graph, each node's nearest kept
// ancestor as parent. Labels are shared.
func (g *Graph[N, E]) FilterNodes(keep func(v string) bool) *Graph[N, E] {
	c := New[N, E](g.opt)
	c.defaultNode, c.defaultEdge = g.defaultNode, g.defaultEdge
	g.order.each(func(n *node[N, E]) {
		if keep(n.id) {
			c.SetNode(n.id, n.label)
		}
	})
	for _, e := range g.edgeList {
		if e != nil && c.HasNode(e.key.V) && c.HasNode(e.key.W) {
			c.SetEdge(e.key, e.label)
		}
	}
	if g.opt.Compound {
		parents := map[string]string{}
		var find func(v string) string
		find = func(v string) string {
			p := g.Parent(v)
			if p == "" || c.HasNode(p) {
				parents[v] = p
				return p
			}
			if q, ok := parents[p]; ok {
				return q
			}
			return find(p)
		}
		for _, v := range c.Nodes() {
			c.SetParent(v, find(v))
		}
	}
	return c
}

// --- Edges ---

func (g *Graph[N, E]) EdgeCount() int { return len(g.edges) }

// Edges returns all edges.
func (g *Graph[N, E]) Edges() []Edge {
	out := make([]Edge, 0, len(g.edges))
	for _, e := range g.edgeList {
		if e != nil {
			out = append(out, e.key)
		}
	}
	return out
}

func (g *Graph[N, E]) key(e Edge) Edge {
	if g.opt.Undirected && Less(e.W, e.V) {
		e.V, e.W = e.W, e.V
	}
	return e
}

// SetEdge adds e (and its nodes if missing) or replaces its label. It panics
// on a named edge in a graph that is not a multigraph.
func (g *Graph[N, E]) SetEdge(e Edge, label E) {
	if r := g.edges[g.key(e)]; r != nil {
		r.label = label
		return
	}
	g.addEdge(e, label)
}

// EnsureEdge adds e with the default label if it is missing.
func (g *Graph[N, E]) EnsureEdge(e Edge) {
	if g.edges[g.key(e)] != nil {
		return
	}
	var label E
	if g.defaultEdge != nil {
		label = g.defaultEdge(e)
	}
	g.addEdge(e, label)
}

// addEdge adds e, creating its nodes in argument order before an
// undirected edge's ends are put in string order.
func (g *Graph[N, E]) addEdge(e Edge, label E) {
	if e.Name != "" && !g.opt.Multigraph {
		panic("graph: cannot set a named edge when the graph is not a multigraph")
	}
	v, w := g.ensure(e.V), g.ensure(e.W)
	if k := g.key(e); k != e {
		e, v, w = k, w, v
	}
	r := &edge[N, E]{key: e, label: label, seq: len(g.edgeList), v: v, w: w}
	g.edges[e] = r
	g.edgeList = append(g.edgeList, r)
	w.preds.inc(v)
	v.sucs.inc(w)
	w.in = append(w.in, r)
	v.out = append(v.out, r)
}

// Edge returns e's label (the zero E if missing).
func (g *Graph[N, E]) Edge(e Edge) E {
	if r := g.edges[g.key(e)]; r != nil {
		return r.label
	}
	var zero E
	return zero
}

// EdgeOK returns e's label and whether e exists.
func (g *Graph[N, E]) EdgeOK(e Edge) (E, bool) {
	if r := g.edges[g.key(e)]; r != nil {
		return r.label, true
	}
	var zero E
	return zero, false
}

func (g *Graph[N, E]) HasEdge(e Edge) bool { return g.edges[g.key(e)] != nil }

func (g *Graph[N, E]) RemoveEdge(e Edge) {
	if r := g.edges[g.key(e)]; r != nil {
		g.dropEdge(r, nil)
	}
}

// dropEdge removes r, leaving the adjacency of skip (a node being removed)
// alone.
func (g *Graph[N, E]) dropEdge(r *edge[N, E], skip *node[N, E]) {
	if g.edges[r.key] != r {
		return // a self-loop met twice while removing its node
	}
	delete(g.edges, r.key)
	g.edgeList[r.seq] = nil
	g.deadEdges++
	if g.deadEdges > 32 && g.deadEdges > len(g.edgeList)/2 {
		k := 0
		for _, x := range g.edgeList {
			if x != nil {
				x.seq = k
				g.edgeList[k] = x
				k++
			}
		}
		clear(g.edgeList[k:])
		g.edgeList = g.edgeList[:k]
		g.deadEdges = 0
	}
	v, w := r.v, r.w
	if w != skip {
		w.preds.dec(v)
		w.in = removeEdgeRec(w.in, r)
	}
	if v != skip {
		v.sucs.dec(w)
		v.out = removeEdgeRec(v.out, r)
	}
}

func removeEdgeRec[N, E any](list []*edge[N, E], r *edge[N, E]) []*edge[N, E] {
	for i := len(list) - 1; i >= 0; i-- {
		if list[i] == r {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// InEdges returns the edges into v, only those from u if u != "".
func (g *Graph[N, E]) InEdges(v, u string) []Edge {
	n := g.nodes[v]
	if n == nil {
		return nil
	}
	out := make([]Edge, 0, len(n.in))
	for _, e := range n.in {
		if u == "" || e.key.V == u {
			out = append(out, e.key)
		}
	}
	return out
}

// OutEdges returns the edges out of v, only those to w if w != "".
func (g *Graph[N, E]) OutEdges(v, w string) []Edge {
	n := g.nodes[v]
	if n == nil {
		return nil
	}
	out := make([]Edge, 0, len(n.out))
	for _, e := range n.out {
		if w == "" || e.key.W == w {
			out = append(out, e.key)
		}
	}
	return out
}

// NodeEdges returns InEdges(v, w) followed by OutEdges(v, w).
func (g *Graph[N, E]) NodeEdges(v, w string) []Edge {
	n := g.nodes[v]
	if n == nil {
		return nil
	}
	return append(g.InEdges(v, w), g.OutEdges(v, w)...)
}

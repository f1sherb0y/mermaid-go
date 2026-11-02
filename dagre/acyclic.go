package dagre

import "github.com/f1sherb0y/mermaid-go/graph"

// acyclic reverses a feedback arc set so the graph has no cycles. Reversed
// edges get the name "rev<n>" and remember their own.
func (l *layout) acyclic() {
	var fas []graph.Edge
	if l.opt.Acyclicer == "greedy" {
		fas = greedyFAS(l.g)
	} else {
		fas = dfsFAS(l.g)
	}
	for _, e := range fas {
		label := l.g.Edge(e)
		l.g.RemoveEdge(e)
		label.forwardName = e.Name
		label.reversed = true
		l.g.SetEdge(graph.Edge{V: e.W, W: e.V, Name: l.uniqueID("rev")}, label)
	}
}

// dfsFAS returns the edges that point back into the depth-first search
// stack, visiting nodes and out-edges in graph order.
func dfsFAS(g *lgraph) []graph.Edge {
	var fas []graph.Edge
	visited := map[string]bool{}
	stack := map[string]bool{}
	var dfs func(v string)
	dfs = func(v string) {
		if visited[v] {
			return
		}
		visited[v] = true
		stack[v] = true
		for _, e := range g.OutEdges(v, "") {
			if stack[e.W] {
				fas = append(fas, e)
			} else {
				dfs(e.W)
			}
		}
		delete(stack, v)
	}
	for _, v := range g.Nodes() {
		dfs(v)
	}
	return fas
}

func (l *layout) acyclicUndo() {
	for _, e := range l.g.Edges() {
		label := l.g.Edge(e)
		if label.reversed {
			l.g.RemoveEdge(e)
			label.reversed = false
			name := label.forwardName
			label.forwardName = ""
			l.g.SetEdge(graph.Edge{V: e.W, W: e.V, Name: name}, label)
		}
	}
}

// greedyFAS is the Eades-Lin-Smyth heuristic for a feedback arc set, with
// weighted edges. Weights must be whole numbers (dagre indexes buckets by
// them).
func greedyFAS(g *lgraph) []graph.Edge {
	if g.NodeCount() <= 1 {
		return nil
	}
	fg := graph.New[*fasEntry, float64](graph.Options{})
	maxIn, maxOut := 0.0, 0.0
	for _, v := range g.Nodes() {
		fg.SetNode(v, &fasEntry{v: v})
	}
	// Sum multi-edge weights into one edge.
	for _, e := range g.Edges() {
		k := graph.Edge{V: e.V, W: e.W}
		w := g.Edge(e).weight
		fg.SetEdge(k, fg.Edge(k)+w)
		vn, wn := fg.Node(e.V), fg.Node(e.W)
		vn.out += w
		wn.in += w
		maxOut = max(maxOut, vn.out)
		maxIn = max(maxIn, wn.in)
	}
	f := &fas{g: fg, zero: int(maxIn) + 1}
	f.buckets = make([]fasList, int(maxOut+maxIn)+3)
	for i := range f.buckets {
		s := &f.buckets[i].sentinel
		s.prev, s.next = s, s
	}
	fg.EachNode(func(_ string, n *fasEntry) { f.assign(n) })

	var results []graph.Edge
	sources, sinks := &f.buckets[len(f.buckets)-1], &f.buckets[0]
	for fg.NodeCount() > 0 {
		for n := sinks.dequeue(); n != nil; n = sinks.dequeue() {
			f.remove(n, nil)
		}
		for n := sources.dequeue(); n != nil; n = sources.dequeue() {
			f.remove(n, nil)
		}
		if fg.NodeCount() > 0 {
			for i := len(f.buckets) - 2; i > 0; i-- {
				if n := f.buckets[i].dequeue(); n != nil {
					f.remove(n, &results)
					break
				}
			}
		}
	}
	// Expand back to the multi-edges.
	var out []graph.Edge
	for _, e := range results {
		out = append(out, g.OutEdges(e.V, e.W)...)
	}
	return out
}

type fasEntry struct {
	v          string
	in, out    float64
	prev, next *fasEntry
}

type fas struct {
	g       *graph.Graph[*fasEntry, float64]
	buckets []fasList
	zero    int
}

// fasList is dagre's doubly linked list: enqueue at the front, dequeue at
// the back.
type fasList struct{ sentinel fasEntry }

func (q *fasList) dequeue() *fasEntry {
	s := &q.sentinel
	e := s.prev
	if e == s {
		return nil
	}
	unlink(e)
	return e
}

func (q *fasList) enqueue(e *fasEntry) {
	s := &q.sentinel
	if e.prev != nil && e.next != nil {
		unlink(e)
	}
	e.next = s.next
	s.next.prev = e
	s.next = e
	e.prev = s
}

func unlink(e *fasEntry) {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.prev, e.next = nil, nil
}

func (f *fas) assign(n *fasEntry) {
	switch {
	case n.out == 0:
		f.buckets[0].enqueue(n)
	case n.in == 0:
		f.buckets[len(f.buckets)-1].enqueue(n)
	default:
		f.buckets[int(n.out-n.in)+f.zero].enqueue(n)
	}
}

// remove takes n out of the graph, collecting its in-edges if results is
// not nil.
func (f *fas) remove(n *fasEntry, results *[]graph.Edge) {
	for _, e := range f.g.InEdges(n.v, "") {
		u := f.g.Node(e.V)
		if results != nil {
			*results = append(*results, graph.Edge{V: e.V, W: e.W})
		}
		u.out -= f.g.Edge(e)
		f.assign(u)
	}
	for _, e := range f.g.OutEdges(n.v, "") {
		w := f.g.Node(e.W)
		w.in -= f.g.Edge(e)
		f.assign(w)
	}
	f.g.RemoveNode(n.v)
}

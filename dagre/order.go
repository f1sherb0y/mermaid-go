package dagre

import (
	"math"
	"slices"

	"github.com/f1sherb0y/mermaid-go/graph"
)

// order sets each node's order within its rank to reduce crossings:
// a DFS-based initial order, then sweeps down and up with barycenters
// until four sweeps in a row bring no improvement.
func (l *layout) order() error {
	g := l.g
	mr := maxRank(g)
	var down, up []*layerGraph
	for r := 1; r <= mr; r++ {
		down = append(down, l.buildLayerGraph(r, true))
	}
	for r := mr - 1; r >= 0; r-- {
		up = append(up, l.buildLayerGraph(r, false))
	}

	assignOrder(g, initOrder(g))

	bestCC := math.Inf(1)
	var best [][]string
	for i, lastBest := 0, 0; lastBest < 4; i, lastBest = i+1, lastBest+1 {
		lgs := up
		if i%2 == 1 {
			lgs = down
		}
		sweepLayerGraphs(lgs, i%4 >= 2)
		layering := buildLayerMatrix(g)
		if cc := crossCount(g, layering); cc < bestCC {
			lastBest = 0
			best = layering
			bestCC = cc
		}
	}
	if hasHoles(best) {
		return errSameOrder
	}
	assignOrder(g, best)
	return nil
}

func assignOrder(g *lgraph, layering [][]string) {
	for _, layer := range layering {
		for i, v := range layer {
			g.Node(v).order = i
		}
	}
}

// layerGraph holds one rank's nodes with their cluster hierarchy under a
// root, plus the nodes they connect to on the neighbouring rank. Edges run
// from the neighbour to the rank's node, with summed weights. Node labels
// are the layout graph's, so orders written here are the layout's.
type layerGraph struct {
	g    *graph.Graph[*node, float64]
	root string
	rank int
}

func (l *layout) buildLayerGraph(rank int, inEdges bool) *layerGraph {
	g := l.g
	var root string
	for root = l.uniqueID("_root"); g.HasNode(root); root = l.uniqueID("_root") {
	}
	lg := graph.New[*node, float64](graph.Options{Compound: true})
	lg.SetDefaultNodeLabel(g.Node)
	for _, v := range g.Nodes() {
		n := g.Node(v)
		if n.ranked() && n.rank != rank || !n.ranked() && (rank < n.minRank || rank > n.maxRank) {
			continue
		}
		lg.EnsureNode(v)
		parent := g.Parent(v)
		if parent == "" {
			parent = root
		}
		lg.SetParent(v, parent)
		var edges []graph.Edge
		if inEdges {
			edges = g.InEdges(v, "")
		} else {
			edges = g.OutEdges(v, "")
		}
		for _, e := range edges {
			u := e.V
			if u == v {
				u = e.W
			}
			k := graph.Edge{V: u, W: v}
			lg.SetEdge(k, g.Edge(e).weight+lg.Edge(k))
		}
	}
	return &layerGraph{lg, root, rank}
}

// initOrder orders each rank by a DFS from the nodes sorted by rank.
func initOrder(g *lgraph) [][]string {
	var simple []string
	mr := -1
	g.EachNode(func(v string, n *node) {
		if !g.HasChildren(v) {
			simple = append(simple, v)
			mr = max(mr, n.rank)
		}
	})
	layers := make([][]string, mr+1)
	visited := map[string]bool{}
	var dfs func(v string)
	dfs = func(v string) {
		if visited[v] {
			return
		}
		visited[v] = true
		n := g.Node(v)
		layers[n.rank] = append(layers[n.rank], v)
		for _, w := range g.Successors(v) {
			dfs(w)
		}
	}
	slices.SortStableFunc(simple, func(a, b string) int { return g.Node(a).rank - g.Node(b).rank })
	for _, v := range simple {
		dfs(v)
	}
	return layers
}

func sweepLayerGraphs(lgs []*layerGraph, biasRight bool) {
	cg := graph.New[struct{}, struct{}](graph.Options{})
	for _, lg := range lgs {
		sorted := lg.sortSubgraph(lg.root, cg, biasRight)
		for i, v := range sorted.vs {
			lg.g.Node(v).order = i
		}
		addSubgraphConstraints(lg.g, cg, sorted.vs)
	}
}

// addSubgraphConstraints records, for consecutive nodes in different
// clusters under the same parent, that the first cluster goes left of the
// second.
func addSubgraphConstraints(g *graph.Graph[*node, float64], cg *graph.Graph[struct{}, struct{}], vs []string) {
	prev := map[string]string{}
	rootPrev := ""
	for _, v := range vs {
		for child := g.Parent(v); child != ""; {
			parent := g.Parent(child)
			var prevChild string
			if parent != "" {
				prevChild = prev[parent]
				prev[parent] = child
			} else {
				prevChild, rootPrev = rootPrev, child
			}
			if prevChild != "" && prevChild != child {
				cg.SetEdge(graph.Edge{V: prevChild, W: child}, struct{}{})
				break
			}
			child = parent
		}
	}
}

type sortResult struct {
	vs         []string
	bc, weight float64
	hasBC      bool
}

// sortSubgraph orders v's children (v is the root or a cluster),
// recursing into clusters, which are kept contiguous and between their
// border nodes.
func (lg *layerGraph) sortSubgraph(v string, cg *graph.Graph[struct{}, struct{}], biasRight bool) sortResult {
	g := lg.g
	movable := g.Children(v)
	var bl, br string
	if v != lg.root {
		n := g.Node(v)
		bl, br = n.borderLeft[lg.rank], n.borderRight[lg.rank]
		movable = slices.DeleteFunc(movable, func(w string) bool { return w == bl || w == br })
	}

	bcs := barycenters(g, movable)
	subgraphs := map[string]sortResult{}
	for i := range bcs {
		e := &bcs[i]
		if !g.HasChildren(e.v) {
			continue
		}
		r := lg.sortSubgraph(e.v, cg, biasRight)
		subgraphs[e.v] = r
		if r.hasBC {
			if e.hasBC {
				e.bc = (e.bc*e.weight + r.bc*r.weight) / (e.weight + r.weight)
				e.weight += r.weight
			} else {
				e.bc, e.weight, e.hasBC = r.bc, r.weight, true
			}
		}
	}

	entries := resolveConflicts(bcs, cg)
	for _, e := range entries {
		var vs []string
		for _, w := range e.vs {
			if s, ok := subgraphs[w]; ok {
				vs = append(vs, s.vs...)
			} else {
				vs = append(vs, w)
			}
		}
		e.vs = vs
	}

	result := sortEntries(entries, biasRight)
	if bl != "" {
		vs := make([]string, 0, len(result.vs)+2)
		vs = append(vs, bl)
		vs = append(vs, result.vs...)
		result.vs = append(vs, br)
		if preds := g.Predecessors(bl); len(preds) > 0 {
			blPred := g.Node(preds[0])
			brPred := g.Node(g.Predecessors(br)[0])
			if !result.hasBC {
				result.bc, result.weight, result.hasBC = 0, 0, true
			}
			result.bc = (result.bc*result.weight + float64(blPred.order) + float64(brPred.order)) / (result.weight + 2)
			result.weight += 2
		}
	}
	return result
}

type bcEntry struct {
	v          string
	bc, weight float64 // weight may be 0, making bc NaN, as in dagre
	hasBC      bool
}

func barycenters(g *graph.Graph[*node, float64], movable []string) []bcEntry {
	out := make([]bcEntry, len(movable))
	for i, v := range movable {
		out[i].v = v
		in := g.InEdges(v, "")
		if len(in) == 0 {
			continue
		}
		var sum, weight float64
		for _, e := range in {
			w := g.Edge(e)
			sum += w * float64(g.Node(e.V).order)
			weight += w
		}
		out[i].bc, out[i].weight, out[i].hasBC = sum/weight, weight, true
	}
	return out
}

type rcEntry struct {
	vs         []string
	i          int
	bc, weight float64
	hasBC      bool
	indegree   int
	in, out    []*rcEntry
	merged     bool
}

// resolveConflicts (after Forster, "A Fast and Simple Heuristic for
// Constrained Two-Level Crossing Reduction") merges entries whose
// barycenters contradict the constraint graph.
func resolveConflicts(entries []bcEntry, cg *graph.Graph[struct{}, struct{}]) []*rcEntry {
	byV := make(map[string]*rcEntry, len(entries))
	list := make([]*rcEntry, len(entries))
	for i, e := range entries {
		r := &rcEntry{vs: []string{e.v}, i: i}
		if e.hasBC {
			r.bc, r.weight, r.hasBC = e.bc, e.weight, true
		}
		byV[e.v] = r
		list[i] = r
	}
	for _, e := range cg.Edges() {
		ev, ew := byV[e.V], byV[e.W]
		if ev != nil && ew != nil {
			ew.indegree++
			ev.out = append(ev.out, ew)
		}
	}
	// dagre lists sources in object-key order of the entries' node ids; the
	// entries come from Children(), which is in that order already.
	var sources []*rcEntry
	for _, r := range list {
		if r.indegree == 0 {
			sources = append(sources, r)
		}
	}

	var done []*rcEntry
	for len(sources) > 0 {
		e := sources[len(sources)-1]
		sources = sources[:len(sources)-1]
		done = append(done, e)
		for i := len(e.in) - 1; i >= 0; i-- {
			u := e.in[i]
			if !u.merged && (!u.hasBC || !e.hasBC || u.bc >= e.bc) {
				mergeEntries(e, u)
			}
		}
		for _, w := range e.out {
			w.in = append(w.in, e)
			if w.indegree--; w.indegree == 0 {
				sources = append(sources, w)
			}
		}
	}
	return slices.DeleteFunc(done, func(e *rcEntry) bool { return e.merged })
}

func mergeEntries(target, source *rcEntry) {
	var sum, weight float64
	if truthy(target.weight) {
		sum += target.bc * target.weight
		weight += target.weight
	}
	if truthy(source.weight) {
		sum += source.bc * source.weight
		weight += source.weight
	}
	target.vs = append(slices.Clip(source.vs), target.vs...)
	target.bc, target.weight, target.hasBC = sum/weight, weight, true
	target.i = min(source.i, target.i)
	source.merged = true
}

// sortEntries orders entries with a barycenter by it (ties by index,
// reversed when biased right) and slots the others back at their index.
func sortEntries(entries []*rcEntry, biasRight bool) sortResult {
	var sortable, unsortable []*rcEntry
	for _, e := range entries {
		if e.hasBC {
			sortable = append(sortable, e)
		} else {
			unsortable = append(unsortable, e)
		}
	}
	slices.SortStableFunc(unsortable, func(a, b *rcEntry) int { return b.i - a.i })
	// NaN barycenters make this comparator inconsistent, so the result
	// depends on the sort algorithm: use V8's.
	jsSort(sortable, func(a, b *rcEntry) int {
		switch {
		case a.bc < b.bc:
			return -1
		case a.bc > b.bc:
			return 1
		case biasRight:
			return b.i - a.i
		default:
			return a.i - b.i
		}
	})

	var vs []string
	var sum, weight float64
	idx := 0
	consume := func() {
		for len(unsortable) > 0 && unsortable[len(unsortable)-1].i <= idx {
			last := unsortable[len(unsortable)-1]
			unsortable = unsortable[:len(unsortable)-1]
			vs = append(vs, last.vs...)
			idx++
		}
	}
	consume()
	for _, e := range sortable {
		idx += len(e.vs)
		vs = append(vs, e.vs...)
		sum += e.bc * e.weight
		weight += e.weight
		consume()
	}
	r := sortResult{vs: vs}
	if truthy(weight) {
		r.bc, r.weight, r.hasBC = sum/weight, weight, true
	}
	return r
}

// crossCount returns the weighted number of crossings between consecutive
// layers (Barth et al., "Bilayer Cross Counting").
func crossCount(g *lgraph, layering [][]string) float64 {
	cc := 0.0
	type entry struct {
		pos    int
		weight float64
	}
	var entries []entry
	var tree []float64
	southPos := map[string]int{}
	for i := 1; i < len(layering); i++ {
		north, south := layering[i-1], layering[i]
		clear(southPos)
		for j, v := range south {
			southPos[v] = j
		}
		entries = entries[:0]
		for _, v := range north {
			start := len(entries)
			for _, e := range g.OutEdges(v, "") {
				// An edge not reaching the south layer (minlen 0) sorts last
				// in dagre and adds nothing.
				if p, ok := southPos[e.W]; ok {
					entries = append(entries, entry{p, g.Edge(e).weight})
				}
			}
			slices.SortStableFunc(entries[start:], func(a, b entry) int { return a.pos - b.pos })
		}
		first := 1
		for first < len(south) {
			first <<= 1
		}
		tree = append(tree[:0], make([]float64, 2*first-1)...)
		first--
		layerCC := 0.0
		for _, e := range entries {
			idx := e.pos + first
			tree[idx] += e.weight
			sum := 0.0
			for idx > 0 {
				if idx%2 == 1 {
					sum += tree[idx+1]
				}
				idx = (idx - 1) >> 1
				tree[idx] += e.weight
			}
			layerCC += e.weight * sum
		}
		cc += layerCC
	}
	return cc
}

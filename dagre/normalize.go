package dagre

import "github.com/f1sherb0y/mermaid-go/graph"

// normalize splits every edge longer than one rank into a chain of dummy
// nodes, one per rank, the one on the label's rank sized like the label.
func (l *layout) normalize() {
	g := l.g
	l.dummyChains = nil
	for _, e := range g.Edges() {
		v, w := e.V, e.W
		vRank, wRank := g.Node(v).rank, g.Node(w).rank
		if wRank == vRank+1 {
			continue
		}
		label := g.Edge(e)
		g.RemoveEdge(e)
		if vRank+1 >= wRank {
			// A same-rank edge (minlen 0): dagre puts it back with a bare
			// label, which is all it keeps of it.
			g.SetEdge(e, &edge{weight: label.weight})
			continue
		}
		label.points = []Point{}
		for i, r := 0, vRank+1; r < wRank; i, r = i+1, r+1 {
			d := &node{edgeLabel: label, edgeObj: e, rank: r}
			kind := dummyEdge
			if label.hasLabelRank && r == label.labelRank {
				d.width, d.height = label.width, label.height
				d.labelPos = label.labelPos
				kind = dummyEdgeLabel
			}
			dummy := l.addDummyNode(kind, d, "_d")
			g.SetEdge(graph.Edge{V: v, W: dummy, Name: e.Name}, &edge{weight: label.weight})
			if i == 0 {
				l.dummyChains = append(l.dummyChains, dummy)
			}
			v = dummy
		}
		g.SetEdge(graph.Edge{V: v, W: w, Name: e.Name}, &edge{weight: label.weight})
	}
}

// denormalize puts the long edges back, their points taken from the dummy
// chain and their label position from the label dummy.
func (l *layout) denormalize() {
	g := l.g
	for _, v := range l.dummyChains {
		n := g.Node(v)
		label := n.edgeLabel
		g.SetEdge(n.edgeObj, label)
		for n.dummy != notDummy {
			w := g.Successors(v)[0]
			g.RemoveNode(v)
			label.points = append(label.points, Point{n.x, n.y})
			if n.dummy == dummyEdgeLabel {
				label.x, label.y, label.hasXY = n.x, n.y, true
				label.width, label.height = n.width, n.height
			}
			v = w
			n = g.Node(v)
		}
	}
}

// parentDummyChains moves each dummy into the cluster its rank falls in,
// along the path from the edge's tail up to the lowest common ancestor and
// down to its head.
func (l *layout) parentDummyChains() {
	g := l.g
	type lowLim struct{ low, lim int }
	post := map[string]lowLim{}
	lim := 0
	var walk func(v string)
	walk = func(v string) {
		low := lim
		for _, c := range g.Children(v) {
			walk(c)
		}
		post[v] = lowLim{low, lim}
		lim++
	}
	for _, v := range g.Children("") {
		walk(v)
	}

	for _, v := range l.dummyChains {
		edgeObj := g.Node(v).edgeObj
		// Path from the tail up to the LCA ("" is the root), then down.
		pv, pw := post[edgeObj.V], post[edgeObj.W]
		low, hi := min(pv.low, pw.low), max(pv.lim, pw.lim)
		var path []string
		p := edgeObj.V
		for {
			p = g.Parent(p)
			path = append(path, p)
			if pp := post[p]; p == "" || !(pp.low > low || hi > pp.lim) {
				break
			}
		}
		lca := p
		k := len(path)
		for p = g.Parent(edgeObj.W); p != lca; p = g.Parent(p) {
			path = append(path, p)
		}
		for i, j := k, len(path)-1; i < j; i, j = i+1, j-1 {
			path[i], path[j] = path[j], path[i]
		}

		idx := 0
		ascending := true
		for v != edgeObj.W {
			n := g.Node(v)
			pathV := ""
			if ascending {
				for pathV = path[idx]; pathV != lca && g.Node(pathV).maxRank < n.rank; pathV = path[idx] {
					idx++
				}
				if pathV == lca {
					ascending = false
				}
			}
			if !ascending {
				for idx < len(path)-1 && g.Node(path[idx+1]).minRank <= n.rank {
					idx++
				}
				pathV = path[idx]
			}
			g.SetParent(v, pathV)
			v = g.Successors(v)[0]
		}
	}
}

// addBorderSegments gives each cluster a left and right border node on
// every rank it spans, chained top to bottom.
func (l *layout) addBorderSegments() {
	g := l.g
	var dfs func(v string)
	dfs = func(v string) {
		for _, c := range g.Children(v) {
			dfs(c)
		}
		n := g.Node(v)
		if n.borderTop == "" {
			return
		}
		n.borderLeft = make([]string, n.maxRank+1)
		n.borderRight = make([]string, n.maxRank+1)
		for r := n.minRank; r <= n.maxRank; r++ {
			l.addBorderSegment(v, n, borderLeft, "_bl", r)
			l.addBorderSegment(v, n, borderRight, "_br", r)
		}
	}
	for _, v := range g.Children("") {
		dfs(v)
	}
}

func (l *layout) addBorderSegment(sg string, sgNode *node, kind borderKind, prefix string, rank int) {
	list := sgNode.borderLeft
	if kind == borderRight {
		list = sgNode.borderRight
	}
	curr := l.addDummyNode(dummyBorder, &node{rank: rank, borderType: kind}, prefix)
	list[rank] = curr
	l.g.SetParent(curr, sg)
	if rank > 0 && list[rank-1] != "" {
		l.g.SetEdge(graph.Edge{V: list[rank-1], W: curr}, &edge{weight: 1})
	}
}

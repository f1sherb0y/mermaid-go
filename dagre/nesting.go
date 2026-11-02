package dagre

import "github.com/f1sherb0y/mermaid-go/graph"

// nestingGraphRun (Sander, "Layout of Compound Directed Graphs") adds a top
// and bottom border node to every cluster, edges that keep the cluster's
// nodes between them, and a root linked to everything so the graph is
// connected. Node ranks are spread nodeRankFactor apart so border nodes get
// ranks of their own.
func (l *layout) nestingGraphRun() {
	g := l.g
	root := l.addDummyNode(dummyRoot, &node{}, "_root")
	depths := map[string]int{}
	maxDepth := 0
	var depth func(v string, d int)
	depth = func(v string, d int) {
		for _, c := range g.Children(v) {
			depth(c, d+1)
		}
		depths[v] = d
		maxDepth = max(maxDepth, d)
	}
	for _, v := range g.Children("") {
		depth(v, 1)
	}
	height := maxDepth - 1
	nodeSep := 2*height + 1
	l.nestingRoot = root

	// A weight big enough to keep clusters compact: all weights plus one
	// (summed from 0 first, as dagre does, for the same rounding).
	weight := 0.0
	for _, e := range g.Edges() {
		edge := g.Edge(e)
		edge.minLen *= nodeSep
		weight += edge.weight
	}
	weight++

	var dfs func(v string)
	dfs = func(v string) {
		children := g.Children(v)
		if len(children) == 0 {
			if v != root {
				g.SetEdge(graph.Edge{V: root, W: v}, &edge{weight: 0, minLen: nodeSep})
			}
			return
		}
		top := l.addBorderNode("_bt")
		bottom := l.addBorderNode("_bb")
		label := g.Node(v)
		g.SetParent(top, v)
		label.borderTop = top
		g.SetParent(bottom, v)
		label.borderBottom = bottom
		for _, c := range children {
			dfs(c)
			cn := g.Node(c)
			childTop, childBottom := c, c
			w := 2 * weight
			if cn.borderTop != "" {
				childTop, childBottom, w = cn.borderTop, cn.borderBottom, weight
			}
			minLen := height - depths[v] + 1
			if childTop != childBottom {
				minLen = 1
			}
			g.SetEdge(graph.Edge{V: top, W: childTop}, &edge{weight: w, minLen: minLen, nestingEdge: true})
			g.SetEdge(graph.Edge{V: childBottom, W: bottom}, &edge{weight: w, minLen: minLen, nestingEdge: true})
		}
		if g.Parent(v) == "" {
			g.SetEdge(graph.Edge{V: root, W: top}, &edge{weight: 0, minLen: height + depths[v]})
		}
	}
	for _, c := range g.Children("") {
		dfs(c)
	}
	l.nodeRankFactor = nodeSep
}

func (l *layout) nestingGraphCleanup() {
	l.g.RemoveNode(l.nestingRoot)
	l.nestingRoot = ""
	for _, e := range l.g.Edges() {
		if l.g.Edge(e).nestingEdge {
			l.g.RemoveEdge(e)
		}
	}
}

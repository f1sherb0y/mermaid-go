package dagre

import "strings"

// adjustCoordinateSystem swaps widths and heights for horizontal layouts so
// the rest can lay out top to bottom.
func (l *layout) adjustCoordinateSystem() {
	if d := strings.ToLower(l.opt.RankDir); d == "lr" || d == "rl" {
		l.swapWidthHeight()
	}
}

func (l *layout) undoCoordinateSystem() {
	d := strings.ToLower(l.opt.RankDir)
	if d == "bt" || d == "rl" {
		l.g.EachNode(func(_ string, n *node) { n.y = -n.y })
		for _, e := range l.g.Edges() {
			edge := l.g.Edge(e)
			for i := range edge.points {
				edge.points[i].Y = -edge.points[i].Y
			}
			if edge.hasXY {
				edge.y = -edge.y
			}
		}
	}
	if d == "lr" || d == "rl" {
		l.g.EachNode(func(_ string, n *node) { n.x, n.y = n.y, n.x })
		for _, e := range l.g.Edges() {
			edge := l.g.Edge(e)
			for i := range edge.points {
				p := &edge.points[i]
				p.X, p.Y = p.Y, p.X
			}
			if edge.hasXY {
				edge.x, edge.y = edge.y, edge.x
			}
		}
		l.swapWidthHeight()
	}
}

// swapWidthHeight swaps sizes of the nodes and edge labels in the graph at
// the time; labels of edges taken out of it (long edges, self-loops) are
// not touched, as in dagre.
func (l *layout) swapWidthHeight() {
	l.g.EachNode(func(_ string, n *node) { n.width, n.height = n.height, n.width })
	for _, e := range l.g.Edges() {
		edge := l.g.Edge(e)
		edge.width, edge.height = edge.height, edge.width
	}
}

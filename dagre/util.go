package dagre

import (
	"errors"
	"math"

	"github.com/f1sherb0y/mermaid-go/graph"
)

// asNonCompoundGraph copies g without the clusters, sharing labels. The copy
// matters: its adjacency lists are rebuilt from g's current edge order,
// which can differ from g's own (graphlib keeps a predecessor where it was
// first added for as long as any edge from it remains).
func asNonCompoundGraph(g *lgraph) *lgraph {
	s := graph.New[*node, *edge](graph.Options{Multigraph: g.IsMultigraph()})
	g.EachNode(func(v string, n *node) {
		if !g.HasChildren(v) {
			s.SetNode(v, n)
		}
	})
	for _, e := range g.Edges() {
		s.SetEdge(e, g.Edge(e))
	}
	return s
}

// nsEdge is the label of an edge in the simplified graph network simplex
// works on.
type nsEdge struct {
	weight float64
	minLen int
}

func (e *nsEdge) minlen() int { return e.minLen }

// simplify merges multi-edges: weights add up, the largest minlen wins.
func simplify(g *lgraph) *graph.Graph[*node, *nsEdge] {
	s := graph.New[*node, *nsEdge](graph.Options{})
	g.EachNode(func(v string, n *node) { s.SetNode(v, n) })
	for _, e := range g.Edges() {
		label := g.Edge(e)
		k := graph.Edge{V: e.V, W: e.W}
		if prev, ok := s.EdgeOK(k); ok {
			s.SetEdge(k, &nsEdge{prev.weight + label.weight, max(prev.minLen, label.minLen)})
		} else {
			s.SetEdge(k, &nsEdge{label.weight, max(1, label.minLen)})
		}
	}
	return s
}

// intersectRect returns where the line from p to the center of n's box
// crosses the box; false if p is the center.
func intersectRect(n *node, p Point) (Point, bool) {
	x, y := n.x, n.y
	dx, dy := p.X-x, p.Y-y
	w, h := n.width/2, n.height/2
	if !truthy(dx) && !truthy(dy) {
		return p, false
	}
	var sx, sy float64
	if math.Abs(dy)*w > math.Abs(dx)*h {
		if dy < 0 {
			h = -h
		}
		sx, sy = h*dx/dy, h
	} else {
		if dx < 0 {
			w = -w
		}
		sx, sy = w, w*dy/dx
	}
	return Point{x + sx, y + sy}, true
}

// errSameOrder is where dagre-d3-es throws "Cannot set properties of
// undefined (setting 'order')": on some compound graphs ordering gives two
// nodes of a rank the same order, leaving a hole in the layer matrix (or
// silently dropping one of them, which is not an error).
var errSameOrder = errors.New("dagre: ordering left a hole in a rank (dagre-d3-es throws here)")

func hasHoles(layering [][]string) bool {
	for _, layer := range layering {
		for _, v := range layer {
			if v == "" {
				return true
			}
		}
	}
	return false
}

// buildLayerMatrix lists each rank's nodes by order; a later node overwrites
// an earlier one with the same order, and a missing order leaves "".
func buildLayerMatrix(g *lgraph) [][]string {
	layers := make([][]string, maxRank(g)+1)
	g.EachNode(func(v string, n *node) {
		if !n.ranked() {
			return
		}
		layer := layers[n.rank]
		for len(layer) <= n.order {
			layer = append(layer, "")
		}
		layer[n.order] = v
		layers[n.rank] = layer
	})
	return layers
}

// maxRank returns the largest rank, -1 if no node has one.
func maxRank(g *lgraph) int {
	m := -1
	g.EachNode(func(_ string, n *node) {
		if n.ranked() && n.rank > m {
			m = n.rank
		}
	})
	return m
}

// minRank returns the smallest rank (0 if none).
func minRank(g *lgraph) int {
	m, any := 0, false
	g.EachNode(func(_ string, n *node) {
		if n.ranked() && (!any || n.rank < m) {
			m, any = n.rank, true
		}
	})
	return m
}

// normalizeRanks shifts ranks so the smallest is 0.
func normalizeRanks(g *lgraph) {
	m := minRank(g)
	g.EachNode(func(_ string, n *node) {
		if n.ranked() {
			n.rank -= m
		}
	})
}

// removeEmptyRanks drops empty ranks except every nodeRankFactor-th, which
// the nesting graph needs.
func (l *layout) removeEmptyRanks() {
	offset := minRank(l.g)
	var layers [][]*node
	l.g.EachNode(func(_ string, n *node) {
		// Clusters have no rank; in dagre a fractional rank indexes no layer.
		if !n.ranked() || n.halfRank {
			return
		}
		r := n.rank - offset
		for len(layers) <= r {
			layers = append(layers, nil)
		}
		layers[r] = append(layers[r], n)
	})
	delta := 0
	for i, vs := range layers {
		if vs == nil && i%l.nodeRankFactor != 0 {
			delta--
		} else if delta != 0 {
			for _, n := range vs {
				n.rank += delta
			}
		}
	}
}

// Package dagre is a port of dagre's layered graph layout as found in
// dagre-d3-es 7.0.14, the version mermaid 12.2.0 uses. For the same input it
// gives the same coordinates, bit for bit.
//
// Exactness rests on reproducing JS iteration order (the graph package
// mirrors graphlib's object-key order), lodash's helpers (stable sortBy,
// first-wins minBy, uniqueId names) and V8's Array.prototype.sort where the
// comparator can be inconsistent (jssort.go). Places where order decides
// the result carry a comment saying so.
package dagre

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/graph"
)

// Options are dagre's graph attributes. Zero values are taken as given; use
// DefaultOptions for dagre's defaults.
type Options struct {
	// RankDir is "TB", "BT", "LR" or "RL". dagre compares it case-blind
	// except when making room for edge labels, where only "TB" and "BT"
	// count as vertical; its default "tb" is therefore not quite "TB".
	RankDir string
	// Align, if set ("UL", "UR", "DL", "DR"), picks one of the four
	// Brandes-Köpf alignments instead of balancing them.
	Align                     string
	NodeSep, EdgeSep, RankSep float64
	MarginX, MarginY          float64
	// Ranker is "network-simplex" (the default), "tight-tree" or
	// "longest-path".
	Ranker string
	// Acyclicer "greedy" breaks cycles with the greedy feedback arc set
	// heuristic; anything else uses depth-first search.
	Acyclicer string
}

// DefaultOptions returns dagre's defaults.
func DefaultOptions() Options {
	return Options{RankDir: "tb", NodeSep: 50, EdgeSep: 20, RankSep: 50}
}

// Node is a node label: its size goes in; its center comes out. A cluster
// (a node with children) also gets its size.
type Node struct {
	Width, Height float64
	X, Y          float64
}

// Edge is an edge label. MinLen, Weight, Width, Height, LabelPos and
// LabelOffset go in (NewEdge has dagre's defaults); Points and, for an edge
// whose label got a place, X and Y come out.
type Edge struct {
	MinLen      int
	Weight      float64
	Width       float64
	Height      float64
	LabelPos    string // "l", "c" or "r"
	LabelOffset float64

	Points []Point
	X, Y   float64
	HasXY  bool // X and Y were set
}

// NewEdge returns an edge label with dagre's defaults.
func NewEdge() *Edge {
	return &Edge{MinLen: 1, Weight: 1, LabelPos: "r", LabelOffset: 10}
}

type Point struct{ X, Y float64 }

// Graph is the input of Layout.
type Graph = graph.Graph[*Node, *Edge]

// Layout lays g out, writing positions into its labels, and returns the size
// of the drawing. Nodes or edges with nil labels take part with zero size
// (edges with dagre's defaults) and get nothing back.
//
// dagre cannot rank a cluster that has edges of its own and throws; Layout
// returns an error instead. (mermaid moves such edges onto nodes inside the
// cluster before calling dagre.)
func Layout(g *Graph, opt Options) (width, height float64, err error) {
	return layoutFrom(g, opt, 0)
}

// layoutFrom lays out with lodash's uniqueId counter starting at ids, which
// names the dummy nodes. The JS counter is global to the page, so names
// differ between runs; the tests start where the JS run started.
func layoutFrom(in *Graph, opt Options, ids int) (width, height float64, err error) {
	if in.IsCompound() {
		for _, e := range in.Edges() {
			if in.HasChildren(e.V) || in.HasChildren(e.W) {
				return 0, 0, fmt.Errorf("dagre: edge %q -> %q touches a cluster", e.V, e.W)
			}
		}
	}
	l := &layout{opt: opt, ids: ids}
	l.buildLayoutGraph(in)
	if err := l.run(); err != nil {
		return 0, 0, err
	}
	l.updateInputGraph(in)
	return l.width, l.height, nil
}

type lgraph = graph.Graph[*node, *edge]

type dummyKind uint8

const (
	notDummy dummyKind = iota
	dummyEdge
	dummyEdgeLabel
	dummyEdgeProxy
	dummyBorder
	dummySelfEdge
	dummyRoot
)

type borderKind uint8

const (
	noBorder borderKind = iota
	borderLeft
	borderRight
)

// node is a layout node label. Which fields mean something depends on the
// kind of node; dagre keeps them all in one object too.
type node struct {
	width, height float64
	x, y          float64
	rank, order   int
	dummy         dummyKind

	// Edge dummies: the long edge they stand for and its label.
	edgeObj   graph.Edge
	edgeLabel *edge
	labelPos  string // edge-label dummies

	// Edge proxies: the edge and whether the label rank is fractional
	// (rank + 0.5), which in dagre leaves the label unplaced.
	e        graph.Edge
	halfRank bool

	// Clusters (nodes with children).
	borderTop, borderBottom string
	borderLeft, borderRight []string // by rank
	minRank, maxRank        int
	borderType              borderKind

	selfEdges []selfEdge
	selfLabel *edge // self-edge dummies; e is the edge
}

type selfEdge struct {
	e     graph.Edge
	label *edge
}

// ranked reports whether n has a rank: every node but the clusters.
func (n *node) ranked() bool { return n.borderTop == "" }

type edge struct {
	minLen      int
	weight      float64
	width       float64
	height      float64
	labelPos    string
	labelOffset float64

	points []Point
	x, y   float64
	hasXY  bool

	labelRank    int
	hasLabelRank bool
	reversed     bool
	forwardName  string
	nestingEdge  bool
}

func (e *edge) minlen() int { return e.minLen }

type layout struct {
	opt            Options
	g              *lgraph
	rankSep        float64
	ids            int
	nestingRoot    string
	nodeRankFactor int
	dummyChains    []string
	width, height  float64
}

// uniqueID is lodash's _.uniqueId: one counter shared by all prefixes.
func (l *layout) uniqueID(prefix string) string {
	l.ids++
	return prefix + strconv.Itoa(l.ids)
}

// addDummyNode adds a node named prefix plus a fresh number.
func (l *layout) addDummyNode(kind dummyKind, n *node, prefix string) string {
	var v string
	for v = l.uniqueID(prefix); l.g.HasNode(v); v = l.uniqueID(prefix) {
	}
	n.dummy = kind
	l.g.SetNode(v, n)
	return v
}

func (l *layout) addBorderNode(prefix string) string {
	return l.addDummyNode(dummyBorder, &node{}, prefix)
}

// run is dagre's runLayout. The step names are dagre's; traceStep, if set
// (by a test), sees the graph after each.
func (l *layout) run() error {
	ok := func(f func()) func() error { return func() error { f(); return nil } }
	steps := []struct {
		name string
		fn   func() error
	}{
		{"makeSpaceForEdgeLabels", ok(l.makeSpaceForEdgeLabels)},
		{"removeSelfEdges", ok(l.removeSelfEdges)},
		{"acyclic", ok(l.acyclic)},
		{"nestingGraph.run", ok(l.nestingGraphRun)},
		{"rank", ok(func() { l.rank(asNonCompoundGraph(l.g)) })},
		{"injectEdgeLabelProxies", ok(l.injectEdgeLabelProxies)},
		{"removeEmptyRanks", ok(l.removeEmptyRanks)},
		{"nestingGraph.cleanup", ok(l.nestingGraphCleanup)},
		{"normalizeRanks", ok(func() { normalizeRanks(l.g) })},
		{"assignRankMinMax", ok(l.assignRankMinMax)},
		{"removeEdgeLabelProxies", ok(l.removeEdgeLabelProxies)},
		{"normalize.run", ok(l.normalize)},
		{"parentDummyChains", ok(l.parentDummyChains)},
		{"addBorderSegments", ok(l.addBorderSegments)},
		{"order", l.order},
		{"insertSelfEdges", l.insertSelfEdges},
		{"adjustCoordinateSystem", ok(l.adjustCoordinateSystem)},
		{"position", l.position},
		{"positionSelfEdges", ok(l.positionSelfEdges)},
		{"removeBorderNodes", ok(l.removeBorderNodes)},
		{"normalize.undo", ok(l.denormalize)},
		{"fixupEdgeLabelCoords", ok(l.fixupEdgeLabelCoords)},
		{"undoCoordinateSystem", ok(l.undoCoordinateSystem)},
		{"translateGraph", ok(l.translateGraph)},
		{"assignNodeIntersects", l.assignNodeIntersects},
		{"reversePoints", ok(l.reversePointsForReversedEdges)},
		{"acyclic.undo", ok(l.acyclicUndo)},
	}
	for _, s := range steps {
		if err := s.fn(); err != nil {
			return err
		}
		if traceStep != nil {
			traceStep(s.name, l.g)
		}
	}
	return nil
}

// traceStep and traceBK are test hooks.
var (
	traceStep func(step string, g *lgraph)
	traceBK   func(alignment string, layering [][]string, root map[string]string, xs map[string]float64)
)

// buildLayoutGraph copies the input into a fresh compound multigraph, taking
// only the attributes that affect layout.
func (l *layout) buildLayoutGraph(in *Graph) {
	l.g = graph.New[*node, *edge](graph.Options{Multigraph: true, Compound: true})
	l.rankSep = l.opt.RankSep
	if l.opt.RankDir == "" {
		l.opt.RankDir = "tb"
	}
	compound := in.IsCompound()
	for _, v := range in.Nodes() {
		n := &node{}
		if in := in.Node(v); in != nil {
			n.width, n.height = in.Width, in.Height
		}
		l.g.SetNode(v, n)
		parent := ""
		if compound {
			parent = in.Parent(v)
		}
		// The parent may come later in the input; setting it here creates it
		// now, which fixes its place in the node order.
		l.g.SetParent(v, parent)
	}
	for _, e := range in.Edges() {
		le := &edge{minLen: 1, weight: 1, labelPos: "r", labelOffset: 10}
		if ie := in.Edge(e); ie != nil {
			le.minLen, le.weight = ie.MinLen, ie.Weight
			le.width, le.height = ie.Width, ie.Height
			le.labelPos, le.labelOffset = ie.LabelPos, ie.LabelOffset
		}
		l.g.SetEdge(e, le)
	}
}

func (l *layout) updateInputGraph(in *Graph) {
	for _, v := range in.Nodes() {
		n := in.Node(v)
		if n == nil {
			continue
		}
		ln := l.g.Node(v)
		n.X, n.Y = ln.x, ln.y
		if l.g.HasChildren(v) {
			n.Width, n.Height = ln.width, ln.height
		}
	}
	for _, e := range in.Edges() {
		ie := in.Edge(e)
		if ie == nil {
			continue
		}
		le := l.g.Edge(e)
		if le == nil {
			continue // dagre throws here (a reversed edge of minlen 0)
		}
		ie.Points = le.points
		if le.hasXY {
			ie.X, ie.Y, ie.HasXY = le.x, le.y, true
		}
	}
}

// makeSpaceForEdgeLabels splits each rank in two (doubling minlen, halving
// ranksep) so labels can sit on the in-between ranks, and pads label sizes.
func (l *layout) makeSpaceForEdgeLabels() {
	l.rankSep /= 2
	vertical := l.opt.RankDir == "TB" || l.opt.RankDir == "BT" // case-sensitive, as in dagre
	for _, e := range l.g.Edges() {
		edge := l.g.Edge(e)
		edge.minLen *= 2
		if strings.ToLower(edge.labelPos) != "c" {
			if vertical {
				edge.width += edge.labelOffset
			} else {
				edge.height += edge.labelOffset
			}
		}
	}
}

func (l *layout) removeSelfEdges() {
	for _, e := range l.g.Edges() {
		if e.V == e.W {
			n := l.g.Node(e.V)
			n.selfEdges = append(n.selfEdges, selfEdge{e, l.g.Edge(e)})
			l.g.RemoveEdge(e)
		}
	}
}

// injectEdgeLabelProxies adds a node on the rank where each label will go so
// removeEmptyRanks keeps that rank.
func (l *layout) injectEdgeLabelProxies() {
	for _, e := range l.g.Edges() {
		edge := l.g.Edge(e)
		if truthy(edge.width) && truthy(edge.height) {
			v, w := l.g.Node(e.V), l.g.Node(e.W)
			d := w.rank - v.rank // (w.rank - v.rank) / 2 + v.rank, kept in halves
			p := &node{rank: v.rank + floorDiv(d, 2), halfRank: d%2 != 0, e: e}
			l.addDummyNode(dummyEdgeProxy, p, "_ep")
		}
	}
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func (l *layout) assignRankMinMax() {
	l.g.EachNode(func(v string, n *node) {
		if n.borderTop != "" {
			n.minRank = l.g.Node(n.borderTop).rank
			n.maxRank = l.g.Node(n.borderBottom).rank
		}
	})
}

func (l *layout) removeEdgeLabelProxies() {
	for _, v := range l.g.Nodes() {
		n := l.g.Node(v)
		if n.dummy == dummyEdgeProxy {
			edge := l.g.Edge(n.e)
			edge.labelRank, edge.hasLabelRank = n.rank, !n.halfRank
			l.g.RemoveNode(v)
		}
	}
}

func (l *layout) translateGraph() {
	minX, maxX := math.Inf(1), 0.0
	minY, maxY := math.Inf(1), 0.0
	extremes := func(x, y, w, h float64) {
		minX = math.Min(minX, x-w/2)
		maxX = math.Max(maxX, x+w/2)
		minY = math.Min(minY, y-h/2)
		maxY = math.Max(maxY, y+h/2)
	}
	l.g.EachNode(func(_ string, n *node) { extremes(n.x, n.y, n.width, n.height) })
	edges := l.g.Edges()
	for _, e := range edges {
		if edge := l.g.Edge(e); edge.hasXY {
			extremes(edge.x, edge.y, edge.width, edge.height)
		}
	}
	marginX, marginY := orZero(l.opt.MarginX), orZero(l.opt.MarginY)
	minX -= marginX
	minY -= marginY
	l.g.EachNode(func(_ string, n *node) {
		n.x -= minX
		n.y -= minY
	})
	for _, e := range edges {
		edge := l.g.Edge(e)
		for i := range edge.points {
			edge.points[i].X -= minX
			edge.points[i].Y -= minY
		}
		if edge.hasXY {
			edge.x -= minX
			edge.y -= minY
		}
	}
	l.width = maxX - minX + marginX
	l.height = maxY - minY + marginY
}

// assignNodeIntersects adds each edge's end points on its nodes' borders.
// dagre throws when an edge points at a node from the node's own center.
func (l *layout) assignNodeIntersects() error {
	for _, e := range l.g.Edges() {
		edge := l.g.Edge(e)
		v, w := l.g.Node(e.V), l.g.Node(e.W)
		var p1, p2 Point
		if edge.points == nil {
			p1, p2 = Point{w.x, w.y}, Point{v.x, v.y}
		} else {
			p1, p2 = edge.points[0], edge.points[len(edge.points)-1]
		}
		a, ok1 := intersectRect(v, p1)
		b, ok2 := intersectRect(w, p2)
		if !ok1 || !ok2 {
			return fmt.Errorf("dagre: edge %q -> %q starts or ends at its node's center", e.V, e.W)
		}
		pts := make([]Point, 0, len(edge.points)+2)
		pts = append(pts, a)
		pts = append(pts, edge.points...)
		edge.points = append(pts, b)
	}
	return nil
}

func (l *layout) fixupEdgeLabelCoords() {
	for _, e := range l.g.Edges() {
		edge := l.g.Edge(e)
		if !edge.hasXY {
			continue
		}
		// Case-sensitive here, unlike elsewhere.
		switch edge.labelPos {
		case "l":
			edge.width -= edge.labelOffset
			edge.x -= edge.width/2 + edge.labelOffset
		case "r":
			edge.width -= edge.labelOffset
			edge.x += edge.width/2 + edge.labelOffset
		}
	}
}

func (l *layout) reversePointsForReversedEdges() {
	for _, e := range l.g.Edges() {
		if edge := l.g.Edge(e); edge.reversed {
			p := edge.points
			for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
				p[i], p[j] = p[j], p[i]
			}
		}
	}
}

func (l *layout) removeBorderNodes() {
	l.g.EachNode(func(v string, n *node) {
		if !l.g.HasChildren(v) {
			return
		}
		t, b := l.g.Node(n.borderTop), l.g.Node(n.borderBottom)
		lft := l.g.Node(n.borderLeft[len(n.borderLeft)-1])
		rgt := l.g.Node(n.borderRight[len(n.borderRight)-1])
		n.width = math.Abs(rgt.x - lft.x)
		n.height = math.Abs(b.y - t.y)
		n.x = lft.x + n.width/2
		n.y = t.y + n.height/2
	})
	for _, v := range l.g.Nodes() {
		if l.g.Node(v).dummy == dummyBorder {
			l.g.RemoveNode(v)
		}
	}
}

func (l *layout) insertSelfEdges() error {
	layering := buildLayerMatrix(l.g)
	if hasHoles(layering) {
		return errSameOrder
	}
	for _, layer := range layering {
		shift := 0
		for i, v := range layer {
			n := l.g.Node(v)
			n.order = i + shift
			for _, se := range n.selfEdges {
				shift++
				l.addDummyNode(dummySelfEdge, &node{
					width: se.label.width, height: se.label.height,
					rank: n.rank, order: i + shift,
					e: se.e, selfLabel: se.label,
				}, "_se")
			}
			n.selfEdges = nil
		}
	}
	return nil
}

func (l *layout) positionSelfEdges() {
	for _, v := range l.g.Nodes() {
		n := l.g.Node(v)
		if n.dummy != dummySelfEdge {
			continue
		}
		self := l.g.Node(n.e.V)
		x := self.x + self.width/2
		y := self.y
		dx := n.x - x
		dy := self.height / 2
		l.g.SetEdge(n.e, n.selfLabel)
		l.g.RemoveNode(v)
		n.selfLabel.points = []Point{
			{x + 2*dx/3, y - dy},
			{x + 5*dx/6, y - dy},
			{x + dx, y},
			{x + 5*dx/6, y + dy},
			{x + 2*dx/3, y + dy},
		}
		n.selfLabel.x, n.selfLabel.y, n.selfLabel.hasXY = n.x, n.y, true
	}
}

// truthy is JS truthiness for a number.
func truthy(f float64) bool { return f != 0 && !math.IsNaN(f) }

// orZero is JS's `f || 0`.
func orZero(f float64) float64 {
	if math.IsNaN(f) {
		return 0
	}
	return f
}

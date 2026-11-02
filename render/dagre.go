package render

import (
	"fmt"
	"sort"

	"github.com/f1sherb0y/mermaid-go/dagre"
	"github.com/f1sherb0y/mermaid-go/graph"
)

// layoutDagre lays the diagram out with dagre as one compound graph
// (layout-algorithms/dagre, simplified: clusters nest as dagre clusters;
// an edge to a cluster is laid out to a node inside it and cut at the
// cluster's frame; clusters keep the diagram's direction).
func (r *renderer) layoutDagre() (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("render: dagre layout: %v", p)
		}
	}()
	g := graph.New[*dagre.Node, *dagre.Edge](graph.Options{Multigraph: true, Compound: true})
	opt := dagre.DefaultOptions()
	opt.RankDir = r.data.Direction
	if opt.RankDir == "" {
		opt.RankDir = "TB"
	}
	section := r.section()
	opt.NodeSep = r.cfg.Num(section+".nodeSpacing", 50)
	opt.RankSep = r.cfg.Num(section+".rankSpacing", 50)
	opt.MarginX, opt.MarginY = 8, 8
	labels := map[string]*dagre.Node{}
	for _, n := range r.nodes {
		l := &dagre.Node{Width: n.w, Height: n.h}
		if n.IsGroup {
			l = &dagre.Node{}
		}
		labels[n.ID] = l
		g.SetNode(n.ID, l)
	}
	for _, n := range r.nodes {
		if n.parent != nil {
			g.SetParent(n.ID, n.parent.ID)
		}
	}
	type laid struct {
		e     *estate
		label *dagre.Edge
	}
	var edges []laid
	for i, e := range r.edges {
		s, t := r.leafOf(e.Start, true), r.leafOf(e.End, false)
		if s == "" || t == "" {
			continue
		}
		l := dagre.NewEdge()
		l.MinLen = max(1, e.Minlen)
		l.Width, l.Height = e.w, e.h
		l.LabelPos = "c"
		g.SetEdge(graph.Edge{V: s, W: t, Name: fmt.Sprint(i)}, l)
		edges = append(edges, laid{e, l})
	}
	if _, _, err := dagre.Layout(g, opt); err != nil {
		return err
	}
	for _, n := range r.nodes {
		l := labels[n.ID]
		n.x, n.y = l.X, l.Y
		if n.IsGroup {
			n.w, n.h = l.Width, l.Height
		}
		n.offX, n.offY = n.x-n.w/2, n.y-n.h/2
		n.laidOut = n.w > 0 || !n.IsGroup
	}
	for _, le := range edges {
		e, l := le.e, le.label
		pts := make([]P, len(l.Points))
		for i, p := range l.Points {
			pts[i] = P{X: p.X, Y: p.Y}
		}
		sn, tn := r.byID[e.Start], r.byID[e.End]
		// The ends meet the shapes' outlines (or the clusters' frames).
		if len(pts) >= 2 {
			pts = r.endOnOutline(pts, sn, true)
			pts = r.endOnOutline(pts, tn, false)
		}
		e.points = pts
		if l.HasXY {
			e.x, e.y, e.hasXY = l.X, l.Y, true
		}
	}
	r.sortForPaint()
	return nil
}

// section is the diagram type's config section.
func (r *renderer) section() string {
	switch r.sheet.kind {
	case "class":
		return "class"
	case "state":
		return "state"
	case "er":
		return "er"
	case "requirement":
		return "requirement"
	}
	return "flowchart"
}

// leafOf is the node an edge to id is laid out to: id itself, or for a
// cluster a node inside it: one the cluster's own edges leave from (as the
// edge's source) or arrive at (as its target) last, so that the edge's
// other end ranks beyond the whole cluster.
func (r *renderer) leafOf(id string, source bool) string {
	n := r.byID[id]
	if n == nil || !n.IsGroup {
		if n == nil {
			return ""
		}
		return n.ID
	}
	inside := n.descendants(map[string]bool{})
	var leaves []string
	var walk func(m *nstate)
	walk = func(m *nstate) {
		for _, c := range m.children {
			if c.IsGroup {
				walk(c)
			} else {
				leaves = append(leaves, c.ID)
			}
		}
	}
	walk(n)
	if len(leaves) == 0 {
		return ""
	}
	busy := map[string]bool{}
	for _, e := range r.edges {
		if inside[e.Start] && inside[e.End] && e.Start != e.End {
			if source {
				busy[e.Start] = true
			} else {
				busy[e.End] = true
			}
		}
	}
	for _, l := range leaves {
		if !busy[l] {
			return l
		}
	}
	return leaves[0]
}

// endOnOutline moves a route's end onto the node's outline: a cluster's
// frame cuts the route where it first enters it; a node's shape takes the
// point where the ray from its centre towards the route meets it.
func (r *renderer) endOnOutline(pts []P, n *nstate, start bool) []P {
	if n == nil {
		return pts
	}
	if !start {
		pts = reversed(pts)
	}
	b := box{X: n.x, Y: n.y, W: n.w, H: n.h}
	if n.IsGroup {
		// The route runs from inside the cluster out: cut at the last point
		// inside.
		i := 0
		for i+1 < len(pts) && !outsideNode(b, pts[i+1]) {
			i++
		}
		if i+1 < len(pts) {
			cross := borderIntersection(b, pts[i+1], pts[i])
			pts = append([]P{cross}, pts[i+1:]...)
		}
	} else if p, ok := n.nodeIntersect(pts[1]); ok {
		pts[0] = p
	} else {
		pts[0] = intersectRect(b, pts[1])
	}
	if !start {
		pts = reversed(pts)
	}
	return pts
}

// sortForPaint orders clusters before nodes, outer clusters first.
func (r *renderer) sortForPaint() {
	sort.SliceStable(r.nodes, func(i, j int) bool {
		a, b := r.nodes[i], r.nodes[j]
		if a.IsGroup != b.IsGroup {
			return a.IsGroup
		}
		return a.IsGroup && a.depth < b.depth
	})
}

package render

// Diagrams that place their nodes themselves (mindmap's cose-bilkent,
// block's grid, kanban's columns) lay out through Options.Place instead of
// ELK or dagre, as mermaid's renderers for them insert the nodes, read
// their sizes and position them.

// Placed is a node, or a cluster, as Options.Place sees it.
type Placed struct {
	// Node is the renderer's copy. Place may give a node a fixed size by
	// setting its Width, Height and WrappingWidth (and Extra); the node is
	// then drawn again with them, as mermaid inserts a positioned node.
	*Node
	// W, H are a node's measured size (its shape's box), a cluster's
	// title's size. Place sets a cluster's frame size here.
	W, H float64
	// X, Y are the centre Place puts the node or cluster at.
	X, Y float64
}

// layoutPlaced runs Options.Place, draws the nodes it resized again, and
// routes each edge straight from centre to centre through the midpoint,
// cut at the end nodes' outlines (where its label goes).
func (r *renderer) layoutPlaced() error {
	placed := make([]*Placed, len(r.nodes))
	type fixed struct{ w, h, wrap float64 }
	before := make([]fixed, len(r.nodes))
	for i, n := range r.nodes {
		p := &Placed{Node: n.Node, W: n.w, H: n.h}
		if n.IsGroup {
			p.W, p.H = n.labelW, n.labelH
		}
		placed[i] = p
		before[i] = fixed{n.Width, n.Height, n.WrappingWidth}
	}
	if err := r.opts.Place(placed); err != nil {
		return err
	}
	for i, n := range r.nodes {
		p := placed[i]
		if n.IsGroup {
			n.w, n.h = p.W, p.H
		} else if (fixed{n.Width, n.Height, n.WrappingWidth}) != before[i] {
			n.shape = r.shapeFn(n)(r, n)
			b := n.shape.bbox.scene()
			n.w, n.h = b.W, b.H
		}
		n.x, n.y = p.X, p.Y
		n.offX, n.offY = n.x-n.w/2, n.y-n.h/2
		n.laidOut = true
	}
	for _, e := range r.edges {
		sn, tn := r.byID[e.Start], r.byID[e.End]
		if sn == nil || tn == nil {
			continue
		}
		mid := P{X: (sn.x + tn.x) / 2, Y: (sn.y + tn.y) / 2}
		pts := []P{{X: sn.x, Y: sn.y}, mid, {X: tn.x, Y: tn.y}}
		pts = r.endOnOutline(pts, sn, true)
		e.points = r.endOnOutline(pts, tn, false)
		e.x, e.y, e.hasXY = mid.X, mid.Y, true
	}
	r.sortForPaint()
	return nil
}

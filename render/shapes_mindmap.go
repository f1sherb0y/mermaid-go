package render

import (
	"math"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// Mindmap's own shapes (defaultMindmapNode.ts, mindmapCircle.ts); its
// other node types use rect, rounded, cloud, bang and hexagon.

func init() {
	shapes["defaultMindmapNode"] = defaultMindmapNode
	shapes["mindmapCircle"] = mindmapCircle
}

// defaultMindmapNode is a rounded box with a line along its bottom (neo:
// rounded at the top only).
func defaultMindmapNode(r *renderer, n *nstate) *shape {
	l := r.nodeLabel(n, false)
	hp := n.Padding / 2
	w, h := l.W+8*hp, l.H+2*hp
	const rd = 5.0
	var segs []scene.Seg
	if n.look == "neo" {
		var b scene.PathBuilder
		b.MoveTo(-w/2, h/2)
		b.LineTo(-w/2, -h/2+rd)
		b.QuadTo(-w/2, -h/2, -w/2+rd, -h/2)
		b.LineTo(w/2-rd, -h/2)
		b.QuadTo(w/2, -h/2, w/2, -h/2+rd)
		b.LineTo(w/2, h/2)
		b.Close()
		segs = b.Segs
	} else {
		segs = scene.RectPath(-w/2, -h/2, w, h, rd, rd)
	}
	s := newShape()
	s.add(segs, r.nodePaint(n, pRect))
	s.items = append(s.items, r.nodePaint(n, pLine).path(polyline([]P{{X: -w / 2, Y: h / 2}, {X: w / 2, Y: h / 2}})))
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = intersectRect
	return s
}

// mindmapCircle is a circle padded by the node's whole padding (the
// flowchart's circle takes half).
func mindmapCircle(r *renderer, n *nstate) *shape {
	l := r.nodeLabel(n, true)
	rad := math.Hypot(l.W, l.H)/2 + n.Padding
	if n.look == "neo" {
		rad = math.Hypot(l.W, l.H)/2 + 16
	}
	s := newShape()
	s.add(scene.Ellipse(0, 0, rad, rad), r.nodePaint(n, pCircle))
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = func(b box, p P) P { return intersectEllipse(b, rad, rad, p) }
	return s
}

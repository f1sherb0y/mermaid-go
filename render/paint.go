package render

import (
	"math"
	"strings"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// paint draws the laid out diagram: clusters, edges, edge labels, nodes (in
// mermaid's group order), then sizes the view box as setupViewPortForSVG
// does from the drawing's box, with the title above.
func (r *renderer) paint() *scene.Scene {
	var clusters, edges, labels, nodes []scene.Item
	bb := emptyRect()
	for _, n := range r.nodes {
		if !n.laidOut {
			continue
		}
		if n.IsGroup {
			items, b := r.drawCluster(n)
			clusters = append(clusters, items...)
			bb.addRect(b.scene())
			continue
		}
	}
	// Nodes keep the data's order (the measuring pass inserted them so).
	nodes = r.nodesInDataOrder()
	for _, n := range r.nodes {
		if n.laidOut && !n.IsGroup && n.shape != nil && !n.shape.bbox.empty() {
			b := n.shape.bbox.scene()
			bb.addRect(scene.Rect{X: b.X + n.x, Y: b.Y + n.y, W: b.W, H: b.H})
		}
	}
	for _, e := range r.edges {
		items := r.drawEdge(e)
		edges = append(edges, items...)
		if len(items) > 0 {
			if p, ok := items[0].(*scene.Path); ok {
				bb.addRect(pathBounds(p.Segs))
			}
		}
	}
	for _, e := range r.edges {
		items := r.drawEdgeLabel(e)
		labels = append(labels, items...)
		if e.label != nil && e.Label != "" {
			bb.addRect(scene.Rect{X: e.x - e.w/2, Y: e.y - e.h/2 + r.subGraphTitleTotal()/2, W: e.w, H: e.h})
		}
		for k, t := range e.terminals {
			c := t.center
			if !t.placed {
				c = r.terminalPosition(e, k)
			}
			bb.addRect(scene.Rect{X: c.X - t.w/2, Y: c.Y - t.h/2, W: t.w, H: t.h})
		}
	}
	root := &scene.Group{}
	root.Items = append(root.Items, clusters...)
	root.Items = append(root.Items, edges...)
	root.Items = append(root.Items, labels...)
	root.Items = append(root.Items, nodes...)
	if r.data.Title != "" {
		margin := r.opts.TitleTopMargin
		if margin == 0 {
			margin = 25
		}
		st := r.sheet.font.style(run{})
		st.Size = r.sheet.titleSize
		asc, desc := r.fonts.Extents(st)
		cx := (bb.x0 + bb.x1) / 2
		if bb.empty() {
			cx = 0
		}
		w := r.fonts.Width(r.data.Title, st)
		root.Items = append(root.Items, &scene.Text{X: cx, Y: -margin, Anchor: scene.Middle, Family: r.sheet.font.Family,
			Size: st.Size, Color: r.sheet.titleColor, Runs: []scene.Span{{Text: r.data.Title}}})
		bb.addRect(scene.Rect{X: cx - w/2, Y: -margin - asc, W: w, H: asc + desc})
	}
	pad := r.opts.Padding
	if pad == 0 {
		pad = 8
	}
	if bb.empty() {
		bb = rect{0, 0, 0, 0}
	}
	vb := scene.Rect{X: bb.x0 - pad, Y: bb.y0 - pad, W: bb.x1 - bb.x0 + 2*pad, H: bb.y1 - bb.y0 + 2*pad}
	return &scene.Scene{ViewBox: vb, Items: []scene.Item{root}}
}

func (r *renderer) nodesInDataOrder() []scene.Item {
	var out []scene.Item
	for _, dn := range r.data.Nodes {
		n := r.byID[dn.ID]
		if n == nil || n.IsGroup || !n.laidOut || n.shape == nil {
			continue
		}
		out = append(out, &scene.Group{Transform: scene.Translate(n.x, n.y), Items: n.shape.items})
	}
	return out
}

// drawCluster draws a cluster (clusters.js) at its laid out box.
func (r *renderer) drawCluster(n *nstate) ([]scene.Item, rect) {
	fn := clusterShapes[n.Shape]
	if fn == nil {
		fn = clusterRect
	}
	return fn(r, n)
}

var clusterShapes = map[string]func(r *renderer, n *nstate) ([]scene.Item, rect){}

func init() {
	clusterShapes["rect"] = clusterRect
	clusterShapes["squareRect"] = clusterRect
	clusterShapes["noteGroup"] = noteGroup
}

// clusterPaint is the stylesheet's paint for a cluster's rect, the palette
// slot's colors over it in color themes, its own styles over all.
func (r *renderer) clusterPaint(n *nstate) paint {
	s := r.sheet
	p := paint{Fill: s.clusterFill, Stroke: s.clusterStroke, Width: s.clusterWidth}
	if n.look == "neo" {
		p.Stroke = s.nodeStroke
		p.Shadow = s.shadow
	}
	if n.ColorIndex != nil && len(s.palette) > 0 {
		sl := s.palette[*n.ColorIndex%len(s.palette)]
		p.Stroke = sl.border
		if !sl.fill.IsNone() {
			p.Fill = sl.fill
		}
	}
	css, _ := nodeStyles(n.Node)
	return p.withCSS(css...)
}

// clusterTitle lays out a cluster's title as createLabel paints it:
// unwrapped (markdown titles wrap at the cluster's width).
func (r *renderer) clusterTitle(n *nstate) *Label {
	_, lcss := nodeStyles(n.Node)
	f := r.sheet.font.withCSS(lcss...)
	f.Color = r.sheet.clusterText
	f = f.withCSS(lcss...)
	width := math.Inf(1)
	if n.LabelType == "markdown" {
		width = n.w
	}
	return r.layoutLabel(n.Label, f, labelOpts{Width: width, Markdown: n.LabelType == "markdown"})
}

func clusterRect(r *renderer, n *nstate) ([]scene.Item, rect) {
	l := r.clusterTitle(n)
	w := n.w
	if n.w <= l.W+n.Padding {
		w = l.W + n.Padding
	}
	h := n.h
	x, y := n.x-w/2, n.y-h/2
	p := r.clusterPaint(n)
	segs := scene.RectPath(x, y, w, h, n.Rx, n.Ry)
	items := []scene.Item{p.path(segs)}
	top := r.cfg.Num("flowchart.subGraphTitleMargin.top", 0)
	items = append(items, r.drawLabel(l, n.x-l.W/2, n.y-n.h/2+top)...)
	bb := emptyRect()
	bb.addRect(scene.Rect{X: x, Y: y, W: w, H: h})
	bb.addRect(scene.Rect{X: n.x - l.W/2, Y: n.y - n.h/2 + top, W: l.W, H: l.H})
	return items, bb
}

// noteGroup is an invisible frame.
func noteGroup(r *renderer, n *nstate) ([]scene.Item, rect) {
	bb := emptyRect()
	bb.addRect(scene.Rect{X: n.x - n.w/2, Y: n.y - n.h/2, W: n.w, H: n.h})
	return nil, bb
}

func init() {
	clusterShapes["roundedWithTitle"] = roundedWithTitle
	clusterShapes["divider"] = dividerCluster
}

// clusterSlot is a cluster's palette slot, if the theme has one for it.
func (r *renderer) clusterSlot(n *nstate) (slot, bool) {
	s := r.sheet
	if n.ColorIndex == nil || len(s.palette) == 0 || n.look != "neo" {
		return slot{}, false
	}
	return s.palette[*n.ColorIndex%len(s.palette)], true
}

// roundedWithTitle is a composite state: a rounded frame whose title sits
// in a band above an inner rect holding the children.
func roundedWithTitle(r *renderer, n *nstate) ([]scene.Item, rect) {
	l := r.clusterTitle(n)
	w := n.w
	if n.w <= l.W+n.Padding {
		w = l.W + n.Padding
	}
	h := n.h
	x, y := n.x-w/2, n.y-h/2
	s := r.sheet
	outer := paint{Fill: r.color("compositeTitleBackground"), Stroke: s.nodeStroke, Width: s.nodeWidth}
	inner := paint{Fill: r.color("compositeBackground"), Stroke: s.nodeStroke, Width: s.nodeWidth}
	if strings.Contains(n.CSSClasses, "statediagram-cluster-alt") {
		inner.Fill = r.color("altBackground")
	}
	rad := 5.0
	if n.look == "neo" {
		rad = 12
		outer.Fill = s.nodeFill
		outer.Shadow = s.shadow
	}
	if sl, ok := r.clusterSlot(n); ok {
		outer.Stroke, inner.Stroke = sl.border, sl.border
		if !sl.fill.IsNone() {
			outer.Fill = sl.fill
		}
	}
	css, _ := nodeStyles(n.Node)
	outer, inner = outer.withCSS(css...), inner.withCSS(css...)
	innerY := y + l.H + 2
	items := []scene.Item{
		outer.path(scene.RectPath(x, y, w, h, rad, rad)),
		inner.path(scene.RectPath(x, innerY, w, h-l.H-6, 0, 0)),
	}
	items = append(items, r.drawLabel(l, n.x-l.W/2, y+1)...)
	bb := emptyRect()
	bb.addRect(scene.Rect{X: x, Y: y, W: w, H: h})
	return items, bb
}

// dividerCluster is one region of a concurrent state: a dashed frame.
func dividerCluster(r *renderer, n *nstate) ([]scene.Item, rect) {
	s := r.sheet
	x, y := n.x-n.w/2, n.y-n.h/2
	p := paint{Fill: r.color("altBackground"), Stroke: s.nodeStroke, Width: s.nodeWidth, Dash: []float64{10, 10}}
	if sl, ok := r.clusterSlot(n); ok {
		p.Stroke = sl.border
		if !sl.fill.IsNone() {
			p.Fill = sl.fill
		}
	}
	css, _ := nodeStyles(n.Node)
	bb := emptyRect()
	bb.addRect(scene.Rect{X: x, Y: y, W: n.w, H: n.h})
	return []scene.Item{p.withCSS(css...).path(scene.RectPath(x, y, n.w, n.h, 0, 0))}, bb
}

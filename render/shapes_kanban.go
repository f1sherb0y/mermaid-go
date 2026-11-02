package render

import (
	"math"

	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

// Kanban's card (kanbanItem.ts) and column (clusters.js kanbanSection).

func init() {
	shapes["kanbanItem"] = kanbanItem
	clusterShapes["kanbanSection"] = kanbanSection
}

// priorityColors are the colors of a card's priority bar; "Medium" and
// unknown priorities have none.
var priorityColors = map[string]string{
	"Very High": "red", "High": "orange", "Low": "blue", "Very Low": "lightblue",
}

// kanbanItem is a card: its title wrapped 10px inside its width, the
// ticket (underlined when kanban.ticketBaseUrl links it) bottom left, the
// assignee bottom right, and a bar on the left colored by priority. Extra
// holds "ticket", "assigned" and "priority".
func kanbanItem(r *renderer, n *nstate) *shape {
	w := n.Width
	if w == 0 {
		w = 200
	}
	// The title wraps 10px inside the card.
	n.Width = w - 10
	title := r.nodeLabel(n, false)
	n.Width = w
	pad := n.Padding
	if pad == 0 {
		pad = 10
	}
	_, lcss := nodeStyles(n.Node)
	f := r.sheet.font.withCSS(lcss...)
	ticket, assigned := n.ExtraString("ticket"), n.ExtraString("assigned")
	tf := f
	tf.Underline = ticket != "" && r.cfg.Str("kanban.ticketBaseUrl", "") != ""
	tl := r.layoutLabel(ticket, tf, labelOpts{Width: w - 10})
	al := r.layoutLabel(assigned, f, labelOpts{Width: w - 10})
	if ticket == "" {
		tl.W, tl.H = 0, 0
	}
	if assigned == "" {
		al.W, al.H = 0, 0
	}
	adj := math.Max(tl.H, al.H) / 2
	h := math.Max(title.H+20, n.Height) + adj
	x, y := -w/2, -h/2
	s := newShape()
	s.add(scene.RectPath(x, y, w, h, n.Rx, n.Ry), r.nodePaint(n, pRect))
	if c, ok := priorityColors[n.ExtraString("priority")]; ok {
		cut := math.Floor(n.Rx / 2)
		s.items = append(s.items, paint{Stroke: style.Color(c), Width: 4}.path(polyline([]P{{X: x + 2, Y: y + cut}, {X: x + 2, Y: y + h - cut}})))
	}
	s.put(r, title, pad-w/2, -adj-title.H/2)
	s.put(r, tl, pad-w/2, -adj+title.H/2)
	s.put(r, al, pad+w/2-al.W-20, -adj+title.H/2)
	s.intersect = intersectRect
	return s
}

// kanbanSection is a column: a rounded rect of the section's color with
// its title at the top.
func kanbanSection(r *renderer, n *nstate) ([]scene.Item, rect) {
	l := r.clusterTitle(n)
	w := math.Max(n.w, l.W+n.Padding)
	x, y := n.x-w/2, n.y-n.h/2
	s := r.sheet
	p := r.sectionPaint(n, pRect, paint{Fill: s.clusterFill, Stroke: s.clusterStroke, Width: s.clusterWidth})
	css, _ := nodeStyles(n.Node)
	items := []scene.Item{p.withCSS(css...).path(scene.RectPath(x, y, w, n.h, n.Rx, n.Ry))}
	items = append(items, r.drawLabel(l, n.x-l.W/2, y)...)
	bb := emptyRect()
	bb.addRect(scene.Rect{X: x, Y: y, W: w, H: n.h})
	return items, bb
}

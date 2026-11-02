// Package architecture is mermaid's architecture diagram
// (diagrams/architecture, `architecture-beta`): services with icons,
// junctions, nested groups, and edges between given sides of them.
//
// mermaid lays it out with cytoscape's fcose (forces under alignment and
// placement constraints taken from the edges' sides). This package makes
// that layout too, exactly (package fcose, with mermaid's graph and
// constraints: fcose.go), and its own on a grid (grid.go, then pixels
// under the same constraints, pixels.go): the same shape, at about the same
// spacing, without fcose's overlaps and scattering, for any nesting. A
// diagram keeps mermaid's layout unless ours is clearly better: no
// overlaps, fewer edges through nodes or titles, fewer crossings and
// misaligned edges, or a far smaller drawing (choose.go). An edge that
// would run through another node goes around it (route.go). Icons are
// Iconify icons (package icons); mermaid's own five are
// built in, and a host can register more sets (icons.Register), which
// mermaid.live does not, so "logos:..." icons are the unknown icon there
// and here by default.
//
// The title and accessibility statements are parsed and not drawn
// (mermaid draws no title for this diagram).
package architecture

import (
	"math"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/icons"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

func init() {
	icons.AddSet(iconPrefix, builtins)
	diagram.Register("architecture", Render)
}

// Render draws an architecture diagram.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	d, err := Parse(code)
	if err != nil {
		return nil, err
	}
	l := newLayout(d, ctx)
	l.place()
	return ctx.Fit(l.draw(), l.padding), nil
}

// colors are the theme's.
type colors struct {
	edge, arrow, border, text scene.Color
	edgeWidth, borderWidth    float64
}

// draw paints the laid-out diagram as mermaid stacks it: edges, then
// services, then the groups' (unfilled) boxes.
func (l *layout) draw() []scene.Item {
	th := l.ctx.Theme
	c := colors{
		edge: style.Color(th.Get("archEdgeColor")), arrow: style.Color(th.Get("archEdgeArrowColor")),
		border: style.Color(th.Get("archGroupBorderColor")), text: style.Color(th.Get("textColor")),
		edgeWidth: style.Px(th.Get("archEdgeWidth"), 3), borderWidth: style.Px(th.Get("archGroupBorderWidth"), 2),
	}
	var items []scene.Item
	for _, e := range l.d.Edges {
		items = append(items, l.edge(e, c)...)
	}
	for _, n := range l.d.Nodes {
		if !n.Junction {
			items = append(items, l.service(n, c)...)
		}
	}
	for _, g := range l.d.Groups {
		items = append(items, l.group(g, c)...)
	}
	return items
}

// service draws a service's icon and its label under it.
func (l *layout) service(n *Node, c colors) []scene.Item {
	s := l.iconSize
	p := l.at[n]
	x, y := p.X-s/2, p.Y-s/2
	var items []scene.Item
	if tb, ok := l.labels[n]; ok {
		items = append(items, l.lines(tb, p.X, y+s+l.font.Size, c.text)...)
	}
	switch {
	case n.Icon != "":
		items = append(items, l.icon(n.Icon, x, y, s, c.text))
	case n.IconText != "":
		items = append(items, l.icon("blank", x, y, s, c.text))
		items = append(items, l.iconText(n.IconText, x, y)...)
	default:
		var b scene.PathBuilder
		b.MoveTo(x, y+s).LineTo(x, y+5).QuadTo(x, y, x+5, y).LineTo(x+s-5, y).QuadTo(x+s, y, x+s, y+5).LineTo(x+s, y+s).Close()
		items = append(items, &scene.Path{Segs: b.Segs, Stroke: c.border, StrokeWidth: c.borderWidth, Dash: []float64{8}})
	}
	return items
}

// lines draws wrapped text centered on x, its first line's middle at y.
func (l *layout) lines(tb textBlock, x, y float64, col scene.Color) []scene.Item {
	var out []scene.Item
	for i, s := range tb.lines {
		out = append(out, &scene.Text{X: x, Y: y + float64(i)*l.lineH, Anchor: scene.Middle, Baseline: scene.MiddleBaseline,
			Family: l.font.Family, Size: l.font.Size, Color: col, Runs: []scene.Span{{Text: s}}})
	}
	return out
}

// icon draws a named icon in an s-sized square, the unknown icon when it
// is not registered.
func (l *layout) icon(name string, x, y, s float64, fill scene.Color) scene.Item {
	ic, ok := icons.Lookup(name, iconPrefix)
	if !ok {
		ic = icons.Unknown
	}
	return ic.Draw(x, y, s, s, fill)
}

// iconText is a service's text in place of an icon: white, wrapped to the
// icon, centered, as many lines as fit.
func (l *layout) iconText(s string, x, y float64) []scene.Item {
	size := l.iconSize
	tb := l.wrap(s, size-2)
	lineH := l.asc + l.desc
	if max := int((size - 2) / l.font.Size); len(tb.lines) > max {
		tb.lines = tb.lines[:max]
	}
	top := y + (size-float64(len(tb.lines))*lineH)/2
	var out []scene.Item
	for i, line := range tb.lines {
		out = append(out, &scene.Text{X: x + size/2, Y: top + float64(i)*lineH + l.asc, Anchor: scene.Middle,
			Family: l.font.Family, Size: l.font.Size, Color: scene.RGB(255, 255, 255), Runs: []scene.Span{{Text: line}}})
	}
	return out
}

// group draws a group's dashed box, and its icon and title in the top left
// corner.
func (l *layout) group(g *Group, c colors) []scene.Item {
	b := l.box[g]
	items := []scene.Item{&scene.Path{Segs: scene.RectPath(b.X, b.Y, b.W, b.H, 0, 0), Stroke: c.border, StrokeWidth: c.borderWidth, Dash: []float64{8}}}
	baseline := 18.0
	if g.Icon != "" {
		items = append(items, l.icon(g.Icon, b.X+1, b.Y+1, 0.75*l.padding, c.text))
		baseline = 23
	}
	if g.Title != "" {
		items = append(items, &scene.Text{X: b.X + l.titleX(g), Y: b.Y + baseline, Family: l.font.Family, Size: l.font.Size,
			Color: c.text, Runs: []scene.Span{{Text: g.Title}}})
	}
	return items
}

// edge draws an edge along its course (straight between opposite sides,
// with one bend between perpendicular ones, or around what is in the way);
// arrowheads; its label.
func (l *layout) edge(e *Edge, c colors) []scene.Item {
	pts := l.routes[e]
	var b scene.PathBuilder
	b.MoveTo(pts[0].X, pts[0].Y)
	for _, p := range pts[1:] {
		b.LineTo(p.X, p.Y)
	}
	items := []scene.Item{&scene.Path{Segs: b.Segs, Stroke: c.edge, StrokeWidth: c.edgeWidth}}
	if e.FromArrow {
		items = append(items, l.arrow(pts[1], pts[0], e.FromSide, c.arrow))
	}
	if e.ToArrow {
		items = append(items, l.arrow(pts[len(pts)-2], pts[len(pts)-1], e.ToSide, c.arrow))
	}
	if tb, ok := l.edgeLabels[e]; ok {
		items = append(items, l.edgeLabel(e, tb, pts, c.text)...)
	}
	return items
}

// endpoint is where an edge meets a node's side: the middle of the icon's
// side (a junction's center; under the label at the bottom, where mermaid
// draws through it), or with {group} the border of the outermost
// group around the node that is not around the other end.
func (l *layout) endpoint(n *Node, s Side, group bool, other *Node) scene.Pt {
	p, u := l.at[n], unit(s)
	if group {
		var out *Group
		for g := n.Parent; g != nil && !inside(other, g); g = g.Parent {
			out = g
		}
		if out != nil {
			b := l.box[out]
			switch s {
			case 'L':
				p.X = b.X
			case 'R':
				p.X = b.X + b.W
			case 'T':
				p.Y = b.Y
			default:
				p.Y = b.Y + b.H
			}
			return p
		}
	}
	if n.Junction {
		return p
	}
	if _, ok := l.labels[n]; ok && s == 'B' && l.at[other].Y > p.Y {
		// Under the label, not through it.
		return scene.Pt{X: p.X, Y: p.Y + l.textB[n] + 3}
	}
	return scene.Pt{X: p.X + u.x*l.iconSize/2, Y: p.Y + u.y*l.iconSize/2}
}

// arrow is mermaid's arrowhead (a sixth of an icon) at the end p of a
// line from q, pointing along it, its tip 2px past p. mermaid points it
// into the node's side whatever the line does; along the line, it follows
// a slanted edge too (into side s when the line has no length).
func (l *layout) arrow(q, p scene.Pt, s Side, col scene.Color) scene.Item {
	size := l.iconSize / 6
	dx, dy := p.X-q.X, p.Y-q.Y
	in := scene.Pt{X: -unit(s).x, Y: -unit(s).y}
	if n := math.Hypot(dx, dy); n > 1e-9 {
		in = scene.Pt{X: dx / n, Y: dy / n}
	}
	tip := scene.Pt{X: p.X + 2*in.X, Y: p.Y + 2*in.Y}
	base := scene.Pt{X: tip.X - size*in.X, Y: tip.Y - size*in.Y}
	across := scene.Pt{X: -in.Y * size / 2, Y: in.X * size / 2}
	return &scene.Path{Segs: scene.Polygon(tip, scene.Pt{X: base.X + across.X, Y: base.Y + across.Y},
		scene.Pt{X: base.X - across.X, Y: base.Y - across.Y}), Fill: col}
}

// edgeLabel places an edge's label as mermaid does: under a horizontal
// edge, along a vertical one (turned, on its right), and at a bend's
// corner, turned 45° into it. An edge that goes around has it on its
// longest line.
func (l *layout) edgeLabel(e *Edge, tb textBlock, pts []scene.Pt, col scene.Color) []scene.Item {
	txt := func(x, y, rot float64, s string, base scene.Baseline) *scene.Text {
		return &scene.Text{X: x, Y: y, Anchor: scene.Middle, Baseline: base, Rotate: rot,
			Family: l.font.Family, Size: l.font.Size, Color: col, Runs: []scene.Span{{Text: s}}}
	}
	from, to := pts[0], pts[len(pts)-1]
	mid := scene.Pt{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}
	horizontal := isX(e.FromSide) && isX(e.ToSide)
	bend := isX(e.FromSide) != isX(e.ToSide)
	if bend {
		mid = pts[1]
	}
	if len(pts) > 3 || len(pts) == 3 && !bend {
		// Around: the longest line.
		k, best := 1, -1.0
		for i := 1; i < len(pts); i++ {
			if d := math.Abs(pts[i].X-pts[i-1].X) + math.Abs(pts[i].Y-pts[i-1].Y); d > best {
				k, best = i, d
			}
		}
		a, b := pts[k-1], pts[k]
		mid = scene.Pt{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
		horizontal, bend = a.Y == b.Y, false
	}
	var out []scene.Item
	switch {
	case horizontal:
		return l.lines(tb, mid.X, mid.Y+l.font.Size, col)
	case !bend:
		y := mid.Y
		for i, s := range tb.lines {
			out = append(out, txt(mid.X+l.font.Size+float64(i)*l.lineH, y, -90, s, scene.MiddleBaseline))
		}
	default:
		// mermaid's factors: which quadrant of the corner the label sits in.
		fx, fy := 1.0, 1.0
		switch string([]byte{byte(e.FromSide), byte(e.ToSide)}) {
		case "LT", "TL":
		case "LB", "BL":
			fy = -1
		case "RB", "BR":
			fx, fy = -1, -1
		default:
			fx = -1
		}
		h := float64(len(tb.lines)) * l.lineH
		side := (tb.w + h) / math.Sqrt2
		rot := -fx * fy * 45
		cx, cy := mid.X+fx*side/2, mid.Y+fy*side/2
		sin, cos := math.Sincos(rot * math.Pi / 180)
		for i, s := range tb.lines {
			dy := (float64(i) - float64(len(tb.lines)-1)/2) * l.lineH
			out = append(out, txt(cx-dy*sin, cy+dy*cos, rot, s, scene.Central))
		}
	}
	return out
}

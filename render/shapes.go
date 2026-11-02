package render

import (
	"math"

	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

// A shape draws a node about its centre (mermaid's shapes.ts and
// shapes/*.ts): its paths and label, its box (what getBBox measures, which
// becomes the node's size), and how an edge meets its outline.
type shape struct {
	items []scene.Item
	bbox  rect
	// intersect is where the ray from the node's centre towards p leaves
	// the outline, the node placed at b; nil for none.
	intersect func(b box, p P) P
}

// shapeFn draws a node's shape.
type shapeFn func(r *renderer, n *nstate) *shape

// part is what a shape element is, for the stylesheet: neo's shadow goes on
// rects, polygons, circles and paths marked outer.
type part uint8

const (
	pRect part = iota
	pPolygon
	pCircle
	pEllipse
	pPath      // a plain path: no shadow
	pOuterPath // the shape's outline path (class outer-path)
	pLine      // decoration lines (neo-line): stroke only, no shadow
)

// add draws segments with the node's paint for the part.
func (s *shape) add(segs []scene.Seg, p paint) *scene.Path {
	path := p.path(segs)
	s.items = append(s.items, path)
	s.bbox.addRect(pathBounds(segs))
	return path
}

// addLabel draws a label with its box's top left at (x, y).
func (s *shape) addLabel(r *renderer, l *Label, x, y float64) {
	if l == nil {
		return
	}
	s.items = append(s.items, r.drawLabel(l, x, y)...)
	// The label's foreignObject counts in the node's box, as does the empty
	// <rect> mermaid puts at the label's origin.
	s.bbox.addRect(scene.Rect{X: x, Y: y, W: l.W, H: l.H})
}

func newShape() *shape { return &shape{bbox: emptyRect()} }

// nodePaint is the stylesheet's paint for a part of a node, with the node's
// own styles (classDef, style; !important in mermaid) over it.
func (r *renderer) nodePaint(n *nstate, k part) paint {
	s := r.sheet
	p := paint{Fill: s.nodeFill, Stroke: s.nodeStroke, Width: s.nodeWidth}
	switch k {
	case pRect, pPolygon, pCircle, pOuterPath:
		p.Shadow = s.shadow
	case pLine:
		p.Fill = scene.None
	}
	if n.look != "neo" {
		p.Shadow = nil
	}
	p = r.slotPaint(n, k, p)
	p = r.sectionPaint(n, k, p)
	css, _ := nodeStyles(n.Node)
	return p.withCSS(css...)
}

// nodeLabel lays out a node's label as labelHelper does: wrapped at the
// node's width, its wrapping width or flowchart.wrappingWidth, widened to
// its minimum width (unless the shape opts out), centred on the node.
func (r *renderer) nodeLabel(n *nstate, ignoreMinWidth bool) *Label {
	_, lcss := nodeStyles(n.Node)
	f := r.sheet.font
	f.Color = r.sectionText(n, f.Color)
	f = f.withCSS(lcss...)
	width := n.Width
	if width == 0 {
		width = n.WrappingWidth
	}
	if width == 0 {
		width = r.cfg.Num("flowchart.wrappingWidth", 200)
	}
	// A label styled white-space: nowrap keeps it when the div switches
	// to wrapping: it never wraps.
	if style.Parse(lcss...)["white-space"] == "nowrap" {
		width = math.Inf(1)
	}
	minW := 0.0
	if n.Label != "" && n.Width == 0 && !ignoreMinWidth {
		minW = n.MinWidth
	}
	return r.layoutLabel(n.Label, f, labelOpts{Width: width, MinWidth: minW, Markdown: n.LabelType == "markdown"})
}

// shapes are mermaid's shape handlers by name and alias.
var shapes = map[string]shapeFn{}

func init() {
	reg := func(fn shapeFn, names ...string) {
		for _, n := range names {
			shapes[n] = fn
		}
	}
	reg(squareRect, "squareRect", "rect", "proc", "process", "rectangle")
	reg(roundedRect, "roundedRect", "rounded", "event")
	reg(stadium, "stadium", "terminal", "pill")
	reg(subroutine, "subroutine", "fr-rect", "subprocess", "subproc", "framed-rectangle")
	reg(cylinder, "cylinder", "cyl", "db", "database")
	reg(circleShape, "circle", "circ")
	reg(question, "question", "diamond", "diam", "decision")
	reg(hexagon, "hexagon", "hex", "prepare")
	reg(state, "state")
	reg(stateStart, "stateStart", "sm-circ", "start", "small-circle")
	reg(stateEnd, "stateEnd", "fr-circ", "stop", "framed-circle")
	reg(choice, "choice")
	reg(forkJoin, "fork", "join", "forkJoin")
	reg(note, "note")
	reg(textShape, "text")
	reg(labelRect, "labelRect")
}

// rectOpts are drawRect's options.
type rectOpts struct {
	rx, ry       float64
	padX, padY   float64
	shadowOnRect bool
}

// drawRect is a rectangle about the label (drawRect.ts): label padded by
// padX, padY on each side, at least the node's fixed size.
func drawRect(r *renderer, n *nstate, o rectOpts) *shape {
	l := r.nodeLabel(n, false)
	w := math.Max(l.W+o.padX*2, n.Width)
	h := math.Max(l.H+o.padY*2, n.Height)
	rx, ry := n.Rx, n.Ry
	if o.rx != 0 && o.ry != 0 {
		rx, ry = o.rx, o.ry
	}
	s := newShape()
	s.add(scene.RectPath(-w/2, -h/2, w, h, rx, ry), r.nodePaint(n, pRect))
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = intersectRect
	return s
}

func squareRect(r *renderer, n *nstate) *shape {
	px, py := n.Padding*2, n.Padding
	if n.look == "neo" {
		px, py = 16, 12
	}
	if v := n.extraNum("labelPaddingX"); v != 0 {
		px = v
	}
	return drawRect(r, n, rectOpts{padX: px, padY: py})
}

func roundedRect(r *renderer, n *nstate) *shape {
	rad := 5.0
	if v := r.th.Get("radius"); v != "" {
		rad = parseNum(v, 5)
	}
	if r.sheet.kind == "state" {
		rad = 5 // .statediagram-state rect.basic
	}
	return drawRect(r, n, rectOpts{rx: rad, ry: rad, padX: n.Padding, padY: n.Padding})
}

func state(r *renderer, n *nstate) *shape {
	rad := 5.0
	if n.look == "neo" {
		rad = 3
	}
	return drawRect(r, n, rectOpts{rx: rad, ry: rad, padX: 0, padY: 0})
}

func labelRect(r *renderer, n *nstate) *shape {
	l := r.nodeLabel(n, false)
	s := newShape()
	s.bbox.addRect(scene.Rect{W: 0.1, H: 0.1})
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = intersectRect
	return s
}

func textShape(r *renderer, n *nstate) *shape {
	l := r.nodeLabel(n, false)
	w := math.Max(l.W+n.Padding, n.Width)
	h := math.Max(l.H+n.Padding, n.Height)
	s := newShape()
	// rect.text has no fill or stroke.
	s.bbox.addRect(scene.Rect{X: -w / 2, Y: -h / 2, W: w, H: h})
	css, _ := nodeStyles(n.Node)
	if p := (paint{}).withCSS(css...); !p.Fill.IsNone() || !p.Stroke.IsNone() {
		s.add(scene.RectPath(-w/2, -h/2, w, h, 0, 0), p)
	}
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = intersectRect
	return s
}

func question(r *renderer, n *nstate) *shape {
	l := r.nodeLabel(n, false)
	w := l.W + n.Padding
	h := l.H + n.Padding
	sz := w + h
	pts := []P{{X: sz / 2, Y: 0}, {X: sz, Y: -sz / 2}, {X: sz / 2, Y: -sz}, {X: 0, Y: -sz / 2}}
	s := newShape()
	s.add(poly(pts, -sz/2+0.5, sz/2), r.nodePaint(n, pPolygon))
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = func(b box, p P) P {
		sz := b.W
		pts := []P{{X: sz / 2, Y: 0}, {X: sz, Y: -sz / 2}, {X: sz / 2, Y: -sz}, {X: 0, Y: -sz / 2}}
		return intersectPolygon(b, pts, p)
	}
	return s
}

// circlePoints are generateCirclePoints' points: on a circle, negated.
func circlePoints(cx, cy, radius float64, n int, start, end float64) []P {
	a0, a1 := start*math.Pi/180, end*math.Pi/180
	step := (a1 - a0) / float64(n-1)
	out := make([]P, n)
	for i := range n {
		a := a0 + float64(i)*step
		out[i] = P{X: -(cx + radius*math.Cos(a)), Y: -(cy + radius*math.Sin(a))}
	}
	return out
}

func stadium(r *renderer, n *nstate) *shape {
	px, py := n.Padding, n.Padding
	if n.look == "neo" {
		px, py = 40, 24
	}
	l := r.nodeLabel(n, false)
	const arcPoints = 50
	h := l.H + py
	inscribed := h * math.Cos(math.Pi/(2*(arcPoints-1)))
	capW := math.Sqrt(math.Max(0, inscribed*inscribed-l.H*l.H))
	w := math.Max(l.W+h/4+px, math.Max(1.5*h+(h-inscribed), l.W+px+h-capW))
	rad := h / 2
	pts := []P{{X: -w/2 + rad, Y: -h / 2}, {X: w/2 - rad, Y: -h / 2}}
	pts = append(pts, circlePoints(-w/2+rad, 0, rad, arcPoints, 90, 270)...)
	pts = append(pts, P{X: w/2 - rad, Y: h / 2})
	pts = append(pts, circlePoints(w/2-rad, 0, rad, arcPoints, 270, 450)...)
	s := newShape()
	s.add(poly(pts, 0, 0), r.nodePaint(n, pOuterPath))
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = func(b box, p P) P { return intersectPolygon(b, pts, p) }
	return s
}

func subroutine(r *renderer, n *nstate) *shape {
	pad := n.Padding
	if pad == 0 {
		pad = 8
	}
	px, py := pad, pad
	if n.look == "neo" {
		px, py = 28, 12
	}
	const frame = 8.0
	l := r.nodeLabel(n, false)
	tw := math.Max(l.W+2*frame+px, n.Width)
	th := math.Max(l.H+py, n.Height)
	w, h := tw-2*frame, th
	pts := []P{{X: 0, Y: 0}, {X: w, Y: 0}, {X: w, Y: -h}, {X: 0, Y: -h}, {X: 0, Y: 0}, {X: -8, Y: 0}, {X: w + 8, Y: 0}, {X: w + 8, Y: -h}, {X: -8, Y: -h}, {X: -8, Y: 0}}
	s := newShape()
	s.add(poly(pts, -w/2, h/2), r.nodePaint(n, pPolygon))
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = func(b box, p P) P { return intersectPolygon(b, pts, p) }
	return s
}

func cylinder(r *renderer, n *nstate) *shape {
	px, py := n.Padding, n.Padding
	if n.look == "neo" {
		px, py = 24, 24
	}
	fixedW, fixedH := n.Width, n.Height
	if fixedW != 0 {
		fixedW = math.Max(8, fixedW-py)
	}
	if fixedH != 0 {
		rx := n.Width / 2
		ry := rx / (2.5 + n.Width/50)
		fixedH = math.Max(8, fixedH-px-ry*3)
	}
	l := r.nodeLabel(n, false)
	w := math.Max(fixedW, l.W) + py
	rx := w / 2
	ry := rx / (2.5 + w/50)
	h := math.Max(fixedH, l.H) + px + ry
	var b scene.PathBuilder
	b.MoveTo(0, ry)
	b.ArcTo(rx, ry, 0, false, false, w, ry)
	b.ArcTo(rx, ry, 0, false, false, 0, ry)
	b.LineTo(0, ry+h)
	b.ArcTo(rx, ry, 0, false, false, w, ry+h)
	b.LineTo(w, ry)
	s := newShape()
	s.add(translate(b.Segs, -w/2, -(h/2+ry)), r.nodePaint(n, pOuterPath))
	s.addLabel(r, l, -l.W/2, -l.H/2+n.Padding/1.5)
	s.intersect = func(bx box, p P) P {
		pos := intersectRect(bx, p)
		x := pos.X - bx.X
		if rx != 0 && (math.Abs(x) < bx.W/2 || (math.Abs(x) == bx.W/2 && math.Abs(pos.Y-bx.Y) > bx.H/2-ry)) {
			y := ry * ry * (1 - x*x/(rx*rx))
			if y > 0 {
				y = math.Sqrt(y)
			}
			y = ry - y
			if p.Y-bx.Y > 0 {
				y = -y
			}
			pos.Y += y
		}
		return pos
	}
	return s
}

func hexagon(r *renderer, n *nstate) *shape {
	f := 4.0
	px, py := n.Padding, n.Padding
	if n.look == "neo" {
		f, px, py = 3.5, 70, 32
	}
	fixedW, fixedH := n.Width, n.Height
	if fixedW != 0 || fixedH != 0 {
		m := fixedH / f
		fixedW = fixedW - 2*m - py
		fixedH = fixedH - px
	}
	l := r.nodeLabel(n, false)
	h := l.H
	if fixedH != 0 {
		h = fixedH
	}
	h += px
	m := h / f
	w := l.W
	if fixedW != 0 {
		w = fixedW
	}
	w += 2*m + py
	pts := []P{{X: m, Y: 0}, {X: w - m, Y: 0}, {X: w, Y: -h / 2}, {X: w - m, Y: -h}, {X: m, Y: -h}, {X: 0, Y: -h / 2}}
	s := newShape()
	s.add(poly(pts, -w/2, h/2), r.nodePaint(n, pPolygon))
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = func(b box, p P) P { return intersectPolygon(b, pts, p) }
	return s
}

func circleShape(r *renderer, n *nstate) *shape {
	l := r.nodeLabel(n, true)
	lr := math.Sqrt(l.W*l.W+l.H*l.H) / 2
	rad := lr + n.Padding/2
	if n.look == "neo" {
		rad = lr + 16
	}
	s := newShape()
	s.add(scene.Ellipse(0, 0, rad, rad), r.nodePaint(n, pCircle))
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = func(b box, p P) P { return intersectEllipse(b, rad, rad, p) }
	return s
}

// smallShadow is the small drop shadow state dots get (neo, nodeShadow).
func (r *renderer) smallShadow(n *nstate, w float64) *scene.Shadow {
	if w >= 25 || n.look != "neo" || r.th.Get("nodeShadow") != "true" {
		return nil
	}
	c := scene.Color{A: 0.06}
	if r.sheet.shadow != nil {
		c = r.sheet.shadow.Color
	}
	return &scene.Shadow{DX: 2, DY: 2, Color: c}
}

func stateStart(r *renderer, n *nstate) *shape {
	w, h := math.Max(n.Width, 14), math.Max(n.Height, 14)
	if n.Width == 0 {
		w = 14
	}
	if n.Height == 0 {
		h = 14
	}
	p := paint{Fill: r.sheet.nodeFill, Stroke: r.sheet.nodeStroke, Width: r.sheet.nodeWidth}
	if r.sheet.kind == "state" {
		p.Fill, p.Stroke = r.sheet.startFill, r.sheet.startFill
	}
	p.Shadow = r.smallShadow(n, w)
	s := newShape()
	s.add(scene.Ellipse(0, 0, w/2, w/2), p)
	_ = h
	s.intersect = func(b box, q P) P { return intersectEllipse(b, w/2, w/2, q) }
	return s
}

func stateEnd(r *renderer, n *nstate) *shape {
	w := math.Max(n.Width, 14)
	if n.Width == 0 {
		w = 14
	}
	css, _ := nodeStyles(n.Node)
	outer := paint{Fill: r.sheet.nodeFill, Stroke: r.color("lineColor"), Width: 2}
	innerFill := r.color("stateBorder")
	if innerFill.IsNone() {
		innerFill = r.color("nodeBorder")
	}
	inner := paint{Fill: innerFill, Stroke: innerFill, Width: 2}
	// The CSS for node paths wins over the roughjs attributes.
	outer = r.sheet.nodePathCSS(outer)
	inner = r.sheet.nodePathCSS(inner)
	outer, inner = outer.withCSS(css...), inner.withCSS(css...)
	outer.Shadow = r.smallShadow(n, w)
	inner.Shadow = outer.Shadow
	s := newShape()
	s.add(scene.Ellipse(0, 0, w/2, w/2), outer)
	ir := w * 5 / 14 / 2
	s.add(scene.Ellipse(0, 0, ir, ir), inner)
	s.intersect = func(b box, q P) P { return intersectEllipse(b, w/2, w/2, q) }
	return s
}

func choice(r *renderer, n *nstate) *shape {
	sz := math.Max(28, n.Width)
	pts := []P{{X: 0, Y: sz / 2}, {X: sz / 2, Y: 0}, {X: 0, Y: -sz / 2}, {X: -sz / 2, Y: 0}}
	s := newShape()
	s.add(poly(pts, 0, 0), r.nodePaint(n, pPath))
	s.bbox = emptyRect()
	s.bbox.addRect(scene.Rect{X: -14, Y: -14, W: 28, H: 28})
	s.intersect = func(b box, p P) P { return intersectPolygon(b, pts, p) }
	return s
}

func forkJoin(r *renderer, n *nstate) *shape {
	w, h := math.Max(70, n.Width), math.Max(10, n.Height)
	if n.dir == "LR" || n.dir == "RL" {
		w, h = math.Max(10, n.Width), math.Max(70, n.Height)
	}
	// State bars are solid; elsewhere the node rule paints them.
	p := paint{Fill: r.sheet.nodeFill, Stroke: r.sheet.nodeStroke, Width: r.sheet.nodeWidth}
	if r.sheet.kind == "state" {
		p = paint{Fill: r.sheet.startFill, Stroke: r.sheet.startFill, Width: 1}
	}
	css, _ := nodeStyles(n.Node)
	s := newShape()
	s.add(scene.RectPath(-w/2, -h/2, w, h, 0, 0), p.withCSS(css...))
	if pad := r.cfg.Num("state.padding", 0); n.Width != 0 && n.Height != 0 {
		s.bbox.x1 += pad / 2
		s.bbox.y1 += pad / 2
	}
	s.intersect = intersectRect
	return s
}

func note(r *renderer, n *nstate) *shape {
	l := r.nodeLabel(n, false)
	w := math.Max(l.W+n.Padding*2, n.Width)
	h := math.Max(l.H+n.Padding*2, n.Height)
	// Neo's node path rule strokes it like the other nodes.
	p := paint{Fill: r.sheet.noteFill, Stroke: r.sheet.noteStroke, Width: 1}
	if n.look == "neo" {
		p.Stroke, p.Width, p.Shadow = r.sheet.nodeStroke, r.sheet.nodeWidth, r.sheet.shadow
	}
	css, _ := nodeStyles(n.Node)
	p = p.withCSS(css...)
	s := newShape()
	s.add(scene.RectPath(-w/2, -h/2, w, h, 0, 0), p)
	s.addLabel(r, l, -l.W/2, -l.H/2)
	s.intersect = intersectRect
	return s
}

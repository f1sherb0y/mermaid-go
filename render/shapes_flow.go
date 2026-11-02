package render

import (
	"fmt"
	"math"

	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

// The rest of mermaid's flowchart shapes (shapes/*.ts), classic and neo
// looks. What roughjs draws with roughness 0 for the neo look is the plain
// polygon or path here. Icons and images are not fetched: their frames and
// labels are drawn where mermaid draws them and the icon or image area is
// left empty.

func init() {
	reg := func(fn shapeFn, names ...string) {
		for _, n := range names {
			shapes[n] = fn
		}
	}
	reg(slanted(leanR), "lean-r", "lean-right", "in-out", "lean_right")
	reg(slanted(leanL), "lean-l", "lean-left", "out-in", "lean_left")
	reg(slanted(trapB), "trap-b", "priority", "trapezoid-bottom", "trapezoid")
	reg(slanted(trapT), "trap-t", "manual", "trapezoid-top", "inv-trapezoid", "inv_trapezoid")
	reg(rectLeftInvArrow, "odd", "rect_left_inv_arrow")
	reg(doubleCircle, "dbl-circ", "double-circle", "doublecircle")
	reg(notchedRect, "notch-rect", "card", "notched-rectangle")
	reg(shadedProcess, "lin-rect", "lined-rectangle", "lined-process", "lin-proc", "shaded-process")
	reg(hourglass, "hourglass", "collate")
	reg(braceLeft, "brace", "comment", "brace-l")
	reg(braceRight, "brace-r")
	reg(braces, "braces")
	reg(lightningBolt, "bolt", "com-link", "lightning-bolt")
	reg(waveEdgedRect, "doc", "document")
	reg(halfRoundedRect, "delay", "half-rounded-rectangle")
	reg(tiltedCylinder, "h-cyl", "das", "horizontal-cylinder")
	reg(linedCylinder, "lin-cyl", "disk", "lined-cylinder")
	reg(curvedTrapezoid, "curv-trap", "curved-trapezoid", "display")
	reg(dividedRect, "div-rect", "div-proc", "divided-rectangle", "divided-process")
	reg(triangle, "tri", "extract", "triangle")
	reg(flippedTriangle, "flip-tri", "manual-file", "flipped-triangle")
	reg(windowPane, "win-pane", "internal-storage", "window-pane")
	reg(filledCircle, "f-circ", "junction", "filled-circle")
	reg(crossedCircle, "cross-circ", "summary", "crossed-circle")
	reg(notchedPentagon, "notch-pent", "loop-limit", "notched-pentagon")
	reg(slopedRect, "sl-rect", "manual-input", "sloped-rectangle")
	reg(stackedDocument, "docs", "documents", "st-doc", "stacked-document")
	reg(stackedRect, "st-rect", "procs", "processes", "stacked-rectangle")
	reg(bowTieRect, "bow-rect", "stored-data", "bow-tie-rectangle")
	reg(taggedDocument, "tag-doc", "tagged-document")
	reg(taggedRect, "tag-rect", "tagged-rectangle", "tag-proc", "tagged-process")
	reg(waveRect, "flag", "paper-tape")
	reg(linedDocument, "lin-doc", "lined-document")
	reg(datastore, "datastore", "data-store")
	reg(folder, "folder", "directory")
	reg(bucket, "bucket")
	reg(consoleWindow, "console")
	reg(browserWindow, "browser")
	reg(person, "person")
	reg(bang, "bang")
	reg(cloud, "cloud")
	reg(anchorShape, "anchor")
	reg(collapsedGroup, "collapsedGroup")
	reg(iconShape(iconPlain), "icon")
	reg(iconShape(iconSquare), "iconSquare")
	reg(iconShape(iconRounded), "iconRounded")
	reg(iconShape(iconCircle), "iconCircle")
	reg(imageSquare, "imageSquare")
}

// Helpers.

// labelAt is labelHelper's label when the shape set the node's width
// first: wrapped at that width, with no minimum width. 0 is the node's own.
func labelAt(r *renderer, n *nstate, width float64) *Label {
	if width == 0 {
		return r.nodeLabel(n, false)
	}
	_, lcss := nodeStyles(n.Node)
	return r.layoutLabel(n.Label, r.sheet.font.withCSS(lcss...), labelOpts{Width: width, Markdown: n.LabelType == "markdown"})
}

// put draws a label with its box's top left at (x, y), not counting in the
// node's box (mermaid sizes these shapes by their outline only).
func (s *shape) put(r *renderer, l *Label, x, y float64) {
	if l != nil {
		s.items = append(s.items, r.drawLabel(l, x, y)...)
	}
}

// pick is the neo look's value or the classic one.
func pick(n *nstate, neo, classic float64) float64 {
	if n.look == "neo" {
		return neo
	}
	return classic
}

// orUnset is v, or def when v is unset (0).
func orUnset(v, def float64) float64 {
	if v != 0 {
		return v
	}
	return def
}

// openPath is an open path through pts, moved by (dx, dy).
func openPath(pts []P, dx, dy float64) []scene.Seg {
	moved := make([]P, len(pts))
	for i, p := range pts {
		moved[i] = P{X: p.X + dx, Y: p.Y + dy}
	}
	return polyline(moved)
}

// svgPath is SVG path data built with fmt, moved by (dx, dy).
func svgPath(dx, dy float64, format string, args ...any) []scene.Seg {
	return translate(parsePath(fmt.Sprintf(format, args...)), dx, dy)
}

// sineWave is generateFullSineWavePoints: a sine of the given amplitude and
// number of cycles along the segment, in 50 steps.
func sineWave(x1, y1, x2, y2, amp, cycles float64) []P {
	const steps = 50
	dx := x2 - x1
	freq := 2 * math.Pi / (dx / cycles)
	mid := y1 + (y2-y1)/2
	pts := make([]P, steps+1)
	for i := range pts {
		x := x1 + float64(i)/steps*dx
		pts[i] = P{X: x, Y: mid + amp*math.Sin(freq*(x-x1))}
	}
	return pts
}

func polygonIntersect(pts []P) func(b box, p P) P {
	return func(b box, p P) P { return intersectPolygon(b, pts, p) }
}

func circleIntersect(rad float64) func(b box, p P) P {
	return func(b box, p P) P { return intersectEllipse(b, rad, rad, p) }
}

// The slanted quadrilaterals.

type slant int

const (
	leanR slant = iota
	leanL
	trapB
	trapT
)

func slanted(k slant) shapeFn {
	return func(r *renderer, n *nstate) *shape {
		py := n.Padding
		px := pick(n, 2*py, py)
		if k == trapT {
			px, py = 2*px, 2*py
		}
		l := r.nodeLabel(n, false)
		h := math.Max(l.H+py, n.Height)
		w := math.Max(l.W+px, n.Width-h)
		d := h / 2
		var pts []P
		switch k {
		case leanR:
			pts = []P{{X: -d}, {X: w}, {X: w + d, Y: -h}, {Y: -h}}
		case leanL:
			pts = []P{{}, {X: w + d}, {X: w, Y: -h}, {X: -d, Y: -h}}
		case trapB:
			pts = []P{{X: -d}, {X: w + d}, {X: w, Y: -h}, {Y: -h}}
		case trapT:
			pts = []P{{}, {X: w}, {X: w + d, Y: -h}, {X: -d, Y: -h}}
		}
		s := newShape()
		s.add(poly(pts, -w/2, h/2), r.nodePaint(n, pPolygon))
		s.put(r, l, -l.W/2, -l.H/2)
		s.intersect = polygonIntersect(pts)
		return s
	}
}

func rectLeftInvArrow(r *renderer, n *nstate) *shape {
	neo := n.look == "neo"
	px, py := pick(n, 21, n.Padding), pick(n, 12, n.Padding)
	if neo {
		px, py = 2*px, 2*py
	}
	l := r.nodeLabel(n, false)
	h := math.Max(l.H+py, n.Height)
	w := math.Max(l.W+px, n.Width-h/4)
	x, y := -w/2, -h/2
	notch := y / 2
	pts := []P{{X: x + notch, Y: y}, {X: x}, {X: x + notch, Y: -y}, {X: -x, Y: -y}, {X: -x, Y: y}}
	s := newShape()
	s.add(poly(pts, -notch/2, 0), r.nodePaint(n, pOuterPath))
	s.put(r, l, -notch/2-l.W/2, -l.H/2)
	s.intersect = polygonIntersect(pts)
	return s
}

// Circles.

func doubleCircle(r *renderer, n *nstate) *shape {
	gap := pick(n, 12, 5)
	l := r.nodeLabel(n, true)
	inner := math.Hypot(l.W, l.H)/2 + pick(n, 16, n.Padding)
	if n.Width != 0 {
		inner = n.Width/2 + pick(n, 16, n.Padding)
	}
	outer := inner + gap
	s := newShape()
	p := r.nodePaint(n, pCircle)
	s.add(scene.Ellipse(0, 0, outer, outer), p)
	s.add(scene.Ellipse(0, 0, inner, inner), p)
	s.put(r, l, -l.W/2, -l.H/2)
	s.intersect = circleIntersect(outer)
	return s
}

func filledCircle(r *renderer, n *nstate) *shape {
	const rad = 7
	p := r.nodePaint(n, pPath)
	if css, _ := nodeStyles(n.Node); len(css) == 0 {
		p.Fill = r.sheet.nodeStroke
	}
	s := newShape()
	s.add(scene.Ellipse(0, 0, rad, rad), p)
	s.intersect = circleIntersect(rad)
	return s
}

func crossedCircle(r *renderer, n *nstate) *shape {
	rad := math.Max(30, n.Width)
	p := r.nodePaint(n, pOuterPath)
	d := rad * math.Cos(math.Pi/4)
	s := newShape()
	s.add(scene.Ellipse(0, 0, rad, rad), p)
	s.add(svgPath(0, 0, "M %v,%v L %v,%v M %v,%v L %v,%v", -d, d, d, -d, d, d, -d, -d), p)
	s.intersect = circleIntersect(rad)
	return s
}

func anchorShape(r *renderer, n *nstate) *shape {
	lc := r.color("lineColor")
	s := newShape()
	s.add(scene.Ellipse(0, 0, 1, 1), paint{Fill: lc})
	s.intersect = circleIntersect(1)
	return s
}

// Rectangles with a twist.

func notchedRect(r *renderer, n *nstate) *shape {
	const notch = 12
	l := r.nodeLabel(n, false)
	w := orUnset(n.Width, l.W) + pick(n, 56, n.Padding+notch)
	h := orUnset(n.Height, l.H) + pick(n, 48, n.Padding)
	pts := []P{{X: notch, Y: -h}, {X: w, Y: -h}, {X: w}, {}, {Y: -h + notch}, {X: notch, Y: -h}}
	s := newShape()
	s.add(poly(pts, -w/2, h/2), r.nodePaint(n, pPolygon))
	s.put(r, l, -l.W/2, -l.H/2)
	s.intersect = polygonIntersect(pts)
	return s
}

func shadedProcess(r *renderer, n *nstate) *shape {
	const frame = 8
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	l := r.nodeLabel(n, false)
	tw := orUnset(n.Width, l.W) + 2*px + pick(n, frame, 2*frame)
	th := orUnset(n.Height, l.H) + 2*py
	w, h := tw-frame, th
	x, y := frame-tw/2, -th/2
	pts := []P{{X: x, Y: y}, {X: x + w, Y: y}, {X: x + w, Y: y + h}, {X: x - frame, Y: y + h}, {X: x - frame, Y: y}, {X: x, Y: y}, {X: x, Y: y + h}}
	s := newShape()
	s.add(poly(pts, 0, 0), r.nodePaint(n, pOuterPath))
	s.put(r, l, frame/2-l.W/2, -l.H/2)
	s.intersect = intersectRect
	return s
}

func dividedRect(r *renderer, n *nstate) *shape {
	pad := pick(n, 16, n.Padding)
	l := r.nodeLabel(n, false)
	w, h := l.W+pad, l.H+pad
	off := h * 0.2
	x, y := -w/2, -h/2-off/2
	pts := []P{{X: x, Y: y + off}, {X: -x, Y: y + off}, {X: -x, Y: -y}, {X: x, Y: -y}, {X: x, Y: y}, {X: -x, Y: y}, {X: -x, Y: y + off}}
	s := newShape()
	s.add(poly(pts, 0, 0), r.nodePaint(n, pOuterPath))
	s.put(r, l, x+n.Padding/2, y+off+n.Padding/2)
	s.intersect = intersectRect
	return s
}

func windowPane(r *renderer, n *nstate) *shape {
	const o = 10
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nw, nh = math.Max(n.Width-2*px-o, 10), math.Max(n.Height-2*py-o, 10)
	}
	l := labelAt(r, n, nw)
	w := orUnset(nw, l.W) + 2*px
	h := orUnset(nh, l.H) + 2*py
	x, y := -w/2, -h/2
	outer := []P{{X: x - o, Y: y - o}, {X: x - o, Y: y + h}, {X: x + w, Y: y + h}, {X: x + w, Y: y - o}}
	s := newShape()
	s.add(svgPath(o/2, o/2, "M%v,%v L%v,%v L%v,%v L%v,%v L%v,%v M%v,%v L%v,%v M%v,%v L%v,%v",
		x-o, y-o, x+w, y-o, x+w, y+h, x-o, y+h, x-o, y-o, x-o, y, x+w, y, x, y-o, x, y+h), r.nodePaint(n, pOuterPath))
	s.put(r, l, -l.W/2+o/2, -l.H/2+o/2)
	s.intersect = polygonIntersect(outer)
	return s
}

func notchedPentagon(r *renderer, n *nstate) *shape {
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nw, nh = math.Max(n.Width-2*px, 15), math.Max(n.Height-2*py, 5)
	}
	l := labelAt(r, n, nw)
	w := orUnset(nw, l.W) + 2*px
	h := orUnset(nh, l.H) + 2*py
	pts := []P{{X: -w / 2 * 0.8, Y: -h / 2}, {X: w / 2 * 0.8, Y: -h / 2}, {X: w / 2, Y: -h / 2 * 0.6}, {X: w / 2, Y: h / 2}, {X: -w / 2, Y: h / 2}, {X: -w / 2, Y: -h / 2 * 0.6}}
	s := newShape()
	s.add(poly(pts, 0, 0), r.nodePaint(n, pOuterPath))
	s.put(r, l, -l.W/2, -l.H/2)
	s.intersect = polygonIntersect(pts)
	return s
}

func slopedRect(r *renderer, n *nstate) *shape {
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nw, nh = math.Max(n.Width-2*px, 10), math.Max(n.Height/1.5-2*py, 10)
	}
	l := labelAt(r, n, nw)
	w := orUnset(nw, l.W) + 2*px
	h := orUnset(nh, l.H) + 2*py
	x, y := -w/2, -h/2
	pts := []P{{X: x, Y: y}, {X: x, Y: y + h}, {X: x + w, Y: y + h}, {X: x + w, Y: y - h/2}}
	s := newShape()
	s.add(poly(pts, 0, h/4), r.nodePaint(n, pOuterPath))
	s.put(r, l, -w/2+n.Padding, -h/4+n.Padding)
	s.intersect = polygonIntersect(pts)
	return s
}

func stackedRect(r *renderer, n *nstate) *shape {
	px, py, o := pick(n, 16, n.Padding), pick(n, 12, n.Padding), pick(n, 10, 5)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nw, nh = math.Max(n.Width-2*px-2*o, 10), math.Max(n.Height-2*py-2*o, 10)
	}
	l := labelAt(r, n, nw)
	w := orUnset(nw, l.W) + 2*px
	h := orUnset(nh, l.H) + 2*py
	x, y := -w/2, -h/2
	outer := []P{
		{X: x - o, Y: y + o}, {X: x - o, Y: y + h + o}, {X: x + w - o, Y: y + h + o}, {X: x + w - o, Y: y + h},
		{X: x + w, Y: y + h}, {X: x + w, Y: y + h - o}, {X: x + w + o, Y: y + h - o}, {X: x + w + o, Y: y - o},
		{X: x + o, Y: y - o}, {X: x + o, Y: y}, {X: x, Y: y}, {X: x, Y: y + o},
	}
	inner := []P{{X: x, Y: y + o}, {X: x + w - o, Y: y + o}, {X: x + w - o, Y: y + h}, {X: x + w, Y: y + h}, {X: x + w, Y: y}, {X: x, Y: y}}
	p := r.nodePaint(n, pOuterPath)
	s := newShape()
	s.add(poly(outer, 0, 0), p)
	s.add(poly(inner, 0, 0), p)
	s.put(r, l, -l.W/2-o, -l.H/2+o)
	s.intersect = polygonIntersect(outer)
	return s
}

func taggedRect(r *renderer, n *nstate) *shape {
	const tag = 0.2
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nh = math.Max(n.Height-2*py, 10)
		nw = math.Max(n.Width-2*px-tag*(nh+2*py), 10)
	}
	l := labelAt(r, n, nw)
	h := orUnset(nh, l.H) + 2*py
	tw, th := tag*h, tag*h
	w := orUnset(nw, l.W) + 2*px
	x, y := -w/2, -h/2
	rect := []P{{X: x - tw/2, Y: y}, {X: x + w + tw/2, Y: y}, {X: x + w + tw/2, Y: y + h}, {X: x - tw/2, Y: y + h}}
	tagPts := []P{{X: x + w - tw/2, Y: y + h}, {X: x + w + tw/2, Y: y + h}, {X: x + w + tw/2, Y: y + h - th}}
	p := r.nodePaint(n, pOuterPath)
	s := newShape()
	s.add(poly(rect, 0, 0), p)
	s.add(poly(tagPts, 0, 0), p)
	s.put(r, l, -l.W/2, -l.H/2)
	s.intersect = polygonIntersect(rect)
	return s
}

func halfRoundedRect(r *renderer, n *nstate) *shape {
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nw, nh = math.Max(n.Width-2*px, 15), math.Max(n.Height-2*py, 10)
	}
	l := labelAt(r, n, nw)
	h := orUnset(nh, math.Max(10, l.H)) + 2*py
	rad := h / 2
	clear := rad - math.Sqrt(math.Max(0, rad*rad-l.H*l.H/4))
	w := orUnset(nw, math.Max(15, l.W)+2*clear) + 2*px
	pts := []P{{X: -w / 2, Y: -h / 2}, {X: w/2 - rad, Y: -h / 2}}
	pts = append(pts, circlePoints(-w/2+rad, 0, rad, 50, 90, 270)...)
	pts = append(pts, P{X: w/2 - rad, Y: h / 2}, P{X: -w / 2, Y: h / 2})
	s := newShape()
	s.add(poly(pts, 0, 0), r.nodePaint(n, pOuterPath))
	s.put(r, l, -l.W/2, -l.H/2)
	s.intersect = polygonIntersect(pts)
	return s
}

func curvedTrapezoid(r *renderer, n *nstate) *shape {
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	l := r.nodeLabel(n, false)
	h := math.Max(5, math.Max(l.H+2*py, n.Height))
	rad := h / 2
	clear := rad - math.Sqrt(math.Max(0, rad*rad-l.H*l.H/4))
	side := math.Max(h/4, clear)
	w := math.Max(math.Max(20, l.W+2*px+2*side), math.Max((l.W+2*px)*1.25, n.Width))
	rw, tw := w-rad, h/4
	pts := []P{{X: rw}, {X: tw}, {Y: h / 2}, {X: tw, Y: h}, {X: rw, Y: h}}
	pts = append(pts, circlePoints(-rw, -h/2, rad, 50, 270, 90)...)
	s := newShape()
	s.add(poly(pts, -w/2, -h/2), r.nodePaint(n, pOuterPath))
	s.put(r, l, -l.W/2, -l.H/2)
	s.intersect = polygonIntersect(pts)
	return s
}

func bowTieRect(r *renderer, n *nstate) *shape {
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	l := r.nodeLabel(n, false)
	h := orUnset(n.Height, l.H) + py
	ry := h / 2
	rx := ry / (2.5 + h/50)
	w := orUnset(n.Width, l.W) + 2*px
	pts := []P{{X: w / 2, Y: -h / 2}, {X: -w / 2, Y: -h / 2}}
	pts = append(pts, arcPoints(-w/2, -h/2, -w/2, h/2, rx, ry, false)...)
	pts = append(pts, P{X: w / 2, Y: h / 2})
	pts = append(pts, arcPoints(w/2, h/2, w/2, -h/2, rx, ry, true)...)
	s := newShape()
	s.add(poly(pts, rx/2, 0), r.nodePaint(n, pOuterPath))
	s.put(r, l, -l.W/2, -l.H/2)
	s.intersect = polygonIntersect(pts)
	return s
}

// arcPoints are 20 points on the elliptical arc from (x1, y1) to (x2, y2)
// (bowTieRect's generateArcPoints).
func arcPoints(x1, y1, x2, y2, rx, ry float64, clockwise bool) []P {
	const n = 20
	mx, my := (x1+x2)/2, (y1+y2)/2
	angle := math.Atan2(y2-y1, x2-x1)
	d := math.Hypot((x2-x1)/2/rx, (y2-y1)/2/ry)
	c := math.Sqrt(math.Max(0, 1-d*d))
	dir := 1.0
	if clockwise {
		dir = -1
	}
	cx := mx + c*ry*math.Sin(angle)*dir
	cy := my - c*rx*math.Cos(angle)*dir
	a0 := math.Atan2((y1-cy)/ry, (x1-cx)/rx)
	a1 := math.Atan2((y2-cy)/ry, (x2-cx)/rx)
	span := a1 - a0
	if clockwise && span < 0 {
		span += 2 * math.Pi
	}
	if !clockwise && span > 0 {
		span -= 2 * math.Pi
	}
	pts := make([]P, n)
	for i := range pts {
		a := a0 + float64(i)/(n-1)*span
		pts[i] = P{X: cx + rx*math.Cos(a), Y: cy + ry*math.Sin(a)}
	}
	return pts
}

// Triangles.

func triangle(r *renderer, n *nstate) *shape {
	lpx := pick(n, 2*n.Padding, n.Padding)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nw, nh = math.Max((n.Width-lpx)/2, 10), math.Max(n.Height, 10)
	}
	l := labelAt(r, n, nw)
	w := orUnset(nw, l.W) + lpx
	h := orUnset(nh, w+l.H)
	pts := []P{{}, {X: h}, {X: h / 2, Y: -h}}
	s := newShape()
	s.add(poly(pts, -h/2, h/2), r.nodePaint(n, pOuterPath))
	s.put(r, l, -l.W/2, h/2-(l.H+n.Padding/2))
	s.intersect = polygonIntersect(pts)
	return s
}

func flippedTriangle(r *renderer, n *nstate) *shape {
	lpx := pick(n, 2*n.Padding, n.Padding)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nh, nw = math.Max(n.Height, 10), math.Max(n.Width-lpx*1.5, 10)
	}
	l := labelAt(r, n, nw)
	w := orUnset(nw, l.W) + lpx
	h := orUnset(nh, w+l.H)
	pts := []P{{Y: -h}, {X: h, Y: -h}, {X: h / 2}}
	s := newShape()
	s.add(poly(pts, -h/2, h/2), r.nodePaint(n, pOuterPath))
	s.put(r, l, -l.W/2, -h/2+n.Padding/2)
	s.intersect = polygonIntersect(pts)
	return s
}

func hourglass(r *renderer, n *nstate) *shape {
	w, h := math.Max(30, n.Width), math.Max(30, n.Height)
	pts := []P{{}, {X: w}, {Y: h}, {X: w, Y: h}}
	s := newShape()
	s.add(poly(pts, -w/2, -h/2), r.nodePaint(n, pOuterPath))
	s.intersect = polygonIntersect(pts)
	return s
}

func lightningBolt(r *renderer, n *nstate) *shape {
	w, h := math.Max(35, n.Width), math.Max(35, n.Height)
	const gap = 7.0
	pts := []P{{X: w}, {Y: h + gap/2}, {X: w - 2*gap, Y: h + gap/2}, {Y: 2 * h}, {X: w, Y: h - gap/2}, {X: 2 * gap, Y: h - gap/2}}
	s := newShape()
	s.add(poly(pts, -w/2, -h), r.nodePaint(n, pOuterPath))
	s.intersect = polygonIntersect(pts)
	return s
}

// Curly braces: open brace strokes; the box around brace and label is
// invisible (the hit area and the node's size).

type braceKind int

const (
	braceL braceKind = iota
	braceR
	braceLR
)

func braceLeft(r *renderer, n *nstate) *shape  { return brace(r, n, braceL) }
func braceRight(r *renderer, n *nstate) *shape { return brace(r, n, braceR) }
func braces(r *renderer, n *nstate) *shape     { return brace(r, n, braceLR) }

func brace(r *renderer, n *nstate, k braceKind) *shape {
	px, py := pick(n, 18, n.Padding), pick(n, 12, n.Padding)
	if k != braceL && n.look == "neo" {
		px, py = 2*px, 2*py
	}
	l := r.nodeLabel(n, false)
	w, h := l.W+px, l.H+py
	rad := math.Max(5, h*0.1)
	cp := circlePoints
	cat := func(parts ...[]P) []P {
		var out []P
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	left := cat(cp(w/2, -h/2, rad, 30, -90, 0), []P{{X: -w/2 - rad, Y: rad}},
		cp(w/2+rad*2, -rad, rad, 20, -180, -270), cp(w/2+rad*2, rad, rad, 20, -90, -180),
		[]P{{X: -w/2 - rad, Y: -h / 2}}, cp(w/2, h/2, rad, 20, 0, 90))
	leftBox := cat([]P{{X: w / 2, Y: -h/2 - rad}, {X: -w / 2, Y: -h/2 - rad}}, cp(w/2, -h/2, rad, 20, -90, 0),
		[]P{{X: -w/2 - rad, Y: -rad}}, cp(w/2+w*0.1, -rad, rad, 20, -180, -270), cp(w/2+w*0.1, rad, rad, 20, -90, -180),
		[]P{{X: -w/2 - rad, Y: h / 2}}, cp(w/2, h/2, rad, 20, 0, 90), []P{{X: -w / 2, Y: h/2 + rad}, {X: w / 2, Y: h/2 + rad}})
	var strokes [][]P
	var box []P
	var dx, lx float64
	ly := -h/2 + n.Padding/2
	switch k {
	case braceL:
		strokes, box, dx, lx = [][]P{left}, leftBox, rad, -w/2+rad
	case braceR:
		// The right brace's arcs are not mirrored (its own generateCirclePoints).
		cp = func(cx, cy, r float64, n int, a0, a1 float64) []P {
			pts := circlePoints(cx, cy, r, n, a0, a1)
			for i := range pts {
				pts[i] = P{X: -pts[i].X, Y: -pts[i].Y}
			}
			return pts
		}
		right := cat(cp(w/2, -h/2, rad, 20, -90, 0), []P{{X: w/2 + rad, Y: -rad}},
			cp(w/2+rad*2, -rad, rad, 20, -180, -270), cp(w/2+rad*2, rad, rad, 20, -90, -180),
			[]P{{X: w/2 + rad, Y: h / 2}}, cp(w/2, h/2, rad, 20, 0, 90))
		box = cat([]P{{X: -w / 2, Y: -h/2 - rad}, {X: w / 2, Y: -h/2 - rad}}, cp(w/2, -h/2, rad, 20, -90, 0),
			[]P{{X: w/2 + rad, Y: -rad}}, cp(w/2+rad*2, -rad, rad, 20, -180, -270), cp(w/2+rad*2, rad, rad, 20, -90, -180),
			[]P{{X: w/2 + rad, Y: h / 2}}, cp(w/2, h/2, rad, 20, 0, 90), []P{{X: w / 2, Y: h/2 + rad}, {X: -w / 2, Y: h/2 + rad}})
		strokes, dx, lx = [][]P{right}, -rad, -w/2+n.Padding/2
	case braceLR:
		rb := cat(cp(-w/2+rad+rad/2, -h/2, rad, 20, -90, -180), []P{{X: w/2 - rad/2, Y: rad}},
			cp(-w/2-rad/2, -rad, rad, 20, 0, 90), cp(-w/2-rad/2, rad, rad, 20, -90, 0),
			[]P{{X: w/2 - rad/2, Y: -rad}}, cp(-w/2+rad+rad/2, h/2, rad, 30, -180, -270))
		box = cat([]P{{X: w / 2, Y: -h/2 - rad}, {X: -w / 2, Y: -h/2 - rad}}, cp(w/2, -h/2, rad, 20, -90, 0),
			[]P{{X: -w/2 - rad, Y: -rad}}, cp(w/2+rad*2, -rad, rad, 20, -180, -270), cp(w/2+rad*2, rad, rad, 20, -90, -180),
			[]P{{X: -w/2 - rad, Y: h / 2}}, cp(w/2, h/2, rad, 20, 0, 90), []P{{X: -w / 2, Y: h/2 + rad}, {X: w/2 - rad - rad/2, Y: h/2 + rad}}, rb)
		// mermaid inserts each at the front: the right brace paints first.
		strokes, dx, lx = [][]P{rb, left}, rad-rad/4, -w/2+n.Padding/2
	}
	// roughjs draws no fill for these (fill: none) and hides the box's
	// stroke: only the braces show; the box counts in the node's size.
	p := r.nodePaint(n, pPath)
	p.Fill = scene.None
	s := newShape()
	for _, st := range strokes {
		s.add(openPath(st, dx, 0), p)
	}
	s.bbox.addRect(pathBounds(poly(box, dx, 0)))
	s.put(r, l, lx, ly)
	s.intersect = polygonIntersect(box)
	return s
}

// Wavy documents.

func waveEdgedRect(r *renderer, n *nstate) *shape {
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	adjust := true
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		adjust = false
		nw, nh = math.Max(n.Width-2*px, 10), math.Max(n.Height-2*py, 10)
	}
	l := labelAt(r, n, nw)
	w := orUnset(nw, l.W) + 2*px
	h := orUnset(nh, l.H) + 2*py
	amp := pick(n, h/4, h/8)
	fh := h - amp
	if adjust {
		fh = h + amp
	}
	e := math.Max(0, (14-w)/2)
	pts := append([]P{{X: -w/2 - e, Y: fh / 2}}, sineWave(-w/2-e, fh/2, w/2+e, fh/2, amp, 0.8)...)
	pts = append(pts, P{X: w/2 + e, Y: -fh / 2}, P{X: -w/2 - e, Y: -fh / 2})
	s := newShape()
	s.add(poly(pts, 0, -amp/2), r.nodePaint(n, pOuterPath))
	s.put(r, l, -w/2+n.Padding, -h/2+n.Padding-amp)
	s.intersect = polygonIntersect(pts)
	return s
}

func linedDocument(r *renderer, n *nstate) *shape {
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nw, nh = math.Max(n.Width*10/11-2*px, 10), math.Max(n.Height-2*py, 10)
	}
	l := labelAt(r, n, nw)
	w := orUnset(nw, l.W) + 2*px
	h := orUnset(nh, l.H) + 2*py
	amp := pick(n, h/4, h/8)
	fh := h + amp
	x0, x1 := -w/2-w/2*0.1, w/2+w/2*0.1
	pts := append([]P{{X: x0, Y: -fh / 2}, {X: x0, Y: fh / 2}}, sineWave(x0, fh/2, x1, fh/2, amp, 0.8)...)
	pts = append(pts, P{X: x1, Y: -fh / 2}, P{X: x0, Y: -fh / 2}, P{X: -w / 2, Y: -fh / 2}, P{X: -w / 2, Y: fh / 2 * 1.1}, P{X: -w / 2, Y: -fh / 2})
	s := newShape()
	s.add(poly(pts, 0, -amp/2), r.nodePaint(n, pOuterPath))
	s.put(r, l, -w/2+n.Padding+w/2*0.1/2, -h/2+n.Padding-amp)
	s.intersect = polygonIntersect(pts)
	return s
}

func stackedDocument(r *renderer, n *nstate) *shape {
	px, py := pick(n, 16, n.Padding), pick(n, 12, n.Padding)
	l := r.nodeLabel(n, false)
	adjust := true
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		adjust = false
		nw, nh = n.Width-2*px, n.Height-3*py
	}
	w := math.Max(l.W, nw) + 2*px
	h := math.Max(l.H, nh) + 3*py
	amp := pick(n, h/4, h/8)
	fh := h - amp/2
	if adjust {
		fh = h + amp/2
	}
	x, y := -w/2, -fh/2
	const o = 10
	wave := sineWave(x-o, y+fh+o, x+w-o, y+fh+o, amp, 0.8)
	last := wave[len(wave)-1]
	outer := append([]P{{X: x - o, Y: y + o}, {X: x - o, Y: y + fh + o}}, wave...)
	outer = append(outer,
		P{X: x + w - o, Y: last.Y - o}, P{X: x + w, Y: last.Y - o}, P{X: x + w, Y: last.Y - 2*o},
		P{X: x + w + o, Y: last.Y - 2*o}, P{X: x + w + o, Y: y - o}, P{X: x + o, Y: y - o},
		P{X: x + o, Y: y}, P{X: x, Y: y}, P{X: x, Y: y + o})
	inner := []P{{X: x, Y: y + o}, {X: x + w - o, Y: y + o}, {X: x + w - o, Y: last.Y - o}, {X: x + w, Y: last.Y - o}, {X: x + w, Y: y}, {X: x, Y: y}}
	p := r.nodePaint(n, pOuterPath)
	s := newShape()
	s.add(poly(outer, 0, -amp/2), p)
	s.add(poly(inner, 0, -amp/2), p)
	s.put(r, l, -l.W/2-o, -l.H/2+o-amp)
	s.intersect = polygonIntersect(outer)
	return s
}

func taggedDocument(r *renderer, n *nstate) *shape {
	pad := n.Padding
	l := r.nodeLabel(n, false)
	w := math.Max(l.W+2*pad, n.Width)
	h := math.Max(l.H+2*pad, n.Height)
	amp := h / 8
	tw, th := 0.2*w, 0.2*h
	fh := h + amp
	x0, x1 := -w/2-w/2*0.1, w/2+w/2*0.1
	pts := append([]P{{X: x0, Y: fh / 2}}, sineWave(x0, fh/2, x1, fh/2, amp, 0.8)...)
	pts = append(pts, P{X: x1, Y: -fh / 2}, P{X: x0, Y: -fh / 2})
	x, y := -w/2+w/2*0.1, -fh/2-th*0.4
	tagPts := append([]P{{X: x + w - tw, Y: (y + h) * 1.3}, {X: x + w, Y: y + h - th}, {X: x + w, Y: (y + h) * 0.9}},
		sineWave(x+w, (y+h)*1.25, x+w-tw, (y+h)*1.3, -h*0.02, 0.5)...)
	p := r.nodePaint(n, pOuterPath)
	s := newShape()
	s.add(poly(pts, 0, -amp/2), p)
	s.add(poly(tagPts, 0, -amp/2), p)
	s.put(r, l, -w/2+pad, -h/2+pad-amp/2)
	s.intersect = polygonIntersect(pts)
	return s
}

func waveRect(r *renderer, n *nstate) *shape {
	px, py := pick(n, 16, n.Padding), pick(n, 20, n.Padding)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nw, nh = math.Max(n.Width, 20), math.Max(n.Height, 10)
		amp := math.Min(nh*0.2, nh/4)
		nh = math.Ceil(nh - py - amp*20/9)
		nw -= 2 * px
	}
	l := labelAt(r, n, nw)
	w := orUnset(nw, l.W) + 2*px
	h := orUnset(nh, l.H) + py
	amp := h / 8
	fh := h + 2*amp
	pts := append([]P{{X: -w / 2, Y: fh / 2}}, sineWave(-w/2, fh/2, w/2, fh/2, amp, 1)...)
	pts = append(pts, P{X: w / 2, Y: -fh / 2})
	pts = append(pts, sineWave(w/2, -fh/2, -w/2, -fh/2, amp, -1)...)
	s := newShape()
	s.add(poly(pts, 0, 0), r.nodePaint(n, pPath))
	s.put(r, l, -l.W/2, -l.H/2)
	s.intersect = polygonIntersect(pts)
	return s
}

// Cylinders.

// cylinderIntersect is where a ray meets a cylinder whose rims are
// ellipses of radii rx, ry: vertical (rims at top and bottom) or not.
func cylinderIntersect(rx, ry float64, vertical bool) func(b box, p P) P {
	return func(b box, p P) P {
		pos := intersectRect(b, p)
		if vertical {
			x := pos.X - b.X
			if rx != 0 && (math.Abs(x) < b.W/2 || (math.Abs(x) == b.W/2 && math.Abs(pos.Y-b.Y) > b.H/2-ry)) {
				y := ry * ry * (1 - x*x/(rx*rx))
				if y > 0 {
					y = math.Sqrt(y)
				}
				y = ry - y
				if p.Y-b.Y > 0 {
					y = -y
				}
				pos.Y += y
			}
			return pos
		}
		y := pos.Y - b.Y
		if ry != 0 && (math.Abs(y) < b.H/2 || (math.Abs(y) == b.H/2 && math.Abs(pos.X-b.X) > b.W/2-rx)) {
			x := rx * rx * (1 - y*y/(ry*ry))
			if x != 0 {
				x = math.Sqrt(math.Abs(x))
			}
			x = rx - x
			if p.X-b.X > 0 {
				x = -x
			}
			pos.X += x
		}
		return pos
	}
}

func tiltedCylinder(r *renderer, n *nstate) *shape {
	lp := pick(n, 12, n.Padding/2)
	nw, nh := 0.0, 0.0
	if n.Height != 0 {
		nh = math.Max(n.Height-lp, 5)
	}
	if n.Width != 0 {
		ry := n.Height / 2
		rx := ry / (2.5 + n.Height/50)
		nw = math.Max(n.Width-lp-rx*3, 10)
	}
	l := labelAt(r, n, nw)
	h := math.Max(nh, l.H) + lp
	ry := h / 2
	rx := ry / (2.5 + h/50)
	w := math.Max(nw, l.W) + rx + lp
	s := newShape()
	s.add(svgPath(-w/2, h/2, "M0,0 a%v,%v 0,0,1 0,%v l%v,0 a%v,%v 0,0,1 0,%v M%v,%v a%v,%v 0,0,0 0,%v l%v,0",
		rx, ry, -h, w, rx, ry, h, w, -h, rx, ry, h, -w), r.nodePaint(n, pOuterPath))
	s.put(r, l, -l.W/2-rx, -l.H/2)
	s.intersect = cylinderIntersect(rx, ry, false)
	return s
}

func linedCylinder(r *renderer, n *nstate) *shape {
	px, py := pick(n, 16, n.Padding), pick(n, 24, n.Padding)
	nw, nh := 0.0, 0.0
	if n.Width != 0 || n.Height != 0 {
		nw = math.Max(n.Width-px, 10)
		rx := n.Width / 2
		ry := rx / (2.5 + n.Width/50)
		nh = math.Max(n.Height-py-ry*3, 10)
	}
	l := labelAt(r, n, nw)
	w := orUnset(nw, l.W) + 2*px
	rx := w / 2
	ry := rx / (2.5 + w/50)
	h := orUnset(nh, l.H) + ry + 2*py
	off := h * 0.1
	s := newShape()
	s.add(svgPath(-w/2, -(h/2+ry), "M0,%v a%v,%v 0,0,0 %v,0 a%v,%v 0,0,0 %v,0 l0,%v a%v,%v 0,0,0 %v,0 l0,%v M0,%v a%v,%v 0,0,0 %v,0",
		ry, rx, ry, w, rx, ry, -w, h, rx, ry, w, -h, ry+off, rx, ry, w), r.nodePaint(n, pOuterPath))
	s.put(r, l, -l.W/2, -l.H/2+ry)
	s.intersect = cylinderIntersect(rx, ry, true)
	return s
}

// The shapes that draw plain SVG elements.

func datastore(r *renderer, n *nstate) *shape {
	px := n.extraNum("labelPaddingX")
	if px == 0 {
		px = 2 * n.Padding
	}
	py := n.extraNum("labelPaddingY")
	if py == 0 {
		py = n.Padding
	}
	s := drawRect(r, n, rectOpts{padX: px, padY: py})
	// Only the top and bottom edges are stroked.
	if p, ok := s.items[0].(*scene.Path); ok && p.Dash == nil {
		b := pathBounds(p.Segs)
		if b.W > 0 && b.H > 0 {
			p.Dash = []float64{b.W, b.H}
		}
	}
	return s
}

func folder(r *renderer, n *nstate) *shape {
	pad := orUnset(n.Padding, 12)
	l := r.nodeLabel(n, false)
	w := math.Max(math.Max(l.W+2*pad, n.Width), 90)
	content := l.H + 2*pad
	tabH := math.Max(math.Min(content*0.16, 14), 8)
	th := math.Max(content+tabH, n.Height)
	tabW := math.Max(w*0.38, 28)
	top := -th / 2
	pts := []P{{X: -w / 2, Y: top}, {X: -w/2 + tabW, Y: top}, {X: -w/2 + tabW, Y: top + tabH}, {X: w / 2, Y: top + tabH}, {X: w / 2, Y: th / 2}, {X: -w / 2, Y: th / 2}}
	s := newShape()
	s.add(poly(pts, 0, 0), r.nodePaint(n, pPath))
	s.put(r, l, -l.W/2, top+tabH+(th-tabH)/2-l.H/2)
	s.intersect = polygonIntersect(pts)
	return s
}

func bucket(r *renderer, n *nstate) *shape {
	pad := orUnset(n.Padding, 12)
	l := r.nodeLabel(n, false)
	w := math.Max(math.Max(l.W+2*pad, n.Width), 80)
	rim := math.Max(math.Min(w*0.08, 12), 5)
	th := math.Max(l.H+2*pad+rim, n.Height)
	topY, botY, bw := -th/2+rim, th/2, w*0.72
	s := newShape()
	s.add(svgPath(0, 0, "M%v,%v L%v,%v A%v,%v 0 0 0 %v,%v L%v,%v A%v,%v 0 0 0 %v,%v Z",
		-w/2, topY, -bw/2, botY, bw/2, rim, bw/2, botY, w/2, topY, w/2, rim, -w/2, topY), r.nodePaint(n, pPath))
	s.add(scene.Ellipse(0, topY, w/2, rim), paint{Stroke: r.sheet.nodeStroke, Width: 1})
	s.put(r, l, -l.W/2, topY+(botY-topY)/2-l.H/2)
	arc := func(rx, cy, sign float64) []P {
		const segs = 12
		out := make([]P, segs+1)
		for i := range out {
			t := math.Pi - float64(i)*math.Pi/segs
			out[i] = P{X: rx * math.Cos(t), Y: cy + sign*rim*math.Sin(t)}
		}
		return out
	}
	outline := arc(w/2, topY, -1)
	bottom := arc(bw/2, botY, 1)
	for i := len(bottom) - 1; i >= 0; i-- {
		outline = append(outline, bottom[i])
	}
	s.intersect = polygonIntersect(outline)
	return s
}

// windowFrame is the rounded window of console and browser shapes.
func windowFrame(r *renderer, n *nstate, band float64) (*shape, *Label, float64, float64) {
	pad := orUnset(n.Padding, 12)
	l := r.nodeLabel(n, false)
	w := math.Max(math.Max(l.W+2*pad, n.Width), 90)
	h := math.Max(l.H+2*pad+band, n.Height)
	s := newShape()
	s.add(scene.RectPath(-w/2, -h/2, w, h, 12, 12), r.nodePaint(n, pRect))
	s.intersect = intersectRect
	return s, l, w, h
}

func consoleWindow(r *renderer, n *nstate) *shape {
	const band = 20
	s, l, w, h := windowFrame(r, n, band)
	top := -h / 2
	s.items = append(s.items, &scene.Text{X: -w/2 + 12, Y: top + 16, Family: "monospace", Size: 14,
		Color: r.sheet.nodeStroke, Runs: []scene.Span{{Text: ">_", Weight: 700}}})
	s.put(r, l, -l.W/2, top+band+(h-band)/2-l.H/2)
	return s
}

func browserWindow(r *renderer, n *nstate) *shape {
	const bar = 18
	s, l, w, h := windowFrame(r, n, bar)
	top := -h / 2
	accent := r.sheet.nodeStroke
	s.add(openPath([]P{{X: -w / 2, Y: top + bar}, {X: w / 2, Y: top + bar}}, 0, 0), paint{Stroke: accent, Width: 1})
	dot := r.nodePaint(n, pCircle)
	dot.Fill, dot.Stroke = accent, scene.None
	for i := range 3 {
		s.add(scene.Ellipse(-w/2+12+float64(i)*9, top+bar/2, 2.5, 2.5), dot)
	}
	addr := r.nodePaint(n, pRect)
	addr.Fill, addr.Stroke, addr.Width = scene.None, accent, 1
	addr.Stroke.A *= 0.6
	s.add(scene.RectPath(-w/2+44, top+4, math.Max(w-56, 10), bar-8, 3, 3), addr)
	s.put(r, l, -l.W/2, top+bar+(h-bar)/2-l.H/2)
	return s
}

func person(r *renderer, n *nstate) *shape {
	pad := orUnset(n.Padding, 20)
	l := r.nodeLabel(n, false)
	w := math.Max(math.Max(l.W+2*pad, n.Width), 100)
	head := math.Min(math.Max(w*0.23, 16), 56)
	overlap := head * 0.27
	bodyH := l.H + 2*pad
	if n.Height != 0 {
		bodyH = math.Max(bodyH, n.Height-(2*head-overlap))
	}
	bodyR := math.Min(w*0.177, bodyH*0.45)
	th := bodyH + 2*head - overlap
	top := -th / 2
	bodyTop := top + 2*head - overlap
	s := newShape()
	s.add(scene.RectPath(-w/2, bodyTop, w, bodyH, bodyR, bodyR), r.nodePaint(n, pRect))
	s.add(scene.Ellipse(0, top+head, head, head), r.nodePaint(n, pCircle))
	s.put(r, l, -l.W/2, bodyTop+bodyH/2-l.H/2)
	headY := top + head
	phi := math.Asin(math.Min(1, (bodyTop-headY)/head)) * 180 / math.Pi
	cp := circlePoints
	outline := cp(0, -headY, head, 24, 180+phi, -phi)
	outline = append(outline, cp(-(-w/2+bodyR), -(bodyTop+bodyR), bodyR, 12, 90, 0)...)
	outline = append(outline, cp(-(-w/2+bodyR), -(th/2-bodyR), bodyR, 12, 360, 270)...)
	outline = append(outline, cp(-(w/2-bodyR), -(th/2-bodyR), bodyR, 12, 270, 180)...)
	outline = append(outline, cp(-(w/2-bodyR), -(bodyTop+bodyR), bodyR, 12, 180, 90)...)
	s.intersect = polygonIntersect(outline)
	return s
}

func bang(r *renderer, n *nstate) *shape {
	l := r.nodeLabel(n, false)
	hp := n.Padding / 2
	w := math.Max(l.W+10*hp, l.W+20)
	h := math.Max(l.H+8*hp, l.H+20)
	rad := 0.15 * (l.W + 10*hp)
	s := newShape()
	s.add(svgPath(-w/2, -h/2, "M0 0 a%[1]v,%[1]v 1 0,0 %[2]v,%[3]v a%[1]v,%[1]v 1 0,0 %[2]v,0 a%[1]v,%[1]v 1 0,0 %[2]v,0 "+
		"a%[1]v,%[1]v 1 0,0 %[2]v,%[4]v a%[1]v,%[1]v 1 0,0 %[5]v,%[6]v a%[7]v,%[7]v 1 0,0 0,%[8]v "+
		"a%[1]v,%[1]v 1 0,0 %[9]v,%[6]v a%[1]v,%[1]v 1 0,0 %[10]v,%[11]v a%[1]v,%[1]v 1 0,0 %[10]v,0 "+
		"a%[1]v,%[1]v 1 0,0 %[10]v,0 a%[1]v,%[1]v 1 0,0 %[10]v,%[12]v a%[1]v,%[1]v 1 0,0 %[13]v,%[14]v "+
		"a%[7]v,%[7]v 1 0,0 0,%[15]v a%[1]v,%[1]v 1 0,0 %[16]v,%[14]v H0 V0 Z",
		rad, w*0.25, -h*0.1, h*0.1, w*0.15, h*0.33, rad*0.8, h*0.34, -w*0.15, -w*0.25, h*0.15,
		-h*0.15, -w*0.1, -h*0.33, -h*0.34, w*0.1), r.nodePaint(n, pPath))
	s.put(r, l, -l.W/2, -l.H/2)
	s.intersect = intersectRect
	return s
}

func cloud(r *renderer, n *nstate) *shape {
	l := r.nodeLabel(n, false)
	hp := n.Padding / 2
	w, h := l.W+2*hp, l.H+2*hp
	r1, r2, r3, r4 := 0.15*w, 0.25*w, 0.35*w, 0.2*w
	s := newShape()
	s.add(svgPath(-w/2, -h/2, "M0 0 a%[1]v,%[1]v 0 0,1 %[5]v,%[6]v a%[3]v,%[3]v 1 0,1 %[7]v,%[6]v "+
		"a%[2]v,%[2]v 1 0,1 %[8]v,%[9]v a%[1]v,%[1]v 1 0,1 %[10]v,%[11]v a%[4]v,%[4]v 1 0,1 %[12]v,%[13]v "+
		"a%[2]v,%[1]v 1 0,1 %[14]v,%[10]v a%[3]v,%[3]v 1 0,1 %[15]v,0 a%[1]v,%[1]v 1 0,1 %[14]v,%[12]v "+
		"a%[1]v,%[1]v 1 0,1 %[16]v,%[17]v a%[4]v,%[4]v 1 0,1 %[18]v,%[19]v H0 V0 Z",
		r1, r2, r3, r4, w*0.25, -w*0.1, w*0.4, w*0.35, w*0.2, w*0.15, h*0.35, -w*0.15, h*0.65,
		-w*0.25, -w*0.5, -w*0.1, -h*0.35, w*0.1, -h*0.65), r.nodePaint(n, pPath))
	s.put(r, l, -l.W/2, -l.H/2)
	s.intersect = intersectRect
	return s
}

// collapsedGroup is a subgraph drawn as a node (flowchart's view:
// collapsed): its title, a dashed separator and three dots.
func collapsedGroup(r *renderer, n *nstate) *shape {
	const (
		rowH = 20
		gap  = 8
		minW = 80
		rad  = 8
	)
	l := r.nodeLabel(n, false)
	pad := orUnset(n.Padding, 8)
	w := math.Max(math.Max(l.W+2*pad, minW), n.Width)
	h := math.Max(l.H+gap+rowH+2*pad, n.Height)
	x, y := -w/2, -h/2
	rect := r.nodePaint(n, pRect)
	border := r.color("clusterBorder")
	if n.ColorIndex != nil && len(r.sheet.palette) > 0 {
		sl := r.sheet.palette[*n.ColorIndex%len(r.sheet.palette)]
		border = sl.border
		rect.Stroke = sl.border
		if !sl.fill.IsNone() {
			rect.Fill = sl.fill
		}
		css, _ := nodeStyles(n.Node)
		rect = rect.withCSS(css...)
	}
	s := newShape()
	s.add(scene.RectPath(x, y, w, h, rad, rad), rect)
	s.put(r, l, -l.W/2, -l.H/2-(gap+rowH)/2)
	sepY := y + pad + l.H + gap
	sep := paint{Stroke: border, Width: 0.75, Dash: []float64{3, 3}}
	s.items = append(s.items, sep.path(openPath([]P{{X: x + 8, Y: sepY}, {X: x + w - 8, Y: sepY}}, 0, 0)))
	dot := r.nodePaint(n, pCircle)
	dot.Fill = border
	dot.Fill.A *= 0.6
	dot.Stroke.A *= 0.6
	for i := -1; i <= 1; i++ {
		s.items = append(s.items, dot.path(scene.Ellipse(float64(i)*10, sepY+rowH/2, 2.5, 2.5)))
	}
	s.intersect = intersectRect
	return s
}

// Icons and images.

type iconKind int

const (
	iconPlain iconKind = iota
	iconSquare
	iconRounded
	iconCircle
)

// assetSize is an icon's size: the larger of its asset sizes, 48 by default.
func assetSize(n *nstate) float64 {
	return math.Max(orUnset(n.AssetHeight, 48), orUnset(n.AssetWidth, 48))
}

// assetLabel is an icon's or image's label: the stylesheet pads its
// paragraph by 2px (the box grows by 4, the text is drawn 2px down; centred
// text moves right by itself).
func assetLabel(r *renderer, n *nstate, width float64) *Label {
	l := *labelAt(r, n, width-4)
	if n.Label != "" {
		l.W += 4
		l.H += 4
	}
	return &l
}

// labelBackground is the box behind an icon's or image's label.
func (s *shape) labelBackground(r *renderer, l *Label, x, y float64) {
	if l.W > 0 && l.H > 0 && !r.sheet.edgeLabelBg.IsNone() {
		s.items = append(s.items, &scene.Path{Segs: scene.RectPath(x, y, l.W, l.H, 0, 0), Fill: r.sheet.edgeLabelBg})
	}
}

// assetPaint is the frame of an icon or image: roughjs' attributes, as no
// stylesheet rule reaches it, under the node's own styles.
func (r *renderer) assetPaint(n *nstate, stroke scene.Color) paint {
	css, _ := nodeStyles(n.Node)
	d := style.Parse(css...)
	p := paint{Fill: r.color("mainBkg"), Stroke: stroke, Width: 1.3}
	if v, ok := d["fill"]; ok {
		p.Fill = style.Color(v)
	}
	if v, ok := d["stroke-width"]; ok {
		p.Width = style.Px(v, p.Width)
	}
	return p
}

func iconShape(k iconKind) shapeFn {
	return func(r *renderer, n *nstate) *shape {
		size := assetSize(n)
		l := assetLabel(r, n, math.Max(size, r.cfg.Num("flowchart.wrappingWidth", 120)))
		top := n.Pos == "t"
		lp := 0.0
		if n.Label != "" {
			lp = 8
		}
		w, h := size, size
		if k == iconSquare || k == iconRounded {
			w, h = size+n.Padding, size+n.Padding
		}
		p := r.assetPaint(n, scene.None)
		if k != iconPlain {
			p.Stroke = p.Fill
		} else {
			p.Fill = scene.None
		}
		if k == iconCircle {
			icon := 0.0
			if n.Icon != "" {
				icon = size
			}
			w = icon*math.Sqrt2 + 40
			h = w
		}
		ow := math.Max(w, l.W)
		oh := h + l.H + lp
		dy := -l.H/2 - lp/2
		if top {
			dy = -dy
		}
		s := newShape()
		switch k {
		case iconCircle:
			s.add(scene.Ellipse(0, dy, w/2, w/2), p)
		case iconRounded:
			s.add(svgRoundedRect(-w/2, -h/2+dy, w, h, 5), p)
		case iconSquare:
			s.add(svgRoundedRect(-w/2, -h/2+dy, w, h, 0.1), p)
		default:
			s.add(scene.RectPath(-w/2, -h/2+dy, w, h, 0, 0), p)
		}
		s.bbox = emptyRect()
		s.bbox.addRect(scene.Rect{X: -ow / 2, Y: -oh / 2, W: ow, H: oh})
		ly := oh/2 - l.H
		if top {
			ly = -oh / 2
		}
		s.labelBackground(r, l, -l.W/2, ly)
		s.put(r, l, -l.W/2, ly+2)
		s.intersect = assetIntersect(n, l, w, h, lp, top)
		return s
	}
}

// svgRoundedRect is createRoundedRectPathD.
func svgRoundedRect(x, y, w, h, rad float64) []scene.Seg {
	return svgPath(0, 0, "M %v %v H %v A %v %v 0 0 1 %v %v V %v A %v %v 0 0 1 %v %v H %v A %v %v 0 0 1 %v %v V %v A %v %v 0 0 1 %v %v Z",
		x+rad, y, x+w-rad, rad, rad, x+w, y+rad, y+h-rad, rad, rad, x+w-rad, y+h, x+rad, rad, rad, x, y+h-rad, y+rad, rad, rad, x+rad, y)
}

// assetIntersect is the outline of an icon or image with its label: the
// asset's box and the label's, stacked.
func assetIntersect(n *nstate, l *Label, w, h, lp float64, top bool) func(b box, p P) P {
	if n.Label == "" {
		return intersectRect
	}
	return func(b box, p P) P {
		nh := b.H
		var pts []P
		if top {
			t := -nh/2 + l.H + lp
			pts = []P{{X: -l.W / 2, Y: -nh / 2}, {X: l.W / 2, Y: -nh / 2}, {X: l.W / 2, Y: t}, {X: w / 2, Y: t},
				{X: w / 2, Y: nh / 2}, {X: -w / 2, Y: nh / 2}, {X: -w / 2, Y: t}, {X: -l.W / 2, Y: t}}
		} else {
			t := -nh/2 + h
			pts = []P{{X: -w / 2, Y: -nh / 2}, {X: w / 2, Y: -nh / 2}, {X: w / 2, Y: t}, {X: l.W / 2, Y: t},
				{X: l.W / 4, Y: nh / 2}, {X: -l.W / 2, Y: nh / 2}, {X: -l.W / 2, Y: t}, {X: -w / 2, Y: t}}
		}
		return intersectPolygon(b, pts, p)
	}
}

// imageSquare: the image is not fetched; its size comes from the asset
// sizes (a square when only one is given).
func imageSquare(r *renderer, n *nstate) *shape {
	wrap := r.cfg.Num("flowchart.wrappingWidth", 120)
	natural := orUnset(n.AssetWidth, orUnset(n.AssetHeight, 48))
	iw := natural
	if n.Label != "" {
		iw = math.Max(wrap, natural)
	}
	ih := orUnset(n.AssetHeight, natural)
	if n.Constraint == "on" {
		if n.AssetHeight != 0 {
			iw = n.AssetHeight
		}
		ih = iw
	}
	l := assetLabel(r, n, math.Max(iw, wrap))
	top := n.Pos == "t"
	lp := 0.0
	if n.Label != "" {
		lp = 8
	}
	ow := math.Max(iw, l.W)
	oh := ih + l.H + lp
	dy := -l.H/2 - lp/2
	if top {
		dy = -dy
	}
	css, _ := nodeStyles(n.Node)
	stroke := r.sheet.nodeStroke
	if v, ok := style.Parse(css...)["stroke"]; ok {
		stroke = style.Color(v)
	}
	s := newShape()
	s.add(scene.RectPath(-iw/2, -ih/2+dy, iw, ih, 0, 0), r.assetPaint(n, stroke))
	s.bbox = emptyRect()
	s.bbox.addRect(scene.Rect{X: -ow / 2, Y: -oh / 2, W: ow, H: oh})
	ly := ih/2 - l.H/2 + lp/2
	if top {
		ly = -ih/2 - l.H/2 - lp/2
	}
	s.labelBackground(r, l, -l.W/2, ly)
	s.put(r, l, -l.W/2, ly+2)
	s.intersect = assetIntersect(n, l, iw, ih, lp, top)
	return s
}

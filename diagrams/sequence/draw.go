package sequence

import (
	"math"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

// The participant row under the neo look (actorBands.ts): a glyph band,
// a gap, the label, a gap, then the lifeline.
const (
	glyphBand = 44.0
	glyphGap  = 6.0
	labelGap  = 6.0
	// iconWidth is the stick figure's (ACTOR_TYPE_WIDTH).
	iconWidth = 36.0
	// The stick figure's extent in its classic units.
	figureTop, figureBottom = -5.0, 60.0
)

// colors are the stylesheet's (styles.js) as the theme fills it in.
type colors struct {
	text, actorBorder, actorBkg, actorText, actorLine, signal, signalText,
	labelBorder, labelBkg, labelText, loopText, noteBorder, noteBkg, noteText,
	actBorder, actBkg, seqNumber, mainBkg, nodeBorder scene.Color
	strokeWidth float64
	// borders and fills are a color theme's palette, by actor.
	borders, fills []scene.Color
	// shadow is the neo look's, on the participants; boxShadow the
	// theme's drop-shadow filter on boxes.
	shadow, boxShadow *scene.Shadow
}

func color(s string) scene.Color { return style.Color(s) }

func newColors(ctx *diagram.Context) colors {
	th := ctx.Theme
	g := func(k string) scene.Color { return color(th.Get(k)) }
	c := colors{
		text: g("textColor"), actorBorder: g("actorBorder"), actorBkg: g("actorBkg"), actorText: g("actorTextColor"),
		actorLine: g("actorLineColor"), signal: g("signalColor"), signalText: g("signalTextColor"),
		labelBorder: g("labelBoxBorderColor"), labelBkg: g("labelBoxBkgColor"), labelText: g("labelTextColor"),
		loopText: g("loopTextColor"), noteBorder: g("noteBorderColor"), noteBkg: g("noteBkgColor"),
		noteText: g("noteTextColor"), actBorder: g("activationBorderColor"), actBkg: g("activationBkgColor"),
		seqNumber: g("sequenceNumberColor"), mainBkg: g("mainBkg"), nodeBorder: g("nodeBorder"),
		strokeWidth: style.Px(th.Get("strokeWidth"), 1),
	}
	list := func(k string) []scene.Color {
		var out []scene.Color
		for _, s := range strings.Split(th.Get(k), ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, color(s))
			}
		}
		return out
	}
	c.borders, c.fills = list("borderColorArray"), list("bkgColorArray")
	if ctx.Look == "neo" {
		// redux and redux-color flood the shadow black, the other themes
		// white.
		flood := scene.Color{R: 255, G: 255, B: 255, A: 0.06}
		if th.Get("dropShadow") == "url(#drop-shadow)" && th.Get("darkMode") != "true" {
			flood = scene.Color{A: 0.06}
		}
		c.shadow = &scene.Shadow{DX: 4, DY: 4, Color: flood}
	}
	c.boxShadow = style.DropShadow(th.Get("dropShadow"))
	return c
}

// border and fill are an actor's palette colors, or the stylesheet's.
func (r *renderer) border(a *actor) scene.Color {
	if n := len(r.col.borders); n > 0 {
		return r.col.borders[a.index%n]
	}
	return r.col.actorBorder
}

func (r *renderer) fill(a *actor) scene.Color {
	if n := len(r.col.fills); n > 0 {
		return r.col.fills[a.index%n]
	}
	return r.col.actorBkg
}

func line(x1, y1, x2, y2 float64) []scene.Seg {
	var b scene.PathBuilder
	b.MoveTo(x1, y1).LineTo(x2, y2)
	return b.Segs
}

func circle(cx, cy, r float64) []scene.Seg { return scene.Ellipse(cx, cy, r, r) }

// lineHeight is a drawn line's height (getBBox's, in the font it shows in).
func (r *renderer) lineHeight(f font) float64 {
	a, d := r.ctx.Fonts.Extents(f.style(f.family))
	return a + d
}

func (r *renderer) label(x, y float64, s string, f font, anchor scene.Anchor, base scene.Baseline, c scene.Color) *scene.Text {
	return &scene.Text{X: x, Y: y, Anchor: anchor, Baseline: base, Family: f.family, Size: f.size, Color: c,
		Runs: []scene.Span{{Text: collapse(s), Weight: f.weight}}}
}

// textBlock is drawText for a vertically centered text: its lines one
// drawn line height apart from y + margin/2, each rounded, shifted down
// by dy. It returns the lines and their total height.
func (r *renderer) textBlock(s string, x, y, margin, dy float64, f font, anchor scene.Anchor, base scene.Baseline, c scene.Color) ([]scene.Item, float64) {
	var out []scene.Item
	h := 0.0
	lh := r.lineHeight(f)
	for _, l := range splitBreaks(s) {
		ly := jsRound(y + h + margin/2)
		if l != "" {
			out = append(out, r.label(x, ly+dy, l, f, anchor, base, c))
		}
		h += lh
	}
	return out, h
}

// alignX is drawText's x for an alignment within [x, x+width].
func alignX(align string, x, width, margin float64) (float64, scene.Anchor) {
	switch align {
	case "left", "start":
		return jsRound(x + margin), scene.Start
	case "right", "end":
		return jsRound(x + width - margin), scene.End
	}
	return jsRound(x + width/2), scene.Middle
}

// actorLabel is an actor's (or box's) name centered at (x, y), its lines
// a font size apart.
func (r *renderer) actorLabel(s string, x, y float64, c scene.Color) []scene.Item {
	f := r.c.actorFont
	lines := splitBreaks(s)
	var out []scene.Item
	for i, l := range lines {
		dy := float64(i)*f.size - f.size*float64(len(lines)-1)/2
		out = append(out, r.label(x, y+dy, l, f, scene.Middle, scene.Central, c))
	}
	return out
}

func (r *renderer) drawNote(n *noteModel) {
	c := &r.c
	r.bump(c.boxMargin)
	n.starty = r.vp
	x, anchor := alignX(c.noteAlign, n.startx, n.width, c.noteMargin)
	texts, th := r.textBlock(n.text, x, n.starty, c.noteMargin, c.noteFont.size, c.noteFont, anchor, scene.MiddleBaseline, r.col.noteText)
	th = jsRound(th)
	h := th + 2*c.noteMargin
	rect := &scene.Path{Segs: scene.RectPath(n.startx, n.starty, n.width, h, 0, 0), Fill: r.col.noteBkg, Stroke: r.col.noteBorder, StrokeWidth: 1}
	r.mainL = append(r.mainL, &scene.Group{Items: append([]scene.Item{rect}, texts...)})
	r.bump(h)
	r.insert(n.startx, n.starty, n.startx+n.width, n.starty+h)
}

func (r *renderer) drawActivation(a *activation, bottom float64) {
	ra := r.actors[a.actor]
	stroke, fill := r.col.actBorder, r.col.actBkg
	if n := len(r.col.borders); n > 0 {
		stroke, fill = r.col.borders[ra.index%n], r.col.mainBkg
		if m := len(r.col.fills); m > 0 {
			fill = r.col.fills[ra.index%m]
		}
	}
	a.g.Items = append(a.g.Items, &scene.Path{Segs: scene.RectPath(a.startx, a.starty, a.stopx-a.startx, bottom-a.starty, 0, 0),
		Fill: fill, Stroke: stroke, StrokeWidth: 1})
}

// drawLoop draws a block: its dashed frame and section lines, the label in
// its corner, the title and the sections' titles.
func (r *renderer) drawLoop(l *loopModel, kind string) {
	if math.IsNaN(l.startx) {
		return
	}
	c := &r.c
	f := c.msgFont
	dashed := func(x1, y1, x2, y2 float64, dash float64) *scene.Path {
		return &scene.Path{Segs: line(x1, y1, x2, y2), Stroke: r.col.labelBorder, StrokeWidth: 2, Dash: []float64{dash, dash}}
	}
	items := []scene.Item{
		dashed(l.startx, l.starty, l.stopx, l.starty, 2),
		dashed(l.stopx, l.starty, l.stopx, l.stopy, 2),
		dashed(l.startx, l.stopy, l.stopx, l.stopy, 2),
		dashed(l.startx, l.starty, l.startx, l.stopy, 2),
	}
	for _, y := range l.sections {
		items = append(items, dashed(l.startx, y, l.stopx, y, 3))
	}
	// The label: a box with a cut corner.
	w := math.Max(c.labelBoxW, 50)
	h := c.labelBoxH
	if c.neo {
		h += 15
	}
	x, y := l.startx, l.starty
	items = append(items, &scene.Path{Segs: scene.Polygon(scene.Pt{X: x, Y: y}, scene.Pt{X: x + w, Y: y},
		scene.Pt{X: x + w, Y: y + h - 7}, scene.Pt{X: x + w - 8.4, Y: y + h}, scene.Pt{X: x, Y: y + h}),
		Fill: r.col.labelBkg, Stroke: r.col.labelBorder, StrokeWidth: 1})
	items = append(items, r.label(jsRound(x+w/2), jsRound(y+h/2+c.boxTextMargin/2), kind, f, scene.Middle, scene.MiddleBaseline, r.col.labelText))
	title := func(s string, x, y float64) {
		t, _ := r.textBlock(s, x, y, c.boxTextMargin, 0, f, scene.Middle, scene.Alphabetic, r.col.loopText)
		items = append(items, t...)
	}
	title(l.title, l.startx+c.labelBoxW/2+(l.stopx-l.startx)/2, l.starty+c.boxMargin+c.boxTextMargin)
	for i, t := range l.titles {
		if t != "" {
			title(t, l.startx+(l.stopx-l.startx)/2, l.sections[i]+c.boxMargin+c.boxTextMargin)
		}
	}
	r.mainL = append(r.mainL, &scene.Group{Items: items})
}

func (r *renderer) drawBox(b *box, height float64) {
	c := &r.c
	pad := c.boxMargin * 2
	x, y := b.x-pad, b.y-pad*0.25
	items := []scene.Item{&scene.Path{Segs: scene.RectPath(x, y, b.w+2*pad, height+pad*0.75, 0, 0),
		Fill: color(b.Fill), Stroke: r.col.nodeBorder, StrokeWidth: 1, Shadow: r.col.boxShadow}}
	if b.name != "" {
		items = append(items, r.actorLabel(b.name, b.x+b.w/2, b.y+c.boxTextMargin+b.textMaxH/2, r.col.text)...)
	}
	r.boxL = append([]scene.Item{&scene.Group{Items: items}}, r.boxL...)
}

// fixLifelines ends the lifelines at the footer, or where an actor was
// destroyed.
func (r *renderer) fixLifelines() {
	for _, k := range r.keys {
		a := r.actors[k]
		if a.lifeline == nil {
			continue
		}
		y2 := 0.0
		switch {
		case r.c.mirror:
			y2 = a.stopy
		case a.stopy != 0:
			y2 = a.stopy + a.h/2
		default:
			continue
		}
		a.lifeline.Segs = line(a.x+a.w/2, a.lifelineTop, a.x+a.w/2, y2)
	}
}

// bands is the neo look's vertical layout of a participant's row.
type bands struct{ lifeline, glyphBottom, labelCenter float64 }

func (r *renderer) bands(a *actor, y float64, footer bool) bands {
	if footer {
		top := y + labelGap
		return bands{y, top + glyphBand, top + glyphBand + glyphGap + a.textH/2}
	}
	life := y + a.h
	return bands{life, life - labelGap - a.textH - glyphGap, life - labelGap - a.textH/2}
}

// drawActor draws a participant in the header or footer, and in the
// header its lifeline; it returns the height it takes.
func (r *renderer) drawActor(a *actor, footer bool) float64 {
	c := &r.c
	y := a.y
	if footer {
		y = a.stopy
	}
	cx := a.x + a.w/2
	b := r.bands(a, y, footer)
	border, fill := r.border(a), r.fill(a)
	shadow := r.col.shadow
	sw := r.col.strokeWidth
	figure := false // drawn above the blocks rather than under everything
	var items []scene.Item
	add := func(it ...scene.Item) { items = append(items, it...) }
	// Straight lines go without the shadow: a filter on a box with no
	// height draws nothing.
	stroke := func(segs []scene.Seg, w float64) *scene.Path {
		return &scene.Path{Segs: segs, Stroke: border, StrokeWidth: w}
	}
	shape := func(segs []scene.Seg) *scene.Path {
		return &scene.Path{Segs: segs, Fill: fill, Stroke: border, StrokeWidth: sw, Shadow: shadow}
	}
	lifeTop := y + a.h
	h := a.h
	switch a.Type {
	default: // participant
		rx := 3.0
		if c.neo {
			rx = 6
		}
		add(shape(scene.RectPath(a.x, y, a.w, a.h, rx, rx)))
		add(r.actorLabel(a.desc, cx, y+a.h/2, r.col.actorText)...)
	case TypeCollections:
		const off = 6.0
		rh := a.h
		if c.neo {
			rh -= off
		}
		add(shape(scene.RectPath(a.x, y, a.w, rh, 0, 0)), shape(scene.RectPath(a.x-off, y+off, a.w, rh, 0, 0)))
		add(r.actorLabel(a.desc, cx-off, y+off+rh/2, r.col.actorText)...)
	case TypeQueue:
		ry := a.h / 2
		rx := ry / (2.5 + a.h/50)
		var body, front scene.PathBuilder
		body.MoveTo(a.x+rx, y).ArcTo(rx, ry, 0, false, false, a.x+rx, y+a.h).LineTo(a.x+a.w-rx, y+a.h).
			ArcTo(rx, ry, 0, false, false, a.x+a.w-rx, y).Close()
		front.MoveTo(a.x+a.w-rx, y).ArcTo(rx, ry, 0, false, false, a.x+a.w-rx, y+a.h)
		add(shape(body.Segs), shape(front.Segs))
		add(r.actorLabel(a.desc, cx, y+a.h/2, r.col.actorText)...)
	case TypeDatabase:
		w := a.w / 3
		erx := w / 2
		ery := erx / (2.5 + w/50)
		hh, top := w, y
		labelY := y + 35 + a.h/2
		if c.neo {
			hh = glyphBand
			top = b.glyphBottom - hh - ery
			labelY = b.labelCenter
		} else {
			lifeTop = y + a.h + 2*c.boxTextMargin
		}
		x0, y0 := a.x+w, top+2*ery
		var p scene.PathBuilder
		p.MoveTo(x0, y0).ArcTo(erx, ery, 0, false, false, x0+w, y0).ArcTo(erx, ery, 0, false, false, x0, y0).
			LineTo(x0, y0+hh-2*ery).ArcTo(erx, ery, 0, false, false, x0+w, y0+hh-2*ery).LineTo(x0+w, y0)
		add(shape(p.Segs))
		add(r.actorLabel(a.desc, cx, labelY, r.col.actorText)...)
		if !c.neo {
			h = hh + c.labelBoxH
		}
	case TypeBoundary:
		figure = true
		const rad = 22.0
		icy, dy := b.glyphBottom-rad, 0.0
		labelY := b.labelCenter
		if !c.neo {
			icy, dy = y+12, rad/2+10
			labelY = y + 15 + a.h/2 + dy
			h = 2*rad + c.labelBoxH
			lifeTop = y + 80
		}
		icy += dy
		add(stroke(line(cx-rad*2.5, icy, cx-15, icy), 2), stroke(line(cx-rad*2.5, icy-10, cx-rad*2.5, icy+10), 2),
			&scene.Path{Segs: circle(cx, icy, rad), Fill: r.col.actorBkg, Stroke: border, StrokeWidth: 2, Shadow: shadow})
		add(r.actorLabel(a.desc, cx, labelY, r.col.actorText)...)
	case TypeControl:
		figure = true
		const rad = 22.0
		cy := b.glyphBottom - rad
		labelY := b.labelCenter
		if !c.neo {
			cy = y + 32
			gap := 12.0
			if footer {
				gap = 5
			}
			labelY = y + rad + gap + a.h/2
			h = 2*rad + 2*c.labelBoxH
			lifeTop = y + 75
		}
		add(&scene.Path{Segs: circle(cx, cy, rad), Fill: r.col.actorBkg, Stroke: border, StrokeWidth: 2, Shadow: shadow})
		// The arrowhead on top of the circle: mermaid's marker, turned to
		// 172.5° and scaled by the line's 2px width.
		sin, cos := math.Sincos(172.5 * math.Pi / 180)
		pt := func(u, v float64) scene.Pt {
			u, v = (u-11)*2, (v-5.8)*2
			return scene.Pt{X: cx + u*cos - v*sin, Y: cy - rad + u*sin + v*cos}
		}
		add(&scene.Path{Segs: scene.Polygon(pt(14.4, 5.6), pt(7.2, 10.4), pt(8.8, 5.6), pt(7.2, 0.8)), Fill: fill, Stroke: border, StrokeWidth: 2.4})
		add(r.actorLabel(a.desc, cx, labelY, r.col.actorText)...)
	case TypeEntity:
		figure = true
		const rad = 22.0
		cy, dy := b.glyphBottom-rad, 0.0
		labelY := b.labelCenter
		if !c.neo {
			cy, dy = y+25, rad/2-5
			gap := 30.0
			if footer {
				cy, dy, gap = y+10, rad, 15
			}
			cy += dy
			labelY = y + gap + a.h/2 + dy
			h = 2*rad + c.labelBoxH
			lifeTop = y + 75
		}
		add(shape(circle(cx, cy, rad)), stroke(line(cx-rad, cy+rad, cx+rad, cy+rad), 2))
		add(r.actorLabel(a.desc, cx, labelY, r.col.actorText)...)
	case TypeActor:
		figure = true
		scale, gy := 1.0, func(o float64) float64 { return y + o }
		labelY := y + 35 + (figureBottom-figureTop)/2
		if c.neo {
			scale = glyphBand / (figureBottom - figureTop)
			gy = func(o float64) float64 { return b.glyphBottom - (figureBottom-o)*scale }
			labelY = b.labelCenter
		} else {
			h = figureBottom - figureTop
			lifeTop = y + 80
		}
		gx := func(o float64) float64 { return cx + o*scale }
		add(stroke(line(cx, gy(25), cx, gy(45)), 2), stroke(line(gx(-iconWidth/2), gy(33), gx(iconWidth/2), gy(33)), 2),
			stroke(line(gx(-iconWidth/2), gy(60), cx, gy(45)), 2), stroke(line(cx, gy(45), gx(iconWidth/2-2), gy(60)), 2))
		add(&scene.Path{Segs: circle(cx, gy(10), 15*scale), Fill: r.col.actorBkg, Stroke: border, StrokeWidth: 2}) // no shadow
		add(r.actorLabel(a.desc, cx, labelY, r.col.actorText)...)
	}
	if c.neo {
		lifeTop = b.lifeline
	} else {
		a.h = h
	}
	var lowered []scene.Item
	if !footer {
		a.lifeline = &scene.Path{Stroke: r.col.actorLine, StrokeWidth: 2}
		a.lifelineTop = lifeTop
		lowered = append(lowered, a.lifeline)
	}
	g := &scene.Group{Items: items}
	switch {
	case !figure:
		lowered = append(lowered, g)
	case footer:
		r.footFigL = append(r.footFigL, g)
	default:
		r.figL = append(r.figL, g)
	}
	if len(lowered) > 0 {
		lg := &scene.Group{Items: lowered}
		if footer {
			r.footL = append([]scene.Item{lg}, r.footL...)
		} else {
			r.headL = append([]scene.Item{lg}, r.headL...)
		}
	}
	return h
}

// head is an arrowhead mermaid draws as a marker.
type head int

const (
	noHead head = iota
	arrowHead
	filledHead
	crossHead
	solidTopHead
	solidBottomHead
	stickTopHead
	stickBottomHead
)

func (k Kind) dotted() bool {
	switch k {
	case Dotted, DottedCross, DottedPoint, DottedOpen, BidirectionalDotted:
		return true
	}
	return k >= SolidTopDotted && k <= StickBottomReverseDotted
}

func (k Kind) bidirectional() bool { return k == BidirectionalSolid || k == BidirectionalDotted }

func (k Kind) reverse() bool {
	return k >= SolidTopReverse && k <= StickBottomReverse || k >= SolidTopReverseDotted && k <= StickBottomReverseDotted
}

// undotted is the kind with a solid line.
func (k Kind) undotted() Kind {
	switch {
	case k >= SolidTopDotted:
		return k - (SolidTopDotted - SolidTop)
	case k == Dotted:
		return Solid
	case k == DottedCross:
		return SolidCross
	case k == DottedPoint:
		return SolidPoint
	case k == DottedOpen:
		return SolidOpen
	case k == BidirectionalDotted:
		return BidirectionalSolid
	}
	return k
}

// heads are the heads at the line's start and end. A reverse half arrow's
// head is at its start, flipped (marker auto-start-reverse), so its top
// side is the other one.
func (k Kind) heads() (start, end head) {
	switch k.undotted() {
	case Solid:
		return noHead, arrowHead
	case BidirectionalSolid:
		return arrowHead, arrowHead
	case SolidPoint:
		return noHead, filledHead
	case SolidCross:
		return noHead, crossHead
	case SolidTop:
		return noHead, solidTopHead
	case SolidBottom:
		return noHead, solidBottomHead
	case StickTop:
		return noHead, stickTopHead
	case StickBottom:
		return noHead, stickBottomHead
	case SolidTopReverse:
		return solidBottomHead, noHead
	case SolidBottomReverse:
		return solidTopHead, noHead
	case StickTopReverse:
		return stickBottomHead, noHead
	case StickBottomReverse:
		return stickTopHead, noHead
	}
	return noHead, noHead
}

// shortensEnd reports the kinds whose line stops short of the lifeline
// for a head at its end.
func (k Kind) shortensEnd() bool {
	switch k {
	case SolidOpen, DottedOpen, StickTop, StickBottom, StickTopDotted, StickBottomDotted:
		return false
	}
	return !k.reverse()
}

// shortensStart reports the kinds whose line starts later for a head at
// its start.
func (k Kind) shortensStart() bool {
	switch k {
	case BidirectionalSolid, BidirectionalDotted, SolidTopReverse, SolidBottomReverse, SolidTopReverseDotted, SolidBottomReverseDotted:
		return true
	}
	return false
}

// drawHead draws a head at (x, y) pointing along angle (radians), as
// mermaid's marker: the marker's points relative to its ref point, scaled
// (markerUnits strokeWidth) and turned.
func (r *renderer) drawHead(h head, x, y, angle float64) []scene.Item {
	sin, cos := math.Sincos(angle)
	at := func(refX, refY, scale float64) func(u, v float64) scene.Pt {
		return func(u, v float64) scene.Pt {
			u, v = (u-refX)*scale, (v-refY)*scale
			return scene.Pt{X: x + u*cos - v*sin, Y: y + u*sin + v*cos}
		}
	}
	const lineW = 1.5 // the message line's width, which scales some markers
	switch h {
	case arrowHead:
		p := at(7.9, 5, 1)
		return []scene.Item{&scene.Path{Segs: scene.Polygon(p(-1, 0), p(10, 5), p(0, 10)), Fill: r.col.signal, Stroke: r.col.signal, StrokeWidth: 1}}
	case filledHead:
		p := at(15.5, 7, lineW)
		return []scene.Item{&scene.Path{Segs: scene.Polygon(p(18, 7), p(9, 13), p(14, 7), p(9, 1)), Fill: r.col.text}}
	case crossHead:
		p := at(4, 4.5, lineW)
		var b scene.PathBuilder
		a, bb, cc, d := p(1, 2), p(6, 7), p(6, 2), p(1, 7)
		b.MoveTo(a.X, a.Y).LineTo(bb.X, bb.Y).MoveTo(cc.X, cc.Y).LineTo(d.X, d.Y)
		return []scene.Item{&scene.Path{Segs: b.Segs, Stroke: r.col.signal, StrokeWidth: 4.0 / 3 * lineW}}
	case solidTopHead:
		p := at(7.9, 7.25, 1)
		return []scene.Item{&scene.Path{Segs: scene.Polygon(p(0, 0), p(10, 8), p(0, 8)), Fill: r.col.text}}
	case solidBottomHead:
		p := at(7.9, 0.75, 1)
		return []scene.Item{&scene.Path{Segs: scene.Polygon(p(0, 0), p(10, 0), p(0, 8)), Fill: r.col.text}}
	case stickTopHead, stickBottomHead:
		p := at(7.5, 7, 1)
		a, b := p(0, 0), p(7, 7)
		if h == stickBottomHead {
			p = at(7.5, 0, 1)
			a, b = p(0, 7), p(7, 0)
		}
		return []scene.Item{&scene.Path{Segs: line(a.X, a.Y, b.X, b.Y), Stroke: scene.RGB(0, 0, 0), StrokeWidth: 1.5}}
	}
	return nil
}

// drawMessage draws a message's text, line, heads and number.
func (r *renderer) drawMessage(mm *msgModel) {
	c := &r.c
	m := mm.Message
	startx, stopx, y := mm.startx, mm.stopx, mm.lineStartY
	f := c.msgFont
	x, anchor := alignX(c.messageAlign, math.Min(startx, stopx), math.Abs(stopx-startx), c.wrapPadding)
	texts, _ := r.textBlock(mm.text, x, mm.starty+10, c.wrapPadding, f.size, f, anchor, scene.MiddleBaseline, r.col.signalText)
	items := texts
	numbered := mm.seqVisible || c.numbers
	startHead, endHead := m.Kind.heads()
	lw := &scene.Path{Stroke: r.col.signal, StrokeWidth: 1.5}
	if m.Kind.dotted() {
		lw.Dash = []float64{3, 3}
	}
	var dots []scene.Item
	var startAngle, endAngle float64
	self := startx == stopx
	x1, x2 := startx, stopx
	if self {
		tw, _ := r.dims(mm.text, f)
		lx := startx
		if numbered && (m.Kind.reverse() || m.Kind.bidirectional()) {
			lx += 10
		}
		var b scene.PathBuilder
		if c.rightAngles {
			b.MoveTo(lx, y).LineTo(startx+math.Max(c.width/2, tw/2), y).LineTo(startx+math.Max(c.width/2, tw/2), y+25).LineTo(startx, y+25)
			startAngle, endAngle = math.Pi, math.Pi
		} else {
			b.MoveTo(lx, y).CubeTo(lx+60, y-10, startx+60, y+30, startx, y+20)
			startAngle, endAngle = math.Atan2(10, -60), math.Atan2(-10, -60)
		}
		lw.Segs = b.Segs
	} else {
		if numbered {
			switch {
			case m.Kind.bidirectional():
				if startx < stopx {
					x1 = startx + 12
				} else {
					x1 = startx - 6
					if m.Central != 0 {
						x1 -= 5
					}
					if m.Central == CentralConnectionDual || m.Central == CentralConnectionReverse {
						x1 -= 7.5
					}
				}
			case m.Kind.reverse():
				if stopx > startx {
					x2 = stopx - 12
				} else {
					x2 = stopx - 6
					if m.Central == CentralConnectionDual || m.Central == CentralConnectionReverse {
						x1 -= 7.5
					}
				}
				if m.Central != 0 {
					x2 += 15
				}
			default:
				x1 = startx + 6
			}
		}
		lw.Segs = line(x1, y, x2, y)
		endAngle = 0
		if x2 < x1 {
			endAngle = math.Pi
		}
		startAngle = endAngle + math.Pi
	}
	if m.Central != 0 {
		dots = r.centralDots(mm)
	}
	items = append(items, lw)
	items = append(items, dots...)
	if self {
		end := lw.Segs[len(lw.Segs)-1].P
		endPt := end[0]
		if lw.Segs[len(lw.Segs)-1].Op == scene.CubeTo {
			endPt = end[2]
		}
		items = append(items, r.drawHead(startHead, lw.Segs[0].P[0].X, y, startAngle)...)
		items = append(items, r.drawHead(endHead, endPt.X, endPt.Y, endAngle)...)
	} else {
		items = append(items, r.drawHead(startHead, x1, y, startAngle)...)
		items = append(items, r.drawHead(endHead, x2, y, endAngle)...)
	}
	if numbered {
		var nx float64
		right := startx <= stopx
		switch {
		case self:
			nx = mm.fromBounds + 1
		case m.Kind.reverse() == right:
			nx = mm.toBounds - 1
		default:
			nx = mm.fromBounds + 1
		}
		label := strconv.FormatFloat(mm.seqIndex, 'f', -1, 64)
		size := 12.0
		switch n := len(label); {
		case n > 5:
			size = 7
		case n > 3:
			size = 9
		}
		items = append(items, &scene.Path{Segs: circle(nx, y, 12), Fill: r.col.signal},
			&scene.Text{X: nx, Y: y + 4, Anchor: scene.Middle, Family: "sans-serif", Size: size, Color: r.col.seqNumber,
				Runs: []scene.Span{{Text: label}}})
	}
	r.msgL = append(r.msgL, &scene.Group{Items: items})
}

// centralDots are the dots of a message's () ends, on the lifelines'
// centers (moved aside for the number when numbering).
func (r *renderer) centralDots(mm *msgModel) []scene.Item {
	m := mm.Message
	from, to := r.actors[m.From], r.actors[m.To]
	fc, tc := from.x+from.w/2, to.x+to.w/2
	right := fc <= tc
	off := func(reverse bool) float64 {
		o := 16.5
		if !right {
			o = -o
		}
		if reverse {
			o = -o
		}
		return o
	}
	rev := m.Kind.reverse()
	if mm.seqVisible {
		switch m.Central {
		case CentralConnection:
			if rev {
				tc += off(true)
			}
		case CentralConnectionReverse:
			if !rev {
				fc += off(false)
			}
		case CentralConnectionDual:
			if rev {
				tc += off(true)
			} else {
				fc += off(false)
			}
		}
	}
	dot := func(x float64) scene.Item { return &scene.Path{Segs: circle(x, mm.lineStartY, 5), Fill: r.col.text} }
	switch m.Central {
	case CentralConnection:
		return []scene.Item{dot(tc)}
	case CentralConnectionReverse:
		return []scene.Item{dot(fc)}
	}
	return []scene.Item{dot(fc), dot(tc)}
}

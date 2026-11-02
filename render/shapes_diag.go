package render

import (
	"math"
	"strings"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// The boxes of the class, ER and requirement diagrams (classBox.ts,
// erBox.ts, requirementBox.ts): stacked text sections with dividers.

func init() {
	shapes["classBox"] = classBox
	shapes["erBox"] = erBox
	shapes["requirementBox"] = requirementBox
}

// text draws a label with its box's top left at (x, y), not counting it in
// the shape's box (these shapes' boxes are their outlines).
func (s *shape) text(r *renderer, l *Label, x, y float64) {
	if l != nil {
		s.items = append(s.items, r.drawLabel(l, x, y)...)
	}
}

// boxFont is the font of a box's texts: the node's label styles, else (as
// classBox does) all its styles.
func (r *renderer) boxFont(n *nstate, color scene.Color) font {
	f := r.sheet.font
	if !color.IsNone() {
		f.Color = color
	}
	css, lcss := nodeStyles(n.Node)
	if len(lcss) == 0 {
		lcss = css
	}
	return f.withCSS(lcss...)
}

// plainText lays out one unwrapped text of a box (the shapes give
// createText their text's width plus room, so nothing wraps).
func (r *renderer) plainText(s string, f font) *Label {
	if s == "" {
		return &Label{font: f}
	}
	return r.layoutLabel(s, f, labelOpts{Width: math.Inf(1), Markdown: true})
}

// classMember is a class attribute or method as getData hands it.
type classMember struct {
	Text       string `json:"text"`
	Classifier string `json:"classifier"`
}

func (m classMember) css() []string {
	switch m.Classifier {
	case "*":
		return []string{"font-style:italic"}
	case "$":
		return []string{"text-decoration:underline"}
	}
	return nil
}

// span is a group of labels' vertical extent.
type span struct{ top, bottom float64 }

func (sp *span) add(top, h float64) {
	if sp.bottom <= sp.top {
		*sp = span{top, top + h}
		return
	}
	sp.top, sp.bottom = math.Min(sp.top, top), math.Max(sp.bottom, top+h)
}

func (sp span) h() float64 { return math.Max(0, sp.bottom-sp.top) }

func classBox(r *renderer, n *nstate) *shape {
	pad := r.cfg.Num("class.padding", 12)
	gap := pad
	var members, methods []classMember
	var annotations, styles []string
	n.ExtraValue("members", &members)
	n.ExtraValue("methods", &methods)
	n.ExtraValue("annotations", &annotations)
	n.ExtraValue("styles", &styles)
	f := r.boxFont(n, r.color("classText"))
	if len(styles) > 0 {
		f = f.withCSS(labelDecls(styles)...)
	}

	// textHelper: the sections stacked at their textHelper offsets; x is
	// the left edge of each text, y its top.
	type placed struct {
		l    *Label
		x, y float64
	}
	var ann, title placed
	annH := 0.0
	if len(annotations) > 0 {
		ann.l = r.plainText("«"+annotations[0]+"»", f)
		annH = ann.l.H
		ann.x, ann.y = -ann.l.W/2, -ann.l.H/2
	}
	text := n.ExtraString("text")
	if text == "" {
		text = n.Label
	}
	bold := f
	bold.Weight = 700
	title.l = r.plainText(text, bold)
	labelH := title.l.H
	title.x, title.y = -title.l.W/2, annH-title.l.H/2
	stack := func(ms []classMember) ([]placed, span) {
		var out []placed
		var sp span
		y := 0.0
		for _, m := range ms {
			l := r.plainText(m.Text, f.withCSS(m.css()...))
			lines := math.Max(1, float64(len(l.lines)))
			p := placed{l: l, y: y - l.H/(2*lines)}
			out = append(out, p)
			sp.add(p.y, l.H)
			y += l.H
		}
		return out, sp
	}
	mem, memSpan := stack(members)
	met, _ := stack(methods)
	membersH := memSpan.h()
	if membersH <= 0 {
		membersH = gap / 2
	}
	memY := annH + labelH + gap*2
	metY := annH + labelH + membersH + gap*4

	// The content's box, as getBBox sees the groups.
	bb := emptyRect()
	addP := func(p placed, dy float64) {
		if p.l != nil && (p.l.W > 0 || p.l.H > 0) {
			bb.addRect(scene.Rect{X: p.x, Y: p.y + dy, W: p.l.W, H: p.l.H})
		}
	}
	addP(ann, 0)
	addP(title, 0)
	for _, p := range mem {
		addP(p, memY)
	}
	for _, p := range met {
		addP(p, metY)
	}
	content := bb.scene()

	empty := len(members) == 0 && len(methods) == 0
	hideEmpty := r.cfg.Get("class.hideEmptyMembersBox") == true
	extraBox := empty && !hideEmpty
	w := math.Max(n.Width, content.W)
	h := math.Max(n.Height, content.H)
	tall := n.Height > content.H
	if empty {
		h += gap
	} else if len(methods) == 0 {
		h += gap * 2
	}
	x, y := -w/2, -h/2
	extraH := 0.0
	shift := 0.0 // how far the frame's top moves up past the text's
	switch {
	case extraBox:
		extraH, shift = pad*2, pad
	case empty:
		extraH, shift = -pad, -pad/2
	}
	if tall {
		extraH = pad * 2
	}
	s := newShape()
	rx, ry := x-pad, y-pad-shift
	rw, rh := w+2*pad, h+2*pad+extraH
	s.add(scene.RectPath(rx, ry, rw, rh, 0, 0), r.classPaint(n, pOuterPath, styles))

	// classBox's adjustments use the groups' heights less half the padding
	// when it adds the empty box.
	less := 0.0
	if extraBox {
		less = pad / 2
	}
	annH2, labelH2, membersH2 := annH-less, labelH-less, memSpan.h()-less
	top := y + pad - shift
	methodsArea := (annH2 + labelH2 + y + pad - ry) / 2
	if ann.l != nil {
		s.text(r, ann.l, ann.x, ann.y+top)
	}
	s.text(r, title.l, title.x, title.y+top)
	for _, p := range mem {
		s.text(r, p.l, x, p.y+memY+top)
	}
	metTop := annH2 + labelH2 + math.Max(membersH2, gap/2) + y + gap*4 + pad
	if tall {
		metTop = math.Max(methodsArea, annH2+labelH2+math.Max(membersH2, gap/2)+y+gap*2+pad) + gap*2
	}
	for _, p := range met {
		s.text(r, p.l, x, p.y+metTop)
	}
	if !empty || extraBox {
		line := r.classPaint(n, pLine, styles)
		y1 := annH2 + labelH2 + y + pad
		y2 := annH2 + labelH2 + membersH2 + y + gap*2 + pad
		if tall {
			y2 = math.Max(methodsArea, y2)
		}
		for _, ly := range []float64{y1, y2} {
			s.items = append(s.items, line.path(polyline([]P{{X: rx, Y: ly}, {X: rx + rw, Y: ly}})))
		}
	}
	s.intersect = intersectRect
	return s
}

// classPaint is a class box part's paint: the node paint, with the class's
// own style statements over it.
func (r *renderer) classPaint(n *nstate, k part, styles []string) paint {
	p := r.nodePaint(n, k)
	if k == pLine {
		p.Fill = scene.None
	}
	return p.withCSS(styles...)
}

// labelDecls are the declarations of styles that style text.
func labelDecls(styles []string) []string {
	var out []string
	for _, s := range styles {
		k, _, _ := strings.Cut(s, ":")
		if isLabelStyle(strings.TrimSpace(k)) {
			out = append(out, s)
		}
	}
	return out
}

// erAttribute is an entity attribute as getData hands it.
type erAttribute struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Keys    []string `json:"keys"`
	Comment string   `json:"comment"`
}

func erBox(r *renderer, n *nstate) *shape {
	if a := n.ExtraString("alias"); a != "" {
		n.Label = a
	}
	pad := r.cfg.Num("er.diagramPadding", 10)
	textPad := r.cfg.Num("er.entityPadding", 6)
	var attrs []erAttribute
	n.ExtraValue("attributes", &attrs)
	f := r.boxFont(n, scene.None)
	if len(attrs) == 0 && n.Label != "" {
		// calculateTextWidth measures in the config's font, not the
		// theme's.
		minW := r.cfg.Num("er.minEntityWidth", 100)
		cf := f.style(run{})
		cf.Family, cf.Size = r.cfg.Str("fontFamily", cf.Family), r.cfg.Num("fontSize", 16)
		if r.fonts.Width(n.Label, cf)+pad*2 < minW && n.Width == 0 {
			n.Width = minW
		}
		return drawRect(r, n, rectOpts{padX: pad, padY: pad * 1.5})
	}
	// Without htmlLabels set at the top level (the default), erBox pads
	// as for SVG text.
	if v, _ := r.cfg.Get("htmlLabels").(bool); !v {
		pad *= 1.25
		textPad *= 1.25
	}
	start := f
	start.Align = scene.Start
	name := r.plainText(genericTypes(n.Label), f)
	nameH := name.H + textPad
	type row struct {
		cells  [4]*Label
		y, h   float64
		isEven bool
	}
	var rows []row
	var colW [4]float64
	maxText := name.W
	y := 0.0
	for i, a := range attrs {
		var rw row
		for c, t := range []string{a.Type, a.Name, strings.Join(a.Keys, ","), a.Comment} {
			l := r.plainText(genericTypes(t), start)
			rw.cells[c] = l
			colW[c] = math.Max(colW[c], l.W+pad)
			rw.h = math.Max(rw.h, l.H)
			maxText = math.Max(maxText, l.W)
		}
		rw.h += textPad
		rw.y = y
		rw.isEven = (i+1)%2 == 0 && y != 0
		y += rw.h
		rows = append(rows, rw)
	}
	sections := 4.0
	keys, comments := colW[2] > pad, colW[3] > pad
	if !keys {
		colW[2] = 0
		sections--
	}
	if !comments {
		colW[3] = 0
		sections--
	}
	if d := name.W + pad*2 - (colW[0] + colW[1] + colW[2] + colW[3]); d > 0 {
		for c := range colW {
			if c < 2 || colW[c] > 0 {
				colW[c] += d / sections
			}
		}
	}
	maxW := colW[0] + colW[1] + colW[2] + colW[3]
	w := math.Max(maxText+pad*2, math.Max(n.Width, maxW))
	h := math.Max(y+nameH, n.Height)
	x, top := -w/2, -h/2

	s := newShape()
	s.add(scene.RectPath(x, top, w, h, 0, 0), r.nodePaint(n, pOuterPath))
	for _, rw := range rows {
		p := r.nodePaint(n, pRect)
		p.Shadow = nil
		if r.sheet.nodeSlots != slotAll {
			fill := r.color("rowOdd")
			if rw.isEven {
				fill = r.color("rowEven")
			}
			if !fill.IsNone() {
				p.Fill = fill
			}
		}
		s.items = append(s.items, p.path(scene.RectPath(x, top+nameH+rw.y, w, rw.h, 0, 0)))
	}
	line := r.nodePaint(n, pLine)
	hline := func(ly float64) {
		s.items = append(s.items, line.path(polyline([]P{{X: x, Y: ly}, {X: x + w, Y: ly}})))
	}
	vline := func(lx float64) {
		s.items = append(s.items, line.path(polyline([]P{{X: lx, Y: top + nameH}, {X: lx, Y: top + h}})))
	}
	hline(top + nameH)
	vline(x + colW[0])
	if keys {
		vline(x + colW[0] + colW[1])
	}
	if comments {
		vline(x + colW[0] + colW[1] + colW[2])
	}
	s.text(r, name, -name.W/2, top+textPad/2)
	for _, rw := range rows {
		cx := x + pad/2
		for c, l := range rw.cells {
			s.text(r, l, cx, rw.y+top+nameH+textPad/2)
			cx += colW[c]
		}
	}
	s.intersect = intersectRect
	return s
}

// genericTypes is parseGenericTypes: Java-style generics written with
// tildes (List~int~) become angle brackets.
func genericTypes(s string) string {
	if !strings.Contains(s, "~") {
		return s
	}
	var b strings.Builder
	open := false
	for _, c := range s {
		if c == '~' {
			if open {
				b.WriteString("&gt;")
			} else {
				b.WriteString("&lt;")
			}
			open = !open
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

func requirementBox(r *renderer, n *nstate) *shape {
	const pad, gap = 20.0, 20.0
	_, isReq := n.Extra["verifyMethod"]
	f := r.boxFont(n, scene.None)
	body := f
	if r.ctx.Layout != "dagre" {
		body.Align = scene.Start
	}
	bold := f
	bold.Weight = 700
	type line struct {
		l *Label
		y float64 // the text's centre line before centring the box
	}
	var lines []line
	acc := 0.0
	add := func(s string, fnt font) float64 {
		if s == "" {
			return 0
		}
		l := r.plainText(s, fnt)
		lines = append(lines, line{l, acc})
		return l.H
	}
	typ := n.ExtraString("type")
	if !isReq {
		typ = "Element"
	}
	typeH := add("&lt;&lt;"+typ+"&gt;&gt;", f)
	acc += typeH
	nameH := add(n.ExtraString("name"), bold)
	acc += nameH + gap
	field := func(prefix, key string) {
		if v := n.ExtraString(key); v != "" {
			acc += add(prefix+v, body)
		}
	}
	if isReq {
		field("ID: ", "requirementId")
		field("Text: ", "text")
		field("Risk: ", "risk")
		field("Verification: ", "verifyMethod")
	} else {
		field("Type: ", "type")
		field("Doc Ref: ", "docRef")
	}
	bb := emptyRect()
	for _, ln := range lines {
		bb.addRect(scene.Rect{X: -ln.l.W / 2, Y: ln.y - ln.l.H/2, W: ln.l.W, H: ln.l.H})
	}
	content := bb.scene()
	w, h := content.W+pad, content.H+pad
	x, y := -w/2, -h/2
	s := newShape()
	s.add(scene.RectPath(x, y, w, h, 0, 0), r.nodePaint(n, pOuterPath))
	for i, ln := range lines {
		lx := x + pad/2
		if i < 2 {
			lx = -ln.l.W / 2
		}
		s.text(r, ln.l, lx, ln.y-ln.l.H/2-h/2+pad)
	}
	if acc > typeH+nameH+gap {
		ly := y + typeH + nameH + gap
		s.items = append(s.items, r.nodePaint(n, pLine).path(polyline([]P{{X: x, Y: ly}, {X: x + w, Y: ly}})))
	}
	s.intersect = intersectRect
	return s
}

// slotPaint puts a node's palette slot over a part's paint, as the color
// themes' stylesheets do in the neo look.
func (r *renderer) slotPaint(n *nstate, k part, p paint) paint {
	s := r.sheet
	if s.nodeSlots == slotNone || n.ColorIndex == nil || len(s.palette) == 0 || n.IsGroup {
		return p
	}
	if s.nodeSlots == slotOutline && k != pOuterPath && k != pLine {
		return p
	}
	sl := s.palette[*n.ColorIndex%len(s.palette)]
	p.Stroke = sl.border
	if k != pLine && !sl.fill.IsNone() {
		p.Fill = sl.fill
	}
	return p
}

package sequence

import (
	"math"
	"regexp"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

func init() { diagram.Register("sequence", Render) }

// conf is mermaid's sequence config, with the diagram's top-level font
// settings applied to the actor, note and message fonts as setConf does.
type conf struct {
	neo                                      bool
	mirror, rightAngles, numbers, hideUnused bool
	activationW, marginX, marginY, actorMargin, width, height, boxMargin, boxTextMargin,
	noteMargin, bottomMarginAdj, wrapPadding, labelBoxW, labelBoxH float64
	messageAlign, noteAlign      string
	actorFont, noteFont, msgFont font
}

// font is a text style. mermaid's default family ends in ';', which is not
// valid CSS, so it measures text in the page's default font and draws it in
// the theme's; we measure what is drawn, so the layout fits the text on any
// machine.
type font struct {
	family string
	size   float64
	weight int
}

func (f font) style(family string) text.Style {
	return text.Style{Family: family, Size: f.size, Weight: f.weight}
}

func newConf(ctx *diagram.Context) conf {
	cfg := ctx.Config
	num := func(k string, def float64) float64 { return cfg.Num("sequence."+k, def) }
	c := conf{
		neo:             ctx.Look == "neo",
		mirror:          cfg.Bool("sequence.mirrorActors", true),
		rightAngles:     cfg.Bool("sequence.rightAngles", false),
		numbers:         cfg.Bool("sequence.showSequenceNumbers", false),
		hideUnused:      cfg.Bool("sequence.hideUnusedParticipants", false),
		activationW:     num("activationWidth", 10),
		marginX:         num("diagramMarginX", 50),
		marginY:         num("diagramMarginY", 10),
		actorMargin:     num("actorMargin", 50),
		width:           num("width", 150),
		height:          num("height", 65),
		boxMargin:       num("boxMargin", 10),
		boxTextMargin:   num("boxTextMargin", 5),
		noteMargin:      num("noteMargin", 10),
		wrapPadding:     num("wrapPadding", 10),
		labelBoxW:       num("labelBoxWidth", 50),
		labelBoxH:       num("labelBoxHeight", 20),
		messageAlign:    cfg.Str("sequence.messageAlign", "center"),
		noteAlign:       cfg.Str("sequence.noteAlign", "center"),
		bottomMarginAdj: num("bottomMarginAdj", 1),
	}
	themeFamily := ctx.Theme.Get("fontFamily")
	mk := func(key string) font {
		family := cfg.Str("sequence."+key+"FontFamily", "")
		if v := cfg.Str("fontFamily", ""); v != "" {
			family = v
		}
		f := font{size: num(key+"FontSize", 14), weight: int(num(key+"FontWeight", 400))}
		if v := cfg.Num("fontSize", 0); v != 0 {
			f.size = v
		}
		if v := cfg.Num("fontWeight", 0); v != 0 {
			f.weight = int(v)
		}
		f.family = family
		if strings.ContainsAny(family, ";{}") || strings.TrimSpace(family) == "" {
			f.family = themeFamily
		}
		return f
	}
	c.actorFont, c.noteFont, c.msgFont = mk("actor"), mk("note"), mk("message")
	return c
}

// actor is a participant being laid out.
type actor struct {
	*Actor
	desc        string // wrapped
	x, y, w, h  float64
	margin      float64 // 0 until set
	stopy       float64 // 0 until set
	textH       float64
	index       int // palette slot
	lifeline    *scene.Path
	lifelineTop float64
}

type box struct {
	*Box
	name                      string
	x, y, w, margin, textMaxH float64
}

// bound is a box whose sides may be unset (NaN), as mermaid's undefined.
type bound struct{ startx, starty, stopx, stopy float64 }

func unset() bound { return bound{math.NaN(), math.NaN(), math.NaN(), math.NaN()} }

// loopModel is a block being laid out.
type loopModel struct {
	bound
	title    string
	fill     string
	sections []float64 // their y
	titles   []string
}

type activation struct {
	bound
	actor string
	g     *scene.Group // where it is drawn, reserved when it starts
}

// msgModel is a message's geometry.
type msgModel struct {
	*Message
	text                 string // wrapped
	width, height        float64
	startx, stopx        float64
	starty, stopy        float64
	fromBounds, toBounds float64
	seqIndex             float64
	seqVisible           bool
	lineStartY           float64
}

type noteModel struct {
	text                  string
	startx, starty, width float64
}

// loopWidth is calculateLoopBounds' width of a block's content.
type loopWidth struct{ from, to, width float64 }

type renderer struct {
	ctx    *diagram.Context
	d      *Diagram
	c      conf
	col    colors
	actors map[string]*actor
	keys   []string
	boxes  []*box
	shown  []*box // the boxes of the shown actors, in order

	vp    float64
	data  bound
	stack []*loopModel
	acts  []*activation

	msgs  map[*Message]*msgModel
	notes map[*Message]*noteModel
	loops map[*Message]*loopWidth

	// Layers, bottom to top, as mermaid's lower() calls leave them.
	boxL, rectL, footL, headL, mainL, figL, msgL, footFigL []scene.Item

	dimsCache map[dimsKey][2]float64
}

// Render draws a sequence diagram.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	autoWrap := ctx.Config.Bool("wrap", ctx.Config.Bool("sequence.wrap", false))
	d, err := Parse(code, autoWrap)
	if err != nil {
		return nil, err
	}
	if d.Title == "" {
		d.Title = ctx.Title
	}
	r := &renderer{ctx: ctx, d: d, c: newConf(ctx), col: newColors(ctx), actors: map[string]*actor{},
		data: unset(), msgs: map[*Message]*msgModel{}, notes: map[*Message]*noteModel{},
		dimsCache: map[dimsKey][2]float64{}}
	return r.draw(), nil
}

// draw is sequenceRenderer's draw: size the actors, place them in a row,
// walk the messages down the page, then draw the actors, the messages and
// the footer.
func (r *renderer) draw() *scene.Scene {
	c := &r.c
	d := r.d
	boxOf := map[*Box]*box{}
	for _, b := range d.Boxes {
		bx := &box{Box: b, name: b.Name}
		boxOf[b] = bx
		r.boxes = append(r.boxes, bx)
	}
	for _, a := range d.Actors {
		r.actors[a.ID] = &actor{Actor: a, desc: a.Description}
	}
	r.actorMargins(r.messageWidths())
	if len(r.boxes) > 0 {
		r.bump(c.boxMargin)
		for _, b := range r.boxes {
			if b.name != "" {
				r.bump(r.boxes[0].textMaxH)
				break
			}
		}
	}
	used := map[string]bool{}
	for _, m := range d.Messages {
		used[m.From], used[m.To] = true, true
	}
	for _, a := range d.Actors {
		if !c.hideUnused || used[a.ID] {
			r.actors[a.ID].index = len(r.keys)
			r.keys = append(r.keys, a.ID)
		}
	}
	r.placeActors(boxOf)
	r.loops = r.loopBounds()
	toDraw := r.walk()

	for _, k := range r.keys {
		r.drawActor(r.actors[k], false)
	}
	for _, mm := range toDraw {
		r.drawMessage(mm)
	}
	if c.mirror {
		r.bump(c.boxMargin * 2)
		maxH := 0.0
		for _, k := range r.keys {
			a := r.actors[k]
			if a.stopy == 0 {
				a.stopy = r.vp
			}
			maxH = math.Max(maxH, r.drawActor(a, true))
		}
		r.bump(maxH + c.boxMargin)
	}
	r.fixLifelines()
	for _, b := range r.shown {
		h := r.vp - b.y
		r.insert(b.x, b.y, b.x+b.w, h)
		r.drawBox(b, h)
	}
	if len(r.boxes) > 0 {
		r.bump(c.boxMargin)
	}
	return r.finish()
}

// walk goes down the messages: it draws the notes, activations and
// blocks, and places the messages, which are drawn later above the
// activations.
func (r *renderer) walk() []*msgModel {
	c := &r.c
	var toDraw []*msgModel
	newLoop := func(title, fill string) {
		r.stack = append(r.stack, &loopModel{bound: bound{math.NaN(), r.vp, math.NaN(), math.NaN()}, title: title, fill: fill})
	}
	seq, step := 1.0, 1.0
	numbers := false
	for index, msg := range r.d.Messages {
		switch msg.Kind {
		case Note:
			r.drawNote(r.notes[msg])
		case ActiveStart, CentralConnection, CentralConnectionReverse:
			r.newActivation(msg)
		case ActiveEnd:
			r.activeEnd(msg)
		case LoopStart, OptStart, AltStart, ParStart, ParOverStart, CriticalStart, BreakStart:
			r.loopTitle(msg, c.boxMargin, c.boxMargin+c.boxTextMargin, func(title string) { newLoop(title, "") })
		case AltElse, ParAnd, CriticalOption:
			r.loopTitle(msg, c.boxMargin+c.boxTextMargin, c.boxMargin, func(title string) {
				l := r.stack[len(r.stack)-1]
				l.sections = append(l.sections, r.vp)
				l.titles = append(l.titles, title)
			})
		case RectStart:
			r.loopTitle(msg, c.boxMargin, c.boxMargin, func(string) {
				fill := msg.Text
				for _, k := range []string{"rectBkgColor", "actorBkg"} {
					if fill == "" {
						fill = r.ctx.Theme.Get(k)
					}
				}
				newLoop("", fill)
			})
		case RectEnd:
			l := r.endLoop()
			if !math.IsNaN(l.startx) {
				// Lowered: under everything, an outer rect under an inner one.
				r.rectL = append([]scene.Item{&scene.Path{Segs: scene.RectPath(l.startx, l.starty, l.stopx-l.startx, l.stopy-l.starty, 0, 0),
					Fill: color(l.fill)}}, r.rectL...)
			}
			r.bumpTo(l.stopy)
		case LoopEnd, OptEnd, AltEnd, ParEnd, CriticalEnd, BreakEnd:
			l := r.endLoop()
			r.drawLoop(l, map[Kind]string{LoopEnd: "loop", OptEnd: "opt", AltEnd: "alt", ParEnd: "par",
				CriticalEnd: "critical", BreakEnd: "break"}[msg.Kind])
			r.bumpTo(l.stopy)
		case Autonumber:
			if msg.Start != 0 {
				seq = msg.Start
			}
			if msg.Step != 0 {
				step = msg.Step
			}
			numbers = msg.Visible
		default:
			if mm := r.msgs[msg]; mm != nil {
				mm.starty = r.vp
				mm.seqIndex = seq
				mm.seqVisible = numbers
				mm.lineStartY = r.boundMessage(mm)
				r.createdDestroyed(mm, index)
				toDraw = append(toDraw, mm)
			}
		}
		if msg.Kind.IsSignal() {
			// Rounded: steps like 0.1 would drift.
			seq = math.Round((seq+step)*100) / 100
		}
	}
	return toDraw
}

// finish stacks the layers and sizes the view box as mermaid does.
func (r *renderer) finish() *scene.Scene {
	c := &r.c
	data := r.data
	for _, v := range []*float64{&data.startx, &data.starty, &data.stopx, &data.stopy} {
		if math.IsNaN(*v) {
			*v = 0
		}
	}
	height := data.stopy - data.starty + 2*c.marginY
	if c.mirror {
		height = height - c.boxMargin + c.bottomMarginAdj
	}
	var items []scene.Item
	for _, l := range [][]scene.Item{r.boxL, r.rectL, r.footL, r.headL, r.mainL, r.figL, r.msgL, r.footFigL} {
		items = append(items, l...)
	}
	extraTitle, extraNeo := 0.0, 0.0
	if r.d.Title != "" {
		extraTitle = 40
		items = append(items, &scene.Text{X: (data.stopx-data.startx)/2 - 2*c.marginX, Y: -25,
			Family: r.ctx.Theme.Get("fontFamily"), Size: style.Px(r.ctx.Theme.Get("fontSize"), 16), Color: r.col.text,
			Runs: []scene.Span{{Text: r.d.Title}}})
	}
	if len(r.d.Actors) > 0 && c.neo {
		extraNeo = 30
	}
	vb := scene.Rect{X: data.startx - c.marginX, Y: -(c.marginY + extraTitle),
		W: data.stopx - data.startx + 2*c.marginX, H: height + extraTitle + extraNeo}
	// A lifeline mermaid leaves at its first length (no footer, never
	// destroyed) runs off the bottom of the drawing.
	for _, k := range r.keys {
		if a := r.actors[k]; a.lifeline != nil && a.lifeline.Segs == nil {
			a.lifeline.Segs = line(a.x+a.w/2, a.lifelineTop, a.x+a.w/2, vb.Y+vb.H)
		}
	}
	return &scene.Scene{ViewBox: vb, Items: items}
}

// messageWidths is getMaxMessageWidthPerActor: the widest message or note
// each actor must leave room for on its right.
func (r *renderer) messageWidths() map[string]float64 {
	c := &r.c
	out := map[string]float64{}
	put := func(id string, w float64) { out[id] = math.Max(out[id], w) }
	for _, m := range r.d.Messages {
		to, from := r.actors[m.To], r.actors[m.From]
		if to == nil || from == nil {
			continue
		}
		if m.Placement == LeftOf && to.Prev == "" || m.Placement == RightOf && to.Next == "" {
			continue
		}
		isNote := m.Kind == Note
		f := c.msgFont
		if isNote {
			f = c.noteFont
		}
		s := m.Text
		if m.Wrap {
			s = r.wrapLabel(s, c.width-2*c.wrapPadding, f)
		}
		w, _ := r.dims(s, f)
		w += 2 * c.wrapPadding
		switch {
		case !isNote && m.From == to.Next:
			put(m.To, w)
		case !isNote && m.From == to.Prev:
			put(m.From, w)
		case !isNote && m.From == m.To:
			put(m.From, w/2)
			put(m.To, w/2)
		case m.Placement == RightOf:
			put(m.From, w)
		case m.Placement == LeftOf:
			put(to.Prev, w)
		case m.Placement == Over:
			if to.Prev != "" {
				put(to.Prev, w/2)
			}
			if to.Next != "" {
				put(m.From, w/2)
			}
		}
	}
	return out
}

// actorMargins is calculateActorMargins: actor sizes, the margins after
// them, the boxes' margins; the row's height becomes conf.height.
func (r *renderer) actorMargins(widths map[string]float64) {
	c := &r.c
	maxH := 0.0
	for _, a := range r.d.Actors {
		ra := r.actors[a.ID]
		if a.Wrap {
			ra.desc = r.wrapLabel(ra.desc, c.width-2*c.wrapPadding, c.actorFont)
		}
		w, h := r.dims(ra.desc, c.actorFont)
		ra.w = math.Max(c.width, w+2*c.wrapPadding)
		if a.Wrap {
			ra.w = c.width
		}
		switch {
		case c.neo:
			ra.textH = h
			ra.h = math.Max(glyphBand+glyphGap+h+labelGap, c.height)
		case a.Wrap:
			ra.h = math.Max(h, c.height)
		default:
			ra.h = c.height
		}
		maxH = math.Max(maxH, ra.h)
	}
	for id, mw := range widths {
		a := r.actors[id]
		if a == nil {
			continue
		}
		w := mw + c.actorMargin - a.w/2
		if next := r.actors[a.Next]; next != nil {
			w -= next.w / 2
		}
		a.margin = math.Max(w, c.actorMargin)
	}
	maxBoxH := 0.0
	for _, b := range r.boxes {
		total := 0.0
		for _, k := range b.Actors {
			total += r.actors[k].w + r.actors[k].margin
		}
		total += c.boxMargin*8 - 2*c.boxTextMargin
		if b.Wrap {
			b.name = r.wrapLabel(b.name, total-2*c.wrapPadding, c.msgFont)
		}
		w, h := r.dims(b.name, c.msgFont)
		maxBoxH = math.Max(maxBoxH, h)
		b.margin = c.boxTextMargin + math.Max(0, (w+2*c.wrapPadding-total)/2)
	}
	for _, b := range r.boxes {
		b.textMaxH = maxBoxH
	}
	c.height = math.Max(maxH, c.height)
}

// placeActors is addActorRenderingData for the header: the actors'
// x along the row, with their boxes.
func (r *renderer) placeActors(boxOf map[*Box]*box) {
	c := &r.c
	prevWidth, prevMargin, maxH := 0.0, 0.0, 0.0
	var prevBox *box
	for _, k := range r.keys {
		a := r.actors[k]
		b := boxOf[a.Box]
		if prevBox != nil && prevBox != b {
			r.shown = append(r.shown, prevBox)
			prevMargin += c.boxMargin + prevBox.margin
		}
		if b != nil && b != prevBox {
			b.x, b.y = prevWidth+prevMargin, 0
			prevMargin += b.margin
		}
		a.w = math.Max(a.w, c.width)
		a.h = math.Max(a.h, c.height)
		if a.margin == 0 {
			a.margin = c.actorMargin
		}
		maxH = math.Max(maxH, a.h)
		if a.Created > 0 {
			prevMargin += a.w / 2
		}
		a.x = prevWidth + prevMargin
		a.y = r.vp
		r.insert(a.x, 0, a.x+a.w, a.h)
		prevWidth += a.w + prevMargin
		if b != nil {
			b.w = prevWidth + b.margin - b.x
		}
		prevMargin = a.margin
		prevBox = b
	}
	if prevBox != nil {
		r.shown = append(r.shown, prevBox)
	}
	r.bump(maxH)
}

// The bounds, as mermaid's bounds object.

func (r *renderer) bump(d float64) {
	r.vp += d
	r.data.stopy = nanMax(r.data.stopy, r.vp)
}

// bumpTo moves the vertical position to y (a block's bottom).
func (r *renderer) bumpTo(y float64) {
	if !math.IsNaN(y) {
		r.bump(y - r.vp)
	}
}

func nanMax(a, b float64) float64 {
	switch {
	case math.IsNaN(a):
		return b
	case math.IsNaN(b):
		return a
	}
	return math.Max(a, b)
}

func nanMin(a, b float64) float64 {
	switch {
	case math.IsNaN(a):
		return b
	case math.IsNaN(b):
		return a
	}
	return math.Min(a, b)
}

// insert grows the diagram's bounds, and those of the open blocks (by a
// margin for each level of nesting) and activations.
func (r *renderer) insert(x1, y1, x2, y2 float64) {
	startx, stopx := math.Min(x1, x2), math.Max(x1, x2)
	starty, stopy := math.Min(y1, y2), math.Max(y1, y2)
	d := &r.data
	d.startx, d.starty = nanMin(d.startx, startx), nanMin(d.starty, starty)
	d.stopx, d.stopy = nanMax(d.stopx, stopx), nanMax(d.stopy, stopy)
	m := r.c.boxMargin
	cnt := 0
	grow := func(b *bound, activation bool) {
		cnt++
		n := float64(len(r.stack) - cnt + 1)
		b.starty = nanMin(b.starty, starty-n*m)
		b.stopy = nanMax(b.stopy, stopy+n*m)
		d.startx = nanMin(d.startx, startx-n*m)
		d.stopx = nanMax(d.stopx, stopx+n*m)
		if !activation {
			b.startx = nanMin(b.startx, startx-n*m)
			b.stopx = nanMax(b.stopx, stopx+n*m)
			d.starty = nanMin(d.starty, starty-n*m)
			d.stopy = nanMax(d.stopy, stopy+n*m)
		}
	}
	for _, l := range r.stack {
		grow(&l.bound, false)
	}
	for _, a := range r.acts {
		grow(&a.bound, true)
	}
}

func (r *renderer) endLoop() *loopModel {
	l := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	return l
}

func (r *renderer) activationsOf(id string) int {
	n := 0
	for _, a := range r.acts {
		if a.actor == id {
			n++
		}
	}
	return n
}

// startActivation is where an actor's next activation box goes.
func (r *renderer) startActivation(id string) *activation {
	a := r.actors[id]
	x := a.x + a.w/2 + float64(r.activationsOf(id)-1)*r.c.activationW/2
	act := &activation{bound: bound{x, math.NaN(), x + r.c.activationW, math.NaN()}, actor: id}
	r.acts = append(r.acts, act)
	return act
}

// endActivation removes an actor's latest activation.
func (r *renderer) endActivation(id string) *activation {
	i := len(r.acts) - 1
	for i >= 0 && r.acts[i].actor != id {
		i--
	}
	if i < 0 {
		i = len(r.acts) - 1 // as JavaScript's splice(-1, 1)
	}
	a := r.acts[i]
	r.acts = append(r.acts[:i], r.acts[i+1:]...)
	return a
}

// activationBounds is the left and right of an actor's lifeline with its
// activations.
func (r *renderer) activationBounds(id string) (float64, float64) {
	a := r.actors[id]
	left, right := a.x+a.w/2-1, a.x+a.w/2+1
	for _, act := range r.acts {
		if act.actor == id {
			left, right = math.Min(left, act.startx), math.Max(right, act.stopx)
		}
	}
	return left, right
}

// loopBounds is calculateLoopBounds: the message and note models, and the
// width of each block's content (for wrapping its title), with the
// activations played through.
func (r *renderer) loopBounds() map[*Message]*loopWidth {
	c := &r.c
	loops := map[*Message]*loopWidth{}
	type open struct {
		start *Message
		w     *loopWidth
	}
	var stack []open
	for _, m := range r.d.Messages {
		switch m.Kind {
		case LoopStart, AltStart, OptStart, ParStart, ParOverStart, CriticalStart, BreakStart:
			stack = append(stack, open{m, &loopWidth{from: math.MaxInt64, to: math.MinInt64}})
		case AltElse, ParAnd, CriticalOption:
			if m.Text != "" {
				top := stack[len(stack)-1]
				loops[top.start], loops[m] = top.w, top.w
			}
		case LoopEnd, AltEnd, OptEnd, ParEnd, CriticalEnd, BreakEnd:
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			loops[top.start] = top.w
		case ActiveStart:
			r.startActivation(m.From)
		case ActiveEnd:
			r.endActivation(m.From)
		}
		if m.Kind == Note {
			n := r.noteModel(m)
			r.notes[m] = n
			for _, o := range stack {
				l := o.w
				l.from = math.Min(l.from, n.startx)
				l.to = math.Max(l.to, n.startx+n.width)
				l.width = math.Max(l.width, math.Abs(l.from-l.to)) - c.labelBoxW
			}
			continue
		}
		mm := r.messageModel(m)
		if mm == nil {
			continue
		}
		r.msgs[m] = mm
		for _, o := range stack {
			l := o.w
			if mm.startx == mm.stopx {
				from, to := r.actors[m.From], r.actors[m.To]
				l.from = math.Min(math.Min(from.x-mm.width/2, from.x-from.w/2), l.from)
				l.to = math.Max(math.Max(to.x+mm.width/2, to.x+from.w/2), l.to)
				l.width = math.Max(l.width, math.Abs(l.to-l.from)) - c.labelBoxW
			} else {
				l.from = math.Min(mm.startx, l.from)
				l.to = math.Max(mm.stopx, l.to)
				l.width = math.Max(l.width, mm.width) - c.labelBoxW
			}
		}
	}
	r.acts = nil
	return loops
}

// noteModel is buildNoteModel: a note's width and left, by placement.
func (r *renderer) noteModel(m *Message) *noteModel {
	c := &r.c
	from, to := r.actors[m.From], r.actors[m.To]
	wrap := m.Wrap && m.Text != ""
	s := m.Text
	if wrap {
		s = r.wrapLabel(s, c.width, c.noteFont)
	}
	tw, _ := r.dims(s, c.noteFont)
	n := &noteModel{text: m.Text, startx: from.x}
	switch {
	case m.Placement == RightOf:
		n.width = math.Max(from.w/2+to.w/2, tw+2*c.noteMargin)
		if wrap {
			n.width = math.Max(c.width, tw)
		}
		n.startx = from.x + (from.w+c.actorMargin)/2
	case m.Placement == LeftOf:
		n.width = math.Max(from.w/2+to.w/2, tw+2*c.noteMargin)
		if wrap {
			n.width = math.Max(c.width, tw+2*c.noteMargin)
		}
		n.startx = from.x - n.width + (from.w-c.actorMargin)/2
	case m.To == m.From:
		if wrap {
			n.width = math.Max(c.width, from.w)
		} else {
			tw, _ = r.dims(m.Text, c.noteFont)
			n.width = math.Max(math.Max(from.w, c.width), tw+2*c.noteMargin)
		}
		n.startx = from.x + (from.w-n.width)/2
	default:
		n.width = math.Abs(from.x+from.w/2-(to.x+to.w/2)) + c.actorMargin
		if from.x < to.x {
			n.startx = from.x + from.w/2 - c.actorMargin/2
		} else {
			n.startx = to.x + to.w/2 - c.actorMargin/2
		}
	}
	if wrap {
		n.text = r.wrapLabel(m.Text, n.width-2*c.wrapPadding, c.noteFont)
	}
	return n
}

// messageModel is buildMessageModel: where a message's line starts and
// stops, short of the lifelines and activations so its heads fit.
func (r *renderer) messageModel(m *Message) *msgModel {
	if !m.Kind.IsSignal() {
		return nil
	}
	c := &r.c
	fromLeft, fromRight := r.activationBounds(m.From)
	toLeft, toRight := r.activationBounds(m.To)
	right := fromLeft <= toLeft
	startx, stopx := fromLeft, toRight
	if right {
		startx, stopx = fromRight, toLeft
	}
	// toward moves x along the message's direction (back for negative v).
	toward := func(v float64) float64 {
		if right {
			return v
		}
		return -v
	}
	if c.neo {
		if m.Kind != SolidOpen {
			stopx -= toward(3)
		}
		if m.Kind == BidirectionalSolid || m.Kind == BidirectionalDotted {
			startx += toward(3)
		}
	}
	if m.Central == CentralConnectionReverse || m.Central == CentralConnectionDual {
		startx += 4
		if m.Kind == BidirectionalSolid || m.Kind == BidirectionalDotted {
			if !right {
				startx -= 6
			}
		}
	}
	if m.From == m.To {
		stopx = startx
	} else {
		if m.Activate && math.Abs(toLeft-toRight) <= 2 {
			stopx -= toward(c.activationW/2 - 1)
		}
		if m.Kind.shortensEnd() {
			stopx -= toward(3)
		}
		if m.Kind.shortensStart() {
			startx += toward(3)
		}
	}
	bounded := math.Abs(startx - stopx)
	s := m.Text
	if m.Wrap && s != "" {
		s = r.wrapLabel(s, math.Max(bounded+2*c.wrapPadding, c.width), c.msgFont)
	}
	w, _ := r.dims(s, c.msgFont)
	width := math.Max(bounded+2*c.wrapPadding, c.width)
	if !m.Wrap {
		width = math.Max(width, w+2*c.wrapPadding)
	}
	return &msgModel{Message: m, text: s, width: width, startx: startx, stopx: stopx,
		fromBounds: math.Min(math.Min(fromLeft, fromRight), math.Min(toLeft, toRight)),
		toBounds:   math.Max(math.Max(fromLeft, fromRight), math.Max(toLeft, toRight))}
}

// loopTitle is adjustLoopHeightForWrap: a block's (or section's) title is
// "[title]", wrapped to the block's width, and the space it takes.
func (r *renderer) loopTitle(m *Message, pre, post float64, add func(title string)) {
	c := &r.c
	r.bump(pre)
	adjust := post
	title := m.Text
	if l := r.loops[m]; l != nil && m.Text != "" {
		title = r.wrapLabel("["+m.Text+"]", l.width-2*c.wrapPadding, c.msgFont)
		_, h := r.dims(title, c.msgFont)
		adjust = post + math.Max(h, c.labelBoxH)
	}
	add(title)
	r.bump(adjust)
}

// boundMessage reserves a message's space and returns the y of its line.
func (r *renderer) boundMessage(mm *msgModel) float64 {
	c := &r.c
	r.bump(10)
	tw, th := r.dims(mm.text, c.msgFont)
	lineH := th / float64(len(splitBreaks(mm.text)))
	mm.height += lineH
	r.bump(lineH)
	var lineY float64
	offset := th - 10
	if mm.startx == mm.stopx {
		if !c.rightAngles {
			offset += c.boxMargin
		}
		lineY = r.vp + offset
		offset += 30
		dx := math.Max(tw/2, c.width/2)
		r.insert(mm.startx-dx, r.vp-10+offset, mm.stopx+dx, r.vp+30+offset)
	} else {
		offset += c.boxMargin
		lineY = r.vp + offset
		r.insert(mm.startx, lineY-10, mm.stopx, lineY)
	}
	r.bump(offset)
	mm.height += offset
	mm.stopy = mm.starty + mm.height
	r.insert(mm.fromBounds, mm.starty, mm.toBounds, mm.stopy)
	return lineY
}

// isIcon reports the participant types drawn as a figure rather than a
// box, whose creating and destroying messages stop at the figure.
func isIcon(t string) bool {
	return t == TypeActor || t == TypeControl || t == TypeEntity || t == TypeDatabase
}

// createdDestroyed is adjustCreatedDestroyedData: a message that creates
// an actor ends at its box, which starts there; one that destroys it ends
// its lifeline.
func (r *renderer) createdDestroyed(mm *msgModel, index int) {
	c := &r.c
	m := mm.Message
	from, to := r.actors[m.From], r.actors[m.To]
	bottom := func(a *actor) float64 { return mm.stopy + a.h/2 + c.noteMargin }
	receiver := func(a *actor, adj float64) {
		if a.x < from.x {
			r.insert(mm.stopx-adj, mm.starty, mm.startx, bottom(a))
			mm.stopx += adj
		} else {
			r.insert(mm.startx, mm.starty, mm.stopx+adj, bottom(a))
			mm.stopx -= adj
		}
	}
	sender := func(a *actor, adj float64) {
		if a.x < to.x {
			r.insert(mm.startx-adj, mm.starty, mm.stopx, bottom(a))
			mm.startx += adj
		} else {
			r.insert(mm.stopx, mm.starty, mm.startx+adj, bottom(a))
			mm.startx -= adj
		}
	}
	adjust := func(a *actor, extra float64) float64 {
		if isIcon(a.Type) {
			return iconWidth/2 + extra
		}
		return a.w/2 + extra
	}
	switch {
	case to.Created == index:
		receiver(to, adjust(to, 3))
		to.y = mm.lineStartY - to.h/2
		r.bump(to.h / 2)
	case from.Destroyed == index:
		if c.mirror {
			sender(from, adjust(from, 0))
		}
		from.stopy = mm.lineStartY - from.h/2
		r.bump(from.h / 2)
	case to.Destroyed == index:
		if c.mirror {
			receiver(to, adjust(to, 3))
		}
		to.stopy = mm.lineStartY - to.h/2
		r.bump(to.h / 2)
	}
}

func (r *renderer) newActivation(m *Message) {
	a := r.startActivation(m.From)
	a.starty = r.vp + 2
	a.g = &scene.Group{}
	r.mainL = append(r.mainL, a.g)
}

func (r *renderer) activeEnd(m *Message) {
	vp := r.vp
	a := r.endActivation(m.From)
	if a.starty+18 > vp {
		a.starty = vp - 6
		vp += 12
	}
	r.drawActivation(a, vp)
	r.insert(a.startx, vp-10, a.stopx, vp)
}

var lineBreak = regexp.MustCompile(`(?i)</?br\s*/?>`)

func splitBreaks(s string) []string { return lineBreak.Split(s, -1) }

type dimsKey struct {
	s string
	f font
}

// dims is calculateTextDimensions: the width of the widest line and the
// sum of the lines' heights, each rounded, as getBBox gives them.
func (r *renderer) dims(s string, f font) (float64, float64) {
	if s == "" {
		return 0, 0
	}
	k := dimsKey{s, f}
	if v, ok := r.dimsCache[k]; ok {
		return v[0], v[1]
	}
	st := f.style(f.family)
	a, d := r.ctx.Fonts.Extents(st)
	w, h := 0.0, 0.0
	for _, l := range splitBreaks(s) {
		w = math.Max(w, r.ctx.Fonts.Width(collapse(l), st))
		h += math.Round(a + d)
	}
	r.dimsCache[k] = [2]float64{math.Round(w), h}
	return math.Round(w), h
}

// wrapLabel is mermaid's wrapLabel: words fill lines up to maxWidth,
// a word too long for a line is broken with hyphens; lines are joined with
// <br/>. A label with breaks already is left as it is.
func (r *renderer) wrapLabel(label string, maxWidth float64, f font) string {
	if label == "" || lineBreak.MatchString(label) {
		return label
	}
	width := func(s string) float64 { w, _ := r.dims(s, f); return w }
	var lines []string
	next := ""
	words := strings.FieldsFunc(label, func(r rune) bool { return r == ' ' })
	for i, w := range words {
		wl := width(w + " ")
		nl := width(next)
		switch {
		case wl > maxWidth:
			// Break the word into pieces that fit, each but the last
			// hyphenated.
			cur := ""
			lines = append(lines, next)
			rs := []rune(w)
			for j, ch := range rs {
				candidate := cur + string(ch)
				if width(candidate) >= maxWidth {
					if j == len(rs)-1 {
						lines = append(lines, candidate)
					} else {
						lines = append(lines, candidate+"-")
					}
					cur = ""
				} else {
					cur = candidate
				}
			}
			next = cur
		case nl+wl >= maxWidth:
			lines = append(lines, next)
			next = w
		case next == "":
			next = w
		default:
			next += " " + w
		}
		if i == len(words)-1 {
			lines = append(lines, next)
		}
	}
	out := lines[:0]
	for _, l := range lines {
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "<br/>")
}

var spaces = regexp.MustCompile(`[ 	

]+`)

// collapse is SVG text's white space handling: runs of spaces become one,
// none at the ends.
func collapse(s string) string { return strings.TrimSpace(spaces.ReplaceAllString(s, " ")) }

// jsRound is JavaScript's Math.round (halves up).
func jsRound(x float64) float64 { return math.Floor(x + 0.5) }

// Package cynefin is mermaid's Cynefin framework diagram (diagrams/cynefin):
// four domains around a central confusion area, items in each, and
// transitions between domains.
package cynefin

import (
	"fmt"
	"math"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/lex"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

func init() { diagram.Register("cynefin", Render) }

var domainNames = []string{"complex", "complicated", "clear", "chaotic", "confusion"}

// Diagram is a parsed Cynefin diagram.
type Diagram struct {
	Title       string
	Items       map[string][]string
	Transitions []Transition
}

// Transition is a move from one domain to another.
type Transition struct{ From, To, Label string }

func isDomain(s string) bool {
	for _, d := range domainNames {
		if s == d {
			return true
		}
	}
	return false
}

// Parse reads a Cynefin diagram (cynefin.langium).
func Parse(code string) (*Diagram, error) {
	toks, err := lex.Lex(code, "-->")
	if err != nil {
		return nil, err
	}
	p := &lex.Parser{Toks: toks}
	p.SkipNewlines()
	if _, err := p.Expect(lex.ID, "cynefin-beta"); err != nil {
		return nil, err
	}
	p.Accept(lex.Punct, ":")
	d := &Diagram{Items: map[string][]string{}}
	for !p.Is(lex.EOF, "") {
		t := p.Next()
		switch {
		case t.Kind == lex.Newline, t.Kind == lex.AccTitle, t.Kind == lex.AccDescr:
		case t.Kind == lex.Title:
			d.Title = t.Text
		case t.Kind == lex.ID && isDomain(t.Text):
			if _, ok := p.Accept(lex.Punct, "-->"); ok {
				to, err := p.Expect(lex.ID, "")
				if err != nil || !isDomain(to.Text) {
					return nil, p.Errorf("expected a domain after -->")
				}
				tr := Transition{From: t.Text, To: to.Text}
				if _, ok := p.Accept(lex.Punct, ":"); ok {
					s, err := p.Expect(lex.String, "")
					if err != nil {
						return nil, err
					}
					tr.Label = s.Text
				}
				d.Transitions = append(d.Transitions, tr)
				continue
			}
			if _, ok := d.Items[t.Text]; !ok {
				d.Items[t.Text] = nil
			}
			for {
				p.SkipNewlines()
				s, ok := p.Accept(lex.String, "")
				if !ok {
					break
				}
				d.Items[t.Text] = append(d.Items[t.Text], s.Text)
			}
		default:
			return nil, fmt.Errorf("cynefin: line %d: unexpected %s", t.Line, t)
		}
	}
	return d, nil
}

type box struct{ cx, cy, x, y, w, h float64 }

var meta = map[string][2]string{
	"complex":     {"Probe → Sense → Respond", "Emergent Practices"},
	"complicated": {"Sense → Analyse → Respond", "Good Practices"},
	"clear":       {"Sense → Categorise → Respond", "Best Practices"},
	"chaotic":     {"Act → Sense → Respond", "Novel Practices"},
	"confusion":   {"", "Disorder"},
}

// Render draws a Cynefin diagram.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	d, err := Parse(code)
	if err != nil {
		return nil, err
	}
	if d.Title == "" {
		d.Title = ctx.Title
	}
	cfg, th := ctx.Config, ctx.Theme
	w, h := cfg.Num("cynefin.width", 800), cfg.Num("cynefin.height", 600)
	pad := cfg.Num("cynefin.padding", 40)
	desc := cfg.Bool("cynefin.showDomainDescriptions", true)
	amp := cfg.Num("cynefin.boundaryAmplitude", 8)
	tv := func(k string) string { return th.Get("cynefin." + k) }
	tn := func(k string, def float64) float64 { return style.Px(tv(k), def) }
	bg := func(dom string) scene.Color { return style.Color(tv(dom + "Bg")) }
	family := th.Get("fontFamily")
	domainSize, itemSize := tn("domainFontSize", 16), tn("itemFontSize", 12)
	textColor, labelColor := style.Color(tv("textColor")), style.Color(tv("labelColor"))
	boundary := style.Color(tv("boundaryColor"))

	hw, hh := w/2, h/2
	boxes := map[string]box{
		"complex":     {hw / 2, hh / 2, 0, 0, hw, hh},
		"complicated": {hw + hw/2, hh / 2, hw, 0, hw, hh},
		"chaotic":     {hw / 2, hh + hh/2, 0, hh, hw, hh},
		"clear":       {hw + hw/2, hh + hh/2, hw, hh, hw, hh},
		"confusion":   {hw, hh, hw * 0.7, hh * 0.7, hw * 0.6, hh * 0.6},
	}
	seed := int32(cfg.Num("cynefin.seed", 0))
	if seed == 0 {
		seed = hashString(ctx.ID)
	}
	quads := []string{"complex", "complicated", "chaotic", "clear"}
	var items []scene.Item
	for _, q := range quads {
		b := boxes[q]
		c := bg(q)
		c.A *= 0.4
		items = append(items, &scene.Path{Segs: scene.RectPath(b.x, b.y, b.w, b.h, 0, 0), Fill: c})
	}
	items = append(items,
		&scene.Path{Segs: wobble(w, h, seed, amp, true), Stroke: boundary, StrokeWidth: tn("boundaryWidth", 2), Dash: []float64{6, 3}},
		&scene.Path{Segs: wobble(w, h, seed+100, amp, false), Stroke: boundary, StrokeWidth: tn("boundaryWidth", 2), Dash: []float64{6, 3}},
		&scene.Path{Segs: cliff(w, h), Stroke: style.Color(tv("cliffColor")), StrokeWidth: tn("cliffWidth", 4)},
	)
	conf := bg("confusion")
	conf.A *= 0.5
	items = append(items, &scene.Path{Segs: scene.Ellipse(w/2, h/2, w*0.15, h*0.15), Fill: conf, Stroke: boundary, StrokeWidth: 1.5, Dash: []float64{4, 2}})
	label := func(x, y float64, s string, size float64, col scene.Color, bold, italic bool, base scene.Baseline) *scene.Text {
		weight := 400
		if bold {
			weight = 700
		}
		return &scene.Text{X: x, Y: y, Anchor: scene.Middle, Baseline: base, Family: family, Size: size, Color: col,
			Runs: []scene.Span{{Text: s, Weight: weight, Italic: italic}}}
	}
	for _, q := range quads {
		b := boxes[q]
		y := b.cy
		if desc {
			y -= 30
		}
		items = append(items, label(b.cx, y, strings.ToUpper(q[:1])+q[1:], domainSize, labelColor, true, false, scene.MiddleBaseline))
	}
	cy := h / 2
	if desc {
		cy -= 10
	}
	items = append(items, label(w/2, cy, "Confusion", domainSize, labelColor, true, false, scene.MiddleBaseline))
	if desc {
		for _, q := range quads {
			b := boxes[q]
			items = append(items,
				label(b.cx, b.cy-10, meta[q][0], itemSize-1, textColor, false, true, scene.MiddleBaseline),
				label(b.cx, b.cy+5, meta[q][1], itemSize-1, textColor, false, true, scene.MiddleBaseline))
		}
		items = append(items, label(w/2, h/2+8, meta["confusion"][1], itemSize-1, textColor, false, true, scene.MiddleBaseline))
	}
	const itemH, itemPad = 26.0, 10.0
	badge := func(cx, y float64, s string, fill scene.Color, overflow bool) {
		bw := ctx.Fonts.Width(s, text.Style{Family: family, Size: itemSize}) + itemPad*2
		x := cx - bw/2
		r := &scene.Path{Segs: scene.RectPath(x, y, bw, itemH, 4, 4), Fill: fill, Stroke: boundary, StrokeWidth: 1}
		if overflow {
			r.Fill.A *= 0.6
			r.Dash = []float64{3, 2}
		} else {
			r.Fill.A *= 0.95
		}
		items = append(items, r, label(x+bw/2, y+itemH/2, s, itemSize, textColor, false, false, scene.Central))
	}
	for _, dom := range []string{"complex", "complicated", "chaotic", "clear", "confusion"} {
		list := d.Items[dom]
		if len(list) == 0 {
			continue
		}
		b := boxes[dom]
		over := 0
		start := b.cy + 15
		if desc {
			start = b.cy + 25
		}
		if dom == "confusion" {
			start = b.cy + 14
			if desc {
				start = b.cy + 22
			}
			if len(list) > 3 {
				over = len(list) - 3
				list = list[:3]
			}
		}
		for i, s := range list {
			badge(b.cx, start+float64(i)*(itemH+4), s, bg(dom), false)
		}
		if over > 0 {
			badge(b.cx, start+float64(len(list))*(itemH+4), fmt.Sprintf("+%d more", over), bg(dom), true)
		}
	}
	arrow := style.Color(tv("arrowColor"))
	aw := tn("arrowWidth", 2)
	for _, tr := range d.Transitions {
		if tr.From == tr.To {
			continue
		}
		a, b := boxes[tr.From], boxes[tr.To]
		dx, dy := b.cx-a.cx, b.cy-a.cy
		l := math.Hypot(dx, dy)
		off := l * 0.15
		cpx, cpy := (a.cx+b.cx)/2-dy/l*off, (a.cy+b.cy)/2+dx/l*off
		var pb scene.PathBuilder
		pb.MoveTo(a.cx, a.cy).QuadTo(cpx, cpy, b.cx, b.cy)
		items = append(items, &scene.Path{Segs: pb.Segs, Stroke: arrow, StrokeWidth: aw})
		// The arrowhead marker: 10×10 units drawn 6 stroke widths wide, its
		// point 1 unit past the line's end, along the end's direction.
		k := 6 * aw / 10
		ang := math.Atan2(b.cy-cpy, b.cx-cpx)
		sin, cos := math.Sincos(ang)
		pt := func(u, v float64) scene.Pt {
			x, y := (u-9)*k, (v-5)*k
			return scene.Pt{X: b.cx + x*cos - y*sin, Y: b.cy + x*sin + y*cos}
		}
		items = append(items, &scene.Path{Segs: scene.Polygon(pt(0, 0), pt(10, 5), pt(0, 10)), Fill: arrow})
		if tr.Label != "" {
			items = append(items, label(cpx, cpy-6, tr.Label, itemSize-1, textColor, false, false, scene.Alphabetic))
		}
	}
	if d.Title != "" {
		items = append(items, label(w/2, -pad/2, d.Title, domainSize+2, labelColor, true, false, scene.MiddleBaseline))
	}
	return &scene.Scene{ViewBox: scene.Rect{W: w + 2*pad, H: h + 2*pad},
		Items: []scene.Item{&scene.Group{Transform: scene.Translate(pad, pad), Items: items}}}, nil
}

// wobble is a hand-drawn-looking boundary through the middle: vertical
// (the fold) or horizontal.
func wobble(w, h float64, seed int32, amp float64, vertical bool) []scene.Seg {
	const segs = 7
	var b scene.PathBuilder
	type pt struct{ x, y float64 }
	pts := make([]pt, segs+1)
	for i := range pts {
		if vertical {
			pts[i] = pt{w/2 + random(seed+int32(i)*17)*amp*2 - amp, float64(i) * h / segs}
		} else {
			pts[i] = pt{float64(i) * w / segs, h/2 + random(seed+int32(i)*23)*amp*2 - amp}
		}
	}
	b.MoveTo(pts[0].x, pts[0].y)
	for i := range segs {
		p0, p1 := pts[i], pts[i+1]
		dir := 1.0
		if i%2 == 1 {
			dir = -1
		}
		if vertical {
			off := amp * 1.5 * dir * random(seed+int32(i)*31+7)
			mid := (p0.y + p1.y) / 2
			b.CubeTo(p0.x+off, mid, p1.x-off, mid, p1.x, p1.y)
		} else {
			off := amp * 1.5 * dir * random(seed+int32(i)*37+11)
			mid := (p0.x + p1.x) / 2
			b.CubeTo(mid, p0.y+off, mid, p1.y-off, p1.x, p1.y)
		}
	}
	return b.Segs
}

// cliff is the line from clear down into chaos.
func cliff(w, h float64) []scene.Seg {
	cx, top, bottom := w/2, h*0.5, h
	a := w * 0.03
	at := func(f float64) float64 { return top + (bottom-top)*f }
	var b scene.PathBuilder
	b.MoveTo(cx, top).
		CubeTo(cx+a, at(0.2), cx-a*1.5, at(0.55), cx+a*0.5, at(0.75)).
		CubeTo(cx-a, at(0.85), cx+a*0.3, at(0.95), cx, bottom)
	return b.Segs
}

// random is mermaid's seeded random (mulberry32), in [0, 1).
func random(seed int32) float64 {
	t := uint32(seed + 0x6d2b79f5)
	t = (t ^ t>>15) * (t | 1)
	t ^= t + (t^t>>7)*(t|61)
	return float64(t^t>>14) / 4294967296
}

// hashString is mermaid's 32-bit string hash (UTF-16 code units).
func hashString(s string) int32 {
	var h int32
	for _, r := range s {
		if r > 0xFFFF {
			r -= 0x10000
			h = h<<5 - h + int32(0xD800+(r>>10))
			r = 0xDC00 + r&0x3FF
		}
		h = h<<5 - h + int32(r)
	}
	return h
}

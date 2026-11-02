package scene

import (
	"fmt"
	"strconv"
	"strings"
)

// SVG writes the scene as a standalone SVG document.
func (s *Scene) SVG() string {
	var w svgWriter
	vb := s.ViewBox
	fmt.Fprintf(&w.b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="%s %s %s %s">`,
		num(vb.W), num(vb.H), num(vb.X), num(vb.Y), num(vb.W), num(vb.H))
	body := w.items(s.Items)
	if len(w.defs) > 0 {
		w.b.WriteString("<defs>")
		for _, d := range w.defs {
			w.b.WriteString(d)
		}
		w.b.WriteString("</defs>")
	}
	if !s.Background.IsNone() {
		fmt.Fprintf(&w.b, `<rect x="%s" y="%s" width="%s" height="%s"%s/>`, num(vb.X), num(vb.Y), num(vb.W), num(vb.H), paint("fill", s.Background))
	}
	w.b.WriteString(body)
	w.b.WriteString("</svg>")
	return w.b.String()
}

type svgWriter struct {
	b       strings.Builder
	defs    []string
	shadows map[Shadow]string
}

func (w *svgWriter) items(items []Item) string {
	var b strings.Builder
	for _, it := range items {
		switch it := it.(type) {
		case *Group:
			b.WriteString("<g")
			if !it.Transform.IsIdentity() {
				m := it.Transform
				fmt.Fprintf(&b, ` transform="matrix(%s %s %s %s %s %s)"`, num(m.A), num(m.B), num(m.C), num(m.D), num(m.E), num(m.F))
			}
			if it.Opacity > 0 && it.Opacity < 1 {
				fmt.Fprintf(&b, ` opacity="%s"`, num(it.Opacity))
			}
			if it.Blend != "" {
				fmt.Fprintf(&b, ` style="mix-blend-mode:%s"`, esc(it.Blend))
			}
			b.WriteString(">")
			b.WriteString(w.items(it.Items))
			b.WriteString("</g>")
		case *Path:
			w.path(&b, it)
		case *Text:
			w.text(&b, it)
		}
	}
	return b.String()
}

func (w *svgWriter) path(b *strings.Builder, p *Path) {
	b.WriteString(`<path d="`)
	b.WriteString(PathData(p.Segs))
	b.WriteString(`"`)
	if p.Fill.IsNone() {
		b.WriteString(` fill="none"`)
	} else {
		b.WriteString(paint("fill", p.Fill))
		if p.EvenOdd {
			b.WriteString(` fill-rule="evenodd"`)
		}
	}
	if (!p.Stroke.IsNone() || p.StrokeGradient != nil) && p.StrokeWidth > 0 {
		if g := p.StrokeGradient; g != nil {
			id := fmt.Sprintf("gradient%d", len(w.defs))
			var stops strings.Builder
			for _, s := range g.Stops {
				fmt.Fprintf(&stops, `<stop offset="%s" stop-color="%s" stop-opacity="%s"/>`, num(s.Offset), hex(s.Color), num(s.Color.A))
			}
			w.defs = append(w.defs, fmt.Sprintf(`<linearGradient id="%s" gradientUnits="userSpaceOnUse" x1="%s" y1="%s" x2="%s" y2="%s">%s</linearGradient>`,
				id, num(g.X1), num(g.Y1), num(g.X2), num(g.Y2), stops.String()))
			fmt.Fprintf(b, ` stroke="url(#%s)"`, id)
		} else {
			b.WriteString(paint("stroke", p.Stroke))
		}
		fmt.Fprintf(b, ` stroke-width="%s"`, num(p.StrokeWidth))
		if len(p.Dash) > 0 {
			ds := make([]string, len(p.Dash))
			for i, d := range p.Dash {
				ds[i] = num(d)
			}
			fmt.Fprintf(b, ` stroke-dasharray="%s"`, strings.Join(ds, " "))
		}
		if p.Cap != ButtCap {
			b.WriteString([]string{"", ` stroke-linecap="round"`, ` stroke-linecap="square"`}[p.Cap])
		}
		if p.Join != MiterJoin {
			b.WriteString([]string{"", ` stroke-linejoin="round"`, ` stroke-linejoin="bevel"`}[p.Join])
		}
	}
	if p.Shadow != nil {
		fmt.Fprintf(b, ` filter="url(#%s)"`, w.shadow(*p.Shadow))
	}
	b.WriteString("/>")
}

func (w *svgWriter) shadow(s Shadow) string {
	if id, ok := w.shadows[s]; ok {
		return id
	}
	if w.shadows == nil {
		w.shadows = map[Shadow]string{}
	}
	id := fmt.Sprintf("shadow%d", len(w.shadows))
	w.shadows[s] = id
	w.defs = append(w.defs, fmt.Sprintf(`<filter id="%s" x="-50%%" y="-50%%" width="200%%" height="200%%"><feDropShadow dx="%s" dy="%s" stdDeviation="%s" flood-color="%s" flood-opacity="%s"/></filter>`,
		id, num(s.DX), num(s.DY), num(s.Blur/2), hex(s.Color), num(s.Color.A)))
	return id
}

func (w *svgWriter) text(b *strings.Builder, t *Text) {
	fmt.Fprintf(b, `<text x="%s" y="%s"`, num(t.X), num(t.Y))
	switch t.Anchor {
	case Middle:
		b.WriteString(` text-anchor="middle"`)
	case End:
		b.WriteString(` text-anchor="end"`)
	}
	switch t.Baseline {
	case Central:
		b.WriteString(` dominant-baseline="central"`)
	case MiddleBaseline:
		b.WriteString(` dominant-baseline="middle"`)
	case Hanging:
		b.WriteString(` dominant-baseline="hanging"`)
	}
	if t.Rotate != 0 {
		fmt.Fprintf(b, ` transform="rotate(%s %s %s)"`, num(t.Rotate), num(t.X), num(t.Y))
	}
	fmt.Fprintf(b, ` font-family="%s" font-size="%s"%s`, esc(t.Family), num(t.Size), paint("fill", t.Color))
	// Spaces kept only where they would collapse.
	full := ""
	for _, r := range t.Runs {
		full += r.Text
	}
	if strings.Contains(full, "  ") || strings.TrimSpace(full) != full {
		b.WriteString(` style="white-space:pre"`)
	}
	// One run: its style on the text itself (a styled tspan rasterizes
	// differently in browsers).
	if len(t.Runs) == 1 {
		b.WriteString(spanAttrs(t.Runs[0]))
		b.WriteString(">")
		b.WriteString(esc(t.Runs[0].Text))
		b.WriteString("</text>")
		return
	}
	b.WriteString(">")
	for _, r := range t.Runs {
		b.WriteString("<tspan")
		b.WriteString(spanAttrs(r))
		b.WriteString(">")
		b.WriteString(esc(r.Text))
		b.WriteString("</tspan>")
	}
	b.WriteString("</text>")
}

func spanAttrs(r Span) string {
	var b strings.Builder
	if r.Weight != 0 && r.Weight != 400 {
		fmt.Fprintf(&b, ` font-weight="%d"`, r.Weight)
	}
	if r.Italic {
		b.WriteString(` font-style="italic"`)
	}
	if r.Underline {
		b.WriteString(` text-decoration="underline"`)
	}
	if !r.Color.IsNone() {
		b.WriteString(paint("fill", r.Color))
	}
	return b.String()
}

// PathData is the segments as SVG path data.
func PathData(segs []Seg) string {
	b := make([]byte, 0, len(segs)*16)
	pt := func(b []byte, p Pt) []byte {
		b = appendNum(b, p.X)
		b = append(b, ',')
		return appendNum(b, p.Y)
	}
	for i, s := range segs {
		if i > 0 {
			b = append(b, ' ')
		}
		switch s.Op {
		case MoveTo:
			b = pt(append(b, 'M'), s.P[0])
		case LineTo:
			b = pt(append(b, 'L'), s.P[0])
		case QuadTo:
			b = pt(append(pt(append(b, 'Q'), s.P[0]), ' '), s.P[1])
		case CubeTo:
			b = pt(append(pt(append(pt(append(b, 'C'), s.P[0]), ' '), s.P[1]), ' '), s.P[2])
		case Close:
			b = append(b, 'Z')
		}
	}
	return string(b)
}

func paint(attr string, c Color) string {
	if c.IsNone() {
		return fmt.Sprintf(` %s="none"`, attr)
	}
	s := fmt.Sprintf(` %s="%s"`, attr, hex(c))
	if c.A < 1 {
		s += fmt.Sprintf(` %s-opacity="%s"`, attr, num(c.A))
	}
	return s
}

func hex(c Color) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// num formats a coordinate with at most 3 decimals.
func num(v float64) string { return string(appendNum(nil, v)) }

// appendNum appends v with at most three decimals and no trailing zeros.
func appendNum(b []byte, v float64) []byte {
	start := len(b)
	b = strconv.AppendFloat(b, v, 'f', 3, 64)
	for b[len(b)-1] == '0' {
		b = b[:len(b)-1]
	}
	if b[len(b)-1] == '.' {
		b = b[:len(b)-1]
	}
	if string(b[start:]) == "-0" {
		b = append(b[:start], '0')
	}
	return b
}

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func esc(s string) string { return escaper.Replace(s) }

package render

import (
	"math"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/internal/typesetting/segmenter"

	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

// font is how a label's text is set: the CSS that reaches a label's div.
type font struct {
	Family     string
	Size       float64
	Weight     int
	Italic     bool
	Underline  bool
	Strike     bool
	Color      scene.Color
	LineHeight float64 // multiple of Size; mermaid's divs have 1.5
	Align      scene.Anchor
}

// withCSS is the font with a label style's declarations applied (color,
// font-size, font-weight, font-style, font-family, text-decoration,
// text-align).
func (f font) withCSS(css ...string) font {
	d := style.Parse(css...)
	if v, ok := d["color"]; ok {
		if c := style.Color(v); !c.IsNone() || strings.EqualFold(v, "transparent") {
			f.Color = c
		}
	}
	if v, ok := d["font-size"]; ok {
		f.Size = style.Px(v, f.Size)
	}
	if v, ok := d["font-weight"]; ok {
		f.Weight = cssWeight(v, f.Weight)
	}
	if v, ok := d["font-style"]; ok {
		f.Italic = v == "italic" || v == "oblique"
	}
	if v, ok := d["font-family"]; ok {
		f.Family = v
	}
	if v, ok := d["text-decoration"]; ok {
		f.Underline = strings.Contains(v, "underline")
		f.Strike = strings.Contains(v, "line-through")
	}
	if v, ok := d["text-align"]; ok {
		switch v {
		case "left", "start":
			f.Align = scene.Start
		case "right", "end":
			f.Align = scene.End
		case "center":
			f.Align = scene.Middle
		}
	}
	return f
}

func cssWeight(v string, def int) int {
	switch strings.ToLower(v) {
	case "normal":
		return 400
	case "bold":
		return 700
	case "bolder":
		return 700
	case "lighter":
		return 300
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return def
}

func (f font) style(r run) text.Style {
	w := f.Weight
	if w == 0 {
		w = 400
	}
	if r.Bold {
		w = 700
	}
	return text.Style{Family: f.Family, Size: f.Size, Weight: w, Italic: f.Italic || r.Italic}
}

// Label is a laid out label: its box (the div mermaid measures) and lines.
type Label struct {
	W, H  float64
	font  font
	lines []laidLine
}

type laidLine struct {
	runs []run
	w    float64
}

// labelOpts are createText's options that matter for the layout.
type labelOpts struct {
	// Width is the wrapping width (max-width); 0 or +Inf for none.
	Width float64
	// MinWidth widens a narrower label (centred).
	MinWidth float64
	Markdown bool
}

// layoutLabel lays a label out as mermaid's HTML label div: white-space
// nowrap at its natural width, unless that reaches the wrapping width, in
// which case the div is the wrapping width and the text wraps there
// (break-spaces); <br> and paragraphs break lines; line-height 1.5.
func (r *renderer) layoutLabel(label string, f font, o labelOpts) *Label {
	lines := parseHTML(labelHTML(label, o.Markdown))
	if f.LineHeight == 0 {
		f.LineHeight = 1.5
	}
	if r.svgLabels {
		f.LineHeight = 1.1
	}
	l := &Label{font: f}
	natural := 0.0
	for _, ln := range lines {
		natural = max(natural, r.lineWidth(trimLine(ln), f))
	}
	maxW := o.Width
	if maxW <= 0 || math.IsInf(maxW, 1) {
		maxW = math.Inf(1)
	}
	if natural >= maxW {
		// The div becomes a table maxW wide: a piece that cannot break
		// widens it (its min-content width), and the text wraps at that.
		l.W = maxW
		for _, ln := range lines {
			for _, p := range r.pieces(ln) {
				l.W = max(l.W, r.lineWidth(p, f))
			}
		}
		for _, ln := range lines {
			for _, wl := range r.wrapLine(ln, f, l.W) {
				l.lines = append(l.lines, laidLine{runs: wl, w: r.lineWidth(wl, f)})
			}
		}
	} else {
		l.W = max(natural, o.MinWidth)
		for _, ln := range lines {
			ln = trimLine(ln)
			l.lines = append(l.lines, laidLine{runs: ln, w: r.lineWidth(ln, f)})
		}
	}
	l.H = float64(len(l.lines)) * r.lineHeight(f)
	if r.svgLabels && len(l.lines) > 0 {
		// The text's box: its lines and the first line's -0.1em shift.
		l.H += 1.6 * f.Size / 14
	}
	return l
}

// lineHeight is the CSS line height in px.
func (r *renderer) lineHeight(f font) float64 {
	lh := f.LineHeight
	if lh == 0 {
		lh = 1.5
	}
	return lh * f.Size
}

// lineWidth is a line's advance, run by run.
func (r *renderer) lineWidth(l mline, f font) float64 {
	w := 0.0
	for _, rn := range l {
		w += r.fonts.Width(rn.Text, f.style(rn))
	}
	return w
}

// trimLine drops the spaces at a line's ends (white-space nowrap).
func trimLine(l mline) mline {
	out := append(mline(nil), l...)
	for len(out) > 0 {
		t := strings.TrimLeft(out[0].Text, " ")
		if t != "" {
			out[0].Text = t
			break
		}
		out = out[1:]
	}
	for len(out) > 0 {
		n := len(out) - 1
		t := strings.TrimRight(out[n].Text, " ")
		if t != "" {
			out[n].Text = t
			break
		}
		out = out[:n]
	}
	return out
}

// wrapLine breaks a line where UAX #14 allows, filling lines greedily; a
// piece wider than the width overflows. With white-space break-spaces the
// spaces stay at the end of the line they end and must fit too.
func (r *renderer) wrapLine(l mline, f font, width float64) []mline {
	var out []mline
	var cur mline
	for _, p := range r.pieces(l) {
		if len(cur) > 0 && r.lineWidth(mergeRuns(append(append(mline(nil), cur...), p...)), f) > width {
			out = append(out, mergeRuns(cur))
			cur = nil
		}
		cur = append(cur, p...)
	}
	return append(out, mergeRuns(cur))
}

// pieces splits a line at its UAX #14 break opportunities (each piece
// keeps the spaces that end it).
func (r *renderer) pieces(l mline) []mline {
	var all []rune
	type span struct {
		start, end int
		run        run
	}
	var spans []span
	for _, rn := range l {
		s := len(all)
		all = append(all, []rune(rn.Text)...)
		spans = append(spans, span{s, len(all), rn})
	}
	if len(all) == 0 {
		return nil
	}
	slice := func(a, b int) mline {
		var out mline
		for _, sp := range spans {
			lo, hi := max(a, sp.start), min(b, sp.end)
			if lo < hi {
				rn := sp.run
				rn.Text = string(all[lo:hi])
				out = append(out, rn)
			}
		}
		return out
	}
	var seg segmenter.Segmenter
	seg.Init(all)
	it := seg.LineIterator()
	var out []mline
	pos := 0
	for it.Next() {
		n := len(it.Line().Text)
		out = append(out, slice(pos, pos+n))
		pos += n
	}
	return out
}

// mergeRuns joins neighbouring runs of one style.
func mergeRuns(l mline) mline {
	var out mline
	for _, rn := range l {
		if n := len(out); n > 0 && out[n-1].Bold == rn.Bold && out[n-1].Italic == rn.Italic &&
			out[n-1].Underline == rn.Underline && out[n-1].Strike == rn.Strike {
			out[n-1].Text += rn.Text
			continue
		}
		out = append(out, rn)
	}
	return out
}

func trimRight(l mline) mline {
	out := append(mline(nil), l...)
	for len(out) > 0 {
		n := len(out) - 1
		t := strings.TrimRight(out[n].Text, " ")
		if t != "" {
			out[n].Text = t
			break
		}
		out = out[:n]
	}
	return out
}

// draw is the label's text with its box's top left corner at (x, y).
func (r *renderer) drawLabel(l *Label, x, y float64) []scene.Item {
	if l == nil {
		return nil
	}
	f := l.font
	lh := r.lineHeight(f)
	asc, desc := r.fonts.Extents(f.style(run{}))
	// The baseline sits half the leading below the line's top, as CSS
	// places a line box's content.
	base := asc + math.Floor((lh-asc-desc)/2)
	var items []scene.Item
	for i, ln := range l.lines {
		if len(ln.runs) == 0 {
			continue
		}
		lx := x
		switch f.Align {
		case scene.Middle:
			lx += (l.W - ln.w) / 2
		case scene.End:
			lx += l.W - ln.w
		}
		t := &scene.Text{X: lx, Y: y + float64(i)*lh + base, Family: f.Family, Size: f.Size, Color: f.Color}
		for _, rn := range ln.runs {
			st := f.style(rn)
			t.Runs = append(t.Runs, scene.Span{Text: rn.Text, Weight: st.Weight, Italic: st.Italic,
				Underline: rn.Underline || f.Underline})
		}
		items = append(items, t)
	}
	return items
}

// Package treemap is mermaid's treemap (diagrams/treemap): sections and
// leaves nested by indentation, laid out by d3's squarified treemap.
package treemap

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/d3"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

func init() { diagram.Register("treemap", Render) }

// Item is a section (with children) or a leaf (with a value).
type Item struct {
	Name     string
	Value    float64
	Leaf     bool
	Class    string
	Children []*Item
}

// Tree is a parsed treemap.
type Tree struct {
	Title   string
	Items   []*Item
	Classes map[string]style.Decls
}

var (
	rowRe      = regexp.MustCompile(`^("[^"]*"|'[^']*')\s*(?:[:,]\s*([0-9_.,]+))?\s*(?::::([a-zA-Z_][a-zA-Z0-9_]*))?\s*$`)
	classDefRe = regexp.MustCompile(`^classDef\s+([A-Z_a-z]\w+)(?:\s+([^\n\r;]*))?;?$`)
)

// Parse reads a treemap (treemap.langium): rows nest under the nearest
// section above them that is indented less.
func Parse(code string) (*Tree, error) {
	t := &Tree{Classes: map[string]style.Decls{}}
	type open struct {
		item   *Item
		indent int
	}
	var stack []open
	started := false
	for n, line := range strings.Split(code, "\n") {
		if i := strings.Index(line, "%%"); i >= 0 {
			line = line[:i]
		}
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		if !started {
			if l != "treemap" && l != "treemap-beta" {
				return nil, fmt.Errorf("treemap: line %d: expected treemap", n+1)
			}
			started = true
			continue
		}
		switch {
		case l == "title" || strings.HasPrefix(l, "title ") || strings.HasPrefix(l, "title\t"):
			t.Title = strings.TrimSpace(l[5:])
			continue
		case strings.HasPrefix(l, "accTitle") || strings.HasPrefix(l, "accDescr"):
			continue
		}
		if m := classDefRe.FindStringSubmatch(l); m != nil {
			// "\," keeps a comma; other commas separate declarations.
			s := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(m[2], `\,`, "\x00"), ",", ";"), "\x00", ",")
			t.Classes[m[1]] = style.Parse(s)
			continue
		}
		m := rowRe.FindStringSubmatch(l)
		if m == nil {
			return nil, fmt.Errorf("treemap: line %d: unexpected %q", n+1, l)
		}
		it := &Item{Name: m[1][1 : len(m[1])-1], Class: m[3]}
		if m[2] != "" {
			it.Leaf = true
			it.Value, _ = strconv.ParseFloat(strings.ReplaceAll(m[2], ",", ""), 64)
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			t.Items = append(t.Items, it)
		} else {
			p := stack[len(stack)-1].item
			p.Children = append(p.Children, it)
		}
		if !it.Leaf {
			stack = append(stack, open{it, indent})
		}
	}
	if !started {
		return nil, fmt.Errorf("treemap: empty")
	}
	return t, nil
}

const (
	headerH      = 25.0
	sectionPad   = 10.0
	defaultInner = 10.0
)

// Render draws a treemap.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	tr, err := Parse(code)
	if err != nil {
		return nil, err
	}
	if tr.Title == "" {
		tr.Title = ctx.Title
	}
	if len(tr.Items) == 0 {
		return &scene.Scene{}, nil
	}
	cfg, th := ctx.Config, ctx.Theme
	family := th.Get("fontFamily")
	format, err := valueFormat(cfg.Str("treemap.valueFormat", ","))
	if err != nil {
		format, _ = d3.Format(",")
	}
	showValues := cfg.Bool("treemap.showValues", true)
	inner := cfg.Num("treemap.padding", defaultInner)
	w, h := 960.0, 500.0
	if v := cfg.Num("treemap.nodeWidth", 0); v != 0 {
		w = v * sectionPad
	}
	if v := cfg.Num("treemap.nodeHeight", 0); v != 0 {
		h = v * sectionPad
	}
	titleH := 0.0
	if tr.Title != "" {
		titleH = 30
	}

	// The hierarchy, under a nameless root.
	var build func(it *Item) *d3.Node
	build = func(it *Item) *d3.Node {
		n := &d3.Node{Data: it}
		if it.Leaf {
			n.Value = it.Value
		}
		for _, c := range it.Children {
			n.Children = append(n.Children, build(c))
		}
		return n
	}
	root := build(&Item{Children: tr.Items})
	root.Sum()
	branch := func(n *d3.Node) float64 {
		if len(n.Children) > 0 {
			return sectionPad
		}
		return 0
	}
	layout := d3.Treemap{W: w, H: h, Round: true,
		PaddingTop:  func(n *d3.Node) float64 { return map[bool]float64{true: headerH + sectionPad}[len(n.Children) > 0] },
		PaddingLeft: branch, PaddingRight: branch, PaddingBottom: branch,
		PaddingInner: func(*d3.Node) float64 { return inner },
	}
	layout.Layout(root)

	// d3's ordinal scales take names in the order first asked for.
	scale := func(rng []string) func(string) string {
		idx := map[string]int{}
		return func(name string) string {
			i, ok := idx[name]
			if !ok {
				i = len(idx)
				idx[name] = i
			}
			return rng[i%len(rng)]
		}
	}
	cs := func(prefix string, transparent bool) []string {
		var out []string
		if transparent {
			out = append(out, "transparent")
		}
		for i := range 12 {
			out = append(out, th.Get(fmt.Sprintf("%s%d", prefix, i)))
		}
		return out
	}
	fillOf, peerOf, labelOf := scale(cs("cScale", true)), scale(cs("cScalePeer", true)), scale(cs("cScaleLabel", false))

	var branches, leaves []*d3.Node
	// d3's descendants() is breadth-first; leaves() depth-first.
	for queue := []*d3.Node{root}; len(queue) > 0; queue = queue[1:] {
		if len(queue[0].Children) > 0 {
			branches = append(branches, queue[0])
		}
		queue = append(queue, queue[0].Children...)
	}
	root.Each(func(n *d3.Node) {
		if len(n.Children) == 0 {
			leaves = append(leaves, n)
		}
	})
	item := func(n *d3.Node) *Item { return n.Data.(*Item) }
	decls := func(n *d3.Node) style.Decls { return tr.Classes[item(n).Class] }
	// Section colors are taken in their own order before the leaves'.
	sectionFill := map[*d3.Node]string{}
	sectionPeer := map[*d3.Node]string{}
	for _, b := range branches {
		sectionFill[b] = fillOf(item(b).Name)
	}
	for _, b := range branches {
		sectionPeer[b] = peerOf(item(b).Name)
	}

	g := &scene.Group{Transform: scene.Translate(0, titleH)}
	labelStyle := text.Style{Family: family, Size: 12, Weight: 700}
	for _, b := range branches {
		if b.Depth == 0 {
			continue
		}
		bw, bh := b.X1-b.X0, b.Y1-b.Y0
		d := decls(b)
		fill := style.Color(sectionFill[b])
		fill.A *= 0.6
		stroke := style.Color(peerOf(item(b).Name))
		stroke.A *= 0.4
		strokeW := 2.0
		// A class's colors keep the cell's opacities.
		if v, ok := d["fill"]; ok {
			fill = style.Color(v)
			fill.A *= 0.6
		}
		if v, ok := d["stroke"]; ok {
			stroke = style.Color(v)
			stroke.A *= 0.4
		}
		if v, ok := d["stroke-width"]; ok {
			strokeW = style.Px(v, strokeW)
		}
		sg := &scene.Group{Transform: scene.Translate(b.X0, b.Y0), Items: []scene.Item{
			&scene.Path{Segs: scene.RectPath(0, 0, bw, bh, 0, 0), Fill: fill, Stroke: stroke, StrokeWidth: strokeW},
		}}
		lc := labelOf(item(b).Name)
		if v, ok := d["color"]; ok {
			lc = v
		}
		// The label gives way to the value, and is cut with an ellipsis.
		space := bw - 6 - 6
		if showValues && b.Value != 0 {
			space = bw - 10 - 30 - 10 - 6
		}
		name := ellipsize(ctx.Fonts, item(b).Name, labelStyle, math.Max(15, space))
		sg.Items = append(sg.Items, &scene.Text{X: 6, Y: headerH / 2, Baseline: scene.MiddleBaseline, Family: family, Size: 12,
			Color: style.Color(lc), Runs: []scene.Span{{Text: name, Weight: 700}}})
		if showValues && b.Value != 0 {
			sg.Items = append(sg.Items, &scene.Text{X: bw - 10, Y: headerH / 2, Anchor: scene.End, Baseline: scene.MiddleBaseline,
				Family: family, Size: 10, Color: style.Color(lc), Runs: []scene.Span{{Text: format(b.Value), Italic: true}}})
		}
		g.Items = append(g.Items, sg)
	}

	complex := len(leaves) > 20
	pick := func(a, b float64) float64 {
		if complex {
			return a
		}
		return b
	}
	baseLabel, baseValue := pick(16, 38), pick(14, 28)
	minLabel, minValue := pick(4, 8), pick(4, 6)
	pad, threshold, gap := pick(2, 4), pick(8, 10), pick(1, 2)
	for _, l := range leaves {
		lw, lh := l.X1-l.X0, l.Y1-l.Y0
		parentName := item(l).Name
		if l.Parent != nil {
			parentName = item(l.Parent).Name
		}
		d := decls(l)
		col := style.Color(fillOf(parentName))
		fill := col
		fill.A *= 0.3
		if v, ok := d["fill"]; ok {
			fill = style.Color(v)
			fill.A *= 0.3
		}
		stroke, strokeW := col, 3.0
		if v, ok := d["stroke"]; ok {
			stroke = style.Color(v)
		}
		if v, ok := d["stroke-width"]; ok {
			strokeW = style.Px(v, strokeW)
		}
		cg := &scene.Group{Transform: scene.Translate(l.X0, l.Y0), Items: []scene.Item{
			&scene.Path{Segs: scene.RectPath(0, 0, lw, lh, 0, 0), Fill: fill, Stroke: stroke, StrokeWidth: strokeW},
		}}
		lc := labelOf(item(l).Name)
		if v, ok := d["color"]; ok {
			lc = v
		}
		aw, ah := lw-2*pad, lh-2*pad
		size, shown := fitLabel(ctx.Fonts, item(l).Name, family, aw, ah, baseLabel, baseValue, minLabel, minValue, gap, threshold, complex)
		if shown {
			cg.Items = append(cg.Items, &scene.Text{X: lw / 2, Y: lh / 2, Anchor: scene.Middle, Baseline: scene.MiddleBaseline,
				Family: family, Size: size, Color: style.Color(lc), Runs: []scene.Span{{Text: item(l).Name}}})
			if showValues && l.Value != 0 {
				vs := math.Max(minValue, math.Min(baseValue, math.Round(size*0.6)))
				vy := lh/2 + size/2 + gap
				v := format(l.Value)
				if ctx.Fonts.Width(v, text.Style{Family: family, Size: vs}) <= aw && vy+vs <= lh-4 {
					cg.Items = append(cg.Items, &scene.Text{X: lw / 2, Y: vy, Anchor: scene.Middle, Baseline: scene.Hanging,
						Family: family, Size: vs, Color: style.Color(lc), Runs: []scene.Span{{Text: v}}})
				}
			}
		}
		g.Items = append(g.Items, cg)
	}
	items := []scene.Item{g}
	if tr.Title != "" {
		tc := th.Get("treemap.titleColor")
		if tc == "" {
			tc = th.Get("titleColor")
		}
		items = append([]scene.Item{&scene.Text{X: w / 2, Y: titleH / 2, Anchor: scene.Middle, Baseline: scene.MiddleBaseline,
			Family: family, Size: 14, Color: style.Color(tc), Runs: []scene.Span{{Text: tr.Title}}}}, items...)
	}
	return ctx.Fit(items, cfg.Num("treemap.diagramPadding", 8)), nil
}

// fitLabel shrinks a leaf's label to fit its cell, with room for the value
// under it; false when it cannot be shown.
func fitLabel(fs *text.Fonts, s, family string, aw, ah, base, baseValue, minLabel, minValue, gap, threshold float64, complex bool) (float64, bool) {
	if aw < threshold || ah < threshold {
		return 0, false
	}
	size := base
	width := func() float64 { return fs.Width(s, text.Style{Family: family, Size: size}) }
	for width() > aw && size > minLabel {
		size--
	}
	value := func() float64 { return math.Max(minValue, math.Min(baseValue, math.Round(size*0.6))) }
	for size+gap+value() > ah && size > minLabel {
		size--
	}
	if complex {
		return size, size >= minLabel && ah >= minLabel
	}
	return size, width() <= aw && size >= minLabel && ah >= size
}

// ellipsize cuts s with "..." until it fits width.
func ellipsize(fs *text.Fonts, s string, st text.Style, width float64) string {
	if fs.Width(s, st) <= width {
		return s
	}
	rs := []rune(s)
	for n := len(rs) - 1; n > 0; n-- {
		if t := string(rs[:n]) + "..."; fs.Width(t, st) <= width {
			return t
		}
	}
	if fs.Width("...", st) <= width {
		return "..."
	}
	return ""
}

// valueFormat is mermaid's treemap value format: d3-format, with "$"
// prefixes handled before it.
func valueFormat(spec string) (func(float64) string, error) {
	dollar := func(f func(float64) string, err error) (func(float64) string, error) {
		if err != nil {
			return nil, err
		}
		return func(v float64) string { return "$" + f(v) }, nil
	}
	switch {
	case spec == "$0,0":
		return dollar(d3.Format(","))
	case strings.HasPrefix(spec, "$") && strings.Contains(spec, ","):
		prec := regexp.MustCompile(`\.\d+`).FindString(spec)
		return dollar(d3.Format("," + prec))
	case strings.HasPrefix(spec, "$"):
		return dollar(d3.Format(spec[1:]))
	}
	return d3.Format(spec)
}

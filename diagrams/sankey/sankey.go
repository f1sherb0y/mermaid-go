// Package sankey is mermaid's sankey diagram (diagrams/sankey): flows given
// as CSV rows "source,target,value", laid out by d3-sankey.
package sankey

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

func init() { diagram.Register("sankey", Render) }

// Node is a node; positions are set by layout.
type Node struct {
	ID               string
	Value            float64
	Depth, Height    int
	Layer            int
	X0, X1, Y0, Y1   float64
	Sources, Targets []*Link // links out of and into the node
	index            int
}

// Link is a flow.
type Link struct {
	Source, Target *Node
	Value, Width   float64
	Y0, Y1         float64
	index          int
}

// Graph is a parsed sankey diagram.
type Graph struct {
	Nodes []*Node
	Links []*Link
}

// Parse reads `sankey` (or `sankey-beta`) and CSV rows (sankey.jison):
// fields may be "quoted", with "" for a quote.
func Parse(code string) (*Graph, error) {
	lines := strings.Split(code, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i == len(lines) {
		return nil, fmt.Errorf("sankey: empty")
	}
	if h := strings.TrimSpace(lines[i]); h != "sankey" && h != "sankey-beta" {
		return nil, fmt.Errorf("sankey: line %d: expected sankey", i+1)
	}
	g := &Graph{}
	byID := map[string]*Node{}
	node := func(id string) *Node {
		id = strings.ReplaceAll(strings.TrimSpace(id), `""`, `"`)
		n := byID[id]
		if n == nil {
			n = &Node{ID: id, index: len(g.Nodes)}
			byID[id] = n
			g.Nodes = append(g.Nodes, n)
		}
		return n
	}
	// Quoted fields may span lines: read the rest as one stream.
	rest := strings.Join(lines[i+1:], "\n")
	for row := 0; ; row++ {
		rest = strings.TrimLeft(rest, " \t\r\n")
		if rest == "" {
			break
		}
		var fields []string
		for len(fields) < 3 {
			f, r, err := field(rest)
			if err != nil {
				return nil, fmt.Errorf("sankey: row %d: %v", row+1, err)
			}
			fields, rest = append(fields, f), r
			if len(fields) < 3 {
				if !strings.HasPrefix(rest, ",") {
					return nil, fmt.Errorf("sankey: row %d: expected 3 fields", row+1)
				}
				rest = rest[1:]
			}
		}
		if end := strings.IndexByte(rest, '\n'); end >= 0 {
			if strings.TrimSpace(rest[:end]) != "" {
				return nil, fmt.Errorf("sankey: row %d: more than 3 fields", row+1)
			}
			rest = rest[end+1:]
		} else if strings.TrimSpace(rest) != "" {
			return nil, fmt.Errorf("sankey: row %d: more than 3 fields", row+1)
		} else {
			rest = ""
		}
		v, _ := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
		s, t := node(fields[0]), node(fields[1])
		l := &Link{Source: s, Target: t, Value: v, index: len(g.Links)}
		g.Links = append(g.Links, l)
	}
	return g, nil
}

// field reads one CSV field.
func field(s string) (string, string, error) {
	t := strings.TrimLeft(s, " \t")
	if strings.HasPrefix(t, `"`) {
		var b strings.Builder
		for i := 1; i < len(t); i++ {
			if t[i] == '"' {
				if i+1 < len(t) && t[i+1] == '"' {
					b.WriteString(`""`)
					i++
					continue
				}
				return b.String(), t[i+1:], nil
			}
			b.WriteByte(t[i])
		}
		return "", "", fmt.Errorf("unterminated quote")
	}
	end := strings.IndexAny(s, ",\n")
	if end < 0 {
		end = len(s)
	}
	return s[:end], s[end:], nil
}

// layout is d3-sankey's: columns by depth, aligned as nodeAlign says, then
// six rounds of relaxing nodes toward their links and resolving overlaps.
func layout(g *Graph, w, h, nodeW, padding float64, align string) error {
	for _, l := range g.Links {
		l.Source.Sources = append(l.Source.Sources, l)
		l.Target.Targets = append(l.Target.Targets, l)
	}
	for _, n := range g.Nodes {
		var in, out float64
		for _, l := range n.Sources {
			out += l.Value
		}
		for _, l := range n.Targets {
			in += l.Value
		}
		n.Value = math.Max(out, in)
	}
	// Depths forwards, heights backwards; sets keep insertion order.
	walk := func(next func(*Node) []*Node, set func(*Node, int)) error {
		cur := g.Nodes
		for x := 0; len(cur) > 0; x++ {
			if x > len(g.Nodes) {
				return fmt.Errorf("circular link")
			}
			var nxt []*Node
			seen := map[*Node]bool{}
			for _, n := range cur {
				set(n, x)
				for _, m := range next(n) {
					if !seen[m] {
						seen[m] = true
						nxt = append(nxt, m)
					}
				}
			}
			cur = nxt
		}
		return nil
	}
	if err := walk(func(n *Node) []*Node {
		out := make([]*Node, len(n.Sources))
		for i, l := range n.Sources {
			out[i] = l.Target
		}
		return out
	}, func(n *Node, x int) { n.Depth = x }); err != nil {
		return err
	}
	if err := walk(func(n *Node) []*Node {
		out := make([]*Node, len(n.Targets))
		for i, l := range n.Targets {
			out[i] = l.Source
		}
		return out
	}, func(n *Node, x int) { n.Height = x }); err != nil {
		return err
	}

	// Columns.
	cols := 0
	for _, n := range g.Nodes {
		cols = max(cols, n.Depth+1)
	}
	kx := (w - nodeW) / float64(cols-1)
	columns := make([][]*Node, cols)
	for _, n := range g.Nodes {
		var i int
		switch align {
		case "left":
			i = n.Depth
		case "right":
			i = cols - 1 - n.Height
		case "center":
			switch {
			case len(n.Targets) > 0:
				i = n.Depth
			case len(n.Sources) > 0:
				i = math.MaxInt
				for _, l := range n.Sources {
					i = min(i, l.Target.Depth)
				}
				i--
			}
		default: // justify
			i = cols - 1
			if len(n.Sources) > 0 {
				i = n.Depth
			}
		}
		i = max(0, min(cols-1, i))
		n.Layer = i
		n.X0 = float64(i) * kx
		n.X1 = n.X0 + nodeW
		columns[i] = append(columns[i], n)
	}
	most := 0
	for _, c := range columns {
		most = max(most, len(c))
	}
	py := math.Min(padding, h/float64(most-1))

	byTarget := func(a, b *Link) int {
		if d := a.Target.Y0 - b.Target.Y0; d != 0 {
			return cmpF(d)
		}
		return a.index - b.index
	}
	bySource := func(a, b *Link) int {
		if d := a.Source.Y0 - b.Source.Y0; d != 0 {
			return cmpF(d)
		}
		return a.index - b.index
	}
	// Initial breadths.
	ky := math.Inf(1)
	for _, c := range columns {
		sum := 0.0
		for _, n := range c {
			sum += n.Value
		}
		ky = math.Min(ky, (h-float64(len(c)-1)*py)/sum)
	}
	for _, c := range columns {
		y := 0.0
		for _, n := range c {
			n.Y0, n.Y1 = y, y+n.Value*ky
			y = n.Y1 + py
			for _, l := range n.Sources {
				l.Width = l.Value * ky
			}
		}
		y = (h - y + py) / float64(len(c)+1)
		for i, n := range c {
			n.Y0 += y * float64(i+1)
			n.Y1 += y * float64(i+1)
		}
		for _, n := range c {
			slices.SortStableFunc(n.Sources, byTarget)
			slices.SortStableFunc(n.Targets, bySource)
		}
	}
	reorder := func(n *Node) {
		for _, l := range n.Targets {
			slices.SortStableFunc(l.Source.Sources, byTarget)
		}
		for _, l := range n.Sources {
			slices.SortStableFunc(l.Target.Targets, bySource)
		}
	}
	targetTop := func(s, t *Node) float64 {
		y := s.Y0 - float64(len(s.Sources)-1)*py/2
		for _, l := range s.Sources {
			if l.Target == t {
				break
			}
			y += l.Width + py
		}
		for _, l := range t.Targets {
			if l.Source == s {
				break
			}
			y -= l.Width
		}
		return y
	}
	sourceTop := func(s, t *Node) float64 {
		y := t.Y0 - float64(len(t.Targets)-1)*py/2
		for _, l := range t.Targets {
			if l.Source == s {
				break
			}
			y += l.Width + py
		}
		for _, l := range s.Sources {
			if l.Target == t {
				break
			}
			y -= l.Width
		}
		return y
	}
	resolve := func(c []*Node, alpha float64) {
		down := func(y float64, i int) {
			for ; i < len(c); i++ {
				if d := (y - c[i].Y0) * alpha; d > 1e-6 {
					c[i].Y0 += d
					c[i].Y1 += d
				}
				y = c[i].Y1 + py
			}
		}
		up := func(y float64, i int) {
			for ; i >= 0; i-- {
				if d := (c[i].Y1 - y) * alpha; d > 1e-6 {
					c[i].Y0 -= d
					c[i].Y1 -= d
				}
				y = c[i].Y0 - py
			}
		}
		i := len(c) >> 1
		up(c[i].Y0-py, i-1)
		down(c[i].Y1+py, i+1)
		up(h, len(c)-1)
		down(0, 0)
	}
	byY := func(a, b *Node) int { return cmpF(a.Y0 - b.Y0) }
	const iterations = 6
	for it := range iterations {
		alpha := math.Pow(0.99, float64(it))
		beta := math.Max(1-alpha, float64(it+1)/iterations)
		for i := len(columns) - 2; i >= 0; i-- {
			for _, s := range columns[i] {
				var y, wsum float64
				for _, l := range s.Sources {
					v := l.Value * float64(l.Target.Layer-s.Layer)
					y += sourceTop(s, l.Target) * v
					wsum += v
				}
				if !(wsum > 0) {
					continue
				}
				d := (y/wsum - s.Y0) * alpha
				s.Y0 += d
				s.Y1 += d
				reorder(s)
			}
			slices.SortStableFunc(columns[i], byY)
			resolve(columns[i], beta)
		}
		for i := 1; i < len(columns); i++ {
			for _, t := range columns[i] {
				var y, wsum float64
				for _, l := range t.Targets {
					v := l.Value * float64(t.Layer-l.Source.Layer)
					y += targetTop(l.Source, t) * v
					wsum += v
				}
				if !(wsum > 0) {
					continue
				}
				d := (y/wsum - t.Y0) * alpha
				t.Y0 += d
				t.Y1 += d
				reorder(t)
			}
			slices.SortStableFunc(columns[i], byY)
			resolve(columns[i], beta)
		}
	}
	// Link ends.
	for _, n := range g.Nodes {
		y0, y1 := n.Y0, n.Y0
		for _, l := range n.Sources {
			l.Y0 = y0 + l.Width/2
			y0 += l.Width
		}
		for _, l := range n.Targets {
			l.Y1 = y1 + l.Width/2
			y1 += l.Width
		}
	}
	return nil
}

func cmpF(d float64) int {
	switch {
	case d < 0:
		return -1
	case d > 0:
		return 1
	}
	return 0
}

// tableau10 is d3.schemeTableau10.
var tableau10 = []string{"#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f", "#edc949", "#af7aa1", "#ff9da7", "#9c755f", "#bab0ab"}

// Render draws a sankey diagram.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	g, err := Parse(code)
	if err != nil {
		return nil, err
	}
	cfg := ctx.Config
	w, h := cfg.Num("sankey.width", 600), cfg.Num("sankey.height", 400)
	showValues := cfg.Bool("sankey.showValues", true)
	prefix, suffix := cfg.Str("sankey.prefix", ""), cfg.Str("sankey.suffix", "")
	padding := cfg.Num("sankey.nodePadding", 12)
	if showValues {
		padding += 15
	}
	if len(g.Nodes) > 0 {
		if err := layout(g, w, h, cfg.Num("sankey.nodeWidth", 10), padding, cfg.Str("sankey.nodeAlignment", "justify")); err != nil {
			return nil, err
		}
	}
	colors := map[string]scene.Color{}
	custom := cfg.Map("sankey.nodeColors")
	colorOf := func(id string) scene.Color {
		if c, ok := custom[id].(string); ok {
			return style.Color(c)
		}
		if c, ok := colors[id]; ok {
			return c
		}
		c := style.Color(tableau10[len(colors)%len(tableau10)])
		colors[id] = c
		return c
	}
	var nodes, labels, links []scene.Item
	for _, n := range g.Nodes {
		nodes = append(nodes, &scene.Path{Segs: scene.RectPath(n.X0, n.Y0, n.X1-n.X0, n.Y1-n.Y0, 0, 0), Fill: colorOf(n.ID)})
	}
	family := ctx.Theme.Get("fontFamily")
	central, maxV := 0, 0.0
	for _, n := range g.Nodes {
		if n.Value > maxV {
			maxV, central = n.Value, n.Layer
		}
	}
	outlined := cfg.Str("sankey.labelStyle", "legacy") == "outlined"
	textColor := style.Color(ctx.Theme.Get("textColor"))
	for _, n := range g.Nodes {
		x, anchor := n.X1+6, scene.Start
		if (outlined && n.Layer < central) || (!outlined && n.X0 >= w/2) {
			x, anchor = n.X0-6, scene.End
		}
		label := n.ID
		y := (n.Y0 + n.Y1) / 2
		if showValues {
			// The value follows the name on one line ("\n" collapses in SVG).
			label += " " + prefix + strconv.FormatFloat(math.Round(n.Value*100)/100, 'f', -1, 64) + suffix
		} else {
			y += 0.35 * 14
		}
		labels = append(labels, &scene.Text{X: x, Y: y, Anchor: anchor, Family: family, Size: 14, Color: textColor, Runs: []scene.Span{{Text: label}}})
	}
	linkColor := cfg.Str("sankey.linkColor", "gradient")
	for _, l := range g.Links {
		x0, x1 := l.Source.X1, l.Target.X0
		mx := (x0 + x1) / 2
		var b scene.PathBuilder
		b.MoveTo(x0, l.Y0).CubeTo(mx, l.Y0, mx, l.Y1, x1, l.Y1)
		p := &scene.Path{Segs: b.Segs, StrokeWidth: math.Max(1, l.Width)}
		switch linkColor {
		case "gradient":
			sc, tc := colorOf(l.Source.ID), colorOf(l.Target.ID)
			sc.A, tc.A = sc.A*0.5, tc.A*0.5
			p.StrokeGradient = &scene.Gradient{X1: x0, X2: x1, Stops: []scene.Stop{{Offset: 0, Color: sc}, {Offset: 1, Color: tc}}}
		case "source":
			p.Stroke = colorOf(l.Source.ID)
			p.Stroke.A *= 0.5
		case "target":
			p.Stroke = colorOf(l.Target.ID)
			p.Stroke.A *= 0.5
		default:
			p.Stroke = style.Color(linkColor)
			p.Stroke.A *= 0.5
		}
		links = append(links, &scene.Group{Blend: "multiply", Items: []scene.Item{p}})
	}
	items := append(append(nodes, labels...), links...)
	return ctx.Fit(items, 0), nil
}

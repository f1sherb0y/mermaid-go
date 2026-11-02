package icons

import (
	"encoding/xml"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

// node is an element of an icon's body.
type node struct {
	name     string
	attrs    map[string]string
	children []*node
	text     string
}

// parseBody reads SVG markup into elements; what XML cannot read is dropped.
func parseBody(body string) *node {
	d := xml.NewDecoder(strings.NewReader("<svg>" + body + "</svg>"))
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity
	root := &node{name: "root", attrs: map[string]string{}}
	stack := []*node{root}
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		top := stack[len(stack)-1]
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			top.children = append(top.children, n)
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			top.text += string(t)
		}
	}
	return root
}

// paintState is what an element inherits.
type paintState struct {
	fill, stroke               scene.Color
	strokeWidth                float64
	fillOpacity, strokeOpacity float64
	opacity                    float64
	evenOdd                    bool
	fontFamily                 string
	fontSize                   float64
	fontWeight                 int
	dash                       []float64
	cap                        scene.Cap
	join                       scene.Join
}

// drawer turns elements into scene items.
type drawer struct {
	ids     map[string]*node
	current scene.Color // currentColor
}

// drawBody draws an icon body; unpainted shapes get fill.
func drawBody(body string, fill scene.Color) []scene.Item {
	root := parseBody(body)
	dr := &drawer{ids: map[string]*node{}, current: scene.RGB(0, 0, 0)}
	var index func(n *node)
	index = func(n *node) {
		if id := n.attrs["id"]; id != "" {
			dr.ids[id] = n
		}
		for _, c := range n.children {
			index(c)
		}
	}
	index(root)
	st := paintState{fill: fill, strokeWidth: 1, fillOpacity: 1, strokeOpacity: 1, opacity: 1, fontSize: 16}
	return dr.children(root, st, 0)
}

func (dr *drawer) children(n *node, st paintState, depth int) []scene.Item {
	var out []scene.Item
	for _, c := range n.children {
		out = append(out, dr.element(c, st, depth)...)
	}
	return out
}

// element draws one element and what it contains.
func (dr *drawer) element(n *node, st paintState, depth int) []scene.Item {
	if depth > 20 {
		return nil
	}
	switch n.name {
	case "defs", "clipPath", "mask", "symbol", "title", "desc", "metadata", "style", "pattern", "filter",
		"linearGradient", "radialGradient", "script", "foreignObject":
		return nil
	}
	st = dr.inherit(n, st)
	var items []scene.Item
	a := func(k string) float64 { return style.Px(n.attrs[k], 0) }
	switch n.name {
	case "g", "svg", "a", "switch":
		items = dr.children(n, st, depth+1)
	case "use":
		href := n.attrs["href"]
		if ref := dr.ids[strings.TrimPrefix(href, "#")]; ref != nil && strings.HasPrefix(href, "#") {
			items = []scene.Item{&scene.Group{Transform: scene.Translate(a("x"), a("y")), Items: dr.element(ref, st, depth+1)}}
		}
	case "path":
		items = dr.shape(scene.ParsePath(n.attrs["d"]), st, true)
	case "rect":
		w, h := a("width"), a("height")
		if w > 0 && h > 0 {
			rx, ry := a("rx"), a("ry")
			items = dr.shape(scene.RectPath(a("x"), a("y"), w, h, rx, ry), st, true)
		}
	case "circle":
		if r := a("r"); r > 0 {
			items = dr.shape(scene.Ellipse(a("cx"), a("cy"), r, r), st, true)
		}
	case "ellipse":
		if rx, ry := a("rx"), a("ry"); rx > 0 && ry > 0 {
			items = dr.shape(scene.Ellipse(a("cx"), a("cy"), rx, ry), st, true)
		}
	case "line":
		var b scene.PathBuilder
		b.MoveTo(a("x1"), a("y1")).LineTo(a("x2"), a("y2"))
		items = dr.shape(b.Segs, st, false)
	case "polygon", "polyline":
		f := numbers(n.attrs["points"])
		var b scene.PathBuilder
		for i := 0; i+1 < len(f); i += 2 {
			if i == 0 {
				b.MoveTo(f[i], f[i+1])
			} else {
				b.LineTo(f[i], f[i+1])
			}
		}
		if n.name == "polygon" && len(b.Segs) > 0 {
			b.Close()
		}
		items = dr.shape(b.Segs, st, n.name == "polygon")
	case "text":
		items = dr.text(n, st)
	}
	if m, ok := transform(n.attrs["transform"]); ok && len(items) > 0 {
		return []scene.Item{&scene.Group{Transform: m, Items: items}}
	}
	return items
}

// shape paints segments; fillable is false for lines, which SVG never fills.
func (dr *drawer) shape(segs []scene.Seg, st paintState, fillable bool) []scene.Item {
	if len(segs) == 0 {
		return nil
	}
	p := &scene.Path{Segs: segs, EvenOdd: st.evenOdd, Dash: st.dash, Cap: st.cap, Join: st.join}
	if fillable {
		p.Fill = alpha(st.fill, st.fillOpacity*st.opacity)
	}
	if st.strokeWidth > 0 {
		p.Stroke = alpha(st.stroke, st.strokeOpacity*st.opacity)
		p.StrokeWidth = st.strokeWidth
	}
	if p.Fill.IsNone() && p.Stroke.IsNone() {
		return nil
	}
	return []scene.Item{p}
}

// text draws a <text> and its <tspan>s (each its own line at its x, y).
func (dr *drawer) text(n *node, st paintState) []scene.Item {
	var out []scene.Item
	put := func(m *node, st paintState, x, y float64) {
		s := strings.TrimSpace(m.text)
		if s == "" || st.fill.IsNone() {
			return
		}
		out = append(out, &scene.Text{X: x, Y: y, Family: st.fontFamily, Size: st.fontSize,
			Color: alpha(st.fill, st.fillOpacity*st.opacity), Runs: []scene.Span{{Text: s, Weight: st.fontWeight}},
			Anchor: anchor(m.attrs["text-anchor"])})
	}
	x, y := style.Px(n.attrs["x"], 0), style.Px(n.attrs["y"], 0)
	put(n, st, x, y)
	for _, c := range n.children {
		if c.name == "tspan" {
			cst := dr.inherit(c, st)
			put(c, cst, style.Px(c.attrs["x"], x), style.Px(c.attrs["y"], y))
		}
	}
	return out
}

func anchor(s string) scene.Anchor {
	switch s {
	case "middle":
		return scene.Middle
	case "end":
		return scene.End
	}
	return scene.Start
}

// inherit applies an element's presentation attributes, then its style
// attribute, over what it inherits.
func (dr *drawer) inherit(n *node, st paintState) paintState {
	props := map[string]string{}
	for k, v := range n.attrs {
		props[k] = v
	}
	for k, v := range style.Parse(n.attrs["style"]) {
		props[k] = v
	}
	// color first: currentColor on the same element is its own color.
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		if (a == "color") != (b == "color") {
			if a == "color" {
				return -1
			}
			return 1
		}
		return strings.Compare(a, b)
	})
	for _, k := range keys {
		v := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(props[k]), "!important"))
		switch k {
		case "fill":
			st.fill = dr.paint(v, st.fill)
		case "stroke":
			st.stroke = dr.paint(v, st.stroke)
		case "stroke-width":
			st.strokeWidth = style.Px(v, st.strokeWidth)
		case "fill-opacity":
			st.fillOpacity = ratio(v, st.fillOpacity)
		case "stroke-opacity":
			st.strokeOpacity = ratio(v, st.strokeOpacity)
		case "opacity":
			st.opacity *= ratio(v, 1)
		case "fill-rule":
			st.evenOdd = v == "evenodd"
		case "font-family":
			st.fontFamily = v
		case "font-size":
			st.fontSize = style.Px(v, st.fontSize)
		case "font-weight":
			if v == "bold" {
				st.fontWeight = 700
			} else if w, err := strconv.Atoi(v); err == nil {
				st.fontWeight = w
			}
		case "stroke-dasharray":
			st.dash = style.Dash(v)
		case "stroke-linecap":
			st.cap = map[string]scene.Cap{"round": scene.RoundCap, "square": scene.SquareCap}[v]
		case "stroke-linejoin":
			st.join = map[string]scene.Join{"round": scene.RoundJoin, "bevel": scene.BevelJoin}[v]
		case "color":
			if c := style.Color(v); !c.IsNone() {
				dr.current = c
			}
		}
	}
	return st
}

// paint reads a fill or stroke: a color, currentColor, or a gradient (drawn
// flat in its stops' mean color).
func (dr *drawer) paint(v string, inherited scene.Color) scene.Color {
	switch {
	case v == "inherit":
		return inherited
	case strings.EqualFold(v, "currentColor"):
		return dr.current
	case strings.HasPrefix(v, "url("):
		id := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(v, "url("), ")"), `'" #`)
		return dr.gradientColor(dr.ids[id], 0)
	}
	return style.Color(v)
}

// gradientColor is the mean of a gradient's stops (following its href).
func (dr *drawer) gradientColor(g *node, depth int) scene.Color {
	if g == nil || depth > 5 {
		return scene.None
	}
	var r, gr, b, a, n float64
	for _, s := range g.children {
		if s.name != "stop" {
			continue
		}
		props := map[string]string{"stop-color": s.attrs["stop-color"], "stop-opacity": s.attrs["stop-opacity"]}
		for k, v := range style.Parse(s.attrs["style"]) {
			props[k] = v
		}
		c := style.Color(props["stop-color"])
		if props["stop-color"] == "" {
			c = scene.RGB(0, 0, 0)
		}
		if c.IsNone() {
			continue
		}
		r, gr, b = r+float64(c.R), gr+float64(c.G), b+float64(c.B)
		a += c.A * ratio(props["stop-opacity"], 1)
		n++
	}
	if n == 0 {
		href := g.attrs["href"]
		return dr.gradientColor(dr.ids[strings.TrimPrefix(href, "#")], depth+1)
	}
	u8 := func(v float64) uint8 { return uint8(math.Round(v / n)) }
	return scene.Color{R: u8(r), G: u8(gr), B: u8(b), A: a / n}
}

func alpha(c scene.Color, a float64) scene.Color {
	c.A *= a
	return c
}

func ratio(v string, def float64) float64 {
	v = strings.TrimSpace(v)
	scale := 1.0
	if strings.HasSuffix(v, "%") {
		v, scale = v[:len(v)-1], 0.01
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return math.Max(0, math.Min(1, f*scale))
}

var numberRe = regexp.MustCompile(`[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?`)

func numbers(s string) []float64 {
	var out []float64
	for _, m := range numberRe.FindAllString(s, -1) {
		f, _ := strconv.ParseFloat(m, 64)
		out = append(out, f)
	}
	return out
}

var transformRe = regexp.MustCompile(`(matrix|translate|scale|rotate|skewX|skewY)\s*\(([^)]*)\)`)

// transform reads an SVG transform list.
func transform(s string) (scene.Matrix, bool) {
	ms := transformRe.FindAllStringSubmatch(s, -1)
	if len(ms) == 0 {
		return scene.Identity, false
	}
	m := scene.Identity
	// The rightmost applies first.
	for i := len(ms) - 1; i >= 0; i-- {
		v := numbers(ms[i][2])
		arg := func(k int, def float64) float64 {
			if k < len(v) {
				return v[k]
			}
			return def
		}
		var t scene.Matrix
		switch ms[i][1] {
		case "matrix":
			t = scene.Matrix{A: arg(0, 1), B: arg(1, 0), C: arg(2, 0), D: arg(3, 1), E: arg(4, 0), F: arg(5, 0)}
		case "translate":
			t = scene.Translate(arg(0, 0), arg(1, 0))
		case "scale":
			t = scene.Scale(arg(0, 1), arg(1, arg(0, 1)))
		case "rotate":
			cx, cy := arg(1, 0), arg(2, 0)
			t = scene.Translate(-cx, -cy).Then(scene.Rotate(arg(0, 0))).Then(scene.Translate(cx, cy))
		case "skewX":
			t = scene.Matrix{A: 1, C: math.Tan(arg(0, 0) * math.Pi / 180), D: 1}
		case "skewY":
			t = scene.Matrix{A: 1, B: math.Tan(arg(0, 0) * math.Pi / 180), D: 1}
		}
		m = m.Then(t)
	}
	return m, true
}

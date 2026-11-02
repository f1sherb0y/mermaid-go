// Package radar is mermaid's radar chart (diagrams/radar): axes around a
// circle, one closed curve per series, a graticule of circles or polygons.
package radar

import (
	"fmt"
	"math"
	"strconv"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/lex"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

func init() { diagram.Register("radar", Render) }

// Chart is a parsed radar chart.
type Chart struct {
	Title      string
	Axes       []Axis
	Curves     []Curve
	ShowLegend bool
	Ticks      int
	Max        float64 // NaN: the largest value
	Min        float64
	Graticule  string // "circle" or "polygon"
}

// Axis is an axis; Label defaults to its name.
type Axis struct{ Name, Label string }

// Curve is a series, its values in the axes' order.
type Curve struct {
	Name, Label string
	Values      []float64
}

const maxTicks = 32

// Parse reads a radar chart (radar.langium).
func Parse(code string) (*Chart, error) {
	toks, err := lex.Lex(code)
	if err != nil {
		return nil, err
	}
	c := &Chart{ShowLegend: true, Ticks: 5, Max: math.NaN(), Graticule: "circle"}
	p := &lex.Parser{Toks: toks}
	p.SkipNewlines()
	if _, err := p.Expect(lex.ID, "radar-beta"); err != nil {
		return nil, err
	}
	p.Accept(lex.Punct, ":")
	label := func(name string) (string, error) {
		if _, ok := p.Accept(lex.Punct, "["); !ok {
			return name, nil
		}
		s, err := p.Expect(lex.String, "")
		if err != nil {
			return "", err
		}
		if _, err := p.Expect(lex.Punct, "]"); err != nil {
			return "", err
		}
		return s.Text, nil
	}
	type ref struct {
		axis  string
		value float64
	}
	var pending [][]ref // curves' entries, resolved once the axes are known
	for !p.Is(lex.EOF, "") {
		t := p.Next()
		switch {
		case t.Kind == lex.Newline, t.Kind == lex.AccTitle, t.Kind == lex.AccDescr:
		case t.Kind == lex.Title:
			c.Title = t.Text
		case t.Kind == lex.ID && t.Text == "axis":
			for {
				n, err := p.Expect(lex.ID, "")
				if err != nil {
					return nil, err
				}
				l, err := label(n.Text)
				if err != nil {
					return nil, err
				}
				c.Axes = append(c.Axes, Axis{n.Text, l})
				if _, ok := p.Accept(lex.Punct, ","); !ok {
					break
				}
			}
		case t.Kind == lex.ID && t.Text == "curve":
			for {
				n, err := p.Expect(lex.ID, "")
				if err != nil {
					return nil, err
				}
				l, err := label(n.Text)
				if err != nil {
					return nil, err
				}
				if _, err := p.Expect(lex.Punct, "{"); err != nil {
					return nil, err
				}
				var entries []ref
				for {
					p.SkipNewlines()
					e := ref{}
					if a, ok := p.Accept(lex.ID, ""); ok {
						e.axis = a.Text
						p.Accept(lex.Punct, ":")
					}
					neg := false
					if _, ok := p.Accept(lex.Punct, "-"); ok {
						neg = true
					}
					v, err := p.Expect(lex.Number, "")
					if err != nil {
						return nil, err
					}
					e.value = v.Num
					if neg {
						e.value = -e.value
					}
					entries = append(entries, e)
					p.SkipNewlines()
					if _, ok := p.Accept(lex.Punct, ","); !ok {
						break
					}
				}
				if _, err := p.Expect(lex.Punct, "}"); err != nil {
					return nil, err
				}
				c.Curves = append(c.Curves, Curve{Name: n.Text, Label: l})
				pending = append(pending, entries)
				if _, ok := p.Accept(lex.Punct, ","); !ok {
					break
				}
			}
		case t.Kind == lex.ID:
			for {
				if err := c.option(t.Text, p); err != nil {
					return nil, err
				}
				if _, ok := p.Accept(lex.Punct, ","); !ok {
					break
				}
				var err error
				if t, err = p.Expect(lex.ID, ""); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("radar: line %d: unexpected %s", t.Line, t)
		}
	}
	// Entries naming their axis follow the axes' order.
	for i, entries := range pending {
		if len(entries) == 0 || entries[0].axis == "" {
			for _, e := range entries {
				c.Curves[i].Values = append(c.Curves[i].Values, e.value)
			}
			continue
		}
		if len(c.Axes) == 0 {
			return nil, fmt.Errorf("Axes must be populated before curves for reference entries")
		}
		for _, a := range c.Axes {
			found := false
			for _, e := range entries {
				if e.axis == a.Name {
					c.Curves[i].Values = append(c.Curves[i].Values, e.value)
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("Missing entry for axis %s", a.Label)
			}
		}
	}
	if c.Ticks > maxTicks {
		c.Ticks = maxTicks
	}
	return c, nil
}

func (c *Chart) option(name string, p *lex.Parser) error {
	switch name {
	case "showLegend":
		v, err := p.Expect(lex.ID, "")
		if err != nil || (v.Text != "true" && v.Text != "false") {
			return p.Errorf("showLegend wants true or false")
		}
		c.ShowLegend = v.Text == "true"
	case "ticks", "max", "min":
		v, err := p.Expect(lex.Number, "")
		if err != nil {
			return err
		}
		switch name {
		case "ticks":
			c.Ticks = int(v.Num)
		case "max":
			c.Max = v.Num
		default:
			c.Min = v.Num
		}
	case "graticule":
		v, err := p.Expect(lex.ID, "")
		if err != nil || (v.Text != "circle" && v.Text != "polygon") {
			return p.Errorf("graticule wants circle or polygon")
		}
		c.Graticule = v.Text
	default:
		return p.Errorf("unknown statement %q", name)
	}
	return nil
}

// Render draws a radar chart.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	c, err := Parse(code)
	if err != nil {
		return nil, err
	}
	if c.Title == "" {
		c.Title = ctx.Title
	}
	cfg, th := ctx.Config, ctx.Theme
	num := func(k string) float64 { return cfg.Num("radar."+k, 0) }
	w, h := num("width"), num("height")
	mt, mr, mb, ml := num("marginTop"), num("marginRight"), num("marginBottom"), num("marginLeft")
	radius := math.Min(w, h) / 2
	opt := func(k string) string { return th.Get("radar." + k) }
	optNum := func(k string, def float64) float64 {
		if v, err := strconv.ParseFloat(opt(k), 64); err == nil {
			return v
		}
		return def
	}
	family := th.Get("fontFamily")
	textColor := style.Color(th.Get("textColor"))
	maxV := c.Max
	if math.IsNaN(maxV) {
		maxV = math.Inf(-1)
		for _, cv := range c.Curves {
			for _, v := range cv.Values {
				maxV = math.Max(maxV, v)
			}
		}
	}
	g := &scene.Group{Transform: scene.Translate(ml+w/2, mt+h/2)}
	n := len(c.Axes)
	angle := func(i int) float64 { return 2*math.Pi*float64(i)/float64(n) - math.Pi/2 }
	gratColor := style.Color(opt("graticuleColor"))
	gratFill := gratColor
	gratFill.A *= optNum("graticuleOpacity", 0.3)
	for i := range c.Ticks {
		r := radius * float64(i+1) / float64(c.Ticks)
		var segs []scene.Seg
		if c.Graticule == "circle" {
			segs = scene.Ellipse(0, 0, r, r)
		} else if n > 0 {
			pts := make([]scene.Pt, n)
			for j := range pts {
				pts[j] = scene.Pt{X: r * math.Cos(angle(j)), Y: r * math.Sin(angle(j))}
			}
			segs = scene.Polygon(pts...)
		}
		g.Items = append(g.Items, &scene.Path{Segs: segs, Fill: gratFill, Stroke: gratColor, StrokeWidth: optNum("graticuleStrokeWidth", 1)})
	}
	axisColor := style.Color(opt("axisColor"))
	labelSize := optNum("axisLabelFontSize", 12)
	for i, a := range c.Axes {
		cos, sin := math.Cos(angle(i)), math.Sin(angle(i))
		var b scene.PathBuilder
		b.MoveTo(0, 0).LineTo(radius*num("axisScaleFactor")*cos, radius*num("axisScaleFactor")*sin)
		g.Items = append(g.Items, &scene.Path{Segs: b.Segs, Stroke: axisColor, StrokeWidth: optNum("axisStrokeWidth", 2)})
		anchor := scene.Middle
		if cos > 0.01 {
			anchor = scene.Start
		} else if cos < -0.01 {
			anchor = scene.End
		}
		base := scene.Central
		if sin > 0.01 {
			base = scene.Hanging
		} else if sin < -0.01 {
			base = scene.Alphabetic
		}
		const pad = 4
		g.Items = append(g.Items, &scene.Text{
			X: radius*num("axisLabelFactor")*cos + pad*cos, Y: radius*num("axisLabelFactor")*sin + pad*sin,
			Anchor: anchor, Baseline: base, Family: family, Size: labelSize, Color: textColor,
			Runs: []scene.Span{{Text: a.Label}},
		})
	}
	opacity := optNum("curveOpacity", 0.5)
	tension := num("curveTension")
	for i, cv := range c.Curves {
		if len(cv.Values) != n {
			continue
		}
		col := style.Color(th.Get(fmt.Sprintf("cScale%d", i%12)))
		fill := col
		fill.A *= opacity
		pts := make([]scene.Pt, n)
		for j, v := range cv.Values {
			r := radius * (math.Min(math.Max(v, c.Min), maxV) - c.Min) / (maxV - c.Min)
			pts[j] = scene.Pt{X: r * math.Cos(angle(j)), Y: r * math.Sin(angle(j))}
		}
		segs := scene.Polygon(pts...)
		if c.Graticule == "circle" {
			segs = roundCurve(pts, tension)
		}
		g.Items = append(g.Items, &scene.Path{Segs: segs, Fill: fill, Stroke: col, StrokeWidth: optNum("curveStrokeWidth", 2)})
	}
	if c.ShowLegend {
		lx := (w/2 + mr) * 3 / 4
		ly := -(h/2 + mt) * 3 / 4
		for i, cv := range c.Curves {
			col := style.Color(th.Get(fmt.Sprintf("cScale%d", i%12)))
			fill := col
			fill.A *= opacity
			g.Items = append(g.Items, &scene.Group{Transform: scene.Translate(lx, ly+float64(i)*20), Items: []scene.Item{
				&scene.Path{Segs: scene.RectPath(0, 0, 12, 12, 0, 0), Fill: fill, Stroke: col, StrokeWidth: 1},
				&scene.Text{X: 16, Y: 0, Baseline: scene.Hanging, Family: family, Size: optNum("legendFontSize", 12),
					Color: textColor, Runs: []scene.Span{{Text: cv.Label}}},
			}})
		}
	}
	g.Items = append(g.Items, &scene.Text{X: 0, Y: -h/2 - mt, Anchor: scene.Middle, Baseline: scene.Hanging,
		Family: family, Size: style.Px(th.Get("fontSize"), 16), Color: textColor, Runs: []scene.Span{{Text: c.Title}}})
	return &scene.Scene{ViewBox: scene.Rect{W: w + ml + mr, H: h + mt + mb}, Items: []scene.Item{g}}, nil
}

// roundCurve is mermaid's closed Catmull-Rom-like curve through points.
func roundCurve(pts []scene.Pt, tension float64) []scene.Seg {
	n := len(pts)
	var b scene.PathBuilder
	b.MoveTo(pts[0].X, pts[0].Y)
	for i := range n {
		p0, p1, p2, p3 := pts[(i-1+n)%n], pts[i], pts[(i+1)%n], pts[(i+2)%n]
		b.CubeTo(p1.X+(p2.X-p0.X)*tension, p1.Y+(p2.Y-p0.Y)*tension,
			p2.X-(p3.X-p1.X)*tension, p2.Y-(p3.Y-p1.Y)*tension, p2.X, p2.Y)
	}
	return b.Close().Segs
}

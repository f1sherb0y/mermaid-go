// Package pie is mermaid's pie chart (diagrams/pie): slices from the
// largest share clockwise from 12 o'clock as d3.pie draws them unsorted,
// percentages inside, a legend on the right.
package pie

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

func init() { diagram.Register("pie", Render) }

// Chart is a parsed pie chart.
type Chart struct {
	Title    string
	ShowData bool
	Sections []Section
}

// Section is a slice; a label given twice keeps its first value.
type Section struct {
	Label string
	Value float64
}

// Parse reads `pie [showData]`, then title, accTitle/accDescr and
// `"label" : value` lines (pie.langium).
func Parse(code string) (*Chart, error) {
	c := &Chart{}
	lines := strings.Split(code, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i == len(lines) {
		return nil, fmt.Errorf("pie: empty")
	}
	head := strings.Fields(lines[i])
	if len(head) == 0 || head[0] != "pie" {
		return nil, fmt.Errorf("pie: line %d: expected pie", i+1)
	}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), "pie"))
	if strings.HasPrefix(rest, "showData") {
		c.ShowData = true
		rest = strings.TrimSpace(strings.TrimPrefix(rest, "showData"))
	}
	// The header line may go on with a title.
	lines[i] = rest
	seen := map[string]bool{}
	for n := i; n < len(lines); n++ {
		l := strings.TrimSpace(lines[n])
		switch {
		case l == "":
		case l == "title" || strings.HasPrefix(l, "title ") || strings.HasPrefix(l, "title\t"):
			c.Title = strings.TrimSpace(l[len("title"):])
		case strings.HasPrefix(l, "accTitle") || strings.HasPrefix(l, "accDescr"):
			// accDescr { ... } runs to its closing brace.
			if strings.HasPrefix(l, "accDescr") && strings.Contains(l, "{") && !strings.Contains(l, "}") {
				for n+1 < len(lines) && !strings.Contains(lines[n], "}") {
					n++
				}
			}
		case strings.HasPrefix(l, `"`):
			end := strings.Index(l[1:], `"`)
			if end < 0 {
				return nil, fmt.Errorf("pie: line %d: unterminated string", n+1)
			}
			label := l[1 : end+1]
			after := strings.TrimSpace(l[end+2:])
			if !strings.HasPrefix(after, ":") {
				return nil, fmt.Errorf("pie: line %d: expected ':'", n+1)
			}
			num := strings.TrimSpace(after[1:])
			v, err := strconv.ParseFloat(num, 64)
			if err != nil || strings.HasPrefix(num, "+") || strings.HasPrefix(num, ".") || strings.HasSuffix(num, ".") {
				return nil, fmt.Errorf("pie: line %d: bad number %q", n+1, num)
			}
			if v < 0 {
				return nil, fmt.Errorf("%q has invalid value: %v. Negative values are not allowed in pie charts. All slice values must be >= 0.", label, num)
			}
			if !seen[label] {
				seen[label] = true
				c.Sections = append(c.Sections, Section{label, v})
			}
		default:
			return nil, fmt.Errorf("pie: line %d: unexpected %q", n+1, l)
		}
	}
	return c, nil
}

const (
	height     = 450.0
	margin     = 40.0
	legendRect = 18.0
	legendGap  = 4.0
)

// Render draws a pie chart.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	c, err := Parse(code)
	if err != nil {
		return nil, err
	}
	if c.Title == "" {
		c.Title = ctx.Title
	}
	th, cfg := ctx.Theme, ctx.Config
	family := th.Get("fontFamily")
	outerStroke := style.Px(th.Get("pieOuterStrokeWidth"), 2)
	textPos := cfg.Num("pie.textPosition", 0.75)
	hole := cfg.Num("pie.donutHole", 0)
	if hole <= 0 || hole > 0.9 {
		hole = 0
	}
	legendPos := cfg.Str("pie.legendPosition", "right")
	radius := height/2 - margin

	sum := 0.0
	for _, s := range c.Sections {
		sum += s.Value
	}
	colors := make([]scene.Color, 12)
	for i := range colors {
		colors[i] = style.Color(th.Get(fmt.Sprintf("pie%d", i+1)))
	}
	// The ordinal scale gives colors in the sections' order, cycling.
	colorOf := map[string]scene.Color{}
	for i, s := range c.Sections {
		colorOf[s.Label] = colors[i%12]
	}

	pie := &scene.Group{}
	pie.Items = append(pie.Items, &scene.Path{
		Segs:   scene.Ellipse(0, 0, radius+outerStroke/2, radius+outerStroke/2),
		Stroke: style.Color(th.Get("pieOuterStrokeColor")), StrokeWidth: outerStroke,
	})
	// Slices under 1% are left out of the pie (but stay in the legend).
	var shown []Section
	for _, s := range c.Sections {
		if s.Value/sum*100 >= 1 {
			shown = append(shown, s)
		}
	}
	shownSum := 0.0
	for _, s := range shown {
		shownSum += s.Value
	}
	sliceStroke := style.Color(th.Get("pieStrokeColor"))
	sliceStrokeW := style.Px(th.Get("pieStrokeWidth"), 2)
	opacity, _ := strconv.ParseFloat(th.Get("pieOpacity"), 64)
	highlight := cfg.Str("pie.highlightSlice", "")
	sliceText := text.Style{Family: family, Size: style.Px(th.Get("pieSectionTextSize"), 17)}
	var labels []scene.Item
	a := 0.0
	for _, s := range shown {
		a1 := a + s.Value/shownSum*2*math.Pi
		if pct := s.Value / sum * 100; math.Round(pct) != 0 || pct >= 0.5 {
			g := &scene.Group{Opacity: opacity, Items: []scene.Item{
				&scene.Path{Segs: arc(hole*radius, radius, a, a1), Fill: colorOf[s.Label], Stroke: sliceStroke, StrokeWidth: sliceStrokeW},
			}}
			if highlight == s.Label {
				g.Opacity = 1
				g.Transform = scene.Scale(1.05, 1.05)
			}
			pie.Items = append(pie.Items, g)
			mid := (a + a1) / 2
			r := radius * textPos
			labels = append(labels, &scene.Text{
				X: r * math.Sin(mid), Y: -r * math.Cos(mid), Anchor: scene.Middle,
				Family: family, Size: sliceText.Size, Color: style.Color(th.Get("pieSectionTextColor")),
				Runs: []scene.Span{{Text: strconv.FormatFloat(s.Value/sum*100, 'f', 0, 64) + "%"}},
			})
		}
		a = a1
	}
	pie.Items = append(pie.Items, labels...)

	root := &scene.Group{Transform: scene.Translate(height/2, height/2)}
	root.Items = append(root.Items, pie)
	titleStyle := text.Style{Family: family, Size: style.Px(th.Get("pieTitleTextSize"), 25)}
	root.Items = append(root.Items, &scene.Text{
		X: 0, Y: -(height - 50) / 2, Anchor: scene.Middle, Family: family, Size: titleStyle.Size,
		Color: style.Color(th.Get("pieTitleTextColor")), Runs: []scene.Span{{Text: c.Title}},
	})
	legendText := text.Style{Family: family, Size: style.Px(th.Get("pieLegendTextSize"), 17)}
	longest := 0.0
	names := make([]string, len(c.Sections))
	for i, s := range c.Sections {
		names[i] = s.Label
		if c.ShowData {
			names[i] = fmt.Sprintf("%s [%s]", s.Label, strconv.FormatFloat(s.Value, 'f', -1, 64))
		}
		longest = math.Max(longest, ctx.Fonts.Width(names[i], legendText))
	}
	w, h := height+margin, height
	rowH := legendRect + legendGap
	total := float64(len(c.Sections)) * rowH
	half := total / 2
	var place func(i int) (float64, float64)
	switch legendPos {
	case "center":
		place = func(i int) (float64, float64) { return -longest/2 - rowH, float64(i)*rowH - half }
	case "top":
		h += total
		place = func(i int) (float64, float64) { return -longest/2 - rowH, float64(i)*rowH - radius }
		pie.Transform = scene.Translate(0, total+rowH)
	case "bottom":
		h += total
		place = func(i int) (float64, float64) { return -longest/2 - rowH, float64(i)*rowH + radius + rowH }
	case "left":
		w += rowH + longest
		place = func(i int) (float64, float64) { return -radius - rowH, float64(i)*rowH - half }
		pie.Transform = scene.Translate(longest+rowH, 0)
	default:
		w += rowH + longest
		place = func(i int) (float64, float64) { return 12 * legendRect, float64(i)*rowH - half }
	}
	for i, s := range c.Sections {
		x, y := place(i)
		col := colorOf[s.Label]
		root.Items = append(root.Items, &scene.Group{Transform: scene.Translate(x, y), Items: []scene.Item{
			&scene.Path{Segs: scene.RectPath(0, 0, legendRect, legendRect, 0, 0), Fill: col, Stroke: col, StrokeWidth: 1},
			&scene.Text{X: legendRect + legendGap, Y: legendRect - legendGap, Family: family, Size: legendText.Size,
				Color: style.Color(th.Get("pieLegendTextColor")), Runs: []scene.Span{{Text: names[i]}}},
		}})
	}
	titleW := ctx.Fonts.Width(c.Title, titleStyle)
	left := math.Min(0, height/2-titleW/2)
	right := math.Max(w, height/2+titleW/2)
	return &scene.Scene{ViewBox: scene.Rect{X: left, Y: 0, W: right - left, H: h}, Items: []scene.Item{root}}, nil
}

// arc is a d3.arc slice (or ring segment) from angle a0 to a1, clockwise
// from 12 o'clock.
func arc(inner, outer, a0, a1 float64) []scene.Seg {
	pt := func(r, a float64) (float64, float64) { return r * math.Sin(a), -r * math.Cos(a) }
	var b scene.PathBuilder
	large := a1-a0 > math.Pi
	if a1-a0 >= 2*math.Pi-1e-9 {
		// A whole circle: two halves.
		x, y := pt(outer, a0)
		b.MoveTo(x, y)
		mx, my := pt(outer, a0+math.Pi)
		b.ArcTo(outer, outer, 0, false, true, mx, my).ArcTo(outer, outer, 0, false, true, x, y)
		if inner > 0 {
			ix, iy := pt(inner, a0)
			b.MoveTo(ix, iy)
			mx, my = pt(inner, a0+math.Pi)
			b.ArcTo(inner, inner, 0, false, false, mx, my).ArcTo(inner, inner, 0, false, false, ix, iy)
		}
		return b.Close().Segs
	}
	x0, y0 := pt(outer, a0)
	x1, y1 := pt(outer, a1)
	b.MoveTo(x0, y0).ArcTo(outer, outer, 0, large, true, x1, y1)
	if inner > 0 {
		ix, iy := pt(inner, a1)
		b.LineTo(ix, iy)
		jx, jy := pt(inner, a0)
		b.ArcTo(inner, inner, 0, large, false, jx, jy)
	} else {
		b.LineTo(0, 0)
	}
	return b.Close().Segs
}

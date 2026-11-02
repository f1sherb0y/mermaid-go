// Package quadrant is mermaid's quadrant chart (diagrams/quadrant-chart):
// four quadrants, axis labels, and points placed on [0, 1] × [0, 1].
package quadrant

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

func init() { diagram.Register("quadrantChart", Render) }

// Chart is a parsed quadrant chart.
type Chart struct {
	Title         string
	Quadrants     [4]string
	XLeft, XRight string
	YBottom, YTop string
	Points        []Point
	classes       map[string]Style
}

// Style is a point's own look; zero values are unset.
type Style struct {
	Radius      float64
	Color       string
	StrokeColor string
	StrokeWidth string
}

// Point is a point, in [0, 1].
type Point struct {
	Text  string
	X, Y  float64
	Class string
	Style
}

var (
	pointRe = regexp.MustCompile(`^(.*?)(?::::(\w+))?\s*:\s*\[\s*(1|0(?:\.\d+)?)\s*,\s*(1|0(?:\.\d+)?)\s*\]\s*(.*)$`)
	hexRe   = regexp.MustCompile(`^#?([\dA-Fa-f]{6}|[\dA-Fa-f]{3})$`)
	pxRe    = regexp.MustCompile(`^\d+px$`)
	numRe   = regexp.MustCompile(`^\d+$`)
)

// Parse reads a quadrant chart (quadrant.jison: one statement per line).
func Parse(code string) (*Chart, error) {
	c := &Chart{classes: map[string]Style{}}
	started := false
	for n, line := range strings.Split(code, "\n") {
		l := strings.TrimSpace(line)
		low := strings.ToLower(l)
		if l == "" {
			continue
		}
		if !started {
			if low != "quadrantchart" && !strings.HasPrefix(low, "quadrantchart ") {
				return nil, fmt.Errorf("quadrant: line %d: expected quadrantChart", n+1)
			}
			started = true
			continue
		}
		word, rest, _ := strings.Cut(l, " ")
		rest = strings.TrimSpace(rest)
		switch strings.ToLower(word) {
		case "title":
			c.Title = rest
			continue
		case "acctitle", "accdescr", "acctitle:", "accdescr:":
			continue
		case "x-axis", "y-axis":
			a, b, delim := axis(rest)
			if delim && b == "" {
				a += " ⟶ "
			}
			if strings.ToLower(word) == "x-axis" {
				c.XLeft, c.XRight = a, b
			} else {
				c.YBottom, c.YTop = a, b
			}
			continue
		case "quadrant-1", "quadrant-2", "quadrant-3", "quadrant-4":
			c.Quadrants[word[len(word)-1]-'1'] = text(rest)
			continue
		case "classdef":
			name, styles, _ := strings.Cut(rest, " ")
			st, err := parseStyles(styles)
			if err != nil {
				return nil, err
			}
			c.classes[name] = st
			continue
		}
		m := pointRe.FindStringSubmatch(l)
		if m == nil {
			return nil, fmt.Errorf("quadrant: line %d: unexpected %q", n+1, l)
		}
		x, _ := strconv.ParseFloat(m[3], 64)
		y, _ := strconv.ParseFloat(m[4], 64)
		st, err := parseStyles(m[5])
		if err != nil {
			return nil, err
		}
		// Points are kept newest first, as mermaid's builder prepends them.
		c.Points = append([]Point{{Text: text(m[1]), X: x, Y: y, Class: m[2], Style: st}}, c.Points...)
	}
	if !started {
		return nil, fmt.Errorf("quadrant: empty")
	}
	return c, nil
}

// axis splits `left --> right`.
func axis(s string) (left, right string, delim bool) {
	if i := strings.Index(s, "--"); i >= 0 {
		j := i
		for j < len(s) && s[j] == '-' {
			j++
		}
		if j < len(s) && s[j] == '>' {
			return text(s[:i]), text(s[j+1:]), true
		}
	}
	return text(s), "", false
}

// text is a label: quoted ("..." or "`...`") or bare.
func text(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 4 && strings.HasPrefix(s, "\"`") && strings.HasSuffix(s, "`\"") {
		return s[2 : len(s)-2]
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func parseStyles(s string) (Style, error) {
	var st Style
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, ":")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "radius":
			if !numRe.MatchString(v) {
				return st, fmt.Errorf("value for %s %s is invalid, please use a valid number", k, v)
			}
			st.Radius, _ = strconv.ParseFloat(v, 64)
		case "color", "stroke-color":
			if !hexRe.MatchString(v) {
				return st, fmt.Errorf("value for %s %s is invalid, please use a valid hex code", k, v)
			}
			if k == "color" {
				st.Color = v
			} else {
				st.StrokeColor = v
			}
		case "stroke-width":
			if !pxRe.MatchString(v) {
				return st, fmt.Errorf("value for %s %s is invalid, please use a valid number of pixels (eg. 10px)", k, v)
			}
			st.StrokeWidth = v
		default:
			return st, fmt.Errorf("style named %s is not supported.", k)
		}
	}
	return st, nil
}

// Render draws a quadrant chart.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	c, err := Parse(code)
	if err != nil {
		return nil, err
	}
	if c.Title == "" {
		c.Title = ctx.Title
	}
	cfg, th := ctx.Config, ctx.Theme
	num := func(k string) float64 { return cfg.Num("quadrantChart."+k, 0) }
	w, h := num("chartWidth"), num("chartHeight")
	family := th.Get("fontFamily")
	textColor := style.Color(th.Get("textColor"))
	// A color the browser cannot use leaves the root's fill, the text color.
	paint := func(s string) scene.Color {
		if col := style.Color(s); !col.IsNone() || s == "none" {
			return col
		}
		return textColor
	}
	label := func(x, y float64, s string, size float64, fill string, centered, top bool, rot float64) scene.Item {
		t := &scene.Text{Family: family, Size: size, Color: paint(fill), Runs: []scene.Span{{Text: s}}, Anchor: scene.Start, Baseline: scene.MiddleBaseline}
		if centered {
			t.Anchor = scene.Middle
		}
		if top {
			t.Baseline = scene.Hanging
		}
		return &scene.Group{Transform: scene.Rotate(rot).Then(scene.Translate(x, y)), Items: []scene.Item{t}}
	}

	showX := c.XLeft != "" || c.XRight != ""
	showY := c.YBottom != "" || c.YTop != ""
	showTitle := c.Title != ""
	xPos := cfg.Str("quadrantChart.xAxisPosition", "top")
	if len(c.Points) > 0 {
		xPos = "bottom"
	}
	yPos := cfg.Str("quadrantChart.yAxisPosition", "left")
	xSpace := num("xAxisLabelPadding")*2 + num("xAxisLabelFontSize")
	ySpace := num("yAxisLabelPadding")*2 + num("yAxisLabelFontSize")
	var xTop, xBottom, yLeft, yRight, titleTop float64
	if showX {
		if xPos == "top" {
			xTop = xSpace
		} else {
			xBottom = xSpace
		}
	}
	if showY {
		if yPos == "left" {
			yLeft = ySpace
		} else {
			yRight = ySpace
		}
	}
	if showTitle {
		titleTop = num("titleFontSize") + num("titlePadding")*2
	}
	pad := num("quadrantPadding")
	left := pad + yLeft
	top := pad + xTop + titleTop
	qw := w - pad*2 - yLeft - yRight
	qh := h - pad*2 - xTop - xBottom - titleTop
	hw, hh := qw/2, qh/2

	var quads, borders, points, labels []scene.Item
	qx := [4]float64{left + hw, left, left, left + hw}
	qy := [4]float64{top, top, top + hh, top + hh}
	for i := range 4 {
		quads = append(quads, &scene.Path{Segs: scene.RectPath(qx[i], qy[i], hw, hh, 0, 0), Fill: paint(th.Get(fmt.Sprintf("quadrant%dFill", i+1)))})
		ty, topAligned := qy[i]+hh/2, false
		if len(c.Points) > 0 {
			ty, topAligned = qy[i]+num("quadrantTextTopPadding"), true
		}
		quads = append(quads, label(qx[i]+hw/2, ty, c.Quadrants[i], num("quadrantLabelFontSize"), th.Get(fmt.Sprintf("quadrant%dTextFill", i+1)), true, topAligned, 0))
	}
	ext, in := num("quadrantExternalBorderStrokeWidth"), num("quadrantInternalBorderStrokeWidth")
	extCol, inCol := paint(th.Get("quadrantExternalBorderStrokeFill")), paint(th.Get("quadrantInternalBorderStrokeFill"))
	half := ext / 2
	line := func(x1, y1, x2, y2, width float64, col scene.Color) {
		var b scene.PathBuilder
		b.MoveTo(x1, y1).LineTo(x2, y2)
		borders = append(borders, &scene.Path{Segs: b.Segs, Stroke: col, StrokeWidth: width})
	}
	line(left-half, top, left+qw+half, top, ext, extCol)
	line(left+qw, top+half, left+qw, top+qh-half, ext, extCol)
	line(left-half, top+qh, left+qw+half, top+qh, ext, extCol)
	line(left, top+half, left, top+qh-half, ext, extCol)
	line(left+hw, top+half, left+hw, top+qh-half, in, inCol)
	line(left+half, top+hh, left+qw-half, top+hh, in, inCol)

	for _, p := range c.Points {
		st := p.Style
		if cl, ok := c.classes[p.Class]; ok {
			if st.Radius == 0 {
				st.Radius = cl.Radius
			}
			if st.Color == "" {
				st.Color = cl.Color
			}
			if st.StrokeColor == "" {
				st.StrokeColor = cl.StrokeColor
			}
			if st.StrokeWidth == "" {
				st.StrokeWidth = cl.StrokeWidth
			}
		}
		fill := th.Get("quadrantPointFill")
		if st.Color != "" {
			fill = st.Color
		}
		stroke := th.Get("quadrantPointFill")
		if st.StrokeColor != "" {
			stroke = st.StrokeColor
		}
		r := num("pointRadius")
		if st.Radius != 0 {
			r = st.Radius
		}
		x := left + p.X*qw
		y := top + qh - p.Y*qh
		path := &scene.Path{Segs: scene.Ellipse(x, y, r, r), Fill: paint(fill)}
		if sw := style.Px(st.StrokeWidth, 0); sw > 0 {
			path.Stroke, path.StrokeWidth = style.Color(stroke), sw
		}
		points = append(points, path, label(x, y+num("pointTextPadding"), p.Text, num("pointLabelFontSize"), th.Get("quadrantPointTextFill"), true, true, 0))
	}

	middleX, middleY := c.XRight != "", c.YTop != ""
	xFill, yFill := th.Get("quadrantXAxisTextFill"), th.Get("quadrantYAxisTextFill")
	xy := num("xAxisLabelPadding") + titleTop
	if xPos != "top" {
		xy = num("xAxisLabelPadding") + top + qh + pad
	}
	shift := func(on bool, d float64) float64 {
		if on {
			return d
		}
		return 0
	}
	if showX && c.XLeft != "" {
		labels = append(labels, label(left+shift(middleX, hw/2), xy, c.XLeft, num("xAxisLabelFontSize"), xFill, middleX, true, 0))
	}
	if showX && c.XRight != "" {
		labels = append(labels, label(left+hw+shift(middleX, hw/2), xy, c.XRight, num("xAxisLabelFontSize"), xFill, middleX, true, 0))
	}
	yx := num("yAxisLabelPadding")
	if yPos != "left" {
		yx = num("yAxisLabelPadding") + left + qw + pad
	}
	if showY && c.YBottom != "" {
		labels = append(labels, label(yx, top+qh-shift(middleY, hh/2), c.YBottom, num("yAxisLabelFontSize"), yFill, middleY, true, -90))
	}
	if showY && c.YTop != "" {
		labels = append(labels, label(yx, top+hh-shift(middleY, hh/2), c.YTop, num("yAxisLabelFontSize"), yFill, middleY, true, -90))
	}

	items := append(quads, borders...)
	items = append(items, points...)
	items = append(items, labels...)
	if showTitle {
		items = append(items, label(w/2, num("titlePadding"), c.Title, num("titleFontSize"), th.Get("quadrantTitleFill"), true, true, 0))
	}
	return &scene.Scene{ViewBox: scene.Rect{W: w, H: h}, Items: items}, nil
}

// Package xychart is mermaid's xychart (diagrams/xychart): bar and line
// plots on a band or linear x axis and a linear y axis, vertical or
// horizontal, with a title and a legend.
package xychart

import (
	"math"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/d3"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

func init() { diagram.Register("xychart", Render) }

type rect struct{ x, y, w, h float64 }

// drawer measures and draws the chart's text.
type drawer struct {
	ctx    *diagram.Context
	family string
	items  []scene.Item
}

// size is the box of the widest and tallest of texts at a font size, as
// mermaid measures them (a text's height is its font's ascent + descent).
func (d *drawer) size(texts []string, fontSize float64) (w, h float64) {
	st := text.Style{Family: d.family, Size: fontSize}
	asc, desc := d.ctx.Fonts.Extents(st)
	for _, t := range texts {
		w = math.Max(w, d.ctx.Fonts.Width(t, st))
		h = math.Max(h, asc+desc)
	}
	return w, h
}

// text draws a label at (x, y): vpos "top" puts the text's top there,
// "middle" its middle; hpos "left", "center" or "right"; rotated by rot
// degrees about the point.
func (d *drawer) text(s string, x, y, size float64, col scene.Color, vpos, hpos string, rot float64) {
	t := &scene.Text{Family: d.family, Size: size, Color: col, Runs: []scene.Span{{Text: s}}, Baseline: scene.MiddleBaseline}
	switch hpos {
	case "center":
		t.Anchor = scene.Middle
	case "right":
		t.Anchor = scene.End
	}
	if vpos == "top" {
		asc, _ := d.ctx.Fonts.Extents(text.Style{Family: d.family, Size: size})
		t.Baseline, t.Y = scene.Alphabetic, asc
	}
	if rot == 0 {
		t.X, t.Y = t.X+x, t.Y+y
		d.items = append(d.items, t)
		return
	}
	d.items = append(d.items, &scene.Group{Transform: scene.Rotate(rot).Then(scene.Translate(x, y)), Items: []scene.Item{t}})
}

func (d *drawer) line(x1, y1, x2, y2 float64, col scene.Color, w float64) {
	var b scene.PathBuilder
	b.MoveTo(x1, y1).LineTo(x2, y2)
	d.items = append(d.items, &scene.Path{Segs: b.Segs, Stroke: col, StrokeWidth: w})
}

// axisConfig is xyChart.xAxis / yAxis.
type axisConfig struct {
	showLabel, showTitle, showTick, showAxisLine       bool
	labelFontSize, labelPadding, titleFontSize         float64
	titlePadding, tickLength, tickWidth, axisLineWidth float64
	labelRotation                                      float64
	titleColor, labelColor, tickColor, axisLineColor   scene.Color
}

// axis is one of the chart's axes, laid out as mermaid's BaseAxis.
type axis struct {
	cfg   axisConfig
	data  Axis
	d     *drawer
	pos   string // "left", "bottom" or "top"
	r0    float64
	r1    float64
	box   rect
	outer float64
	rot   float64 // label rotation, radians

	showTitle, showLabel, showTick, showLine bool
	titleH                                   float64
}

func (a *axis) setRange(r0, r1 float64) {
	a.r0, a.r1 = r0, r1
	if a.pos == "left" {
		a.box.h = r1 - r0
	} else {
		a.box.w = r1 - r0
	}
}

func (a *axis) setPosition(p string) { a.pos = p; a.setRange(a.r0, a.r1) }

// span is the range less the outer padding.
func (a *axis) span() (float64, float64) { return a.r0 + a.outer, a.r1 - a.outer }

// tick is a tick: its label and, on a linear axis, its value.
type tick struct {
	label string
	v     float64
}

func (a *axis) ticks() []tick {
	var out []tick
	if a.data.Band {
		for _, c := range a.data.Categories {
			out = append(out, tick{label: c})
		}
		return out
	}
	lo, hi := a.data.Min, a.data.Max
	if a.pos == "left" {
		lo, hi = hi, lo
	}
	for _, v := range d3.Ticks(lo, hi, 10) {
		out = append(out, tick{d3.Num(v), v})
	}
	return out
}

func (a *axis) labels() []string {
	var out []string
	for _, t := range a.ticks() {
		out = append(out, t.label)
	}
	return out
}

// at is a position on the axis: a category's (band) or a number's, given
// as text (linear).
func (a *axis) at(x string) float64 {
	lo, hi := a.span()
	if a.data.Band {
		// d3.scaleBand with paddingInner 1: the categories' points, ends
		// included; a lone one in the middle.
		var domain []string
		idx := -1
		for _, c := range a.data.Categories {
			if !contains(domain, c) {
				if c == x {
					idx = len(domain)
				}
				domain = append(domain, c)
			}
		}
		if idx < 0 {
			return lo
		}
		if len(domain) == 1 {
			return (lo + hi) / 2
		}
		return lo + (hi-lo)/float64(len(domain)-1)*float64(idx)
	}
	v, err := strconv.ParseFloat(x, 64)
	if err != nil {
		return math.NaN()
	}
	return a.atValue(v)
}

func (a *axis) atValue(v float64) float64 {
	lo, hi := a.span()
	s := d3.Linear{D0: a.data.Min, D1: a.data.Max, R0: lo, R1: hi}
	if a.pos == "left" {
		s.D0, s.D1 = s.D1, s.D0
	}
	return s.At(v)
}

func (a *axis) tickAt(t tick) float64 {
	if a.data.Band {
		return a.at(t.label)
	}
	return a.atValue(t.v)
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func (a *axis) tickDistance() float64 {
	lo, hi := a.span()
	return math.Abs(hi-lo) / float64(len(a.ticks()))
}

// fitBars widens the outer padding so bars fit at the ends.
func (a *axis) fitBars() {
	if d := 0.7 * a.tickDistance(); d > a.outer*2 {
		a.outer = math.Floor(d / 2)
	}
}

// space lays the axis out in what is available and returns its size.
func (a *axis) space(availW, availH float64) (float64, float64) {
	c := a.cfg
	if a.pos == "left" {
		avail := availW
		if c.showAxisLine && avail > c.axisLineWidth {
			avail -= c.axisLineWidth
			a.showLine = true
		}
		if c.showLabel {
			w, h := a.d.size(a.labels(), c.labelFontSize)
			a.outer = math.Min(h/2, 0.2*availH)
			if need := w + c.labelPadding*2; need <= avail {
				avail -= need
				a.showLabel = true
			}
		}
		if c.showTick && avail >= c.tickLength {
			a.showTick = true
			avail -= c.tickLength
		}
		if c.showTitle && a.data.Title != "" {
			_, h := a.d.size([]string{a.data.Title}, c.titleFontSize)
			a.titleH = h
			if need := h + c.titlePadding*2; need <= avail {
				avail -= need
				a.showTitle = true
			}
		}
		a.box.w, a.box.h = availW-avail, availH
		return a.box.w, a.box.h
	}
	avail := availH
	if c.showAxisLine && avail > c.axisLineWidth {
		avail -= c.axisLineWidth
		a.showLine = true
	}
	if c.showLabel {
		w, h := a.d.size(a.labels(), c.labelFontSize)
		a.outer = math.Min(w/2, 0.2*availW)
		need := h
		if a.pos == "bottom" && a.rot != 0 {
			need = math.Max(need, math.Abs(math.Sin(a.rot)*w)+math.Abs(math.Cos(a.rot)*h))
		}
		need += c.labelPadding * 2
		if need <= avail {
			avail -= need
			a.showLabel = true
		}
	}
	if c.showTick && avail >= c.tickLength {
		a.showTick = true
		avail -= c.tickLength
	}
	if c.showTitle && a.data.Title != "" {
		_, h := a.d.size([]string{a.data.Title}, c.titleFontSize)
		a.titleH = h
		if need := h + c.titlePadding*2; need <= avail {
			avail -= need
			a.showTitle = true
		}
	}
	a.box.w, a.box.h = availW, availH-avail
	return a.box.w, a.box.h
}

// rotationOffset is half the labels' size along a dimension, turned by the
// label rotation.
func (a *axis) rotationOffset(height bool) float64 {
	if a.rot == 0 {
		return 0
	}
	w, h := a.d.size(a.labels(), a.cfg.labelFontSize)
	if height {
		return math.Sin(a.rot) * h / 2
	}
	return math.Sin(a.rot) * w / 2
}

func (a *axis) draw() {
	c, b, d := a.cfg, a.box, a.d
	on := func(show bool, v float64) float64 {
		if show {
			return v
		}
		return 0
	}
	switch a.pos {
	case "left":
		if a.showLine {
			x := b.x + b.w - c.axisLineWidth/2
			d.line(x, b.y, x, b.y+b.h, c.axisLineColor, c.axisLineWidth)
		}
		if a.showLabel {
			x := b.x + b.w - c.labelPadding - on(a.showTick, c.tickLength) - on(a.showLine, c.axisLineWidth)
			for _, t := range a.ticks() {
				d.text(t.label, x, a.tickAt(t), c.labelFontSize, c.labelColor, "middle", "right", 0)
			}
		}
		if a.showTick {
			x := b.x + b.w - on(a.showLine, c.axisLineWidth)
			for _, t := range a.ticks() {
				y := a.tickAt(t)
				d.line(x, y, x-c.tickLength, y, c.tickColor, c.tickWidth)
			}
		}
		if a.showTitle {
			d.text(a.data.Title, b.x+c.titlePadding, b.y+b.h/2, c.titleFontSize, c.titleColor, "top", "center", 270)
		}
	case "bottom":
		if a.showLine {
			y := b.y + c.axisLineWidth/2
			d.line(b.x, y, b.x+b.w, y, c.axisLineColor, c.axisLineWidth)
		}
		if a.showLabel {
			y := b.y + c.labelPadding + on(a.showTick, c.tickLength) + on(a.showLine, c.axisLineWidth) + math.Abs(a.rotationOffset(false))
			for _, t := range a.ticks() {
				d.text(t.label, a.tickAt(t)+a.rotationOffset(true), y, c.labelFontSize, c.labelColor, "top", "center", a.rot*180/math.Pi)
			}
		}
		if a.showTick {
			y := b.y + on(a.showLine, c.axisLineWidth)
			for _, t := range a.ticks() {
				x := a.tickAt(t)
				d.line(x, y, x, y+c.tickLength, c.tickColor, c.tickWidth)
			}
		}
		if a.showTitle {
			d.text(a.data.Title, a.r0+(a.r1-a.r0)/2, b.y+b.h-c.titlePadding-a.titleH, c.titleFontSize, c.titleColor, "top", "center", 0)
		}
	case "top":
		if a.showLine {
			y := b.y + b.h - c.axisLineWidth/2
			d.line(b.x, y, b.x+b.w, y, c.axisLineColor, c.axisLineWidth)
		}
		if a.showLabel {
			y := b.y + on(a.showTitle, a.titleH+c.titlePadding*2) + c.labelPadding
			for _, t := range a.ticks() {
				d.text(t.label, a.tickAt(t), y, c.labelFontSize, c.labelColor, "top", "center", 0)
			}
		}
		if a.showTick {
			y := b.y + b.h - on(a.showLine, c.axisLineWidth)
			for _, t := range a.ticks() {
				x := a.tickAt(t)
				d.line(x, y, x, y-c.tickLength, c.tickColor, c.tickWidth)
			}
		}
		if a.showTitle {
			d.text(a.data.Title, b.x+b.w/2, b.y+c.titlePadding, c.titleFontSize, c.titleColor, "top", "center", 0)
		}
	}
}

// Render draws an xychart.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	c, err := Parse(code)
	if err != nil {
		return nil, err
	}
	if c.Title == "" {
		c.Title = ctx.Title
	}
	cfg, th := ctx.Config, ctx.Theme
	num := func(k string, def float64) float64 { return cfg.Num("xyChart."+k, def) }
	flag := func(k string, def bool) bool { return cfg.Bool("xyChart."+k, def) }
	color := func(k string) scene.Color { return style.Color(th.Get("xyChart." + k)) }
	W, H := num("width", 700), num("height", 500)
	horizontal := c.Horizontal || cfg.Str("xyChart.chartOrientation", "vertical") == "horizontal"
	d := &drawer{ctx: ctx, family: th.Get("fontFamily")}

	axisCfg := func(name, prefix string) axisConfig {
		n := func(k string, def float64) float64 { return num(name+"."+k, def) }
		f := func(k string) bool { return flag(name+"."+k, true) }
		return axisConfig{
			showLabel: f("showLabel"), showTitle: f("showTitle"), showTick: f("showTick"), showAxisLine: f("showAxisLine"),
			labelFontSize: n("labelFontSize", 14), labelPadding: n("labelPadding", 5), titleFontSize: n("titleFontSize", 16),
			titlePadding: n("titlePadding", 5), tickLength: n("tickLength", 5), tickWidth: n("tickWidth", 2),
			axisLineWidth: n("axisLineWidth", 2), labelRotation: n("labelRotation", 0),
			titleColor: color(prefix + "TitleColor"), labelColor: color(prefix + "LabelColor"),
			tickColor: color(prefix + "TickColor"), axisLineColor: color(prefix + "LineColor"),
		}
	}
	newAxis := func(data Axis, ac axisConfig) *axis {
		a := &axis{cfg: ac, data: data, d: d, pos: "left", r1: 10}
		if ac.labelRotation >= -90 && ac.labelRotation <= 90 {
			a.rot = ac.labelRotation * math.Pi / 180
		}
		return a
	}
	xAxis := newAxis(c.X, axisCfg("xAxis", "xAxis"))
	yAxis := newAxis(c.Y, axisCfg("yAxis", "yAxis"))

	// Title and legend sizes.
	titleSize := num("titleFontSize", 20)
	titlePad := num("titlePadding", 10)
	titleW, titleH := d.size([]string{c.Title}, titleSize)
	showTitle := flag("showTitle", true) && c.Title != ""
	var legendPlots []int // the plots with titles
	if flag("showLegend", true) {
		for i, p := range c.Plots {
			if p.Title != "" {
				legendPlots = append(legendPlots, i)
			}
		}
	}
	legendFont := num("legendFontSize", 14)
	legendPad := num("legendPadding", 10)
	marker, markerGap, itemGap := legendFont*0.75, legendFont*0.35, legendFont*0.5
	legendSpace := func(availW, availH float64) (float64, float64) {
		if len(legendPlots) == 0 {
			return 0, 0
		}
		var titles []string
		for _, i := range legendPlots {
			titles = append(titles, c.Plots[i].Title)
		}
		tw, _ := d.size(titles, legendFont)
		n := float64(len(legendPlots))
		w := legendPad*2 + marker + markerGap + tw
		h := legendPad*2 + n*legendFont + (n-1)*itemGap
		if w <= availW && h <= availH {
			return w, h
		}
		legendPlots = nil
		return 0, 0
	}

	// The orchestrator: the plot takes its share first, then the title, the
	// axes and the legend; the plot gets what is left.
	pct := num("plotReservedSpacePercent", 50)
	chartW, chartH := math.Floor(W*pct/100), math.Floor(H*pct/100)
	availW, availH := W-chartW, H-chartH
	titleBoxH := 0.0
	if showTitle && titleH+2*titlePad <= availH {
		titleBoxH = titleH + 2*titlePad
	} else {
		showTitle = false
	}
	availH -= titleBoxH
	var plotX, plotY, legendW, legendH float64
	if !horizontal {
		plotY = titleBoxH
		xAxis.setPosition("bottom")
		_, h := xAxis.space(availW, availH)
		availH -= h
		yAxis.setPosition("left")
		w, _ := yAxis.space(availW, availH)
		plotX = w
		availW -= w
	} else {
		xAxis.setPosition("left")
		w, _ := xAxis.space(availW, availH)
		availW -= w
		plotX = w
		yAxis.setPosition("top")
		_, h := yAxis.space(availW, availH)
		availH -= h
		plotY = titleBoxH + h
	}
	legendW, legendH = legendSpace(availW, chartH)
	availW -= legendW
	if availW > 0 {
		chartW += availW
	}
	if availH > 0 {
		chartH += availH
	}
	plot := rect{plotX, plotY, chartW, chartH}
	legendX, legendY := plotX+chartW, plotY+math.Max((chartH-legendH)/2, 0)
	if !horizontal {
		xAxis.setRange(plotX, plotX+chartW)
		xAxis.box.x, xAxis.box.y = plotX, plotY+chartH
		yAxis.setRange(plotY, plotY+chartH)
		yAxis.box.x, yAxis.box.y = 0, plotY
	} else {
		yAxis.setRange(plotX, plotX+chartW)
		yAxis.box.x, yAxis.box.y = plotX, titleBoxH
		xAxis.setRange(plotY, plotY+chartH)
		xAxis.box.x, xAxis.box.y = 0, plotY
	}
	for _, p := range c.Plots {
		if p.Bar {
			xAxis.fitBars()
			break
		}
	}

	// Draw: background, title, plots, legend, axes.
	d.items = append(d.items, &scene.Path{Segs: scene.RectPath(0, 0, W, H, 0, 0), Fill: color("backgroundColor")})
	if showTitle {
		half := math.Min(titleW, W) / 2
		center := math.Min(math.Max(plotX+chartW/2, half), W-half)
		d.text(c.Title, center, titleBoxH/2, titleSize, color("titleColor"), "middle", "center", 0)
	}
	palette := strings.Split(th.Get("xyChart.plotColorPalette"), ",")
	plotColor := func(i int) scene.Color { return style.Color(strings.TrimSpace(palette[i%len(palette)])) }
	var firstValues []float64
	if len(c.Plots) > 0 {
		for _, pt := range c.Plots[0].Points {
			firstValues = append(firstValues, pt.Y)
		}
	}
	for i, p := range c.Plots {
		col := plotColor(i)
		type xy struct{ x, y float64 }
		var pts []xy
		for _, pt := range p.Points {
			pts = append(pts, xy{xAxis.at(pt.X), yAxis.atValue(pt.Y)})
		}
		if p.Bar {
			bw := math.Min(xAxis.outer*2, xAxis.tickDistance()) * 0.95
			var bars []rect
			for _, q := range pts {
				r := rect{q.x - bw/2, q.y, bw, plot.y + plot.h - q.y}
				if horizontal {
					r = rect{plot.x, q.x - bw/2, q.y - plot.x, bw}
				}
				bars = append(bars, r)
				if !math.IsNaN(r.x + r.y + r.w + r.h) {
					d.items = append(d.items, &scene.Path{Segs: scene.RectPath(r.x, r.y, r.w, r.h, 0, 0), Fill: col})
				}
			}
			if flag("showDataLabel", false) {
				dataLabels(d, bars, firstValues, horizontal, flag("showDataLabelOutsideBar", false), color("dataLabelColor"))
			}
			continue
		}
		var b scene.PathBuilder
		started := false
		for _, q := range pts {
			x, y := q.x, q.y
			if horizontal {
				x, y = q.y, q.x
			}
			if math.IsNaN(x + y) {
				continue
			}
			if !started {
				b.MoveTo(x, y)
				started = true
			} else {
				b.LineTo(x, y)
			}
		}
		if started {
			d.items = append(d.items, &scene.Path{Segs: b.Segs, Stroke: col, StrokeWidth: 2})
		}
		for k, q := range pts {
			if k >= len(p.Labels) || p.Labels[k] == "" || math.IsNaN(q.x+q.y) {
				continue
			}
			if horizontal {
				d.text(p.Labels[k], q.y+10, q.x, 12, col, "middle", "left", 0)
			} else {
				d.text(p.Labels[k], q.x, q.y-10, 12, col, "middle", "center", 0)
			}
		}
	}
	if len(legendPlots) > 0 {
		x0, y0 := legendX+legendPad, legendY+legendPad
		// Bar markers, then line markers, then the names, as mermaid groups them.
		var lines []scene.Item
		for k, i := range legendPlots {
			y := y0 + float64(k)*(legendFont+itemGap)
			if c.Plots[i].Bar {
				d.items = append(d.items, &scene.Path{Segs: scene.RectPath(x0, y, marker, marker, 0, 0), Fill: plotColor(i)})
				continue
			}
			var b scene.PathBuilder
			b.MoveTo(x0, y+marker/2).LineTo(x0+marker, y+marker/2)
			lines = append(lines, &scene.Path{Segs: b.Segs, Stroke: plotColor(i), StrokeWidth: 2})
		}
		d.items = append(d.items, lines...)
		for k, i := range legendPlots {
			d.text(c.Plots[i].Title, x0+marker+markerGap, y0+float64(k)*(legendFont+itemGap)+marker/2, legendFont, color("legendTextColor"), "middle", "left", 0)
		}
	}
	xAxis.draw()
	yAxis.draw()
	return &scene.Scene{ViewBox: scene.Rect{W: W, H: H}, Items: d.items}, nil
}

// dataLabels writes the values on (or beyond) the bars, in one font size
// that fits them all — mermaid's estimate of a text's width is 0.7 em per
// character. The values are the first plot's, as mermaid's.
func dataLabels(d *drawer, bars []rect, values []float64, horizontal, outside bool, col scene.Color) {
	type item struct {
		r     rect
		label string
	}
	var items []item
	for i, r := range bars {
		if r.w > 0 && r.h > 0 && i < len(values) {
			items = append(items, item{r, d3.Num(values[i])})
		}
	}
	if len(items) == 0 {
		return
	}
	size := math.Inf(1)
	for _, it := range items {
		n := float64(len([]rune(it.label)))
		var fs float64
		if horizontal {
			for fs = it.r.h * 0.7; fs > 0 && fs*n*0.7 > it.r.w-10; fs-- {
			}
		} else {
			fits := func(fs float64) bool {
				w := fs * n * 0.7
				cx := it.r.x + it.r.w/2
				return cx-w/2 >= it.r.x && cx+w/2 <= it.r.x+it.r.w && it.r.y+10+fs <= it.r.y+it.r.h
			}
			for fs = it.r.w / (n * 0.7); fs > 0 && !fits(fs); fs-- {
			}
		}
		size = math.Min(size, fs)
	}
	size = math.Floor(size)
	for _, it := range items {
		t := &scene.Text{Family: d.family, Size: size, Color: col, Runs: []scene.Span{{Text: it.label}}}
		if horizontal {
			t.X, t.Y, t.Baseline, t.Anchor = it.r.x+it.r.w-10, it.r.y+it.r.h/2, scene.MiddleBaseline, scene.End
			if outside {
				t.X, t.Anchor = it.r.x+it.r.w+10, scene.Start
			}
		} else {
			t.X, t.Y, t.Anchor, t.Baseline = it.r.x+it.r.w/2, it.r.y+10, scene.Middle, scene.Hanging
			if outside {
				t.Y, t.Baseline = it.r.y-10, scene.Alphabetic
			}
		}
		d.items = append(d.items, t)
	}
}

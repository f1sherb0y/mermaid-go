// Package timeline is mermaid's timeline (diagrams/timeline): periods with
// their events, grouped in sections, along a horizontal (LR, the default)
// or vertical (TD) axis.
package timeline

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

func init() { diagram.Register("timeline", Render) }

// Timeline is a parsed timeline.
type Timeline struct {
	Title    string
	Vertical bool
	Sections []string
	Tasks    []*Task
}

// Task is a period and its events.
type Task struct {
	Period  string
	Section string
	Events  []string
}

// Parse reads a timeline (timeline.jison): `period : event : event`, and
// lines starting with ":" add events to the last period.
func Parse(code string) (*Timeline, error) {
	t := &Timeline{}
	started := false
	section := ""
	for n, line := range strings.Split(code, "\n") {
		if i := strings.Index(line, "%%"); i >= 0 {
			line = line[:i]
		}
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		if !started {
			f := strings.Fields(l)
			if f[0] != "timeline" {
				return nil, fmt.Errorf("timeline: line %d: expected timeline", n+1)
			}
			if len(f) > 1 {
				switch f[1] {
				case "TD":
					t.Vertical = true
				case "LR":
				default:
					return nil, fmt.Errorf("timeline: line %d: unknown direction %q", n+1, f[1])
				}
			}
			started = true
			continue
		}
		word, rest, _ := strings.Cut(l, " ")
		switch {
		case word == "title":
			t.Title = strings.TrimSpace(rest)
		case strings.HasPrefix(word, "accTitle") || strings.HasPrefix(word, "accDescr"):
		case word == "section":
			section = strings.TrimSpace(rest)
			t.Sections = append(t.Sections, section)
		case strings.HasPrefix(l, ":"):
			if len(t.Tasks) == 0 {
				return nil, fmt.Errorf("timeline: line %d: an event needs a period first", n+1)
			}
			last := t.Tasks[len(t.Tasks)-1]
			last.Events = append(last.Events, events(l)...)
		default:
			period, evs, _ := strings.Cut(l, ":")
			task := &Task{Period: strings.TrimSpace(period), Section: section}
			if evs != "" {
				task.Events = events(":" + evs)
			}
			t.Tasks = append(t.Tasks, task)
		}
	}
	if !started {
		return nil, fmt.Errorf("timeline: empty")
	}
	return t, nil
}

func events(s string) []string {
	var out []string
	for _, e := range strings.Split(s, ":")[1:] {
		out = append(out, strings.TrimSpace(e))
	}
	return out
}

var wordRe = regexp.MustCompile(`\s+|<br\s*/?>`)

// wrap splits a label into lines no wider than width (d3 wrap: words, and
// <br> breaks).
func wrap(fs *text.Fonts, s string, st text.Style, width float64) []string {
	var lines []string
	cur := []string{}
	last := 0
	flush := func() {
		lines = append(lines, strings.TrimSpace(strings.Join(cur, " ")))
	}
	tokens := wordRe.FindAllStringIndex(s, -1)
	var words []string
	for _, t := range tokens {
		if w := s[last:t[0]]; w != "" {
			words = append(words, w)
		}
		if strings.HasPrefix(s[t[0]:t[1]], "<br") {
			words = append(words, "<br>")
		}
		last = t[1]
	}
	if w := s[last:]; w != "" {
		words = append(words, w)
	}
	for _, w := range words {
		if w == "<br>" {
			flush()
			cur = nil
			continue
		}
		cur = append(cur, w)
		if len(cur) > 1 && fs.Width(strings.Join(cur, " "), st) > width {
			cur = cur[:len(cur)-1]
			flush()
			cur = []string{w}
		}
	}
	flush()
	return lines
}

// node is a box with wrapped text; Height is set by measure.
type node struct {
	lines         []string
	width, height float64
	padding       float64
}

// Render draws a timeline.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	tl, err := Parse(code)
	if err != nil {
		return nil, err
	}
	if tl.Title == "" {
		tl.Title = ctx.Title
	}
	cfg, th := ctx.Config, ctx.Theme
	family := th.Get("fontFamily")
	fontSize := cfg.Num("fontSize", 16)
	st := text.Style{Family: family, Size: fontSize}
	asc, desc := ctx.Fonts.Extents(st)
	leftMargin := cfg.Num("timeline.leftMargin", 50)
	multicolor := !cfg.Bool("timeline.disableMulticolor", false)

	// measure is mermaid's node height: the wrapped text's box, half a line
	// and the padding; at least minH.
	measure := func(s string, width, padding, minH float64) node {
		ls := wrap(ctx.Fonts, s, st, width)
		h := float64(len(ls)-1)*1.1*fontSize + asc + desc
		return node{lines: ls, width: width + 2*padding, padding: padding,
			height: math.Max(h+fontSize*1.1*0.5+padding, minH)}
	}
	colorOf := func(k string, i int) scene.Color { return style.Color(th.Get(fmt.Sprintf("%s%d", k, i%12))) }
	lineColor := colorOf("cScaleLabel", 11)
	var items []scene.Item
	// box draws a node at (x, y) in section color i; events are brighter.
	box := func(n node, x, y float64, i int, event bool) {
		fill, txt, under := colorOf("cScale", i), colorOf("cScaleLabel", i), colorOf("cScaleInv", i)
		if event {
			fill, txt, under = bright(fill), bright(txt), bright(under)
		}
		const r = 5.0
		var b scene.PathBuilder
		b.MoveTo(x, y+n.height-r).LineTo(x, y+r).QuadTo(x, y, x+r, y).LineTo(x+n.width-r, y).
			QuadTo(x+n.width, y, x+n.width, y+r).LineTo(x+n.width, y+n.height).LineTo(x, y+n.height).Close()
		var line scene.PathBuilder
		line.MoveTo(x, y+n.height).LineTo(x+n.width, y+n.height)
		items = append(items, &scene.Path{Segs: b.Segs, Fill: fill}, &scene.Path{Segs: line.Segs, Stroke: under, StrokeWidth: 3})
		for k, l := range n.lines {
			items = append(items, &scene.Text{X: x + n.width/2, Y: y + n.padding/2 + fontSize + float64(k)*1.1*fontSize,
				Anchor: scene.Middle, Baseline: scene.MiddleBaseline, Family: family, Size: fontSize, Color: txt, Runs: []scene.Span{{Text: l}}})
		}
	}
	// arrow draws a line ending in mermaid's arrowhead (6×4, its point one
	// unit past the end, sized in stroke widths).
	arrow := func(x1, y1, x2, y2, w float64, dashed bool) {
		var b scene.PathBuilder
		b.MoveTo(x1, y1).LineTo(x2, y2)
		p := &scene.Path{Segs: b.Segs, Stroke: lineColor, StrokeWidth: w}
		if dashed {
			p.Dash = []float64{5, 5}
		}
		sin, cos := math.Sincos(math.Atan2(y2-y1, x2-x1))
		pt := func(u, v float64) scene.Pt {
			u, v = (u-5)*w, (v-2)*w
			return scene.Pt{X: x2 + u*cos - v*sin, Y: y2 + u*sin + v*cos}
		}
		items = append(items, p, &scene.Path{Segs: scene.Polygon(pt(0, 0), pt(0, 4), pt(6, 2)), Fill: style.Color(th.Get("textColor"))})
	}
	sectionOf := func(s string) []*Task {
		var out []*Task
		for _, t := range tl.Tasks {
			if t.Section == s {
				out = append(out, t)
			}
		}
		return out
	}
	if tl.Vertical {
		renderVertical(tl, ctx, measure, box, arrow, sectionOf, multicolor, leftMargin, fontSize, &items)
	} else {
		renderHorizontal(tl, measure, box, arrow, sectionOf, multicolor, leftMargin, &items)
	}
	if tl.Title != "" {
		b, _ := scene.BoundsOf(items, ctx.TextBox)
		ex := ctx.Fonts.XHeight(text.Style{Family: family, Size: fontSize, Weight: 700})
		x := b.X + b.W/2 - leftMargin
		if ctx.Look == "neo" {
			x = b.X*2 + leftMargin
		}
		items = append(items, &scene.Text{X: x, Y: 20, Family: family, Size: 4 * ex, Color: style.Color(th.Get("textColor")),
			Runs: []scene.Span{{Text: tl.Title, Weight: 700}}})
	}
	return ctx.Fit(items, cfg.Num("timeline.padding", 50)), nil
}

type measureFn func(s string, width, padding, minH float64) node
type boxFn func(n node, x, y float64, i int, event bool)
type arrowFn func(x1, y1, x2, y2, w float64, dashed bool)

func renderHorizontal(tl *Timeline, measure measureFn, box boxFn, arrow arrowFn, sectionOf func(string) []*Task, multicolor bool, leftMargin float64, items *[]scene.Item) {
	maxSection, maxTask, maxEvents := 0.0, 0.0, 0.0
	for _, s := range tl.Sections {
		maxSection = math.Max(maxSection, measure(s, 150, 20, 0).height+20)
	}
	for _, t := range tl.Tasks {
		maxTask = math.Max(maxTask, measure(t.Period, 150, 20, 0).height+20)
		total := 0.0
		for _, e := range t.Events {
			total += measure(e, 150, 20, 0).height
		}
		if len(t.Events) > 0 {
			total += float64(len(t.Events)-1) * 10
		}
		maxEvents = math.Max(maxEvents, total)
	}
	x, y := 50+leftMargin, 50.0
	drawTasks := func(tasks []*Task, color int, x, y float64, each bool) {
		for _, t := range tasks {
			n := measure(t.Period, 150, 20, maxTask)
			box(n, x, y, color, false)
			if t.Events != nil {
				ey := y + 200
				for _, e := range t.Events {
					en := measure(e, 150, 20, 50)
					box(en, x, ey, color, true)
					ey += 10 + en.height
				}
				arrow(x+95, y+maxTask, x+95, y+maxTask+200+maxEvents, 2, true)
			}
			x += 200
			if each && multicolor {
				color++
			}
		}
	}
	if len(tl.Sections) > 0 {
		for i, s := range tl.Sections {
			tasks := sectionOf(s)
			n := measure(s, 200*math.Max(float64(len(tasks)), 1)-50, 20, maxSection)
			box(n, x, 50, i, false)
			if len(tasks) > 0 {
				drawTasks(tasks, i, x, y+maxSection+50, false)
			}
			x += 200 * math.Max(float64(len(tasks)), 1)
		}
	} else {
		drawTasks(tl.Tasks, 0, x, y, true)
	}
	depth := maxTask + 100
	if len(tl.Sections) > 0 {
		depth = maxSection + maxTask + 150
	}
	b, _ := scene.BoundsOf(*items, nil)
	arrow(leftMargin, depth, b.X+b.W+3*leftMargin, depth, 4, false)
}

func renderVertical(tl *Timeline, ctx *diagram.Context, measure measureFn, box boxFn, arrow arrowFn, sectionOf func(string) []*Task, multicolor bool, leftMargin, fontSize float64, items *[]scene.Item) {
	const (
		nodeW, pad     = 200.0, 5.0
		eventW         = nodeW + 100
		sectionTaskGap = 20.0
		taskAxisGap    = 20.0
		taskVGap       = 30.0
		eventAxisGap   = 50.0
	)
	left := nodeW + 2*pad + taskAxisGap
	right := eventW + 2*pad + eventAxisGap
	x0 := 50 + leftMargin
	axisX := x0 + left
	if len(tl.Sections) == 0 {
		axisX = x0 + left
	}
	maxTask, maxEvents := 0.0, 0.0
	for _, t := range tl.Tasks {
		maxTask = math.Max(maxTask, measure(t.Period, nodeW, pad, 0).height)
		total := 0.0
		for _, e := range t.Events {
			total += measure(e, eventW, pad, 0).height
		}
		if len(t.Events) > 0 {
			total += float64(len(t.Events)-1) * 10
		}
		maxEvents = math.Max(maxEvents, total)
	}
	spacing := math.Max(maxTask, maxEvents) + taskVGap
	maxSection := 0.0
	for _, s := range tl.Sections {
		maxSection = math.Max(maxSection, measure(s, math.Max(50, left+right-2*pad), pad, maxSection).height)
	}
	drawTasks := func(tasks []*Task, color int, y float64, each bool) {
		for _, t := range tasks {
			n := measure(t.Period, nodeW, pad, maxTask)
			box(n, axisX-taskAxisGap-n.width, y, color, false)
			ey := y
			for _, e := range t.Events {
				en := measure(e, eventW, pad, 0)
				box(en, axisX+eventAxisGap, ey, color, true)
				arrow(axisX, ey+en.height/2, axisX+eventAxisGap, ey+en.height/2, 2, true)
				ey += en.height + 10
			}
			y += spacing
			if each && multicolor {
				color++
			}
		}
	}
	y := 50.0
	if len(tl.Sections) > 0 {
		for i, s := range tl.Sections {
			tasks := sectionOf(s)
			n := measure(s, math.Max(50, left+right-2*pad), pad, maxSection)
			box(n, axisX-left, y, i, false)
			if len(tasks) > 0 {
				drawTasks(tasks, i, y+n.height+sectionTaskGap, false)
			}
			h := n.height + sectionTaskGap + spacing*math.Max(float64(len(tasks)), 1)
			if len(tasks) > 0 {
				h -= taskVGap * 2
			}
			y += h
		}
	} else {
		drawTasks(tl.Tasks, 0, y, true)
	}
	b, _ := scene.BoundsOf(*items, ctx.TextBox)
	// The axis goes under everything.
	before := len(*items)
	arrow(axisX, 50-fontSize*2, axisX, b.Y+b.H+fontSize*0.5+20, 4, false)
	*items = append((*items)[before:], (*items)[:before]...)
}

// bright is CSS brightness(120%).
func bright(c scene.Color) scene.Color {
	f := func(v uint8) uint8 { return uint8(math.Min(255, math.Round(float64(v)*1.2))) }
	return scene.Color{R: f(c.R), G: f(c.G), B: f(c.B), A: c.A}
}

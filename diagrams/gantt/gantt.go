// Package gantt is mermaid's gantt chart (diagrams/gantt): tasks as bars
// on a time axis, in sections, with milestones, vertical markers and
// excluded days.
//
// mermaid sizes a gantt to its container's width; we use gantt.useWidth,
// else defaultWidth. The today line is not drawn: it depends on when the
// chart is drawn (and so would every drawing).
package gantt

import (
	"math"
	"regexp"
	"sort"
	"time"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/d3"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

func init() { diagram.Register("gantt", Render) }

// defaultWidth is the chart's width without gantt.useWidth: the
// reference page's (mermaid falls back to 1200 without a container).
const defaultWidth = 1600

var (
	tickIntervalRe = regexp.MustCompile(`^([1-9]\d*)(millisecond|second|minute|hour|day|week|month)$`)
	brRe           = regexp.MustCompile(`(?i)<br\s*/?>`)
	weekdays       = map[string]time.Weekday{"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday,
		"wednesday": time.Wednesday, "thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday}
)

// Render draws a gantt chart.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	g, err := Parse(code)
	if err != nil {
		return nil, err
	}
	if g.Title == "" {
		g.Title = ctx.Title
	}
	cfg, th := ctx.Config, ctx.Theme
	num := func(k string, def float64) float64 { return cfg.Num("gantt."+k, def) }
	col := func(k string) scene.Color { return style.Color(th.Get(k)) }
	family := th.Get("fontFamily")
	var (
		w         = num("useWidth", defaultWidth)
		barH      = num("barHeight", 20)
		gap       = barH + num("barGap", 4)
		topPad    = num("topPadding", 50)
		leftPad   = num("leftPadding", 75)
		rightPad  = num("rightPadding", 75)
		gridStart = num("gridLineStartPadding", 35)
		fontSize  = num("fontSize", 11)
		nStyles   = max(1, int(num("numberSectionStyles", 4)))
		titleTop  = num("titleTopMargin", 25)
		compact   = cfg.Str("gantt.displayMode", "") == "compact"
	)
	tasks := g.Tasks
	bars := nonVert(tasks)

	// Sections in order of their first task, and their heights in rows.
	var categories []string
	for _, t := range bars {
		if !contains(categories, t.Section) {
			categories = append(categories, t.Section)
		}
	}
	section := func(t *Task) int {
		for i, c := range categories {
			if c == t.Section {
				return i % nStyles
			}
		}
		return 0
	}
	heights := map[string]int{}
	h := 2 * topPad
	if compact {
		rows := 0
		for _, c := range categories {
			var in []*Task
			for _, t := range bars {
				if t.Section == c {
					in = append(in, t)
				}
			}
			n := packRows(in, rows) + 1
			rows += n
			heights[c] = n
			h += float64(n) * gap
		}
	} else {
		h += float64(len(bars)) * gap
		for _, t := range bars {
			heights[t.Section]++
		}
	}

	// The time scale.
	var lo, hi time.Time
	for i, t := range tasks {
		if i == 0 || t.Start.Before(lo) {
			lo = t.Start
		}
		if i == 0 || t.End.After(hi) {
			hi = t.End
		}
	}
	span := w - leftPad - rightPad
	at := func(t time.Time) float64 {
		a, b := float64(lo.UnixMilli()), float64(hi.UnixMilli())
		return math.Floor(d3.Linear{D0: a, D1: b, R0: 0, R1: span}.At(float64(t.UnixMilli())) + 0.5)
	}
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].Start.Before(tasks[j].Start) })

	var items []scene.Item
	rect := func(x, y, rw, rh, r float64, fill, stroke scene.Color, sw float64) *scene.Path {
		return &scene.Path{Segs: scene.RectPath(x, y, rw, rh, r, r), Fill: fill, Stroke: stroke, StrokeWidth: sw}
	}
	label := func(s string, x, y, size float64, c scene.Color) *scene.Text {
		return &scene.Text{X: x, Y: y, Family: family, Size: size, Color: c, Runs: []scene.Span{{Text: s}}}
	}

	// Excluded days, as grey bands (not over more than five years).
	if len(tasks) > 0 && (len(g.Excludes) > 0 || len(g.Includes) > 0) && hi.Before(lo.AddDate(5, 0, 1)) {
		var start, end *time.Time
		flush := func() {
			if start != nil {
				x0 := at(dayStart(*start))
				items = append(items, rect(x0+leftPad, gridStart, at(dayStart(*end).AddDate(0, 0, 1).Add(-time.Millisecond))-x0, h-topPad-gridStart, 0, col("excludeBkgColor"), scene.None, 0))
				start, end = nil, nil
			}
		}
		for d := lo; !d.After(hi); d = d.AddDate(0, 0, 1) {
			if g.excluded(d) {
				dd := d
				if start == nil {
					start = &dd
				}
				end = &dd
			} else {
				flush()
			}
		}
		flush()
	}

	// The grid: ticks from the scale, or on the tick interval.
	axisFormat := g.AxisFormat
	if axisFormat == "" {
		axisFormat = cfg.Str("gantt.axisFormat", "%Y-%m-%d")
		if g.DateFormat == "D" {
			axisFormat = "%d"
		}
	}
	var interval *d3.Interval
	tickSpec := g.TickInterval
	if tickSpec == "" {
		tickSpec = cfg.Str("gantt.tickInterval", "")
	}
	if m := tickIntervalRe.FindStringSubmatch(tickSpec); m != nil && len(tasks) > 0 {
		every := 0
		for _, c := range m[1] {
			every = every*10 + int(c-'0')
		}
		unit := map[string]time.Duration{"millisecond": time.Millisecond, "second": time.Second, "minute": time.Minute,
			"hour": time.Hour, "day": 24 * time.Hour, "week": 7 * 24 * time.Hour, "month": 30 * 24 * time.Hour}[m[2]]
		if float64(hi.Sub(lo))/float64(unit*time.Duration(every)) <= 10000 {
			iv := map[string]*d3.Interval{"millisecond": d3.Millisecond, "second": d3.Second, "minute": d3.Minute,
				"hour": d3.Hour, "day": d3.Day, "month": d3.Month}[m[2]]
			if m[2] == "week" {
				wd := g.Weekday
				if _, ok := weekdays[wd]; !ok {
					wd = cfg.Str("gantt.weekday", "sunday")
				}
				iv = d3.Week(weekdays[wd])
			}
			interval = iv.Every(every)
		}
	}
	var ticks []time.Time
	if len(tasks) > 0 {
		ticks = d3.TimeTicks(lo, hi, interval, 10)
	}
	format := d3.TimeFormat(axisFormat)
	grid := func(axisY, y1, y2, textY float64) {
		var lines, labels []scene.Item
		for _, t := range ticks {
			x := at(t) + 0.5 + leftPad
			var b scene.PathBuilder
			b.MoveTo(x, y1).LineTo(x, y2)
			lines = append(lines, &scene.Path{Segs: b.Segs, Stroke: scene.RGB(0, 0, 0), StrokeWidth: 1})
			t := label(format(t), x, textY, 10, col("textColor"))
			t.Anchor = scene.Middle
			labels = append(labels, t)
		}
		items = append(items, &scene.Group{Opacity: 0.8, Items: append(lines, labels...)})
	}
	grid(h-50, topPad+gridStart-50, h-50, h-50+3+10)
	if g.TopAxis || cfg.Bool("gantt.topAxis", false) {
		grid(topPad, topPad, h-gridStart, topPad-3)
	}

	// Section rows behind the bars, one per row.
	sectionFill := func(i int) scene.Color {
		c := col([]string{"sectionBkgColor", "altSectionBkgColor", "sectionBkgColor2", "altSectionBkgColor"}[i%4])
		c.A *= 0.2
		return c
	}
	sort.SliceStable(tasks, func(i, j int) bool { return !tasks[i].Vert && tasks[j].Vert })
	seen := map[int]bool{}
	for _, t := range tasks {
		if t.Vert || seen[t.Order] {
			continue
		}
		seen[t.Order] = true
		items = append(items, rect(0, float64(t.Order)*gap+topPad-2, w-rightPad/2, gap, 0, sectionFill(section(t)), scene.None, 0))
	}

	// The bars, then their labels.
	var texts []scene.Item
	for _, t := range tasks {
		x0, x1 := at(t.Start), at(t.End)
		endX := x1
		if t.RenderEnd != nil {
			endX = at(*t.RenderEnd)
		}
		y := float64(t.Order)*gap + topPad
		fill, stroke := col("taskBkgColor"), col("taskBorderColor")
		switch {
		case t.Active && t.Crit:
			fill, stroke = col("activeTaskBkgColor"), col("critBorderColor")
		case t.Active:
			fill, stroke = col("activeTaskBkgColor"), col("activeTaskBorderColor")
		case t.Done && t.Crit:
			fill, stroke = col("doneTaskBkgColor"), col("critBorderColor")
		case t.Done:
			fill, stroke = col("doneTaskBkgColor"), col("doneTaskBorderColor")
		case t.Crit:
			fill, stroke = col("critBkgColor"), col("critBorderColor")
		}
		switch {
		case t.Vert:
			items = append(items, rect(x0+leftPad, gridStart, 0.08*barH, float64(len(bars))*gap+2*barH, 3, fill, col("vertLineColor"), 2))
		case t.Milestone:
			// A square turned 45° and scaled 0.8 about the bar's middle.
			cx, cy := x0+leftPad+(x1-x0)/2, y+barH/2
			m := scene.Translate(-cx, -cy).Then(scene.Scale(0.8, 0.8)).Then(scene.Rotate(45)).Then(scene.Translate(cx, cy))
			items = append(items, &scene.Group{Transform: m, Items: []scene.Item{rect(cx-barH/2, y, barH, barH, 3, fill, stroke, 2)}})
		case endX > x0: // SVG draws nothing for a rect of no width
			items = append(items, rect(x0+leftPad, y, endX-x0, barH, 3, fill, stroke, 2))
		}

		// The label: inside the bar when it fits, else beside it.
		st := text.Style{Family: family, Size: fontSize}
		tw := ctx.Fonts.Width(t.Name, st)
		startX := x0
		if t.Milestone {
			startX += (x1-x0)/2 - barH/2
			endX = startX + barH
		}
		lb := label(t.Name, 0, y+barH/2+(fontSize/2-2), fontSize, col("taskTextColor"))
		lb.Runs[0].Italic = t.Milestone
		inside := tw <= endX-startX
		switch {
		case t.Vert:
			lb.X, lb.Y, lb.Size, lb.Anchor, lb.Color = x0+leftPad, gridStart+float64(len(bars))*gap+60, 15, scene.Middle, col("vertLineColor")
		case inside:
			lb.X, lb.Anchor = (endX-startX)/2+startX+leftPad, scene.Middle
		case endX+tw+1.5*leftPad > w:
			lb.X, lb.Anchor, lb.Color = startX+leftPad-5, scene.End, col("taskTextOutsideColor")
		default:
			lb.X, lb.Color = endX+leftPad+5, col("taskTextOutsideColor")
		}
		if !t.Vert && (t.Active || (t.Done && inside)) {
			lb.Color = col("taskTextDarkColor")
		}
		texts = append(texts, lb)
	}
	items = append(items, texts...)

	// Section titles, centered on their rows.
	prev := 0
	size := num("sectionFontSize", 11)
	for _, c := range categories {
		rows := brRe.Split(c, -1)
		y := float64(heights[c])*gap/2 + float64(prev)*gap + topPad
		prev += heights[c]
		for j, row := range rows {
			t := label(row, 10, y+(float64(j)-float64(len(rows)-1)/2)*size, size, col("titleColor"))
			t.Baseline = scene.Central
			items = append(items, t)
		}
	}

	if g.Title != "" {
		c := col("titleColor")
		if c.IsNone() {
			c = col("textColor")
		}
		t := label(g.Title, w/2, titleTop, 18, c)
		t.Anchor = scene.Middle
		items = append(items, t)
	}
	return &scene.Scene{ViewBox: scene.Rect{W: w, H: h}, Items: items}, nil
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// packRows puts a section's tasks on as few rows as they fit on without
// overlapping (displayMode compact), numbering the rows from offset; it
// returns the last row used, less offset.
func packRows(tasks []*Task, offset int) int {
	sorted := append([]*Task(nil), tasks...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].Start.Equal(sorted[j].Start) {
			return sorted[i].Start.Before(sorted[j].Start)
		}
		return sorted[i].Order < sorted[j].Order
	})
	ends := make([]*time.Time, len(tasks))
	most := 0
	for _, t := range sorted {
		for j := range ends {
			if ends[j] == nil || !t.Start.Before(*ends[j]) {
				e := t.End
				ends[j] = &e
				t.Order = j + offset
				most = max(most, j)
				break
			}
		}
	}
	return most
}

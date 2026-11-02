// Package journey is mermaid's user journey (diagrams/user-journey): tasks
// in sections along a line, each with its people and a face for its score.
package journey

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

func init() { diagram.Register("journey", Render) }

// Journey is a parsed user journey.
type Journey struct {
	Title string
	Tasks []Task
}

// Task is a step: its section, score (NaN when not a number) and people.
type Task struct {
	Name, Section string
	Score         float64
	People        []string
}

// Actors are the people of all tasks, sorted.
func (j *Journey) Actors() []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range j.Tasks {
		for _, p := range t.People {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

var (
	keyword = regexp.MustCompile(`(?i)^(journey|title|section|accTitle\s*:|accDescr\s*:|accDescr\s*\{)`)
	stop    = regexp.MustCompile(`[#;]`)
	brRe    = regexp.MustCompile(`(?i)<br\s*/?>`)
)

// Parse reads a user journey (journey.jison): one statement per line —
// title, section, or `task: score: person, person`; # starts a comment.
func Parse(code string) (*Journey, error) {
	j := &Journey{}
	section := ""
	started, inDescr := false, false
	for n, line := range strings.Split(code, "\n") {
		if inDescr {
			if _, _, ok := strings.Cut(line, "}"); ok {
				inDescr = false
			}
			continue
		}
		if i := strings.Index(line, "%%"); i >= 0 {
			line = line[:i]
		}
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		kw := strings.ToLower(keyword.FindString(l))
		switch {
		case !started:
			if kw != "journey" {
				return nil, fmt.Errorf("journey: line %d: expected journey", n+1)
			}
			started = true
			if rest := strings.TrimSpace(l[len(kw):]); rest != "" {
				return nil, fmt.Errorf("journey: line %d: unexpected %q", n+1, rest)
			}
			continue
		case strings.HasPrefix(kw, "acctitle"), strings.HasPrefix(kw, "accdescr") && strings.HasSuffix(kw, ":"):
			continue
		case strings.HasPrefix(kw, "accdescr"):
			inDescr = !strings.Contains(l, "}")
			continue
		}
		l = stop.Split(l, 2)[0]
		switch {
		case kw == "title" && len(l) > 5 && (l[5] == ' ' || l[5] == '\t'):
			j.Title = strings.TrimSpace(l[6:])
		case kw == "section" && len(l) > 7 && (l[7] == ' ' || l[7] == '\t'):
			name, _, _ := strings.Cut(l[8:], ":")
			section = name
		default:
			name, data, ok := strings.Cut(l, ":")
			if !ok || strings.TrimSpace(data) == "" {
				return nil, fmt.Errorf("journey: line %d: expected `task: score: people`", n+1)
			}
			score, people, _ := strings.Cut(data, ":")
			t := Task{Name: strings.TrimSpace(name), Section: section, Score: jsNumber(score)}
			if people != "" {
				for _, p := range strings.Split(people, ",") {
					t.People = append(t.People, strings.TrimSpace(p))
				}
			}
			j.Tasks = append(j.Tasks, t)
		}
	}
	if !started {
		return nil, fmt.Errorf("journey: empty")
	}
	return j, nil
}

// jsNumber is JavaScript's Number(s) for decimal numbers: "" is 0.
func jsNumber(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return v
}

// Render draws a user journey.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	j, err := Parse(code)
	if err != nil {
		return nil, err
	}
	if j.Title == "" {
		j.Title = ctx.Title
	}
	cfg, th := ctx.Config, ctx.Theme
	num := func(k string, def float64) float64 { return cfg.Num("journey."+k, def) }
	family := th.Get("fontFamily")
	textColor := style.Color(th.Get("textColor"))
	labelStyle := text.Style{Family: family, Size: 16}
	var (
		width     = num("width", 150)
		height    = num("height", 50)
		taskGap   = num("taskMargin", 50)
		marginX   = num("diagramMarginX", 50)
		marginY   = num("diagramMarginY", 10)
		maxLabelW = num("maxLabelWidth", 360)
	)
	actorColors := strList(cfg.Get("journey.actorColours"), []string{"#8FBC8F", "#7CFC00", "#00FFFF", "#20B2AA", "#B0E0E6", "#FFFFE0"})
	sectionFills := strList(cfg.Get("journey.sectionFills"), []string{"#191970", "#8B008B", "#4B0082", "#2F4F4F", "#800000", "#8B4513", "#00008B"})
	// A section's fill: the theme's fillType<n> (mermaid's stylesheet), else
	// the config's sectionFills.
	sectionFill := func(n int) scene.Color {
		if c := th.Get(fmt.Sprintf("fillType%d", n%8)); c != "" {
			return style.Color(c)
		}
		return style.Color(sectionFills[n%len(sectionFills)])
	}

	var items []scene.Item
	circle := func(cx, cy, r float64, fill, stroke scene.Color, sw float64) {
		items = append(items, &scene.Path{Segs: scene.Ellipse(cx, cy, r, r), Fill: fill, Stroke: stroke, StrokeWidth: sw})
	}
	line := func(x1, y1, x2, y2 float64, col scene.Color, w float64, dash []float64) *scene.Path {
		var b scene.PathBuilder
		b.MoveTo(x1, y1).LineTo(x2, y2)
		p := &scene.Path{Segs: b.Segs, Stroke: col, StrokeWidth: w, Dash: dash}
		items = append(items, p)
		return p
	}
	// label is mermaid's foreignObject text: wrapped in the box, centered.
	asc, desc := ctx.Fonts.Extents(labelStyle)
	label := func(s string, x, y, w, h float64) {
		var lines []string
		for _, part := range brRe.Split(s, -1) {
			lines = append(lines, ctx.Fonts.Wrap(strings.TrimSpace(part), labelStyle, w)...)
		}
		lh := asc + desc
		top := y + (h-lh*float64(len(lines)))/2
		for i, l := range lines {
			items = append(items, &scene.Text{X: x + w/2, Y: top + float64(i)*lh + asc, Anchor: scene.Middle,
				Family: family, Size: 16, Color: textColor, Runs: []scene.Span{{Text: l}}})
		}
	}

	// The legend: one circle and name per actor.
	actors := j.Actors()
	color := map[string]scene.Color{}
	legendW := 0.0
	y := 60.0
	for i, a := range actors {
		color[a] = style.Color(actorColors[i%len(actorColors)])
		circle(20, y, 7, color[a], scene.RGB(0, 0, 0), 1)
		lines := wrapHyphen(ctx.Fonts, a, labelStyle, maxLabelW)
		for k, l := range lines {
			items = append(items, &scene.Text{X: 50, Y: y + 7 + float64(k)*20, Family: family, Size: 16, Color: textColor, Runs: []scene.Span{{Text: l}}})
			if w := ctx.Fonts.Width(l, labelStyle); w > legendW && w > num("leftMargin", 150)-w {
				legendW = w
			}
		}
		y += math.Max(20, float64(len(lines))*20)
	}
	left := num("leftMargin", 150) + legendW

	// Sections and tasks.
	section, sectionNum := "\x00", -1
	fill := scene.None
	const faceR = 15
	for i, t := range j.Tasks {
		x := float64(i)*(taskGap+width) + left
		if t.Section != section {
			section = t.Section
			sectionNum++
			fill = sectionFill(sectionNum)
			n := 0
			for _, u := range j.Tasks[i:] {
				if u.Section != section {
					break
				}
				n++
			}
			w := width*float64(n) + marginX*float64(n-1)
			items = append(items, &scene.Path{Segs: scene.RectPath(x, 50, w, height, 3, 3), Fill: fill, Stroke: scene.RGB(0x66, 0x66, 0x66), StrokeWidth: 1})
			label(t.Section, x, 50, w, height)
		}
		ty := 2*height + marginY
		center := x + width/2
		line(center, ty, center, 450, textColor, 1, []float64{4, 2})
		if !math.IsNaN(t.Score) {
			drawFace(&items, center, 300+(5-t.Score)*30, t.Score, th)
		}
		items = append(items, &scene.Path{Segs: scene.RectPath(x, ty, width, height, 3, 3), Fill: fill, Stroke: scene.RGB(0x66, 0x66, 0x66), StrokeWidth: 1})
		px := x + 14
		for _, p := range t.People {
			if c, ok := color[p]; ok {
				circle(px, ty, 7, c, scene.RGB(0, 0, 0), 1)
			}
			px += 10
		}
		label(t.Name, x, ty, width, height)
	}

	// The bounds mermaid sizes the drawing by: the legend's corner and each
	// task's column down to the faces' floor.
	stopX := left
	if n := len(j.Tasks); n > 0 {
		stopX = math.Max(stopX, float64(n-1)*(taskGap+width)+left+marginX+taskGap)
	}
	stopY := math.Max(float64(len(actors))*50, 450)
	if len(j.Tasks) == 0 {
		stopY = float64(len(actors)) * 50
	}
	h := stopY + 2*marginY
	w := left + stopX + 2*marginX

	// The activity line, ending in an arrowhead (6×4 in stroke widths).
	ax2 := w - left - 4
	line(left, height*4, ax2, height*4, textColor, 4, nil)
	tip := func(u, v float64) scene.Pt { return scene.Pt{X: ax2 + (u-5)*4, Y: height*4 + (v-2)*4} }
	items = append(items, &scene.Path{Segs: scene.Polygon(tip(0, 0), tip(0, 4), tip(6, 2)), Fill: textColor})

	extra := 0.0
	if j.Title != "" {
		size := 4 * ctx.Fonts.XHeight(text.Style{Family: family, Size: 16})
		titleColor := textColor
		if c := style.Color(cfg.Str("journey.titleColor", "")); !c.IsNone() {
			titleColor = c
		}
		items = append(items, &scene.Text{X: left, Y: 25, Family: cfg.Str("journey.titleFontFamily", family), Size: size,
			Color: titleColor, Runs: []scene.Span{{Text: j.Title, Weight: 700}}})
		extra = 70
	}
	return &scene.Scene{ViewBox: scene.Rect{X: 0, Y: -25, W: w, H: h + extra}, Items: items}, nil
}

// drawFace draws a score's face: a smile above 3, a frown below, a flat
// mouth at 3.
func drawFace(items *[]scene.Item, cx, cy, score float64, th diagram.Theme) {
	const r = 15
	faceFill := style.Color("#FFF8DC")
	if c := th.Get("faceColor"); c != "" {
		faceFill = style.Color(c)
	}
	grey := scene.RGB(0x66, 0x66, 0x66)
	*items = append(*items,
		&scene.Path{Segs: scene.Ellipse(cx, cy, r, r), Fill: faceFill, Stroke: scene.RGB(0x99, 0x99, 0x99), StrokeWidth: 2},
		&scene.Path{Segs: scene.Ellipse(cx-r/3, cy-r/3, 1.5, 1.5), Fill: grey, Stroke: grey, StrokeWidth: 2},
		&scene.Path{Segs: scene.Ellipse(cx+r/3, cy-r/3, 1.5, 1.5), Fill: grey, Stroke: grey, StrokeWidth: 2})
	// The mouth is a thin half ring (d3.arc, radii r/2 and r/2.2).
	mouth := func(oy float64, smile bool) {
		ro, ri := r/2.0, r/2.2
		s := 1.0
		if !smile {
			s = -1
		}
		var b scene.PathBuilder
		b.MoveTo(cx+s*ro, cy+oy).ArcTo(ro, ro, 0, true, true, cx-s*ro, cy+oy).
			LineTo(cx-s*ri, cy+oy).ArcTo(ri, ri, 0, true, false, cx+s*ri, cy+oy).Close()
		*items = append(*items, &scene.Path{Segs: b.Segs, Fill: style.Color(th.Get("textColor")), Stroke: grey, StrokeWidth: 1})
	}
	switch {
	case score > 3:
		mouth(2, true)
	case score < 3:
		mouth(7, false)
	default:
		var b scene.PathBuilder
		b.MoveTo(cx-5, cy+7).LineTo(cx+5, cy+7)
		*items = append(*items, &scene.Path{Segs: b.Segs, Stroke: grey, StrokeWidth: 1})
	}
}

// wrapHyphen is the legend's wrapping: words up to width, and words longer
// than that broken with a hyphen.
func wrapHyphen(fs *text.Fonts, s string, st text.Style, width float64) []string {
	if fs.Width(s, st) <= width {
		return []string{s}
	}
	var lines []string
	cur := ""
	for _, word := range strings.Split(s, " ") {
		next := word
		if cur != "" {
			next = cur + " " + word
		}
		if fs.Width(next, st) <= width {
			cur = next
			continue
		}
		if cur != "" {
			lines = append(lines, cur)
		}
		cur = word
		if fs.Width(word, st) > width {
			piece := ""
			for _, r := range word {
				piece += string(r)
				if fs.Width(piece+"-", st) > width {
					rs := []rune(piece)
					lines = append(lines, string(rs[:len(rs)-1])+"-")
					piece = string(r)
				}
			}
			cur = piece
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// strList is a config list of strings, or def.
func strList(v any, def []string) []string {
	l, ok := v.([]any)
	if !ok || len(l) == 0 {
		return def
	}
	out := make([]string, 0, len(l))
	for _, x := range l {
		out = append(out, fmt.Sprint(x))
	}
	return out
}

// Package gitgraph is mermaid's gitGraph (diagrams/git): commits on branch
// lanes, left to right (LR) or top to bottom (TB, BT), with merges,
// cherry-picks, tags and commit labels.
//
// Drawn with the classic themes' styles (mermaid's normalTheme); the
// redux and neo themes' geometry and colors are not done.
package gitgraph

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
	"github.com/f1sherb0y/mermaid-go/text"
)

func init() { diagram.Register("gitGraph", Render) }

const (
	layoutOffset = 10.0
	commitStep   = 40.0
	defaultPos   = 30.0
	px, py       = 4.0, 2.0
	colors       = 8
)

type point struct{ x, y float64 }

type branchPos struct {
	pos   float64
	index int
}

// renderer is the drawing's state, as mermaid's module globals.
type renderer struct {
	ctx      *diagram.Context
	g        *Graph
	byID     map[string]*Commit
	dir      string
	vertical bool
	parallel bool
	rotate   bool
	showLbl  bool
	branch   map[string]branchPos
	pos      map[string]point
	maxPos   float64
	lanes    []float64
	family   string

	branches, arrows, bullets, tagBkgs, labels []scene.Item
}

func (r *renderer) color(name string) scene.Color { return style.Color(r.ctx.Theme.Get(name)) }

// box is a text's size as getBBox measures it: rows of 1em, the last one
// the font's ascent + descent.
func (r *renderer) box(rows []string, size float64) (w, h float64) {
	st := text.Style{Family: r.family, Size: size}
	for _, row := range rows {
		w = math.Max(w, r.ctx.Fonts.Width(row, st))
	}
	asc, desc := r.ctx.Fonts.Extents(st)
	return w, float64(len(rows)-1)*size + asc + desc
}

func (r *renderer) text(s string, x, y, size float64, col scene.Color) *scene.Text {
	return &scene.Text{X: x, Y: y, Family: r.family, Size: size, Color: col, Runs: []scene.Span{{Text: s}}}
}

var rowSplit = regexp.MustCompile(`(?i)\\n|\n|<br\s*/?>`)

// Render draws a gitGraph.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	cfg := ctx.Config
	g, err := Parse(code, cfg.Str("gitGraph.mainBranchName", "main"), cfg.Num("gitGraph.mainBranchOrder", 0))
	if err != nil {
		return nil, err
	}
	if g.Title == "" {
		g.Title = ctx.Title
	}
	r := &renderer{ctx: ctx, g: g, byID: map[string]*Commit{}, dir: g.Dir, vertical: g.Dir != "LR",
		parallel: cfg.Bool("gitGraph.parallelCommits", false), rotate: cfg.Bool("gitGraph.rotateCommitLabel", true),
		showLbl: cfg.Bool("gitGraph.showCommitLabel", true),
		branch:  map[string]branchPos{}, pos: map[string]point{}, family: ctx.Theme.Get("fontFamily")}
	for _, c := range g.Commits {
		r.byID[c.ID] = c
	}
	p := 0.0
	for i, b := range g.Branches {
		w, _ := r.box(rowSplit.Split(b, -1), 16)
		r.branch[b] = branchPos{p, i}
		p += 50
		if r.rotate {
			p += 40
		}
		if r.vertical {
			p += w / 2
		}
	}
	r.commits(false)
	if cfg.Bool("gitGraph.showBranches", true) {
		r.drawBranches()
	}
	r.drawArrows()
	r.commits(true)

	items := append(append(append(append(r.branches, r.arrows...), r.bullets...), r.tagBkgs...), r.labels...)
	if g.Title != "" {
		b, _ := scene.BoundsOf(items, ctx.TextBox)
		t := r.text(g.Title, b.X+b.W/2, -cfg.Num("gitGraph.titleTopMargin", 25), 18, r.color("textColor"))
		t.Anchor = scene.Middle
		items = append(items, t)
	}
	return ctx.Fit(items, cfg.Num("gitGraph.diagramPadding", 8)), nil
}

func (r *renderer) sorted() []*Commit {
	cs := append([]*Commit(nil), r.g.Commits...)
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].Seq < cs[j].Seq })
	return cs
}

// closestParent is the parent furthest along the drawing's direction.
func (r *renderer) closestParent(parents []string) (string, bool) {
	best, target, found := "", 0.0, false
	if r.dir == "BT" {
		target = math.Inf(1)
	}
	for _, p := range parents {
		pp, ok := r.pos[p]
		if !ok {
			continue
		}
		v := pp.x
		if r.vertical {
			v = pp.y
		}
		if (r.dir == "BT" && v <= target) || (r.dir != "BT" && v >= target) {
			best, target, found = p, v, true
		}
	}
	return best, found
}

// parallelBT places BT's commits when parallel: by parent, upwards.
func (r *renderer) parallelBT(sorted []*Commit) {
	cur, maxP := defaultPos, defaultPos
	var roots []*Commit
	for _, c := range sorted {
		if len(c.Parents) > 0 {
			if p, ok := r.closestParent(c.Parents); ok {
				cur = r.pos[p].y + commitStep
			}
			maxP = math.Max(cur, maxP)
		} else {
			roots = append(roots, c)
		}
		r.pos[c.ID] = point{r.branch[c.Branch].pos, cur + layoutOffset}
	}
	for _, c := range roots {
		r.pos[c.ID] = point{r.branch[c.Branch].pos, maxP + defaultPos}
	}
	for _, c := range sorted {
		if len(c.Parents) == 0 {
			continue
		}
		best, minY := "", math.Inf(1)
		for _, p := range c.Parents {
			if y := r.pos[p].y; y <= minY {
				best, minY = p, y
			}
		}
		cur = r.pos[best].y - commitStep
		if cur <= maxP {
			maxP = cur
		}
		r.pos[c.ID] = point{r.branch[c.Branch].pos, cur - layoutOffset}
	}
}

// parallelPos is where a commit goes when parallel: one step past its
// closest parent.
func (r *renderer) parallelPos(c *Commit) float64 {
	if p, ok := r.closestParent(c.Parents); ok && len(c.Parents) > 0 {
		pp := r.pos[p]
		switch r.dir {
		case "TB":
			return pp.y + commitStep
		case "BT":
			return r.pos[c.ID].y - commitStep
		}
		return pp.x + commitStep
	} else if len(c.Parents) > 0 {
		return 0
	}
	switch r.dir {
	case "TB":
		return defaultPos
	case "BT":
		return r.pos[c.ID].y - commitStep
	}
	return 0
}

// commits places the commits and, on the second pass, draws them.
func (r *renderer) commits(draw bool) {
	p := 0.0
	if r.vertical {
		p = defaultPos
	}
	sorted := r.sorted()
	if r.dir == "BT" {
		if r.parallel {
			r.parallelBT(sorted)
		}
		for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
			sorted[i], sorted[j] = sorted[j], sorted[i]
		}
	}
	btParallel := r.dir == "BT" && r.parallel
	for _, c := range sorted {
		if r.parallel {
			p = r.parallelPos(c)
		}
		off := p + layoutOffset
		if btParallel {
			off = p
		}
		lane := r.branch[c.Branch].pos
		at := point{off, lane - 2}
		if r.vertical {
			at = point{lane, off}
		}
		if draw {
			idx := r.branch[c.Branch].index % colors
			r.bullet(c, at, idx)
			r.commitLabel(c, at, off, p)
			r.tags(c, at, off, p)
		}
		r.pos[c.ID] = at
		if btParallel {
			p += commitStep
		} else {
			p += commitStep + layoutOffset
		}
		r.maxPos = math.Max(r.maxPos, p)
	}
}

func (r *renderer) bullet(c *Commit, at point, idx int) {
	git, inv, primary := r.color(fmt.Sprintf("git%d", idx)), r.color(fmt.Sprintf("gitInv%d", idx)), r.color("primaryColor")
	add := func(it ...scene.Item) { r.bullets = append(r.bullets, it...) }
	circle := func(cx, cy, rad float64, fill, stroke scene.Color) *scene.Path {
		return &scene.Path{Segs: scene.Ellipse(cx, cy, rad, rad), Fill: fill, Stroke: stroke, StrokeWidth: 1}
	}
	switch c.Symbol {
	case Highlight:
		add(&scene.Path{Segs: scene.RectPath(at.x-10, at.y-10, 20, 20, 0, 0), Fill: inv, Stroke: inv, StrokeWidth: 1},
			&scene.Path{Segs: scene.RectPath(at.x-6, at.y-6, 12, 12, 0, 0), Fill: primary, Stroke: primary, StrokeWidth: 1})
	case CherryPick:
		white := scene.RGB(255, 255, 255)
		var l scene.PathBuilder
		l.MoveTo(at.x+3, at.y+1).LineTo(at.x, at.y-5).MoveTo(at.x-3, at.y+1).LineTo(at.x, at.y-5)
		add(circle(at.x, at.y, 10, r.color("textColor"), scene.None),
			circle(at.x-3, at.y+2, 2.75, white, scene.None), circle(at.x+3, at.y+2, 2.75, white, scene.None),
			&scene.Path{Segs: l.Segs, Stroke: white, StrokeWidth: 1})
	default:
		add(circle(at.x, at.y, 10, git, git))
		switch c.Symbol {
		case Merge:
			add(circle(at.x, at.y, 6, primary, primary))
		case Reverse:
			var b scene.PathBuilder
			b.MoveTo(at.x-5, at.y-5).LineTo(at.x+5, at.y+5).MoveTo(at.x-5, at.y+5).LineTo(at.x+5, at.y-5)
			add(&scene.Path{Segs: b.Segs, Stroke: primary, StrokeWidth: 3})
		}
	}
}

// rotateAbout is SVG's rotate(deg, cx, cy).
func rotateAbout(deg, cx, cy float64) scene.Matrix {
	return scene.Translate(-cx, -cy).Then(scene.Rotate(deg)).Then(scene.Translate(cx, cy))
}

func (r *renderer) commitLabel(c *Commit, at point, off, p float64) {
	if c.Type == CherryPick || (c.Type == Merge && !c.CustomID) || !r.showLbl {
		return
	}
	size := style.Px(r.ctx.Theme.Get("commitLabelFontSize"), 10)
	w, h := r.box([]string{c.ID}, size)
	bg := r.color("commitLabelBackground")
	bg.A *= 0.5
	bx, by, tx, ty := off-w/2-py, at.y+13.5, off-w/2, at.y+25
	if r.vertical {
		bx, by, tx, ty = at.x-(w+4*px+5), at.y-12, at.x-(w+4*px), at.y+h-12
	}
	g := &scene.Group{Items: []scene.Item{
		&scene.Path{Segs: scene.RectPath(bx, by, w+2*py, h+2*py, 0, 0), Fill: bg},
		r.text(c.ID, tx, ty, size, r.color("commitLabelColor")),
	}}
	if r.rotate {
		if r.vertical {
			g.Transform = rotateAbout(-45, at.x, at.y)
		} else {
			rx := -7.5 - ((w+10)/25)*9.5
			ry := 10 + (w/25)*8.5
			g.Transform = rotateAbout(-45, p, at.y).Then(scene.Translate(rx, ry))
		}
	}
	r.labels = append(r.labels, g)
}

func (r *renderer) tags(c *Commit, at point, off, p float64) {
	if len(c.Tags) == 0 {
		return
	}
	size := style.Px(r.ctx.Theme.Get("tagLabelFontSize"), 10)
	type tag struct {
		s       string
		w       float64
		yOffset float64
	}
	var tags []tag
	maxW, maxH := 0.0, 0.0
	for i := len(c.Tags) - 1; i >= 0; i-- {
		w, h := r.box([]string{c.Tags[i]}, size)
		maxW, maxH = math.Max(maxW, w), math.Max(maxH, h)
		tags = append(tags, tag{c.Tags[i], w, float64(len(tags)) * 20})
	}
	fill, stroke, hole, col := r.color("tagLabelBackground"), r.color("tagLabelBorder"), r.color("textColor"), r.color("tagLabelColor")
	h2 := maxH / 2
	for _, t := range tags {
		var pts []scene.Pt
		holeAt := point{}
		label := r.text(t.s, off-t.w/2, at.y-16-t.yOffset, size, col)
		var shape, text scene.Matrix
		if r.vertical {
			y0 := p + t.yOffset
			x := at.x
			pts = []scene.Pt{{X: x, Y: y0 + 2}, {X: x, Y: y0 - 2}, {X: x + layoutOffset, Y: y0 - h2 - 2},
				{X: x + layoutOffset + maxW + 4, Y: y0 - h2 - 2}, {X: x + layoutOffset + maxW + 4, Y: y0 + h2 + 2}, {X: x + layoutOffset, Y: y0 + h2 + 2}}
			holeAt = point{x + px/2, y0}
			label.X, label.Y = x+5, y0+3
			shape = rotateAbout(45, x, p).Then(scene.Translate(12, 12))
			text = rotateAbout(45, x, p).Then(scene.Translate(14, 14))
		} else {
			ly := at.y - 19.2 - t.yOffset
			pts = []scene.Pt{{X: p - maxW/2 - px/2, Y: ly + py}, {X: p - maxW/2 - px/2, Y: ly - py},
				{X: off - maxW/2 - px, Y: ly - h2 - py}, {X: off + maxW/2 + px, Y: ly - h2 - py},
				{X: off + maxW/2 + px, Y: ly + h2 + py}, {X: off - maxW/2 - px, Y: ly + h2 + py}}
			holeAt = point{p - maxW/2 + px/2, ly}
		}
		poly := &scene.Path{Segs: scene.Polygon(pts...), Fill: fill, Stroke: stroke, StrokeWidth: 1}
		holeDot := &scene.Path{Segs: scene.Ellipse(holeAt.x, holeAt.y, 1.5, 1.5), Fill: hole}
		if r.vertical {
			r.tagBkgs = append(r.tagBkgs, &scene.Group{Transform: shape, Items: []scene.Item{poly}})
			r.labels = append(r.labels, &scene.Group{Transform: shape, Items: []scene.Item{holeDot}},
				&scene.Group{Transform: text, Items: []scene.Item{label}})
			continue
		}
		r.tagBkgs = append(r.tagBkgs, poly)
		r.labels = append(r.labels, holeDot, label)
	}
}

func (r *renderer) drawBranches() {
	labelColor := func(i int) scene.Color { return r.color(fmt.Sprintf("gitBranchLabel%d", i%colors)) }
	for i, name := range r.g.Branches {
		p := r.branch[name].pos
		idx := i % colors
		spine := p - 2
		var l scene.PathBuilder
		switch r.dir {
		case "TB":
			l.MoveTo(p, defaultPos).LineTo(p, r.maxPos)
		case "BT":
			l.MoveTo(p, r.maxPos).LineTo(p, defaultPos)
		default:
			l.MoveTo(0, spine).LineTo(r.maxPos, spine)
		}
		if r.vertical {
			spine = p
		}
		r.lanes = append(r.lanes, spine)
		r.branches = append(r.branches, &scene.Path{Segs: l.Segs, Stroke: r.color("lineColor"),
			StrokeWidth: style.Px(r.ctx.Theme.Get("strokeWidth"), 1), Dash: []float64{2}})

		rows := rowSplit.Split(name, -1)
		w, h := r.box(rows, 16)
		shift := 0.0
		if r.rotate {
			shift = 30
		}
		bx, by := -w-4-shift-19, -h/2+10+spine-12
		lx, ly := -w-14-shift, spine-h/2-2
		switch r.dir {
		case "TB":
			bx, by, lx, ly = p-w/2-10, 0, p-w/2-5, 0
		case "BT":
			bx, by, lx, ly = p-w/2-10, r.maxPos, p-w/2-5, r.maxPos
		}
		r.branches = append(r.branches, &scene.Path{Segs: scene.RectPath(bx, by, w+18, h+4, 4, 4), Fill: r.color(fmt.Sprintf("git%d", idx))})
		for k, row := range rows {
			r.branches = append(r.branches, r.text(strings.TrimSpace(row), lx, ly+float64(k+1)*16, 16, labelColor(i)))
		}
	}
}

func (r *renderer) drawArrows() {
	for _, c := range r.g.Commits {
		for _, p := range c.Parents {
			if a := r.byID[p]; a != nil {
				r.arrow(a, c)
			}
		}
	}
}

// findLane is a free line between two positions for a rerouted arrow.
func (r *renderer) findLane(y1, y2 float64, depth int) float64 {
	cand := y1 + math.Abs(y1-y2)/2
	if depth > 5 {
		return cand
	}
	for _, l := range r.lanes {
		if math.Abs(l-cand) < 10 {
			return r.findLane(y1, y2-math.Abs(y1-y2)/5, depth+1)
		}
	}
	r.lanes = append(r.lanes, cand)
	return cand
}

func (r *renderer) arrow(a, b *Commit) {
	p1, p2 := r.pos[a.ID], r.pos[b.ID]
	// Rerouted: a commit on the curving branch lies between the two.
	furthest := p1.y < p2.y
	if r.vertical {
		furthest = p1.x < p2.x
	}
	curving := a.Branch
	if furthest {
		curving = b.Branch
	}
	reroute := false
	for _, x := range r.g.Commits {
		if x.Seq > a.Seq && x.Seq < b.Seq && x.Branch == curving {
			reroute = true
			break
		}
	}
	color := r.branch[b.Branch].index
	mergeSide := b.Type == Merge && a.ID != b.Parents[0]
	if mergeSide {
		color = r.branch[a.Branch].index
	}
	var d scene.PathBuilder
	d.MoveTo(p1.x, p1.y)
	if reroute {
		const rad = 10.0
		lineY := r.findLane(math.Min(p1.y, p2.y), math.Max(p1.y, p2.y), 0)
		lineX := r.findLane(math.Min(p1.x, p2.x), math.Max(p1.x, p2.x), 0)
		switch {
		case r.dir == "TB" && p1.x < p2.x:
			d.LineTo(lineX-rad, p1.y).ArcTo(rad, rad, 0, false, true, lineX, p1.y+rad).
				LineTo(lineX, p2.y-rad).ArcTo(rad, rad, 0, false, false, lineX+rad, p2.y)
		case r.dir == "TB":
			color = r.branch[a.Branch].index
			d.LineTo(lineX+rad, p1.y).ArcTo(rad, rad, 0, false, false, lineX, p1.y+rad).
				LineTo(lineX, p2.y-rad).ArcTo(rad, rad, 0, false, true, lineX-rad, p2.y)
		case r.dir == "BT" && p1.x < p2.x:
			d.LineTo(lineX-rad, p1.y).ArcTo(rad, rad, 0, false, false, lineX, p1.y-rad).
				LineTo(lineX, p2.y+rad).ArcTo(rad, rad, 0, false, true, lineX+rad, p2.y)
		case r.dir == "BT":
			color = r.branch[a.Branch].index
			d.LineTo(lineX+rad, p1.y).ArcTo(rad, rad, 0, false, true, lineX, p1.y-rad).
				LineTo(lineX, p2.y+rad).ArcTo(rad, rad, 0, false, false, lineX-rad, p2.y)
		case p1.y < p2.y:
			d.LineTo(p1.x, lineY-rad).ArcTo(rad, rad, 0, false, false, p1.x+rad, lineY).
				LineTo(p2.x-rad, lineY).ArcTo(rad, rad, 0, false, true, p2.x, lineY+rad)
		default:
			color = r.branch[a.Branch].index
			d.LineTo(p1.x, lineY+rad).ArcTo(rad, rad, 0, false, true, p1.x+rad, lineY).
				LineTo(p2.x-rad, lineY).ArcTo(rad, rad, 0, false, false, p2.x, lineY-rad)
		}
	} else {
		const rad = 20.0
		switch {
		case r.dir == "TB" && p1.x < p2.x && mergeSide:
			d.LineTo(p1.x, p2.y-rad).ArcTo(rad, rad, 0, false, false, p1.x+rad, p2.y)
		case r.dir == "TB" && p1.x < p2.x:
			d.LineTo(p2.x-rad, p1.y).ArcTo(rad, rad, 0, false, true, p2.x, p1.y+rad)
		case r.dir == "TB" && p1.x > p2.x && mergeSide:
			d.LineTo(p1.x, p2.y-rad).ArcTo(rad, rad, 0, false, true, p1.x-rad, p2.y)
		case r.dir == "TB" && p1.x > p2.x:
			d.LineTo(p2.x+rad, p1.y).ArcTo(rad, rad, 0, false, false, p2.x, p1.y+rad)
		case r.dir == "BT" && p1.x < p2.x && mergeSide:
			d.LineTo(p1.x, p2.y+rad).ArcTo(rad, rad, 0, false, true, p1.x+rad, p2.y)
		case r.dir == "BT" && p1.x < p2.x:
			d.LineTo(p2.x-rad, p1.y).ArcTo(rad, rad, 0, false, false, p2.x, p1.y-rad)
		case r.dir == "BT" && p1.x > p2.x && mergeSide:
			d.LineTo(p1.x, p2.y+rad).ArcTo(rad, rad, 0, false, false, p1.x-rad, p2.y)
		case r.dir == "BT" && p1.x > p2.x:
			d.LineTo(p2.x+rad, p1.y).ArcTo(rad, rad, 0, false, true, p2.x, p1.y-rad)
		case r.vertical:
		case p1.y < p2.y && mergeSide:
			d.LineTo(p2.x-rad, p1.y).ArcTo(rad, rad, 0, false, true, p2.x, p1.y+rad)
		case p1.y < p2.y:
			d.LineTo(p1.x, p2.y-rad).ArcTo(rad, rad, 0, false, false, p1.x+rad, p2.y)
		case p1.y > p2.y && mergeSide:
			d.LineTo(p2.x-rad, p1.y).ArcTo(rad, rad, 0, false, false, p2.x, p1.y-rad)
		case p1.y > p2.y:
			d.LineTo(p1.x, p2.y+rad).ArcTo(rad, rad, 0, false, true, p1.x+rad, p2.y)
		}
	}
	d.LineTo(p2.x, p2.y)
	r.arrows = append(r.arrows, &scene.Path{Segs: d.Segs, Stroke: r.color(fmt.Sprintf("git%d", color%colors)),
		StrokeWidth: 8, Cap: scene.RoundCap})
}

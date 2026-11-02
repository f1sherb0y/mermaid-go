// Package treeview is mermaid's tree view (diagrams/treeView): a file tree
// given by indentation or by box-drawing characters (├── └── │).
package treeview

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

func init() { diagram.Register("treeView", Render) }

// Node is an entry; Dir when its name ended with "/".
type Node struct {
	Name        string
	Dir         bool
	Class       string
	Icon        string // "" none given; "none" for icon()
	Description string
	Children    []*Node
	level       int
}

var (
	boxChars   = regexp.MustCompile(`[─━│┃└┗├┣]`)
	branchChar = regexp.MustCompile(`[└┗├┣]`)
	decoOnly   = regexp.MustCompile(`^[\s│┃]+$`)
	metaLine   = regexp.MustCompile(`^\s*(title[\t ]|accTitle[\t ]*:|accDescr[\t ]*[:{])`)
	annotRe    = regexp.MustCompile(`^(.*?)((?:[ \t]+(?::::[ \t]*[A-Za-z_][\w-]*|icon\([\w-]*(?::[\w-]+)?\)|##[^\n\r]*))*)$`)
	classRe    = regexp.MustCompile(`[ \t]+:::[ \t]*([A-Za-z_][\w-]*)`)
	iconRe     = regexp.MustCompile(`[ \t]+icon\(([\w-]*(?::[\w-]+)?)\)`)
	descRe     = regexp.MustCompile(`[ \t]+##[ \t]*([^\n\r]*)`)
)

// Parse reads a tree view; returns the root ("/", holding the top level).
func Parse(code string) (*Node, string, error) {
	lines := strings.Split(code, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "treeView-beta" {
			start = i
			break
		}
		if strings.TrimSpace(l) != "" {
			break
		}
	}
	if start < 0 {
		return nil, "", fmt.Errorf("treeView: expected treeView-beta")
	}
	body := lines[start+1:]
	boxed := false
	var content []string
	for _, l := range body {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "%%") || metaLine.MatchString(l) || decoOnly.MatchString(l) {
			continue
		}
		content = append(content, strings.ReplaceAll(l, "\t", "    "))
		if boxChars.MatchString(l) {
			boxed = true
		}
	}
	// Box drawing: a branch's column gives its depth.
	if boxed {
		seg := 4
		for _, l := range content {
			if loc := branchChar.FindStringIndex(l); loc != nil && loc[0] > 0 {
				seg = len([]rune(l[:loc[0]]))
				break
			}
		}
		for i, l := range body {
			n := strings.ReplaceAll(l, "\t", "    ")
			t := strings.TrimSpace(n)
			if t == "" || strings.HasPrefix(t, "%%") || metaLine.MatchString(n) {
				continue
			}
			if decoOnly.MatchString(n) || regexp.MustCompile(`^[\s─━│┃└┗├┣]+$`).MatchString(n) {
				body[i] = ""
				continue
			}
			loc := branchChar.FindStringIndex(n)
			if loc == nil {
				if !boxChars.MatchString(n) && strings.TrimLeft(n, " ") != n {
					return nil, "", fmt.Errorf("Line %d: Unexpected indentation without box-drawing characters. In box-drawing format, use ├── or └── prefixes for indented nodes.", start+i+2)
				}
				continue
			}
			col := len([]rune(n[:loc[0]]))
			depth := int(math.Round(float64(col)/float64(seg))) + 1
			rest := strings.TrimLeft(strings.TrimLeft(n[loc[1]:], "─━"), " ")
			rest = strings.TrimRight(rest, " \t\r")
			if rest == "" {
				return nil, "", fmt.Errorf("Line %d: Empty node — expected a filename or directory name after the box-drawing prefix", start+i+2)
			}
			body[i] = strings.Repeat("    ", depth) + rest
		}
	}
	root := &Node{Name: "/", Dir: true, level: -1}
	stack := []*Node{root}
	title := ""
	for _, l := range body {
		if i := strings.Index(l, "%%"); i >= 0 {
			l = l[:i]
		}
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if m := metaLine.FindStringSubmatch(l); m != nil {
			if strings.HasPrefix(t, "title") {
				title = strings.TrimSpace(t[5:])
			}
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " \t"))
		line := strings.TrimRight(l[indent:], " \t\r")
		m := annotRe.FindStringSubmatch(line)
		name, annots := m[1], m[2]
		n := &Node{level: indent}
		if len(name) >= 2 && (name[0] == '"' || name[0] == '\'') && name[len(name)-1] == name[0] {
			name = name[1 : len(name)-1]
		}
		if strings.HasSuffix(name, "/") {
			n.Dir, name = true, strings.TrimSuffix(name, "/")
		}
		n.Name = name
		if c := classRe.FindStringSubmatch(annots); c != nil {
			n.Class = c[1]
		}
		if c := iconRe.FindStringSubmatch(annots); c != nil {
			n.Icon = c[1]
			if n.Icon == "" {
				n.Icon = "none"
			}
		}
		if c := descRe.FindStringSubmatch(annots); c != nil {
			n.Description = strings.TrimSpace(c[1])
		}
		for indent <= stack[len(stack)-1].level {
			stack = stack[:len(stack)-1]
		}
		p := stack[len(stack)-1]
		p.Children = append(p.Children, n)
		stack = append(stack, n)
	}
	return root, title, nil
}

const (
	iconSize = 14.0
	iconGap  = 4.0
	descGap  = 16.0
)

// Render draws a tree view.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	root, _, err := Parse(code)
	if err != nil {
		return nil, err
	}
	cfg, th := ctx.Config, ctx.Theme
	num := func(k string) float64 { return cfg.Num("treeView."+k, 0) }
	padX, padY, indentStep, thick := num("paddingX"), num("paddingY"), num("rowIndent"), num("lineThickness")
	showIcons := cfg.Bool("treeView.showIcons", false)
	opt := func(k, def string) string {
		if v := th.Get("treeView." + k); v != "" {
			return v
		}
		return def
	}
	family := th.Get("fontFamily")
	size := style.Px(opt("labelFontSize", "16px"), 16)
	labelColor, lineColor := style.Color(opt("labelColor", "black")), style.Color(opt("lineColor", "black"))
	descColor := style.Color(opt("descriptionColor", "#6a9955"))
	asc, desc := ctx.Fonts.Extents(text.Style{Family: family, Size: size})
	rowH := asc + desc + 2*padY

	type row struct {
		n            *Node
		x, y, labelR float64
		group        *scene.Group
	}
	var rows []*row
	var lines []scene.Item
	line := func(x1, y1, x2, y2 float64) {
		var b scene.PathBuilder
		b.MoveTo(x1, y1).LineTo(x2, y2)
		lines = append(lines, &scene.Path{Segs: b.Segs, Stroke: lineColor, StrokeWidth: thick})
	}
	var totalW, totalH float64
	var walk func(n *Node, depth int) *row
	walk = func(n *Node, depth int) *row {
		x := float64(depth) * (indentStep + padX)
		y := totalH
		icon := n.Icon != "none" && (n.Icon != "" || showIcons)
		off := 0.0
		if icon {
			off = iconSize + iconGap
		}
		weight := 400
		if n.Dir {
			weight = 700
		}
		w := ctx.Fonts.Width(n.Name, text.Style{Family: family, Size: size, Weight: weight})
		lx := x + padX + off
		g := &scene.Group{Items: []scene.Item{&scene.Text{X: lx, Y: y + rowH/2, Baseline: scene.MiddleBaseline, Family: family, Size: size,
			Color: labelColor, Runs: []scene.Span{{Text: n.Name, Weight: weight}}}}}
		r := &row{n: n, x: x, y: y, labelR: lx + w, group: g}
		rows = append(rows, r)
		line(x-indentStep, y+rowH/2, x, y+rowH/2)
		totalW = math.Max(totalW, x+w+2*padX+off)
		totalH += rowH
		var last *row
		for _, c := range n.Children {
			last = walk(c, depth+1)
		}
		if last != nil {
			line(x+padX, y+rowH, x+padX, last.y+rowH/2+thick/2)
		}
		return r
	}
	walk(root, 0)
	maxRight := 0.0
	for _, r := range rows {
		maxRight = math.Max(maxRight, r.labelR)
	}
	for _, r := range rows {
		if r.n.Description == "" {
			continue
		}
		dx := maxRight + descGap
		r.group.Items = append(r.group.Items, &scene.Text{X: dx, Y: r.y + rowH/2, Baseline: scene.MiddleBaseline, Family: family, Size: size,
			Color: descColor, Runs: []scene.Span{{Text: r.n.Description, Italic: true}}})
		totalW = math.Max(totalW, dx+ctx.Fonts.Width(r.n.Description, text.Style{Family: family, Size: size, Italic: true})+padX)
	}
	for _, r := range rows {
		if !strings.Contains(" "+r.n.Class+" ", " highlight ") {
			continue
		}
		w := totalW - r.x + 8
		r.group.Items = append([]scene.Item{&scene.Path{Segs: scene.RectPath(r.x, r.y+1, w, rowH-2, 3, 3),
			Fill: style.Color(opt("highlightBg", "rgba(255, 193, 7, 0.15)")), Stroke: style.Color(opt("highlightStroke", "#ffc107")), StrokeWidth: 1}},
			r.group.Items...)
		totalW = math.Max(totalW, r.x+w+2)
	}
	items := lines
	for _, r := range rows {
		items = append(items, r.group)
	}
	return &scene.Scene{ViewBox: scene.Rect{X: -thick / 2, W: totalW, H: totalH}, Items: items}, nil
}

package render

import (
	"strings"

	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

// sheet is what a diagram type's stylesheet (diagrams/<type>/styles.*, the
// global styles.ts and the look's rules) gives the elements the renderer
// draws, resolved against the theme. Inline styles from the diagram
// (classDef, style, linkStyle) are applied over it per element.
type sheet struct {
	// kind is the diagram family: "flowchart", "class", "state", "er",
	// "requirement" or "" for others; the per-type rules key off it.
	kind string
	font font // node labels

	nodeFill, nodeStroke scene.Color
	nodeWidth            float64
	// shadow is the neo look's drop shadow (rects, polygons, circles and
	// outer paths of nodes and clusters); nil for other looks.
	shadow *scene.Shadow

	clusterFill, clusterStroke scene.Color
	clusterWidth               float64
	clusterText                scene.Color
	// palette: color themes paint each cluster with a slot's colors.
	palette []slot
	// nodeSlots says which parts of a node take its palette slot's colors
	// in the neo look: none, the outline and dividers (class), or every
	// path (ER, requirement).
	nodeSlots slotParts

	noteFill, noteStroke scene.Color
	// startFill colors state start dots and fork bars.
	startFill scene.Color
	// terminalSize is the end labels' font size (class multiplicities).
	terminalSize float64

	edgeStroke  scene.Color
	edgeWidth   float64 // edge-thickness-normal
	edgeLabelBg scene.Color
	edgeText    scene.Color
	marker      scene.Color

	titleColor scene.Color
	titleSize  float64

	markers                                     map[string]*markerDef
	markerFill, markerStroke, markerCrossStroke scene.Color

	// sections are mindmap's and kanban's section colors (sections.go).
	sections *sections
}

type slot struct{ border, fill scene.Color }

type slotParts uint8

const (
	slotNone slotParts = iota
	slotOutline
	slotAll
)

// diagramKind is the family of a mermaid diagram type id.
func diagramKind(typ string) string {
	switch {
	case strings.HasPrefix(typ, "flowchart"), typ == "graph":
		return "flowchart"
	case strings.HasPrefix(typ, "class"):
		return "class"
	case strings.HasPrefix(typ, "state"):
		return "state"
	case typ == "er" || strings.HasPrefix(typ, "entityRelationship"):
		return "er"
	case strings.HasPrefix(typ, "requirement"):
		return "requirement"
	case typ == "mindmap", typ == "kanban":
		return typ
	}
	return ""
}

func (r *renderer) color(name string) scene.Color { return style.Color(r.th.Get(name)) }

// newSheet resolves the stylesheet of the diagram type.
func (r *renderer) newSheet() *sheet {
	th := r.th
	textColor := r.color("textColor")
	nodeText := textColor
	if c := r.color("nodeTextColor"); !c.IsNone() {
		nodeText = c
	}
	sw := style.Px(th.Get("strokeWidth"), 1)
	s := &sheet{
		font: font{
			Family: th.Get("fontFamily"), Size: style.Px(th.Get("fontSize"), 16),
			Weight: 400, Color: nodeText, LineHeight: 1.5, Align: scene.Middle,
		},
		nodeFill:      r.color("mainBkg"),
		nodeStroke:    r.color("nodeBorder"),
		nodeWidth:     sw,
		clusterFill:   r.color("clusterBkg"),
		clusterStroke: r.color("clusterBorder"),
		clusterWidth:  1,
		clusterText:   r.color("titleColor"),
		edgeStroke:    r.color("lineColor"),
		edgeWidth:     sw,
		edgeLabelBg:   r.color("edgeLabelBackground"),
		edgeText:      nodeText,
		marker:        r.color("lineColor"),
		titleColor:    textColor,
		titleSize:     18,
		noteFill:      r.color("noteBkgColor"),
		noteStroke:    r.color("noteBorderColor"),
		startFill:     r.color("lineColor"),
		terminalSize:  0,
	}
	s.kind = diagramKind(r.data.DiagramType)
	s.markerFill, s.markerStroke, s.markerCrossStroke = s.marker, s.marker, s.marker
	if c := r.color("specialStateColor"); !c.IsNone() {
		s.startFill = c
	}
	// The diagram types' stylesheets (diagrams/<type>/styles.*).
	switch s.kind {
	case "class":
		s.markers = markerSet("aggregation", "extension", "composition", "dependency", "lollipop")
		s.edgeLabelBg = s.nodeFill // .labelBkg { background: mainBkg }
		s.nodeSlots = slotOutline
		s.terminalSize = 11
	case "state":
		s.markers = markerSet("barb")
		s.clusterStroke = s.nodeStroke
	case "er":
		s.markers = markerSet("onlyOne", "zeroOrOne", "oneOrMore", "zeroOrMore")
		s.markerFill = scene.None
		s.edgeLabelBg = r.color("erEdgeLabelBackground")
		if s.edgeLabelBg.IsNone() {
			s.edgeLabelBg = r.color("tertiaryColor")
			s.edgeLabelBg.A *= 0.5
		}
		s.nodeSlots = slotAll
	case "requirement":
		s.markers = markerSet("requirement_contains", "requirement_arrow")
		s.edgeLabelBg = r.color("requirementEdgeLabelBackground")
		if s.edgeLabelBg.IsNone() {
			s.edgeLabelBg = r.color("edgeLabelBackground")
		}
		s.nodeSlots = slotAll
	case "mindmap":
		// No markers; nodes are filled by section, without a border.
		s.sections = r.newSections(s.kind)
		s.nodeStroke = scene.None
	case "kanban":
		s.sections = r.newSections(s.kind)
		// .node rect, .cluster-label, .label
		s.nodeFill, s.nodeStroke, s.nodeWidth = r.color("background"), r.color("nodeBorder"), 1
		s.font.Color, s.clusterText = textColor, textColor
	default:
		s.markers = markerSet("point", "circle", "cross")
	}
	if r.look != "neo" {
		s.nodeSlots = slotNone
	}
	// Only flowchart's and state's stylesheets size their title's class;
	// the others' titles take the diagram's font size.
	if s.kind != "flowchart" && s.kind != "state" {
		s.titleSize = s.font.Size
	}
	if r.look == "neo" && th.Get("dropShadow") != "" {
		flood := scene.Color{A: 0.06}
		if strings.Contains(r.themeName, "dark") {
			flood = scene.Color{R: 255, G: 255, B: 255, A: 0.06}
		}
		s.shadow = &scene.Shadow{DX: 4, DY: 4, Color: flood}
	}
	if borders := splitList(th.Get("borderColorArray")); len(borders) > 0 && isColorTheme(r.themeName) {
		fills := splitList(th.Get("bkgColorArray"))
		n := len(borders)
		if lim := int(style.Px(th.Get("THEME_COLOR_LIMIT"), 0)); lim > 0 && lim < n {
			n = lim
		}
		for i := range n {
			sl := slot{border: style.Color(borders[i%len(borders)])}
			if len(fills) > 0 {
				sl.fill = style.Color(fills[i%len(fills)])
			}
			s.palette = append(s.palette, sl)
		}
	}
	return s
}

// isColorTheme reports the themes with a per-container palette
// (colorThemeGate.ts).
func isColorTheme(name string) bool { return strings.HasSuffix(name, "-color") }

// splitList splits a JavaScript array's String() at its top-level commas.
func splitList(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if t := strings.TrimSpace(s[start:]); t != "" {
		out = append(out, t)
	}
	return out
}

// paint is how a path is filled and stroked.
type paint struct {
	Fill, Stroke scene.Color
	Width        float64
	Dash         []float64
	Shadow       *scene.Shadow
}

// path is the paint applied to segments.
func (p paint) path(segs []scene.Seg) *scene.Path {
	return &scene.Path{Segs: segs, Fill: p.Fill, Stroke: p.Stroke, StrokeWidth: p.Width, Dash: p.Dash, Shadow: p.Shadow}
}

// withCSS is the paint with inline declarations applied: fill, stroke,
// stroke-width, stroke-dasharray, opacities.
func (p paint) withCSS(css ...string) paint {
	d := style.Parse(css...)
	if v, ok := d["fill"]; ok {
		p.Fill = style.Color(v)
	}
	if v, ok := d["stroke"]; ok {
		p.Stroke = style.Color(v)
	}
	if v, ok := d["stroke-width"]; ok {
		p.Width = style.Px(v, p.Width)
	}
	if v, ok := d["stroke-dasharray"]; ok {
		p.Dash = style.Dash(v)
		if v == "none" || allZero(p.Dash) {
			p.Dash = nil
		}
	}
	if v, ok := d["fill-opacity"]; ok {
		p.Fill.A *= style.Px(v, 1)
	}
	if v, ok := d["stroke-opacity"]; ok {
		p.Stroke.A *= style.Px(v, 1)
	}
	if v, ok := d["opacity"]; ok {
		o := style.Px(v, 1)
		p.Fill.A *= o
		p.Stroke.A *= o
	}
	if v, ok := d["filter"]; ok && v == "none" {
		p.Shadow = nil
	}
	return p
}

func allZero(fs []float64) bool {
	for _, f := range fs {
		if f != 0 {
			return false
		}
	}
	return true
}

// isLabelStyle reports the declarations mermaid gives a node's label rather
// than its shape (handDrawnShapeStyles.ts).
func isLabelStyle(key string) bool {
	switch key {
	case "color", "font-size", "font-family", "font-weight", "font-style", "text-decoration",
		"text-align", "text-transform", "line-height", "letter-spacing", "word-spacing",
		"text-shadow", "text-overflow", "white-space", "word-wrap", "word-break",
		"overflow-wrap", "hyphens":
		return true
	}
	return false
}

// nodeStyles splits a node's styles (classDef styles, then its own, then
// its label style; later ones win) into the shape's and the label's
// declarations, as styles2String.
func nodeStyles(n *Node) (shape, label []string) {
	var order []string
	m := map[string]string{}
	add := func(s string) {
		k, v, _ := strings.Cut(s, ":")
		k = strings.TrimSpace(k)
		// mermaid keeps only what is before a second colon.
		v, _, _ = strings.Cut(v, ":")
		if _, ok := m[k]; !ok {
			order = append(order, k)
		}
		m[k] = strings.TrimSpace(v)
	}
	for _, s := range n.CSSCompiledStyles {
		add(s)
	}
	for _, s := range n.CSSStyles {
		add(s)
	}
	for _, s := range strings.Split(n.LabelStyle, ";") {
		if strings.TrimSpace(s) != "" {
			add(strings.TrimSpace(s))
		}
	}
	for _, k := range order {
		d := k + ":" + m[k]
		if isLabelStyle(k) {
			label = append(label, d)
		} else {
			shape = append(shape, d)
		}
	}
	return shape, label
}

// Per-diagram parts of the sheet the shapes and edges ask for.

// markerSet is the markers a diagram type defines (its renderer's
// data4Layout.markers), by id suffix.
func markerSet(names ...string) map[string]*markerDef {
	m := map[string]*markerDef{}
	for _, n := range names {
		for _, suffix := range []string{"Start", "End", "Start-margin", "End-margin"} {
			if d := markerDefs[n+suffix]; d != nil {
				m[n+suffix] = d
			}
		}
	}
	return m
}

// edgeStrokeFor is the stroke the edge's classes give it.
func (s *sheet) edgeStrokeFor(e *estate) scene.Color {
	if s.sections != nil {
		if i, ok := s.sections.index(e.Classes, "section-edge-"); ok {
			return s.sections.fill[i]
		}
	}
	return s.edgeStroke
}

// edgeLabelBgFor is the background behind an edge's label.
func (s *sheet) edgeLabelBgFor(e *estate) scene.Color { return s.edgeLabelBg }

// nodePathCSS is what the stylesheet's node path rules do to a path whose
// fill and stroke roughjs set as attributes (CSS wins over them).
func (s *sheet) nodePathCSS(p paint) paint {
	p.Fill, p.Stroke, p.Width = s.nodeFill, s.nodeStroke, s.nodeWidth
	return p
}

// resolve is a color value: a theme variable's name or a CSS color.
func (s *sheet) resolve(v string) scene.Color { return style.Color(v) }

// markerColors is a marker's fill and stroke as the stylesheet gives them:
// the class diagram's hollow markers are transparent and its lollipop is
// filled with the node background.
func (s *sheet) markerColors(name string) (fill, stroke scene.Color) {
	fill, stroke = s.markerFill, s.markerStroke
	if s.kind == "class" {
		switch name {
		case "extension", "aggregation":
			fill = scene.None
		case "lollipop":
			fill = s.nodeFill
		}
	}
	return fill, stroke
}

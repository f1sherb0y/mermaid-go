package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/text"
)

// Options tune a render beyond what the diagram's context says.
type Options struct {
	// ELK runs the ELK layout on a graph mermaid built (ELK JSON); nil uses
	// LayoutELK.
	ELK func(g *ElkNode) error
	// Padding around the drawing (the diagram's diagramPadding); 0 for 8.
	Padding float64
	// TitleTopMargin is how far above the drawing the title's baseline
	// sits (titleTopMargin); 0 for 25.
	TitleTopMargin float64
	// Place lays the diagram out instead of ELK or dagre (see Placed).
	Place func(nodes []*Placed) error
}

// LayoutELK is the ELK engine behind ELK JSON: d2lang/elk-go (see
// elkengine.go). Options.ELK overrides it for one render.
var LayoutELK func(g *ElkNode) error

// renderer is one render's state.
type renderer struct {
	ctx       *diagram.Context
	fonts     *text.Fonts
	th        diagram.Theme
	themeName string
	cfg       *config.Config
	look      string
	data      *Data
	opts      Options
	sheet     *sheet
	// svgLabels: htmlLabels is off, so labels are SVG text (createText's
	// formatted text: 1.1em lines).
	svgLabels bool

	nodes []*nstate
	byID  map[string]*nstate
	edges []*estate
}

// nstate is a node as the renderer works on it.
type nstate struct {
	*Node
	look string
	// dir is the direction that applies to the node: its own (clusters) or
	// its nearest ancestor's, else the diagram's.
	dir   string
	shape *shape
	// title is a cluster's label.
	title *Label
	// w, h are the measured size, then the laid out one; x, y the centre.
	w, h, x, y float64
	// labelW, labelH: a cluster's measured label (labelData).
	labelW, labelH float64
	// ELK: the box's top left in absolute coordinates, and where ELK put a
	// container before evenGroupFrames moved its frame.
	offX, offY   float64
	elkX, elkY   float64
	elkMoved     bool
	laidOut      bool
	parent       *nstate
	children     []*nstate
	depth        int
	elkLayoutOpt map[string]any
}

// estate is an edge as the renderer works on it.
type estate struct {
	*Edge
	arrowStart, arrowEnd string
	thickness, pattern   string
	style                []string
	labelStyle           []string
	curve                string
	label                *Label
	w, h                 float64 // label size
	x, y                 float64 // label centre
	hasXY                bool
	points               []P
	terminals            map[string]*terminal
	reversed             bool
}

// terminal is an end label (startLeft, startRight, endLeft, endRight).
type terminal struct {
	text   string
	label  *Label
	w, h   float64
	center P
	placed bool
}

// Render lays out and draws a diagram's data as mermaid's unified renderer
// does, with the context's theme, look and layout ("elk" or "dagre").
func Render(d *Data, ctx *diagram.Context, opts Options) (*scene.Scene, error) {
	r := &renderer{
		ctx: ctx, fonts: ctx.Fonts, th: ctx.Theme, cfg: ctx.Config, look: ctx.Look, data: d, opts: opts,
		byID: map[string]*nstate{},
	}
	if r.cfg == nil {
		r.cfg = config.Defaults()
	}
	r.themeName = guessThemeName(ctx.Theme)
	if v, ok := r.cfg.Get("htmlLabels").(bool); ok {
		r.svgLabels = !v
	} else if v, ok := r.cfg.Get("flowchart.htmlLabels").(bool); ok {
		r.svgLabels = !v
	}
	r.sheet = r.newSheet()
	r.prepare()
	r.measure()
	var err error
	switch {
	case opts.Place != nil:
		err = r.layoutPlaced()
	case ctx.Layout == "dagre":
		err = r.layoutDagre()
	default:
		// If the ELK engine fails, dagre lays the diagram out instead.
		if err = r.layoutELK(); err != nil && opts.ELK == nil {
			err = r.layoutDagre()
		}
	}
	if err != nil {
		return nil, err
	}
	return r.paint(), nil
}

// guessThemeName tells the themes apart by what only some define: color
// themes have a palette, dark ones darkMode.
func guessThemeName(th diagram.Theme) string {
	name := "theme"
	if th.Get("darkMode") == "true" {
		name += "-dark"
	}
	if th.Get("borderColorArray") != "" {
		name += "-color"
	}
	return name
}

// prepare makes the node and edge states.
func (r *renderer) prepare() {
	for _, n := range r.data.Nodes {
		look := n.Look
		if look == "" {
			look = r.look
		}
		// A copy: shapes adjust their node as mermaid's do.
		nc := *n
		ns := &nstate{Node: &nc, look: look}
		r.nodes = append(r.nodes, ns)
		r.byID[n.ID] = ns
	}
	for _, ns := range r.nodes {
		if p := r.byID[ns.ParentID]; p != nil && ns.ParentID != "" {
			ns.parent = p
			p.children = append(p.children, ns)
		}
	}
	for _, ns := range r.nodes {
		for p := ns; p != nil; p = p.parent {
			if p.Dir != "" {
				ns.dir = p.Dir
				break
			}
			if p.parent != nil {
				ns.depth++
			}
		}
		if ns.dir == "" {
			ns.dir = r.data.Direction
		}
	}
	for _, e := range r.data.Edges {
		r.edges = append(r.edges, r.edgeData(e))
	}
}

// arrowMap is [start, end] markers per edge type.
var arrowMap = map[string][2]string{
	"arrow_open":          {"none", "none"},
	"arrow_cross":         {"none", "arrow_cross"},
	"double_arrow_cross":  {"arrow_cross", "arrow_cross"},
	"arrow_point":         {"none", "arrow_point"},
	"double_arrow_point":  {"arrow_point", "arrow_point"},
	"arrow_circle":        {"none", "arrow_circle"},
	"double_arrow_circle": {"arrow_circle", "arrow_circle"},
}

// edgeData resolves an edge's markers, stroke and curve (the ELK adapter's
// buildEdgeData; dagre's is the same for what matters here).
func (r *renderer) edgeData(e *Edge) *estate {
	es := &estate{Edge: e}
	am, ok := arrowMap[e.Type]
	if !ok {
		am = arrowMap["arrow_open"]
	}
	es.arrowStart, es.arrowEnd = e.ArrowTypeStart, e.ArrowTypeEnd
	if es.arrowStart == "" {
		es.arrowStart = am[0]
	}
	if es.arrowEnd == "" {
		es.arrowEnd = am[1]
	}
	thickness, pattern := "normal", "solid"
	var st []string
	switch e.Stroke {
	case "dotted":
		pattern = "dotted"
		st = []string{"fill:none", "stroke-width:2px", "stroke-dasharray:3"}
	case "thick":
		thickness = "thick"
		st = []string{"stroke-width: 3.5px", "fill:none"}
	default:
		st = []string{"fill:none"}
	}
	es.thickness, es.pattern = e.Thickness, e.Pattern
	if es.thickness == "" {
		es.thickness = thickness
	}
	if es.pattern == "" {
		es.pattern = pattern
	}
	es.style = e.Style
	if es.style == nil {
		es.style = st
	}
	es.labelStyle = e.LabelStyle
	es.curve = e.Curve
	if es.curve == "" {
		es.curve = r.cfg.Str("flowchart.curve", "basis")
	}
	if e.Look == "" {
		es.Look = r.look
	} else {
		es.Look = e.Look
	}
	return es
}

// measure sizes nodes, cluster labels and edge labels.
func (r *renderer) measure() {
	for _, n := range r.nodes {
		if n.IsGroup {
			r.measureGroupLabel(n)
			continue
		}
		n.shape = r.shapeFn(n)(r, n)
		b := n.shape.bbox.scene()
		n.w, n.h = b.W, b.H
	}
	for _, e := range r.edges {
		r.measureEdgeLabels(e)
	}
}

// shapeFn is the shape a node is drawn with.
func (r *renderer) shapeFn(n *nstate) shapeFn {
	fn := shapes[n.Shape]
	if n.Shape == "rect" {
		if n.Rx != 0 && n.Ry != 0 {
			fn = shapes["roundedRect"]
		} else {
			fn = shapes["squareRect"]
		}
	}
	if fn == nil {
		fn = shapes["squareRect"]
	}
	return fn
}

// measureGroupLabel measures a cluster's title as insertCluster paints it:
// unwrapped (ELK), or at the wrapping width (dagre, markdown).
func (r *renderer) measureGroupLabel(n *nstate) {
	if n.Label == "" {
		return
	}
	_, lcss := nodeStyles(n.Node)
	f := r.sheet.font.withCSS(lcss...)
	width := math.Inf(1)
	if r.ctx.Layout == "dagre" || n.LabelType == "markdown" {
		width = n.Width
		if width == 0 {
			width = n.WrappingWidth
		}
		if width == 0 {
			width = r.cfg.Num("flowchart.wrappingWidth", 200)
		}
	}
	l := r.layoutLabel(n.Label, f, labelOpts{Width: width, Markdown: n.LabelType == "markdown"})
	n.labelW, n.labelH = l.W, l.H
}

// edgeFont is an edge label's font: the node font with the edge's label
// style.
func (r *renderer) edgeFont(e *estate) font {
	f := r.sheet.font
	f.Color = r.sheet.edgeText
	return f.withCSS(edgeLabelCSS(e)...)
}

func edgeLabelCSS(e *estate) []string {
	var out []string
	for _, s := range e.CSSCompiledStyles {
		k, _, _ := strings.Cut(s, ":")
		if isLabelStyle(strings.TrimSpace(k)) {
			out = append(out, s)
		}
	}
	return append(out, e.labelStyle...)
}

// measureEdgeLabels lays out an edge's label (wrapped at 200px, as
// createText's default) and its end labels.
func (r *renderer) measureEdgeLabels(e *estate) {
	if e.Label != "" {
		e.label = r.layoutLabel(e.Label, r.edgeFont(e), labelOpts{Width: 200, Markdown: e.LabelType == "markdown"})
		e.w, e.h = e.label.W, e.label.H
		if r.svgLabels {
			e.w, e.h = e.w+4, e.h+4 // the background rect's padding
		}
	}
	for _, t := range []struct{ key, text string }{
		{"startLeft", e.StartLabelLeft}, {"startRight", e.StartLabelRight},
		{"endLeft", e.EndLabelLeft}, {"endRight", e.EndLabelRight},
	} {
		if t.text == "" {
			continue
		}
		l := r.layoutLabel(t.text, r.terminalFont(e), labelOpts{Width: math.Inf(1)})
		if e.terminals == nil {
			e.terminals = map[string]*terminal{}
		}
		e.terminals[t.key] = &terminal{text: t.text, label: l, w: l.W, h: l.H}
	}
}

// terminalFont is an end label's font: the edge label's, at the
// stylesheet's edgeTerminals size.
func (r *renderer) terminalFont(e *estate) font {
	f := r.edgeFont(e)
	if r.sheet.terminalSize != 0 {
		f.Size = r.sheet.terminalSize
	}
	return f
}

// extraNum is a numeric extra field, or 0.
func (n *nstate) extraNum(key string) float64 {
	var f float64
	n.ExtraValue(key, &f)
	return f
}

func parseNum(s string, def float64) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "px")), 64)
	if err != nil {
		return def
	}
	return f
}

// jsNum formats a number as JavaScript's String() does (for the values
// mermaid builds ELK options from).
func jsNum(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

var errNoELK = fmt.Errorf("render: no ELK engine registered (render.LayoutELK)")

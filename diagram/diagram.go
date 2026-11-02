// Package diagram is what the diagram types share: the registry, detection
// in mermaid's order, and the context a diagram is drawn with.
package diagram

import (
	"fmt"
	"regexp"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/text"
)

// Context is what a diagram is drawn with.
type Context struct {
	// Config is mermaid's config with the diagram's own settings merged in.
	Config *config.Config
	// Theme is the resolved theme's variables, by mermaid's names.
	Theme Theme
	Fonts *text.Fonts
	// Title is the front matter's title.
	Title string
	// Look ("classic", "neo", "handDrawn") and Layout ("elk", "dagre", ...)
	// as resolved for the diagram's type.
	Look, Layout string
	// ID is the drawing's id, which some diagrams seed their randomness
	// with (mermaid uses the SVG element's id).
	ID string
}

// Theme looks theme variables up by mermaid's names ("primaryColor",
// nested ones as "radar.axisColor",
// "nodeBorder", "fontFamily", ...); "" when unset.
type Theme interface {
	Get(name string) string
}

// Type is a diagram type.
type Type struct {
	// ID is mermaid's id ("flowchart-v2", "sequence", ...).
	ID string
	// Section is its config key ("flowchart", "sequence", ...).
	Section string
	// Detect reports a source of this type (front matter and directives
	// already taken out).
	Detect func(code string) bool
	// Render parses and draws it.
	Render func(code string, ctx *Context) (*scene.Scene, error)
}

// types are registered in the order mermaid tries its detectors.
var types []*Type

// Register adds a type's renderer. The type must be one of mermaid's (see
// order); its place in detection follows mermaid's.
func Register(id string, render func(code string, ctx *Context) (*scene.Scene, error)) {
	for _, t := range order {
		if t.ID == id {
			t.Render = render
			return
		}
	}
	panic("diagram: unknown type " + id)
}

// Detect finds a source's type.
func Detect(code string) (*Type, error) {
	for _, t := range order {
		if t.Detect(code) {
			if t.Render == nil {
				return t, fmt.Errorf("diagram type %s is not supported yet", t.ID)
			}
			return t, nil
		}
	}
	return nil, fmt.Errorf("no diagram type detected")
}

func re(expr string) func(string) bool { return regexp.MustCompile(expr).MatchString }

// order is mermaid 12.2.0's detection order (diagram-orchestration.ts) and
// detectors. zenuml is an external diagram, not in mermaid's core.
var order = []*Type{
	{ID: "flowchart-elk", Section: "flowchart", Detect: re(`^\s*flowchart-elk`)},
	{ID: "mindmap", Section: "mindmap", Detect: re(`^\s*mindmap`)},
	{ID: "architecture", Section: "architecture", Detect: re(`^\s*architecture`)},
	{ID: "agentflow", Section: "agentflow", Detect: re(`^\s*agentflow-beta\b`)},
	{ID: "c4", Section: "c4", Detect: re(`^\s*C4Context|C4Container|C4Component|C4Dynamic|C4Deployment`)},
	{ID: "kanban", Section: "kanban", Detect: re(`^\s*kanban`)},
	{ID: "classDiagram", Section: "class", Detect: re(`^\s*classDiagram`)},
	{ID: "er", Section: "er", Detect: re(`^\s*erDiagram`)},
	{ID: "gantt", Section: "gantt", Detect: re(`^\s*gantt`)},
	{ID: "info", Section: "info", Detect: re(`^\s*info`)},
	{ID: "pie", Section: "pie", Detect: re(`^\s*pie`)},
	{ID: "requirement", Section: "requirement", Detect: re(`^\s*requirement(Diagram)?`)},
	{ID: "sequence", Section: "sequence", Detect: re(`^\s*sequenceDiagram`)},
	{ID: "swimlane", Section: "swimlane", Detect: re(`^\s*swimlane-beta\b`)},
	{ID: "flowchart-v2", Section: "flowchart", Detect: re(`^\s*(graph|flowchart)`)},
	{ID: "timeline", Section: "timeline", Detect: re(`^\s*timeline`)},
	{ID: "gitGraph", Section: "gitGraph", Detect: re(`^\s*gitGraph`)},
	{ID: "stateDiagram", Section: "state", Detect: re(`^\s*stateDiagram`)},
	{ID: "journey", Section: "journey", Detect: re(`^\s*journey`)},
	{ID: "quadrantChart", Section: "quadrantChart", Detect: re(`^\s*quadrantChart`)},
	{ID: "sankey", Section: "sankey", Detect: re(`^\s*sankey(-beta)?`)},
	{ID: "packet", Section: "packet", Detect: re(`^\s*packet(-beta)?`)},
	{ID: "xychart", Section: "xyChart", Detect: re(`^\s*xychart(-beta)?`)},
	{ID: "block", Section: "block", Detect: re(`^\s*block(-beta)?`)},
	{ID: "eventmodeling", Section: "eventmodeling", Detect: re(`^\s*eventmodeling`)},
	{ID: "treeView", Section: "treeView", Detect: re(`^\s*treeView-beta`)},
	{ID: "radar", Section: "radar", Detect: re(`^\s*radar-beta`)},
	{ID: "ishikawa", Section: "ishikawa", Detect: re(`(?i)^\s*ishikawa(-beta)?\b`)},
	{ID: "treemap", Section: "treemap", Detect: re(`^\s*treemap`)},
	{ID: "railroad", Section: "railroad", Detect: re(`(?i)^\s*railroad-beta`)},
	{ID: "railroadEbnf", Section: "railroad", Detect: re(`(?i)^\s*railroad-ebnf-beta`)},
	{ID: "railroadAbnf", Section: "railroad", Detect: re(`(?i)^\s*railroad-abnf-beta`)},
	{ID: "railroadPeg", Section: "railroad", Detect: re(`(?i)^\s*railroad-peg-beta`)},
	{ID: "venn", Section: "venn", Detect: re(`^\s*venn-beta`)},
	{ID: "wardley", Section: "wardley", Detect: re(`(?i)^\s*wardley-beta`)},
	{ID: "cynefin", Section: "cynefin", Detect: re(`^\s*cynefin-beta(?:[\s:]|$)`)},
	{ID: "usecase", Section: "usecase", Detect: re(`^\s*usecase-beta(?:\s|$)`)},
}

// TextBox is a text's box as SVG's getBBox gives it, measured with ctx's
// fonts.
func (ctx *Context) TextBox(t *scene.Text) scene.Rect {
	w := 0.0
	for _, r := range t.Runs {
		w += ctx.Fonts.Width(r.Text, text.Style{Family: t.Family, Size: t.Size, Weight: r.Weight, Italic: r.Italic})
	}
	asc, desc := ctx.Fonts.Extents(text.Style{Family: t.Family, Size: t.Size})
	x := t.X
	switch t.Anchor {
	case scene.Middle:
		x -= w / 2
	case scene.End:
		x -= w
	}
	h := asc + desc
	var y float64
	switch t.Baseline {
	case scene.Alphabetic:
		y = t.Y - asc
	case scene.Hanging:
		y = t.Y
	default: // central, middle: about the middle of the line
		y = t.Y - h/2
	}
	return scene.Rect{X: x, Y: y, W: w, H: h}
}

// Fit is a scene of items whose view box is their box with padding around,
// as mermaid's setupViewPortForSVG.
func (ctx *Context) Fit(items []scene.Item, padding float64) *scene.Scene {
	b, _ := scene.BoundsOf(items, ctx.TextBox)
	return &scene.Scene{ViewBox: scene.Rect{X: b.X - padding, Y: b.Y - padding, W: b.W + 2*padding, H: b.H + 2*padding}, Items: items}
}

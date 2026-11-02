package theme

// The theme and look a diagram type gets when the user sets none (config.schema.yaml): most
// keep the global default, the ones below have their own.
const (
	DefaultName = "default" // the schema's theme
	DefaultLook = "classic" // the schema's look
)

var diagramAppearance = map[string]struct{ theme, look string }{
	"flowchart":   {"redux-color", "neo"},
	"swimlane":    {"redux-color", "neo"},
	"agentflow":   {"redux-color", "neo"},
	"sequence":    {"redux-color", "neo"},
	"class":       {"redux-color", "neo"},
	"state":       {"redux-color", "neo"},
	"er":          {"redux-color", "neo"},
	"requirement": {"redux-color", "neo"},
	"usecase":     {"redux-color", "neo"},
	"venn":        {"redux-color", "neo"},
}

// DiagramDefault returns the default theme and look for a diagram's config key (mermaid's
// getDiagramConfigKey: "flowchart" for flowchart-v2, "class" for classDiagram, "state" for
// stateDiagram, "xyChart" for xychart, ...).
func DiagramDefault(configKey string) (theme, look string) {
	if a, ok := diagramAppearance[configKey]; ok {
		return a.theme, a.look
	}
	return DefaultName, DefaultLook
}

// DefaultFor returns the default theme for a diagram's config key.
func DefaultFor(configKey string) string {
	t, _ := DiagramDefault(configKey)
	return t
}

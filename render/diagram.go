package render

import (
	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
)

// Diagram makes a diagram type's renderer from its parser, for the types
// mermaid draws with its unified renderer (flowchart, class, state, ...).
func Diagram(parse func(code string, cfg *config.Config) (*Data, error)) func(string, *diagram.Context) (*scene.Scene, error) {
	return func(code string, ctx *diagram.Context) (*scene.Scene, error) {
		d, err := parse(code, ctx.Config)
		if err != nil {
			return nil, err
		}
		if d.Title == "" {
			d.Title = ctx.Title
		}
		return Render(d, ctx, Options{})
	}
}

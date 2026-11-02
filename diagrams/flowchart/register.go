package flowchart

import (
	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/render"
	"github.com/f1sherb0y/mermaid-go/scene"
)

func init() {
	draw := render.Diagram(Parse)
	diagram.Register("flowchart-v2", draw)
	// flowchart-elk is a flowchart that always uses ELK.
	diagram.Register("flowchart-elk", func(code string, ctx *diagram.Context) (*scene.Scene, error) {
		c := *ctx
		c.Layout = "elk"
		return draw(code, &c)
	})
}

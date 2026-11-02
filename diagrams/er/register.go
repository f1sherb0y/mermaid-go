package er

import (
	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/render"
)

func init() { diagram.Register("er", render.Diagram(Parse)) }

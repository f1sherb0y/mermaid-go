// Package info is mermaid's info diagram: the version mermaid.live runs.
package info

import (
	"fmt"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

// Version is the mermaid.js version this library follows.
const Version = "12.2.0"

func init() { diagram.Register("info", Render) }

// Render draws `info [showInfo]`.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	for i, l := range strings.Split(code, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || l == "info" || l == "showInfo" || strings.HasPrefix(l, "title") || strings.HasPrefix(l, "acc") {
			continue
		}
		return nil, fmt.Errorf("info: line %d: unexpected %q", i+1, l)
	}
	return &scene.Scene{ViewBox: scene.Rect{W: 400, H: 100}, Items: []scene.Item{&scene.Text{
		X: 100, Y: 40, Anchor: scene.Middle, Family: ctx.Theme.Get("fontFamily"), Size: 32,
		Color: style.Color(ctx.Theme.Get("textColor")), Runs: []scene.Span{{Text: "v" + Version}},
	}}}, nil
}

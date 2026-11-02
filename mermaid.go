// Package mermaid renders Mermaid diagrams to a drawing and to SVG, with the
// layout and look of mermaid.js 12.2.0 as mermaid.live shows them. See
// DESIGN.md.
package mermaid

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/text"
	"github.com/f1sherb0y/mermaid-go/theme"
)

// Options are a render's settings.
type Options struct {
	// Fonts measure and name the text. Required: no font is built in.
	Fonts *text.Fonts
	// Config is what mermaid.initialize would get (theme, themeVariables,
	// flowchart: {...}, ...); a diagram's own front matter and directives
	// override it.
	Config map[string]any
	// ID seeds the diagrams that draw with randomness (mermaid uses the SVG
	// element's id); "mermaid-svg" when empty.
	ID string
}

// Render draws a diagram.
func Render(src string, opts Options) (*scene.Scene, error) {
	if opts.Fonts == nil {
		return nil, fmt.Errorf("mermaid: no fonts")
	}
	doc, err := config.Preprocess(src)
	if err != nil {
		return nil, err
	}
	t, err := diagram.Detect(doc.Code)
	if err != nil {
		return nil, err
	}
	// A directive's "config" is the diagram type's section.
	if c, ok := doc.Overrides["config"].(map[string]any); ok {
		delete(doc.Overrides, "config")
		doc.Overrides[t.Section] = c
	}
	themeName, look, layout := config.Appearance(t.Section, doc.Overrides, opts.Config)
	cfg := config.Defaults().With(opts.Config).With(doc.Overrides)
	overrides := map[string]any{}
	for _, m := range []map[string]any{opts.Config, doc.Overrides} {
		if tv, ok := m["themeVariables"].(map[string]any); ok {
			for k, v := range tv {
				// mermaid's config merge drops nulls before the theme sees them.
				if v != nil {
					overrides[k] = v
				}
			}
		}
		if dm, ok := m["darkMode"]; ok {
			overrides["darkMode"] = dm
		}
	}
	vars, err := theme.Get(themeName, overrides)
	if err != nil {
		return nil, err
	}
	return t.Render(doc.Code, &diagram.Context{
		Config: cfg,
		Theme:  themeVars{vars},
		Fonts:  opts.Fonts,
		Title:  doc.Title,
		Look:   look,
		Layout: layout,
		ID:     cmp.Or(opts.ID, "mermaid-svg"),
	})
}

// themeVars looks a theme's variables up by mermaid's names; nested ones as
// "radar.axisColor".
type themeVars struct{ v *theme.Variables }

func (t themeVars) Get(name string) string { return t.v.Str(strings.Split(name, ".")...) }

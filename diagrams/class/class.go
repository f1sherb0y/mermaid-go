// Package class parses mermaid's class diagram (diagrams/class) into the
// nodes and edges its db.getData() hands the unified renderer: classes as
// classBox nodes with their members, methods and annotations; namespaces
// as clusters; notes; lollipop interfaces; relations with their markers
// and multiplicities.
//
// Not kept: click handlers, links and tooltips (a click still makes the
// class "clickable", as in mermaid), accessibility text, DOM ids.
package class

import (
	"fmt"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/render"
)

// Parse reads a class diagram (from its classDiagram header on) as
// classDiagram.jison and classDb.ts do.
func Parse(code string, cfg *config.Config) (data *render.Data, err error) {
	code = common.EncodeEntities(code)
	toks, err := lexer.Lex(code)
	if err != nil {
		return nil, fmt.Errorf("class: %w", err)
	}
	p := &parser{toks: toks, db: newDB()}
	defer func() {
		if r := recover(); r != nil {
			pe, ok := r.(parseError)
			if !ok {
				panic(r)
			}
			data, err = nil, fmt.Errorf("class: %w", pe.err)
		}
	}()
	p.document()
	return p.db.data(cfg), nil
}

var arrowMarkers = map[int]string{
	none: "none", aggregation: "aggregation", extension: "extension",
	composition: "composition", dependency: "dependency", lollipop: "lollipop",
}

// data is classDb.getData.
func (d *db) data(cfg *config.Config) *render.Data {
	out := &render.Data{DiagramType: "classDiagram", Direction: d.direction}
	hierarchical := cfg.Bool("class.hierarchicalNamespaces", true)
	parentOf := func(id string) string {
		if hierarchical {
			return id
		}
		return d.explicitAncestor(id)
	}
	for _, ns := range d.namespaces {
		if !hierarchical && !ns.explicit {
			continue
		}
		n := &render.Node{ID: ns.id, Label: ns.id, IsGroup: true, Shape: "rect",
			Padding: cfg.Num("class.padding", 16), CSSStyles: []string{}}
		if hierarchical {
			n.Label, n.ParentID = ns.label, ns.parent
		}
		out.Nodes = append(out.Nodes, n)
	}
	for i, c := range d.classes {
		n := &render.Node{ID: c.id, Label: c.label, Shape: "classBox", CSSClasses: c.cssClasses,
			ParentID: parentOf(c.parent), ColorIndex: &i}
		common.SetExtra(n, "text", c.text)
		common.SetExtra(n, "methods", nonNil(c.methods))
		common.SetExtra(n, "members", nonNil(c.members))
		common.SetExtra(n, "annotations", nonNil(c.annotations))
		common.SetExtra(n, "styles", nonNil(c.styles))
		if c.parent != "" {
			common.SetExtra(n, "parent", c.parent)
		}
		out.Nodes = append(out.Nodes, n)
	}
	for _, nt := range d.notes {
		out.Nodes = append(out.Nodes, &render.Node{
			ID: nt.id, Label: nt.text, Shape: "note", Padding: cfg.Num("class.padding", 6),
			CSSStyles: []string{
				"text-align: left",
				"white-space: nowrap",
				"fill: " + cfg.Str("themeVariables.noteBkgColor", "#fff5ad"),
				"stroke: " + cfg.Str("themeVariables.noteBorderColor", "#aaaa33"),
			},
			ParentID:  parentOf(nt.parent),
			LabelType: "markdown",
		})
		if c := d.classByID[nt.class]; c != nil {
			out.Edges = append(out.Edges, &render.Edge{
				ID: fmt.Sprintf("edgeNote%d", nt.index), Start: nt.id, End: c.id,
				Type: "normal", Thickness: "normal", Classes: "relation",
				ArrowTypeStart: "none", ArrowTypeEnd: "none",
				LabelStyle: render.StrList{""}, Style: render.StrList{"fill: none"}, Pattern: "dotted",
			})
		}
	}
	for _, f := range d.interfaces {
		out.Nodes = append(out.Nodes, &render.Node{ID: f.id, Label: f.label, Shape: "rect", CSSStyles: []string{"opacity: 0;"}})
	}
	for i, r := range d.relations {
		e := &render.Edge{
			ID:    common.EdgeID(r.id1, r.id2, i+1, "id"),
			Start: r.id1, End: r.id2, Label: r.title,
			Type: "normal", Thickness: "normal", Classes: "relation",
			ArrowTypeStart: arrowMarkers[r.type1], ArrowTypeEnd: arrowMarkers[r.type2],
			LabelStyle: render.StrList{"display: inline-block"},
			Pattern:    "solid", LabelType: "markdown",
		}
		if r.title1 != "none" {
			e.StartLabelRight = r.title1
		}
		if r.title2 != "none" {
			e.EndLabelLeft = r.title2
		}
		if r.dotted {
			e.Pattern = "dashed"
		}
		out.Edges = append(out.Edges, e)
	}
	return out
}

// nonNil keeps an empty list a list in JSON.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

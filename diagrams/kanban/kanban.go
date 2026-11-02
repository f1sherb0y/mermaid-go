// Package kanban is mermaid's kanban diagram: columns (the outline's first
// level) of cards (the levels below), each card with optional ticket,
// assignee and priority. It is written in mindmap's outline syntax with
// "@{...}" data, and drawn by the unified renderer's kanbanSection and
// kanbanItem.
package kanban

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/diagrams/mindmap"
	"github.com/f1sherb0y/mermaid-go/render"
	"github.com/f1sherb0y/mermaid-go/scene"
)

func init() { diagram.Register("kanban", Render) }

// Section is a column.
type Section struct {
	ID, Label string
	Items     []*Item
}

// Item is a card.
type Item struct {
	ID, Label                  string
	Ticket, Assigned, Priority string
	Icon                       string
}

// Board is a kanban diagram.
type Board struct{ Sections []*Section }

// Parse reads a kanban diagram. The first node's indentation is the
// columns'; deeper nodes are cards of the column before them.
func Parse(code string) (*Board, error) {
	items, err := mindmap.ParseOutline(code, "kanban", true)
	if err != nil {
		return nil, err
	}
	b := &Board{}
	var levels []int
	unnamed := 0
	for _, it := range items {
		id := common.SanitizeText(it.ID)
		if id == "" {
			id = "kbn" + strconv.Itoa(unnamed)
			unnamed++
		}
		card := &Item{ID: id, Label: common.SanitizeText(it.Descr), Icon: it.Icon}
		if it.ShapeData != "" {
			if err := card.setData(it.ShapeData); err != nil {
				return nil, fmt.Errorf("line %d: %w", it.Line, err)
			}
		}
		// getSection: nothing may sit left of the columns.
		for _, l := range levels {
			if l < levels[0] {
				return nil, fmt.Errorf("line %d: items without section detected", it.Line)
			}
		}
		levels = append(levels, it.Indent)
		if len(b.Sections) == 0 || it.Indent == levels[0] {
			b.Sections = append(b.Sections, &Section{ID: card.ID, Label: card.Label})
			continue
		}
		s := b.Sections[len(b.Sections)-1]
		s.Items = append(s.Items, card)
	}
	return b, nil
}

// setData applies a card's @{...} data (YAML, mermaid's JSON schema).
func (c *Item) setData(data string) error {
	if strings.Contains(data, "\n") {
		data += "\n"
	} else {
		data = "{\n" + data + "\n}"
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(data), &doc); err != nil {
		return err
	}
	str := func(k string) string {
		if v, ok := doc[k]; ok && v != nil && v != false && v != "" {
			return fmt.Sprint(v)
		}
		return ""
	}
	if s := str("shape"); s != "" && (s != strings.ToLower(s) || strings.Contains(s, "_")) {
		return fmt.Errorf("no such shape: %s. Shape names should be lowercase", s)
	}
	if s := str("label"); s != "" {
		c.Label = s
	}
	if s := str("icon"); s != "" {
		c.Icon = s
	}
	if s := str("assigned"); s != "" {
		c.Assigned = s
	}
	if s := str("ticket"); s != "" {
		c.Ticket = s
	}
	if s := str("priority"); s != "" {
		c.Priority = s
	}
	return nil
}

// Data is getData: a kanbanSection cluster per column, a kanbanItem per
// card. Node ids are positions (mermaid's ids may repeat).
func Data(b *Board, cfg *config.Config) *render.Data {
	width := cfg.Num("kanban.sectionWidth", 200)
	d := &render.Data{DiagramType: "kanban"}
	for i, s := range b.Sections {
		sid := strconv.Itoa(len(d.Nodes))
		d.Nodes = append(d.Nodes, &render.Node{ID: sid, Label: s.Label, LabelType: "markdown", IsGroup: true,
			Shape: "kanbanSection", Width: width, Rx: 5, Ry: 5, CSSClasses: "section-" + strconv.Itoa(i+1)})
		for _, c := range s.Items {
			n := &render.Node{ID: strconv.Itoa(len(d.Nodes)), ParentID: sid, Label: c.Label, LabelType: "markdown",
				Shape: "kanbanItem", Icon: c.Icon, Rx: 5, Ry: 5, CSSStyles: []string{"text-align: left"},
				Width: width - 15}
			common.SetExtra(n, "ticket", c.Ticket)
			common.SetExtra(n, "assigned", c.Assigned)
			common.SetExtra(n, "priority", c.Priority)
			d.Nodes = append(d.Nodes, n)
		}
	}
	return d
}

// Render draws a kanban diagram: columns side by side, cards stacked under
// the titles, each column as tall as its cards (kanbanRenderer).
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	b, err := Parse(code)
	if err != nil {
		return nil, err
	}
	width := ctx.Config.Num("kanban.sectionWidth", 200)
	const padding = 10.0
	place := func(nodes []*render.Placed) error {
		titleH := 25.0
		for _, n := range nodes {
			if n.IsGroup {
				titleH = max(titleH, n.H)
			}
		}
		top := -width*3/2 + titleH
		col := 0
		var section *render.Placed
		y := top
		closeSection := func() {
			if section != nil {
				h := max(y-top+3*padding, 50) + titleH - 25
				section.W, section.H = width, h
				section.Y = -width*3/2 + h/2
			}
		}
		for _, n := range nodes {
			if n.IsGroup {
				closeSection()
				col++
				section, y = n, top
				n.X = width*float64(col) + float64(col-1)*padding/2
				continue
			}
			n.X, n.Y = section.X, y+n.H/2
			y = n.Y + n.H/2 + padding/2
		}
		closeSection()
		return nil
	}
	return render.Render(Data(b, ctx.Config), ctx, render.Options{Place: place, Padding: ctx.Config.Num("mindmap.padding", 10)})
}

// Package mindmap is mermaid's mindmap: an indented outline drawn as a tree
// around its root, laid out by cose-bilkent (cose.go) and drawn by the
// unified renderer with the mindmap shapes and section colors.
package mindmap

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/render"
	"github.com/f1sherb0y/mermaid-go/scene"
)

func init() { diagram.Register("mindmap", Render) }

// maxSections is MAX_SECTIONS: branches cycle through 11 section colors.
const maxSections = 12

// Node is a mindmap node.
type Node struct {
	// ID is its number in the source's order; NodeID the id it was given.
	ID     int
	NodeID string
	Descr  string
	Type   NodeType
	// Level is its indentation relative to the root's.
	Level int
	// Section is its color section (-1 for the root): its top-level
	// branch's index, or with colorBy depth its depth.
	Section  int
	Icon     string
	Class    string
	Children []*Node
}

// Parse reads a mindmap into its root node.
func Parse(code string) (*Node, error) {
	items, err := ParseOutline(code, "mindmap", false)
	if err != nil {
		return nil, err
	}
	var nodes []*Node
	base := 0
	for i, it := range items {
		n := &Node{ID: i, NodeID: common.SanitizeText(it.ID), Descr: common.SanitizeText(it.Descr),
			Type: it.Type, Icon: it.Icon, Class: it.Class, Section: -1}
		if i == 0 {
			base = it.Indent
			nodes = append(nodes, n)
			continue
		}
		n.Level = it.Indent - base
		// The parent is the last node indented less.
		var parent *Node
		for j := len(nodes) - 1; j >= 0; j-- {
			if nodes[j].Level < n.Level {
				parent = nodes[j]
				break
			}
		}
		if parent == nil {
			return nil, fmt.Errorf("line %d: there can be only one root. No parent could be found for (%q)", it.Line, n.Descr)
		}
		parent.Children = append(parent.Children, n)
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("mindmap: no nodes")
	}
	return nodes[0], nil
}

// assignSections colors the tree: each top-level branch its own section
// (or each depth, byDepth).
func assignSections(n *Node, section, depth int, byDepth bool) {
	n.Section = -1
	if n.Level != 0 {
		n.Section = section
		if byDepth {
			n.Section = (depth - 1) % (maxSections - 1)
		}
	}
	for i, c := range n.Children {
		cs := section
		if n.Level == 0 {
			cs = i % (maxSections - 1)
		}
		assignSections(c, cs, depth+1, byDepth)
	}
}

// Data is what mermaid's getData gives the renderer (and mindmapRenderer
// adjusts): a node per mindmap node with its shape and section classes,
// an edge from each node to each child.
func Data(root *Node, cfg *config.Config) *render.Data {
	assignSections(root, 0, 0, cfg.Str("mindmap.colorBy", "branch") == "depth")
	redux := strings.Contains(strings.ToLower(cfg.Str("theme", "")), "redux")
	d := &render.Data{DiagramType: "mindmap", Direction: "TB"}
	padding := cfg.Num("mindmap.padding", 10)
	maxWidth := cfg.Num("mindmap.maxNodeWidth", 200)
	var walk func(n *Node)
	walk = func(n *Node) {
		classes := []string{"mindmap-node"}
		if n.Level == 0 {
			classes = append(classes, "section-root", "section--1")
		} else {
			classes = append(classes, "section-"+strconv.Itoa(n.Section))
		}
		if n.Class != "" {
			classes = append(classes, n.Class)
		}
		rn := &render.Node{ID: strconv.Itoa(n.ID), Label: n.Descr, LabelType: "markdown",
			CSSClasses: strings.Join(classes, " "), Icon: n.Icon, CenterLabel: true,
			Width: maxWidth, Padding: padding}
		switch n.Type {
		case RoundedRect:
			rn.Shape, rn.Width, rn.Padding = "rounded", 0, 15
		case Rect:
			rn.Shape, rn.Width = "rect", 0
		case Hexagon:
			rn.Shape, rn.Width, rn.Padding = "hexagon", 0, 2*padding
		case Circle:
			rn.Shape = "mindmapCircle"
		case Cloud:
			rn.Shape = "cloud"
		case Bang:
			rn.Shape = "bang"
		default:
			rn.Shape = "defaultMindmapNode"
			if redux {
				rn.Shape, rn.Width, rn.Padding = "rounded", 0, 15
			}
		}
		d.Nodes = append(d.Nodes, rn)
		for _, c := range n.Children {
			classes := "edge section-edge-" + strconv.Itoa(c.Section) + " edge-depth-" + strconv.Itoa(n.Level+1)
			d.Edges = append(d.Edges, &render.Edge{ID: fmt.Sprintf("edge_%d_%d", n.ID, c.ID),
				Start: strconv.Itoa(n.ID), End: strconv.Itoa(c.ID), Type: "normal", Curve: "basis",
				Thickness: "normal", Classes: classes})
			walk(c)
		}
	}
	walk(root)
	return d
}

// Render draws a mindmap.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	root, err := Parse(code)
	if err != nil {
		return nil, err
	}
	d := Data(root, ctx.Config)
	index := map[string]int{}
	for i, n := range d.Nodes {
		index[n.ID] = i
	}
	var edges [][2]int
	for _, e := range d.Edges {
		edges = append(edges, [2]int{index[e.Start], index[e.End]})
	}
	place := func(nodes []*render.Placed) error {
		sizes := make([][2]float64, len(nodes))
		for i, n := range nodes {
			sizes[i] = [2]float64{n.W, n.H}
		}
		for i, c := range coseLayout(sizes, edges) {
			nodes[i].X, nodes[i].Y = c[0], c[1]
		}
		return nil
	}
	return render.Render(d, ctx, render.Options{Place: place, Padding: ctx.Config.Num("mindmap.padding", 10)})
}

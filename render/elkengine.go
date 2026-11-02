package render

import (
	"fmt"

	elkgo "github.com/d2lang/elk-go"
)

// The ELK engine: d2lang/elk-go (ELK 0.12's layered algorithm in Go) behind
// ELK JSON. Its layouts differ a little from mermaid's elkjs 0.9; that is
// accepted.

func init() { LayoutELK = elkGo }

// elkGo lays the graph out with elk-go; a panic in the engine comes back
// as an error.
func elkGo(g *ElkNode) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("render: ELK layout: %v", p)
		}
	}()
	reserveTitles(g)
	in := toElkGo(g)
	if err := elkgo.Layout(in); err != nil {
		return fmt.Errorf("render: ELK layout: %w", err)
	}
	fromElkGo(g, in)
	return nil
}

// toElkGo builds elk-go's graph from ours; elk-go keeps every list's order,
// so the answer is read back by position.
func toElkGo(n *ElkNode) *elkgo.Node {
	out := &elkgo.Node{
		ID: elkgo.StringID(n.ID), LayoutOptions: elkgo.LayoutOptions(n.LayoutOptions),
		X: elkgo.Float64(n.X), Y: elkgo.Float64(n.Y), Width: elkgo.Float64(n.Width), Height: elkgo.Float64(n.Height),
		Labels: toElkGoLabels(n.Labels),
	}
	for _, c := range n.Children {
		out.Children = append(out.Children, toElkGo(c))
	}
	for _, e := range n.Edges {
		oe := &elkgo.Edge{ID: elkgo.StringID(e.ID), LayoutOptions: elkgo.LayoutOptions(e.LayoutOptions), Labels: toElkGoLabels(e.Labels)}
		for _, id := range e.Sources {
			oe.Sources = append(oe.Sources, elkgo.StringID(id))
		}
		for _, id := range e.Targets {
			oe.Targets = append(oe.Targets, elkgo.StringID(id))
		}
		out.Edges = append(out.Edges, oe)
	}
	return out
}

func toElkGoLabels(ls []*ElkLabel) []*elkgo.Label {
	var out []*elkgo.Label
	for _, l := range ls {
		out = append(out, &elkgo.Label{
			Text: elkgo.String(l.Text), LayoutOptions: elkgo.LayoutOptions(l.LayoutOptions),
			X: elkgo.Float64(l.X), Y: elkgo.Float64(l.Y), Width: elkgo.Float64(l.Width), Height: elkgo.Float64(l.Height),
		})
	}
	return out
}

// fromElkGo copies elk-go's answer back into our graph.
func fromElkGo(n *ElkNode, o *elkgo.Node) {
	n.X, n.Y, n.Width, n.Height = elkgo.Number(o.X), elkgo.Number(o.Y), elkgo.Number(o.Width), elkgo.Number(o.Height)
	fromElkGoLabels(n.Labels, o.Labels)
	for i, c := range n.Children {
		fromElkGo(c, o.Children[i])
	}
	for i, e := range n.Edges {
		oe := o.Edges[i]
		e.Sections = nil
		for _, s := range oe.Sections {
			sec := &ElkSection{ID: s.ID.String(), StartPoint: point(s.StartPoint), EndPoint: point(s.EndPoint)}
			for _, b := range s.BendPoints {
				sec.BendPoints = append(sec.BendPoints, ElkPoint{b.X, b.Y})
			}
			e.Sections = append(e.Sections, sec)
		}
		e.Container = ""
		if oe.Container != nil {
			e.Container = oe.Container.String()
		}
		fromElkGoLabels(e.Labels, oe.Labels)
	}
}

func fromElkGoLabels(ls []*ElkLabel, o []*elkgo.Label) {
	for i, l := range ls {
		ol := o[i]
		l.X, l.Y, l.Width, l.Height = elkgo.Number(ol.X), elkgo.Number(ol.Y), elkgo.Number(ol.Width), elkgo.Number(ol.Height)
	}
}

func point(p *elkgo.Point) ElkPoint {
	if p == nil {
		return ElkPoint{}
	}
	return ElkPoint{p.X, p.Y}
}

// reserveTitles makes room for cluster titles at the top of the clusters
// laid out with the standard padding: elkjs 0.9 reserves the label's
// height and the label spacing (5) there, elk-go does not.
func reserveTitles(n *ElkNode) {
	for _, c := range n.Children {
		reserveTitles(c)
	}
	if len(n.Children) == 0 || len(n.Labels) == 0 || n.LayoutOptions == nil {
		return
	}
	std := "[top=" + jsNum(subgraphPadding) + ",left=24,bottom=24,right=24]"
	if n.LayoutOptions["elk.padding"] != std || n.Labels[0].Height <= 0 {
		return
	}
	top := subgraphPadding + n.Labels[0].Height + 5
	n.LayoutOptions["elk.padding"] = "[top=" + jsNum(top) + ",left=24,bottom=24,right=24]"
}

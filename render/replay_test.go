package render

import (
	"encoding/json"
	"fmt"
)

// applyOutput copies a captured ELK answer (ELK JSON) into our graph,
// matching nodes by id and edges by their place in their node's list.
func applyOutput(g *ElkNode, out map[string]any) {
	byID := map[string]map[string]any{}
	var walk func(n map[string]any)
	walk = func(n map[string]any) {
		byID[fmt.Sprint(n["id"])] = n
		cs, _ := n["children"].([]any)
		for _, c := range cs {
			if m, ok := c.(map[string]any); ok {
				walk(m)
			}
		}
	}
	walk(out)
	setLabels := func(ls []*ElkLabel, raw any) {
		ol, _ := raw.([]any)
		for i, l := range ls {
			if i < len(ol) {
				lm, _ := ol[i].(map[string]any)
				l.X, l.Y, l.Width, l.Height = num(lm["x"]), num(lm["y"]), num(lm["width"]), num(lm["height"])
			}
		}
	}
	var set func(n *ElkNode)
	set = func(n *ElkNode) {
		if o := byID[n.ID]; o != nil {
			n.X, n.Y, n.Width, n.Height = num(o["x"]), num(o["y"]), num(o["width"]), num(o["height"])
			setLabels(n.Labels, o["labels"])
			oe, _ := o["edges"].([]any)
			for i, e := range n.Edges {
				if i >= len(oe) {
					break
				}
				em, _ := oe[i].(map[string]any)
				b, _ := json.Marshal(em["sections"])
				e.Sections = nil
				json.Unmarshal(b, &e.Sections)
				e.Container, _ = em["container"].(string)
				setLabels(e.Labels, em["labels"])
			}
		}
		for _, c := range n.Children {
			set(c)
		}
	}
	set(g)
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}

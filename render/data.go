// Package render is mermaid's unified renderer (rendering-util): it lays
// out and draws the nodes, clusters and edges that flowchart, class, state,
// ER, requirement and other diagram types describe as Data, as mermaid
// draws what their db.getData() returns.
package render

import (
	"encoding/json"
	"strings"
)

// Data is what a diagram hands the renderer: mermaid's LayoutData.
type Data struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
	// Direction is the diagram's: "TB", "BT", "LR" or "RL".
	Direction string `json:"direction"`
	// DiagramType is mermaid's id ("flowchart-v2", "classDiagram", ...); it
	// picks the stylesheet and the markers' names.
	DiagramType string `json:"type"`
	// Title is drawn above the diagram (front matter or a title statement).
	Title string `json:"-"`
}

// Node is a node or, with IsGroup, a cluster (subgraph, composite state,
// namespace). Fields are mermaid's names; zero means unset.
type Node struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	ParentID string `json:"parentId"`
	IsGroup  bool   `json:"isGroup"`
	// Shape is mermaid's shape id ("rect", "roundedRect", "diamond",
	// "classBox", "rectWithTitle", ...); clusters: "rect", "roundedWithTitle",
	// "noteGroup", "divider", "kanbanSection".
	Shape string `json:"shape"`
	// Dir is a cluster's own direction ("TB", "LR", ...), if set.
	Dir string `json:"dir"`

	CSSStyles         []string `json:"cssStyles"`
	CSSCompiledStyles []string `json:"cssCompiledStyles"`
	CSSClasses        string   `json:"cssClasses"`
	LabelStyle        string   `json:"labelStyle"`
	LabelType         string   `json:"labelType"` // "markdown", "string", "text"
	Description       StrList  `json:"description"`
	Padding           float64  `json:"padding"`
	Look              string   `json:"look"`
	Rx                float64  `json:"rx"`
	Ry                float64  `json:"ry"`
	// Width and Height, when given, are a fixed size.
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	// MinWidth widens a short label (flowchart.minNodeWidth).
	MinWidth float64 `json:"minWidth"`
	// WrappingWidth is where the label wraps (else flowchart.wrappingWidth).
	WrappingWidth float64 `json:"wrappingWidth"`
	// ColorIndex is a cluster's palette slot in color themes.
	ColorIndex  *int    `json:"colorIndex"`
	Icon        string  `json:"icon"`
	Img         string  `json:"img"`
	Pos         string  `json:"pos"`
	Constraint  string  `json:"constraint"`
	AssetWidth  float64 `json:"assetWidth"`
	AssetHeight float64 `json:"assetHeight"`
	Position    string  `json:"position"` // notes: "left of", "right of"
	CenterLabel bool    `json:"centerLabel"`
	SpreadPorts bool    `json:"spreadPorts"`
	// Type-specific data mermaid hangs on nodes (class members, ER
	// attributes, requirement fields, ...), by mermaid's names.
	Extra map[string]json.RawMessage `json:"-"`
}

// Edge is a link between nodes (or clusters).
type Edge struct {
	ID    string `json:"id"`
	Start string `json:"start"`
	End   string `json:"end"`
	Label string `json:"label"`
	// Type is mermaid's ("arrow_point", "arrow_open", "double_arrow_point",
	// ...); ArrowTypeStart/End name the markers ("arrow_point",
	// "arrow_cross", "arrow_circle", "aggregation", "extension", ...).
	Type           string `json:"type"`
	ArrowTypeStart string `json:"arrowTypeStart"`
	ArrowTypeEnd   string `json:"arrowTypeEnd"`
	// Stroke: "normal", "thick", "dotted", "invisible".
	Stroke            string   `json:"stroke"`
	Thickness         string   `json:"thickness"`
	Pattern           string   `json:"pattern"` // "solid", "dashed", "dotted"
	Style             StrList  `json:"style"`
	LabelStyle        StrList  `json:"labelStyle"`
	CSSCompiledStyles []string `json:"cssCompiledStyles"`
	Classes           string   `json:"classes"`
	LabelType         string   `json:"labelType"`
	// Curve is the interpolation ("basis", "linear", "rounded", "step", ...).
	Curve     string `json:"curve"`
	Minlen    int    `json:"minlen"`
	Length    int    `json:"length"` // extra ranks
	Animate   bool   `json:"animate"`
	Animation string `json:"animation"`
	Look      string `json:"look"`
	// Terminal labels: class diagram multiplicities.
	StartLabelLeft  string `json:"startLabelLeft"`
	StartLabelRight string `json:"startLabelRight"`
	EndLabelLeft    string `json:"endLabelLeft"`
	EndLabelRight   string `json:"endLabelRight"`
	IsUserDefinedID bool   `json:"isUserDefinedId"`
	// IsLayoutOnly edges shape the layout but are not drawn.
	IsLayoutOnly bool                       `json:"isLayoutOnly"`
	Extra        map[string]json.RawMessage `json:"-"`
}

// StrList is a list of strings that mermaid sometimes gives as one string.
type StrList []string

func (l *StrList) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		if s == "" {
			*l = nil
		} else {
			*l = StrList{s}
		}
		return nil
	}
	var a []string
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*l = a
	return nil
}

// UnmarshalJSON reads mermaid's node, keeping the fields Node has no field
// for in Extra.
func (n *Node) UnmarshalJSON(b []byte) error {
	type plain Node
	if err := json.Unmarshal(b, (*plain)(n)); err != nil {
		return err
	}
	n.Extra = extra(b, Node{})
	return nil
}

// UnmarshalJSON reads mermaid's edge, keeping the fields Edge has no field
// for in Extra.
func (e *Edge) UnmarshalJSON(b []byte) error {
	type plain Edge
	if err := json.Unmarshal(b, (*plain)(e)); err != nil {
		return err
	}
	e.Extra = extra(b, Edge{})
	return nil
}

// extra is what of an object's JSON the struct has no tag for.
func extra(b []byte, v any) map[string]json.RawMessage {
	var all map[string]json.RawMessage
	if json.Unmarshal(b, &all) != nil {
		return nil
	}
	for _, k := range jsonNames(v) {
		delete(all, k)
	}
	if len(all) == 0 {
		return nil
	}
	return all
}

var jsonNameCache = map[string][]string{}

func jsonNames(v any) []string {
	switch v.(type) {
	case Node:
		if n, ok := jsonNameCache["node"]; ok {
			return n
		}
	case Edge:
		if n, ok := jsonNameCache["edge"]; ok {
			return n
		}
	}
	// Marshalling the zero value lists every tagged field.
	b, _ := json.Marshal(v)
	var m map[string]json.RawMessage
	json.Unmarshal(b, &m)
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	switch v.(type) {
	case Node:
		jsonNameCache["node"] = names
	case Edge:
		jsonNameCache["edge"] = names
	}
	return names
}

// ExtraString is a string-valued extra field, or "".
func (n *Node) ExtraString(key string) string {
	var s string
	json.Unmarshal(n.Extra[key], &s)
	return s
}

// ExtraValue decodes an extra field into v; false when absent or not v's
// shape.
func (n *Node) ExtraValue(key string, v any) bool {
	raw, ok := n.Extra[key]
	return ok && json.Unmarshal(raw, v) == nil
}

// ExtraString is a string-valued extra field, or "".
func (e *Edge) ExtraString(key string) string {
	var s string
	json.Unmarshal(e.Extra[key], &s)
	return s
}

// LoadData reads what mermaid's db.getData() returns, as JSON, wrapped as
// {"type": ..., "data": {...}} (the getdata dumps) or bare.
func LoadData(b []byte) (*Data, error) {
	var wrapped struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &wrapped); err != nil {
		return nil, err
	}
	raw := b
	if len(wrapped.Data) > 0 {
		raw = wrapped.Data
	}
	d := &Data{}
	if err := json.Unmarshal(raw, d); err != nil {
		return nil, err
	}
	if wrapped.Type != "" {
		d.DiagramType = wrapped.Type
	}
	d.Direction = strings.ToUpper(d.Direction)
	return d, nil
}

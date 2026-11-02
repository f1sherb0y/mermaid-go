package requirement

import (
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/render"
)

// node is a requirement or (req false) an element.
type node struct {
	name    string
	req     bool
	typ     string // the requirement type, or the element's type
	id      string // requirementId
	text    string
	risk    string
	verify  string
	docRef  string
	styles  []string
	classes []string
}

type relation struct{ typ, src, dst string }

// db is requirementDb: what the statements build, in order.
type db struct {
	reqs, elems         map[string]*node
	reqOrder, elemOrder []*node
	classes             map[string][]string // class name to styles
	rels                []relation
	direction           string
}

func newDB() *db {
	return &db{reqs: map[string]*node{}, elems: map[string]*node{}, classes: map[string][]string{}, direction: "TB"}
}

// add adds a requirement or element; one already there keeps its fields.
func (d *db) add(n *node) {
	m, order := d.elems, &d.elemOrder
	if n.req {
		m, order = d.reqs, &d.reqOrder
	}
	if m[n.name] == nil {
		n.classes = []string{"default"}
		m[n.name] = n
		*order = append(*order, n)
	}
}

// node is the requirement named, else the element.
func (d *db) node(name string) *node {
	if n := d.reqs[name]; n != nil {
		return n
	}
	return d.elems[name]
}

// setStyle is a style statement. As in mermaid, it stops at the first name
// that is neither a requirement nor an element.
func (d *db) setStyle(ids, styles []string) {
	for _, id := range ids {
		n := d.node(id)
		if n == nil {
			return
		}
		for _, s := range styles {
			n.styles = append(n.styles, strings.Split(s, ",")...)
		}
	}
}

// setClass adds classes to nodes, with the styles the classes have so far.
func (d *db) setClass(ids, classes []string) {
	for _, id := range ids {
		if n := d.node(id); n != nil {
			for _, c := range classes {
				n.classes = append(n.classes, c)
				n.styles = append(n.styles, d.classes[c]...)
			}
		}
	}
}

// defineClass is a classDef: its styles also go to the nodes that already
// have the class.
func (d *db) defineClass(ids, styles []string) {
	var split []string
	for _, s := range styles {
		split = append(split, strings.Split(s, ",")...)
	}
	for _, id := range ids {
		d.classes[id] = append(d.classes[id], styles...)
		for _, order := range [][]*node{d.reqOrder, d.elemOrder} {
			for _, n := range order {
				for _, c := range n.classes {
					if c == id {
						n.styles = append(n.styles, split...)
						break
					}
				}
			}
		}
	}
}

// data is requirementDb.getData: requirements, then elements, then
// relationships.
func (d *db) data() *render.Data {
	out := &render.Data{DiagramType: "requirement", Direction: d.direction, Nodes: []*render.Node{}, Edges: []*render.Edge{}}
	for _, n := range append(append([]*node{}, d.reqOrder...), d.elemOrder...) {
		ci := len(out.Nodes)
		rn := &render.Node{
			ID:         n.name,
			Shape:      "requirementBox",
			CSSStyles:  append([]string{}, n.styles...),
			CSSClasses: strings.Join(n.classes, " "),
			ColorIndex: &ci,
		}
		common.SetExtra(rn, "name", n.name)
		common.SetExtra(rn, "type", n.typ)
		common.SetExtra(rn, "classes", n.classes)
		if n.req {
			common.SetExtra(rn, "requirementId", n.id)
			common.SetExtra(rn, "text", n.text)
			common.SetExtra(rn, "risk", n.risk)
			common.SetExtra(rn, "verifyMethod", n.verify)
		} else {
			common.SetExtra(rn, "docRef", n.docRef)
		}
		out.Nodes = append(out.Nodes, rn)
	}
	for i, r := range d.rels {
		e := &render.Edge{
			ID:        r.src + "-" + r.dst + "-" + strconv.Itoa(i),
			Start:     d.name(r.src),
			End:       d.name(r.dst),
			Label:     "&lt;&lt;" + r.typ + "&gt;&gt;",
			Classes:   "relationshipLine",
			Thickness: "normal",
			Type:      "normal",
			LabelType: "markdown",
		}
		if r.typ == "contains" {
			e.Style = render.StrList{"fill:none", ""}
			e.Pattern = "normal"
			e.ArrowTypeStart = "requirement_contains"
		} else {
			e.Style = render.StrList{"fill:none", "stroke-dasharray: 10,7"}
			e.Pattern = "dashed"
			e.ArrowTypeEnd = "requirement_arrow"
		}
		out.Edges = append(out.Edges, e)
	}
	return out
}

// name is the node's name if there is such a node, else "".
func (d *db) name(id string) string {
	if n := d.node(id); n != nil {
		return n.name
	}
	return ""
}

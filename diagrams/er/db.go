package er

import (
	"fmt"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/render"
)

// attribute is an entity's attribute, as mermaid's Attribute.
type attribute struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Keys    []string `json:"keys"`
	Comment string   `json:"comment"`
}

type entity struct {
	id, alias string
	attrs     []attribute
	classes   string // "default" and the classes given, space-separated
	styles    []string
}

type subgraph struct {
	id, title, dir  string
	members         []string
	classes, styles []string
}

// relSpec is a relationship's cardinalities (cardA at the second entity,
// cardB at the first) and whether it is identifying.
type relSpec struct {
	cardA, cardB string
	identifying  bool
}

type relationship struct {
	a, b, role string // a and b are node ids
	spec       relSpec
}

type class struct{ styles, textStyles []string }

// db is erDb: what the statements build, in order.
type db struct {
	entities  map[string]*entity
	names     []string // entity names in the order added
	classes   map[string]*class
	rels      []relationship
	subgraphs []*subgraph
	byID      map[string]*subgraph
	direction string
}

func newDB() *db {
	return &db{
		entities:  map[string]*entity{},
		classes:   map[string]*class{},
		byID:      map[string]*subgraph{},
		direction: "TB",
	}
}

func (d *db) addEntity(name, alias string) *entity {
	e := d.entities[name]
	if e == nil {
		e = &entity{id: fmt.Sprintf("entity-%s-%d", name, len(d.names)), alias: alias, classes: "default"}
		d.entities[name] = e
		d.names = append(d.names, name)
	} else if e.alias == "" {
		e.alias = alias
	}
	return e
}

func (d *db) addAttributes(name string, attrs []attribute) {
	e := d.addEntity(name, "")
	e.attrs = append(e.attrs, attrs...)
}

// addRelationship links two entities, or subgraphs by their ids.
func (d *db) addRelationship(a, role, b string, spec relSpec) {
	id := func(name string) string {
		if d.byID[name] != nil {
			return name
		}
		return d.addEntity(name, "").id
	}
	ia := id(a)
	d.rels = append(d.rels, relationship{a: ia, b: id(b), role: role, spec: spec})
}

// addStyles is a style statement: on entities and subgraphs that exist.
func (d *db) addStyles(ids, styles []string) {
	for _, id := range ids {
		if e := d.entities[id]; e != nil {
			e.styles = append(e.styles, styles...)
		}
		if sg := d.byID[id]; sg != nil {
			sg.styles = append(sg.styles, styles...)
		}
	}
}

// addClass is a classDef: a style naming a color also goes to the label,
// with fill read as the background.
func (d *db) addClass(ids, styles []string) {
	for _, id := range ids {
		c := d.classes[id]
		if c == nil {
			c = &class{}
			d.classes[id] = c
		}
		for _, s := range styles {
			if strings.Contains(s, "color") {
				c.textStyles = append(c.textStyles, strings.Replace(s, "fill", "bgFill", 1))
			}
			c.styles = append(c.styles, s)
		}
	}
}

func (d *db) setClass(ids, classes []string) {
	for _, id := range ids {
		if e := d.entities[id]; e != nil {
			for _, c := range classes {
				e.classes += " " + c
			}
		}
		if sg := d.byID[id]; sg != nil {
			sg.classes = append(sg.classes, classes...)
		}
	}
}

// addSubgraph adds a subgraph of the names its statements gave, each once,
// leaving out those an earlier subgraph (one nested in it, say) has.
func (d *db) addSubgraph(id, title string, names []string, dir string) string {
	taken := map[string]bool{}
	for _, sg := range d.subgraphs {
		for _, m := range sg.members {
			taken[m] = true
		}
	}
	seen := map[string]bool{}
	var members []string
	for _, n := range names {
		t := strings.TrimSpace(n)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		if !taken[n] {
			members = append(members, n)
		}
	}
	id = strings.TrimSpace(id)
	sg := &subgraph{id: id, title: strings.TrimSpace(common.SanitizeText(title)), dir: dir, members: members}
	d.subgraphs = append(d.subgraphs, sg)
	d.byID[id] = sg
	return id
}

// compiled is the styles of the classes named, the label's after each
// class's own.
func (d *db) compiled(names []string) []string {
	out := []string{}
	for _, n := range names {
		if c := d.classes[n]; c != nil {
			for _, s := range append(append([]string{}, c.styles...), c.textStyles...) {
				out = append(out, strings.TrimSpace(s))
			}
		}
	}
	return out
}

// data is erDb.getData: subgraphs (the last first), then entities that are
// not subgraphs, then relationships.
func (d *db) data() *render.Data {
	out := &render.Data{DiagramType: "er", Direction: d.direction, Nodes: []*render.Node{}, Edges: []*render.Edge{}}
	parent := map[string]string{}
	for i := len(d.subgraphs) - 1; i >= 0; i-- {
		for _, m := range d.subgraphs[i].members {
			parent[m] = d.subgraphs[i].id
		}
	}
	for i := len(d.subgraphs) - 1; i >= 0; i-- {
		sg := d.subgraphs[i]
		out.Nodes = append(out.Nodes, &render.Node{
			ID:                sg.id,
			Label:             sg.title,
			LabelType:         "markdown",
			ParentID:          parent[sg.id],
			Padding:           8,
			CSSCompiledStyles: d.compiled(sg.classes),
			CSSStyles:         nonNil(sg.styles),
			CSSClasses:        strings.Join(sg.classes, " "),
			Shape:             "rect",
			Dir:               sg.dir,
			IsGroup:           true,
		})
	}
	color := 0
	for _, name := range d.names {
		if d.byID[name] != nil {
			continue
		}
		e := d.entities[name]
		ci := color
		color++
		n := &render.Node{
			ID:                e.id,
			Label:             name,
			Shape:             "erBox",
			CSSClasses:        e.classes,
			CSSStyles:         nonNil(e.styles),
			CSSCompiledStyles: d.compiled(strings.Split(e.classes, " ")),
			LabelType:         "markdown",
			ColorIndex:        &ci,
			ParentID:          parent[name],
		}
		common.SetExtra(n, "attributes", nonNil(e.attrs))
		common.SetExtra(n, "alias", e.alias)
		out.Nodes = append(out.Nodes, n)
	}
	for i, r := range d.rels {
		pattern := "dashed"
		if r.spec.identifying {
			pattern = "solid"
		}
		out.Edges = append(out.Edges, &render.Edge{
			ID:             common.EdgeID(r.a, r.b, i, "id"),
			Type:           "normal",
			Curve:          "basis",
			Start:          r.a,
			End:            r.b,
			Label:          r.role,
			Thickness:      "normal",
			Classes:        "relationshipLine",
			ArrowTypeStart: strings.ToLower(r.spec.cardB),
			ArrowTypeEnd:   strings.ToLower(r.spec.cardA),
			Pattern:        pattern,
			LabelType:      "markdown",
		})
	}
	return out
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

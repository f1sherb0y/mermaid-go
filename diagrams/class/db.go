package class

import (
	"fmt"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
)

// db is what classDb.ts keeps while parsing, without the click handlers,
// links and tooltips.
type db struct {
	classes    []*classNode
	classByID  map[string]*classNode
	relations  []*relation
	notes      []*note
	interfaces []iface
	namespaces []*namespace
	nsByID     map[string]*namespace
	nsStack    []string
	direction  string
}

type classNode struct {
	id, typ, label, text string
	cssClasses           string
	methods              []method
	members              []member
	annotations          []string
	styles               []string
	parent               string
}

type note struct {
	id, class, text, parent string
	index                   int
}

// iface is a lollipop interface: an invisible node the lollipop ends at.
type iface struct{ id, label string }

type namespace struct {
	id, label, parent string
	explicit          bool
}

// relation types, as their markers: none, then classDb's relationType.
const (
	none = iota - 1
	aggregation
	extension
	composition
	dependency
	lollipop
)

type relation struct {
	id1, id2       string
	title1, title2 string // the multiplicities; "none" when not given
	title          string
	type1, type2   int
	dotted         bool
}

func newDB() *db {
	return &db{classByID: map[string]*classNode{}, nsByID: map[string]*namespace{}, direction: "TB"}
}

func sanitize(s string) string { return common.SanitizeText(s) }

// splitName splits "Name~T~" into the class's id and generic type.
func splitName(id string) (name, typ string) {
	id = sanitize(id)
	if strings.Index(id, "~") > 0 {
		parts := strings.Split(id, "~")
		return sanitize(parts[0]), sanitize(parts[1])
	}
	return id, ""
}

func (d *db) addClass(id string) {
	name, typ := splitName(sanitize(id))
	if d.classByID[name] != nil {
		return
	}
	name = sanitize(name)
	text := name
	if typ != "" {
		text += "&lt;" + typ + "&gt;"
	}
	c := &classNode{id: name, typ: typ, label: name, text: text, cssClasses: "default"}
	d.classes = append(d.classes, c)
	d.classByID[name] = c
}

// class is the class named id (generics allowed); it must exist, as
// mermaid fails on a missing one.
func (d *db) class(id string) *classNode {
	name, _ := splitName(id)
	c := d.classByID[name]
	if c == nil {
		panic(parseError{fmt.Errorf("class %q is not defined", name)})
	}
	return c
}

func (d *db) setClassLabel(id, label string) {
	if label != "" {
		label = sanitize(label)
	}
	c := d.class(sanitize(id))
	c.label = label
	c.text = label
	if c.typ != "" {
		c.text += "<" + c.typ + ">"
	}
}

func (d *db) addAnnotation(id, annotation string) {
	c := d.class(id)
	c.annotations = append(c.annotations, annotation)
}

// addMember adds a member line: <<annotation>>, a method (it has a ')'),
// or an attribute.
func (d *db) addMember(id, m string) {
	d.addClass(id)
	c := d.class(id)
	m = strings.TrimSpace(m)
	switch {
	case strings.HasPrefix(m, "<<") && strings.HasSuffix(m, ">>"):
		c.annotations = append(c.annotations, sanitize(m[2:len(m)-2]))
	case strings.Index(m, ")") > 0:
		c.methods = append(c.methods, parseMethod(m))
	case m != "":
		c.members = append(c.members, parseAttribute(m))
	}
}

func (d *db) addMembers(id string, ms []string) {
	for _, m := range ms {
		d.addMember(id, m)
	}
}

func (d *db) addNote(text, class string) string {
	n := &note{id: fmt.Sprintf("note%d", len(d.notes)), class: class, text: text, index: len(d.notes)}
	d.notes = append(d.notes, n)
	return n.id
}

// setCSSClass adds a CSS class to the classes in a comma-separated list.
func (d *db) setCSSClass(ids, class string) {
	for _, id := range strings.Split(ids, ",") {
		if id != "" && id[0] >= '0' && id[0] <= '9' {
			id = "classId-" + id // as mermaid, which then finds no class
		}
		name, _ := splitName(id)
		if c := d.classByID[name]; c != nil {
			c.cssClasses += " " + class
		}
	}
}

// defineClass is classDef: its styles go to the classes that already have
// it (by substring, as mermaid).
func (d *db) defineClass(ids, styles []string) {
	var split []string
	for _, s := range styles {
		split = append(split, strings.Split(s, ",")...)
	}
	for _, id := range ids {
		for _, c := range d.classes {
			if strings.Contains(c.cssClasses, id) {
				c.styles = append(c.styles, split...)
			}
		}
	}
}

func (d *db) setCSSStyle(id string, styles []string) {
	c := d.classByID[id]
	if c == nil {
		return
	}
	for _, s := range styles {
		c.styles = append(c.styles, strings.Split(s, ",")...)
	}
}

func (d *db) addRelation(r *relation) {
	switch {
	case r.type1 == lollipop && r.type2 == none:
		d.addClass(r.id2)
		r.id1 = d.addInterface(r.id1)
	case r.type2 == lollipop && r.type1 == none:
		d.addClass(r.id1)
		r.id2 = d.addInterface(r.id2)
	default:
		d.addClass(r.id1)
		d.addClass(r.id2)
	}
	r.id1, _ = splitName(r.id1)
	r.id2, _ = splitName(r.id2)
	r.title1 = sanitize(strings.TrimSpace(r.title1))
	r.title2 = sanitize(strings.TrimSpace(r.title2))
	d.relations = append(d.relations, r)
}

func (d *db) addInterface(label string) string {
	id := fmt.Sprintf("interface%d", len(d.interfaces))
	d.interfaces = append(d.interfaces, iface{id, label})
	return id
}

// addNamespace opens a namespace, inside the open one if any; a dotted
// name makes its ancestors too. It returns the qualified id.
func (d *db) addNamespace(name, label string) string {
	prefix := "" // the open namespace, which exists with its ancestors
	if n := len(d.nsStack); n > 0 {
		prefix = d.nsStack[n-1]
	}
	id := qualify(prefix, name)
	d.nsStack = append(d.nsStack, id)
	if ns := d.nsByID[id]; ns != nil {
		ns.explicit = true
		if label != "" {
			ns.label = label
		}
		return id
	}
	parts := strings.Split(name, ".")
	for i, part := range parts {
		cur, leaf := qualify(prefix, part), i == len(parts)-1
		if ns := d.nsByID[cur]; ns == nil {
			l := part
			if leaf && label != "" {
				l = label
			}
			ns = &namespace{id: cur, label: l, parent: prefix, explicit: leaf}
			d.namespaces = append(d.namespaces, ns)
			d.nsByID[cur] = ns
		} else if leaf {
			ns.explicit = true
		}
		prefix = cur
	}
	return id
}

func qualify(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func (d *db) closeNamespace() { d.nsStack = d.nsStack[:len(d.nsStack)-1] }

func (d *db) addClassesToNamespace(id string, classes, notes []string) {
	if d.nsByID[id] == nil {
		return
	}
	for _, name := range classes {
		d.class(name).parent = id
	}
	for _, n := range notes {
		for _, nt := range d.notes {
			if nt.id == n {
				nt.parent = id
			}
		}
	}
}

// explicitAncestor is the nearest namespace from id up that was declared,
// not made for a dotted name.
func (d *db) explicitAncestor(id string) string {
	for id != "" {
		ns := d.nsByID[id]
		if ns == nil {
			return ""
		}
		if ns.explicit {
			return id
		}
		id = ns.parent
	}
	return ""
}

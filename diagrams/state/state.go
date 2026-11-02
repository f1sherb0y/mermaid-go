// Package state parses mermaid's state diagram (diagrams/state; both
// stateDiagram and stateDiagram-v2) into the nodes and edges its
// db.getData() hands the unified renderer: states, start and end points,
// forks, joins and choices, composite states as clusters split into
// concurrent regions at their dividers, notes, and transitions.
//
// Not kept: clicks and links, accessibility text, DOM ids.
package state

import (
	"fmt"
	"strings"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/render"
)

// stmt is a parsed statement, as the grammar's actions build them. kind
// "" is one the db ignores.
type stmt struct {
	kind string // "state", "relation", "classDef", "style", "applyClass", "dir"
	id   string
	typ  string // a state's: "default", "fork", "join", "choice", "divider"
	// descr is a state's description; descrList when the grammar made it
	// a list (`state "a" as b:c`).
	descr     []string
	descrList bool
	doc       []*stmt // a composite's statements; non-nil even when empty
	note      *note
	classes   []string
	start     *bool // for [*]: true at the start of a transition
	// s1, s2 and value (the label) for a relation; value for classDef,
	// style, applyClass and dir.
	s1, s2 *stmt
	value  string
}

type note struct{ position, text string }

// Parse reads a state diagram (from its header on) as stateDiagram.jison
// and stateDb.ts do.
func Parse(code string, cfg *config.Config) (data *render.Data, err error) {
	defer func() {
		if r := recover(); r != nil {
			pe, ok := r.(parseError)
			if !ok {
				panic(r)
			}
			data, err = nil, fmt.Errorf("state: %w", pe.err)
		}
	}()
	code = common.EncodeEntities(code)
	toks, err := lexer.Lex(code)
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	p := &parser{toks: toks}
	doc := p.root()
	root := &stmt{kind: "root", id: "root", doc: doc}
	regions := 0
	translate(root, root, true, &regions)
	return newFetcher(root, cfg).data(), nil
}

// translate is stateDb.docTranslator: it names [*] after its document
// (X_start, X_end) and splits a composite's statements at its dividers
// into one divider cluster per region.
func translate(parent, s *stmt, first bool, regions *int) {
	if s.kind == "relation" {
		translate(parent, s.s1, true, regions)
		translate(parent, s.s2, false, regions)
		return
	}
	if s.kind == "state" && s.id == "[*]" {
		suffix := "_end"
		if first {
			suffix = "_start"
		}
		s.id, s.start = parent.id+suffix, &first
	} else if s.kind == "state" {
		s.id = strings.TrimSpace(s.id)
	}
	if s.kind != "root" && s.kind != "state" || s.doc == nil {
		return
	}
	var doc, cur []*stmt
	for _, c := range s.doc {
		if c.typ == "divider" {
			region := *c
			region.doc = append([]*stmt{}, cur...)
			doc, cur = append(doc, &region), nil
		} else {
			cur = append(cur, c)
		}
	}
	// As mermaid, a composite ending in a divider is left unsplit.
	if len(doc) > 0 && len(cur) > 0 {
		*regions++
		doc = append(doc, &stmt{kind: "state", id: fmt.Sprintf("id-region-%d", *regions), typ: "divider", doc: cur})
		s.doc = doc
	}
	for _, c := range s.doc {
		translate(s, c, true, regions)
	}
}

// dir is a document's direction: its last direction statement's.
func dir(s *stmt) string {
	d := "TB"
	for _, c := range s.doc {
		if c.kind == "dir" {
			d = c.value
		}
	}
	return d
}

// styleClass is a classDef's styles.
type styleClass []string

// dbState is what stateDb keeps of a root-level state: its classes and
// styles.
type dbState struct {
	classes, styles []string
}

// nodeInfo is dataFetcher's nodeDb entry: what a state accumulates over
// its mentions.
type nodeInfo struct {
	shape      string
	descr      []string
	descrList  bool
	cssClasses string
	cssStyles  []string
	group      bool
	dir        string
	colorIndex *int
}

// fetcher is stateDb.extract and dataFetcher.
type fetcher struct {
	cfg     *config.Config
	root    *stmt
	states  map[string]*dbState
	classes map[string]styleClass

	info      map[string]*nodeInfo
	nodes     []*render.Node
	byID      map[string]*render.Node
	edges     []*render.Edge
	count     int // graphItemCount
	nextColor int
	colorOf   map[string]*int
}

func newFetcher(root *stmt, cfg *config.Config) *fetcher {
	f := &fetcher{cfg: cfg, root: root, states: map[string]*dbState{}, classes: map[string]styleClass{},
		info: map[string]*nodeInfo{}, byID: map[string]*render.Node{}, colorOf: map[string]*int{}}
	f.extract()
	return f
}

func (f *fetcher) state(id string) *dbState {
	id = strings.TrimSpace(id)
	s := f.states[id]
	if s == nil {
		s = &dbState{}
		f.states[id] = s
	}
	return s
}

// extract reads the root document's statements into the db: states and
// their classes and styles, and classDefs. Those inside composites are
// not read, as in mermaid.
func (f *fetcher) extract() {
	for _, s := range f.root.doc {
		switch s.kind {
		case "state":
			f.state(s.id)
			if s.note != nil {
				s.note.text = common.SanitizeText(s.note.text)
			}
		case "relation":
			for _, e := range []*stmt{s.s1, s.s2} {
				st := f.state(e.id)
				for _, c := range e.classes {
					st.classes = append(st.classes, strings.TrimSpace(c))
				}
			}
		case "classDef":
			c := f.classes[s.id]
			for _, a := range strings.Split(s.value, ",") {
				if i := strings.IndexByte(a, ';'); i >= 0 {
					a = a[:i] + a[i+1:]
				}
				c = append(c, strings.TrimSpace(a))
			}
			f.classes[s.id] = c
		case "style":
			var styles []string
			for _, v := range strings.Split(s.value, ",") {
				styles = append(styles, strings.TrimSpace(strings.ReplaceAll(v, ";", "")))
			}
			for _, id := range strings.Split(s.id, ",") {
				f.state(id).styles = styles
			}
		case "applyClass":
			for _, id := range strings.Split(s.id, ",") {
				st := f.state(id)
				st.classes = append(st.classes, s.value)
			}
		}
	}
}

// data is getData.
func (f *fetcher) data() *render.Data {
	f.fetch(nil, f.root, true)
	wrap := f.cfg.Num("state.wrappingWidth", 120)
	minW := f.cfg.Num("state.minNodeWidth", 120)
	for _, n := range f.nodes {
		n.WrappingWidth = wrap
		if !n.IsGroup {
			n.MinWidth = minW
		}
	}
	return &render.Data{Nodes: f.nodes, Edges: f.edges, Direction: dir(f.root), DiagramType: "stateDiagram"}
}

// fetch is dataFetcher: it adds the item's node (and its note), then what
// its document holds.
func (f *fetcher) fetch(parent, it *stmt, alt bool) {
	if it.id != "root" {
		f.node(parent, it, alt)
	}
	if it.doc == nil {
		return
	}
	for _, s := range it.doc {
		switch s.kind {
		case "state":
			f.fetch(it, s, !alt)
		case "relation":
			f.fetch(it, s.s1, !alt)
			f.fetch(it, s.s2, !alt)
			f.edge(&render.Edge{
				ID: fmt.Sprintf("edge%d", f.count), Start: s.s1.id, End: s.s2.id,
				ArrowTypeEnd: "arrow_barb", Label: common.SanitizeText(s.value),
			}, "normal", "transition")
			f.count++
		}
	}
}

// edge adds a transition or note edge.
func (f *fetcher) edge(e *render.Edge, arrowhead, classes string) {
	e.Style, e.LabelType, e.Thickness, e.Classes = render.StrList{"fill:none"}, "markdown", "normal", classes
	common.SetEdgeExtra(e, "arrowhead", arrowhead)
	f.edges = append(f.edges, e)
}

func (f *fetcher) node(parent, it *stmt, alt bool) {
	id := it.id
	db := f.states[id]
	var classStr string
	var styles []string
	if db != nil {
		classStr, styles = strings.Join(db.classes, " "), db.styles
	}
	userStyled := strings.TrimSpace(classStr) != "" || len(styles) > 0

	n := f.info[id]
	if n == nil {
		shape := "rect"
		switch {
		case it.typ != "default":
			shape = it.typ
		case it.start != nil && *it.start:
			shape = "stateStart"
		case it.start != nil:
			shape = "stateEnd"
		}
		n = &nodeInfo{shape: shape, descr: []string{common.SanitizeText(id)},
			cssClasses: classStr + " statediagram-state", cssStyles: nonNil(styles)}
		f.info[id] = n
	}
	if len(it.descr) > 0 && (it.descrList || it.descr[0] != "") {
		switch {
		case n.descrList:
			n.shape = "rectWithTitle"
			n.descr = append(n.descr, it.descr...)
		case n.descr[0] != "":
			n.shape = "rectWithTitle"
			if n.descr[0] == id {
				n.descr = append([]string{}, it.descr...)
			} else {
				n.descr = append([]string{n.descr[0]}, it.descr...)
			}
			n.descrList = true
		default:
			n.shape = "rect"
			n.descr = append([]string{}, it.descr...)
			n.descrList = it.descrList
		}
		for i, d := range n.descr {
			n.descr[i] = common.SanitizeText(d)
		}
	}
	if n.descrList && len(n.descr) == 1 && n.shape == "rectWithTitle" {
		n.shape = "rect"
		if n.group {
			n.shape = "roundedWithTitle"
		}
	}
	if !n.group && it.doc != nil {
		n.group = true
		n.dir = dir(it)
		n.shape = "roundedWithTitle"
		if it.typ == "divider" {
			n.shape = "divider"
		}
		n.colorIndex = f.colorSlot(n.shape, id, parent, userStyled)
		n.cssClasses += " statediagram-cluster "
		if alt {
			n.cssClasses += "statediagram-cluster-alt"
		}
	}

	nd := &render.Node{
		ID: id, Shape: n.shape, CSSClasses: n.cssClasses, CSSStyles: n.cssStyles,
		Dir: n.dir, IsGroup: n.group, ColorIndex: n.colorIndex,
		Padding: 8, Rx: 10, Ry: 10, LabelType: "markdown", CenterLabel: true,
	}
	if nd.Shape != "divider" {
		f.setLabel(nd, n)
	}
	if n.group {
		common.SetExtra(nd, "type", "group")
	}
	if parent != nil && parent.id != "root" {
		nd.ParentID = parent.id
	}
	if it.note == nil {
		f.insert(nd)
		return
	}

	group := id + "----parent"
	noteNode := &render.Node{
		ID: fmt.Sprintf("%s----note-%d", id, f.count), Shape: "note", Label: it.note.text,
		LabelType: "markdown", CSSClasses: "statediagram-note", CSSStyles: []string{},
		Padding: f.cfg.Num("flowchart.padding", 15), Position: it.note.position, ParentID: group,
	}
	common.SetExtra(noteNode, "type", "node")
	groupNode := &render.Node{
		ID: group, Shape: "noteGroup", Label: it.note.text, CSSClasses: n.cssClasses,
		CSSStyles: []string{}, IsGroup: true, Padding: 16, Position: it.note.position,
	}
	common.SetExtra(groupNode, "type", "group")
	f.count++
	f.insert(groupNode)
	f.insert(noteNode)
	f.insert(nd)
	from, to := id, noteNode.ID
	if it.note.position == "left of" {
		from, to = to, from
	}
	f.edge(&render.Edge{ID: from + "-" + to, Start: from, End: to, Pattern: "dashed"}, "none", "transition note-edge")
}

// setLabel gives the node its description: the first line is the label,
// the rest (for rectWithTitle) the description.
func (f *fetcher) setLabel(nd *render.Node, n *nodeInfo) {
	if !n.descrList {
		nd.Label = n.descr[0]
		return
	}
	if len(n.descr) > 0 {
		nd.Label = n.descr[0]
		nd.Description = append(render.StrList{}, n.descr[1:]...)
	}
	if n.group && len(nd.Description) > 0 {
		panic(parseError{fmt.Errorf("group nodes can only have label; remove the additional description for node [%s]", nd.ID)})
	}
}

// colorSlot is a cluster's palette slot: the next one, but a divider
// shares its composite's; none for a cluster the author styled.
func (f *fetcher) colorSlot(shape, id string, parent *stmt, userStyled bool) *int {
	if shape == "divider" && parent != nil {
		if c, ok := f.colorOf[parent.id]; ok {
			f.colorOf[id] = c
			return c
		}
	}
	slot := f.nextColor
	f.nextColor++
	var c *int
	if !userStyled {
		c = &slot
	}
	f.colorOf[id] = c
	return c
}

// insert adds a node, or updates the one with its id in place (keeping its
// parent if the update has none). Its classes' classDef styles are
// compiled in.
func (f *fetcher) insert(n *render.Node) {
	if n.ID == "" {
		return
	}
	if n.CSSClasses != "" {
		n.CSSCompiledStyles = []string{}
		for _, c := range strings.Split(n.CSSClasses, " ") {
			n.CSSCompiledStyles = append(n.CSSCompiledStyles, f.classes[c]...)
		}
	}
	if old := f.byID[n.ID]; old != nil {
		if n.ParentID == "" {
			n.ParentID = old.ParentID
		}
		*old = *n
		return
	}
	f.byID[n.ID] = n
	f.nodes = append(f.nodes, n)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

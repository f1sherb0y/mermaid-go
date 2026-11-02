// Package flowchart parses mermaid's flowcharts (diagrams/flowchart) into
// the unified renderer's data: what flowDb.getData() returns.
//
// Not kept, as nothing draws them: click, link, callback and tooltip
// statements and accessibility text (parsed and dropped), node domIds,
// [|field:value|] vertex props, and the look (the renderer's context sets
// it).
package flowchart

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/render"
)

// Parse reads a flowchart (code as config.Preprocess leaves it).
func Parse(code string, cfg *config.Config) (*render.Data, error) {
	if cfg == nil {
		cfg = config.Defaults()
	}
	toks, err := lexer.Lex(common.EncodeEntities(code))
	if err != nil {
		return nil, fmt.Errorf("flowchart: %w", err)
	}
	d := &db{cfg: cfg, vertices: map[string]*vertex{}, classes: map[string]*class{}, subLookup: map[string]*subgraph{}}
	p := &parser{toks: toks, db: d}
	if err := p.parse(); err != nil {
		return nil, fmt.Errorf("flowchart: %w", err)
	}
	if d.err != nil {
		return nil, fmt.Errorf("flowchart: %w", d.err)
	}
	return d.data(), nil
}

type vertex struct {
	id, text, labelType, typ string
	styles, classes          []string
	icon, form, pos, img     string
	constraint               string
	assetW, assetH           float64
}

type edge struct {
	id, start, end, typ, stroke string
	text, labelType             string
	length                      int
	userID                      bool
	classes, style              []string
	interpolate, animation      string
	animate                     bool
}

type class struct{ styles, textStyles []string }

type subgraph struct {
	id, title, labelType, dir string
	nodes, classes            []string
	metadata                  map[string]any
}

// db is flowDb's state.
type db struct {
	cfg       *config.Config
	direction string
	vertices  map[string]*vertex
	order     []*vertex
	edges     []*edge
	classes   map[string]*class
	subgraphs []*subgraph
	subLookup map[string]*subgraph
	// defaultStyle and defaultCurve are linkStyle default's.
	defaultStyle []string
	defaultCurve string
	// err is the first error an action raised (mermaid throws from the db).
	err error
}

func (d *db) fail(err error) {
	if d.err == nil {
		d.err = err
	}
}

func (d *db) setDirection(dir string) {
	dir = strings.TrimSpace(dir)
	switch {
	case strings.Contains(dir, "v"), dir == "TD":
		dir = "TB"
	case strings.Contains(dir, ">"):
		dir = "LR"
	case strings.Contains(dir, "^"):
		dir = "BT"
	case strings.Contains(dir, "<"):
		dir = "RL"
	}
	d.direction = dir
}

// linkInfo is a parsed arrow: its type, stroke, length, text and id.
type linkInfo struct {
	typ, stroke string
	length      int
	text        *label
	id          string
}

// destructLink reads an arrow (and the start of one with text in it).
func destructLink(end, start string) *linkInfo {
	info := destructEndLink(end)
	if start == "" {
		return info
	}
	s := strings.TrimSpace(start)
	typ := "arrow_open"
	switch s[0] {
	case '<':
		typ, s = "arrow_point", s[1:]
	case 'x':
		typ, s = "arrow_cross", s[1:]
	case 'o':
		typ, s = "arrow_circle", s[1:]
	}
	stroke := "normal"
	if strings.Contains(s, "=") {
		stroke = "thick"
	}
	if strings.Contains(s, ".") {
		stroke = "dotted"
	}
	if stroke != info.stroke {
		return &linkInfo{typ: "INVALID", stroke: "INVALID", length: info.length}
	}
	if typ == "arrow_open" {
		typ = info.typ
	} else {
		if typ != info.typ {
			return &linkInfo{typ: "INVALID", stroke: "INVALID", length: info.length}
		}
		typ = "double_" + typ
	}
	return &linkInfo{typ: typ, stroke: stroke, length: info.length}
}

func destructEndLink(s string) *linkInfo {
	s = strings.TrimSpace(s)
	line := s[:len(s)-1]
	typ := "arrow_open"
	double := func(t string, c byte) {
		typ = t
		if s[0] == c {
			typ = "double_" + t
			line = line[1:]
		}
	}
	switch s[len(s)-1] {
	case 'x':
		double("arrow_cross", 'x')
	case '>':
		double("arrow_point", '<')
	case 'o':
		double("arrow_circle", 'o')
	}
	l := &linkInfo{typ: typ, stroke: "normal", length: len(line) - 1}
	switch {
	case strings.HasPrefix(line, "="):
		l.stroke = "thick"
	case strings.HasPrefix(line, "~"):
		l.stroke = "invisible"
	}
	if dots := strings.Count(line, "."); dots > 0 {
		l.stroke, l.length = "dotted", dots
	}
	return l
}

// vertex returns the vertex for id, made if new.
func (d *db) vertex(id string) *vertex {
	v := d.vertices[id]
	if v == nil {
		v = &vertex{id: id, labelType: "text", text: id}
		d.vertices[id] = v
		d.order = append(d.order, v)
	}
	return v
}

func (d *db) edgeByID(id string) *edge {
	for _, e := range d.edges {
		if e.id == id {
			return e
		}
	}
	return nil
}

// addVertex is a vertex as written: its text and shape, if given. An id an
// edge already has names that edge, not a vertex.
func (d *db) addVertex(id string, t *label, typ string) {
	if strings.TrimSpace(id) == "" || d.edgeByID(id) != nil {
		return
	}
	v := d.vertex(id)
	if t != nil {
		txt := common.SanitizeText(strings.TrimSpace(t.text))
		v.labelType = t.typ
		if len(txt) >= 2 && strings.HasPrefix(txt, `"`) && strings.HasSuffix(txt, `"`) {
			txt = txt[1 : len(txt)-1]
		}
		v.text = txt
	}
	if typ != "" {
		v.typ = typ
	}
}

func (d *db) addVertexStyles(id string, styles []string) {
	if strings.TrimSpace(id) == "" || d.edgeByID(id) != nil {
		return
	}
	v := d.vertex(id)
	v.styles = append(v.styles, styles...)
}

// addVertexData applies @{...} shape data to a vertex, an edge or a
// subgraph.
func (d *db) addVertexData(id, data string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	src := data + "\n"
	if !strings.Contains(data, "\n") {
		src = "{\n" + data + "\n}"
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		d.fail(fmt.Errorf("shape data of %s: %w", id, err))
		return
	}
	str := func(k string) string {
		if v, ok := doc[k]; ok && v != nil {
			return fmt.Sprint(v)
		}
		return ""
	}
	if sg := d.subLookup[id]; sg != nil && doc != nil {
		if sg.metadata == nil {
			sg.metadata = map[string]any{}
		}
		for k, v := range doc {
			sg.metadata[k] = v
		}
		return
	}
	if e := d.edgeByID(id); e != nil {
		if a, ok := doc["animate"]; ok {
			e.animate = a == true || a == "true"
		}
		if a := str("animation"); a != "" {
			e.animation = a
		}
		if c := str("curve"); c != "" {
			e.interpolate = c
		}
		return
	}
	v := d.vertex(id)
	if shape := str("shape"); shape != "" {
		if shape != strings.ToLower(shape) || strings.Contains(shape, "_") {
			d.fail(fmt.Errorf("No such shape: %s. Shape names should be lowercase.", shape))
		} else if !validShapes[shape] {
			d.fail(fmt.Errorf("No such shape: %s.", shape))
		}
		v.typ = shape
	}
	if l := str("label"); l != "" {
		v.text = l
		switch lt := str("labelType"); lt {
		case "markdown", "string", "text":
			v.labelType = lt
		default:
			v.labelType = "markdown"
		}
	}
	noLabel := strings.TrimSpace(str("label")) == ""
	if icon := str("icon"); icon != "" {
		v.icon = icon
		if noLabel && v.text == id {
			v.text = ""
		}
	}
	if f := str("form"); f != "" {
		v.form = f
	}
	if p := str("pos"); p != "" {
		v.pos = p
	}
	if img := str("img"); img != "" {
		v.img = img
		if noLabel && v.text == id {
			v.text = ""
		}
	}
	if c := str("constraint"); c != "" {
		v.constraint = c
	}
	v.assetW, v.assetH = number(doc["w"]), number(doc["h"])
}

func number(v any) float64 {
	switch v := v.(type) {
	case int:
		return float64(v)
	case float64:
		return v
	case string:
		var f float64
		fmt.Sscan(v, &f)
		return f
	}
	return 0
}

// addLink links every start to every end; a link's id goes to the one from
// the last start to the first end.
func (d *db) addLink(starts, ends []string, l *linkInfo) {
	for i, s := range starts {
		for j, e := range ends {
			id := ""
			if i == len(starts)-1 && j == 0 {
				id = l.id
			}
			d.addSingleLink(s, e, l, id)
		}
	}
}

func (d *db) addSingleLink(start, end string, l *linkInfo, id string) {
	e := &edge{start: start, end: end, typ: l.typ, stroke: l.stroke, labelType: "text", interpolate: d.defaultCurve, length: min(l.length, 10)}
	if l.text != nil {
		e.text = common.SanitizeText(strings.TrimSpace(l.text.text))
		if len(e.text) >= 2 && strings.HasPrefix(e.text, `"`) && strings.HasSuffix(e.text, `"`) {
			e.text = e.text[1 : len(e.text)-1]
		}
		e.labelType = l.text.typ
	}
	if id != "" && d.edgeByID(id) == nil {
		e.id, e.userID = id, true
	} else {
		n := 0
		for _, o := range d.edges {
			if o.start == start && o.end == end {
				n++
			}
		}
		if n > 0 {
			n++
		}
		e.id = common.EdgeID(start, end, n, "L")
	}
	if max := int(d.cfg.Num("maxEdges", 500)); len(d.edges) >= max {
		d.fail(fmt.Errorf("Edge limit exceeded. %d edges found, but the limit is %d.", len(d.edges), max))
		return
	}
	d.edges = append(d.edges, e)
}

func (d *db) updateLinkInterpolate(pos []int, curve string) {
	for _, i := range pos {
		if i < 0 {
			d.defaultCurve = curve
		} else if i < len(d.edges) {
			d.edges[i].interpolate = curve
		} else {
			d.fail(fmt.Errorf("The index %d for linkStyle is out of bounds.", i))
		}
	}
}

// updateLink styles links; a style without a fill gets fill:none.
func (d *db) updateLink(pos []int, style []string) error {
	for _, i := range pos {
		if i < 0 {
			d.defaultStyle = style
			continue
		}
		if i >= len(d.edges) {
			return fmt.Errorf("The index %d for linkStyle is out of bounds. Valid indices for linkStyle are between 0 and %d.", i, len(d.edges)-1)
		}
		st := append([]string{}, style...)
		hasFill := false
		for _, s := range st {
			hasFill = hasFill || strings.HasPrefix(s, "fill")
		}
		if len(st) > 0 && !hasFill {
			st = append(st, "fill:none")
		}
		d.edges[i].style = st
	}
	return nil
}

// addClass defines classes; color styles also style the text.
func (d *db) addClass(ids string, styles []string) {
	joined := strings.ReplaceAll(strings.Join(styles, ","), `\,`, "§§§")
	joined = strings.ReplaceAll(strings.ReplaceAll(joined, ",", ";"), "§§§", ",")
	parts := strings.Split(joined, ";")
	for _, id := range strings.Split(ids, ",") {
		c := d.classes[id]
		if c == nil {
			c = &class{}
			d.classes[id] = c
		}
		for _, s := range parts {
			if strings.Contains(s, "color") {
				c.textStyles = append(c.textStyles, strings.Replace(s, "fill", "bgFill", 1))
			}
			c.styles = append(c.styles, s)
		}
	}
}

// setClass gives vertices, edges and subgraphs a class.
func (d *db) setClass(ids, name string) {
	for _, id := range strings.Split(ids, ",") {
		if v := d.vertices[id]; v != nil {
			v.classes = append(v.classes, name)
		}
		if e := d.edgeByID(id); e != nil {
			e.classes = append(e.classes, name)
		}
		if sg := d.subLookup[id]; sg != nil {
			sg.classes = append(sg.classes, name)
		}
	}
}

// addSubGraph closes a subgraph: its members are the document's nodes and
// subgraphs not already in another subgraph.
func (d *db) addSubGraph(idText, title *label, doc []item) string {
	id := strings.TrimSpace(idText.text)
	if idText == title && strings.ContainsAny(title.text, " \t\n\r\f\v") {
		id = ""
	}
	var nodes []string
	var dir string
	seen := map[string]bool{}
	for _, it := range doc {
		if it.dir != "" {
			dir = it.dir
			continue
		}
		if strings.TrimSpace(it.id) == "" || seen[it.id] {
			continue
		}
		seen[it.id] = true
		nodes = append(nodes, it.id)
	}
	if dir == "" && d.cfg.Bool("flowchart.inheritDir", false) {
		dir = d.direction
	}
	if id == "" {
		id = fmt.Sprintf("subGraph%d", len(d.subgraphs))
	}
	sg := &subgraph{
		id:        id,
		title:     strings.TrimSpace(common.SanitizeText(title.text)),
		dir:       dir,
		labelType: title.typ,
	}
	if sg.labelType != "text" && sg.labelType != "string" {
		sg.labelType = "markdown"
	}
	for _, n := range nodes {
		if n != id && !d.inSubgraph(n) {
			sg.nodes = append(sg.nodes, n)
		}
	}
	if existing := d.subLookup[id]; existing != nil {
		existing.nodes = append(existing.nodes, sg.nodes...)
	} else {
		d.subgraphs = append(d.subgraphs, sg)
		d.subLookup[id] = sg
	}
	return id
}

func (d *db) inSubgraph(id string) bool {
	for _, sg := range d.subgraphs {
		for _, n := range sg.nodes {
			if n == id {
				return true
			}
		}
	}
	return false
}

func (d *db) compiledStyles(classes []string) []string {
	var out []string
	for _, name := range classes {
		if c := d.classes[name]; c != nil {
			for _, s := range c.styles {
				out = append(out, strings.TrimSpace(s))
			}
			for _, s := range c.textStyles {
				out = append(out, strings.TrimSpace(s))
			}
		}
	}
	return out
}

// shape is a vertex's shape id in the renderer.
func (v *vertex) shape() string {
	switch {
	case v.img != "":
		return "imageSquare"
	case v.icon != "":
		switch v.form {
		case "circle":
			return "iconCircle"
		case "square":
			return "iconSquare"
		case "rounded":
			return "iconRounded"
		}
		return "icon"
	}
	switch v.typ {
	case "square", "":
		return "squareRect"
	case "round":
		return "roundedRect"
	}
	return v.typ
}

var edgeArrows = map[string][2]string{
	"arrow_point":         {"none", "arrow_point"},
	"arrow_circle":        {"none", "arrow_circle"},
	"arrow_cross":         {"none", "arrow_cross"},
	"double_arrow_point":  {"arrow_point", "arrow_point"},
	"double_arrow_circle": {"arrow_circle", "arrow_circle"},
	"double_arrow_cross":  {"arrow_cross", "arrow_cross"},
}

// data is getData: subgraphs (innermost-closed last, so outermost first),
// then vertices, then edges, with the members of collapsed subgraphs folded
// into them.
func (d *db) data() *render.Data {
	out := &render.Data{Direction: d.direction, DiagramType: "flowchart-v2"}
	parentOfSub := map[string]string{}
	for _, sg := range d.subgraphs {
		for _, c := range sg.nodes {
			if d.subLookup[c] != nil {
				parentOfSub[c] = sg.id
			}
		}
	}
	// Colour slots follow the source order: a pre-order walk of the
	// containment forest.
	children := map[string][]string{}
	for _, sg := range d.subgraphs {
		if p, ok := parentOfSub[sg.id]; ok {
			children[p] = append(children[p], sg.id)
		}
	}
	colorIndex := map[string]int{}
	var walk func(id string)
	walk = func(id string) {
		colorIndex[id] = len(colorIndex)
		for _, c := range children[id] {
			walk(c)
		}
	}
	for _, sg := range d.subgraphs {
		if _, ok := parentOfSub[sg.id]; !ok {
			walk(sg.id)
		}
	}
	// Collapsed subgraphs hide what is inside them: the outermost collapsed
	// ancestor stands in.
	collapsed := func(id string) bool { return d.subLookup[id].metadata["view"] == "collapsed" }
	outermost := func(id string) string {
		res := ""
		seen := map[string]bool{}
		for cur, ok := id, true; ok && !seen[cur]; cur, ok = parentOfSub[cur] {
			seen[cur] = true
			if collapsed(cur) {
				res = cur
			}
		}
		return res
	}
	standIn := map[string]string{}
	for _, sg := range d.subgraphs {
		a := outermost(sg.id)
		if a == "" {
			continue
		}
		if sg.id != a {
			standIn[sg.id] = a
		}
		for _, c := range sg.nodes {
			if c != a {
				standIn[c] = a
			}
		}
	}
	parent := map[string]string{}
	for i := len(d.subgraphs) - 1; i >= 0; i-- {
		sg := d.subgraphs[i]
		if _, hidden := standIn[sg.id]; hidden {
			continue
		}
		for _, c := range sg.nodes {
			parent[c] = sg.id
		}
	}
	byID := map[string]*render.Node{}
	for i := len(d.subgraphs) - 1; i >= 0; i-- {
		sg := d.subgraphs[i]
		if _, hidden := standIn[sg.id]; hidden {
			continue
		}
		ci := colorIndex[sg.id]
		n := &render.Node{
			ID: sg.id, Label: sg.title, LabelType: sg.labelType, ParentID: parent[sg.id], Padding: 8,
			CSSCompiledStyles: d.compiledStyles(sg.classes), CSSClasses: strings.Join(sg.classes, " "),
			Shape: "rect", Dir: sg.dir, IsGroup: true, ColorIndex: &ci,
		}
		if sg.metadata["view"] == "collapsed" {
			n.Shape, n.IsGroup = "collapsedGroup", false
		} else if sg.metadata != nil {
			common.SetExtra(n, "metadata", sg.metadata)
		}
		out.Nodes = append(out.Nodes, n)
		byID[n.ID] = n
	}
	padding := d.cfg.Num("flowchart.padding", 0)
	if padding == 0 {
		padding = 8
	}
	for _, v := range d.order {
		if _, hidden := standIn[v.id]; hidden {
			continue
		}
		if n := byID[v.id]; n != nil {
			// A vertex named as a subgraph styles it.
			n.CSSStyles = v.styles
			n.CSSCompiledStyles = d.compiledStyles(v.classes)
			n.CSSClasses = strings.Join(v.classes, " ")
			continue
		}
		n := &render.Node{
			ID: v.id, Label: v.text, LabelType: v.labelType, ParentID: parent[v.id], Padding: padding,
			CSSStyles:         v.styles,
			CSSCompiledStyles: d.compiledStyles(append([]string{"default", "node"}, v.classes...)),
			CSSClasses:        "default " + strings.Join(v.classes, " "),
			Icon:              v.icon, Pos: v.pos, Img: v.img, Constraint: v.constraint,
			AssetWidth: v.assetW, AssetHeight: v.assetH,
			Shape: v.shape(), MinWidth: d.cfg.Num("flowchart.minNodeWidth", 0),
		}
		out.Nodes = append(out.Nodes, n)
		byID[n.ID] = n
	}
	for _, e := range d.edges {
		start, end := e.start, e.end
		if a, ok := standIn[start]; ok {
			start = a
		}
		if a, ok := standIn[end]; ok {
			end = a
		}
		if start == end && (standIn[e.start] != "" || standIn[e.end] != "") {
			continue
		}
		styles := append(append([]string{}, d.defaultStyle...), e.style...)
		arrows, ok := edgeArrows[e.typ]
		if !ok {
			arrows = [2]string{"none", "arrow_point"}
		}
		if e.stroke == "invisible" || e.typ == "arrow_open" {
			arrows = [2]string{"none", "none"}
		}
		classes := "edge-thickness-normal edge-pattern-solid flowchart-link"
		if e.stroke == "invisible" {
			classes = ""
		}
		curve := e.interpolate
		if curve == "" {
			curve = d.defaultCurve
		}
		if curve == "" {
			curve = d.cfg.Str("flowchart.curve", "basis")
		}
		re := &render.Edge{
			ID: e.id, IsUserDefinedID: e.userID, Start: start, End: end, Type: e.typ,
			Label: e.text, LabelType: e.labelType, Thickness: e.stroke, Pattern: e.stroke,
			Minlen: e.length, Classes: classes, ArrowTypeStart: arrows[0], ArrowTypeEnd: arrows[1],
			CSSCompiledStyles: d.compiledStyles(e.classes), LabelStyle: styles,
			Animate: e.animate, Animation: e.animation, Curve: curve,
		}
		if len(styles) > 0 {
			re.Style = styles
		}
		out.Edges = append(out.Edges, re)
	}
	return out
}

// validShapes are the shape ids @{shape: ...} may name.
var validShapes = map[string]bool{}

func init() {
	for _, s := range strings.Fields(`anchor bang block_arrow bolt bow-rect bow-tie-rectangle brace brace-l
		brace-r braces browser bucket card choice circ circle classBox cloud collapsedGroup collate com-link
		comment composite console cross-circ crossed-circle curv-trap curved-trapezoid cyl cylinder das
		data-store database datastore db dbl-circ decision defaultMindmapNode delay diam diamond directory
		disk display div-proc div-rect divided-process divided-rectangle doc docs document documents
		double-circle doublecircle erBox event extract f-circ filled-circle flag flip-tri flipped-triangle
		folder fork forkJoin fr-circ fr-rect framed-circle framed-rectangle h-cyl half-rounded-rectangle hex
		hexagon horizontal-cylinder hourglass icon iconCircle iconRounded iconSquare imageSquare in-out
		internal-storage inv-trapezoid inv_trapezoid join junction kanbanItem labelRect lean-l lean-left
		lean-r lean-right lean_left lean_right lightning-bolt lin-cyl lin-doc lin-proc lin-rect
		lined-cylinder lined-document lined-process lined-rectangle loop-limit manual manual-file
		manual-input mindmapCircle notch-pent notch-rect notched-pentagon notched-rectangle note odd out-in
		paper-tape person pill prepare priority proc process processes procs question rect rectWithTitle
		rect_left_inv_arrow rectangle requirementBox rounded roundedRect shaded-process sl-rect
		sloped-rectangle sm-circ small-circle squareRect st-doc st-rect stacked-document stacked-rectangle
		stadium start state stateEnd stateStart stop stored-data subproc subprocess subroutine summary
		tag-doc tag-proc tag-rect tagged-document tagged-process tagged-rectangle terminal text trap-b
		trap-t trapezoid trapezoid-bottom trapezoid-top tri triangle usecaseActor usecaseActorAwesome
		usecaseActorHollow usecaseActorIcon usecaseBusiness usecaseEllipse usecaseJsonTable win-pane
		window-pane`) {
		validShapes[s] = true
	}
}

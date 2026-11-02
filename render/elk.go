package render

import (
	"math"
	"strings"
)

// The ELK adapter (layout-algorithms/elk/render.ts): builds the ELK graph
// mermaid builds, runs ELK, and turns its answer into node boxes and edge
// routes the way mermaid post-processes it.

// ElkNode is a node of an ELK JSON graph; the root is the graph.
type ElkNode struct {
	ID            string         `json:"id"`
	X             float64        `json:"x"`
	Y             float64        `json:"y"`
	Width         float64        `json:"width"`
	Height        float64        `json:"height"`
	Children      []*ElkNode     `json:"children,omitempty"`
	Labels        []*ElkLabel    `json:"labels,omitempty"`
	Edges         []*ElkEdge     `json:"edges,omitempty"`
	LayoutOptions map[string]any `json:"layoutOptions,omitempty"`
}

// ElkLabel is a node or edge label.
type ElkLabel struct {
	Text          string         `json:"text"`
	X             float64        `json:"x"`
	Y             float64        `json:"y"`
	Width         float64        `json:"width"`
	Height        float64        `json:"height"`
	LayoutOptions map[string]any `json:"layoutOptions,omitempty"`
	// terminal is the end label's key, for mermaid's own use.
	terminal string
}

// ElkEdge is an edge; ELK fills in Sections and Container.
type ElkEdge struct {
	ID            string         `json:"id"`
	Sources       []string       `json:"sources"`
	Targets       []string       `json:"targets"`
	Labels        []*ElkLabel    `json:"labels,omitempty"`
	Sections      []*ElkSection  `json:"sections,omitempty"`
	Container     string         `json:"container,omitempty"`
	LayoutOptions map[string]any `json:"layoutOptions,omitempty"`
	// es is the edge it stands for; reversed when laid out backwards.
	es       *estate
	reversed bool
}

// ElkSection is an edge's route.
type ElkSection struct {
	ID         string     `json:"id,omitempty"`
	StartPoint ElkPoint   `json:"startPoint"`
	EndPoint   ElkPoint   `json:"endPoint"`
	BendPoints []ElkPoint `json:"bendPoints,omitempty"`
}

// ElkPoint is a position.
type ElkPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

const (
	portsSurroundingMargin = 12
	subgraphPadding        = 24
	minEndMarkerSegment    = 8
	terminalLabelClearance = 6
)

var portsSurrounding = "[top=12,left=12,bottom=12,right=12]"

// elkPreset is a resolved config.elk.preset.
type elkPreset struct{ layering, placement, containerPlacement, alignment, cycleBreaking string }

var elkPresets = map[string]elkPreset{
	"default":    {"NETWORK_SIMPLEX", "BRANDES_KOEPF", "BRANDES_KOEPF", "BALANCED", "DEPTH_FIRST"},
	"legacy":     {"NETWORK_SIMPLEX", "BRANDES_KOEPF", "BRANDES_KOEPF", "NONE", "GREEDY"},
	"modelOrder": {"NETWORK_SIMPLEX", "NETWORK_SIMPLEX", "BRANDES_KOEPF", "NONE", "GREEDY_MODEL_ORDER"},
	"depthFirst": {"NETWORK_SIMPLEX", "NETWORK_SIMPLEX", "BRANDES_KOEPF", "NONE", "DEPTH_FIRST"},
}

func (r *renderer) elkPreset() elkPreset {
	if p, ok := elkPresets[r.cfg.Str("elk.preset", "default")]; ok {
		return p
	}
	return elkPresets["default"]
}

// elkOpt is a config.elk value as mermaid passes it on: nil when unset.
func (r *renderer) elkOpt(key string) any { return r.cfg.Get("elk." + key) }

func strOr(v any, def string) any {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

func dir2Elk(dir string) string {
	switch dir {
	case "LR":
		return "RIGHT"
	case "RL":
		return "LEFT"
	case "BT":
		return "UP"
	}
	return "DOWN"
}

// rootOptions are createRootElkGraph's layout options.
func (r *renderer) rootOptions() map[string]any {
	p := r.elkPreset()
	o := map[string]any{
		"elk.hierarchyHandling":                              "INCLUDE_CHILDREN",
		"elk.algorithm":                                      "elk.layered",
		"elk.layered.nodePlacement.strategy":                 strOr(r.elkOpt("nodePlacementStrategy"), p.placement),
		"elk.layered.nodePlacement.bk.fixedAlignment":        strOr(r.elkOpt("nodePlacementAlignment"), p.alignment),
		"elk.direction":                                      dir2Elk(r.data.Direction),
		"spacing.baseValue":                                  40.0,
		"elk.layered.unnecessaryBendpoints":                  true,
		"elk.layered.cycleBreaking.strategy":                 strOr(r.elkOpt("cycleBreakingStrategy"), p.cycleBreaking),
		"elk.layered.layering.strategy":                      strOr(r.elkOpt("layeringStrategy"), p.layering),
		"elk.layered.wrapping.multiEdge.improveCuts":         true,
		"elk.layered.wrapping.multiEdge.improveWrappedEdges": true,
		"elk.layered.edgeRouting.selfLoopDistribution":       "EQUALLY",
		"elk.layered.mergeHierarchyEdges":                    true,
		"elk.spacing.portsSurrounding":                       portsSurrounding,
	}
	for k, key := range map[string]string{
		"elk.layered.mergeEdges":                               "mergeEdges",
		"elk.layered.crossingMinimization.forceNodeModelOrder": "forceNodeModelOrder",
		"elk.layered.considerModelOrder.strategy":              "considerModelOrder",
		"elk.layered.layering.coffmanGraham.layerBound":        "layeringLayerBound",
	} {
		if v := r.elkOpt(key); v != nil {
			o[k] = v
		}
	}
	return o
}

// clusterPaintsTitle: note groups and dividers paint no title.
func clusterPaintsTitle(shape string) bool { return shape != "noteGroup" && shape != "divider" }

// groupTitleWidth is the painted title's width plus padding, the frame's
// minimum.
func groupTitleWidth(n *nstate) float64 {
	if !clusterPaintsTitle(n.Shape) {
		return 0
	}
	return n.labelW + n.Padding
}

var containerAlgorithms = map[string]bool{
	"elk.layered": true, "elk.box": true, "elk.rectpacking": true, "elk.stress": true,
	"elk.force": true, "elk.mrtree": true, "elk.radial": true, "elk.sporeOverlap": true,
}

func (n *nstate) containerAlgorithm() string {
	var md map[string]any
	if n.ExtraValue("metadata", &md) {
		if a, ok := md["algorithm"].(string); ok && containerAlgorithms[a] {
			return a
		}
	}
	return ""
}

var rectpackingOptions = map[string]any{
	"spacing.baseValue":      15.0,
	"spacing.nodeNode":       15.0,
	"elk.aspectRatio":        "1.6",
	"elk.expandNodes":        "true",
	"elk.rectpacking.trybox": "true",
	"elk.rectpacking.packing.compaction.rowHeightReevaluation": "true",
	"elk.rectpacking.packing.compaction.iterations":            10.0,
	"elk.rectpacking.whiteSpaceElimination.strategy":           "EQUAL_BETWEEN_STRUCTURES",
	"elk.rectpacking.widthApproximation.strategy":              "SCANLINE",
}

// subgraphOptions are buildSubgraphLayoutOptions'.
func (r *renderer) subgraphOptions(n *nstate) map[string]any {
	p := r.elkPreset()
	o := map[string]any{}
	if clusterPaintsTitle(n.Shape) {
		o["nodeSize.constraints"] = "[MINIMUM_SIZE, NODE_LABELS]"
		o["nodeSize.minimum"] = "(" + jsNum(groupTitleWidth(n)) + ", 0)"
	}
	o["spacing.baseValue"] = 24.0
	o["elk.layered.spacing.edgeNodeBetweenLayers"] = 30.0
	o["elk.spacing.edgeEdge"] = 20.0
	o["spacing.nodeNode"] = 50.0
	o["elk.padding"] = "[top=24,left=24,bottom=24,right=24]"
	o["nodeLabels.placement"] = "[H_CENTER V_TOP, INSIDE]"
	if v := r.elkOpt("mergeEdges"); v != nil {
		o["elk.layered.mergeEdges"] = v
	}
	o["elk.layered.nodePlacement.bk.fixedAlignment"] = strOr(r.elkOpt("nodePlacementAlignment"), p.alignment)
	o["elk.layered.nodePlacement.strategy"] = strOr(r.elkOpt("nodePlacementStrategy"), p.containerPlacement)
	o["elk.layered.cycleBreaking.strategy"] = strOr(r.elkOpt("cycleBreakingStrategy"), p.cycleBreaking)
	o["elk.layered.nodePlacement.networkSimplex.nodeFlexibility"] = "PORT_POSITION"
	o["elk.spacing.portsSurrounding"] = portsSurrounding
	if algo := n.containerAlgorithm(); algo != "" {
		minW := n.labelW + 2*n.Padding
		padTop := n.labelH + 15
		o["nodeSize.constraints"] = "[MINIMUM_SIZE, NODE_LABELS]"
		o["nodeSize.minimum"] = "(" + jsNum(minW) + ", " + jsNum(padTop+15) + ")"
		o["elk.algorithm"] = algo
		o["elk.hierarchyHandling"] = "SEPARATE_CHILDREN"
		o["elk.aspectRatio"] = "2.0"
		o["elk.contentAlignment"] = "H_CENTER V_TOP"
		o["elk.expandNodes"] = "true"
		o["elk.padding"] = "[top=" + jsNum(padTop) + ",left=15,bottom=15,right=15]"
		if algo == "elk.rectpacking" {
			for k, v := range rectpackingOptions {
				o[k] = v
			}
			top := n.labelH + 10
			o["elk.padding"] = "[top=" + jsNum(top) + ",left=10,bottom=10,right=10]"
			o["nodeSize.minimum"] = "(" + jsNum(minW) + ", " + jsNum(top+10) + ")"
		}
	} else if n.Dir != "" {
		o["elk.algorithm"] = "elk.layered"
		o["elk.direction"] = dir2Elk(n.Dir)
		o["elk.hierarchyHandling"] = "SEPARATE_CHILDREN"
	}
	return o
}

// clearContainerAlgorithm undoes a container's own algorithm when edges
// cross its border.
func (n *nstate) clearContainerAlgorithm(o map[string]any) {
	for _, k := range []string{"nodeSize.constraints", "nodeSize.minimum", "elk.algorithm", "elk.aspectRatio",
		"elk.contentAlignment", "elk.expandNodes", "elk.padding"} {
		delete(o, k)
	}
	for k := range rectpackingOptions {
		delete(o, k)
	}
	o["spacing.baseValue"] = 24.0
	o["spacing.nodeNode"] = 50.0
	if clusterPaintsTitle(n.Shape) {
		o["nodeSize.constraints"] = "[MINIMUM_SIZE, NODE_LABELS]"
		o["nodeSize.minimum"] = "(" + jsNum(groupTitleWidth(n)) + ", 0)"
	}
}

// elkState is the graph built and the lookups the post-processing uses.
type elkState struct {
	root   *ElkNode
	byID   map[string]*ElkNode
	edges  []*ElkEdge
	parent map[string]string // tree of the nodes that are in some cluster
}

// buildElkGraph is buildElkGraphFromLayoutData.
func (r *renderer) buildElkGraph() *elkState {
	st := &elkState{byID: map[string]*ElkNode{}, parent: map[string]string{}}
	st.root = &ElkNode{ID: "root", LayoutOptions: r.rootOptions()}
	// addSubGraphs: the tree, from the groups' children.
	hasChildren := map[string]bool{}
	for _, g := range r.nodes {
		if !g.IsGroup {
			continue
		}
		for _, n := range r.nodes {
			if n.ParentID == g.ID {
				st.parent[n.ID] = g.ID
				hasChildren[g.ID] = true
			}
		}
	}
	var addVertices func(into *ElkNode, parent string)
	addVertices = func(into *ElkNode, parent string) {
		for _, n := range r.nodes {
			if n.ParentID != parent {
				continue
			}
			c := &ElkNode{ID: n.ID}
			if !n.IsGroup {
				c.Width, c.Height = n.w, n.h
				if n.SpreadPorts {
					c.LayoutOptions = map[string]any{"elk.portAlignment.default": "CENTER"}
				}
			}
			into.Children = append(into.Children, c)
			st.byID[n.ID] = c
			if n.IsGroup {
				addVertices(c, n.ID)
			}
		}
	}
	addVertices(st.root, "")
	r.elkEdges(st)
	// configureSubgraphNodes
	for _, n := range r.nodes {
		c := st.byID[n.ID]
		if c == nil || !hasChildren[n.ID] {
			continue
		}
		c.Labels = []*ElkLabel{{Text: n.Label, Width: n.labelW, Height: math.Max(0, n.labelH-2)}}
		c.LayoutOptions = r.subgraphOptions(n)
	}
	// configureCrossHierarchyEdges
	for _, e := range st.edges {
		s, t := e.Sources[0], e.Targets[0]
		ns, nt := r.byID[s], r.byID[t]
		if st.byID[s] == nil || st.byID[t] == nil || ns.ParentID == nt.ParentID {
			continue
		}
		anc := st.commonAncestor(s, t)
		r.includeChildren(st, s, anc)
		r.includeChildren(st, t, anc)
	}
	if v, ok := r.elkOpt("orientFeedbackEdges").(bool); !ok || v {
		st.reverseFeedbackEdges()
	}
	if v, _ := r.elkOpt("keepEntryNodeOnTop").(bool); v {
		r.constrainCyclicEntries(st)
	}
	return st
}

func (r *renderer) includeChildren(st *elkState, id, ancestor string) {
	c := st.byID[id]
	n := r.byID[id]
	if c == nil {
		return
	}
	if c.LayoutOptions == nil {
		c.LayoutOptions = map[string]any{}
	}
	if c.LayoutOptions["elk.hierarchyHandling"] == "SEPARATE_CHILDREN" && n.containerAlgorithm() != "" {
		n.clearContainerAlgorithm(c.LayoutOptions)
	}
	c.LayoutOptions["elk.hierarchyHandling"] = "INCLUDE_CHILDREN"
	if id != ancestor && n.ParentID != "" {
		r.includeChildren(st, n.ParentID, ancestor)
	}
}

// commonAncestor is findCommonAncestor: the lowest cluster holding both
// (one of them if it holds the other), "root" if none.
func (st *elkState) commonAncestor(a, b string) string {
	if a == b {
		if p := st.parent[a]; p != "" {
			return p
		}
		return "root"
	}
	seen := map[string]bool{}
	for c := a; c != ""; c = st.parent[c] {
		seen[c] = true
		if c == b {
			return c
		}
	}
	for c := b; c != ""; c = st.parent[c] {
		if seen[c] {
			return c
		}
	}
	return "root"
}

// elkEdges is addEdgesToElkGraph.
func (r *renderer) elkEdges(st *elkState) {
	for _, e := range r.edges {
		ee := &ElkEdge{ID: e.ID, Sources: []string{e.Start}, Targets: []string{e.End}, es: e}
		ee.Labels = append(ee.Labels, &ElkLabel{
			Text: e.Label, Width: e.w, Height: e.h,
			LayoutOptions: map[string]any{"edgeLabels.inline": "true", "edgeLabels.placement": "CENTER"},
		})
		for _, key := range []string{"startLeft", "startRight", "endLeft", "endRight"} {
			t := e.terminals[key]
			if t == nil {
				continue
			}
			atStart := strings.HasPrefix(key, "start")
			marker := e.arrowEnd
			placement := "HEAD"
			if atStart {
				marker, placement = e.arrowStart, "TAIL"
			}
			pad := 0.0
			if marker != "" && marker != "none" {
				pad = terminalLabelClearance
			}
			ee.Labels = append(ee.Labels, &ElkLabel{
				Text: t.text, Width: t.w + 2*pad, Height: t.h + 2*pad, terminal: key,
				LayoutOptions: map[string]any{"edgeLabels.placement": placement},
			})
		}
		st.root.Edges = append(st.root.Edges, ee)
		st.edges = append(st.edges, ee)
	}
}

// childOnPath is the ancestor of id that is a direct child of ancestor.
func (st *elkState) childOnPath(id, ancestor string) string {
	cur := id
	for {
		p := st.parent[cur]
		if p == "" {
			p = "root"
		}
		if p == ancestor {
			return cur
		}
		cur = st.parent[cur]
	}
}

// reverseFeedbackEdges hands ELK the edges that close a cycle only through
// a collapsed subgraph the other way round (subgraphFeedbackEdges.ts).
func (st *elkState) reverseFeedbackEdges() {
	type ledge struct {
		index     int
		from, to  string
		collapsed bool
	}
	flags := make([]bool, len(st.edges))
	levels := map[string][]ledge{}
	var levelOrder []string
	for i, e := range st.edges {
		s, t := e.Sources[0], e.Targets[0]
		if s == t {
			continue
		}
		anc := st.commonAncestor(s, t)
		if anc == s || anc == t {
			continue
		}
		from, to := st.childOnPath(s, anc), st.childOnPath(t, anc)
		if _, ok := levels[anc]; !ok {
			levelOrder = append(levelOrder, anc)
		}
		levels[anc] = append(levels[anc], ledge{i, from, to, from != s || to != t})
	}
	for _, anc := range levelOrder {
		level := levels[anc]
		succ := map[string][]ledge{}
		var nodes []string
		indeg := map[string]int{}
		for _, e := range level {
			for _, n := range []string{e.from, e.to} {
				if _, ok := succ[n]; !ok {
					succ[n] = nil
					nodes = append(nodes, n)
					indeg[n] = 0
				}
			}
			succ[e.from] = append(succ[e.from], e)
			indeg[e.to]++
		}
		const onStack, done = 1, 2
		state := map[string]int{}
		var roots []string
		for _, n := range nodes {
			if indeg[n] == 0 {
				roots = append(roots, n)
			}
		}
		roots = append(roots, nodes...)
		for _, root := range roots {
			if state[root] != 0 {
				continue
			}
			state[root] = onStack
			type frame struct {
				node string
				next int
			}
			stack := []frame{{root, 0}}
			for len(stack) > 0 {
				f := &stack[len(stack)-1]
				out := succ[f.node]
				if f.next >= len(out) {
					state[f.node] = done
					stack = stack[:len(stack)-1]
					continue
				}
				e := out[f.next]
				f.next++
				switch state[e.to] {
				case onStack:
					flags[e.index] = e.collapsed
				case 0:
					state[e.to] = onStack
					stack = append(stack, frame{e.to, 0})
				}
			}
		}
	}
	for i, e := range st.edges {
		if flags[i] {
			e.Sources, e.Targets = e.Targets, e.Sources
			e.reversed = true
		}
	}
}

// constrainCyclicEntries pins each recursive flow's entry to the first layer
// (config.elk.keepEntryNodeOnTop; findCyclicEntryNodes).
func (r *renderer) constrainCyclicEntries(st *elkState) {
	groups := map[string][]string{}
	var order []string
	for _, n := range r.nodes {
		if _, ok := groups[n.ParentID]; !ok {
			order = append(order, n.ParentID)
		}
		groups[n.ParentID] = append(groups[n.ParentID], n.ID)
	}
	for _, g := range order {
		ids := groups[g]
		in := map[string]bool{}
		for _, id := range ids {
			in[id] = true
		}
		indeg := map[string]int{}
		nb := map[string][]string{}
		var internal [][2]string
		for _, e := range r.edges {
			s, t := e.Start, e.End
			if s == "" || t == "" || s == t || !in[s] || !in[t] {
				continue
			}
			indeg[t]++
			nb[s] = append(nb[s], t)
			nb[t] = append(nb[t], s)
			internal = append(internal, [2]string{s, t})
		}
		comp := map[string]int{}
		nc := 0
		for _, id := range ids {
			if _, ok := comp[id]; ok {
				continue
			}
			stack := []string{id}
			comp[id] = nc
			for len(stack) > 0 {
				c := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				for _, x := range nb[c] {
					if _, ok := comp[x]; !ok {
						comp[x] = nc
						stack = append(stack, x)
					}
				}
			}
			nc++
		}
		hasSource := make([]bool, nc)
		for _, id := range ids {
			if indeg[id] == 0 {
				hasSource[comp[id]] = true
			}
		}
		all := true
		for _, h := range hasSource {
			all = all && h
		}
		if all {
			continue
		}
		fwd := map[string][]string{}
		resid := map[string]int{}
		reaches := func(from, to string) bool {
			seen := map[string]bool{from: true}
			stack := []string{from}
			for len(stack) > 0 {
				c := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if c == to {
					return true
				}
				for _, x := range fwd[c] {
					if !seen[x] {
						seen[x] = true
						stack = append(stack, x)
					}
				}
			}
			return false
		}
		for _, e := range internal {
			if reaches(e[1], e[0]) {
				continue
			}
			fwd[e[0]] = append(fwd[e[0]], e[1])
			resid[e[1]]++
		}
		nominated := make([]bool, nc)
		for _, id := range ids {
			c := comp[id]
			if !hasSource[c] && !nominated[c] && resid[id] == 0 {
				nominated[c] = true
				if en := st.byID[id]; en != nil {
					if en.LayoutOptions == nil {
						en.LayoutOptions = map[string]any{}
					}
					en.LayoutOptions["elk.layered.layering.layerConstraint"] = "FIRST"
				}
			}
		}
	}
}

// layoutELK builds the graph, runs ELK and applies its result.
func (r *renderer) layoutELK() error {
	st := r.buildElkGraph()
	run := r.opts.ELK
	if run == nil {
		run = LayoutELK
	}
	if run == nil {
		return errNoELK
	}
	if err := run(st.root); err != nil {
		return err
	}
	r.applyElkPositions(st.root.Children, 0, 0)
	r.evenGroupFrames(st, st.root.Children)
	r.applyElkEdges(st)
	return nil
}

// applyElkPositions turns ELK's relative boxes into absolute centres.
func (r *renderer) applyElkPositions(nodes []*ElkNode, relX, relY float64) {
	for _, c := range nodes {
		n := r.byID[c.ID]
		if n == nil {
			continue
		}
		n.offX, n.offY = c.X+relX, c.Y+relY
		n.x, n.y = n.offX+c.Width/2, n.offY+c.Height/2
		n.w, n.h = c.Width, c.Height
		n.laidOut = true
		if n.IsGroup {
			r.applyElkPositions(c.Children, n.offX, n.offY)
		}
	}
}

// boundsOf is a node's box for clipping: a cluster at least as wide as its
// title.
func (n *nstate) boundsBox() box {
	w := n.w
	if n.IsGroup {
		w = math.Max(n.w, groupTitleWidth(n))
	}
	return box{X: n.offX + n.w/2, Y: n.offY + n.h/2, W: w, H: n.h}
}

func (n *nstate) descendants(into map[string]bool) map[string]bool {
	for _, c := range n.children {
		into[c.ID] = true
		c.descendants(into)
	}
	return into
}

// evenGroupFrames pulls each cluster's frame in to subgraphPadding from its
// contents (and the edges routed inside it) on the left, right and bottom.
func (r *renderer) evenGroupFrames(st *elkState, nodes []*ElkNode) {
	for _, c := range nodes {
		g := r.byID[c.ID]
		if g == nil || !g.IsGroup {
			continue
		}
		r.evenGroupFrames(st, c.Children)
		var boxes []*nstate
		for _, cc := range c.Children {
			if n := r.byID[cc.ID]; n != nil && n.laidOut && n.w != 0 && n.h != 0 {
				boxes = append(boxes, n)
			}
		}
		if !g.laidOut || len(boxes) == 0 {
			continue
		}
		desc := g.descendants(map[string]bool{})
		var lane []P
		for _, e := range st.edges {
			s, t := e.Sources[0], e.Targets[0]
			internal := desc[s] && desc[t]
			atStart, atEnd := s == g.ID, t == g.ID
			if !internal && !atStart && !atEnd {
				continue
			}
			ox, oy := r.calcOffset(st, s, t)
			for _, sec := range e.Sections {
				var pts []ElkPoint
				if internal {
					pts = append(append([]ElkPoint{sec.StartPoint}, sec.BendPoints...), sec.EndPoint)
				} else {
					if atStart {
						pts = append(pts, sec.StartPoint)
					}
					if atEnd {
						pts = append(pts, sec.EndPoint)
					}
				}
				for _, p := range pts {
					lane = append(lane, P{X: p.X + ox, Y: p.Y + oy})
				}
			}
		}
		minX, maxX, maxY := math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, b := range boxes {
			minX, maxX = math.Min(minX, b.offX), math.Max(maxX, b.offX+b.w)
			maxY = math.Max(maxY, b.offY+b.h)
		}
		for _, p := range lane {
			minX, maxX, maxY = math.Min(minX, p.X), math.Max(maxX, p.X), math.Max(maxY, p.Y)
		}
		left := math.Max(g.offX, minX-subgraphPadding)
		right := math.Min(g.offX+g.w, maxX+subgraphPadding)
		bottom := math.Min(g.offY+g.h, maxY+subgraphPadding)
		top := g.offY
		floor := groupTitleWidth(g)
		x, w := left, right-left
		if w < floor {
			x -= (floor - w) / 2
			w = floor
			origRight := g.offX + g.w
			x = math.Max(g.offX, math.Min(x, origRight-w))
			w = math.Min(w, g.w)
		}
		h := bottom - top
		if h <= 0 || w <= 0 {
			continue
		}
		if !g.elkMoved {
			g.elkX, g.elkY, g.elkMoved = g.offX, g.offY, true
		}
		g.offX = x
		g.w, g.h = w, h
		g.x, g.y = x+w/2, top+h/2
	}
}

// calcOffset is the origin of the edge's coordinate system: the absolute
// top left of the lowest cluster holding both ends, where ELK put it.
func (r *renderer) calcOffset(st *elkState, src, dst string) (float64, float64) {
	anc := st.commonAncestor(src, dst)
	if anc == "root" {
		return 0, 0
	}
	n := r.byID[anc]
	if n == nil {
		return 0, 0
	}
	if n.elkMoved {
		return n.elkX, n.elkY
	}
	return n.offX, n.offY
}

// applyElkEdges builds each edge's points from its section as mermaid does:
// the node centres added, clipped back to the outlines, short marker stubs
// dropped, terminal jogs straightened; then labels are placed.
func (r *renderer) applyElkEdges(st *elkState) {
	for _, e := range r.edges {
		e.terminalCentersReset()
	}
	// Small nodes whose anchor ELK parked off-centre move onto the anchor.
	aligned := map[string]bool{}
	for _, ee := range st.edges {
		if len(ee.Sections) == 0 {
			continue
		}
		sn, tn := r.byID[ee.Sources[0]], r.byID[ee.Targets[0]]
		if sn == nil || tn == nil || st.byID[sn.ID] == nil || st.byID[tn.ID] == nil {
			continue
		}
		ox, oy := r.calcOffset(st, ee.es.Start, ee.es.End)
		sec := ee.Sections[0]
		r.alignDegenerate(sn, P{X: sec.StartPoint.X + ox, Y: sec.StartPoint.Y + oy}, aligned)
		r.alignDegenerate(tn, P{X: sec.EndPoint.X + ox, Y: sec.EndPoint.Y + oy}, aligned)
	}
	type ports struct{ start, end P }
	terminalPorts := map[*estate]ports{}
	for _, ee := range st.edges {
		e := ee.es
		startID, endID := ee.Sources[0], ee.Targets[0]
		if ee.reversed {
			startID, endID = endID, startID
		}
		sn, tn := r.byID[startID], r.byID[endID]
		if sn == nil || tn == nil || st.byID[startID] == nil || st.byID[endID] == nil {
			continue
		}
		if len(ee.Sections) == 0 {
			from := P{X: sn.offX + sn.w/2, Y: sn.offY + sn.h/2}
			to := P{X: tn.offX + tn.w/2, Y: tn.offY + tn.h/2}
			sn.x, sn.y, tn.x, tn.y = from.X, from.Y, to.X, to.Y
			pts := r.sanitizeElkPoints([]P{from, to}, sn, tn)
			e.points = pts
			e.curve = "linear"
			e.x, e.y = (pts[0].X+pts[len(pts)-1].X)/2, (pts[0].Y+pts[len(pts)-1].Y)/2
			e.hasXY = true
			continue
		}
		ox, oy := r.calcOffset(st, e.Start, e.End)
		sec := ee.Sections[0]
		pts := []P{{X: sec.StartPoint.X + ox, Y: sec.StartPoint.Y + oy}}
		for _, b := range sec.BendPoints {
			pts = append(pts, P{X: b.X + ox, Y: b.Y + oy})
		}
		pts = append(pts, P{X: sec.EndPoint.X + ox, Y: sec.EndPoint.Y + oy})
		if ee.reversed {
			for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
				pts[i], pts[j] = pts[j], pts[i]
			}
		}
		pp := ports{pts[0], pts[len(pts)-1]}
		sn.x, sn.y = sn.offX+sn.w/2, sn.offY+sn.h/2
		tn.x, tn.y = tn.offX+tn.w/2, tn.offY+tn.h/2
		pts = append([]P{{X: sn.x, Y: sn.y}}, pts...)
		pts = append(pts, P{X: tn.x, Y: tn.y})
		clipped := r.sanitizeElkPoints(pts, sn, tn)
		clipped = ensureEndMarkerSegment(clipped, tn.boundsBox(), markerPathOffset(e.arrowEnd))
		clipped = ensureStartMarkerSegment(clipped, sn.boundsBox(), markerPathOffset(e.arrowStart))
		e.points = clipped
		e.curve = "rounded"
		if l := ee.Labels[0]; l != nil {
			e.x, e.y = l.X+ox+l.Width/2, l.Y+oy+l.Height/2
			e.hasXY = true
		}
		for _, l := range ee.Labels[1:] {
			if l.terminal == "" || (l.X == 0 && l.Y == 0) {
				continue
			}
			if t := e.terminals[l.terminal]; t != nil {
				t.center = P{X: l.X + ox + l.Width/2, Y: l.Y + oy + l.Height/2}
				t.placed = true
				terminalPorts[e] = pp
			}
		}
	}
	if v, ok := r.cfg.Get("elk.straightenEdges").(bool); !ok || v {
		for _, ch := range straightenEdgeTerminals(r.edges) {
			if !ch.edge.hasXY {
				continue
			}
			if p, ok := projectLabelOntoRun(P{X: ch.edge.x, Y: ch.edge.y}, ch.runs); ok {
				ch.edge.x, ch.edge.y = p.X, p.Y
			}
		}
	}
	for e, pp := range terminalPorts {
		followMovedEndpoints(e, pp.start, pp.end)
	}
	r.slideTerminalLabelsOffFrames()
	r.putTerminalLabelsOnTheirSide()
	separateOppositeEdgeLabels(r.edges)
	r.sortForPaint()
}

func (e *estate) terminalCentersReset() {
	for _, t := range e.terminals {
		t.placed = false
	}
}

// alignDegenerate moves a node whose side is too short for ELK's corner
// margin onto the anchor ELK clamped its port to.
func (r *renderer) alignDegenerate(n *nstate, anchor P, aligned map[string]bool) {
	const tol = 0.5
	top, bottom := n.offY, n.offY+n.h
	alongW := math.Abs(anchor.Y-top) <= tol || math.Abs(anchor.Y-bottom) <= tol
	side := n.h
	if alongW {
		side = n.w
	}
	if side >= 2*portsSurroundingMargin || aligned[n.ID] {
		return
	}
	aligned[n.ID] = true
	var d float64
	if alongW {
		d = anchor.X - (n.offX + n.w/2)
	} else {
		d = anchor.Y - (n.offY + n.h/2)
	}
	if math.Abs(d) < 0.01 {
		return
	}
	if alongW {
		n.offX += d
	} else {
		n.offY += d
	}
	n.x, n.y = n.offX+n.w/2, n.offY+n.h/2
}

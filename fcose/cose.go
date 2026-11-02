package fcose

import "math"

// cose-base's graph: graphs (the root and one per compound) own nodes and
// the edges between their own nodes; edges between graphs belong to the
// layout (interEdges). Names follow cose-base.

const (
	maxInt = 2147483647.0 // layout-base's Integer.MAX_VALUE
	minInt = -2147483648.0

	graphMargin       = 15 // DEFAULT_GRAPH_MARGIN
	simpleNodeSize    = 40 // SIMPLE_NODE_SIZE
	emptyCompoundSize = 40 // EMPTY_COMPOUND_NODE_SIZE

	convergenceCheckPeriod = 100
	gridCheckPeriod        = 10
	maxNodeDisplacement    = 100 // MAX_NODE_DISPLACEMENT_INCREMENTAL
	finalTemperature       = 0.04
)

type lnode struct {
	id    string
	index int // in Graph.Nodes
	order int // in allNodes
	// rect
	x, y, w, h float64

	owner, child *lgraph
	edges        []*ledge
	padding      float64
	hasPadding   bool // a compound's padding is its child graph's margin

	noOfChildren       float64
	estimatedSize      float64
	inclusionTreeDepth int
	repulsion          float64

	springX, springY, repulsionX, repulsionY, gravityX, gravityY float64
	dispX, dispY                                                 float64

	startX, finishX, startY, finishY int
	surrounding                      []*lnode
}

type lgraph struct {
	parent                   *lnode
	nodes                    []*lnode
	edges                    []*ledge
	left, right, top, bottom float64
	estimatedSize            float64
	connected                bool
}

type ledge struct {
	source, target           *lnode
	interGraph               bool
	ideal, elasticity        float64
	lca                      *lgraph
	sourceInLca, targetInLca *lnode
	length, lengthX, lengthY float64
	overlapping              bool
}

type layout struct {
	o          Options
	root       *lgraph
	graphs     []*lgraph
	interEdges []*ledge
	nodes      []*lnode // getAllNodes
	edges      []*ledge // getAllEdges
	gravity    []*lnode // nodes to apply gravitation to
	c          *Constraints
	byIndex    []*lnode

	// FDLayoutConstants as fcose sets them for this run.
	defaultEdgeLength, minRepulsionDist float64

	idealEdgeLength                            float64
	coolingFactor, initialCoolingFactor        float64
	maxIterations, totalIterations             int
	coolingCycle                               int
	maxCoolingCycle                            float64
	totalDisplacement, oldTotalDisplacement    float64
	totalDisplacementThreshold, repulsionRange float64

	grid         grid
	surroundMark []int // per node order: the pass it was last added to a surrounding in
	pass         int

	rel *relaxation
}

// build is fcose's processChildrenList, processEdges and the constants.
func build(g *Graph, c *Constraints, o Options) *layout {
	l := &layout{o: o, c: c, byIndex: make([]*lnode, len(g.Nodes))}
	rootParent := &lnode{index: -1}
	l.root = &lgraph{parent: rootParent}
	l.graphs = []*lgraph{l.root}
	kids := make([][]int, len(g.Nodes))
	var top []int
	for i, n := range g.Nodes {
		if n.Parent < 0 {
			top = append(top, i)
		} else {
			kids[n.Parent] = append(kids[n.Parent], i)
		}
	}
	var add func(owner *lgraph, list []int)
	add = func(owner *lgraph, list []int) {
		for _, i := range list {
			n := g.Nodes[i]
			ln := &lnode{id: n.ID, index: i, x: n.X - n.W/2, y: n.Y - n.H/2, w: n.W, h: n.H,
				owner: owner, padding: n.Padding, hasPadding: true, repulsion: o.NodeRepulsion,
				estimatedSize: minInt, inclusionTreeDepth: math.MaxInt32}
			if math.IsNaN(ln.x) {
				ln.x = 0
			}
			if math.IsNaN(ln.y) {
				ln.y = 0
			}
			owner.nodes = append(owner.nodes, ln)
			l.byIndex[i] = ln
			if len(kids[i]) > 0 {
				ln.child = &lgraph{parent: ln}
				l.graphs = append(l.graphs, ln.child)
				add(ln.child, kids[i])
			}
		}
	}
	add(l.root, top)

	total, count := 0.0, 0
	for _, e := range g.Edges {
		s, t := l.byIndex[e.Source], l.byIndex[e.Target]
		if s == nil || t == nil || s == t || between(s, t) {
			continue
		}
		le := &ledge{source: s, target: t, ideal: e.IdealLength, elasticity: e.Elasticity}
		if s.owner == t.owner {
			s.owner.edges = append(s.owner.edges, le)
		} else {
			le.interGraph = true
			l.interEdges = append(l.interEdges, le)
		}
		s.edges = append(s.edges, le)
		t.edges = append(t.edges, le)
		total += le.ideal
		count++
	}
	// The ideal edge length constant is the edges' mean (50 without edges,
	// since mermaid's idealEdgeLength is a function).
	l.defaultEdgeLength = 50
	if count > 0 {
		l.defaultEdgeLength = total / float64(count)
	}
	l.minRepulsionDist = l.defaultEdgeLength / 10.0

	// includesInvalidEdge drops edges between a node and its ancestor.
	kept := l.interEdges[:0]
	for _, e := range l.interEdges {
		if ancestorOf(e.source, e.target) {
			e.source.edges = remove(e.source.edges, e)
			e.target.edges = remove(e.target.edges, e)
			continue
		}
		kept = append(kept, e)
	}
	l.interEdges = kept

	for _, gr := range l.graphs {
		l.nodes = append(l.nodes, gr.nodes...)
	}
	for i, n := range l.nodes {
		n.order = i
	}
	for _, gr := range l.graphs {
		l.edges = append(l.edges, gr.edges...)
	}
	l.edges = append(l.edges, l.interEdges...)
	return l
}

func between(a, b *lnode) bool {
	for _, e := range a.edges {
		if e.source == b || e.target == b {
			return true
		}
	}
	return false
}

func remove(es []*ledge, e *ledge) []*ledge {
	for i, x := range es {
		if x == e {
			return append(es[:i:i], es[i+1:]...)
		}
	}
	return es
}

// ancestorOf is LGraphManager.isOneAncestorOfOther.
func ancestorOf(a, b *lnode) bool {
	if a == b {
		return true
	}
	for p := a.owner.parent; p != nil && p.owner != nil; p = p.owner.parent {
		if p == b {
			return true
		}
	}
	for p := b.owner.parent; p != nil && p.owner != nil; p = p.owner.parent {
		if p == a {
			return true
		}
	}
	return false
}

func (n *lnode) centerX() float64 { return n.x + n.w/2 }
func (n *lnode) centerY() float64 { return n.y + n.h/2 }
func (n *lnode) right() float64   { return n.x + n.w }
func (n *lnode) bottom() float64  { return n.y + n.h }
func (n *lnode) setCenter(cx, cy float64) {
	n.x = cx - n.w/2
	n.y = cy - n.h/2
}

// margin is the graph's: its compound's padding, 15 for the root.
func (g *lgraph) margin() float64 {
	if g.parent.hasPadding {
		return g.parent.padding
	}
	return graphMargin
}

// updateBounds is LGraph.updateBounds(true): compounds take their members'
// bounds plus their padding.
func (g *lgraph) updateBounds() {
	left, right, top, bottom := maxInt, -maxInt, maxInt, -maxInt
	for _, n := range g.nodes {
		if n.child != nil && len(n.child.nodes) > 0 {
			n.child.updateBounds()
			c := n.child
			n.x, n.y = c.left, c.top
			n.w, n.h = c.right-c.left, c.bottom-c.top
		}
		if left > n.x {
			left = n.x
		}
		if r := n.right(); right < r {
			right = r
		}
		if top > n.y {
			top = n.y
		}
		if b := n.bottom(); bottom < b {
			bottom = b
		}
	}
	// RectangleD(left, top, right-left, bottom-top), then its sides.
	w, h := right-left, bottom-top
	m := g.margin()
	g.left = left - m
	g.right = left + w + m
	g.top = top - m
	g.bottom = top + h + m
}

func (n *lnode) calcEstimatedSize() float64 {
	if n.child == nil {
		n.estimatedSize = (n.w + n.h) / 2
		return n.estimatedSize
	}
	n.estimatedSize = n.child.calcEstimatedSize()
	n.w, n.h = n.estimatedSize, n.estimatedSize
	return n.estimatedSize
}

func (g *lgraph) calcEstimatedSize() float64 {
	size := 0.0
	for _, n := range g.nodes {
		size += n.calcEstimatedSize()
	}
	if size == 0 {
		g.estimatedSize = emptyCompoundSize
	} else {
		g.estimatedSize = size / math.Sqrt(float64(len(g.nodes)))
	}
	return g.estimatedSize
}

func (g *lgraph) inclusionTreeDepth(root *lgraph) int {
	if g == root {
		return 1
	}
	return g.parent.inclusionTreeDepth
}

func (n *lnode) noOfChildrenCalc() float64 {
	if n.child == nil {
		return 1
	}
	k := 0.0
	for _, c := range n.child.nodes {
		k += c.noOfChildrenCalc()
	}
	if k == 0 {
		k = 1
	}
	return k
}

// otherEndInGraph is LEdge.getOtherEndInGraph.
func (e *ledge) otherEndInGraph(n *lnode, g, root *lgraph) *lnode {
	o := e.target
	if e.target == n {
		o = e.source
	}
	for {
		if o.owner == g {
			return o
		}
		if o.owner == root {
			return nil
		}
		o = o.owner.parent
	}
}

// updateConnected is LGraph.updateConnected: whether edges join all of the
// graph's nodes (through their members).
func (l *layout) updateConnected(g *lgraph) {
	if len(g.nodes) == 0 {
		g.connected = true
		return
	}
	visited := map[*lnode]bool{}
	var queue []*lnode
	var withChildren func(n *lnode)
	withChildren = func(n *lnode) {
		if !visited[n] {
			visited[n] = true
		}
		queue = append(queue, n)
		if n.child != nil {
			for _, c := range n.child.nodes {
				withChildren(c)
			}
		}
	}
	withChildren(g.nodes[0])
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range cur.edges {
			if nb := e.otherEndInGraph(cur, g, l.root); nb != nil && !visited[nb] {
				withChildren(nb)
			}
		}
	}
	g.connected = false
	if len(visited) >= len(g.nodes) {
		k := 0
		for n := range visited {
			if n.owner == g {
				k++
			}
		}
		g.connected = k == len(g.nodes)
	}
}

// calcLowestCommonAncestors is LGraphManager's, as written.
func (l *layout) calcLowestCommonAncestors() {
	for _, e := range l.edges {
		s, t := e.source, e.target
		e.lca, e.sourceInLca, e.targetInLca = nil, s, t
		if s == t {
			e.lca = s.owner
			continue
		}
		sag := s.owner
		for e.lca == nil {
			e.targetInLca = t
			tag := t.owner
			for e.lca == nil {
				if tag == sag {
					e.lca = tag
					break
				}
				if tag == l.root {
					break
				}
				e.targetInLca = tag.parent
				tag = e.targetInLca.owner
			}
			if sag == l.root {
				break
			}
			if e.lca == nil {
				e.sourceInLca = sag.parent
				sag = e.sourceInLca.owner
			}
		}
	}
}

func (l *layout) calcInclusionTreeDepths(g *lgraph, depth int) {
	for _, n := range g.nodes {
		n.inclusionTreeDepth = depth
		if n.child != nil {
			l.calcInclusionTreeDepths(n.child, depth+1)
		}
	}
}

// calcIdealEdgeLengths lengthens edges between graphs by the sizes of
// their ends' ancestors under the common graph and by the nesting.
func (l *layout) calcIdealEdgeLengths() {
	for _, e := range l.edges {
		if !e.interGraph {
			continue
		}
		orig := e.ideal
		e.ideal += e.sourceInLca.estimatedSize + e.targetInLca.estimatedSize - 2*simpleNodeSize
		lcaDepth := e.lca.inclusionTreeDepth(l.root)
		e.ideal += orig * l.o.NestingFactor * float64(e.source.inclusionTreeDepth+e.target.inclusionTreeDepth-2*lcaDepth)
	}
}

// classicLayout is CoSELayout.classicLayout in the incremental mode, then
// runSpringEmbedder.
func (l *layout) classicLayout() error {
	for _, g := range l.graphs {
		l.updateConnected(g)
		if !g.connected {
			l.gravity = append(l.gravity, g.nodes...)
		}
	}
	for _, n := range l.nodes {
		n.noOfChildren = n.noOfChildrenCalc()
	}
	l.calcLowestCommonAncestors()
	l.calcInclusionTreeDepths(l.root, 1)
	l.root.calcEstimatedSize()
	l.calcIdealEdgeLengths()

	l.handleConstraints()
	l.initConstraintVariables()

	// initParameters and initSpringEmbedder.
	l.idealEdgeLength = l.defaultEdgeLength
	if l.idealEdgeLength < 10 {
		l.idealEdgeLength = 10
	}
	l.coolingFactor = l.o.InitialEnergy
	l.initialCoolingFactor = l.o.InitialEnergy
	s := len(l.nodes)
	if s > 1000 { // ADAPTATION_LOWER_NODE_LIMIT
		l.coolingFactor = math.Max(l.coolingFactor*0.33, l.coolingFactor-float64(s-1000)/float64(5000-1000)*l.coolingFactor*(1-0.33))
	}
	l.maxIterations = max(s*5, l.o.NumIter)
	l.maxCoolingCycle = float64(l.maxIterations) / convergenceCheckPeriod
	l.totalDisplacementThreshold = 3.0 * l.defaultEdgeLength / 100 * float64(s)
	l.repulsionRange = 2 * l.idealEdgeLength
	l.surroundMark = make([]int, len(l.nodes))

	for {
		done, err := l.tick()
		if err != nil {
			return err
		}
		if done {
			break
		}
	}
	l.root.updateBounds()
	return nil
}

// coolingPow25 is V8's Math.pow(c, Math.log(26) / Math.log(25)): the
// cooling schedule's power for the default initial energy (0.3) and up to
// 2500 iterations, taken from Chrome (llvm-libc's pow, which Go's math.Pow
// does not match in the last bit).
var coolingPow25 = [...]float64{0, 1, 2.016962973183477, 3.0404284395288546, 4.068139635193131,
	5.0990195135927845, 6.132431585143718, 7.167954192654435, 8.205287013924684, 9.244205095895865,
	10.284533558456669, 11.32613250293628, 12.368887442815735, 13.412702938985271, 14.457498200059257,
	15.50320394284009, 16.549760091429302, 17.597114052203946, 18.645219394935975, 19.694034827061778,
	20.74352338387001, 21.793651780586632, 22.844389887792374, 23.895710302136873, 24.9475879916334, 26}

func (l *layout) coolingPow() float64 {
	k := 100 * (l.initialCoolingFactor - finalTemperature)
	if k == 26 && l.maxCoolingCycle == 25 && l.coolingCycle < len(coolingPow25) {
		return coolingPow25[l.coolingCycle]
	}
	return math.Pow(float64(l.coolingCycle), math.Log(k)/math.Log(l.maxCoolingCycle))
}

// tick is one iteration; it reports the end.
func (l *layout) tick() (bool, error) {
	l.totalIterations++
	if l.totalIterations == l.maxIterations {
		return true, nil
	}
	if l.totalIterations%convergenceCheckPeriod == 0 {
		if l.isConverged() {
			return true, nil
		}
		l.coolingCycle++
		// Quality "proof": the cooling adjuster stays 1.
		l.coolingFactor = math.Max(l.initialCoolingFactor-l.coolingPow()/100*1, finalTemperature)
	}
	l.totalDisplacement = 0
	l.root.updateBounds()
	l.calcSpringForces()
	if err := l.calcRepulsionForces(); err != nil {
		return false, err
	}
	l.calcGravitationalForces()
	l.moveNodes()
	return false, nil
}

func (l *layout) isConverged() bool {
	oscillating := false
	if float64(l.totalIterations) > float64(l.maxIterations)/3 {
		oscillating = math.Abs(l.totalDisplacement-l.oldTotalDisplacement) < 2
	}
	converged := l.totalDisplacement < l.totalDisplacementThreshold
	l.oldTotalDisplacement = l.totalDisplacement
	return converged || oscillating
}

func sign(v float64) float64 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func (l *layout) calcSpringForces() {
	var clip [4]float64
	for _, e := range l.edges {
		s, t := e.source, e.target
		// updateLength: between the clipping points of the centers' line.
		e.overlapping = intersection(t, s, &clip)
		if e.overlapping {
			continue
		}
		e.lengthX = clip[0] - clip[2]
		e.lengthY = clip[1] - clip[3]
		if math.Abs(e.lengthX) < 1.0 {
			e.lengthX = sign(e.lengthX)
		}
		if math.Abs(e.lengthY) < 1.0 {
			e.lengthY = sign(e.lengthY)
		}
		e.length = math.Sqrt(e.lengthX*e.lengthX + e.lengthY*e.lengthY)
		length := e.length
		if length == 0 {
			continue
		}
		f := e.elasticity * (length - e.ideal)
		fx := f * (e.lengthX / length)
		fy := f * (e.lengthY / length)
		s.springX += fx
		s.springY += fy
		t.springX -= fx
		t.springY -= fy
	}
}

// calcRepulsionForces is the FR grid variant: every tenth iteration the
// grid is rebuilt and each node finds its surrounding, the nodes of its
// graph near it not yet processed.
func (l *layout) calcRepulsionForces() error {
	update := l.totalIterations%gridCheckPeriod == 1
	if update {
		if r := l.root; l.o.Stop != nil && l.o.Stop(r.right-r.left, r.bottom-r.top) {
			return ErrStopped
		}
		if err := l.updateGrid(); err != nil {
			return err
		}
	}
	for _, a := range l.nodes {
		if update {
			l.surrounding(a)
		}
		for _, b := range a.surrounding {
			l.calcRepulsionForce(a, b)
		}
	}
	return nil
}

func (l *layout) updateGrid() error {
	r := l.root
	fx := math.Ceil((r.right - r.left) / l.repulsionRange)
	fy := math.Ceil((r.bottom - r.top) / l.repulsionRange)
	// new Array(n) throws a RangeError for NaN, Infinity or ≥ 2^32; past
	// 2^22 cells the drawing is blown apart and the grid would not be worth
	// its memory: both fail.
	if !(fx >= 0 && fy >= 0 && fx < 1<<32 && fy < 1<<32) || fx*fy > 1<<22 {
		return ErrInvalid
	}
	g := &l.grid
	g.sx, g.sy = int(fx), int(fy)
	cells := g.sx * g.sy
	g.start = resize(g.start, cells+1)
	clear(g.start)
	left, top := r.left, r.top
	// Each node's cells, then the cells' nodes in node order (counting
	// sort; fcose pushes them in that order).
	for _, v := range l.nodes {
		v.startX = int(math.Floor((v.x - left) / l.repulsionRange))
		v.finishX = int(math.Floor((v.w + v.x - left) / l.repulsionRange))
		v.startY = int(math.Floor((v.y - top) / l.repulsionRange))
		v.finishY = int(math.Floor((v.h + v.y - top) / l.repulsionRange))
		if v.startX < 0 || v.finishX >= g.sx || v.startY < 0 || v.finishY >= g.sy {
			return ErrInvalid // fcose: TypeError on an undefined cell
		}
		for i := v.startX; i <= v.finishX; i++ {
			for j := v.startY; j <= v.finishY; j++ {
				g.start[i*g.sy+j+1]++
			}
		}
	}
	for c := 1; c <= cells; c++ {
		g.start[c] += g.start[c-1]
	}
	g.items = resize(g.items, int(g.start[cells]))
	g.fill = resize(g.fill, cells)
	copy(g.fill, g.start[:cells])
	for _, v := range l.nodes {
		for i := v.startX; i <= v.finishX; i++ {
			for j := v.startY; j <= v.finishY; j++ {
				c := i*g.sy + j
				g.items[g.fill[c]] = v
				g.fill[c]++
			}
		}
	}
	return nil
}

// grid is the repulsion grid: cell (i, j)'s nodes are
// items[start[i*sy+j]:start[i*sy+j+1]].
type grid struct {
	sx, sy      int
	start, fill []int32
	items       []*lnode
}

func (g *grid) cell(i, j int) []*lnode {
	c := i*g.sy + j
	return g.items[g.start[c]:g.start[c+1]]
}

func resize[T any](s []T, n int) []T {
	if cap(s) >= n {
		return s[:n]
	}
	return make([]T, n)
}

func (l *layout) surrounding(a *lnode) {
	a.surrounding = a.surrounding[:0]
	l.pass++
	mark := l.pass
	g := &l.grid
	for i := a.startX - 1; i < a.finishX+2; i++ {
		for j := a.startY - 1; j < a.finishY+2; j++ {
			if i < 0 || j < 0 || i >= g.sx || j >= g.sy {
				continue
			}
			for _, b := range g.cell(i, j) {
				if a.owner != b.owner || a == b {
					continue
				}
				// processed: earlier in this pass.
				if b.order < a.order || l.surroundMark[b.order] == mark {
					continue
				}
				dx := math.Abs(a.centerX()-b.centerX()) - (a.w/2 + b.w/2)
				dy := math.Abs(a.centerY()-b.centerY()) - (a.h/2 + b.h/2)
				if dx <= l.repulsionRange && dy <= l.repulsionRange {
					l.surroundMark[b.order] = mark
					a.surrounding = append(a.surrounding, b)
				}
			}
		}
	}
}

func intersects(a, b *lnode) bool {
	return !(a.right() < b.x || a.bottom() < b.y || b.right() < a.x || b.bottom() < a.y)
}

func (l *layout) calcRepulsionForce(a, b *lnode) {
	if intersects(a, b) {
		ox, oy := separation(a, b, l.defaultEdgeLength/2.0)
		rx, ry := 2*ox, 2*oy
		k := a.noOfChildren * b.noOfChildren / (a.noOfChildren + b.noOfChildren)
		a.repulsionX -= k * rx
		a.repulsionY -= k * ry
		b.repulsionX += k * rx
		b.repulsionY += k * ry
		return
	}
	var clip [4]float64
	intersection(a, b, &clip)
	dx, dy := clip[2]-clip[0], clip[3]-clip[1]
	if math.Abs(dx) < l.minRepulsionDist {
		dx = sign(dx) * l.minRepulsionDist
	}
	if math.Abs(dy) < l.minRepulsionDist {
		dy = sign(dy) * l.minRepulsionDist
	}
	d2 := dx*dx + dy*dy
	d := math.Sqrt(d2)
	f := (a.repulsion/2 + b.repulsion/2) * a.noOfChildren * b.noOfChildren / d2
	fx, fy := f*dx/d, f*dy/d
	a.repulsionX -= fx
	a.repulsionY -= fy
	b.repulsionX += fx
	b.repulsionY += fy
}

func (l *layout) calcGravitationalForces() {
	for _, n := range l.gravity {
		g := n.owner
		cx := (g.right + g.left) / 2
		cy := (g.top + g.bottom) / 2
		dx, dy := n.centerX()-cx, n.centerY()-cy
		ax, ay := math.Abs(dx)+n.w/2, math.Abs(dy)+n.h/2
		if g == l.root {
			est := g.estimatedSize * l.o.GravityRange
			if ax > est || ay > est {
				n.gravityX = -l.o.Gravity * dx
				n.gravityY = -l.o.Gravity * dy
			}
		} else {
			est := g.estimatedSize * l.o.GravityRangeCompound
			if ax > est || ay > est {
				n.gravityX = -l.o.Gravity * dx * l.o.GravityCompound
				n.gravityY = -l.o.Gravity * dy * l.o.GravityCompound
			}
		}
	}
}

// moveNodes is CoSELayout.moveNodes: displacements (a compound's goes to
// its members), the constraints' corrections, then the moves.
func (l *layout) moveNodes() {
	for _, n := range l.nodes {
		n.dispX += l.coolingFactor * (n.springX + n.repulsionX + n.gravityX) / n.noOfChildren
		n.dispY += l.coolingFactor * (n.springY + n.repulsionY + n.gravityY) / n.noOfChildren
		lim := l.coolingFactor * maxNodeDisplacement
		if math.Abs(n.dispX) > lim {
			n.dispX = lim * sign(n.dispX)
		}
		if math.Abs(n.dispY) > lim {
			n.dispY = lim * sign(n.dispY)
		}
		if n.child != nil && len(n.child.nodes) > 0 {
			propagate(n, n.dispX, n.dispY)
		}
	}
	l.updateDisplacements()
	for _, n := range l.nodes {
		if n.child == nil || len(n.child.nodes) == 0 {
			n.x += n.dispX
			n.y += n.dispY
			l.totalDisplacement += math.Abs(n.dispX) + math.Abs(n.dispY)
		}
		n.springX, n.springY, n.repulsionX, n.repulsionY, n.gravityX, n.gravityY = 0, 0, 0, 0, 0, 0
		n.dispX, n.dispY = 0, 0
	}
}

func propagate(n *lnode, dx, dy float64) {
	for _, c := range n.child.nodes {
		if c.child == nil {
			c.dispX += dx
			c.dispY += dy
		} else {
			propagate(c, dx, dy)
		}
	}
}

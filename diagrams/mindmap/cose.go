package mindmap

import (
	"math"
	"slices"
)

// The layout is cytoscape-cose-bilkent's (layout-base's FDLayout and
// CoSELayout) as mermaid runs it on a mindmap: quality "proof", random
// start, no compounds. A mindmap is one tree, so only that path is here:
// the tree is first placed radially around its "centre", then a spring
// embedder (springs along edges, repulsion between nearby boxes, cooling)
// moves the nodes until they settle. It is deterministic, as in
// cose-bilkent (whose randomness only seeds disconnected graphs).

const (
	idealEdgeLength = 50.0
	springStrength  = 0.45
	repulsion       = 4500.0
	repulsionRange  = 2 * idealEdgeLength // calcRepulsionRange at level 0
	minRepulsion    = idealEdgeLength / 10
	maxDisplacement = 300.0
	maxIterations   = 2500
	checkPeriod     = 100 // CONVERGENCE_CHECK_PERIOD
	gridPeriod      = 10  // GRID_CALCULATION_CHECK_PERIOD
	worldCenterX    = 1200.0
	worldCenterY    = 900.0
	graphMargin     = 15.0
)

// coseNode is a node's box (top left, size) and the forces on it.
type coseNode struct {
	x, y, w, h       float64
	edges            []*coseEdge
	springX, springY float64
	repX, repY       float64
	surrounding      []*coseNode
}

type coseEdge struct{ source, target *coseNode }

func (n *coseNode) cx() float64 { return n.x + n.w/2 }
func (n *coseNode) cy() float64 { return n.y + n.h/2 }

func (n *coseNode) setCenter(x, y float64) { n.x, n.y = x-n.w/2, y-n.h/2 }

func (e *coseEdge) other(n *coseNode) *coseNode {
	if e.source == n {
		return e.target
	}
	return e.source
}

// intersects is RectangleD.intersects: the boxes overlap or touch.
func (n *coseNode) intersects(m *coseNode) bool {
	return !(n.x+n.w < m.x || n.y+n.h < m.y || m.x+m.w < n.x || m.y+m.h < n.y)
}

// clip is where the ray from n's centre towards (px, py) leaves its box.
func (n *coseNode) clip(px, py float64) (float64, float64) {
	dx, dy := px-n.cx(), py-n.cy()
	t := math.Min(n.w/2/math.Abs(dx), n.h/2/math.Abs(dy))
	return n.cx() + t*dx, n.cy() + t*dy
}

// coseLayout lays out a tree given each node's size and its edges (by
// index, parent first) and returns the nodes' centres.
func coseLayout(sizes [][2]float64, edges [][2]int) [][2]float64 {
	nodes := make([]*coseNode, len(sizes))
	for i, s := range sizes {
		nodes[i] = &coseNode{w: s[0], h: s[1]}
	}
	seen := map[[2]int]bool{}
	var all []*coseEdge
	for _, e := range edges {
		a, b := min(e[0], e[1]), max(e[0], e[1])
		if a == b || seen[[2]int{a, b}] {
			continue
		}
		seen[[2]int{a, b}] = true
		ed := &coseEdge{nodes[e[0]], nodes[e[1]]}
		ed.source.edges = append(ed.source.edges, ed)
		ed.target.edges = append(ed.target.edges, ed)
		all = append(all, ed)
	}
	if len(nodes) > 0 {
		positionRadially(nodes)
		springEmbed(nodes, all)
	}
	out := make([][2]float64, len(nodes))
	for i, n := range nodes {
		out[i] = [2]float64{n.cx(), n.cy()}
	}
	return out
}

// positionRadially is positionNodesRadially for one tree: branches spread
// over the angle their parent got, one ring per level, around the tree's
// centre node, the whole moved to the world's centre.
func positionRadially(nodes []*coseNode) {
	order := bfs(nodes[0])
	sep := idealEdgeLength
	for _, n := range nodes {
		sep = math.Max(sep, math.Hypot(n.w, n.h))
	}
	branch(treeCenter(order), nil, 0, 359, 0, sep)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, n := range order {
		minX, minY = math.Min(minX, n.x), math.Min(minY, n.y)
		maxX, maxY = math.Max(maxX, n.x+n.w), math.Max(maxY, n.y+n.h)
	}
	// The tree's top left goes to the origin, then the graph's (its box
	// with the margin) to the world's centre less half the tree.
	dx := worldCenterX - (maxX-minX)/2 + graphMargin - minX
	dy := worldCenterY - (maxY-minY)/2 + graphMargin - minY
	for _, n := range order {
		n.x += dx
		n.y += dy
	}
}

// bfs lists a tree's nodes breadth first, neighbours in edge order.
func bfs(root *coseNode) []*coseNode {
	order := []*coseNode{root}
	seen := map[*coseNode]bool{root: true}
	for i := 0; i < len(order); i++ {
		for _, e := range order[i].edges {
			if o := e.other(order[i]); !seen[o] {
				seen[o] = true
				order = append(order, o)
			}
		}
	}
	return order
}

// treeCenter is Layout.findCenterOfTree as it runs: each round meant to
// peel the leaves removes every other node of the list instead (it splices
// the list it iterates), so the centre is the node the halving leaves
// first.
func treeCenter(order []*coseNode) *coseNode {
	list := slices.Clone(order)
	for len(list) > 2 {
		var odd []*coseNode
		for i := 1; i < len(list); i += 2 {
			odd = append(odd, list[i])
		}
		list = odd
	}
	return list[0]
}

// branch is branchRadialLayout: n at the middle of its angle interval at
// the given distance, its other neighbours sharing the interval, starting
// after the parent's edge.
func branch(n, parent *coseNode, start, end, dist, sep float64) {
	half := (end - start + 1) / 2
	if half < 0 {
		half += 180
	}
	teta := math.Mod(half+start, 360) * 2 * math.Pi / 360
	n.setCenter(dist*math.Cos(teta), dist*math.Sin(teta))
	count := len(n.edges)
	children := count
	first := 0
	if parent != nil {
		children--
		for i, e := range n.edges {
			if e.other(n) == parent {
				first = (i + 1) % count
				break
			}
		}
	}
	step := math.Abs(end-start) / float64(children)
	placed := 0
	for i := first; placed != children; i = (i + 1) % count {
		c := n.edges[i].other(n)
		if c == parent {
			continue
		}
		cs := math.Mod(start+float64(placed)*step, 360)
		branch(c, n, cs, math.Mod(cs+step, 360), dist+sep, sep)
		placed++
	}
}

// springEmbed runs the spring embedder until it converges or runs out of
// iterations (CoSELayout.tick without tree reduction).
func springEmbed(nodes []*coseNode, edges []*coseEdge) {
	n := float64(len(nodes))
	cooling := 1.0
	if n > 1000 { // ADAPTATION_LOWER_NODE_LIMIT .. UPPER
		cooling = math.Max(0.33, 1-(n-1000)/(5000-1000)*(1-0.33))
	}
	initialCooling := cooling
	iterations := max(len(nodes)*5, maxIterations)
	threshold := 3 * idealEdgeLength / 100 * n
	const finalTemperature = float64(checkPeriod) / maxIterations
	maxCoolingCycle := float64(maxIterations) / checkPeriod
	cycle := 0.0
	total, oldTotal := 0.0, 0.0
	for it := 1; it != iterations; it++ {
		if it%checkPeriod == 0 {
			oscillating := float64(it) > float64(iterations)/3 && math.Abs(total-oldTotal) < 2
			converged := total < threshold
			oldTotal = total
			if converged || oscillating {
				return
			}
			cycle++
			cooling = math.Max(initialCooling-math.Pow(cycle, math.Log(100*(initialCooling-finalTemperature))/math.Log(maxCoolingCycle))/100, finalTemperature)
		}
		total = 0
		for _, e := range edges {
			springForce(e)
		}
		if it%gridPeriod == 1 {
			findSurroundings(nodes)
		}
		for _, a := range nodes {
			for _, b := range a.surrounding {
				repulse(a, b)
			}
		}
		for _, a := range nodes {
			dx := clampAbs(cooling*(a.springX+a.repX), cooling*maxDisplacement)
			dy := clampAbs(cooling*(a.springY+a.repY), cooling*maxDisplacement)
			a.x += dx
			a.y += dy
			total += math.Abs(dx) + math.Abs(dy)
			a.springX, a.springY, a.repX, a.repY = 0, 0, 0, 0
		}
	}
}

func clampAbs(v, limit float64) float64 {
	if math.Abs(v) > limit {
		return math.Copysign(limit, v)
	}
	return v
}

// findSurroundings gives each node the later nodes whose boxes are within
// the repulsion range of its own (what the FR grid finds).
func findSurroundings(nodes []*coseNode) {
	for i, a := range nodes {
		a.surrounding = a.surrounding[:0]
		for _, b := range nodes[i+1:] {
			gapX := math.Abs(a.cx()-b.cx()) - (a.w/2 + b.w/2)
			gapY := math.Abs(a.cy()-b.cy()) - (a.h/2 + b.h/2)
			if gapX <= repulsionRange && gapY <= repulsionRange {
				a.surrounding = append(a.surrounding, b)
			}
		}
	}
}

// springForce pulls an edge's ends towards the ideal length between their
// boxes' borders (none while the boxes overlap).
func springForce(e *coseEdge) {
	s, t := e.source, e.target
	if t.intersects(s) {
		return
	}
	tx, ty := t.clip(s.cx(), s.cy())
	sx, sy := s.clip(t.cx(), t.cy())
	lx, ly := atLeastOne(tx-sx), atLeastOne(ty-sy)
	l := math.Hypot(lx, ly)
	if l == 0 {
		return
	}
	f := springStrength * (l - idealEdgeLength)
	s.springX += f * lx / l
	s.springY += f * ly / l
	t.springX -= f * lx / l
	t.springY -= f * ly / l
}

// atLeastOne rounds a length under 1 to its sign.
func atLeastOne(v float64) float64 {
	if math.Abs(v) < 1 {
		return sign(v)
	}
	return v
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

// repulse pushes two boxes apart: overlapping ones by their overlap,
// others with an inverse-square force between their borders.
func repulse(a, b *coseNode) {
	if a.intersects(b) {
		ox, oy := separation(a, b, idealEdgeLength/2)
		// childrenConstant: 1*1/(1+1) for leaves.
		a.repX -= ox
		a.repY -= oy
		b.repX += ox
		b.repY += oy
		return
	}
	ax, ay := a.clip(b.cx(), b.cy())
	bx, by := b.clip(a.cx(), a.cy())
	dx, dy := bx-ax, by-ay
	if math.Abs(dx) < minRepulsion {
		dx = sign(dx) * minRepulsion
	}
	if math.Abs(dy) < minRepulsion {
		dy = sign(dy) * minRepulsion
	}
	d2 := dx*dx + dy*dy
	d := math.Sqrt(d2)
	f := repulsion / d2
	a.repX -= f * dx / d
	a.repY -= f * dy / d
	b.repX += f * dx / d
	b.repY += f * dy / d
}

// separation is IGeometry.calcSeparationAmount: how far apart (with a
// buffer) to move two overlapping boxes along the line between their
// centres, halved (the force the pair's children constant gives).
func separation(a, b *coseNode, buffer float64) (float64, float64) {
	dirX, dirY := 1.0, 1.0
	if a.cx() < b.cx() {
		dirX = -1
	}
	if a.cy() < b.cy() {
		dirY = -1
	}
	ox := math.Min(a.x+a.w, b.x+b.w) - math.Max(a.x, b.x)
	oy := math.Min(a.y+a.h, b.y+b.h) - math.Max(a.y, b.y)
	ox += containedExtra(a.x, a.x+a.w, b.x, b.x+b.w)
	oy += containedExtra(a.y, a.y+a.h, b.y, b.y+b.h)
	slope := math.Abs((b.cy() - a.cy()) / (b.cx() - a.cx()))
	if b.cy() == a.cy() && b.cx() == a.cx() {
		slope = 1
	}
	moveY := slope * ox
	moveX := oy / slope
	if ox < moveX {
		moveX = ox
	} else {
		moveY = oy
	}
	// The force is twice the amount, times the children constant 1/2.
	return -dirX * (moveX/2 + buffer), -dirY * (moveY/2 + buffer)
}

// containedExtra is what calcSeparationAmount adds when one interval
// holds the other: the smaller of the gaps at its ends.
func containedExtra(a0, a1, b0, b1 float64) float64 {
	switch {
	case a0 <= b0 && a1 >= b1:
		return math.Min(b0-a0, a1-b1)
	case b0 <= a0 && b1 >= a1:
		return math.Min(a0-b0, b1-a1)
	}
	return 0
}

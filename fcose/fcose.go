// Package fcose is a port of the layout cytoscape-fcose 2.2.0 makes (with
// cose-base 2.2.0 and layout-base 2.0.1, the versions mermaid 12.2.0
// bundles) in the configuration mermaid's architecture diagram uses it:
// quality "proof", randomize false (so no spectral layout: the nodes start
// where they are), animate false, and always constraints, so no tiling,
// no component packing and no tree reduction. For the same input it gives
// the same rects as fcose in Chrome, bit for bit on the captured cases
// (testdata).
//
// What it does (CoSE, the compound spring embedder, after constraint
// enforcement): the constraints are enforced on the start positions
// (alignments to their mean, relative placements by a longest-path pass
// over the placement DAG); then up to max(2500, 5·nodes) iterations of
// spring forces along edges, repulsion between nodes of the same graph
// that are near on a grid (the FR grid variant), gravity towards the
// center of the owner graph for graphs that are not connected, and moves
// limited by a falling temperature, with the constraints kept by averaging
// displacements (alignments) and by shuffled relative placement passes
// (mulberry32 in mermaid; Options.Random here).
//
// Exactness rests on JS evaluation order (sums over nodes in cose-base's
// getAllNodes order, which is the graphs in creation order), on Map and Set
// insertion order in the constraint handler, and on V8's Math.pow for the
// cooling schedule, which is llvm-libc's (not correctly rounded): it is a
// table taken from Chrome for the usual schedule (fewer than 501 nodes),
// and Go's math.Pow beyond.
package fcose

import (
	"errors"
	"math"
)

// Graph is what fcose reads off cytoscape: nodes in cytoscape's order
// (eles.nodes()), edges in its order (eles.edges()).
type Graph struct {
	Nodes []Node
	Edges []Edge
}

// Node is a cytoscape node.
type Node struct {
	ID string
	// Parent is the index of the compound node this one is in, -1 at the
	// top. A node is compound when another names it as parent.
	Parent int
	// X, Y is the node's position (its center); W, H its layout
	// dimensions. A compound's are not used: cose-base sizes compounds
	// from their members.
	X, Y, W, H float64
	// Padding is a compound's padding (parseInt of its css padding), the
	// margin of its members' bounds.
	Padding float64
}

// Edge joins two nodes, by index. Loops and second edges between the same
// two nodes are dropped, as fcose drops them.
type Edge struct {
	Source, Target int
	// IdealLength and Elasticity are fcose's idealEdgeLength and
	// edgeElasticity for the edge.
	IdealLength, Elasticity float64
}

// Constraints are fcose's alignmentConstraint and
// relativePlacementConstraint, with nodes by index.
type Constraints struct {
	// Horizontal lists share a y, Vertical lists an x.
	Horizontal, Vertical [][]int
	Relative             []Relative
}

// Relative puts A left of B (or above, if Vertical) by at least Gap,
// between centers.
type Relative struct {
	A, B     int
	Gap      float64
	Vertical bool
}

// Options are fcose's numeric options.
type Options struct {
	NodeRepulsion        float64 // nodeRepulsion, for every node
	NestingFactor        float64
	Gravity              float64
	GravityRange         float64
	GravityCompound      float64
	GravityRangeCompound float64
	InitialEnergy        float64 // initialEnergyOnIncremental
	NumIter              int
	// Random is Math.random during the layout (mermaid seeds it with
	// mulberry32; see Mulberry32). Nil means Mulberry32(1).
	Random func() float64
	// Stop, if set, is asked every tenth iteration with the drawing's
	// width and height; true ends the layout with ErrStopped (not fcose's:
	// for a caller that has no use for a drawing blown apart).
	Stop func(w, h float64) bool
}

// DefaultOptions are fcose's defaults, with mermaid's numIter.
func DefaultOptions() Options {
	return Options{NodeRepulsion: 4500, NestingFactor: 0.1, Gravity: 0.25, GravityRange: 3.8,
		GravityCompound: 1, GravityRangeCompound: 1.5, InitialEnergy: 0.3, NumIter: 2500}
}

// Mulberry32 is mermaid's withSeededRandom generator (architectureSeed.ts).
func Mulberry32(seed uint32) func() float64 {
	state := seed
	return func() float64 {
		state += 0x6d2b79f5
		t := state
		t = (t ^ t>>15) * (t | 1)
		t ^= t + (t^t>>7)*(t|61)
		return float64(t^t>>14) / 4294967296
	}
}

// Rect is a node's box: its left top corner and size.
type Rect struct{ X, Y, W, H float64 }

// Center is the rect's center, computed as cose-base does.
func (r Rect) Center() (float64, float64) { return r.X + r.W/2, r.Y + r.H/2 }

// Result is the layout's answer.
type Result struct {
	// Rects are the nodes' rects (compounds' too) after the layout, by
	// node index, before fcose moves the drawing back to where it was.
	Rects []Rect
	// Iterations is the number of iterations run.
	Iterations int
}

// ErrInvalid is returned where fcose throws: its repulsion grid cannot be
// made because a coordinate is not finite (constraints that contradict
// each other, as an `align` chain against the edges, give -Infinity) or
// the drawing is absurdly large.
var ErrInvalid = errors.New("fcose: the layout has no finite extent (contradicting constraints)")

// ErrStopped is returned when Options.Stop ended the layout.
var ErrStopped = errors.New("fcose: stopped")

// Layout runs fcose's CoSE layout (coseLayout and runLayout).
func Layout(g *Graph, c *Constraints, o Options) (*Result, error) {
	if o.Random == nil {
		o.Random = Mulberry32(1)
	}
	l := build(g, c, o)
	res := &Result{Rects: make([]Rect, len(g.Nodes))}
	if len(l.root.nodes) > 0 {
		if err := l.classicLayout(); err != nil {
			return nil, err
		}
	}
	for _, n := range l.nodes {
		res.Rects[n.index] = Rect{n.x, n.y, n.w, n.h}
	}
	res.Iterations = l.totalIterations
	return res, nil
}

// Recenter is fcose's relocateComponent and the transfer to cytoscape:
// the rects' bounding box is moved to be centered on (cx, cy), the center
// of the elements' bounding box before the layout, and each node's
// position is its rect's center. The positions are by node index.
func Recenter(rects []Rect, cx, cy float64) [][2]float64 {
	minX, maxX, minY, maxY := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, r := range rects {
		if r.X < minX {
			minX = r.X
		}
		if r.X+r.W > maxX {
			maxX = r.X + r.W
		}
		if r.Y < minY {
			minY = r.Y
		}
		if r.Y+r.H > maxY {
			maxY = r.Y + r.H
		}
	}
	dx, dy := cx-(maxX+minX)/2, cy-(maxY+minY)/2
	out := make([][2]float64, len(rects))
	for i, r := range rects {
		// setCenter(getCenterX() + diff), then getCenterX().
		x := r.X + r.W/2 + dx - r.W/2
		y := r.Y + r.H/2 + dy - r.H/2
		out[i] = [2]float64{x + r.W/2, y + r.H/2}
	}
	return out
}

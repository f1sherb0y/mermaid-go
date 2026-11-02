package d3

import "math"

// Node is a hierarchy node for the layouts: its own value (leaves), the
// sum of its subtree after Sum, and its box after a layout.
type Node struct {
	Value          float64
	Children       []*Node
	Parent         *Node
	Depth          int
	X0, Y0, X1, Y1 float64
	// Data is the caller's.
	Data any
}

// Sum sets every node's Value to its own plus its descendants', and sorts
// children by value, largest first (stable), as
// hierarchy.sum(...).sort((a, b) => b.value - a.value).
func (n *Node) Sum() float64 {
	for _, c := range n.Children {
		c.Parent, c.Depth = n, n.Depth+1
		n.Value += c.Sum()
	}
	sortByValue(n.Children)
	return n.Value
}

func sortByValue(ns []*Node) {
	// Insertion sort: stable, and the lists are short.
	for i := 1; i < len(ns); i++ {
		for j := i; j > 0 && ns[j].Value > ns[j-1].Value; j-- {
			ns[j], ns[j-1] = ns[j-1], ns[j]
		}
	}
}

// Each visits n and its descendants, parents first.
func (n *Node) Each(fn func(*Node)) {
	fn(n)
	for _, c := range n.Children {
		c.Each(fn)
	}
}

// Treemap is d3.treemap with the squarify tiling.
type Treemap struct {
	W, H float64
	// Padding of a node with children (zero for leaves), and between
	// siblings.
	PaddingTop, PaddingRight, PaddingBottom, PaddingLeft func(*Node) float64
	PaddingInner                                         func(*Node) float64
	Round                                                bool
}

var phi = (1 + math.Sqrt(5)) / 2

// Layout places root and its descendants.
func (t *Treemap) Layout(root *Node) {
	zero := func(*Node) float64 { return 0 }
	for _, f := range []*func(*Node) float64{&t.PaddingTop, &t.PaddingRight, &t.PaddingBottom, &t.PaddingLeft, &t.PaddingInner} {
		if *f == nil {
			*f = zero
		}
	}
	stack := []float64{0}
	root.X0, root.Y0, root.X1, root.Y1 = 0, 0, t.W, t.H
	root.Each(func(n *Node) {
		p := stack[n.Depth]
		x0, y0, x1, y1 := n.X0+p, n.Y0+p, n.X1-p, n.Y1-p
		if x1 < x0 {
			x0 = (x0 + x1) / 2
			x1 = x0
		}
		if y1 < y0 {
			y0 = (y0 + y1) / 2
			y1 = y0
		}
		n.X0, n.Y0, n.X1, n.Y1 = x0, y0, x1, y1
		if len(n.Children) == 0 {
			return
		}
		inner := t.PaddingInner(n) / 2
		for len(stack) <= n.Depth+1 {
			stack = append(stack, 0)
		}
		stack[n.Depth+1] = inner
		x0 += t.PaddingLeft(n) - inner
		y0 += t.PaddingTop(n) - inner
		x1 -= t.PaddingRight(n) - inner
		y1 -= t.PaddingBottom(n) - inner
		if x1 < x0 {
			x0 = (x0 + x1) / 2
			x1 = x0
		}
		if y1 < y0 {
			y0 = (y0 + y1) / 2
			y1 = y0
		}
		squarify(n, x0, y0, x1, y1)
	})
	if t.Round {
		root.Each(func(n *Node) {
			n.X0, n.Y0, n.X1, n.Y1 = math.Round(n.X0), math.Round(n.Y0), math.Round(n.X1), math.Round(n.Y1)
		})
	}
}

// squarify is d3's squarifyRatio(phi, ...): rows of children whose aspect
// ratios are as close to phi as greedy filling gets.
func squarify(parent *Node, x0, y0, x1, y1 float64) {
	nodes := parent.Children
	value := parent.Value
	for i0, i1, n := 0, 0, len(nodes); i0 < n; i0 = i1 {
		dx, dy := x1-x0, y1-y0
		var sum float64
		for {
			sum = nodes[i1].Value
			i1++
			if sum != 0 || i1 >= n {
				break
			}
		}
		minV, maxV := sum, sum
		alpha := math.Max(dy/dx, dx/dy) / (value * phi)
		beta := sum * sum * alpha
		minRatio := math.Max(maxV/beta, beta/minV)
		for ; i1 < n; i1++ {
			v := nodes[i1].Value
			sum += v
			minV, maxV = math.Min(minV, v), math.Max(maxV, v)
			beta = sum * sum * alpha
			ratio := math.Max(maxV/beta, beta/minV)
			if ratio > minRatio {
				sum -= v
				break
			}
			minRatio = ratio
		}
		// The row takes the strip from the old edge to the advanced one.
		row := nodes[i0:i1]
		if dx < dy {
			top, bottom := y0, y1
			if value != 0 {
				y0 += dy * sum / value
				bottom = y0
			}
			dice(row, sum, x0, top, x1, bottom)
		} else {
			left, right := x0, x1
			if value != 0 {
				x0 += dx * sum / value
				right = x0
			}
			slice(row, sum, left, y0, right, y1)
		}
		value -= sum
	}
}

// dice lays a row out left to right in the box.
func dice(row []*Node, sum, x0, y0, x1, y1 float64) {
	k := 0.0
	if sum != 0 {
		k = (x1 - x0) / sum
	}
	for _, n := range row {
		n.Y0, n.Y1 = y0, y1
		n.X0 = x0
		x0 += n.Value * k
		n.X1 = x0
	}
}

// slice lays a row out top to bottom in the box.
func slice(row []*Node, sum, x0, y0, x1, y1 float64) {
	k := 0.0
	if sum != 0 {
		k = (y1 - y0) / sum
	}
	for _, n := range row {
		n.X0, n.X1 = x0, x1
		n.Y0 = y0
		y0 += n.Value * k
		n.Y1 = y0
	}
}

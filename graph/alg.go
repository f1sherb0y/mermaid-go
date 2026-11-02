package graph

// Preorder returns the nodes reachable from vs in depth-first preorder,
// following successors (neighbors if the graph is undirected).
func Preorder[N, E any](g *Graph[N, E], vs ...string) []string { return dfs(g, vs, false) }

// Postorder is Preorder's postorder counterpart.
func Postorder[N, E any](g *Graph[N, E], vs ...string) []string { return dfs(g, vs, true) }

func dfs[N, E any](g *Graph[N, E], vs []string, post bool) []string {
	visited := make(map[*node[N, E]]bool, len(g.nodes))
	acc := make([]string, 0, len(g.nodes))
	var visit func(n *node[N, E])
	visit = func(n *node[N, E]) {
		if visited[n] {
			return
		}
		visited[n] = true
		if !post {
			acc = append(acc, n.id)
		}
		// Neighbors: predecessors first, then successors not among them.
		if g.opt.Undirected {
			for _, p := range n.preds.items {
				visit(p.n)
			}
		}
		for _, s := range n.sucs.items {
			if !g.opt.Undirected || n.preds.find(s.n) < 0 {
				visit(s.n)
			}
		}
		if post {
			acc = append(acc, n.id)
		}
	}
	for _, v := range vs {
		n := g.nodes[v]
		if n == nil {
			panic("graph: no node " + v)
		}
		visit(n)
	}
	return acc
}

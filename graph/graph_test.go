package graph

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
)

// opsCase is a random sequence of graphlib operations and graphlib's
// listings afterwards (tools/dagre/gen.mjs).
type opsCase struct {
	Opt     struct{ Directed, Multigraph, Compound bool }
	Ops     [][]any
	Nodes   []string
	Edges   [][4]any
	Roots   []string
	Sources []string
	Sinks   []string
	Detail  map[string]struct {
		Label                  any
		Preds, Sucs, Neighbors []string
		In, Out, NodeEdges     [][3]string
		Children               []string
		Parent                 string
	}
}

func TestGraphlibOrder(t *testing.T) {
	f, err := os.Open("testdata/ops.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []opsCase
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			g := New[any, any](Options{Undirected: !c.Opt.Directed, Multigraph: c.Opt.Multigraph, Compound: c.Opt.Compound})
			for _, op := range c.Ops {
				s := func(i int) string { return op[i].(string) }
				switch op[0] {
				case "n":
					if len(op) > 2 {
						g.SetNode(s(1), op[2])
					} else {
						g.EnsureNode(s(1))
					}
				case "rn":
					g.RemoveNode(s(1))
				case "e":
					g.SetEdge(Edge{s(1), s(2), s(3)}, op[4])
				case "re":
					g.RemoveEdge(Edge{s(1), s(2), s(3)})
				case "p":
					g.SetParent(s(1), s(2))
				}
			}
			eq := func(what string, got, want []string) {
				t.Helper()
				if len(got) != len(want) || !slices.Equal(got, want) && len(want) > 0 {
					t.Errorf("%s = %q, want %q", what, got, want)
				}
			}
			eqEdges := func(what string, got []Edge, want [][3]string) {
				t.Helper()
				var g2 [][3]string
				for _, e := range got {
					g2 = append(g2, [3]string{e.V, e.W, e.Name})
				}
				if len(g2) != len(want) || !slices.Equal(g2, want) && len(want) > 0 {
					t.Errorf("%s = %q, want %q", what, g2, want)
				}
			}
			eq("nodes", g.Nodes(), c.Nodes)
			eq("roots", g.Children(""), c.Roots)
			eq("sources", g.Sources(), c.Sources)
			eq("sinks", g.Sinks(), c.Sinks)
			var wantEdges [][3]string
			for _, e := range c.Edges {
				wantEdges = append(wantEdges, [3]string{e[0].(string), e[1].(string), e[2].(string)})
			}
			edges := g.Edges()
			eqEdges("edges", edges, wantEdges)
			for i, e := range edges {
				if i < len(c.Edges) && g.Edge(e) != c.Edges[i][3] {
					t.Errorf("edge %v label = %v, want %v", e, g.Edge(e), c.Edges[i][3])
				}
			}
			for v, d := range c.Detail {
				if g.Node(v) != d.Label {
					t.Errorf("node %q label = %v, want %v", v, g.Node(v), d.Label)
				}
				eq(v+" preds", g.Predecessors(v), d.Preds)
				eq(v+" sucs", g.Successors(v), d.Sucs)
				eq(v+" neighbors", g.Neighbors(v), d.Neighbors)
				eqEdges(v+" in", g.InEdges(v, ""), d.In)
				eqEdges(v+" out", g.OutEdges(v, ""), d.Out)
				eqEdges(v+" nodeEdges", g.NodeEdges(v, ""), d.NodeEdges)
				eq(v+" children", g.Children(v), d.Children)
				if g.Parent(v) != d.Parent {
					t.Errorf("%s parent = %q, want %q", v, g.Parent(v), d.Parent)
				}
			}
		})
	}
}

func TestLess(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"a", "b", true}, {"b", "a", false}, {"a", "ab", true}, {"", "a", true}, {"a", "a", false},
		{"\U0001F600", "！", true}, // surrogate pair before U+FF01 in UTF-16
		{"！", "\U0001F600", false},
		{"é", "z", false}, {"\U0001F600", "\U0001F601", true},
	} {
		if got := Less(c.a, c.b); got != c.want {
			t.Errorf("Less(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestArrayIndex(t *testing.T) {
	for s, want := range map[string]int64{
		"0": 0, "7": 7, "10": 10, "4294967294": 4294967294,
		"4294967295": -1, "01": -1, "-1": -1, "1.5": -1, "": -1, "a": -1, "99999999999": -1,
	} {
		if got := arrayIndex(s); got != want {
			t.Errorf("arrayIndex(%q) = %d, want %d", s, got, want)
		}
	}
}

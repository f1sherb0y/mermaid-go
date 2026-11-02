package dagre

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// TestTrace compares the layout graph after every step with dagre's, for the
// cases dumped by tools/dagre/trace.mjs. Run with DAGRE_TRACE=1.
func TestTrace(t *testing.T) {
	if os.Getenv("DAGRE_TRACE") == "" {
		t.Skip("set DAGRE_TRACE=1 after running tools/dagre/trace.mjs")
	}
	b, err := os.ReadFile("testdata/trace.json")
	if err != nil {
		t.Fatal(err)
	}
	var traces []struct {
		Case  layoutCase
		Steps []struct {
			Step  string
			Nodes [][8]any
			Edges [][3]string
		}
		Err string
		BK  [][2]json.RawMessage
	}
	if err := json.Unmarshal(b, &traces); err != nil {
		t.Fatal(err)
	}
	defer func() { traceStep = nil }()
	for _, tr := range traces {
		t.Run(tr.Case.Name, func(t *testing.T) {
			g, opt := tr.Case.input()
			i := 0
			stop := false
			traceStep = func(step string, lg *lgraph) {
				if stop || i >= len(tr.Steps) {
					return
				}
				want := tr.Steps[i]
				i++
				if want.Step != step {
					t.Errorf("step %s, dagre at %s", step, want.Step)
					stop = true
					return
				}
				var diffs []string
				nodes := lg.Nodes()
				if len(nodes) != len(want.Nodes) {
					diffs = append(diffs, fmt.Sprintf("%d nodes, dagre %d", len(nodes), len(want.Nodes)))
				}
				for k, v := range nodes {
					if k >= len(want.Nodes) {
						break
					}
					w := want.Nodes[k]
					n := lg.Node(v)
					if w[0] != v {
						diffs = append(diffs, fmt.Sprintf("node %d is %q, dagre %q", k, v, w[0]))
						break
					}
					rank := float64(n.rank)
					if n.halfRank {
						rank += 0.5
					}
					got := []any{rank, float64(n.order), n.x, n.y, n.width, n.height, lg.Parent(v)}
					for j, name := range []string{"rank", "order", "x", "y", "width", "height", "parent"} {
						if w[j+1] == nil {
							continue
						}
						if fmt.Sprint(got[j]) != fmt.Sprint(w[j+1]) {
							diffs = append(diffs, fmt.Sprintf("%s.%s = %v, dagre %v", v, name, got[j], w[j+1]))
						}
					}
				}
				edges := lg.Edges()
				if len(edges) != len(want.Edges) {
					diffs = append(diffs, fmt.Sprintf("%d edges, dagre %d", len(edges), len(want.Edges)))
				}
				for k, e := range edges {
					if k < len(want.Edges) && [3]string{e.V, e.W, e.Name} != want.Edges[k] {
						diffs = append(diffs, fmt.Sprintf("edge %d is %v, dagre %v", k, e, want.Edges[k]))
						break
					}
				}
				if len(diffs) > 0 {
					if len(diffs) > 20 {
						diffs = diffs[:20]
					}
					t.Errorf("after %s:\n  %s", step, fmt.Sprint(diffs))
					stop = true
				}
			}
			bk := 1 // after conflicts
			traceBK = func(name string, layering [][]string, root map[string]string, xs map[string]float64) {
				if bk >= len(tr.BK) {
					return
				}
				var want struct {
					Layering [][]string
					Root     map[string]string
					XS       map[string]num
				}
				json.Unmarshal(tr.BK[bk][1], &want)
				bk++
				if fmt.Sprint(want.Layering) != fmt.Sprint(layering) {
					t.Errorf("bk %s layering %v, dagre %v", name, layering, want.Layering)
				}
				for v, r := range want.Root {
					if root[v] != r {
						t.Errorf("bk %s root[%s] = %s, dagre %s", name, v, root[v], r)
					}
				}
				for v, x := range want.XS {
					if fmt.Sprint(xs[v]) != fmt.Sprint(float64(x)) {
						t.Errorf("bk %s xs[%s] = %v, dagre %v", name, v, xs[v], x)
					}
				}
			}
			defer func() { traceBK = nil }()
			_, _, err := layoutFrom(g, opt, tr.Case.Want.IDStart)
			t.Logf("Go error %v, dagre error %q", err, tr.Err)
		})
	}
}

package fcose

import (
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"testing"
)

// The captures (testdata/captures.json.gz, by case) are mermaid 12.2.0's fcose runs in Chrome,
// recorded by `CAPTURE_FCOSE=<dir> node tools/ref.mjs ref ...`: for each of
// the two runs per diagram, the cose-base graph fcose built, the
// constraints, the rects after the layout; then cytoscape's final
// positions.

type capRect struct {
	ID       string  `json:"id"`
	Parent   *string `json:"parent"`
	Compound bool    `json:"compound"`
	Pad      float64 `json:"pad"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	W        float64 `json:"w"`
	H        float64 `json:"h"`
}

type capRun struct {
	Nodes []capRect `json:"nodes"`
	Edges []struct {
		ID    string  `json:"id"`
		Ideal float64 `json:"ideal"`
		Elas  float64 `json:"elas"`
	} `json:"edges"`
	BBox struct{ X1, Y1, X2, Y2 float64 } `json:"bbox"`
	Cy   []struct {
		ID   string  `json:"id"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		Dims struct{ W, H float64 }
	} `json:"cy"`
	CyEdges []struct {
		ID string `json:"id"`
		S  string `json:"s"`
		T  string `json:"t"`
	} `json:"cyEdges"`
	Alignment *struct {
		Horizontal [][]string `json:"horizontal"`
		Vertical   [][]string `json:"vertical"`
	} `json:"alignment"`
	Relative []map[string]any `json:"relative"`
	Randoms  int              `json:"randoms"`
	Iter     int              `json:"iterations"`
	Out      []capRect        `json:"out"`
	Err      string           `json:"err"`
}

type capture struct {
	Err   string   `json:"err"`
	Runs  []capRun `json:"runs"`
	Final []struct {
		ID     string  `json:"id"`
		Parent bool    `json:"parent"`
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
	} `json:"final"`
}

func loadCaptures(t *testing.T) map[string]*capture {
	f, err := os.Open("testdata/captures.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*capture{}
	if err := json.NewDecoder(z).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

// input rebuilds fcose's input from a captured run.
func (r *capRun) input() (*Graph, *Constraints, map[string]int) {
	idx := map[string]int{}
	for i, n := range r.Cy {
		idx[n.ID] = i
	}
	info := map[string]capRect{}
	for _, n := range r.Nodes {
		info[n.ID] = n
	}
	g := &Graph{}
	for _, n := range r.Cy {
		p := -1
		if in := info[n.ID]; in.Parent != nil {
			p = idx[*in.Parent]
		}
		g.Nodes = append(g.Nodes, Node{ID: n.ID, Parent: p, X: n.X, Y: n.Y, W: n.Dims.W, H: n.Dims.H, Padding: info[n.ID].Pad})
	}
	ideal := map[string][2]float64{}
	for _, e := range r.Edges {
		ideal[e.ID] = [2]float64{e.Ideal, e.Elas}
	}
	for _, e := range r.CyEdges {
		v := ideal[e.ID]
		g.Edges = append(g.Edges, Edge{Source: idx[e.S], Target: idx[e.T], IdealLength: v[0], Elasticity: v[1]})
	}
	c := &Constraints{}
	ids := func(l []string) []int {
		var o []int
		for _, s := range l {
			o = append(o, idx[s])
		}
		return o
	}
	if r.Alignment != nil {
		for _, l := range r.Alignment.Horizontal {
			c.Horizontal = append(c.Horizontal, ids(l))
		}
		for _, l := range r.Alignment.Vertical {
			c.Vertical = append(c.Vertical, ids(l))
		}
	}
	for _, m := range r.Relative {
		gap, _ := m["gap"].(float64)
		if l, ok := m["left"].(string); ok {
			c.Relative = append(c.Relative, Relative{A: idx[l], B: idx[m["right"].(string)], Gap: gap})
		} else {
			c.Relative = append(c.Relative, Relative{A: idx[m["top"].(string)], B: idx[m["bottom"].(string)], Gap: gap, Vertical: true})
		}
	}
	return g, c, idx
}

// TestCaptures replays every captured run and compares the rects with
// fcose's.
func TestCaptures(t *testing.T) {
	exact, close, runs := 0, 0, 0
	for name, c := range loadCaptures(t) {
		for ri, r := range c.Runs {
			runs++
			g, cons, idx := r.input()
			calls := 0
			rnd := Mulberry32(1)
			o := DefaultOptions()
			o.Random = func() float64 { calls++; return rnd() }
			// The graph fcose builds, in its order.
			l := build(g, cons, o)
			for i, n := range l.nodes {
				want := r.Nodes[i]
				if n.id != want.ID || n.x != want.X || n.y != want.Y || n.w != want.W || n.h != want.H {
					t.Errorf("%s run %d: node %d is %s %v, fcose has %s %v", name, ri, i, n.id, [4]float64{n.x, n.y, n.w, n.h}, want.ID, [4]float64{want.X, want.Y, want.W, want.H})
				}
			}
			res, err := Layout(g, cons, o)
			if r.Err != "" {
				if err == nil {
					t.Errorf("%s run %d: fcose threw %q, we did not", name, ri, r.Err)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s run %d: %v", name, ri, err)
				continue
			}
			worst := 0.0
			for _, want := range r.Out {
				got := res.Rects[idx[want.ID]]
				for _, d := range [4]float64{got.X - want.X, got.Y - want.Y, got.W - want.W, got.H - want.H} {
					worst = math.Max(worst, math.Abs(d))
				}
			}
			switch {
			case worst == 0:
				exact++
			case worst < 1e-6:
				close++
				t.Logf("%s run %d: off by %g", name, ri, worst)
			default:
				t.Errorf("%s run %d: off by %g", name, ri, worst)
			}
			if res.Iterations != r.Iter || calls != r.Randoms {
				t.Errorf("%s run %d: %d iterations and %d random numbers, fcose %d and %d", name, ri, res.Iterations, calls, r.Iter, r.Randoms)
			}
			if ri == len(c.Runs)-1 && c.Final != nil {
				pos := Recenter(res.Rects, r.BBox.X1+(r.BBox.X2-r.BBox.X1)/2, r.BBox.Y1+(r.BBox.Y2-r.BBox.Y1)/2)
				for _, f := range c.Final {
					if f.Parent {
						continue
					}
					p := pos[idx[f.ID]]
					if math.Abs(p[0]-f.X) > 1e-9 || math.Abs(p[1]-f.Y) > 1e-9 {
						t.Errorf("%s: %s at %v, mermaid %v", name, f.ID, p, [2]float64{f.X, f.Y})
					}
				}
			}
		}
	}
	t.Logf("%d runs: %d exact, %d within 1e-6", runs, exact, close)
}

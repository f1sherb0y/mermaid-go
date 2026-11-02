package dagre

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/f1sherb0y/mermaid-go/graph"
)

// The reference data comes from dagre-d3-es itself: tools/dagre/gen.mjs.

// num is a JSON number that may also be "NaN", "Infinity" or "-Infinity".
type num float64

func (n *num) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		s, err := strconv.Unquote(string(b))
		if err != nil {
			return err
		}
		f, err := strconv.ParseFloat(s, 64) // accepts NaN, Infinity
		*n = num(f)
		return err
	}
	f, err := strconv.ParseFloat(string(b), 64)
	*n = num(f)
	return err
}

type layoutCase struct {
	Name       string
	Multigraph bool
	Compound   bool
	Opts       struct {
		RankDir                   *string
		NodeSep, EdgeSep, RankSep *num
		MarginX, MarginY          *num
		Ranker, Acyclicer, Align  *string
	}
	Nodes []struct {
		ID            string
		Width, Height num
		Parent        *string
		NoLabel       bool
	}
	Edges []struct {
		V, W, Name    string
		MinLen        *int
		Weight        *num
		Width, Height *num
		LabelPos      *string
		LabelOffset   *num
	}
	Throws string
	Want   struct {
		IDStart       int
		Width, Height num
		Nodes         map[string][4]num
		Edges         []struct {
			V, W, Name string
			Points     []num
			XY         []num
		}
	}
}

func loadCases(t testing.TB, file string) []layoutCase {
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []layoutCase
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// input builds the case's graph the way ref.mjs does.
func (c *layoutCase) input() (*Graph, Options) {
	g := graph.New[*Node, *Edge](graph.Options{Multigraph: c.Multigraph, Compound: c.Compound})
	for _, n := range c.Nodes {
		if n.NoLabel {
			g.EnsureNode(n.ID)
		} else {
			g.SetNode(n.ID, &Node{Width: float64(n.Width), Height: float64(n.Height)})
		}
		if n.Parent != nil {
			g.SetParent(n.ID, *n.Parent)
		}
	}
	for _, e := range c.Edges {
		l := NewEdge()
		if e.MinLen != nil {
			l.MinLen = *e.MinLen
		}
		setNum(&l.Weight, e.Weight)
		setNum(&l.Width, e.Width)
		setNum(&l.Height, e.Height)
		setNum(&l.LabelOffset, e.LabelOffset)
		if e.LabelPos != nil {
			l.LabelPos = *e.LabelPos
		}
		g.SetEdge(graph.Edge{V: e.V, W: e.W, Name: e.Name}, l)
	}
	o := DefaultOptions()
	if c.Opts.RankDir != nil {
		o.RankDir = *c.Opts.RankDir
	}
	setNum(&o.NodeSep, c.Opts.NodeSep)
	setNum(&o.EdgeSep, c.Opts.EdgeSep)
	setNum(&o.RankSep, c.Opts.RankSep)
	setNum(&o.MarginX, c.Opts.MarginX)
	setNum(&o.MarginY, c.Opts.MarginY)
	for _, s := range []struct{ dst, src *string }{{&o.Ranker, c.Opts.Ranker}, {&o.Acyclicer, c.Opts.Acyclicer}, {&o.Align, c.Opts.Align}} {
		if s.src != nil {
			*s.dst = *s.src
		}
	}
	return g, o
}

func setNum(dst *float64, src *num) {
	if src != nil {
		*dst = float64(*src)
	}
}

// The task allows 1e-9 for float formatting; JSON round-trips float64
// exactly, so any difference at all would be a bug.
const tolerance = 1e-9

type checker struct {
	t       testing.TB
	maxDiff float64
}

func (ck *checker) eq(what string, got float64, want num) {
	ck.t.Helper()
	w := float64(want)
	if math.IsNaN(got) && math.IsNaN(w) || got == w {
		return
	}
	d := math.Abs(got - w)
	if math.IsNaN(d) || d > tolerance {
		ck.t.Errorf("%s = %v, want %v", what, got, w)
		return
	}
	ck.maxDiff = max(ck.maxDiff, d)
}

func (c *layoutCase) check(t testing.TB) float64 {
	g, opt := c.input()
	width, height, err := layoutFrom(g, opt, c.Want.IDStart)
	if c.Throws != "" {
		if err == nil {
			t.Errorf("dagre throws %q, Layout returned no error", c.Throws)
		}
		return 0
	}
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	ck := &checker{t: t}
	ck.eq("width", width, c.Want.Width)
	ck.eq("height", height, c.Want.Height)
	labeled := 0
	g.EachNode(func(v string, n *Node) {
		if n == nil {
			return
		}
		labeled++
		w, ok := c.Want.Nodes[v]
		if !ok {
			t.Errorf("node %q not in dagre's output", v)
			return
		}
		ck.eq(v+".x", n.X, w[0])
		ck.eq(v+".y", n.Y, w[1])
		ck.eq(v+".width", n.Width, w[2])
		ck.eq(v+".height", n.Height, w[3])
	})
	if labeled != len(c.Want.Nodes) {
		t.Errorf("%d labeled nodes, dagre has %d", labeled, len(c.Want.Nodes))
	}
	edges := g.Edges()
	if len(edges) != len(c.Want.Edges) {
		t.Fatalf("%d edges, dagre has %d", len(edges), len(c.Want.Edges))
	}
	for i, e := range edges {
		w := c.Want.Edges[i]
		if e != (graph.Edge{V: w.V, W: w.W, Name: w.Name}) {
			t.Errorf("edge %d is %v, dagre has %v", i, e, w)
			continue
		}
		l := g.Edge(e)
		name := fmt.Sprintf("edge %s->%s %s", e.V, e.W, e.Name)
		if len(l.Points)*2 != len(w.Points) {
			t.Errorf("%s: %d points, dagre has %d", name, len(l.Points), len(w.Points)/2)
			continue
		}
		for j, p := range l.Points {
			ck.eq(fmt.Sprintf("%s point %d x", name, j), p.X, w.Points[2*j])
			ck.eq(fmt.Sprintf("%s point %d y", name, j), p.Y, w.Points[2*j+1])
		}
		if l.HasXY != (w.XY != nil) {
			t.Errorf("%s: has label position %v, dagre %v", name, l.HasXY, w.XY != nil)
		} else if l.HasXY {
			ck.eq(name+" x", l.X, w.XY[0])
			ck.eq(name+" y", l.Y, w.XY[1])
		}
	}
	return ck.maxDiff
}

func TestLayout(t *testing.T) {
	cases := loadCases(t, "testdata/layouts.json.gz")
	cases = append(cases, loadCases(t, "testdata/bench.json.gz")...)
	maxDiff, throws := 0.0, 0
	for i := range cases {
		c := &cases[i]
		if c.Throws != "" {
			throws++
		}
		t.Run(c.Name, func(t *testing.T) { maxDiff = max(maxDiff, c.check(t)) })
	}
	t.Logf("%d layouts (%d where dagre throws), largest difference %g", len(cases), throws, maxDiff)
}

func TestJSSort(t *testing.T) {
	f, err := os.Open("testdata/jssort.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Bias  bool
		BC    []num
		Order []int
	}
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	type entry struct {
		b float64
		i int
	}
	for k, c := range cases {
		es := make([]entry, len(c.BC))
		for i, b := range c.BC {
			es[i] = entry{float64(b), i}
		}
		jsSort(es, func(x, y entry) int {
			switch {
			case x.b < y.b:
				return -1
			case x.b > y.b:
				return 1
			case c.Bias:
				return y.i - x.i
			default:
				return x.i - y.i
			}
		})
		for i, e := range es {
			if e.i != c.Order[i] {
				t.Errorf("case %d (n=%d): position %d holds %d, V8 has %d", k, len(es), i, e.i, c.Order[i])
				break
			}
		}
	}
}

func BenchmarkLayout(b *testing.B) {
	for _, c := range loadCases(b, "testdata/bench.json.gz") {
		b.Run(c.Name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				g, opt := c.input()
				b.StartTimer()
				if _, _, err := Layout(g, opt); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

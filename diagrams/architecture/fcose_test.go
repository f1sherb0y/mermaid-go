package architecture

import (
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/f1sherb0y/mermaid-go/config"
)

// TestFcoseMatchesMermaid lays out every diagram captured in mermaid.live's
// Chrome (fcose's testdata) with mermaid's graph and constraints, built
// here, and compares the services' and junctions' positions with
// cytoscape's final ones, up to where the drawing sits.
func TestFcoseMatchesMermaid(t *testing.T) {
	f, err := os.Open("../../fcose/testdata/captures.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var caps map[string]struct {
		Src   string `json:"src"`
		Err   string `json:"err"`
		Final []struct {
			ID     string  `json:"id"`
			Parent bool    `json:"parent"`
			X      float64 `json:"x"`
			Y      float64 `json:"y"`
		} `json:"final"`
	}
	if err := json.NewDecoder(z).Decode(&caps); err != nil {
		t.Fatal(err)
	}
	ctx := context(t)
	names := make([]string, 0, len(caps))
	for k := range caps {
		names = append(names, k)
	}
	sort.Strings(names)
	exact, close, failed := 0, 0, 0
	for _, name := range names {
		c := caps[name]
		doc, err := config.Preprocess(c.Src)
		if err != nil {
			t.Fatal(err)
		}
		d, err := Parse(doc.Code)
		if err != nil {
			t.Fatal(name, err)
		}
		l := newLayout(d, ctx)
		err = l.placeFcose(0)
		if c.Err != "" {
			if err == nil {
				t.Errorf("%s: mermaid fails (%s), fcose did not", name, c.Err)
			} else {
				failed++
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		byID := map[string]*Node{}
		for _, n := range d.Nodes {
			byID[n.ID] = n
		}
		// Up to a translation: the first node's offset.
		var dx, dy float64
		first, worst := true, 0.0
		for _, fn := range c.Final {
			n := byID[fn.ID]
			if n == nil {
				continue
			}
			p := l.at[n]
			if first {
				dx, dy, first = fn.X-p.X, fn.Y-p.Y, false
			}
			worst = math.Max(worst, math.Max(math.Abs(p.X+dx-fn.X), math.Abs(p.Y+dy-fn.Y)))
		}
		switch {
		case worst < 1e-9:
			exact++
		case worst < 1e-6:
			close++
		default:
			t.Errorf("%s: off by %g", name, worst)
		}
	}
	t.Logf("%d diagrams: %d within 1e-9, %d within 1e-6, %d fail as in mermaid", len(names), exact, close, failed)
}

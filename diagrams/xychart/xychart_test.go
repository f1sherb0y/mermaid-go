package xychart_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/f1sherb0y/mermaid-go"
	"github.com/f1sherb0y/mermaid-go/diagrams/xychart"
	"github.com/f1sherb0y/mermaid-go/text"
)

func TestParse(t *testing.T) {
	c, err := xychart.Parse(`xychart-beta
  title Sales Revenue 2024
  x-axis "Month" [jan, "feb", mar]
  y-axis "Revenue (in $)" 4000 --> 11000
  bar [5000, 6000, 7500, 8200]
  line "trend" [5000 "start", 6000, 7500]`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "Sales Revenue 2024" || c.X.Title != "Month" || strings.Join(c.X.Categories, ",") != "jan,feb,mar" {
		t.Errorf("title/x = %q %+v", c.Title, c.X)
	}
	if c.Y.Band || c.Y.Min != 4000 || c.Y.Max != 11000 || c.Y.Title != "Revenue (in $)" {
		t.Errorf("y = %+v", c.Y)
	}
	// Values past the categories are dropped.
	if len(c.Plots) != 2 || !c.Plots[0].Bar || len(c.Plots[0].Points) != 3 || c.Plots[0].Points[1] != (xychart.Point{X: "feb", Y: 6000}) {
		t.Errorf("plots = %+v", c.Plots)
	}
	if p := c.Plots[1]; p.Title != "trend" || strings.Join(p.Labels, ",") != "start,," {
		t.Errorf("line = %+v", p)
	}

	// Axes left out come from the data: x 1..n from the first plot, y the
	// values' range over all of them.
	c, err = xychart.Parse(`xychart horizontal
  line [1.5, 2.5, 0.3, 4]
  line [9]`)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Horizontal || c.X.Band || c.X.Min != 1 || c.X.Max != 4 || c.Y.Min != 0.3 || c.Y.Max != 9 {
		t.Errorf("axes = %+v %+v", c.X, c.Y)
	}
	if p := c.Plots[0].Points; p[2].X != "3" || p[2].Y != 0.3 {
		t.Errorf("points = %+v", p)
	}
	if _, err := xychart.Parse("xychart\n title t"); err == nil {
		t.Error("a chart without plots parsed")
	}
}

// TestCorpus renders every xychart of the corpus.
func TestCorpus(t *testing.T) { renderCorpus(t, "xyChart", "xychart") }

func renderCorpus(t *testing.T, dir, keyword string) {
	fs := text.NewFonts()
	if err := fs.AddFile("Recursive Variable", "../../../../data/ref/fonts/RecursiveSansLinear-VF.ttf"); err != nil {
		t.Skip("no fonts:", err)
	}
	fs.AddFile("trebuchet ms", "C:/Windows/Fonts/trebuc.ttf")
	files, _ := filepath.Glob(filepath.Join("../../testdata/corpus", dir, "*.mmd"))
	examples, _ := filepath.Glob("../../testdata/corpus/examples/*.mmd")
	for _, f := range examples {
		if b, _ := os.ReadFile(f); strings.HasPrefix(strings.TrimSpace(string(b)), keyword) {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		t.Fatal("no corpus")
	}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		s, err := mermaid.Render(string(b), mermaid.Options{Fonts: fs})
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if len(s.Items) == 0 || !(s.ViewBox.W > 0 && s.ViewBox.H > 0) || strings.Contains(s.SVG(), "NaN") {
			t.Errorf("%s: empty or broken drawing (view box %+v)", f, s.ViewBox)
		}
	}
}

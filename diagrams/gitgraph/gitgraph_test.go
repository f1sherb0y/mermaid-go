package gitgraph_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/f1sherb0y/mermaid-go"
	"github.com/f1sherb0y/mermaid-go/diagrams/gitgraph"
	"github.com/f1sherb0y/mermaid-go/text"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/datatest"
)

func TestParse(t *testing.T) {
	g, err := gitgraph.Parse(`gitGraph TB:
  commit id: "1"
  branch nice_feature order: 2
  checkout nice_feature
  commit id: "2" tag: "v1" type: HIGHLIGHT
  checkout main
  commit "a message"
  merge nice_feature id: "m" type: REVERSE tag: "t"
  branch fix/x.y
  commit id: "3"
  switch nice_feature
  cherry-pick id: "3"`, "main", 0)
	if err != nil {
		t.Fatal(err)
	}
	if g.Dir != "TB" || strings.Join(g.Branches, ",") != "main,fix/x.y,nice_feature" {
		t.Errorf("dir %s, branches %v", g.Dir, g.Branches)
	}
	if len(g.Commits) != 6 {
		t.Fatalf("%d commits", len(g.Commits))
	}
	if c := g.Commits[1]; c.Symbol != gitgraph.Highlight || c.Branch != "nice_feature" || strings.Join(c.Tags, ",") != "v1" {
		t.Errorf("commit = %+v", c)
	}
	if m := g.Commits[3]; m.ID != "m" || m.Type != gitgraph.Merge || m.Symbol != gitgraph.Reverse || !m.CustomID ||
		strings.Join(m.Parents, ",") != g.Commits[2].ID+",2" {
		t.Errorf("merge = %+v", m)
	}
	if c := g.Commits[5]; c.Type != gitgraph.CherryPick || strings.Join(c.Tags, ",") != "cherry-pick:3" || c.Parents[1] != "3" {
		t.Errorf("cherry-pick = %+v", c)
	}

	// Branches without an order sort at 0.<their index>.
	g, err = gitgraph.Parse(`gitGraph
  commit
  branch test1 order: 3
  branch test2
  branch test3
  branch test4 order: 1`, "main", 2)
	if err != nil {
		t.Fatal(err)
	}
	if b := strings.Join(g.Branches, ","); b != "test2,test3,test4,main,test1" {
		t.Errorf("branches = %s", b)
	}

	for _, bad := range []string{
		"gitGraph\n commit\n merge main",                      // into itself
		"gitGraph\n checkout nope",                            // no such branch
		"gitGraph\n branch main",                              // exists
		"gitGraph\n commit type: BOLD",                        // no such type
		"gitGraph\n commit id: \"a\"\n cherry-pick id: \"a\"", // already on this branch
	} {
		if _, err := gitgraph.Parse(bad, "main", 0); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

// TestCorpus renders every gitGraph of the corpus.
func TestCorpus(t *testing.T) { renderCorpus(t, "gitgraph", "gitGraph") }

func renderCorpus(t *testing.T, dir, keyword string) {
	fs := text.NewFonts()
	if err := fs.AddFile("Recursive Variable", datatest.Data("ref/fonts/RecursiveSansLinear-VF.ttf")); err != nil {
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

package journey_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/f1sherb0y/mermaid-go"
	"github.com/f1sherb0y/mermaid-go/diagrams/journey"
	"github.com/f1sherb0y/mermaid-go/text"
)

func TestParse(t *testing.T) {
	j, err := journey.Parse(`journey
    title My working day
    section Go to work
      Make tea: 5: Me
      Do work: 1: Me, Cat
    section Go home
      Sit down: 5`)
	if err != nil {
		t.Fatal(err)
	}
	if j.Title != "My working day" || len(j.Tasks) != 3 {
		t.Fatalf("got %+v", j)
	}
	if w := j.Tasks[1]; strings.TrimSpace(w.Name) != "Do work" || w.Section != "Go to work" || w.Score != 1 || strings.Join(w.People, "|") != "Me|Cat" {
		t.Errorf("task = %+v", w)
	}
	if w := j.Tasks[2]; w.Section != "Go home" || len(w.People) != 0 {
		t.Errorf("task = %+v", w)
	}
	if a := strings.Join(j.Actors(), ","); a != "Cat,Me" {
		t.Errorf("actors = %s", a)
	}
	if _, err := journey.Parse("journey\n  no data here"); err == nil {
		t.Error("a task without data parsed")
	}
}

// TestCorpus renders every user journey of the corpus.
func TestCorpus(t *testing.T) { renderCorpus(t, "userJourney", "journey") }

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

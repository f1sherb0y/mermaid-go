package sequence_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/f1sherb0y/mermaid-go"
	_ "github.com/f1sherb0y/mermaid-go/diagrams/sequence"
	"github.com/f1sherb0y/mermaid-go/text"
)

// cases are the sequence diagrams of mermaid's docs (testdata/corpus) and
// this package's own (testdata), by the name viewboxes.json uses.
func cases(t *testing.T) map[string]string {
	out := map[string]string{}
	add := func(glob, prefix string) {
		files, _ := filepath.Glob(glob)
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			out[prefix+strings.TrimSuffix(filepath.Base(f), ".mmd")] = string(b)
		}
	}
	add("../../testdata/corpus/sequenceDiagram/*.mmd", "sequenceDiagram/")
	for _, n := range []string{"03", "06", "07", "08"} {
		add("../../testdata/corpus/examples/"+n+".mmd", "examples/")
	}
	add("testdata/*.mmd", "")
	if len(out) < 50 {
		t.Fatalf("only %d cases", len(out))
	}
	return out
}

// fonts has Recursive, what mermaid.live draws with, also standing in for
// families not registered (the other themes'); nil without it.
func fonts() *text.Fonts {
	fs := text.NewFonts()
	if fs.AddFile("Recursive Variable", "../../../../data/ref/fonts/RecursiveSansLinear-VF.ttf") != nil {
		return nil
	}
	fs.AddFile("trebuchet ms", "C:/Windows/Fonts/trebuc.ttf")
	fs.Fallback("Recursive Variable")
	return fs
}

// TestRender renders every case; with Recursive, the view box must be
// close to mermaid's (viewboxes.json, from its SVGs). mermaid measures
// sequence text in the page's default font (its default font family is not
// valid CSS) and we measure the font drawn, so sizes differ a little:
// widths within 5%, heights within 10%.
func TestRender(t *testing.T) {
	fs := fonts()
	exact := fs != nil
	if !exact {
		fs = text.NewFonts()
		t.Log("no reference fonts: only checking that everything renders")
	}
	var want map[string][4]float64
	b, err := os.ReadFile("testdata/viewboxes.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	for name, src := range cases(t) {
		s, err := mermaid.Render(src, mermaid.Options{Fonts: fs})
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if s.SVG() == "" || len(s.Items) == 0 {
			t.Errorf("%s: nothing drawn", name)
		}
		w, ok := want[name]
		// 35's messages have entities (#9829;), which mermaid measures in
		// their placeholder form, wider; we measure the characters.
		if !exact || !ok || name == "sequenceDiagram/35" {
			continue
		}
		vb := s.ViewBox
		if math.Abs(vb.W-w[2]) > 0.05*w[2] || math.Abs(vb.H-w[3]) > 0.1*w[3] {
			t.Errorf("%s: view box %v, mermaid's %v", name, vb, w)
		}
	}
}

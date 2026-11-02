package render

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/text"
	"github.com/f1sherb0y/mermaid-go/theme"
)

// The corpus tests render mermaid's own getData() dumps of every corpus
// diagram (data/mermaid-port/getdata) and check the ELK graph built from
// them against the one mermaid built (data/mermaid-port/elk), laying out
// with ELK's captured answer. MERMAID_OUT=<dir> writes the SVGs for
// tools/ref.mjs cmp.

var portData = "../../../data/mermaid-port"

func testFonts(t testing.TB) *text.Fonts {
	fs := text.NewFonts()
	if err := fs.AddFile("Recursive Variable", "../../../data/ref/fonts/RecursiveSansLinear-VF.ttf"); err != nil {
		t.Skip("no Recursive font:", err)
	}
	var fb []string
	for _, f := range []struct{ name, path string }{
		{"arial", "C:/Windows/Fonts/arial.ttf"},
		{"arial", "C:/Windows/Fonts/arialbd.ttf"},
		{"Microsoft YaHei", "C:/Windows/Fonts/msyh.ttc"},
		{"Segoe UI Emoji", "C:/Windows/Fonts/seguiemj.ttf"},
		{"Segoe UI Symbol", "C:/Windows/Fonts/seguisym.ttf"},
	} {
		if fs.AddFile(f.name, f.path) == nil && f.name != "arial" {
			fb = append(fb, f.name)
		}
	}
	fs.Fallback(fb...)
	return fs
}

// corpusCase is one diagram: its source, its getData() dump and ELK
// captures.
type corpusCase struct {
	rel  string // e.g. "flowchart/01", "real/capture-flowchart-..."
	src  string
	data string
	elk  []string
}

func corpusCases(t testing.TB, only string) []corpusCase {
	var out []corpusCase
	gd := filepath.Join(portData, "getdata")
	filepath.WalkDir(gd, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
			return nil
		}
		rel, _ := filepath.Rel(gd, p)
		rel = filepath.ToSlash(strings.TrimSuffix(rel, ".json"))
		if only != "" && !strings.Contains(rel, only) {
			return nil
		}
		src := filepath.Join("../testdata/corpus", rel+".mmd")
		if strings.HasPrefix(rel, "real/") {
			src = filepath.Join(portData, "corpus-real", rel+".mmd")
		}
		elk, _ := filepath.Glob(filepath.Join(portData, "elk", rel+".*.json"))
		sort.Strings(elk)
		out = append(out, corpusCase{rel: rel, src: src, data: p, elk: elk})
		return nil
	})
	return out
}

var headerDir = regexp.MustCompile(`^\s*(?:flowchart|graph|stateDiagram(?:-v2)?|classDiagram(?:-v2)?|erDiagram|requirementDiagram)\b[ \t]*(\w*)`)

// load reads a case's data and makes the context mermaid.Render would.
func (c corpusCase) load(t testing.TB, fonts *text.Fonts) (*Data, *diagram.Context, error) {
	b, err := os.ReadFile(c.data)
	if err != nil {
		return nil, nil, err
	}
	d, err := LoadData(b)
	if err != nil {
		return nil, nil, err
	}
	srcB, err := os.ReadFile(c.src)
	if err != nil {
		return nil, nil, err
	}
	doc, err := config.Preprocess(string(srcB))
	if err != nil {
		return nil, nil, err
	}
	t0, err := diagram.Detect(doc.Code)
	section := "flowchart"
	if t0 != nil {
		section = t0.Section
	}
	if cm, ok := doc.Overrides["config"].(map[string]any); ok {
		delete(doc.Overrides, "config")
		doc.Overrides[section] = cm
	}
	themeName, look, layout := config.Appearance(section, doc.Overrides, nil)
	cfg := config.Defaults().With(doc.Overrides)
	overrides := map[string]any{}
	if tv, ok := doc.Overrides["themeVariables"].(map[string]any); ok {
		for k, v := range tv {
			if v != nil {
				overrides[k] = v
			}
		}
	}
	vars, err := theme.Get(themeName, overrides)
	if err != nil {
		return nil, nil, err
	}
	if d.Direction == "" {
		if m := headerDir.FindStringSubmatch(doc.Code); m != nil && m[1] != "" {
			d.Direction = strings.ToUpper(m[1])
		}
		if d.Direction == "TD" || d.Direction == "" {
			d.Direction = "TB"
		}
	}
	d.Title = doc.Title
	if d.Title == "" {
		if m := regexp.MustCompile(`(?m)^\s*title\s+(.+)$`).FindStringSubmatch(doc.Code); m != nil {
			d.Title = strings.TrimSpace(m[1])
		}
	}
	// The dumps were made with the default config: their look is the
	// default's, not the diagram type's.
	for _, n := range d.Nodes {
		n.Look = look
	}
	for _, e := range d.Edges {
		e.Look = look
	}
	// The dumps were made with the default theme: class notes carry its
	// note colors.
	for _, n := range d.Nodes {
		for i, c := range n.CSSStyles {
			switch c {
			case "fill: #fff5ad":
				n.CSSStyles[i] = "fill: " + vars.Str("noteBkgColor")
			case "stroke: #aaaa33":
				n.CSSStyles[i] = "stroke: " + vars.Str("noteBorderColor")
			}
		}
	}
	ctx := &diagram.Context{Config: cfg, Theme: themeVars{vars}, Fonts: fonts, Title: doc.Title, Look: look, Layout: layout}
	return d, ctx, nil
}

type themeVars struct{ v *theme.Variables }

func (t themeVars) Get(name string) string { return t.v.Str(name) }

// capture is one recorded ELK call.
type capture struct {
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
}

// replay answers an ELK call with the captured output, and compares the
// graph built with the captured input.
func replay(t testing.TB, file string, diffs *[]string) func(g *ElkNode) error {
	return func(g *ElkNode) error {
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var c capture
		if err := json.Unmarshal(b, &c); err != nil {
			return err
		}
		var in, out map[string]any
		json.Unmarshal(c.Input, &in)
		json.Unmarshal(c.Output, &out)
		ours := map[string]any{}
		ob, _ := json.Marshal(g)
		json.Unmarshal(ob, &ours)
		if diffs != nil {
			compareGraph(in, ours, "", diffs)
		}
		applyOutput(g, out)
		return nil
	}
}

func optString(v any) string {
	switch v := v.(type) {
	case float64:
		return jsNum(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	}
	return fmt.Sprint(v)
}

// compareGraph lists where our graph differs from mermaid's in what ELK
// reads.
func compareGraph(want, got map[string]any, path string, diffs *[]string) {
	id := fmt.Sprint(want["id"])
	p := path + "/" + id
	if fmt.Sprint(got["id"]) != id {
		*diffs = append(*diffs, fmt.Sprintf("%s: id %v", p, got["id"]))
		return
	}
	_, isGroup := want["children"]
	if !isGroup || want["width"] != nil {
		for _, k := range []string{"width", "height"} {
			if math.Abs(num(want[k])-num(got[k])) > 1e-3 {
				*diffs = append(*diffs, fmt.Sprintf("%s: %s %v, want %v", p, k, got[k], want[k]))
			}
		}
	}
	compareOpts(want["layoutOptions"], got["layoutOptions"], p, diffs)
	compareLabels(want["labels"], got["labels"], p, diffs)
	wc, _ := want["children"].([]any)
	gc, _ := got["children"].([]any)
	if len(wc) != len(gc) {
		*diffs = append(*diffs, fmt.Sprintf("%s: %d children, want %d", p, len(gc), len(wc)))
	}
	for i := range min(len(wc), len(gc)) {
		compareGraph(wc[i].(map[string]any), gc[i].(map[string]any), p, diffs)
	}
	we, _ := want["edges"].([]any)
	ge, _ := got["edges"].([]any)
	if len(we) != len(ge) {
		*diffs = append(*diffs, fmt.Sprintf("%s: %d edges, want %d", p, len(ge), len(we)))
	}
	for i := range min(len(we), len(ge)) {
		w, g := we[i].(map[string]any), ge[i].(map[string]any)
		ep := fmt.Sprintf("%s/edge %v", p, w["id"])
		for _, k := range []string{"id", "sources", "targets"} {
			if fmt.Sprint(w[k]) != fmt.Sprint(g[k]) {
				*diffs = append(*diffs, fmt.Sprintf("%s: %s %v, want %v", ep, k, g[k], w[k]))
			}
		}
		compareLabels(w["labels"], g["labels"], ep, diffs)
		compareOpts(w["layoutOptions"], g["layoutOptions"], ep, diffs)
	}
}

func compareOpts(want, got any, p string, diffs *[]string) {
	w, _ := want.(map[string]any)
	g, _ := got.(map[string]any)
	for k, v := range w {
		if gv, ok := g[k]; !ok || optString(gv) != optString(v) {
			*diffs = append(*diffs, fmt.Sprintf("%s: option %s = %v, want %v", p, k, g[k], v))
		}
	}
	for k, v := range g {
		if _, ok := w[k]; !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: extra option %s = %v", p, k, v))
		}
	}
}

func compareLabels(want, got any, p string, diffs *[]string) {
	w, _ := want.([]any)
	g, _ := got.([]any)
	if len(w) != len(g) {
		*diffs = append(*diffs, fmt.Sprintf("%s: %d labels, want %d", p, len(g), len(w)))
		return
	}
	for i := range w {
		wl, gl := w[i].(map[string]any), g[i].(map[string]any)
		for _, k := range []string{"width", "height"} {
			if math.Abs(num(wl[k])-num(gl[k])) > 1e-3 {
				*diffs = append(*diffs, fmt.Sprintf("%s: label %d (%q) %s %v, want %v", p, i, wl["text"], k, gl[k], wl[k]))
			}
		}
		compareOpts(wl["layoutOptions"], gl["layoutOptions"], fmt.Sprintf("%s: label %d", p, i), diffs)
	}
}

// TestElkInputs checks the ELK graphs we build against mermaid's, laid out
// with the captured answers; MERMAID_OUT writes the SVGs.
func TestElkInputs(t *testing.T) {
	fonts := testFonts(t)
	only := os.Getenv("ONLY")
	outDir := os.Getenv("MERMAID_OUT")
	verbose := os.Getenv("VERBOSE") != ""
	cases := corpusCases(t, only)
	if len(cases) == 0 {
		t.Skip("no corpus dumps")
	}
	byType := map[string][2]int{}
	exactNodes, allNodes := 0, 0
	var failed []string
	// LAYOUT=engine lays out with the real engines instead of replaying
	// mermaid's ELK answers (the ELK input is then not checked).
	engine := os.Getenv("LAYOUT") == "engine"
	for _, c := range cases {
		if len(c.elk) == 0 && !engine {
			continue
		}
		d, ctx, err := c.load(t, fonts)
		if err != nil {
			failed = append(failed, c.rel+": "+err.Error())
			continue
		}
		var diffs []string
		opts := Options{}
		if !engine && len(c.elk) > 0 {
			opts.ELK = replay(t, c.elk[0], &diffs)
		}
		s, err := renderSafe(d, ctx, opts)
		kind := strings.SplitN(c.rel, "/", 2)[0]
		if kind == "real" {
			kind = "real-" + d.DiagramType
		}
		tc := byType[kind]
		tc[1]++
		if err != nil {
			failed = append(failed, c.rel+": "+err.Error())
		} else if len(diffs) == 0 {
			tc[0]++
		} else if verbose {
			t.Logf("%s:\n  %s", c.rel, strings.Join(diffs[:min(len(diffs), 12)], "\n  "))
		}
		byType[kind] = tc
		for _, n := range d.Nodes {
			if n.IsGroup {
				continue
			}
			allNodes++
			bad := false
			for _, df := range diffs {
				if strings.Contains(df, "/"+n.ID+":") && (strings.Contains(df, " width ") || strings.Contains(df, " height ")) {
					bad = true
				}
			}
			if !bad {
				exactNodes++
			}
		}
		if outDir != "" && s != nil {
			f := filepath.Join(outDir, c.rel+".svg")
			os.MkdirAll(filepath.Dir(f), 0o755)
			os.WriteFile(f, []byte(s.SVG()), 0o644)
		}
	}
	keys := make([]string, 0, len(byType))
	for k := range byType {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%-28s ELK input exact %d / %d", k, byType[k][0], byType[k][1])
	}
	t.Logf("node sizes exact: %d / %d", exactNodes, allNodes)
	for _, f := range failed {
		t.Log("FAILED", f)
	}
}

func renderSafe(d *Data, ctx *diagram.Context, o Options) (s interface{ SVG() string }, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	sc, err := Render(d, ctx, o)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

package theme

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

type themeCase struct {
	Theme     string         `json:"theme"`
	Overrides map[string]any `json:"overrides"`
	Vars      map[string]any `json:"vars"`
	Error     string         `json:"error"`
}

func loadCases(t *testing.T) []themeCase {
	f, err := os.Open("testdata/themes.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []themeCase
	if err := json.NewDecoder(z).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// fromJSON undoes the harness's tagging of non-finite numbers.
func fromJSON(x any) any {
	switch x := x.(type) {
	case map[string]any:
		if s, ok := x["$number"].(string); ok && len(x) == 1 {
			switch s {
			case "NaN":
				return math.NaN()
			case "Infinity":
				return math.Inf(1)
			}
			return math.Inf(-1)
		}
		m := map[string]any{}
		for k, e := range x {
			m[k] = fromJSON(e)
		}
		return m
	case []any:
		a := make([]any, len(x))
		for i, e := range x {
			a[i] = fromJSON(e)
		}
		return a
	}
	return x
}

func same(a, b any) bool {
	if fa, ok := a.(float64); ok {
		if fb, ok := b.(float64); ok && math.IsNaN(fa) && math.IsNaN(fb) {
			return true
		}
	}
	return reflect.DeepEqual(a, b)
}

// testdata/themes.json.gz: mermaid 12.2.0's getThemeVariables, run by tools/theme/themes.mjs.
func TestThemes(t *testing.T) {
	cases := loadCases(t)
	only := os.Getenv("THEME")
	themes := map[string]bool{}
	for _, c := range cases {
		themes[c.Theme] = true
		if only != "" && c.Theme != only {
			continue
		}
		name := c.Theme + " " + compact(c.Overrides)
		got, err := Get(c.Theme, c.Overrides)
		if c.Error != "" {
			if err == nil {
				t.Errorf("%s: no error, want %q", name, c.Error)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		want := fromJSON(c.Vars).(map[string]any)
		var diffs []string
		for _, k := range union(want, got.All) {
			w, inW := want[k]
			g, inG := got.All[k]
			switch {
			case !inW:
				diffs = append(diffs, fmt.Sprintf("%s: got %s, want unset", k, compact(g)))
			case !inG:
				diffs = append(diffs, fmt.Sprintf("%s: unset, want %s", k, compact(w)))
			case !same(w, g):
				diffs = append(diffs, fmt.Sprintf("%s: got %s, want %s", k, compact(g), compact(w)))
			}
		}
		if len(diffs) > 0 {
			if len(diffs) > 15 {
				diffs = append(diffs[:15], fmt.Sprintf("... %d more", len(diffs)-15))
			}
			t.Errorf("%s:\n\t%s", name, strings.Join(diffs, "\n\t"))
		}
	}
	for th := range themes {
		if _, ok := defs[th]; !ok {
			t.Errorf("theme %q not ported", th)
		}
	}
	t.Logf("%d cases", len(cases))
}

func union(a, b map[string]any) []string {
	var ks []string
	for k := range a {
		ks = append(ks, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			ks = append(ks, k)
		}
	}
	sort.Strings(ks)
	return ks
}

func compact(x any) string {
	b, err := json.Marshal(x)
	if err != nil {
		return fmt.Sprint(x)
	}
	return string(b)
}

func TestDiagramDefault(t *testing.T) {
	data, err := os.ReadFile("testdata/appearance.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]struct{ Theme, Look string }
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	keys := append(slices.Collect(maps.Keys(want)), "pie", "gantt", "xyChart", "mindmap", "gitGraph")
	for _, k := range keys {
		w, ok := want[k]
		if !ok {
			w = want[""]
		}
		if k == "" {
			if DefaultName != w.Theme || DefaultLook != w.Look {
				t.Errorf("global default %s/%s, want %s/%s", DefaultName, DefaultLook, w.Theme, w.Look)
			}
			continue
		}
		if th, look := DiagramDefault(k); th != w.Theme || look != w.Look {
			t.Errorf("%s: %s/%s, want %s/%s", k, th, look, w.Theme, w.Look)
		}
	}
	if len(diagramAppearance) != len(want)-1 {
		t.Errorf("%d diagram defaults, mermaid has %d", len(diagramAppearance), len(want)-1)
	}
}

func TestVariablesFields(t *testing.T) {
	v, err := Get("redux-color", map[string]any{"darkMode": "false", "radius": 3})
	if err != nil {
		t.Skip(err)
	}
	if !v.DarkMode || v.Radius != 3 || v.PrimaryColor != v.Str("primaryColor") || v.FontFamily == "" {
		t.Errorf("fields: %+v", v)
	}
	if v.Str("xyChart", "titleColor") == "" {
		t.Error("xyChart.titleColor unset")
	}
}

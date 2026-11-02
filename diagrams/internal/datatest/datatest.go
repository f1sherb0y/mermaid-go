// Package datatest compares the render.Data a diagram's parser makes with
// mermaid's own db.getData() dumps (tools/getdata.mjs), on what drawing
// reads, and finds the corpus cases.
package datatest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/render"
)

// Ignored are the fields no drawing reads: DOM ids, links and tooltips,
// the look (the renderer's context decides it; the dumps have the default
// config's), the edge's arrowhead style and label position (constants the
// renderer does not take from data), and bookkeeping.
var Ignored = map[string]bool{
	"domId": true, "look": true, "link": true, "linkTarget": true, "tooltip": true,
	"haveCallback": true, "arrowheadStyle": true, "labelpos": true, "config": true,
}

// strLists are the fields mermaid gives as a string or a list of strings.
var strLists = map[string]bool{"description": true, "style": true, "labelStyle": true}

// Diff lists where got differs from mermaid's dump (the getdata JSON) as
// "category: detail", the category being "nodes", "edges", "node <field>"
// or "edge <field>". Fields in skip are not compared, nor Ignored ones.
//
// The dumps drop an object met twice (the dumper's cycle guard), so a list
// or object field missing from a dump is not compared.
func Diff(got *render.Data, dump []byte, skip ...string) []string {
	var w struct {
		Data struct {
			Nodes []map[string]any `json:"nodes"`
			Edges []map[string]any `json:"edges"`
		} `json:"data"`
	}
	if err := json.Unmarshal(dump, &w); err != nil {
		return []string{"dump: " + err.Error()}
	}
	skipped := map[string]bool{}
	for _, s := range skip {
		skipped[s] = true
	}
	var diffs []string
	gotNodes := make([]map[string]any, len(got.Nodes))
	for i, n := range got.Nodes {
		gotNodes[i] = asMap(n, n.Extra)
	}
	gotEdges := make([]map[string]any, len(got.Edges))
	for i, e := range got.Edges {
		gotEdges[i] = asMap(e, e.Extra)
	}
	diffs = append(diffs, diffList("node", gotNodes, w.Data.Nodes, skipped)...)
	diffs = append(diffs, diffList("edge", gotEdges, w.Data.Edges, skipped)...)
	return diffs
}

func diffList(kind string, got, want []map[string]any, skip map[string]bool) []string {
	var diffs []string
	ids := func(l []map[string]any) []string {
		var out []string
		for _, m := range l {
			out = append(out, fmt.Sprint(m["id"]))
		}
		return out
	}
	gi, wi := ids(got), ids(want)
	if strings.Join(gi, "\x00") != strings.Join(wi, "\x00") {
		diffs = append(diffs, fmt.Sprintf("%ss: %v, want %v", kind, gi, wi))
	}
	// Pair by id, in order: ids can repeat (a requirement and an element
	// of the same name).
	byID := map[string][]map[string]any{}
	for _, m := range got {
		id := fmt.Sprint(m["id"])
		byID[id] = append(byID[id], m)
	}
	for _, wm := range want {
		id := fmt.Sprint(wm["id"])
		if len(byID[id]) == 0 {
			continue
		}
		gm := byID[id][0]
		byID[id] = byID[id][1:]
		keys := map[string]bool{}
		for k := range wm {
			keys[k] = true
		}
		for k := range gm {
			keys[k] = true
		}
		var sorted []string
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			if skip[k] || Ignored[k] {
				continue
			}
			wv, inDump := wm[k]
			gv := gm[k]
			if strLists[k] {
				wv, gv = asList(wv), asList(gv)
			}
			if !inDump {
				switch gv.(type) {
				case []any, map[string]any:
					continue
				}
			}
			if zero(wv) && zero(gv) {
				continue
			}
			if !reflect.DeepEqual(wv, gv) {
				diffs = append(diffs, fmt.Sprintf("%s %s: %s %s = %s, want %s", kind, k, kind, id, js(gv), js(wv)))
			}
		}
	}
	return diffs
}

// asMap is a node's or edge's JSON with its extra fields, as plain values.
func asMap(v any, extra map[string]json.RawMessage) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	json.Unmarshal(b, &m)
	for k, raw := range extra {
		var x any
		json.Unmarshal(raw, &x)
		m[k] = x
	}
	return m
}

func asList(v any) any {
	if s, ok := v.(string); ok {
		if s == "" {
			return nil
		}
		return []any{s}
	}
	return v
}

func zero(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case bool:
		return !v
	case float64:
		return v == 0
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}

func js(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Case is a corpus diagram: its source and mermaid's dump ("" when mermaid
// failed to parse it).
type Case struct {
	Name, Src, Dump string
}

// Cases lists the .mmd files under dirs whose header is one of headers
// (e.g. "flowchart", "graph"), each with the .json dump beside it or, for
// directories under data/mermaid-port/corpus-real, in getdata.
func Cases(dirs []string, headers ...string) []Case {
	var out []Case
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.mmd"))
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil || !hasHeader(string(src), headers) {
				continue
			}
			dump := strings.TrimSuffix(f, ".mmd") + ".json"
			if real := strings.Replace(filepath.ToSlash(dump), "corpus-real/", "getdata/", 1); real != filepath.ToSlash(dump) {
				dump = real
			}
			if _, err := os.Stat(dump); err != nil {
				dump = ""
			}
			name := filepath.Base(filepath.Dir(f)) + "/" + strings.TrimSuffix(filepath.Base(f), ".mmd")
			out = append(out, Case{Name: name, Src: f, Dump: dump})
		}
	}
	return out
}

func hasHeader(src string, headers []string) bool {
	doc, err := config.Preprocess(src)
	if err != nil {
		return false
	}
	first := strings.Fields(doc.Code)
	if len(first) == 0 {
		return false
	}
	for _, h := range headers {
		if first[0] == h || strings.HasPrefix(first[0], h+";") {
			return true
		}
	}
	return false
}

// Parser is a diagram package's Parse.
type Parser func(code string, cfg *config.Config) (*render.Data, error)

// Run parses every case and compares it with its dump: a case without a
// dump must fail to parse. It logs a summary by difference category and
// fails on cases with differences, except those in allowed (case name to
// the categories accepted for it, "*" for all).
func Run(t *testing.T, parse Parser, cases []Case, allowed map[string][]string, skip ...string) {
	t.Helper()
	if len(cases) == 0 {
		t.Skip("no cases")
	}
	var matching, differing, failing int
	cats := map[string]int{}
	for _, c := range cases {
		b, err := os.ReadFile(c.Src)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := config.Preprocess(string(b))
		if err != nil {
			t.Errorf("%s: preprocess: %v", c.Name, err)
			continue
		}
		cfg := config.Defaults().With(doc.Overrides)
		d, err := parse(doc.Code, cfg)
		if c.Dump == "" {
			if err == nil {
				t.Errorf("%s: parsed, but mermaid fails on it", c.Name)
			} else {
				failing++
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			differing++
			cats["parse error"]++
			continue
		}
		dump, err := os.ReadFile(c.Dump)
		if err != nil {
			t.Fatal(err)
		}
		diffs := Diff(d, dump, skip...)
		if len(diffs) == 0 {
			matching++
			continue
		}
		differing++
		seen := map[string]bool{}
		for _, df := range diffs {
			cat, _, _ := strings.Cut(df, ":")
			if !seen[cat] {
				cats[cat]++
				seen[cat] = true
			}
			if !accepted(allowed[c.Name], cat) {
				t.Errorf("%s: %s", c.Name, df)
			}
		}
	}
	var cs []string
	for k, v := range cats {
		cs = append(cs, fmt.Sprintf("%s ×%d", k, v))
	}
	sort.Strings(cs)
	t.Logf("%d files: %d match, %d differ, %d fail as in mermaid; differences: %s",
		len(cases), matching, differing, failing, strings.Join(cs, ", "))
}

func accepted(cats []string, cat string) bool {
	for _, c := range cats {
		if c == "*" || c == cat {
			return true
		}
	}
	return false
}

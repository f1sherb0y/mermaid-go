// Package config is mermaid's configuration: its defaults (defaults.json,
// dumped from mermaid.js by tools/defaults.mjs), what a diagram's front
// matter and %%{init}%% directives set, and the theme, look and layout each
// diagram type ends up with.
package config

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed defaults.json
var defaultsJSON []byte

// Config is a tree of settings, as mermaid's config object.
type Config struct {
	m map[string]any
}

// Defaults is mermaid's default config. It is shared: a Config is never
// changed, and what its getters return must not be changed either.
func Defaults() *Config { return defaults() }

var defaults = sync.OnceValue(func() *Config {
	var m map[string]any
	if err := json.Unmarshal(defaultsJSON, &m); err != nil {
		panic(err)
	}
	return &Config{m}
})

// With is c with overrides merged in (objects merge key by key; anything
// else replaces), as a new Config; c itself when there are none.
func (c *Config) With(overrides map[string]any) *Config {
	if len(overrides) == 0 {
		return c
	}
	return &Config{merge(clone(c.m), overrides)}
}

func merge(dst, src map[string]any) map[string]any {
	for k, v := range src {
		if sv, ok := v.(map[string]any); ok {
			if dv, ok := dst[k].(map[string]any); ok {
				dst[k] = merge(dv, sv)
				continue
			}
			dst[k] = merge(map[string]any{}, sv)
			continue
		}
		dst[k] = v
	}
	return dst
}

func clone(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			v = clone(sub)
		}
		out[k] = v
	}
	return out
}

// Get is the value at a dotted path ("flowchart.nodeSpacing"), or nil.
func (c *Config) Get(path string) any {
	var cur any = c.m
	for _, k := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[k]
	}
	return cur
}

// Num is a number setting (numbers given as strings count), or def.
func (c *Config) Num(path string, def float64) float64 {
	switch v := c.Get(path).(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		var f float64
		if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimSpace(v), "px")), &f); err == nil {
			return f
		}
	}
	return def
}

// Str is a string setting, or def.
func (c *Config) Str(path, def string) string {
	if v, ok := c.Get(path).(string); ok {
		return v
	}
	return def
}

// Bool is a boolean setting, or def. Strings "false", "null" and "0" are
// false, as mermaid's evaluate.
func (c *Config) Bool(path string, def bool) bool {
	switch v := c.Get(path).(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		return s != "false" && s != "null" && s != "0"
	}
	return def
}

// Map is an object setting, or nil.
func (c *Config) Map(path string) map[string]any {
	m, _ := c.Get(path).(map[string]any)
	return m
}

// Appearance is the theme, look and layout a diagram type uses: what the
// diagram itself set (front matter, directives) first, its section's value
// before the top-level one; then the user's (Options); then the type's own
// default; then the global default. section is the type's config key
// ("flowchart", "class", "state", ...).
func Appearance(section string, diagram, user map[string]any) (theme, look, layout string) {
	defaults := Defaults()
	pick := func(key string) string {
		for _, layer := range []map[string]any{diagram, user} {
			if s, ok := layer[section].(map[string]any); ok {
				if v, ok := s[key].(string); ok && v != "" {
					return v
				}
			}
			if v, ok := layer[key].(string); ok && v != "" {
				return v
			}
		}
		if v := defaults.Str(section+"."+key, ""); v != "" {
			return v
		}
		return defaults.Str(key, "")
	}
	return pick("theme"), pick("look"), pick("layout")
}

// Doc is a diagram's source with its front matter, directives and comments
// taken out.
type Doc struct {
	// Code is what the diagram's parser reads.
	Code string
	// Title is the front matter's title.
	Title string
	// Overrides is what the front matter's config and the directives set.
	Overrides map[string]any
	// LineOffset is how many lines the front matter took.
	LineOffset int
}

var (
	frontMatter   = regexp.MustCompile(`(?s)^([^\S\n\r]*)-{3}\s*[\n\r](.*?)[\n\r]`)
	directiveName = regexp.MustCompile(`^\s*(\w+)\s*:?`)
	comment       = regexp.MustCompile(`(?m)^\s*%%[^{\n][^\n]*\n?`)
	tagAttr       = regexp.MustCompile(`<(\w+)([^>]*)>`)
	dqAttr        = regexp.MustCompile(`="([^"]*)"`)
)

// Preprocess splits a diagram's source as mermaid's preprocessDiagram does.
func Preprocess(src string) (Doc, error) {
	src = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(src)
	// Double-quoted attributes in tags become single-quoted.
	src = tagAttr.ReplaceAllStringFunc(src, func(m string) string {
		return dqAttr.ReplaceAllString(m, "='$1'")
	})
	doc := Doc{Overrides: map[string]any{}}
	text := src
	if m := frontMatter.FindStringSubmatchIndex(src); m != nil {
		if body, rest, ok := closeFrontMatter(src, m); ok {
			indent := src[m[2]:m[3]]
			if indent != "" {
				lines := strings.Split(body, "\n")
				for i, l := range lines {
					lines[i] = strings.TrimPrefix(l, indent)
				}
				body = strings.Join(lines, "\n")
			}
			var meta map[string]any
			if err := yaml.Unmarshal([]byte(body), &meta); err != nil {
				return doc, err
			}
			if t, ok := meta["title"]; ok && t != nil {
				doc.Title = toString(t)
			}
			if c, ok := meta["config"].(map[string]any); ok {
				doc.Overrides = merge(doc.Overrides, c)
			}
			if dm, ok := meta["displayMode"]; ok && dm != nil {
				doc.Overrides = merge(doc.Overrides, map[string]any{"gantt": map[string]any{"displayMode": toString(dm)}})
			}
			doc.LineOffset = strings.Count(src[:len(src)-len(rest)], "\n")
			text = rest
		}
	}
	// %%{init: {...}}%% and %%{initialize: ...}%%, merged in order; wrap.
	var kept strings.Builder
	for {
		i := strings.Index(text, "%%{")
		if i < 0 {
			kept.WriteString(text)
			break
		}
		kept.WriteString(text[:i])
		body := text[i+3:]
		if end := strings.Index(body, "}%%"); end >= 0 {
			body, text = body[:end], body[end+3:]
		} else {
			text = ""
		}
		m := directiveName.FindStringSubmatchIndex(body)
		if m == nil {
			continue
		}
		switch strings.ToLower(body[m[2]:m[3]]) {
		case "init", "initialize":
			var v map[string]any
			if json.Unmarshal([]byte(strings.ReplaceAll(strings.TrimSpace(body[m[1]:]), "'", `"`)), &v) == nil {
				doc.Overrides = merge(doc.Overrides, v)
			}
		case "wrap":
			doc.Overrides["wrap"] = true
		}
	}
	text = comment.ReplaceAllString(kept.String(), "")
	doc.Code = strings.TrimLeft(text, " \t\n")
	return doc, nil
}

// closeFrontMatter finds the closing --- at the opening one's indent.
func closeFrontMatter(src string, m []int) (body, rest string, ok bool) {
	indent := src[m[2]:m[3]]
	start := m[4]
	for i := start; i < len(src); {
		nl := strings.IndexByte(src[i:], '\n')
		if nl < 0 {
			return "", "", false
		}
		line := i + nl + 1
		if strings.HasPrefix(src[line:], indent+"---") {
			after := strings.TrimLeft(src[line+len(indent)+3:], "-")
			trimmed := strings.TrimLeft(after, " \t")
			if trimmed == "" || trimmed[0] == '\n' || trimmed[0] == '\r' {
				body = src[start : i+nl]
				rest = strings.TrimLeft(trimmed, "\n\r")
				return body, rest, true
			}
		}
		i = line
	}
	return "", "", false
}

func toString(v any) string {
	switch v := v.(type) {
	case string:
		return v
	default:
		b, _ := json.Marshal(v)
		return strings.Trim(string(b), `"`)
	}
}

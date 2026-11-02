// Package theme is mermaid's themes (src/themes, mermaid 12.2.0): the variables each
// produces, given the user's themeVariables, with the same values to the last character.
package theme

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sync"
)

// A def is one of mermaid's Theme classes.
type def struct {
	init   func(vars) // the constructor
	update func(vars) // updateColors
	// clearCalculated: calculate() first turns the constructor's 'calculated' placeholders
	// into undefined (theme-default only).
	clearCalculated bool
	// gradientYieldsToNodeBorder: an overridden nodeBorder turns useGradient off unless
	// that is overridden too (theme-base only).
	gradientYieldsToNodeBorder bool
}

var defs = map[string]def{}

// Names are the themes, as mermaid names them.
func Names() []string { return slices.Sorted(maps.Keys(defs)) }

// Variables are a theme's variables.
//
// The fields are the variables shared by most diagrams, converted to Go types (strings as
// JavaScript's String() gives them, "" for unset; booleans by JavaScript truthiness;
// numbers by Number()). All has every variable by mermaid's name, as JavaScript values:
// string, float64, bool, nil (null), []any, and map[string]any for the nested ones
// (xyChart, radar, cynefin, wardley). Diagram-specific variables (cScale0..11, pie1..12,
// git0..7, fillType0..7, ...) are read from All, or with Str.
type Variables struct {
	Name     string // the theme
	DarkMode bool   `js:"darkMode"`

	Background           string `js:"background"`
	PrimaryColor         string `js:"primaryColor"`
	SecondaryColor       string `js:"secondaryColor"`
	TertiaryColor        string `js:"tertiaryColor"`
	PrimaryBorderColor   string `js:"primaryBorderColor"`
	SecondaryBorderColor string `js:"secondaryBorderColor"`
	TertiaryBorderColor  string `js:"tertiaryBorderColor"`
	PrimaryTextColor     string `js:"primaryTextColor"`
	SecondaryTextColor   string `js:"secondaryTextColor"`
	TertiaryTextColor    string `js:"tertiaryTextColor"`
	LineColor            string `js:"lineColor"`
	TextColor            string `js:"textColor"`
	MainBkg              string `js:"mainBkg"`
	NodeBkg              string `js:"nodeBkg"`
	NodeBorder           string `js:"nodeBorder"`
	NodeTextColor        string `js:"nodeTextColor"`
	ClusterBkg           string `js:"clusterBkg"`
	ClusterBorder        string `js:"clusterBorder"`
	DefaultLinkColor     string `js:"defaultLinkColor"`
	TitleColor           string `js:"titleColor"`
	EdgeLabelBackground  string `js:"edgeLabelBackground"`
	ArrowheadColor       string `js:"arrowheadColor"`
	NoteBkgColor         string `js:"noteBkgColor"`
	NoteBorderColor      string `js:"noteBorderColor"`
	NoteTextColor        string `js:"noteTextColor"`
	ErrorBkgColor        string `js:"errorBkgColor"`
	ErrorTextColor       string `js:"errorTextColor"`

	FontFamily     string  `js:"fontFamily"`
	FontSize       string  `js:"fontSize"`
	FontWeight     string  `js:"fontWeight"`
	NoteFontWeight string  `js:"noteFontWeight"`
	Radius         float64 `js:"radius"`
	StrokeWidth    float64 `js:"strokeWidth"`

	UseGradient   bool   `js:"useGradient"`
	GradientStart string `js:"gradientStart"`
	GradientStop  string `js:"gradientStop"`
	DropShadow    string `js:"dropShadow"`

	All map[string]any
}

// Str is the variable at path (a name, then keys into nested objects) as String() gives
// it, "" if unset or null.
func (v *Variables) Str(path ...string) string {
	var x any = v.All
	for _, k := range path {
		m, ok := x.(map[string]any)
		if !ok {
			return ""
		}
		x = m[k]
	}
	return jsString(x)
}

// Get is the variable named k and whether it is set (null counts as set).
func (v *Variables) Get(k string) (any, bool) {
	x, ok := v.All[k]
	return x, ok
}

// Get returns the named theme's variables with the user's overrides (themeVariables,
// darkMode among them), as mermaid's getThemeVariables(overrides): the theme's base values,
// then the overrides, then the derived values (which keep any the user set), then the
// overrides again, nested objects (xyChart, ...) merged key by key.
//
// Overrides are JavaScript values as encoding/json decodes them (nil is null); Go ints are
// accepted as numbers. It fails for an unknown theme and where mermaid throws, as for a
// color khroma cannot parse.
//
// mermaid.initialize runs this a second time over its own output, which changes nothing,
// except that mermaid's config merge drops null overrides before the theme sees them.
//
// Without overrides the result is computed once and shared; it must not be changed.
func Get(name string, overrides map[string]any) (*Variables, error) {
	if len(overrides) == 0 {
		if v, ok := plain.Load(name); ok {
			return v.(*Variables), nil
		}
		v, err := get(name, nil)
		if err == nil {
			plain.Store(name, v)
		}
		return v, err
	}
	return get(name, overrides)
}

// plain holds the themes without overrides, by name.
var plain sync.Map

func get(name string, overrides map[string]any) (*Variables, error) {
	d, ok := defs[name]
	if !ok {
		return nil, fmt.Errorf("theme: unknown theme %q", name)
	}
	m, err := d.compute(overrides)
	if err != nil {
		return nil, fmt.Errorf("theme %s: %w", name, err)
	}
	vs := &Variables{Name: name, All: m}
	rv := reflect.ValueOf(vs).Elem()
	for i := range rv.NumField() {
		name := rv.Type().Field(i).Tag.Get("js")
		if name == "" {
			continue
		}
		switch f := rv.Field(i); f.Kind() {
		case reflect.String:
			f.SetString(jsString(m[name]))
		case reflect.Bool:
			f.SetBool(truthy(m[name]))
		case reflect.Float64:
			f.SetFloat(toNumber(m[name]))
		}
	}
	return vs, nil
}

func (d def) compute(overrides map[string]any) (m map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			f, ok := r.(failure)
			if !ok {
				panic(r)
			}
			err = f.err
		}
	}()
	v := vars{}
	d.init(v)
	if d.clearCalculated {
		for k, x := range v {
			if x == "calculated" {
				delete(v, k)
			}
		}
	}
	for k, x := range overrides {
		v.set(k, fromUser(x))
	}
	d.update(v)
	for k, x := range overrides {
		x = fromUser(x)
		gen, ok1 := v[k].(map[string]any)
		over, ok2 := x.(map[string]any)
		if ok1 && ok2 {
			merged := maps.Clone(gen)
			maps.Copy(merged, over)
			x = merged
		}
		v.set(k, x)
	}
	if d.gradientYieldsToNodeBorder {
		_, nb := overrides["nodeBorder"]
		_, ug := overrides["useGradient"]
		if nb && !ug {
			v["useGradient"] = false
		}
	}
	return toUser(v).(map[string]any), nil
}

// fromUser copies a user value into theme form: numbers as float64, null as null{}.
func fromUser(x any) any {
	switch x := x.(type) {
	case nil:
		return null{}
	case int:
		return float64(x)
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[k] = fromUser(e)
		}
		return m
	case []any:
		a := make([]any, len(x))
		for i, e := range x {
			a[i] = fromUser(e)
		}
		return a
	}
	return x
}

// toUser is the inverse of fromUser.
func toUser(x any) any {
	switch x := x.(type) {
	case null:
		return nil
	case vars:
		return toUser(map[string]any(x))
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[k] = toUser(e)
		}
		return m
	case []any:
		a := make([]any, len(x))
		for i, e := range x {
			a[i] = toUser(e)
		}
		return a
	}
	return x
}

package theme

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/color"
)

// vars is a theme object while it is computed: mermaid's variable names to JavaScript
// values (string, float64, bool, map[string]any, []any, null). An absent key is undefined.
//
// The themes are written in the order mermaid's are, statement for statement where the
// order matters, since later values read earlier ones and user overrides land in between.
// Color operations evaluate eagerly; one that fails yields a bad value, which only raises
// the error if it is stored, as JavaScript would only have run it then.
type vars map[string]any

// null is JavaScript's null, which a user override can carry. It is kept apart from an
// absent key (undefined) because the two print differently.
type null struct{}

// bad is a color operation that threw.
type bad struct{ err error }

// failure is the panic value set raises with a bad value; compute recovers it.
type failure struct{ err error }

func truthy(x any) bool {
	switch x := x.(type) {
	case nil, null, bad:
		return false
	case string:
		return x != ""
	case float64:
		return x != 0 && !math.IsNaN(x)
	case bool:
		return x
	}
	return true
}

func nullish(x any) bool {
	switch x.(type) {
	case nil, null:
		return true
	}
	return false
}

// norm turns Go integers into JavaScript numbers.
func norm(x any) any {
	if i, ok := x.(int); ok {
		return float64(i)
	}
	return x
}

// set is "this.k = x"; storing undefined removes the key.
func (v vars) set(k string, x any) {
	switch x := x.(type) {
	case bad:
		panic(failure{x.err})
	case nil:
		delete(v, k)
		return
	}
	v[k] = norm(x)
}

// or is "this.k = this.k || x".
func (v vars) or(k string, x any) {
	if !truthy(v[k]) {
		v.set(k, x)
	}
}

// ifNull is "this.k = this.k ?? x".
func (v vars) ifNull(k string, x any) {
	if nullish(v[k]) {
		v.set(k, x)
	}
}

// first is "a || b || c": the first truthy value, else the last. A bad value met before a
// truthy one is returned, as the throw would have happened there.
func first(xs ...any) any {
	for _, x := range xs {
		if _, ok := x.(bad); ok || truthy(x) {
			return x
		}
	}
	return xs[len(xs)-1]
}

// pick is "cond ? a : b".
func pick(cond bool, a, b any) any {
	if cond {
		return a
	}
	return b
}

func (v vars) dark() bool { return truthy(v["darkMode"]) }

// sub is "this.k?.field".
func (v vars) sub(k, field string) any {
	if m, ok := v[k].(map[string]any); ok {
		return m[field]
	}
	return nil
}

// field is one entry of a nested object a theme builds: "name: this.obj?.name || def".
type field struct {
	name string
	def  any
}

// nested is "this.k = { name: this.k?.name || def, ... }".
func (v vars) nested(k string, fields ...field) {
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		x := first(v.sub(k, f.name), norm(f.def))
		if b, ok := x.(bad); ok {
			panic(failure{b.err})
		}
		if x != nil {
			m[f.name] = x
		}
	}
	v[k] = m
}

// elem is "this.k[i]" on an array; length is "this.k.length".
func (v vars) elem(k string, i int) any {
	if a, ok := v[k].([]any); ok && i < len(a) {
		return a[i]
	}
	return nil
}

func (v vars) length(k string) int {
	a, _ := v[k].([]any)
	return len(a)
}

// limit is the bound of "for (let i = 0; i < this.THEME_COLOR_LIMIT; i++)".
func (v vars) limit() int {
	n := toNumber(v["THEME_COLOR_LIMIT"])
	if !(n > 0) {
		return 0
	}
	return int(math.Ceil(math.Min(n, 1000)))
}

// key is "'cScale' + i".
func key(prefix string, i int) string { return prefix + strconv.Itoa(i) }

// list is an array literal of strings.
func list(xs ...string) []any {
	a := make([]any, len(xs))
	for i, x := range xs {
		a[i] = x
	}
	return a
}

// The color operations, on values that should be color strings. A non-string makes khroma
// throw, except for isDark, where it reads channels of a non-color and decides "dark".

func colorOp(x any, op func(string) (string, error)) any {
	switch x := x.(type) {
	case bad:
		return x
	case string:
		s, err := op(x)
		if err != nil {
			return bad{err}
		}
		return s
	}
	return bad{errNotColor(x)}
}

func adjust(x any, d color.Delta) any {
	return colorOp(x, func(s string) (string, error) { return color.Adjust(s, d) })
}

func darken(x any, amount float64) any {
	return colorOp(x, func(s string) (string, error) { return color.Darken(s, amount) })
}

func lighten(x any, amount float64) any {
	return colorOp(x, func(s string) (string, error) { return color.Lighten(s, amount) })
}

func invert(x any) any {
	return colorOp(x, func(s string) (string, error) { return color.Invert(s, 100) })
}

func rgba(r, g, b, a float64) string { return color.RGBA(r, g, b, a) }

// isDark is khroma's isDark as a value: true, false, or bad.
func isDark(x any) any {
	switch x := x.(type) {
	case bad:
		return x
	case string:
		d, err := color.IsDark(x)
		if err != nil {
			return bad{err}
		}
		return d
	case nil, null:
		return bad{errNotColor(x)}
	}
	return true
}

// shade lightens a dark color and darkens a light one by amount.
func shade(x any, amount float64) any {
	switch d := isDark(x).(type) {
	case bad:
		return d
	case bool:
		if d {
			return lighten(x, amount)
		}
	}
	return darken(x, amount)
}

// mkBorder is theme-helpers' mkBorder: a desaturated, darker (lighter in dark mode) shade.
func mkBorder(x any, dark bool) any {
	return adjust(x, color.Delta{S: -40, L: pick(dark, 10.0, -10.0).(float64)})
}

func toNumber(x any) float64 {
	switch x := x.(type) {
	case float64:
		return x
	case bool:
		if x {
			return 1
		}
		return 0
	case null:
		return 0
	case string:
		return jsStringToNumber(x)
	}
	return math.NaN()
}

func errNotColor(x any) error { return fmt.Errorf("theme: %s is not a color", jsString(x)) }

var decimal = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// jsStringToNumber is JavaScript's Number(string) for decimal text (enough for a loop bound).
func jsStringToNumber(s string) float64 {
	s = strings.TrimSpace(s)
	switch strings.TrimPrefix(s, "+") {
	case "":
		return 0
	case "Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if !decimal.MatchString(s) {
		return math.NaN()
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// jsString is String(x), except that undefined and null are "".
func jsString(x any) string {
	switch x := x.(type) {
	case nil, null:
		return ""
	case string:
		return x
	case float64:
		return color.FormatNumber(x)
	case bool:
		return strconv.FormatBool(x)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = jsString(e)
		}
		return strings.Join(parts, ",")
	case map[string]any:
		return "[object Object]"
	}
	return fmt.Sprint(x)
}

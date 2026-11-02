// Package style turns mermaid's style values (theme colors, CSS
// declarations such as "fill:#f9f;stroke-width:4px") into scene values.
package style

import (
	"math"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/color"
	"github.com/f1sherb0y/mermaid-go/scene"
)

// Color is a CSS color as a scene color; None for "none", "transparent" or
// anything that is not a color.
func Color(s string) scene.Color {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "!important"))
	switch strings.ToLower(s) {
	case "", "none", "transparent":
		return scene.None
	}
	ch := func(c color.Channel) float64 {
		v, err := color.ChannelOf(s, c)
		if err != nil {
			return math.NaN()
		}
		return v
	}
	r, g, b, a := ch(color.R), ch(color.G), ch(color.B), ch(color.A)
	if math.IsNaN(r) || math.IsNaN(a) {
		return scene.None
	}
	u8 := func(v float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(255, v)))) }
	return scene.Color{R: u8(r), G: u8(g), B: u8(b), A: a}
}

// Px is a CSS length in px ("4px", "4", "1.5em" at 16px); def when unset or
// not a length.
func Px(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	scale := 1.0
	switch {
	case strings.HasSuffix(s, "px"):
		s = s[:len(s)-2]
	case strings.HasSuffix(s, "em"):
		s, scale = s[:len(s)-2], 16
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return def
	}
	return f * scale
}

// Decls is a CSS declaration list, later declarations winning.
type Decls map[string]string

// Parse reads declarations: "fill:#f9f;stroke:#333" or a list of them.
func Parse(css ...string) Decls {
	d := Decls{}
	for _, s := range css {
		for _, decl := range strings.Split(s, ";") {
			k, v, ok := strings.Cut(decl, ":")
			if !ok {
				continue
			}
			k = strings.ToLower(strings.TrimSpace(k))
			v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
			if k != "" && v != "" {
				d[k] = v
			}
		}
	}
	return d
}

// Dash is a stroke-dasharray value ("5,5", "3 2", "none").
func Dash(s string) []float64 {
	var out []float64
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		if v := Px(f, -1); v >= 0 {
			out = append(out, v)
		}
	}
	return out
}

// DropShadow is a CSS filter's drop-shadow(dx dy blur color), nil for
// anything else (none, url(...)).
func DropShadow(s string) *scene.Shadow {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), ";"))
	inner, ok := strings.CutPrefix(s, "drop-shadow(")
	if !ok || !strings.HasSuffix(inner, ")") {
		return nil
	}
	inner = strings.TrimSpace(inner[:len(inner)-1])
	var lengths []float64
	for len(lengths) < 3 {
		f, rest, _ := strings.Cut(inner, " ")
		v := Px(f, math.NaN())
		if math.IsNaN(v) {
			break
		}
		lengths = append(lengths, v)
		inner = strings.TrimSpace(rest)
	}
	if len(lengths) < 2 {
		return nil
	}
	sh := &scene.Shadow{DX: lengths[0], DY: lengths[1], Color: scene.RGB(0, 0, 0)}
	if len(lengths) == 3 {
		sh.Blur = lengths[2]
	}
	if inner != "" {
		sh.Color = Color(inner)
	}
	return sh
}

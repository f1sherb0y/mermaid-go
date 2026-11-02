// Package color is khroma 2.1, the color library mermaid's themes and styles use: it parses
// the CSS colors khroma accepts and does its operations with khroma's exact results, down
// to the text: an unchanged color comes back as written, a color whose hue, saturation or
// lightness changed prints as hsl(), one with fractional channels or alpha as rgb(), and
// the rest as #rrggbb.
//
// The arithmetic follows khroma's operation by operation. Products added to something are
// wrapped in float64() so that no platform fuses them into one rounding (Go may, JavaScript
// does not).
package color

import (
	"errors"
	"fmt"
	"math"
)

// Channel names one of a color's channels.
type Channel int

const (
	R Channel = iota // red, 0–255
	G                // green, 0–255
	B                // blue, 0–255
	H                // hue, degrees
	S                // saturation, 0–100
	L                // lightness, 0–100
	A                // alpha, 0–1
)

var (
	// ErrFormat is returned for text khroma does not parse as a color.
	ErrFormat = errors.New("unsupported color format")
	// ErrMixedChannels is returned when one change touches both RGB and HSL channels.
	ErrMixedChannels = errors.New("cannot change both RGB and HSL channels at the same time")
)

type model int

const (
	anyModel model = iota
	rgbModel
	hslModel
)

// color is khroma's Channels: the channels it was given, the other model worked out on
// demand, and the text it was parsed from, which it prints as until something changes.
type color struct {
	v       [7]float64
	known   [7]bool // JavaScript's "!== undefined" for each channel
	model   model   // the model a change was made in
	changed bool
	text    string // "" for a color made from numbers
}

func (c *color) get(ch Channel) float64 {
	switch {
	case ch == A:
		return c.v[A]
	case ch <= B:
		if c.model != hslModel && c.known[ch] {
			return c.v[ch]
		}
		c.fill(H, rgbToHSL)
		return hslToRGB(&c.v, ch)
	default:
		if c.model != rgbModel && c.known[ch] {
			return c.v[ch]
		}
		c.fill(R, hslToRGB)
		return rgbToHSL(&c.v, ch)
	}
}

// fill stores the unknown channels of the model starting at first, worked out from the other.
func (c *color) fill(first Channel, from func(*[7]float64, Channel) float64) {
	for ch := first; ch < first+3; ch++ {
		if !c.known[ch] {
			c.v[ch], c.known[ch] = from(&c.v, ch), true
		}
	}
}

func (c *color) set(ch Channel, x float64) error {
	if ch != A {
		m := rgbModel
		if ch >= H {
			m = hslModel
		}
		if c.model != anyModel && c.model != m {
			return ErrMixedChannels
		}
		c.model = m
	}
	c.changed = true
	c.v[ch], c.known[ch] = x, true
	return nil
}

// clamp is khroma's clamp for a channel; hue wraps (keeping its sign) rather than clamps.
func clamp(ch Channel, x float64) float64 {
	hi := 100.0
	switch ch {
	case H:
		return math.Mod(x, 360)
	case R, G, B:
		hi = 255
	case A:
		hi = 1
	}
	if x >= hi {
		return hi
	}
	if x < 0 {
		return 0
	}
	return x
}

func hslToRGB(v *[7]float64, ch Channel) float64 {
	h, s, l := v[H], v[S], v[L]
	if s == 0 || math.IsNaN(s) {
		return l * 2.55
	}
	h, s, l = h/360, s/100, l/100
	q := l + s - float64(l*s)
	if l < .5 {
		q = l * (1 + s)
	}
	p := float64(2*l) - q
	t := h + [3]float64{1.0 / 3, 0, -1.0 / 3}[ch]
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	var x float64
	switch {
	case t < 1.0/6:
		x = p + float64((q-p)*6*t)
	case t < 1.0/2:
		x = q
	case t < 2.0/3:
		x = p + float64((q-p)*(2.0/3-t)*6)
	default:
		x = p
	}
	return x * 255
}

func rgbToHSL(v *[7]float64, ch Channel) float64 {
	r, g, b := v[R]/255, v[G]/255, v[B]/255
	hi, lo := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l := (hi + lo) / 2
	if ch == L {
		return l * 100
	}
	if hi == lo {
		return 0
	}
	d := hi - lo
	s := d / (hi + lo)
	if l > .5 {
		s = d / (2 - hi - lo)
	}
	if ch == S {
		return s * 100
	}
	switch hi {
	case r:
		h := (g - b) / d
		if g < b {
			h += 6
		}
		return h * 60
	case g:
		return ((b-r)/d + 2) * 60
	case b:
		return ((r-g)/d + 4) * 60
	}
	return -1
}

func (c *color) String() string {
	if !c.changed && c.text != "" {
		return c.text
	}
	num := func(ch Channel) string { return FormatNumber(round10(c.get(ch))) }
	a := c.v[A]
	if c.model == hslModel || !c.known[R] {
		if a < 1 {
			return "hsla(" + num(H) + ", " + num(S) + "%, " + num(L) + "%, " + FormatNumber(a) + ")"
		}
		return "hsl(" + num(H) + ", " + num(S) + "%, " + num(L) + "%)"
	}
	if a < 1 || !isInt(c.get(R)) || !isInt(c.get(G)) || !isInt(c.get(B)) {
		if a < 1 {
			return "rgba(" + num(R) + ", " + num(G) + ", " + num(B) + ", " + num(A) + ")"
		}
		return "rgb(" + num(R) + ", " + num(G) + ", " + num(B) + ")"
	}
	return fmt.Sprintf("#%02x%02x%02x", int(c.get(R)), int(c.get(G)), int(c.get(B)))
}

func isInt(x float64) bool { return !math.IsInf(x, 0) && x == math.Trunc(x) }

func fromRGBA(r, g, b, a float64) *color {
	return &color{
		v:     [7]float64{clamp(R, r), clamp(G, g), clamp(B, b), 0, 0, 0, clamp(A, a)},
		known: [7]bool{true, true, true, false, false, false, true},
	}
}

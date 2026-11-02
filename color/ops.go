package color

import "math"

// Delta is a change to some channels, as khroma's adjust takes it. A zero (or NaN) field is
// no change. RGB and HSL fields cannot be mixed.
type Delta struct{ R, G, B, H, S, L, A float64 }

// Adjust adds d to the color's channels (khroma's adjust).
func Adjust(s string, d Delta) (string, error) {
	c, err := parse(s)
	if err != nil {
		return "", err
	}
	type change struct {
		ch Channel
		x  float64
	}
	var changes []change
	for ch, x := range [7]float64{d.R, d.G, d.B, d.H, d.S, d.L, d.A} {
		if x != 0 && !math.IsNaN(x) {
			changes = append(changes, change{Channel(ch), c.get(Channel(ch)) + x})
		}
	}
	// khroma's change: set each from a fresh parse.
	c, _ = parse(s)
	for _, x := range changes {
		if err := c.set(x.ch, clamp(x.ch, x.x)); err != nil {
			return "", err
		}
	}
	return c.String(), nil
}

// adjustChannel adds amount to one channel, leaving the color untouched if that changes nothing.
func adjustChannel(s string, ch Channel, amount float64) (string, error) {
	c, err := parse(s)
	if err != nil {
		return "", err
	}
	cur := c.get(ch)
	if next := clamp(ch, cur+amount); next != cur { // NaN != NaN, as in JavaScript
		c.set(ch, next)
	}
	return c.String(), nil
}

// Lighten raises the lightness by amount (0–100). khroma's lighten(c) without an amount is
// Lighten(c, NaN).
func Lighten(s string, amount float64) (string, error) { return adjustChannel(s, L, amount) }

// Darken lowers the lightness by amount (0–100).
func Darken(s string, amount float64) (string, error) { return adjustChannel(s, L, -amount) }

// Transparentize lowers the alpha by amount (0–1).
func Transparentize(s string, amount float64) (string, error) { return adjustChannel(s, A, -amount) }

// SetAlpha sets the alpha (khroma's rgba(color, alpha)).
func SetAlpha(s string, alpha float64) (string, error) {
	c, err := parse(s)
	if err != nil {
		return "", err
	}
	c.set(A, clamp(A, alpha))
	return c.String(), nil
}

// RGBA makes a color from channels (khroma's rgba(r, g, b, a)).
func RGBA(r, g, b, a float64) string { return fromRGBA(r, g, b, a).String() }

// Invert mixes the color's inverse into it, weight percent of the inverse (khroma's
// invert(color, weight); mermaid always uses 100).
func Invert(s string, weight float64) (string, error) {
	c, err := parse(s)
	if err != nil {
		return "", err
	}
	for ch := R; ch <= B; ch++ {
		c.set(ch, 255-c.get(ch))
	}
	orig, _ := parse(s)
	return mix(c, orig, weight), nil
}

// Mix mixes two colors as Sass does, weight percent of the first.
func Mix(s1, s2 string, weight float64) (string, error) {
	c1, err := parse(s1)
	if err != nil {
		return "", err
	}
	c2, err := parse(s2)
	if err != nil {
		return "", err
	}
	return mix(c1, c2, weight), nil
}

func mix(c1, c2 *color, weight float64) string {
	r1, g1, b1, a1 := c1.get(R), c1.get(G), c1.get(B), c1.get(A)
	r2, g2, b2, a2 := c2.get(R), c2.get(G), c2.get(B), c2.get(A)
	scale := weight / 100
	norm := float64(scale*2) - 1
	da := a1 - a2
	w1 := norm
	if float64(norm*da) != -1 {
		w1 = (norm + da) / (1 + float64(norm*da))
	}
	w1 = (w1 + 1) / 2
	w2 := 1 - w1
	lerp := func(x1, x2 float64) float64 { return float64(x1*w1) + float64(x2*w2) }
	return RGBA(lerp(r1, r2), lerp(g1, g2), lerp(b1, b2), float64(a1*scale)+float64(a2*(1-scale)))
}

// ChannelOf returns one channel of the color, rounded to 10 decimals (khroma's channel).
func ChannelOf(s string, ch Channel) (float64, error) {
	c, err := parse(s)
	if err != nil {
		return 0, err
	}
	return round10(c.get(ch)), nil
}

// Luminance is the color's relative luminance, rounded to 10 decimals.
func Luminance(s string) (float64, error) {
	c, err := parse(s)
	if err != nil {
		return 0, err
	}
	lin := func(ch Channel) float64 {
		x := c.get(ch)
		n := x / 255
		if x > .03928 {
			return math.Pow((n+.055)/1.055, 2.4)
		}
		return n / 12.92
	}
	return round10(float64(.2126*lin(R)) + float64(.7152*lin(G)) + float64(.0722*lin(B))), nil
}

// IsLight reports whether the luminance is at least 0.5.
func IsLight(s string) (bool, error) {
	l, err := Luminance(s)
	return l >= .5, err
}

// IsDark is !IsLight.
func IsDark(s string) (bool, error) {
	l, err := IsLight(s)
	return !l && err == nil, err
}

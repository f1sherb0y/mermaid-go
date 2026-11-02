package color

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// khroma's regular expressions, which are case-insensitive. JavaScript's /i folds only
// ASCII letters here, so the text is matched with its ASCII letters lowered rather than with
// Go's (?i), which would also fold ſ and K. \s is JavaScript's, which is wider than Go's.
const (
	ws     = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`
	sep    = ws + `*?(?:,|` + ws + `)` + ws + `*?`
	alpha  = `(?:` + ws + `*?(?:,|/)` + ws + `*?\+?(NUM(%?)))?` + ws + `*?\)$`
	number = `-?(?:\d+(?:\.\d+)?|(?:\.\d+))`
)

var (
	rgbRe = regexp.MustCompile(strings.ReplaceAll(`^rgba?\(`+ws+`*?(NUM(%?))`+sep+`(NUM(%?))`+sep+`(NUM(%?))`+alpha,
		"NUM", number+`(?:e\d+)?`))
	hslRe = regexp.MustCompile(strings.ReplaceAll(`^hsla?\(`+ws+`*?(NUM(?:deg|grad|rad|turn)?)`+sep+`(NUM%)`+sep+`(NUM%)`+alpha,
		"NUM", number+`(?:e-?\d+)?`))
)

func parse(s string) (*color, error) {
	if c := parseHex(s, s); c != nil {
		return c, nil
	}
	low := lowerASCII(s)
	if m := rgbRe.FindStringSubmatchIndex(low); m != nil {
		g := func(i int) string { return s[m[2*i]:m[2*i+1]] }
		ch := func(i int) float64 {
			x := parseFloat(g(i))
			if g(i+1) != "" {
				x *= 2.55
			}
			return x
		}
		return &color{
			v:     [7]float64{clamp(R, ch(1)), clamp(G, ch(3)), clamp(B, ch(5)), 0, 0, 0, parseAlpha(s, m, 7)},
			known: [7]bool{true, true, true, false, false, false, true},
			text:  s,
		}, nil
	}
	if m := hslRe.FindStringSubmatchIndex(low); m != nil {
		g := func(i int) string { return s[m[2*i]:m[2*i+1]] }
		return &color{
			v:     [7]float64{0, 0, 0, hueDegrees(g(1)), clamp(S, parseFloat(g(2))), clamp(L, parseFloat(g(3))), parseAlpha(s, m, 4)},
			known: [7]bool{false, false, false, true, true, true, true},
			text:  s,
		}, nil
	}
	// JavaScript's toLowerCase, which unlike Go's turns İ into two runes, neither ASCII.
	if !strings.ContainsRune(s, 'İ') {
		if hex, ok := keywords[strings.ToLower(s)]; ok {
			return parseHex(hex, hex), nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrFormat, s)
}

// parseHex parses #rgb, #rgba, #rrggbb and #rrggbbaa; text is what the color prints as.
func parseHex(s, text string) *color {
	n := len(s) - 1
	if n != 3 && n != 4 && n != 6 && n != 8 || s[0] != '#' {
		return nil
	}
	dec, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return nil
	}
	bits, mask, mul := uint(4), uint64(15), 17.0
	if n > 4 {
		bits, mask, mul = 8, 255, 1
	}
	digit := func(i uint) float64 { return float64(dec>>(bits*i)&mask) * mul }
	c := &color{known: [7]bool{true, true, true, false, false, false, true}, text: text}
	if n%4 == 0 {
		c.v = [7]float64{R: digit(3), G: digit(2), B: digit(1), A: digit(0) / 255}
	} else {
		c.v = [7]float64{R: digit(2), G: digit(1), B: digit(0), A: 1}
	}
	return c
}

// parseAlpha reads the optional alpha at submatch i (number) and i+1 (its "%").
func parseAlpha(s string, m []int, i int) float64 {
	if m[2*i] < 0 {
		return 1
	}
	a := parseFloat(s[m[2*i]:m[2*i+1]])
	if m[2*i+3] > m[2*i+2] {
		a /= 100
	}
	return clamp(A, a)
}

// hueDegrees is khroma's _hue2deg. Its unit switch is case-sensitive, so "1GRAD" is 1°.
func hueDegrees(hue string) float64 {
	low := lowerASCII(hue)
	for _, u := range []struct {
		name  string
		scale func(float64) float64
	}{
		{"deg", nil},
		{"grad", func(x float64) float64 { return x * .9 }},
		{"rad", func(x float64) float64 { return x * 180 / math.Pi }},
		{"turn", func(x float64) float64 { return x * 360 }},
	} {
		if strings.HasSuffix(low, u.name) {
			if u.scale != nil && strings.HasSuffix(hue, u.name) {
				return clamp(H, u.scale(parseFloat(hue)))
			}
			break
		}
	}
	return clamp(H, parseFloat(hue))
}

// parseFloat is JavaScript's parseFloat on text the regexps matched: a number, then perhaps
// a unit or "%", which is ignored.
func parseFloat(s string) float64 {
	end := strings.IndexFunc(s, func(r rune) bool {
		return !(r >= '0' && r <= '9' || r == '.' || r == '-' || r == 'e' || r == 'E')
	})
	if end >= 0 {
		s = s[:end]
	}
	// On a range error ParseFloat still returns ±Inf or 0, as JavaScript does.
	x, _ := strconv.ParseFloat(s, 64)
	return x
}

func lowerASCII(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}, s)
}

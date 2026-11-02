// Package d3 holds the pieces of d3 mermaid's diagrams use: number
// formatting (d3-format), scales and ticks, hierarchy layouts.
package d3

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// spec is d3-format's [[fill]align][sign][symbol][0][width][,][.precision][~][type].
var spec = regexp.MustCompile(`^(?:(.)?([<>=^]))?([+\-( ])?([$#])?(0)?(\d+)?(,)?(\.\d+)?(~)?([a-z%])?$`)

// Format returns d3.format(specifier): a function formatting numbers.
func Format(specifier string) (func(float64) string, error) {
	m := spec.FindStringSubmatch(specifier)
	if m == nil {
		return nil, fmt.Errorf("invalid format: %s", specifier)
	}
	fill, align, sign, symbol, zero, widthS, comma, precS, trim, typ := m[1], m[2], m[3], m[4], m[5] != "", m[6], m[7] != "", m[8], m[9] != "", m[10]
	if fill == "" {
		fill = " "
	}
	if zero || (fill == "0" && align == "=") {
		zero, fill, align = true, "0", "="
	}
	if sign == "" {
		sign = "-"
	}
	width := 0
	if widthS != "" {
		width, _ = strconv.Atoi(widthS)
	}
	precision := -1
	if precS != "" {
		precision, _ = strconv.Atoi(precS[1:])
	}
	if typ == "n" {
		comma, typ = true, "g"
	}
	if typ == "" {
		trim = true
		typ = "g"
		if precision < 0 {
			precision = 12
		}
	} else if precision < 0 {
		if typ == "s" || typ == "r" || typ == "g" || typ == "e" || typ == "p" {
			precision = 6
		} else if typ != "d" && typ != "x" && typ != "X" && typ != "o" && typ != "b" && typ != "c" {
			precision = 6
		}
	}
	switch typ {
	case "g", "p", "r", "s":
		precision = max(1, min(21, precision))
	default:
		precision = max(0, min(20, precision))
	}
	prefix, suffix := "", ""
	if symbol == "$" {
		prefix = "$"
	}
	if typ == "%" || typ == "p" {
		suffix = "%"
	}
	return func(v float64) string {
		neg := v < 0 || (v == 0 && math.Signbit(v))
		if typ == "%" || typ == "p" {
			v *= 100
		}
		a := math.Abs(v)
		var body, unit string
		switch typ {
		case "f", "%":
			body = strconv.FormatFloat(a, 'f', precision, 64)
		case "e":
			body = jsExp(a, precision)
		case "d":
			body = strconv.FormatFloat(math.Round(a), 'f', 0, 64)
		case "x", "X", "o", "b":
			base := map[string]int{"x": 16, "X": 16, "o": 8, "b": 2}[typ]
			body = strconv.FormatInt(int64(math.Round(a)), base)
			if typ == "X" {
				body = strings.ToUpper(body)
			}
		case "c":
			body = string(rune(int(a)))
		case "g", "p":
			body = jsPrecision(a, precision)
		case "r":
			body = rounded(a, precision)
		case "s":
			body, unit = siPrefix(a, precision)
		}
		if trim {
			body = trimZeros(body)
		}
		if neg && strings.Trim(body, "0.") == "" {
			neg = false // -0.0 formats as 0
		}
		if comma && !strings.Contains(body, "e") {
			intPart, frac, hasDot := strings.Cut(body, ".")
			body = group(intPart)
			if hasDot {
				body += "." + frac
			}
		}
		signStr := ""
		switch {
		case neg && sign == "(":
			signStr = "("
		case neg:
			signStr = "-"
		case sign == "+":
			signStr = "+"
		case sign == " ":
			signStr = " "
		}
		closing := ""
		if neg && sign == "(" {
			closing = ")"
		}
		out := prefix + body + unit + suffix + closing
		n := len([]rune(signStr + out))
		pad := ""
		if n < width {
			pad = strings.Repeat(fill, width-n)
		}
		switch align {
		case "<":
			return signStr + out + pad
		case "^":
			h := len([]rune(pad)) / 2
			return string([]rune(pad)[:h]) + signStr + out + string([]rune(pad)[h:])
		case "=":
			return signStr + pad + out
		default:
			return pad + signStr + out
		}
	}, nil
}

func group(intPart string) string {
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func trimZeros(s string) string {
	mant, exp, hasExp := strings.Cut(s, "e")
	if strings.Contains(mant, ".") {
		mant = strings.TrimRight(strings.TrimRight(mant, "0"), ".")
	}
	if hasExp {
		return mant + "e" + exp
	}
	return mant
}

// jsPrecision is JavaScript's Number.prototype.toPrecision.
func jsPrecision(a float64, p int) string {
	if a == 0 {
		return strconv.FormatFloat(0, 'f', p-1, 64)
	}
	// The exponent after rounding to p digits.
	r, _ := strconv.ParseFloat(strconv.FormatFloat(a, 'e', p-1, 64), 64)
	e := int(math.Floor(math.Log10(r)))
	if e < -6 || e >= p {
		return jsExp(a, p-1)
	}
	return strconv.FormatFloat(a, 'f', max(0, p-1-e), 64)
}

// jsExp is JavaScript's toExponential: "1.5e+3".
func jsExp(a float64, p int) string {
	s := strconv.FormatFloat(a, 'e', p, 64)
	mant, exp, _ := strings.Cut(s, "e")
	n, _ := strconv.Atoi(exp)
	if n >= 0 {
		return mant + "e+" + strconv.Itoa(n)
	}
	return mant + "e-" + strconv.Itoa(-n)
}

func rounded(a float64, p int) string {
	if a == 0 {
		return "0"
	}
	e := int(math.Floor(math.Log10(a)))
	scale := math.Pow(10, float64(p-1-e))
	return strconv.FormatFloat(math.Round(a*scale)/scale, 'f', max(0, p-1-e), 64)
}

var siPrefixes = []string{"y", "z", "a", "f", "p", "n", "µ", "m", "", "k", "M", "G", "T", "P", "E", "Z", "Y"}

func siPrefix(a float64, p int) (string, string) {
	if a == 0 {
		return jsPrecision(0, p), ""
	}
	r, _ := strconv.ParseFloat(strconv.FormatFloat(a, 'e', p-1, 64), 64)
	e := int(math.Floor(math.Log10(r)))
	k := max(-8, min(8, int(math.Floor(float64(e)/3))))
	v := a / math.Pow(10, float64(3*k))
	return jsPrecision(v, p), siPrefixes[k+8]
}

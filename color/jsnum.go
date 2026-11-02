package color

import (
	"math"
	"strconv"
	"strings"
)

// round is JavaScript's Math.round: halves go up, toward +Inf.
func round(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	r := math.Floor(x)
	if x-r >= 0.5 {
		r++
	}
	if r == 0 {
		return math.Copysign(0, x)
	}
	return r
}

// round10 is khroma's lang.round: 10 decimal places.
func round10(x float64) float64 { return round(x*1e10) / 1e10 }

// FormatNumber formats f as JavaScript's String(f) does.
func FormatNumber(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	sign := ""
	if f < 0 {
		sign, f = "-", -f
	}
	// Shortest digits that round-trip, as ECMAScript's Number::toString picks them.
	e := strconv.FormatFloat(f, 'e', -1, 64)
	mant, exp, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	x, _ := strconv.Atoi(exp)
	k, n := len(digits), x+1 // value = 0.digits × 10^n
	var s string
	switch {
	case k <= n && n <= 21:
		s = digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		s = digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		s = "0." + strings.Repeat("0", -n) + digits
	default:
		s = digits[:1]
		if k > 1 {
			s += "." + digits[1:]
		}
		if n-1 >= 0 {
			s += "e+" + strconv.Itoa(n-1)
		} else {
			s += "e" + strconv.Itoa(n-1)
		}
	}
	return sign + s
}

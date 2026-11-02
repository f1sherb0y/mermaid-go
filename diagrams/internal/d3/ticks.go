package d3

import (
	"math"
	"strconv"
	"strings"
)

var (
	e10 = math.Sqrt(50)
	e5  = math.Sqrt(10)
	e2  = math.Sqrt(2)
)

// tickSpec is d3-array's: ticks are i1..i2 times inc (inc > 0) or divided
// by -inc (inc < 0, for steps below 1, to keep them exact).
func tickSpec(start, stop, count float64) (i1, i2, inc float64) {
	step := (stop - start) / math.Max(0, count)
	power := math.Floor(math.Log10(step))
	err := step / math.Pow(10, power)
	factor := 1.0
	switch {
	case err >= e10:
		factor = 10
	case err >= e5:
		factor = 5
	case err >= e2:
		factor = 2
	}
	if power < 0 {
		inc = math.Pow(10, -power) / factor
		i1, i2 = jsRound(start*inc), jsRound(stop*inc)
		if i1/inc < start {
			i1++
		}
		if i2/inc > stop {
			i2--
		}
		inc = -inc
	} else {
		inc = math.Pow(10, power) * factor
		i1, i2 = jsRound(start/inc), jsRound(stop/inc)
		if i1*inc < start {
			i1++
		}
		if i2*inc > stop {
			i2--
		}
	}
	if i2 < i1 && 0.5 <= count && count < 2 {
		return tickSpec(start, stop, count*2)
	}
	return i1, i2, inc
}

// jsRound is JavaScript's Math.round (halves go up).
func jsRound(x float64) float64 { return math.Floor(x + 0.5) }

// Ticks is d3.ticks: about count round values from start to stop, in their
// order.
func Ticks(start, stop, count float64) []float64 {
	if !(count > 0) {
		return nil
	}
	if start == stop {
		return []float64{start}
	}
	reverse := stop < start
	if reverse {
		start, stop = stop, start
	}
	i1, i2, inc := tickSpec(start, stop, count)
	if !(i2 >= i1) {
		return nil
	}
	n := int(i2 - i1 + 1)
	out := make([]float64, n)
	for i := range n {
		k := i1 + float64(i)
		if reverse {
			k = i2 - float64(i)
		}
		if inc < 0 {
			out[i] = k / -inc
		} else {
			out[i] = k * inc
		}
	}
	return out
}

// TickStep is d3.tickStep: the step Ticks would use.
func TickStep(start, stop, count float64) float64 {
	reverse := stop < start
	var inc float64
	if reverse {
		_, _, inc = tickSpec(stop, start, count)
	} else {
		_, _, inc = tickSpec(start, stop, count)
	}
	if inc < 0 {
		inc = 1 / -inc
	}
	if reverse {
		return -inc
	}
	return inc
}

// Linear is a d3 linear scale from a domain to a range, without clamping.
type Linear struct{ D0, D1, R0, R1 float64 }

// At maps a value; the middle of the range when the domain is a point.
func (s Linear) At(v float64) float64 {
	if s.D1 == s.D0 {
		return (s.R0 + s.R1) / 2
	}
	return s.R0 + (v-s.D0)/(s.D1-s.D0)*(s.R1-s.R0)
}

// Ticks is scale.ticks(count).
func (s Linear) Ticks(count float64) []float64 { return Ticks(s.D0, s.D1, count) }

// Num is a number as JavaScript's String(number) writes it.
func Num(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	case v == 0:
		return "0"
	}
	a := math.Abs(v)
	if a >= 1e21 || a < 1e-6 {
		// Go writes "1e-07", JavaScript "1e-7".
		mant, exp, _ := strings.Cut(strconv.FormatFloat(v, 'e', -1, 64), "e")
		return mant + "e" + exp[:1] + strings.TrimLeft(exp[1:], "0")
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

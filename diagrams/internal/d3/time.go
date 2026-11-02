package d3

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Time scales, intervals and formats (d3-scale's scaleTime, d3-time,
// d3-time-format), in UTC: mermaid's run in the browser's local time, which
// for a drawing only moves where days and hours begin.

// Interval is a d3-time interval: a way to round times down and step them.
type Interval struct {
	floor  func(time.Time) time.Time
	offset func(t time.Time, n int) time.Time
	// field is the number every(n) keeps multiples of; nil counts the
	// intervals since the epoch instead.
	field func(time.Time) int
}

const ms = time.Millisecond

var (
	Millisecond = &Interval{
		floor:  func(t time.Time) time.Time { return t },
		offset: func(t time.Time, n int) time.Time { return t.Add(time.Duration(n) * ms) },
	}
	Second = fixed(time.Second, func(t time.Time) int { return t.Second() })
	Minute = fixed(time.Minute, func(t time.Time) int { return t.Minute() })
	Hour   = fixed(time.Hour, func(t time.Time) int { return t.Hour() })
	Day    = &Interval{
		floor:  func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) },
		offset: func(t time.Time, n int) time.Time { return t.AddDate(0, 0, n) },
		field:  func(t time.Time) int { return t.Day() - 1 },
	}
	Month = &Interval{
		floor:  func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC) },
		offset: func(t time.Time, n int) time.Time { return t.AddDate(0, n, 0) },
		field:  func(t time.Time) int { return int(t.Month()) - 1 },
	}
	Year = &Interval{
		floor:  func(t time.Time) time.Time { return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC) },
		offset: func(t time.Time, n int) time.Time { return t.AddDate(n, 0, 0) },
		field:  func(t time.Time) int { return t.Year() },
	}
)

func fixed(unit time.Duration, field func(time.Time) int) *Interval {
	return &Interval{
		floor:  func(t time.Time) time.Time { return t.Truncate(unit) },
		offset: func(t time.Time, n int) time.Time { return t.Add(time.Duration(n) * unit) },
		field:  field,
	}
}

// Week is the weeks starting on a weekday (d3's timeSunday, timeMonday, ...).
func Week(start time.Weekday) *Interval {
	return &Interval{
		floor: func(t time.Time) time.Time {
			t = Day.floor(t)
			return t.AddDate(0, 0, -((int(t.Weekday()) - int(start) + 7) % 7))
		},
		offset: func(t time.Time, n int) time.Time { return t.AddDate(0, 0, 7*n) },
	}
}

// Floor rounds a time down to the interval.
func (iv *Interval) Floor(t time.Time) time.Time { return iv.floor(t) }

// Ceil rounds a time up to the interval.
func (iv *Interval) Ceil(t time.Time) time.Time {
	return iv.floor(iv.offset(iv.floor(t.Add(-ms)), 1))
}

// Every is interval.every(step): every step-th interval, counted from the
// start of the enclosing field (days of the month, hours of the day, ...)
// or, for weeks, from the epoch.
func (iv *Interval) Every(step int) *Interval {
	if step <= 1 {
		return iv
	}
	switch iv {
	case Millisecond:
		k := time.Duration(step) * ms
		return &Interval{
			floor:  func(t time.Time) time.Time { return t.Truncate(k) },
			offset: func(t time.Time, n int) time.Time { return t.Add(time.Duration(n) * k) },
		}
	case Year:
		return &Interval{
			floor: func(t time.Time) time.Time {
				y := t.Year()
				return time.Date(y-((y%step)+step)%step, 1, 1, 0, 0, 0, 0, time.UTC)
			},
			offset: func(t time.Time, n int) time.Time { return t.AddDate(n*step, 0, 0) },
		}
	}
	test := func(t time.Time) bool {
		if iv.field != nil {
			return iv.field(t)%step == 0
		}
		return iv.count(time.UnixMilli(0).UTC(), t)%step == 0
	}
	return &Interval{
		floor: func(t time.Time) time.Time {
			for t = iv.floor(t); !test(t); t = iv.floor(t.Add(-ms)) {
			}
			return t
		},
		offset: func(t time.Time, n int) time.Time {
			for ; n > 0; n-- {
				for t = iv.offset(t, 1); !test(t); t = iv.offset(t, 1) {
				}
			}
			return t
		},
	}
}

// count is how many interval boundaries lie between two times, for the
// intervals of a fixed length (weeks and shorter, in UTC).
func (iv *Interval) count(a, b time.Time) int {
	a, b = iv.floor(a), iv.floor(b)
	return int(b.Sub(a) / iv.offset(a, 1).Sub(a))
}

// Range is the interval's times from start (rounded up) to before stop.
func (iv *Interval) Range(start, stop time.Time) []time.Time {
	var out []time.Time
	t := iv.Ceil(start)
	for t.Before(stop) {
		out = append(out, t)
		next := iv.floor(iv.offset(t, 1))
		if !next.After(t) {
			break
		}
		t = next
	}
	return out
}

// TimeTicks is scaleTime's ticks from start to stop, both included: on
// interval, or about count of them on an interval d3 picks.
func TimeTicks(start, stop time.Time, iv *Interval, count float64) []time.Time {
	if stop.Before(start) {
		start, stop = stop, start
	}
	if iv == nil {
		iv = tickInterval(start, stop, count)
	}
	return iv.Range(start, stop.Add(ms))
}

const (
	durSecond = 1000.0
	durMinute = 60 * durSecond
	durHour   = 60 * durMinute
	durDay    = 24 * durHour
	durWeek   = 7 * durDay
	durMonth  = 30 * durDay
	durYear   = 365 * durDay
)

var tickIntervals = []struct {
	iv   *Interval
	step int
	dur  float64
}{
	{Second, 1, durSecond}, {Second, 5, 5 * durSecond}, {Second, 15, 15 * durSecond}, {Second, 30, 30 * durSecond},
	{Minute, 1, durMinute}, {Minute, 5, 5 * durMinute}, {Minute, 15, 15 * durMinute}, {Minute, 30, 30 * durMinute},
	{Hour, 1, durHour}, {Hour, 3, 3 * durHour}, {Hour, 6, 6 * durHour}, {Hour, 12, 12 * durHour},
	{Day, 1, durDay}, {Day, 2, 2 * durDay},
	{Week(time.Sunday), 1, durWeek},
	{Month, 1, durMonth}, {Month, 3, 3 * durMonth},
	{Year, 1, durYear},
}

// tickInterval is d3-time's choice of interval for about count ticks.
func tickInterval(start, stop time.Time, count float64) *Interval {
	a, b := float64(start.UnixMilli()), float64(stop.UnixMilli())
	target := (b - a) / count
	i := sort.Search(len(tickIntervals), func(i int) bool { return tickIntervals[i].dur > target })
	switch i {
	case len(tickIntervals):
		return Year.Every(int(TickStep(a/durYear, b/durYear, count)))
	case 0:
		return Millisecond.Every(int(max(TickStep(a, b, count), 1)))
	}
	if target/tickIntervals[i-1].dur < tickIntervals[i].dur/target {
		i--
	}
	return tickIntervals[i].iv.Every(tickIntervals[i].step)
}

var (
	shortDays   = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	shortMonths = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
)

// TimeFormat is d3.timeFormat (en-US locale): strftime-like directives
// (%Y, %m, %d, %H, %M, %b, ...), with the padding modifiers - (none),
// _ (spaces) and 0.
func TimeFormat(spec string) func(time.Time) string {
	return func(t time.Time) string {
		var b strings.Builder
		for i := 0; i < len(spec); i++ {
			if spec[i] != '%' || i+1 >= len(spec) {
				b.WriteByte(spec[i])
				continue
			}
			i++
			pad := byte(0)
			if c := spec[i]; (c == '-' || c == '_' || c == '0') && i+1 < len(spec) {
				pad = c
				i++
			}
			b.WriteString(directive(t, spec[i], pad))
		}
		return b.String()
	}
}

func directive(t time.Time, c, pad byte) string {
	num := func(v, width int, def byte) string {
		if pad != 0 {
			def = pad
		}
		s := fmt.Sprint(v)
		switch def {
		case '-':
			return s
		case '_':
			return strings.Repeat(" ", max(0, width-len(s))) + s
		}
		return strings.Repeat("0", max(0, width-len(s))) + s
	}
	weekNumber := func(start time.Weekday) int {
		w := Week(start)
		return w.count(Year.floor(t).Add(-ms), t)
	}
	switch c {
	case 'a':
		return shortDays[t.Weekday()]
	case 'A':
		return t.Weekday().String()
	case 'b', 'h':
		return shortMonths[t.Month()-1]
	case 'B':
		return t.Month().String()
	case 'c':
		return directive(t, 'x', 0) + ", " + directive(t, 'X', 0)
	case 'x':
		return fmt.Sprintf("%d/%d/%d", t.Month(), t.Day(), t.Year())
	case 'X':
		return directive(t, 'I', '-') + ":" + directive(t, 'M', 0) + ":" + directive(t, 'S', 0) + " " + directive(t, 'p', 0)
	case 'd':
		return num(t.Day(), 2, '0')
	case 'e':
		return num(t.Day(), 2, '_')
	case 'f':
		return num(t.Nanosecond()/1000, 6, '0')
	case 'H':
		return num(t.Hour(), 2, '0')
	case 'I':
		return num((t.Hour()+11)%12+1, 2, '0')
	case 'j':
		return num(t.YearDay(), 3, '0')
	case 'L':
		return num(t.Nanosecond()/1e6, 3, '0')
	case 'm':
		return num(int(t.Month()), 2, '0')
	case 'M':
		return num(t.Minute(), 2, '0')
	case 'p':
		if t.Hour() < 12 {
			return "AM"
		}
		return "PM"
	case 'q':
		return fmt.Sprint((int(t.Month())-1)/3 + 1)
	case 'Q':
		return fmt.Sprint(t.UnixMilli())
	case 's':
		return fmt.Sprint(t.UnixMilli() / 1000)
	case 'S':
		return num(t.Second(), 2, '0')
	case 'u':
		return fmt.Sprint((int(t.Weekday())+6)%7 + 1)
	case 'w':
		return fmt.Sprint(int(t.Weekday()))
	case 'U':
		return num(weekNumber(time.Sunday), 2, '0')
	case 'W':
		return num(weekNumber(time.Monday), 2, '0')
	case 'V':
		_, w := t.ISOWeek()
		return num(w, 2, '0')
	case 'G':
		y, _ := t.ISOWeek()
		return num(y, 4, '0')
	case 'y':
		return num(((t.Year()%100)+100)%100, 2, '0')
	case 'Y':
		return num(t.Year(), 4, '0')
	case 'Z':
		return "+0000"
	case '%':
		return "%"
	}
	return "%" + string(c)
}

package gantt

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Dates are dayjs's, in UTC (mermaid's are in the browser's local time).
// A format without the date ("HH:mm") puts times on refDay, where mermaid
// uses the day it renders on: a fixed day keeps drawings repeatable.
var refDay = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// tokenRe splits a dayjs format into tokens and literals ([escaped]).
var tokenRe = regexp.MustCompile(`\[[^\]]*\]|YYYY|YY|Do|MMMM|MMM|MM|M|DD|D|dddd|ddd|dd|d|HH|H|hh|h|kk|k|mm|m|ss|s|SSS|SS|S|ZZ|Z|A|a|X|x|Q|ww|w`)

var (
	monthNames = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	dayNames   = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
)

// formatDate is dayjs's format (with the advancedFormat plugin's tokens).
func formatDate(t time.Time, format string) string {
	pad := func(v, n int) string {
		s := strconv.Itoa(int(math.Abs(float64(v))))
		if len(s) < n {
			s = strings.Repeat("0", n-len(s)) + s
		}
		if v < 0 {
			s = "-" + s
		}
		return s
	}
	return tokenRe.ReplaceAllStringFunc(format, func(tok string) string {
		h12 := (t.Hour()+11)%12 + 1
		switch tok {
		case "YYYY":
			return pad(t.Year(), 4)
		case "YY":
			return pad(t.Year()%100, 2)
		case "M":
			return strconv.Itoa(int(t.Month()))
		case "MM":
			return pad(int(t.Month()), 2)
		case "MMM":
			return monthNames[t.Month()-1][:3]
		case "MMMM":
			return monthNames[t.Month()-1]
		case "D":
			return strconv.Itoa(t.Day())
		case "DD":
			return pad(t.Day(), 2)
		case "Do":
			return ordinal(t.Day())
		case "d":
			return strconv.Itoa(int(t.Weekday()))
		case "dd":
			return dayNames[t.Weekday()][:2]
		case "ddd":
			return dayNames[t.Weekday()][:3]
		case "dddd":
			return dayNames[t.Weekday()]
		case "H":
			return strconv.Itoa(t.Hour())
		case "HH":
			return pad(t.Hour(), 2)
		case "h":
			return strconv.Itoa(h12)
		case "hh":
			return pad(h12, 2)
		case "k":
			return strconv.Itoa((t.Hour()+23)%24 + 1)
		case "kk":
			return pad((t.Hour()+23)%24+1, 2)
		case "m":
			return strconv.Itoa(t.Minute())
		case "mm":
			return pad(t.Minute(), 2)
		case "s":
			return strconv.Itoa(t.Second())
		case "ss":
			return pad(t.Second(), 2)
		case "S":
			return strconv.Itoa(t.Nanosecond() / 1e8)
		case "SS":
			return pad(t.Nanosecond()/1e7, 2)
		case "SSS":
			return pad(t.Nanosecond()/1e6, 3)
		case "Z":
			return "+00:00"
		case "ZZ":
			return "+0000"
		case "A", "a":
			ap := "AM"
			if t.Hour() >= 12 {
				ap = "PM"
			}
			if tok == "a" {
				ap = strings.ToLower(ap)
			}
			return ap
		case "X":
			return strconv.FormatInt(t.Unix(), 10)
		case "x":
			return strconv.FormatInt(t.UnixMilli(), 10)
		case "Q":
			return strconv.Itoa((int(t.Month())-1)/3 + 1)
		case "w", "ww":
			_, w := t.ISOWeek() // the en locale's week is close enough
			if tok == "ww" {
				return pad(w, 2)
			}
			return strconv.Itoa(w)
		}
		return tok[1 : len(tok)-1] // [literal]
	})
}

func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}

// parsePatterns are what each token reads.
var parsePatterns = map[string]string{
	"YYYY": `\d{4}`, "YY": `\d\d`, "M": `\d\d?`, "MM": `\d\d`, "MMM": `[A-Za-z]+`, "MMMM": `[A-Za-z]+`,
	"D": `\d\d?`, "DD": `\d\d`, "Do": `\d\d?(?:st|nd|rd|th)`, "H": `\d\d?`, "HH": `\d\d`, "h": `\d\d?`, "hh": `\d\d`,
	"k": `\d\d?`, "kk": `\d\d`, "m": `\d\d?`, "mm": `\d\d`, "s": `\d\d?`, "ss": `\d\d`, "S": `\d`, "SS": `\d\d`, "SSS": `\d{3}`,
	"A": `[AaPp][Mm]`, "a": `[AaPp][Mm]`, "Z": `[+-]\d\d:?\d\d|Z`, "ZZ": `[+-]\d\d:?\d\d|Z`, "X": `[+-]?\d+(?:\.\d{1,3})?`, "x": `[+-]?\d+`,
}

// parseDate is dayjs(s, format, true): the format's fields read from s,
// which must then format back to exactly s.
func parseDate(s, format string) (time.Time, bool) {
	if format == "" {
		return time.Time{}, false
	}
	var re strings.Builder
	var toks []string
	re.WriteString("^")
	last := 0
	for _, m := range tokenRe.FindAllStringIndex(format, -1) {
		re.WriteString(regexp.QuoteMeta(format[last:m[0]]))
		tok := format[m[0]:m[1]]
		last = m[1]
		if tok[0] == '[' {
			re.WriteString(regexp.QuoteMeta(tok[1 : len(tok)-1]))
			continue
		}
		pat := parsePatterns[tok]
		if pat == "" {
			pat = `[^\s]+?` // day names, weeks, quarters: read and ignored
		}
		re.WriteString("(" + pat + ")")
		toks = append(toks, tok)
	}
	re.WriteString(regexp.QuoteMeta(format[last:]) + "$")
	m := regexp.MustCompile(re.String()).FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	year, month, day, hour, min, sec, nsec := 0, 0, 0, 0, 0, 0, 0
	pm, ampm := false, false
	var direct *time.Time
	for i, tok := range toks {
		v := m[i+1]
		n, _ := strconv.Atoi(v)
		switch tok {
		case "YYYY":
			year = n
		case "YY":
			year = n + 2000
			if n > 68 {
				year = n + 1900
			}
		case "M", "MM":
			month = n
		case "MMM", "MMMM":
			for k, name := range monthNames {
				if strings.EqualFold(v, name) || strings.EqualFold(v, name[:3]) {
					month = k + 1
				}
			}
		case "D", "DD":
			day = n
		case "Do":
			day, _ = strconv.Atoi(strings.TrimRight(v, "stndrh"))
		case "H", "HH", "h", "hh", "k", "kk":
			hour = n
		case "m", "mm":
			min = n
		case "s", "ss":
			sec = n
		case "S":
			nsec = n * 1e8
		case "SS":
			nsec = n * 1e7
		case "SSS":
			nsec = n * 1e6
		case "A", "a":
			ampm, pm = true, strings.EqualFold(v, "pm")
		case "X":
			f, _ := strconv.ParseFloat(v, 64)
			t := time.UnixMilli(int64(math.Round(f * 1000))).UTC()
			direct = &t
		case "x":
			t := time.UnixMilli(int64(n)).UTC()
			direct = &t
		}
	}
	var t time.Time
	if direct != nil {
		t = *direct
	} else {
		if ampm && pm && hour < 12 {
			hour += 12
		} else if ampm && !pm && hour == 12 {
			hour = 0
		}
		// Missing fields as dayjs: the year and month default to the
		// reference day's, the day to 1 when a year or month is given.
		y, mo, d := year, month, day
		if y == 0 {
			y = refDay.Year()
		}
		if mo == 0 {
			mo = int(refDay.Month())
			if year != 0 {
				mo = 1
			}
		}
		if d == 0 {
			d = refDay.Day()
			if year != 0 || month != 0 {
				d = 1
			}
		}
		t = time.Date(y, time.Month(mo), d, hour, min, sec, nsec, time.UTC)
	}
	if formatDate(t, format) != s {
		return time.Time{}, false
	}
	return t, true
}

// jsDate is JavaScript's new Date(s) for the ISO forms (date-only ones in
// UTC, as JavaScript reads them).
func jsDate(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", "2006-01", "2006", "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05.000Z07:00", "2006-01-02 15:04", "2006-01-02 15:04:05", "2006/01/02", "1/2/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

var durationRe = regexp.MustCompile(`^(\d+(?:\.\d+)?)(ms|[Mdhmswy])$`)

// addDuration adds a duration such as "3d" or "20h" as dayjs's add does
// (days and weeks rounded to whole days; months and years keep the day,
// clamped to the month's length); ok is false for anything else.
func addDuration(t time.Time, s string) (time.Time, bool) {
	m := durationRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return t, false
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	switch m[2] {
	case "ms":
		return t.Add(time.Duration(n * float64(time.Millisecond))), true
	case "s":
		return t.Add(time.Duration(n * float64(time.Second))), true
	case "m":
		return t.Add(time.Duration(n * float64(time.Minute))), true
	case "h":
		return t.Add(time.Duration(n * float64(time.Hour))), true
	case "d":
		return t.AddDate(0, 0, int(math.Floor(n+0.5))), true
	case "w":
		return t.AddDate(0, 0, int(math.Floor(7*n+0.5))), true
	case "M":
		return addMonths(t, int(n)), true
	case "y":
		return addMonths(t, 12*int(n)), true
	}
	return t, false
}

func addMonths(t time.Time, n int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(n), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(t.Day(), last)-1)
}

func dateErr(s string) error { return fmt.Errorf("gantt: invalid date: %s", s) }

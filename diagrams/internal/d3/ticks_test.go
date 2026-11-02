package d3

import (
	"fmt"
	"testing"
	"time"
)

// Expected ticks are those of mermaid.js's drawings (data/mermaid-port/ref:
// the xychart y axes and the gantt time axes).
func TestTicks(t *testing.T) {
	for _, c := range []struct {
		lo, hi float64
		want   string
	}{
		{4000, 11000, "[4000 4500 5000 5500 6000 6500 7000 7500 8000 8500 9000 9500 10000 10500 11000]"},
		{0, 198.2, "[0 20 40 60 80 100 120 140 160 180]"},
		{0, 600, "[0 50 100 150 200 250 300 350 400 450 500 550 600]"},
		{1, 4, "[1 1.2 1.4 1.6 1.8 2 2.2 2.4 2.6 2.8 3 3.2 3.4 3.6 3.8 4]"},
		{50, 0, "[50 45 40 35 30 25 20 15 10 5 0]"},
	} {
		var got []string
		for _, v := range Ticks(c.lo, c.hi, 10) {
			got = append(got, Num(v))
		}
		if s := fmt.Sprint(got); s != c.want {
			t.Errorf("Ticks(%v, %v) = %s, want %s", c.lo, c.hi, s, c.want)
		}
	}
	for v, want := range map[float64]string{0.30000000000000004: "0.30000000000000004", 1e21: "1e+21", 1.5e-7: "1.5e-7", -42: "-42"} {
		if got := Num(v); got != want {
			t.Errorf("Num(%v) = %s, want %s", v, got, want)
		}
	}
}

func TestTimeTicks(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2014, 1, d, 0, 0, 0, 0, time.UTC) }
	clock := func(h, m int) time.Time { return time.Date(2024, 1, 1, h, m, 0, 0, time.UTC) }
	for _, c := range []struct {
		lo, hi time.Time
		iv     *Interval
		format string
		want   string
	}{
		{day(1), day(1).AddDate(0, 0, 50), nil, "%Y-%m-%d", "2014-01-05 2014-01-12 2014-01-19 2014-01-26 2014-02-02 2014-02-09 2014-02-16"},
		{day(6), day(28), nil, "%Y-%m-%d", "2014-01-07 2014-01-09 2014-01-11 2014-01-13 2014-01-15 2014-01-17 2014-01-19 2014-01-21 2014-01-23 2014-01-25 2014-01-27"},
		{clock(17, 49), clock(18, 12), nil, "%H:%M", "17:50 17:55 18:00 18:05 18:10"},
		{time.UnixMilli(0).UTC(), time.UnixMilli(71000).UTC(), nil, "%s", "0 5 10 15 20 25 30 35 40 45 50 55 60 65 70"},
		{time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(1935, 1, 1, 0, 0, 0, 0, time.UTC), nil, "%y", "00 05 10 15 20 25 30 35"},
		{day(1), day(31), Week(time.Monday).Every(1), "%a %d", "Mon 06 Mon 13 Mon 20 Mon 27"},
		{day(1), day(31), Day.Every(10), "%-d %b", "1 Jan 11 Jan 21 Jan 31 Jan"},
	} {
		var got []string
		f := TimeFormat(c.format)
		for _, tk := range TimeTicks(c.lo, c.hi, c.iv, 10) {
			got = append(got, f(tk))
		}
		if s := fmt.Sprint(got); s != "["+c.want+"]" {
			t.Errorf("ticks %v..%v = %s, want [%s]", c.lo, c.hi, s, c.want)
		}
	}
}

package xychart

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/d3"
)

// Chart is a parsed xychart, with mermaid's db rules applied: an axis left
// out is derived from the data.
type Chart struct {
	Title      string
	Horizontal bool
	X, Y       Axis
	Plots      []Plot
}

// Axis is a band axis (Categories) or a linear one (Min..Max).
type Axis struct {
	Title      string
	Band       bool
	Categories []string
	Min, Max   float64
}

// Plot is a bar or line series: values at x positions (categories, or
// numbers written as text for a linear x axis), and a line's point labels.
type Plot struct {
	Bar    bool
	Title  string
	Points []Point
	Labels []string
}

// Point is a value at an x position.
type Point struct {
	X string
	Y float64
}

// scanner reads a line of the chart (xychart.jison's lexer, by hand).
type scanner struct {
	s    string
	line int
}

var (
	numberRe = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d+)?|\.\d+)`)
	wordRe   = regexp.MustCompile(`^[A-Za-z0-9+=*#_.&-]+`)
)

func (sc *scanner) errorf(format string, args ...any) error {
	return fmt.Errorf("xychart: line %d: %s", sc.line, fmt.Sprintf(format, args...))
}

func (sc *scanner) skip() { sc.s = strings.TrimLeft(sc.s, " \t\r") }

func (sc *scanner) eol() bool { sc.skip(); return sc.s == "" }

// accept takes a literal (case-insensitive).
func (sc *scanner) accept(lit string) bool {
	sc.skip()
	if len(sc.s) >= len(lit) && strings.EqualFold(sc.s[:len(lit)], lit) {
		sc.s = sc.s[len(lit):]
		return true
	}
	return false
}

func (sc *scanner) number() (float64, bool) {
	sc.skip()
	m := numberRe.FindString(sc.s)
	if m == "" {
		return 0, false
	}
	sc.s = sc.s[len(m):]
	v, _ := strconv.ParseFloat(m, 64)
	return v, true
}

// text is a label: "quoted", "`markdown`", or bare words (letters, digits
// and + = * # _ . & -; mermaid glues bare words together, we keep the
// spaces between them).
func (sc *scanner) text() (string, bool) {
	sc.skip()
	switch {
	case strings.HasPrefix(sc.s, "\"`"):
		end := strings.Index(sc.s[2:], "`\"")
		if end < 0 {
			return "", false
		}
		t := sc.s[2 : 2+end]
		sc.s = sc.s[4+end:]
		return t, true
	case strings.HasPrefix(sc.s, `"`):
		end := strings.IndexByte(sc.s[1:], '"')
		if end < 0 {
			return "", false
		}
		t := sc.s[1 : 1+end]
		sc.s = sc.s[2+end:]
		return t, true
	}
	var words []string
	for {
		w := wordRe.FindString(sc.s)
		if w == "" || strings.HasPrefix(w, "-->") {
			break
		}
		words = append(words, w)
		sc.s = sc.s[len(w):]
		rest := strings.TrimLeft(sc.s, " \t")
		if wordRe.FindString(rest) == "" {
			break
		}
		sc.s = rest
	}
	return strings.Join(words, " "), len(words) > 0
}

// Parse reads an xychart (xychart.jison) and applies its db's rules.
func Parse(code string) (*Chart, error) {
	c := &Chart{Y: Axis{Min: math.Inf(1), Max: math.Inf(-1)}, X: Axis{Band: true}}
	var setX, setY, started bool
	sanitize := func(s string) string { return common.SanitizeText(strings.TrimSpace(s)) }
	// addPlot is the db's setLineData/setBarData.
	addPlot := func(bar bool, title string, values []float64, labels []string) {
		if len(values) > 0 {
			if !setX {
				lo, hi := 1.0, float64(len(values))
				if !c.X.Band {
					lo, hi = math.Min(c.X.Min, lo), math.Max(c.X.Max, hi)
				}
				c.X = Axis{Title: c.X.Title, Min: lo, Max: hi}
				setX = true
			}
			if c.X.Band && len(values) > len(c.X.Categories) {
				values = values[:len(c.X.Categories)]
			}
			if !setY {
				lo, hi := math.Inf(1), math.Inf(-1)
				for _, v := range values {
					lo, hi = math.Min(lo, v), math.Max(hi, v)
				}
				c.Y = Axis{Title: c.Y.Title, Min: math.Min(c.Y.Min, lo), Max: math.Max(c.Y.Max, hi)}
			}
		}
		p := Plot{Bar: bar, Title: sanitize(title)}
		switch {
		case len(values) == 0:
		case c.X.Band:
			for i, cat := range c.X.Categories {
				v := math.NaN()
				if i < len(values) {
					v = values[i]
				}
				p.Points = append(p.Points, Point{cat, v})
			}
		case len(values) == 1:
			p.Points = []Point{{d3.Num(c.X.Min), values[0]}}
		default:
			step := (c.X.Max - c.X.Min) / float64(len(values)-1)
			for i, v := range values {
				p.Points = append(p.Points, Point{d3.Num(c.X.Min + float64(i)*step), v})
			}
		}
		if !bar {
			for _, l := range labels {
				if l != "" {
					p.Labels = labels
					break
				}
			}
		}
		c.Plots = append(c.Plots, p)
	}

	inDescr := false
	for n, raw := range strings.Split(code, "\n") {
		if i := strings.Index(raw, "%%"); i >= 0 {
			raw = raw[:i]
		}
		if inDescr {
			if _, _, ok := strings.Cut(raw, "}"); ok {
				inDescr = false
			}
			continue
		}
		for _, stmt := range statements(raw) {
			sc := &scanner{s: stmt, line: n + 1}
			if sc.eol() {
				continue
			}
			if !started {
				if !sc.accept("xychart-beta") && !sc.accept("xychart") {
					return nil, sc.errorf("expected xychart")
				}
				started = true
				switch {
				case sc.accept("horizontal"):
					c.Horizontal = true
				case sc.accept("vertical"):
				}
				if sc.eol() {
					continue
				}
			}
			switch {
			case sc.accept("accTitle"):
				continue
			case sc.accept("accDescr"):
				if !strings.HasPrefix(strings.TrimSpace(sc.s), ":") {
					inDescr = !strings.Contains(sc.s, "}")
				}
				continue
			case sc.accept("title"):
				t, _ := sc.text()
				c.Title = sanitize(t)
			case sc.accept("x-axis"):
				title, data, err := axisStatement(sc, &c.X, true)
				if err != nil {
					return nil, err
				}
				c.X.Title = sanitize(title)
				for i, cat := range c.X.Categories {
					c.X.Categories[i] = sanitize(cat)
				}
				setX = setX || data
			case sc.accept("y-axis"):
				title, data, err := axisStatement(sc, &c.Y, false)
				if err != nil {
					return nil, err
				}
				c.Y.Title = sanitize(title)
				setY = setY || data
			case sc.accept("line"), sc.accept("bar"):
				bar := !strings.HasPrefix(strings.ToLower(strings.TrimSpace(stmt)), "line")
				title := ""
				if !strings.HasPrefix(strings.TrimSpace(sc.s), "[") {
					title, _ = sc.text()
				}
				values, labels, err := plotData(sc)
				if err != nil {
					return nil, err
				}
				addPlot(bar, title, values, labels)
			default:
				return nil, sc.errorf("unexpected %q", strings.TrimSpace(sc.s))
			}
			if !sc.eol() {
				return nil, sc.errorf("unexpected %q", strings.TrimSpace(sc.s))
			}
		}
	}
	if !started {
		return nil, fmt.Errorf("xychart: empty")
	}
	if len(c.Plots) == 0 {
		return nil, fmt.Errorf("xychart: no plot to render, please provide a plot with some data")
	}
	return c, nil
}

// statements splits a line at the semicolons outside quotes.
func statements(line string) []string {
	var out []string
	quoted, start := false, 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			quoted = !quoted
		case ';':
			if !quoted {
				out, start = append(out, line[start:i]), i+1
			}
		}
	}
	return append(out, line[start:])
}

// axisStatement reads an axis: `title`, `title [a, b]`, `[a, b]` (x only),
// `title min --> max` or `min --> max`; data reports whether it gave the
// axis's categories or range.
func axisStatement(sc *scanner, a *Axis, band bool) (title string, data bool, err error) {
	if v, ok := sc.number(); ok {
		return "", true, axisRange(sc, a, v)
	}
	if !strings.HasPrefix(strings.TrimSpace(sc.s), "[") {
		title, _ = sc.text()
	}
	switch {
	case sc.eol():
		return title, false, nil
	case band && sc.accept("["):
		var cats []string
		for {
			t, ok := sc.text()
			if !ok {
				return "", false, sc.errorf("expected a category")
			}
			cats = append(cats, t)
			if !sc.accept(",") {
				break
			}
		}
		if !sc.accept("]") {
			return "", false, sc.errorf("expected ]")
		}
		*a = Axis{Band: true, Categories: cats}
		return title, true, nil
	}
	v, ok := sc.number()
	if !ok {
		return "", false, sc.errorf("expected axis data")
	}
	return title, true, axisRange(sc, a, v)
}

func axisRange(sc *scanner, a *Axis, lo float64) error {
	if !sc.accept("-->") {
		return sc.errorf("expected -->")
	}
	hi, ok := sc.number()
	if !ok {
		return sc.errorf("expected a number")
	}
	*a = Axis{Min: lo, Max: hi}
	return nil
}

// plotData reads `[v "label", v, ...]`.
func plotData(sc *scanner) ([]float64, []string, error) {
	if !sc.accept("[") {
		return nil, nil, sc.errorf("expected [")
	}
	var values []float64
	var labels []string
	for {
		v, ok := sc.number()
		if !ok {
			return nil, nil, sc.errorf("expected a number")
		}
		label := ""
		if strings.HasPrefix(strings.TrimLeft(sc.s, " \t"), `"`) {
			label, _ = sc.text()
			label = common.SanitizeText(strings.TrimSpace(label))
		}
		values, labels = append(values, v), append(labels, label)
		if !sc.accept(",") {
			break
		}
	}
	if !sc.accept("]") {
		return nil, nil, sc.errorf("expected ]")
	}
	return values, labels, nil
}

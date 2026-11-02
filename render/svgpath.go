package render

import (
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// parsePath reads SVG path data (M L H V C S Q T A Z, absolute and
// relative) into segments. Malformed data stops the parse.
func parsePath(d string) []scene.Seg {
	var b scene.PathBuilder
	toks := pathTokens(d)
	i := 0
	num := func() (float64, bool) {
		if i >= len(toks) || isCmd(toks[i]) {
			return 0, false
		}
		f, err := strconv.ParseFloat(toks[i], 64)
		i++
		return f, err == nil
	}
	var cmd byte
	var start, last2 P // subpath start; last control point (for S/T)
	var lastCmd byte
	for i < len(toks) {
		if isCmd(toks[i]) {
			cmd = toks[i][0]
			i++
		} else if cmd == 0 {
			return b.Segs
		}
		rel := cmd >= 'a'
		c := b.Current()
		ox, oy := 0.0, 0.0
		if rel {
			ox, oy = c.X, c.Y
		}
		up := cmd &^ 0x20
		switch up {
		case 'Z':
			b.Close()
			lastCmd = 'Z'
			continue
		case 'M':
			x, ok1 := num()
			y, ok2 := num()
			if !ok1 || !ok2 {
				return b.Segs
			}
			b.MoveTo(x+ox, y+oy)
			start = P{X: x + ox, Y: y + oy}
			_ = start
			// Further pairs are line-tos.
			if rel {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'L':
			x, ok1 := num()
			y, ok2 := num()
			if !ok1 || !ok2 {
				return b.Segs
			}
			b.LineTo(x+ox, y+oy)
		case 'H':
			x, ok := num()
			if !ok {
				return b.Segs
			}
			b.LineTo(x+ox, c.Y)
		case 'V':
			y, ok := num()
			if !ok {
				return b.Segs
			}
			if rel {
				b.LineTo(c.X, c.Y+y)
			} else {
				b.LineTo(c.X, y)
			}
		case 'C':
			var v [6]float64
			for k := range v {
				f, ok := num()
				if !ok {
					return b.Segs
				}
				v[k] = f
			}
			b.CubeTo(v[0]+ox, v[1]+oy, v[2]+ox, v[3]+oy, v[4]+ox, v[5]+oy)
			last2 = P{X: v[2] + ox, Y: v[3] + oy}
		case 'S':
			var v [4]float64
			for k := range v {
				f, ok := num()
				if !ok {
					return b.Segs
				}
				v[k] = f
			}
			c1 := c
			if lastCmd == 'C' || lastCmd == 'S' {
				c1 = P{X: 2*c.X - last2.X, Y: 2*c.Y - last2.Y}
			}
			b.CubeTo(c1.X, c1.Y, v[0]+ox, v[1]+oy, v[2]+ox, v[3]+oy)
			last2 = P{X: v[0] + ox, Y: v[1] + oy}
		case 'Q':
			var v [4]float64
			for k := range v {
				f, ok := num()
				if !ok {
					return b.Segs
				}
				v[k] = f
			}
			b.QuadTo(v[0]+ox, v[1]+oy, v[2]+ox, v[3]+oy)
			last2 = P{X: v[0] + ox, Y: v[1] + oy}
		case 'T':
			var v [2]float64
			for k := range v {
				f, ok := num()
				if !ok {
					return b.Segs
				}
				v[k] = f
			}
			c1 := c
			if lastCmd == 'Q' || lastCmd == 'T' {
				c1 = P{X: 2*c.X - last2.X, Y: 2*c.Y - last2.Y}
			}
			b.QuadTo(c1.X, c1.Y, v[0]+ox, v[1]+oy)
			last2 = c1
		case 'A':
			var v [7]float64
			for k := range v {
				f, ok := num()
				if !ok {
					return b.Segs
				}
				v[k] = f
			}
			b.ArcTo(v[0], v[1], v[2], v[3] != 0, v[4] != 0, v[5]+ox, v[6]+oy)
		default:
			return b.Segs
		}
		lastCmd = up
	}
	return b.Segs
}

func isCmd(t string) bool {
	return len(t) == 1 && strings.ContainsRune("MmLlHhVvCcSsQqTtAaZz", rune(t[0]))
}

// pathTokens splits path data into commands and numbers.
func pathTokens(d string) []string {
	var out []string
	i := 0
	for i < len(d) {
		c := d[i]
		switch {
		case c == ' ' || c == ',' || c == '\n' || c == '\t' || c == '\r':
			i++
		case isCmd(string(c)):
			out = append(out, string(c))
			i++
		default:
			j := i
			if d[j] == '-' || d[j] == '+' {
				j++
			}
			dot := false
			for j < len(d) && (d[j] >= '0' && d[j] <= '9' || d[j] == '.' && !dot) {
				if d[j] == '.' {
					dot = true
				}
				j++
			}
			if j < len(d) && (d[j] == 'e' || d[j] == 'E') {
				j++
				if j < len(d) && (d[j] == '-' || d[j] == '+') {
					j++
				}
				for j < len(d) && d[j] >= '0' && d[j] <= '9' {
					j++
				}
			}
			if j == i {
				return out
			}
			out = append(out, d[i:j])
			i = j
		}
	}
	return out
}

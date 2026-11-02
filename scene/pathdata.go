package scene

import "strconv"

// ParsePath reads SVG path data (all of M L H V C S Q T A Z, absolute and
// relative, repeated arguments, numbers run together as minifiers write them:
// "1.5.5", "-.4-.2", arc flags without separators). It stops at the first
// malformed command and returns what came before.
func ParsePath(d string) []Seg {
	p := pathParser{d: d}
	var b PathBuilder
	var cmd byte
	var ctrl Pt // the last control point, for S and T
	var prev byte
	for {
		p.skip()
		if p.i >= len(p.d) {
			break
		}
		if c := p.d[p.i]; isPathCmd(c) {
			cmd = c
			p.i++
		} else if cmd == 0 || cmd == 'Z' || cmd == 'z' {
			break
		}
		cur := b.Current()
		var o Pt // relative commands' origin
		if cmd >= 'a' {
			o = cur
		}
		up := cmd &^ 0x20
		args := map[byte]int{'M': 2, 'L': 2, 'H': 1, 'V': 1, 'C': 6, 'S': 4, 'Q': 4, 'T': 2, 'A': 7, 'Z': 0}[up]
		var v [7]float64
		for k := range args {
			var ok bool
			if up == 'A' && (k == 3 || k == 4) {
				v[k], ok = p.flag()
			} else {
				v[k], ok = p.num()
			}
			if !ok {
				return b.Segs
			}
		}
		switch up {
		case 'Z':
			b.Close()
		case 'M':
			b.MoveTo(v[0]+o.X, v[1]+o.Y)
			// Further pairs are line-tos.
			cmd -= 'M' - 'L'
		case 'L':
			b.LineTo(v[0]+o.X, v[1]+o.Y)
		case 'H':
			b.LineTo(v[0]+o.X, cur.Y)
		case 'V':
			b.LineTo(cur.X, v[0]+o.Y)
		case 'C':
			b.CubeTo(v[0]+o.X, v[1]+o.Y, v[2]+o.X, v[3]+o.Y, v[4]+o.X, v[5]+o.Y)
			ctrl = Pt{v[2] + o.X, v[3] + o.Y}
		case 'S':
			c1 := cur
			if prev == 'C' || prev == 'S' {
				c1 = Pt{2*cur.X - ctrl.X, 2*cur.Y - ctrl.Y}
			}
			b.CubeTo(c1.X, c1.Y, v[0]+o.X, v[1]+o.Y, v[2]+o.X, v[3]+o.Y)
			ctrl = Pt{v[0] + o.X, v[1] + o.Y}
		case 'Q':
			b.QuadTo(v[0]+o.X, v[1]+o.Y, v[2]+o.X, v[3]+o.Y)
			ctrl = Pt{v[0] + o.X, v[1] + o.Y}
		case 'T':
			c1 := cur
			if prev == 'Q' || prev == 'T' {
				c1 = Pt{2*cur.X - ctrl.X, 2*cur.Y - ctrl.Y}
			}
			b.QuadTo(c1.X, c1.Y, v[0]+o.X, v[1]+o.Y)
			ctrl = c1
		case 'A':
			b.ArcTo(v[0], v[1], v[2], v[3] != 0, v[4] != 0, v[5]+o.X, v[6]+o.Y)
		}
		prev = up
	}
	return b.Segs
}

func isPathCmd(c byte) bool {
	switch c | 0x20 {
	case 'm', 'l', 'h', 'v', 'c', 's', 'q', 't', 'a', 'z':
		return true
	}
	return false
}

type pathParser struct {
	d string
	i int
}

func (p *pathParser) skip() {
	for p.i < len(p.d) {
		switch p.d[p.i] {
		case ' ', ',', '\t', '\n', '\r', '\f':
			p.i++
		default:
			return
		}
	}
}

// num reads a number: sign, digits with at most one dot, exponent.
func (p *pathParser) num() (float64, bool) {
	p.skip()
	d, j := p.d, p.i
	if j < len(d) && (d[j] == '-' || d[j] == '+') {
		j++
	}
	digits, dot := 0, false
	for ; j < len(d); j++ {
		if d[j] >= '0' && d[j] <= '9' {
			digits++
		} else if d[j] == '.' && !dot {
			dot = true
		} else {
			break
		}
	}
	if digits == 0 {
		return 0, false
	}
	if j < len(d) && (d[j] == 'e' || d[j] == 'E') {
		k := j + 1
		if k < len(d) && (d[k] == '-' || d[k] == '+') {
			k++
		}
		if k < len(d) && d[k] >= '0' && d[k] <= '9' {
			for j = k; j < len(d) && d[j] >= '0' && d[j] <= '9'; j++ {
			}
		}
	}
	f, err := strconv.ParseFloat(d[p.i:j], 64)
	p.i = j
	return f, err == nil
}

// flag reads an arc flag: a single 0 or 1.
func (p *pathParser) flag() (float64, bool) {
	p.skip()
	if p.i < len(p.d) && (p.d[p.i] == '0' || p.d[p.i] == '1') {
		p.i++
		return float64(p.d[p.i-1] - '0'), true
	}
	return 0, false
}

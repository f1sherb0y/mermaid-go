// Package packet is mermaid's packet diagram (diagrams/packet): bit fields
// in rows of bitsPerRow bits, a field running past a row's end continuing on
// the next.
package packet

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

func init() { diagram.Register("packet", Render) }

// Block is a field, or the part of one in a row.
type Block struct {
	Start, End int
	Label      string
}

// Packet is a parsed packet diagram.
type Packet struct {
	Title string
	Rows  [][]Block
}

const maxRows = 10000

// Parse reads `packet` (or `packet-beta`), then title/acc lines and fields:
// `start[-end]: "label"` or `+bits: "label"` (packet.langium).
func Parse(code string, bitsPerRow int) (*Packet, error) {
	p := &Packet{}
	lines := strings.Split(code, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i == len(lines) {
		return nil, fmt.Errorf("packet: empty")
	}
	head := strings.TrimSpace(lines[i])
	switch {
	case strings.HasPrefix(head, "packet-beta"):
		head = head[len("packet-beta"):]
	case strings.HasPrefix(head, "packet"):
		head = head[len("packet"):]
	default:
		return nil, fmt.Errorf("packet: line %d: expected packet", i+1)
	}
	lines[i] = head
	last := -1
	var word []Block
	row := 1
	push := func() {
		if len(word) > 0 && len(p.Rows) < maxRows {
			p.Rows = append(p.Rows, word)
		}
		word = nil
	}
	for n := i; n < len(lines); n++ {
		l := strings.TrimSpace(lines[n])
		switch {
		case l == "":
			continue
		case l == "title" || strings.HasPrefix(l, "title ") || strings.HasPrefix(l, "title\t"):
			p.Title = strings.TrimSpace(l[len("title"):])
			continue
		case strings.HasPrefix(l, "accTitle") || strings.HasPrefix(l, "accDescr"):
			if strings.HasPrefix(l, "accDescr") && strings.Contains(l, "{") && !strings.Contains(l, "}") {
				for n+1 < len(lines) && !strings.Contains(lines[n], "}") {
					n++
				}
			}
			continue
		}
		start, end, bits, label, err := field(l)
		if err != nil {
			return nil, fmt.Errorf("packet: line %d: %v", n+1, err)
		}
		if start >= 0 && end >= 0 && end < start {
			return nil, fmt.Errorf("Packet block %d - %d is invalid. End must be greater than start.", start, end)
		}
		if start < 0 {
			start = last + 1
		}
		if start != last+1 {
			e := end
			if e < 0 {
				e = start
			}
			return nil, fmt.Errorf("Packet block %d - %d is not contiguous. It should start from %d.", start, e, last+1)
		}
		if bits == 0 {
			return nil, fmt.Errorf("Packet block %d is invalid. Cannot have a zero bit field.", start)
		}
		if end < 0 {
			if bits < 0 {
				bits = 1
			}
			end = start + bits - 1
		}
		last = end
		// Split at row ends.
		for len(p.Rows) < maxRows {
			b := Block{start, end, label}
			rowEnd := row*bitsPerRow - 1
			split := end > rowEnd
			if split {
				b.End = rowEnd
			}
			word = append(word, b)
			if b.End == rowEnd {
				push()
				row++
			}
			if !split {
				break
			}
			start = rowEnd + 1
		}
	}
	push()
	return p, nil
}

// field reads `start[-end]: "label"` or `+bits: "label"`; -1 for what is
// not given.
func field(l string) (start, end, bits int, label string, err error) {
	start, end, bits = -1, -1, -1
	num := func() (int, error) {
		j := 0
		for j < len(l) && l[j] >= '0' && l[j] <= '9' {
			j++
		}
		if j == 0 {
			return 0, fmt.Errorf("expected a number in %q", l)
		}
		v, _ := strconv.Atoi(l[:j])
		l = strings.TrimSpace(l[j:])
		return v, nil
	}
	if strings.HasPrefix(l, "+") {
		l = strings.TrimSpace(l[1:])
		if bits, err = num(); err != nil {
			return
		}
	} else {
		if start, err = num(); err != nil {
			return
		}
		if strings.HasPrefix(l, "-") {
			l = strings.TrimSpace(l[1:])
			if end, err = num(); err != nil {
				return
			}
		}
	}
	if !strings.HasPrefix(l, ":") {
		return 0, 0, 0, "", fmt.Errorf("expected ':' in %q", l)
	}
	l = strings.TrimSpace(l[1:])
	if len(l) < 2 || (l[0] != '"' && l[0] != '\'') || l[len(l)-1] != l[0] {
		return 0, 0, 0, "", fmt.Errorf("expected a quoted label, got %q", l)
	}
	return start, end, bits, l[1 : len(l)-1], nil
}

// Render draws a packet diagram.
func Render(code string, ctx *diagram.Context) (*scene.Scene, error) {
	cfg := ctx.Config
	bitsPerRow := int(cfg.Num("packet.bitsPerRow", 32))
	p, err := Parse(code, bitsPerRow)
	if err != nil {
		return nil, err
	}
	if p.Title == "" {
		p.Title = ctx.Title
	}
	rowH := cfg.Num("packet.rowHeight", 32)
	padX := cfg.Num("packet.paddingX", 5)
	padY := cfg.Num("packet.paddingY", 5)
	bitW := cfg.Num("packet.bitWidth", 32)
	showBits := cfg.Bool("packet.showBits", true)
	descending := cfg.Str("packet.bitOrder", "ascending") == "descending"
	if showBits {
		padY += 10
	}
	// Style: the theme's packet object over these defaults.
	opt := func(name, def string) string {
		if v := ctx.Theme.Get("packet." + name); v != "" {
			return v
		}
		return def
	}
	family := ctx.Theme.Get("fontFamily")
	label := func(x, y float64, s, size, col string, a scene.Anchor, b scene.Baseline) *scene.Text {
		return &scene.Text{X: x, Y: y, Anchor: a, Baseline: b, Family: family, Size: style.Px(size, 16),
			Color: style.Color(col), Runs: []scene.Span{{Text: s}}}
	}
	total := rowH + padY
	h := total*float64(len(p.Rows)+1) - rowH
	if p.Title != "" {
		h += rowH
	}
	w := bitW*float64(bitsPerRow) + 2
	var items []scene.Item
	for r, word := range p.Rows {
		y := float64(r)*(rowH+padY) + padY
		for _, b := range word {
			bits := b.End - b.Start + 1
			col := b.Start % bitsPerRow
			if descending {
				col = bitsPerRow - col - bits
			}
			x := float64(col)*bitW + 1
			bw := float64(bits)*bitW - padX
			items = append(items,
				&scene.Path{Segs: scene.RectPath(x, y, bw, rowH, 0, 0), Fill: style.Color(opt("blockFillColor", "#efefef")),
					Stroke: style.Color(opt("blockStrokeColor", "black")), StrokeWidth: style.Px(opt("blockStrokeWidth", "1"), 1)},
				label(x+bw/2, y+rowH/2, b.Label, opt("labelFontSize", "12px"), opt("labelColor", "black"), scene.Middle, scene.MiddleBaseline))
			if !showBits {
				continue
			}
			lead, trail := b.Start, b.End
			if descending {
				lead, trail = b.End, b.Start
			}
			byteSize := opt("byteFontSize", "10px")
			if bits == 1 {
				items = append(items, label(x+bw/2, y-2, strconv.Itoa(lead), byteSize, opt("startByteColor", "black"), scene.Middle, scene.Alphabetic))
				continue
			}
			items = append(items,
				label(x, y-2, strconv.Itoa(lead), byteSize, opt("startByteColor", "black"), scene.Start, scene.Alphabetic),
				label(x+bw, y-2, strconv.Itoa(trail), byteSize, opt("endByteColor", "black"), scene.End, scene.Alphabetic))
		}
	}
	items = append(items, label(w/2, h-total/2, p.Title, opt("titleFontSize", "14px"), opt("titleColor", "black"), scene.Middle, scene.MiddleBaseline))
	return &scene.Scene{ViewBox: scene.Rect{W: w, H: h}, Items: items}, nil
}

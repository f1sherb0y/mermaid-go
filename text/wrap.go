package text

import (
	"strings"

	"github.com/f1sherb0y/mermaid-go/internal/typesetting/segmenter"
)

// Wrap breaks a line into lines no wider than width where it can, as a
// browser wraps text (white-space: normal or break-spaces): at the line break
// opportunities of UAX #14 (spaces, between CJK characters, after hyphens),
// filling each line greedily. A piece wider than width is left whole, as a
// browser lets it overflow. Spaces at the end of a line are dropped.
func (fs *Fonts) Wrap(s string, st Style, width float64) []string {
	if fs.Width(s, st) <= width {
		return []string{s}
	}
	var seg segmenter.Segmenter
	seg.Init([]rune(s))
	var pieces []string
	it := seg.LineIterator()
	for it.Next() {
		pieces = append(pieces, string(it.Line().Text))
	}
	var lines []string
	cur := ""
	for _, p := range pieces {
		if cur != "" && fs.Width(strings.TrimRight(cur+p, " "), st) > width {
			lines = append(lines, strings.TrimRight(cur, " "))
			cur = ""
		}
		cur += p
	}
	if cur != "" || len(lines) == 0 {
		lines = append(lines, strings.TrimRight(cur, " "))
	}
	return lines
}

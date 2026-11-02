package text

import (
	"math"
	"unicode"

	"github.com/f1sherb0y/mermaid-go/internal/typesetting/di"
	"github.com/f1sherb0y/mermaid-go/internal/typesetting/font"
	"github.com/f1sherb0y/mermaid-go/internal/typesetting/language"
	"github.com/f1sherb0y/mermaid-go/internal/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// Style is how a piece of text is set.
type Style struct {
	// Family is a CSS font-family list.
	Family string
	Size   float64
	// Weight is a CSS weight: 400 normal, 700 bold.
	Weight int
	Italic bool
}

// weight is the style's weight, normal when unset.
func (st Style) weight() float64 {
	if st.Weight == 0 {
		return 400
	}
	return float64(st.Weight)
}

// Run is a stretch of text set in one face.
type Run struct {
	Text  string
	Width float64
	// Family is the registered family the run is set in ("" for none).
	Family string
}

// Width is a single line's advance width in px, rounded up to 1/64 px as a
// browser's layout does.
func (fs *Fonts) Width(s string, st Style) float64 {
	if s == "" {
		return 0
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	k := widthKey{s, st}
	if w, ok := fs.widths[k]; ok {
		return w
	}
	w := 0.0
	fs.runs(s, st, func(_ []rune, rw float64, _ *family) { w += rw })
	w = math.Ceil(w*64-1e-6) / 64
	fs.widths[k] = w
	return w
}

// Runs splits a line into the stretches each face sets, for drawing.
func (fs *Fonts) Runs(s string, st Style) []Run {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	names := map[*family]string{}
	for n, f := range fs.families {
		names[f] = n
	}
	var out []Run
	fs.runs(s, st, func(r []rune, w float64, f *family) {
		out = append(out, Run{Text: string(r), Width: w, Family: names[f]})
	})
	return out
}

// Extents is the ascent and descent, in px, of the style's first available
// family (CSS's primary font), each rounded to whole px as browsers do. Zero
// when no family is registered.
func (fs *Fonts) Extents(st Style) (ascent, descent float64) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fams := fs.resolve(st.Family)
	if len(fams) == 0 {
		return 0, 0
	}
	fc := fs.faceFor(fams[0].pick(st.weight(), st.Italic), st.weight())
	return math.Round(fc.ascent * st.Size), math.Round(fc.descent * st.Size)
}

// runs calls fn for each stretch of s set in one face, with its width. A
// character no family has is given an em (wide characters) or half an em: a
// guess, for when the caller registered no font for it.
func (fs *Fonts) runs(s string, st Style, fn func(r []rune, w float64, f *family)) {
	fams := fs.resolve(st.Family)
	rs := []rune(s)
	wt := st.weight()
	files := make([]*file, len(fams))
	for i, f := range fams {
		files[i] = f.pick(wt, st.Italic)
	}
	which := func(r rune) int {
		for i, f := range files {
			if _, ok := f.font.NominalGlyph(r); ok {
				return i
			}
		}
		return -1
	}
	start, cur := 0, -2
	flush := func(end int) {
		if end <= start {
			return
		}
		if cur < 0 {
			w := 0.0
			for _, r := range rs[start:end] {
				if unicode.Is(unicode.Han, r) || wide(r) {
					w += st.Size
				} else {
					w += st.Size / 2
				}
			}
			fn(rs[start:end], w, nil)
			return
		}
		fc := fs.faceFor(files[cur], wt)
		// Shaped at one unit per font unit, then scaled: exact advances.
		out := fs.shaper.Shape(shaping.Input{
			Text: rs, RunStart: start, RunEnd: end,
			Direction: di.DirectionLTR, Face: fc.face,
			Size:     fixed.I(int(fc.upem)),
			Script:   language.LookupScript(rs[start]),
			Language: language.DefaultLanguage(),
		})
		fn(rs[start:end], float64(out.Advance)/64*st.Size/fc.upem, fams[cur])
	}
	for i, r := range rs {
		w := cur
		// Marks stay with their base; anything else takes the first face that
		// has it, as CSS font matching does character by character.
		if !(cur >= 0 && unicode.Is(unicode.Mn, r)) {
			w = which(r)
		}
		if i == 0 {
			cur = w
			continue
		}
		if w != cur {
			flush(i)
			start, cur = i, w
		}
	}
	flush(len(rs))
}

// wide reports East Asian wide and fullwidth characters, roughly: the CJK
// blocks, Hangul, fullwidth forms.
func wide(r rune) bool {
	return r >= 0x1100 && r <= 0x115F || r >= 0x2E80 && r <= 0xA4CF || r >= 0xAC00 && r <= 0xD7A3 ||
		r >= 0xF900 && r <= 0xFAFF || r >= 0xFE30 && r <= 0xFE4F || r >= 0xFF00 && r <= 0xFF60 ||
		r >= 0xFFE0 && r <= 0xFFE6 || r >= 0x20000 && r <= 0x3FFFD
}

// XHeight is the x-height, in px, of the style's primary font (CSS's ex);
// half an em when no family is registered or the font has none.
func (fs *Fonts) XHeight(st Style) float64 {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fams := fs.resolve(st.Family)
	if len(fams) == 0 {
		return st.Size / 2
	}
	fc := fs.faceFor(fams[0].pick(st.weight(), st.Italic), st.weight())
	if x := float64(fc.face.LineMetric(font.XHeight)); x > 0 {
		return x / fc.upem * st.Size
	}
	return st.Size / 2
}

// Package text measures and wraps text as a browser does: a CSS font-family
// list resolved against the fonts the caller registered, per-character
// fallback, HarfBuzz shaping (go-text), widths rounded up to 1/64 px.
package text

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/f1sherb0y/mermaid-go/internal/typesetting/font"
	ot "github.com/f1sherb0y/mermaid-go/internal/typesetting/font/opentype"
	"github.com/f1sherb0y/mermaid-go/internal/typesetting/font/opentype/tables"
	"github.com/f1sherb0y/mermaid-go/internal/typesetting/shaping"
)

// Fonts is a set of font families, by name, and the families tried after a
// style's own list (as a browser falls back to system fonts). Safe for
// concurrent use once set up.
type Fonts struct {
	families map[string]*family
	fallback []string

	mu     sync.Mutex
	shaper shaping.HarfbuzzShaper
	faces  map[faceKey]*face
	widths map[widthKey]float64
}

type family struct {
	files []*file
}

// file is one font: its weight range (a variable font's wght axis, else one
// weight) and style.
type file struct {
	font       *font.Font
	minW, maxW float64
	italic     bool
	variable   bool
}

// face is a file at a weight, ready to shape.
type face struct {
	face *font.Face
	upem float64
	// ascent and descent, in em.
	ascent, descent float64
}

type faceKey struct {
	f      *file
	weight float64
}

type widthKey struct {
	s  string
	st Style
}

// NewFonts returns an empty set.
func NewFonts() *Fonts {
	return &Fonts{families: map[string]*family{}, faces: map[faceKey]*face{}, widths: map[widthKey]float64{}}
}

// Add registers a font file's faces (a collection gives several) under a
// family name, as CSS @font-face does. Names match case-insensitively.
func (fs *Fonts) Add(name string, data []byte) error {
	r := bytes.NewReader(data)
	lds, err := ot.NewLoaders(r)
	if err != nil {
		return fmt.Errorf("font %s: %w", name, err)
	}
	fam := fs.families[key(name)]
	if fam == nil {
		fam = &family{}
		fs.families[key(name)] = fam
	}
	for _, ld := range lds {
		f, err := font.NewFont(ld)
		if err != nil {
			return fmt.Errorf("font %s: %w", name, err)
		}
		a := f.Describe().Aspect
		fl := &file{font: f, minW: float64(a.Weight), maxW: float64(a.Weight), italic: a.Style == font.StyleItalic}
		if raw, err := ld.RawTable(ot.MustNewTag("fvar")); err == nil {
			if fvar, _, err := tables.ParseFvar(raw); err == nil {
				for _, ax := range fvar.FvarRecords.Axis {
					if ax.Tag == ot.MustNewTag("wght") {
						fl.minW, fl.maxW, fl.variable = float64(ax.Minimum), float64(ax.Maximum), true
					}
				}
			}
		}
		fam.files = append(fam.files, fl)
	}
	return nil
}

// AddFile is Add with a file's content.
func (fs *Fonts) AddFile(name, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return fs.Add(name, data)
}

// Fallback sets the families tried, in order, for characters no family of a
// style's list has.
func (fs *Fonts) Fallback(names ...string) { fs.fallback = names }

func key(name string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(name), `"'`))
}

// families resolves a CSS font-family list, then the fallbacks, to the
// registered families.
func (fs *Fonts) resolve(list string) []*family {
	var out []*family
	for _, n := range append(strings.Split(list, ","), fs.fallback...) {
		if f := fs.families[key(n)]; f != nil {
			out = append(out, f)
		}
	}
	return out
}

// pick chooses a family's file for a weight and style, by CSS font matching:
// style first, then the weight nearest in CSS's order of preference.
func (fam *family) pick(weight float64, italic bool) *file {
	var cands []*file
	for _, f := range fam.files {
		if f.italic == italic {
			cands = append(cands, f)
		}
	}
	if len(cands) == 0 {
		cands = fam.files
	}
	var best *file
	bestD := 0.0
	for _, f := range cands {
		if d := weightDistance(weight, f.minW, f.maxW); best == nil || d < bestD {
			best, bestD = f, d
		}
	}
	return best
}

// weightDistance orders a file's weights for a desired one as CSS Fonts 4
// does: below 400 lighter ones first, above 500 heavier ones first, and in
// between up to 500 first.
func weightDistance(w, lo, hi float64) float64 {
	if w >= lo && w <= hi {
		return 0
	}
	near := lo
	if w > hi {
		near = hi
	}
	d := near - w
	switch {
	case w < 400:
		if d < 0 {
			return -d
		}
		return 1000 + d
	case w > 500:
		if d > 0 {
			return d
		}
		return 1000 - d
	default:
		if d > 0 && near <= 500 {
			return d
		}
		if d < 0 {
			return 500 - d
		}
		return 1000 + d
	}
}

// faceFor is a file at a weight, made once.
func (fs *Fonts) faceFor(f *file, weight float64) *face {
	if !f.variable {
		weight = f.minW
	}
	weight = max(f.minW, min(f.maxW, weight))
	k := faceKey{f, weight}
	if fc := fs.faces[k]; fc != nil {
		return fc
	}
	ff := font.NewFace(f.font)
	if f.variable {
		ff.SetVariations([]font.Variation{{Tag: ot.MustNewTag("wght"), Value: float32(weight)}})
	}
	upem := float64(f.font.Upem())
	fc := &face{face: ff, upem: upem}
	if ext, ok := ff.FontHExtents(); ok {
		fc.ascent, fc.descent = float64(ext.Ascender)/upem, -float64(ext.Descender)/upem
	}
	fs.faces[k] = fc
	return fc
}

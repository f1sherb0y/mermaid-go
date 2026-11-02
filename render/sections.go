package render

import (
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/color"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/style"
)

// The section colors of mindmap and kanban (their styles.ts): a node,
// cluster or edge with the class section-N (section-edge-N) takes the
// theme's cScale(N+1) and its label cScaleLabel(N+1); mindmap's root
// (section-root) the git0 colors.

// sections are a theme's section colors, by cScale index.
type sections struct {
	fill, text, line []scene.Color
	rootFill         scene.Color
	rootText         scene.Color
}

// newSections reads the section colors; kanban's fills are lightened by
// 10 (darkened in dark themes).
func (r *renderer) newSections(kind string) *sections {
	n := int(style.Px(r.th.Get("THEME_COLOR_LIMIT"), 12))
	s := &sections{rootFill: r.color("git0"), rootText: r.color("gitBranchLabel0")}
	for i := range n {
		c := r.th.Get("cScale" + strconv.Itoa(i))
		if kind == "kanban" {
			adjust := color.Lighten
			if r.th.Get("darkMode") == "true" {
				adjust = color.Darken
			}
			if v, err := adjust(c, 10); err == nil {
				c = v
			}
		}
		s.fill = append(s.fill, style.Color(c))
		s.text = append(s.text, r.color("cScaleLabel"+strconv.Itoa(i)))
		s.line = append(s.line, r.color("cScaleInv"+strconv.Itoa(i)))
	}
	return s
}

// sectionIndex is the cScale index of the class prefix+N in classes
// (prefix "section-" or "section-edge-"): N+1, cycling through the theme's
// colors (mermaid's rules stop at THEME_COLOR_LIMIT and leave the rest
// unpainted).
func (s *sections) index(classes, prefix string) (int, bool) {
	for _, c := range strings.Fields(classes) {
		if v, ok := strings.CutPrefix(c, prefix); ok {
			if n, err := strconv.Atoi(v); err == nil && len(s.fill) > 0 {
				return (n + 1) % len(s.fill), true
			}
		}
	}
	return 0, false
}

func hasClass(classes, c string) bool {
	for _, f := range strings.Fields(classes) {
		if f == c {
			return true
		}
	}
	return false
}

// sectionPaint applies the section rules to a node's part.
func (r *renderer) sectionPaint(n *nstate, k part, p paint) paint {
	s := r.sheet.sections
	if s == nil {
		return p
	}
	i, ok := s.index(n.CSSClasses, "section-")
	if !ok {
		return p
	}
	switch r.sheet.kind {
	case "mindmap":
		fill := s.fill[i]
		if hasClass(n.CSSClasses, "section-root") {
			fill = s.rootFill
		}
		if k == pLine {
			// .section-N line
			return paint{Fill: scene.None, Stroke: s.line[i], Width: 3}
		}
		p.Fill, p.Stroke = fill, scene.None
		if n.look == "neo" {
			p.Stroke, p.Width = fill, style.Px(r.th.Get("strokeWidth"), 2)
		}
	case "kanban":
		p.Fill, p.Stroke = s.fill[i], s.fill[i]
	}
	return p
}

// sectionText is a mindmap node's label color.
func (r *renderer) sectionText(n *nstate, c scene.Color) scene.Color {
	s := r.sheet.sections
	if s == nil || r.sheet.kind != "mindmap" {
		return c
	}
	if hasClass(n.CSSClasses, "section-root") {
		return s.rootText
	}
	if i, ok := s.index(n.CSSClasses, "section-"); ok {
		return s.text[i]
	}
	return c
}

// edgeWidthFor is the stroke width the edge's classes give it: mindmap's
// edge-depth-D rules (17 - 3(D+1), the .edge rule's 3 where that is not a
// valid width).
func (s *sheet) edgeWidthFor(e *estate) float64 {
	if s.kind != "mindmap" {
		return s.edgeWidth
	}
	for _, c := range strings.Fields(e.Classes) {
		if v, ok := strings.CutPrefix(c, "edge-depth-"); ok {
			if d, err := strconv.Atoi(v); err == nil && d < 11 && 14-3*d > 0 {
				return float64(14 - 3*d)
			}
		}
	}
	return 3
}

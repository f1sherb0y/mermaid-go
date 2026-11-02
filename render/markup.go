package render

import (
	"html"
	"regexp"
	"strings"
)

// A label's markup, parsed: mermaid labels are HTML (nonMarkdownToHTML,
// markdownToHTML in handle-markdown-text.ts, then a browser parses them),
// so a label is lines of styled runs.

// run is a stretch of text in one style.
type run struct {
	Text      string
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
}

// mline is one line of a label as written (before wrapping).
type mline []run

var (
	// mermaid's placeholders for entities written #name; in the source.
	entityPlaceholders = strings.NewReplacer("ﬂ°°", "&#", "ﬂ°", "&", "¶ß", ";")
	faIcon             = regexp.MustCompile(`(fa[bklrs]?):fa-[\w-]+`)
	tagRe              = regexp.MustCompile(`^<(/?)([a-zA-Z][a-zA-Z0-9]*)([^>]*?)(/?)>`)
	space              = regexp.MustCompile(`[ \t\n\r\f]+`)
)

// decodeEntities turns mermaid's entity placeholders back into entities.
func decodeEntities(s string) string { return entityPlaceholders.Replace(s) }

// labelHTML is the HTML mermaid makes of a label (createText with
// useHtmlLabels): markdown through marked, anything else wrapped in a <p>
// with newlines as <br>.
func labelHTML(label string, markdown bool) string {
	label = decodeEntities(label)
	var h string
	if markdown {
		h = markdownToHTML(label)
	} else if label != "" {
		h = "<p>" + strings.NewReplacer(`\n`, "<br />", "\n", "<br />").Replace(label) + "</p>"
	}
	// Font Awesome icons need the icon font, which is not there: they come
	// out empty.
	return faIcon.ReplaceAllString(decodeEntities(h), "")
}

// parseHTML reads a label's HTML as a browser lays it out: <br> and block
// elements break lines, b/strong, i/em, u, s/del style the text, other tags
// are transparent, entities are decoded, and white space collapses.
func parseHTML(h string) []mline {
	var lines []mline
	var cur mline
	curOpen := false // cur holds text or was opened by a <br>
	lastBR := false
	var bold, italic, under, strike int
	push := func(text string) {
		text = html.UnescapeString(space.ReplaceAllString(text, " "))
		if text == "" {
			return
		}
		r := run{Text: text, Bold: bold > 0, Italic: italic > 0, Underline: under > 0, Strike: strike > 0}
		if n := len(cur); n > 0 && cur[n-1].Bold == r.Bold && cur[n-1].Italic == r.Italic &&
			cur[n-1].Underline == r.Underline && cur[n-1].Strike == r.Strike {
			cur[n-1].Text += r.Text
		} else {
			cur = append(cur, r)
		}
		curOpen, lastBR = true, false
	}
	// block ends the current line at a block boundary: an empty line counts
	// only when a <br> made it, and a <br> at the end of a block adds none.
	block := func() {
		if len(cur) > 0 && !onlySpace(cur) {
			lines = append(lines, cur)
		} else if curOpen && !lastBR && len(cur) > 0 {
			lines = append(lines, cur)
		}
		cur, curOpen, lastBR = nil, false, false
	}
	for h != "" {
		i := strings.IndexByte(h, '<')
		if i < 0 {
			push(h)
			break
		}
		if i > 0 {
			push(h[:i])
			h = h[i:]
		}
		m := tagRe.FindStringSubmatch(h)
		if m == nil {
			// A lone '<' is text.
			push("<")
			h = h[1:]
			continue
		}
		h = h[len(m[0]):]
		closing := m[1] == "/"
		d := 1
		if closing {
			d = -1
		}
		switch strings.ToLower(m[2]) {
		case "br":
			lines = append(lines, cur)
			cur, curOpen, lastBR = nil, true, true
		case "p", "div", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "blockquote", "table", "tr":
			block()
		case "b", "strong", "th":
			bold = max(0, bold+d)
		case "i", "em", "cite", "dfn", "var":
			italic = max(0, italic+d)
		case "u", "ins":
			under = max(0, under+d)
		case "s", "del", "strike":
			strike = max(0, strike+d)
		case "script", "style":
			// Dropped with their content (DOMPurify).
			if !closing {
				if j := strings.Index(strings.ToLower(h), "</"+strings.ToLower(m[2])); j >= 0 {
					h = h[j:]
				}
			}
		}
	}
	block()
	return lines
}

func onlySpace(l mline) bool {
	for _, r := range l {
		if strings.TrimSpace(r.Text) != "" {
			return false
		}
	}
	return true
}

// markdownToHTML is marked's lexer and mermaid's output() for what labels
// use: paragraphs, **strong**, *em*, escapes; other blocks stay as written.
func markdownToHTML(md string) string {
	var b strings.Builder
	for _, para := range splitParagraphs(md) {
		b.WriteString("<p>")
		b.WriteString(regexp.MustCompile(`\n *`).ReplaceAllString(inlineMarkdown(para), "<br/>"))
		b.WriteString("</p>")
	}
	return b.String()
}

// splitParagraphs splits at blank lines; lines are trimmed at the end as
// marked does.
func splitParagraphs(md string) []string {
	var out []string
	var cur []string
	for _, l := range strings.Split(md, "\n") {
		if strings.TrimSpace(l) == "" {
			if len(cur) > 0 {
				out = append(out, strings.Join(cur, "\n"))
				cur = nil
			}
			continue
		}
		if len(cur) == 0 {
			l = strings.TrimLeft(l, " ")
		}
		cur = append(cur, strings.TrimRight(l, " "))
	}
	if len(cur) > 0 {
		out = append(out, strings.Join(cur, "\n"))
	}
	return out
}

// inlineMarkdown turns emphasis into tags: ** and __ strong, * and _ em,
// with marked's flanking rules simplified (a delimiter run opens before a
// non-space and closes after one; _ does not work inside words).
func inlineMarkdown(s string) string {
	rs := []rune(s)
	var b strings.Builder
	type open struct {
		delim string
		pos   int // index into out where the tag goes
	}
	var out []string
	var stack []open
	isSpace := func(i int) bool { return i < 0 || i >= len(rs) || rs[i] == ' ' || rs[i] == '\n' }
	isWord := func(i int) bool {
		if i < 0 || i >= len(rs) {
			return false
		}
		r := rs[i]
		return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 127
	}
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '\\' && i+1 < len(rs) && strings.ContainsRune("\\`*_{}[]()#+-.!|<>~", rs[i+1]) {
			out = append(out, html.EscapeString(string(rs[i+1])))
			i++
			continue
		}
		if r == '*' || r == '_' {
			n := 1
			for i+n < len(rs) && rs[i+n] == r && n < 2 {
				n++
			}
			d := strings.Repeat(string(r), n)
			canOpen := !isSpace(i + n)
			canClose := !isSpace(i - 1)
			if r == '_' {
				canOpen = canOpen && !isWord(i-1)
				canClose = canClose && !isWord(i+n)
			}
			// Close the innermost matching opener.
			if canClose {
				k := len(stack) - 1
				for k >= 0 && stack[k].delim != d {
					k--
				}
				if k >= 0 {
					tag := "em"
					if n == 2 {
						tag = "strong"
					}
					o := stack[k]
					stack = stack[:k]
					out[o.pos] = "<" + tag + ">"
					out = append(out, "</"+tag+">")
					i += n - 1
					continue
				}
			}
			if canOpen {
				stack = append(stack, open{d, len(out)})
				out = append(out, d)
				i += n - 1
				continue
			}
			out = append(out, d)
			i += n - 1
			continue
		}
		out = append(out, string(r))
	}
	for _, s := range out {
		b.WriteString(s)
	}
	return b.String()
}

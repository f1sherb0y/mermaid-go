// Package common holds what mermaid's flowchart, class, state, ER and
// requirement dbs share (diagrams/common/common.ts, utils.ts): entity
// placeholders, label sanitizing, edge ids, and setting a node's
// type-specific data.
package common

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/f1sherb0y/mermaid-go/render"
)

var (
	styleSemi    = regexp.MustCompile(`style.*:\S*#.*;`)
	classDefSemi = regexp.MustCompile(`classDef.*:\S*#.*;`)
	entity       = regexp.MustCompile(`#\w+;`)
)

// EncodeEntities is what mermaid does to a diagram's code before parsing
// (utils.encodeEntities, plus the newline Diagram.fromText appends): the
// last ';' of a style or classDef line with a color goes, and #name; and
// #123; entities become placeholders the renderer turns back into HTML
// entities.
func EncodeEntities(code string) string {
	dropLast := func(s string) string { return s[:len(s)-1] }
	code = styleSemi.ReplaceAllStringFunc(code, dropLast)
	code = classDefSemi.ReplaceAllStringFunc(code, dropLast)
	code = entity.ReplaceAllStringFunc(code, func(s string) string {
		inner := s[1 : len(s)-1]
		if _, err := strconv.Atoi(strings.TrimPrefix(inner, "+")); err == nil && !strings.HasPrefix(inner, "-") {
			return "ﬂ°°" + inner + "¶ß"
		}
		return "ﬂ°" + inner + "¶ß"
	})
	return code + "\n"
}

// EdgeID is utils.getEdgeId: the given id, else prefix_from_to_counter.
func EdgeID(from, to string, counter int, prefix string) string {
	if prefix != "" {
		prefix += "_"
	}
	return prefix + from + "_" + to + "_" + strconv.Itoa(counter)
}

// SetExtra stores type-specific data on a node under mermaid's name, as
// render.LoadData would have read it.
func SetExtra(n *render.Node, key string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	if n.Extra == nil {
		n.Extra = map[string]json.RawMessage{}
	}
	n.Extra[key] = b
}

// SetEdgeExtra stores type-specific data on an edge.
func SetEdgeExtra(e *render.Edge, key string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	if e.Extra == nil {
		e.Extra = map[string]json.RawMessage{}
	}
	e.Extra[key] = b
}

// StylesFromArray is utils.getStylesFromArray: color and text-align go to
// the label, the rest to the shape, each ending in ';'.
func StylesFromArray(arr []string) (style, labelStyle string) {
	for _, s := range arr {
		if strings.HasPrefix(s, "color:") || strings.HasPrefix(s, "text-align:") {
			labelStyle += s + ";"
		} else {
			style += s + ";"
		}
	}
	return style, labelStyle
}

// SanitizeText is common.sanitizeText with mermaid's default config
// (strict security level, HTML labels): DOMPurify parses the label as HTML
// and writes it back. So stray '<', '>' and '&' come out escaped, entities
// are decoded (and the three re-escaped), script and style elements go with
// their content, unknown elements go but keep their content, and void
// elements are written <br> style. A '<' starting a tag that never closes
// drops the rest, as a browser does. Text without a '<' is returned as it
// is (DOMPurify's shortcut).
func SanitizeText(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	var b strings.Builder
	var open []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '<')
		if i < 0 {
			writeText(&b, s)
			break
		}
		writeText(&b, s[:i])
		s = s[i:]
		switch {
		case strings.HasPrefix(s, "<!--"):
			end := strings.Index(s[4:], "-->")
			if end < 0 {
				return closeAll(&b, open)
			}
			s = s[4+end+3:]
		case len(s) > 1 && isLetter(s[1]), len(s) > 2 && s[1] == '/' && isLetter(s[2]):
			t, n, ok := readTag(s)
			if !ok {
				return closeAll(&b, open)
			}
			s = s[n:]
			if dropContent[t.name] {
				if !t.end {
					s = skipRawText(s, t.name)
				}
				continue
			}
			if t.end && t.name == "br" {
				t.end = false
			}
			if !allowedTags[t.name] {
				continue
			}
			if t.end {
				for j := len(open) - 1; j >= 0; j-- {
					if open[j] == t.name {
						for k := len(open) - 1; k >= j; k-- {
							b.WriteString("</" + open[k] + ">")
						}
						open = open[:j]
						break
					}
				}
				continue
			}
			b.WriteString("<" + t.name)
			for _, a := range t.attrs {
				if strings.HasPrefix(a[0], "on") || (a[0] == "href" || a[0] == "src") && unsafeURL(a[1]) {
					continue
				}
				b.WriteString(" " + a[0] + `="` + escapeAttr(a[1]) + `"`)
			}
			b.WriteString(">")
			if !voidTags[t.name] {
				open = append(open, t.name)
			}
		default:
			b.WriteString("&lt;")
			s = s[1:]
		}
	}
	return closeAll(&b, open)
}

func closeAll(b *strings.Builder, open []string) string {
	for k := len(open) - 1; k >= 0; k-- {
		b.WriteString("</" + open[k] + ">")
	}
	return b.String()
}

type tag struct {
	name  string
	end   bool
	attrs [][2]string
}

// readTag reads a start or end tag at the start of s: the tag, its length,
// and false when it never closes.
func readTag(s string) (tag, int, bool) {
	var t tag
	i := 1
	if s[i] == '/' {
		t.end = true
		i++
	}
	j := i
	for j < len(s) && !isSpace(s[j]) && s[j] != '/' && s[j] != '>' {
		j++
	}
	t.name = strings.ToLower(s[i:j])
	i = j
	for i < len(s) {
		c := s[i]
		switch {
		case c == '>':
			return t, i + 1, true
		case isSpace(c) || c == '/':
			i++
			continue
		}
		j := i
		for j < len(s) && !isSpace(s[j]) && s[j] != '/' && s[j] != '>' && s[j] != '=' {
			j++
		}
		if j == i {
			j++ // a lone '=' starts a name
		}
		name := strings.ToLower(s[i:j])
		i = j
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		val := ""
		if i < len(s) && s[i] == '=' {
			i++
			for i < len(s) && isSpace(s[i]) {
				i++
			}
			if i < len(s) && (s[i] == '"' || s[i] == '\'') {
				q := s[i]
				end := strings.IndexByte(s[i+1:], q)
				if end < 0 {
					return t, 0, false
				}
				val = s[i+1 : i+1+end]
				i += end + 2
			} else {
				j := i
				for j < len(s) && !isSpace(s[j]) && s[j] != '>' {
					j++
				}
				val, i = s[i:j], j
			}
		}
		if !t.end {
			t.attrs = append(t.attrs, [2]string{name, html.UnescapeString(val)})
		}
	}
	return t, 0, false
}

// skipRawText skips a script or style element's content and its end tag.
func skipRawText(s, name string) string {
	lower := strings.ToLower(s)
	i := strings.Index(lower, "</"+name)
	if i < 0 {
		return ""
	}
	if end := strings.IndexByte(s[i:], '>'); end >= 0 {
		return s[i+end+1:]
	}
	return ""
}

func writeText(b *strings.Builder, s string) {
	if s == "" {
		return
	}
	b.WriteString(textEscaper.Replace(html.UnescapeString(s)))
}

var (
	textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\u00a0", "&nbsp;")
	attrEscaper = strings.NewReplacer("&", "&amp;", `"`, "&quot;", "\u00a0", "&nbsp;")
)

func escapeAttr(s string) string { return attrEscaper.Replace(s) }

func unsafeURL(u string) bool {
	u = strings.ToLower(strings.Map(func(r rune) rune {
		if r <= ' ' {
			return -1
		}
		return r
	}, u))
	return strings.HasPrefix(u, "javascript:") || strings.HasPrefix(u, "vbscript:")
}

func isLetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }
func isSpace(c byte) bool  { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

func set(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

var (
	// allowedTags is DOMPurify's default HTML and SVG allow list.
	allowedTags = set(`a abbr acronym address area article aside audio b bdi bdo big blink blockquote
		body br button canvas caption center cite code col colgroup content data datalist dd decorator
		del details dfn dialog dir div dl dt element em fieldset figcaption figure font footer form h1
		h2 h3 h4 h5 h6 head header hgroup hr html i img input ins kbd label legend li main map mark
		marquee menu menuitem meter nav nobr ol optgroup option output p picture pre progress q rp rt
		ruby s samp search section select shadow slot small source spacer span strike strong sub summary
		sup table tbody td template textarea tfoot th thead time tr track tt u ul var video wbr
		svg circle clippath defs desc ellipse g line lineargradient marker mask path pattern polygon
		polyline radialgradient rect stop switch symbol text textpath title tspan use`)
	// dropContent are removed with what is inside them.
	dropContent = set(`script style iframe noscript noembed noframes xmp plaintext object embed math`)
	voidTags    = set(`area base br col embed hr img input link meta source track wbr`)
)

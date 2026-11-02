package mindmap

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The outline syntax mindmap and kanban share (mindmap.jison, kanban.jison):
// one node per line, nested by indentation, written as an id, a shape
// around a description, or both; "::icon(...)" and ":::classes" lines
// decorate the node before them. Kanban nodes may carry "@{...}" data.

// NodeType is a node's shape as its brackets give it (mindmap's nodeType).
type NodeType int

const (
	Default     NodeType = iota // a bare id
	RoundedRect                 // (text)
	Rect                        // [text]
	Circle                      // ((text))
	Cloud                       // )text(
	Bang                        // ))text((
	Hexagon                     // {{text}}
)

// nodeType is getType: the shape a pair of brackets makes.
func nodeType(open, close string) NodeType {
	switch open {
	case "[":
		return Rect
	case "(":
		if close == ")" {
			return RoundedRect
		}
		return Cloud
	case "((":
		return Circle
	case ")":
		return Cloud
	case "))":
		return Bang
	case "{{":
		return Hexagon
	}
	return Default
}

// Item is one node of an outline.
type Item struct {
	// Indent is the node's indentation (whitespace characters).
	Indent int
	// ID and Descr are its id and text (the id when it has no shape).
	ID, Descr string
	Type      NodeType
	// ShapeData is a kanban node's "@{...}" content, YAML without braces.
	ShapeData string
	Icon      string
	Class     string
	Line      int
}

type tokKind int

const (
	tEOF       tokKind = iota
	tKeyword           // mindmap, kanban
	tSpaceLine         // a line of only whitespace, or a comment
	tNL
	tSpaceList // indentation
	tNodeID
	tStart // a shape's opening bracket
	tDescr
	tEnd // a shape's closing bracket
	tIcon
	tClass
	tShapeData
)

type token struct {
	kind tokKind
	text string
	line int
}

func (t token) String() string {
	switch t.kind {
	case tEOF:
		return "end of input"
	case tNL, tSpaceLine:
		return "newline"
	}
	return fmt.Sprintf("%q", t.text)
}

// isSpace is JavaScript's \s.
func isSpace(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' }

// outlineLexer is the jison lexer: rules tried in order, the first match
// wins; the start conditions (NODE, NSTR, ICON, CLASS, shapeData) are code
// paths.
type outlineLexer struct {
	src     string
	pos     int
	line    int
	keyword string
	kanban  bool
	toks    []token
}

func (lx *outlineLexer) emit(k tokKind, text string) {
	lx.toks = append(lx.toks, token{k, text, lx.line})
}

// take consumes n bytes, counting lines.
func (lx *outlineLexer) take(n int) string {
	s := lx.src[lx.pos : lx.pos+n]
	lx.line += strings.Count(s, "\n")
	lx.pos += n
	return s
}

// spaceRun is the length of the whitespace at the position.
func (lx *outlineLexer) spaceRun() int {
	n := 0
	for {
		r, size := utf8.DecodeRuneInString(lx.src[lx.pos+n:])
		if size == 0 || !isSpace(r) {
			return n
		}
		n += size
	}
}

// lenUntil is how far the input runs before any of the bytes in stop (or
// a newline, with stopNL).
func (lx *outlineLexer) lenUntil(stop string) int {
	i := strings.IndexAny(lx.src[lx.pos:], stop)
	if i < 0 {
		return len(lx.src) - lx.pos
	}
	return i
}

func (lx *outlineLexer) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: "+format, append([]any{lx.line}, args...)...)
}

var (
	nodeStarts = []string{"-)", "(-", "))", ")", "((", "{{", "(", "["}
	nodeEnds   = []string{"))", ")", "]", "}}", "(-", "-)", "((", "("}
)

func lexOutline(src, keyword string, kanban bool) ([]token, error) {
	lx := &outlineLexer{src: src, line: 1, keyword: keyword, kanban: kanban}
	for lx.pos < len(lx.src) {
		if err := lx.next(); err != nil {
			return nil, err
		}
	}
	lx.emit(tEOF, "")
	return lx.toks, nil
}

// next lexes one token in the initial state.
func (lx *outlineLexer) next() error {
	rest := lx.src[lx.pos:]
	sp := lx.spaceRun()
	idChars := "([\n){}"
	if lx.kanban {
		idChars += "@"
	}
	switch {
	case lx.kanban && strings.HasPrefix(rest, "@{"):
		lx.take(2)
		return lx.shapeData()
	case strings.HasPrefix(rest[sp:], "%%"):
		// \s*%%.* : a comment, with the whitespace before it.
		lx.take(sp)
		lx.emit(tSpaceLine, lx.take(lx.lenUntil("\n")))
	case len(rest) >= len(lx.keyword) && strings.EqualFold(rest[:len(lx.keyword)], lx.keyword) && !isWordAt(rest, len(lx.keyword)):
		lx.emit(tKeyword, lx.take(len(lx.keyword)))
	case strings.HasPrefix(rest, ":::"):
		lx.take(3)
		if n := lx.lenUntil("\n"); n > 0 {
			lx.emit(tClass, lx.take(n))
		}
	case strings.HasPrefix(rest, "::icon("):
		lx.take(7)
		if n := lx.lenUntil(")"); n > 0 {
			lx.emit(tIcon, lx.take(n))
		}
		if lx.pos < len(lx.src) {
			lx.take(1)
		}
	case sp > 0 && strings.LastIndexByte(rest[:sp], '\n') > 0:
		// [\s]+[\n]: whitespace up to its last newline (blank lines).
		lx.emit(tSpaceLine, lx.take(strings.LastIndexByte(rest[:sp], '\n')+1))
	case rest[0] == '\n':
		lx.emit(tNL, lx.take(len(rest)-len(strings.TrimLeft(rest, "\n"))))
	case hasAnyPrefix(rest, nodeStarts) != "":
		lx.emit(tStart, lx.take(len(hasAnyPrefix(rest, nodeStarts))))
		return lx.node()
	case sp > 0:
		lx.emit(tSpaceList, lx.take(sp))
	default:
		n := lx.lenUntil(idChars)
		if n == 0 {
			return lx.errorf("unexpected %q", rest[:1])
		}
		lx.emit(tNodeID, lx.take(n))
	}
	return nil
}

// node lexes a shape's description and closing bracket (state NODE).
func (lx *outlineLexer) node() error {
	for lx.pos < len(lx.src) {
		rest := lx.src[lx.pos:]
		switch {
		case strings.HasPrefix(rest, "\"`"):
			// A markdown string: "`...`".
			lx.take(2)
			if n := lx.lenUntil("`\""); n > 0 {
				lx.emit(tDescr, lx.take(n))
			}
			if !strings.HasPrefix(lx.src[lx.pos:], "`\"") {
				return lx.errorf("unterminated markdown string")
			}
			lx.take(2)
		case rest[0] == '"':
			lx.take(1)
			if n := lx.lenUntil(`"`); n > 0 {
				lx.emit(tDescr, lx.take(n))
			}
			if lx.pos == len(lx.src) {
				return lx.errorf("unterminated string")
			}
			lx.take(1)
		case hasAnyPrefix(rest, nodeEnds) != "":
			lx.emit(tEnd, lx.take(len(hasAnyPrefix(rest, nodeEnds))))
			return nil
		default:
			n := lx.lenUntil(")](}")
			if n == 0 {
				// .+ : a lone "}" takes the rest of the line.
				n = lx.lenUntil("\n")
			}
			lx.emit(tDescr, lx.take(n))
		}
	}
	return nil
}

// shapeData lexes kanban's "@{...}": its text, quotes kept, newlines in
// quoted strings turned into <br/>.
func (lx *outlineLexer) shapeData() error {
	var b strings.Builder
	for lx.pos < len(lx.src) {
		switch c := lx.src[lx.pos]; c {
		case '}':
			lx.take(1)
			lx.emit(tShapeData, b.String())
			return nil
		case '"':
			b.WriteString(lx.take(1))
			n := lx.lenUntil(`"`)
			b.WriteString(brNewlines(lx.take(n)))
			if lx.pos < len(lx.src) {
				b.WriteString(lx.take(1))
			}
		default:
			b.WriteString(lx.take(lx.lenUntil(`}"`)))
		}
	}
	return lx.errorf("unterminated @{")
}

// brNewlines is /\n\s*/g replaced by <br/>.
func brNewlines(s string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return b.String() + s
		}
		b.WriteString(s[:i] + "<br/>")
		s = strings.TrimLeftFunc(s[i+1:], isSpace)
	}
}

func hasAnyPrefix(s string, prefixes []string) string {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return p
		}
	}
	return ""
}

// isWordAt reports a word character at s[i] (jison's \b after a keyword).
func isWordAt(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	c := s[i]
	return c == '_' || c >= '0' && c <= '9' || c|0x20 >= 'a' && c|0x20 <= 'z'
}

// ParseOutline parses an outline diagram whose header is keyword: its
// nodes in order (with kanban, their @{...} data too).
func ParseOutline(code, keyword string, kanban bool) ([]*Item, error) {
	toks, err := lexOutline(code, keyword, kanban)
	if err != nil {
		return nil, err
	}
	p := &outlineParser{toks: toks}
	return p.parse()
}

type outlineParser struct {
	toks  []token
	i     int
	items []*Item
}

func (p *outlineParser) peek() token { return p.toks[p.i] }

func (p *outlineParser) next() token {
	t := p.toks[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

func (p *outlineParser) errorf(t token, format string, args ...any) error {
	return fmt.Errorf("line %d: "+format, append([]any{t.line}, args...)...)
}

func isStop(k tokKind) bool { return k == tNL || k == tSpaceLine || k == tEOF }

// parse is start: blank lines, the keyword, then statements, each ended by
// a newline, a blank line or the end.
func (p *outlineParser) parse() ([]*Item, error) {
	for k := p.peek().kind; k == tSpaceLine || k == tNL; k = p.peek().kind {
		p.next()
	}
	if t := p.next(); t.kind != tKeyword {
		return nil, p.errorf(t, "expected the diagram keyword, got %v", t)
	}
	if p.peek().kind == tNL {
		p.next()
	}
	statements := 0
	for {
		if p.peek().kind == tEOF && statements > 0 {
			return p.items, nil
		}
		if err := p.statement(); err != nil {
			return nil, err
		}
		statements++
		t := p.next()
		if !isStop(t.kind) {
			return nil, p.errorf(t, "unexpected %v", t)
		}
		for isStop(p.peek().kind) && p.peek().kind != tEOF {
			p.next()
		}
	}
}

// statement is a node, an icon or classes, any of them indented, or
// nothing (a blank line, an indented blank).
func (p *outlineParser) statement() error {
	indent := 0
	if p.peek().kind == tSpaceList {
		indent = len(p.next().text)
	}
	switch t := p.peek(); t.kind {
	case tSpaceLine:
		// A blank line: it ends the (empty) statement too.
		return nil
	case tIcon, tClass:
		p.next()
		if len(p.items) == 0 {
			return p.errorf(t, "%v decorates no node", t)
		}
		last := p.items[len(p.items)-1]
		if t.kind == tIcon {
			last.Icon = t.text
		} else {
			last.Class = t.text
		}
		return nil
	case tNodeID, tStart:
		return p.node(indent)
	case tNL, tEOF:
		if indent > 0 {
			return nil
		}
	}
	t := p.peek()
	return p.errorf(t, "unexpected %v", t)
}

// node is an id, a shape, or an id and a shape, then kanban's data.
func (p *outlineParser) node(indent int) error {
	it := &Item{Indent: indent, Line: p.peek().line}
	if p.peek().kind == tNodeID {
		it.ID = p.next().text
		it.Descr = it.ID
	}
	if p.peek().kind == tStart {
		open := p.next()
		d := p.next()
		if d.kind != tDescr {
			return p.errorf(d, "expected a description, got %v", d)
		}
		end := p.next()
		if end.kind != tEnd {
			return p.errorf(end, "expected the end of the shape, got %v", end)
		}
		it.Descr, it.Type = d.text, nodeType(open.text, end.text)
		if it.ID == "" {
			it.ID = d.text
		}
	}
	if p.peek().kind == tShapeData {
		it.ShapeData = p.next().text
	}
	p.items = append(p.items, it)
	return nil
}

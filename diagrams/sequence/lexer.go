package sequence

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The lexer follows sequenceDiagram.jison's: rules are tried in the file's
// order and the first that matches wins; keywords ignore case and, ending
// in a word character, match only whole words; the start conditions (ID
// after participant/actor/activate/..., ALIAS before "as", LINE for the
// rest of a line, CONFIG inside @{...}) are a stack whose bottom never pops.

type kind uint8

const (
	tEOF kind = iota
	tNewline
	tInvalid
	tNum
	tActor
	tText       // ":..." message text
	tRest       // the rest of a line (LINE)
	tArrow      // a signal type; tok.arrow says which
	tPlus       // +
	tMinus      // -
	tCircle     // ()
	tComma      // ,
	tAs         // as (ALIAS)
	tConfig     // @{...}'s content
	tTitle      // title ..., title: ...
	tAcc        // accTitle/accDescr and their values, dropped
	tKeyword    // tok.text is the keyword, lower case
	tLeftOf     // left of
	tRightOf    // right of
	tConfigOpen // @{
)

type token struct {
	kind  kind
	text  string
	arrow Kind
	line  int
}

// keywords in jison's order; the true ones begin the LINE condition, the
// ID ones begin ID.
var keywords = []struct {
	word  string
	state string
}{
	{"box", "LINE"}, {"participant", "ID"}, {"actor", "ID"}, {"create", ""}, {"destroy", "ID"},
	{"loop", "LINE"}, {"rect", "LINE"}, {"opt", "LINE"}, {"alt", "LINE"}, {"else", "LINE"},
	{"par", "LINE"}, {"par_over", "LINE"}, {"and", "LINE"}, {"critical", "LINE"},
	{"option", "LINE"}, {"break", "LINE"}, {"end", ""},
}

// arrows in jison's order: the first that is a prefix wins.
var arrows = []struct {
	text string
	kind Kind
}{
	{"->>", Solid}, {"<<->>", BidirectionalSolid}, {"-->>", Dotted}, {"<<-->>", BidirectionalDotted},
	{"->", SolidOpen}, {"-->", DottedOpen}, {"-x", SolidCross}, {"--x", DottedCross},
	{"-)", SolidPoint}, {"--)", DottedPoint},
	{`--|\`, SolidTopDotted}, {`--|/`, SolidBottomDotted}, {`--\\`, StickTopDotted}, {`--//`, StickBottomDotted},
	{`/|--`, SolidTopReverseDotted}, {`\|--`, SolidBottomReverseDotted}, {`//--`, StickTopReverseDotted}, {`\\--`, StickBottomReverseDotted},
	{`-|\`, SolidTop}, {`-|/`, SolidBottom}, {`-\\`, StickTop}, {`-//`, StickBottom},
	{`/|-`, SolidTopReverse}, {`\|-`, SolidBottomReverse}, {`//-`, StickTopReverse}, {`\\-`, StickBottomReverse},
}

// notArrow is what may not follow a hyphen inside an actor's name: the
// starts of arrows (the ACTOR rule's negative lookahead).
var notArrow = []string{`-x`, `--x`, `-)`, `--)`, `-|\`, `-\`, `-/`, `-//`, `-|/`, `/|-`, `\|-`, `//-`, `\\-`, `--|\`, `--`, `()`}

type lexer struct {
	src   string
	pos   int
	line  int
	stack []string
	out   []token
}

func lex(src string) ([]token, error) {
	l := &lexer{src: src, line: 1, stack: []string{"INITIAL"}}
	for {
		done := l.pos >= len(src)
		if err := l.next(); err != nil {
			return nil, err
		}
		if done {
			break
		}
	}
	return append(l.out, token{kind: tEOF, line: l.line}), nil
}

func (l *lexer) state() string { return l.stack[len(l.stack)-1] }
func (l *lexer) push(s string) { l.stack = append(l.stack, s) }
func (l *lexer) pop() {
	if len(l.stack) > 1 {
		l.stack = l.stack[:len(l.stack)-1]
	}
}

// emit adds a token for the next n bytes and moves past them.
func (l *lexer) emit(k kind, n int, text string) {
	l.out = append(l.out, token{kind: k, text: text, line: l.line})
	l.skip(n)
}

func (l *lexer) skip(n int) {
	l.line += strings.Count(l.src[l.pos:l.pos+n], "\n")
	l.pos += n
}

// next reads one token (or skips one stretch); at the end of the input
// it gives jison's <<EOF>> token, a newline.
func (l *lexer) next() error {
	s := l.src[l.pos:]
	switch l.state() {
	case "ID", "ALIAS", "LINE":
		if n := spaceRun(s, false); n > 0 {
			l.skip(n)
			return nil
		}
		if strings.HasPrefix(s, "#") {
			l.skip(lineEnd(s))
			return nil
		}
		switch l.state() {
		case "ID":
			return l.id(s)
		case "ALIAS":
			l.pop()
			l.pop()
			if word(s, "as") {
				l.push("LINE")
				l.emit(tAs, 2, "as")
			} else {
				l.emit(tNewline, 0, "")
			}
			return nil
		}
		// LINE: (?:[:]?(?:no)?wrap:)?[^#\n;]*
		n := strings.IndexAny(s, "#\n;")
		if n < 0 {
			n = len(s)
		}
		l.pop()
		l.emit(tRest, n, s[:n])
		return nil
	case "CONFIG":
		if !strings.HasPrefix(s, "}") {
			n := strings.IndexByte(s, '}')
			if n < 0 {
				n = len(s)
			}
			if n == 0 {
				return l.errorf(s)
			}
			l.emit(tConfig, n, s[:n])
			return nil
		}
		l.pop()
		if asAhead(s[1:]) {
			l.push("ALIAS")
		} else {
			l.pop()
		}
		l.skip(1)
		return nil
	case "ACC":
		n := lineEnd(s)
		l.pop()
		l.emit(tAcc, n, "")
		return nil
	case "ACC_MULTI":
		if strings.HasPrefix(s, "}") {
			l.pop()
			l.skip(1)
			return nil
		}
		n := strings.IndexByte(s, '}')
		if n < 0 {
			n = len(s)
		}
		l.emit(tAcc, n, "")
		return nil
	}
	return l.initial(s)
}

func (l *lexer) initial(s string) error {
	if s == "" {
		l.emit(tNewline, 0, "")
		return nil
	}
	c := s[0]
	switch {
	case c == '\n':
		l.emit(tNewline, len(s)-len(strings.TrimLeft(s, "\n")), "\n")
		return nil
	case spaceRun(s, true) > 0:
		l.skip(spaceRun(s, true))
		return nil
	case c == '#', strings.HasPrefix(s, "%%") && !strings.HasPrefix(s, "%%{"):
		l.skip(lineEnd(s))
		return nil
	}
	if _, n := utf8.DecodeRuneInString(s); c != '}' && strings.HasPrefix(s[n:], "%%") {
		l.skip(lineEnd(s))
		return nil
	}
	if n := number(s); n > 0 {
		l.emit(tNum, n, s[:n])
		return nil
	}
	for _, k := range keywords {
		if word(s, k.word) {
			if k.state != "" {
				l.push(k.state)
			}
			l.emit(tKeyword, len(k.word), k.word)
			return nil
		}
	}
	switch {
	case word(s, "left of"):
		l.emit(tLeftOf, 7, "")
		return nil
	case word(s, "right of"):
		l.emit(tRightOf, 8, "")
		return nil
	}
	for _, w := range []string{"links", "link", "properties", "details"} {
		if hasFold(s, w) && linkAhead(s[len(w):]) {
			l.emit(tKeyword, len(w), w)
			return nil
		}
	}
	for _, w := range []string{"over", "note", "activate", "deactivate"} {
		if word(s, w) {
			if w == "activate" || w == "deactivate" {
				l.push("ID")
			}
			l.emit(tKeyword, len(w), w)
			return nil
		}
	}
	for _, w := range []string{"title", "title:"} {
		if hasFold(s, w) && len(s) > len(w) && isSpace(rune(s[len(w)])) {
			rest := s[len(w)+1:]
			if n := strings.IndexAny(rest, "#\n;"); n != 0 {
				if n < 0 {
					n = len(rest)
				}
				l.emit(tTitle, len(w)+1+n, rest[:n])
				return nil
			}
		}
	}
	for _, w := range []string{"accTitle", "accDescr"} {
		if !hasFold(s, w) {
			continue
		}
		rest := s[len(w):]
		n := len(w) + spaceRun(rest, true)
		switch {
		case strings.HasPrefix(s[n:], ":"):
			n++
			n += spaceRun(s[n:], true)
			l.push("ACC")
			l.emit(tAcc, n, "")
			return nil
		case w == "accDescr" && strings.HasPrefix(s[n:], "{"):
			n++
			n += spaceRun(s[n:], true)
			l.push("ACC_MULTI")
			l.skip(n)
			return nil
		}
	}
	for _, w := range []string{"sequenceDiagram", "autonumber", "off"} {
		if word(s, w) {
			l.emit(tKeyword, len(w), strings.ToLower(w))
			return nil
		}
	}
	switch {
	case c == ',':
		l.emit(tComma, 1, ",")
		return nil
	case c == ';':
		l.emit(tNewline, 1, ";")
		return nil
	}
	if n := actorName(s); n > 0 {
		l.emit(tActor, n, strings.TrimSpace(s[:n]))
		return nil
	}
	for _, a := range arrows {
		if hasFold(s, a.text) {
			l.out = append(l.out, token{kind: tArrow, text: a.text, arrow: a.kind, line: l.line})
			l.skip(len(a.text))
			return nil
		}
	}
	switch {
	case c == ':':
		n := strings.IndexAny(s, "#\n;")
		if n < 0 {
			n = len(s)
		}
		l.emit(tText, n, s[:n])
	case c == '+':
		l.emit(tPlus, 1, "+")
	case c == '-':
		l.emit(tMinus, 1, "-")
	case strings.HasPrefix(s, "()"):
		l.emit(tCircle, 2, "()")
	default:
		_, n := utf8.DecodeRuneInString(s)
		l.emit(tInvalid, n, s[:n])
	}
	return nil
}

// id lexes in the ID condition: an actor's name, perhaps followed by
// @{...} or "as".
func (l *lexer) id(s string) error {
	if strings.HasPrefix(s, "@{") {
		l.push("CONFIG")
		l.emit(tConfigOpen, 2, "@{")
		return nil
	}
	// [^<>:\n,;@\s]+ followed by @{ or by " as ".
	n := strings.IndexFunc(s, func(r rune) bool { return strings.ContainsRune("<>:\n,;@", r) || isSpace(r) })
	if n < 0 {
		n = len(s)
	}
	if n > 0 {
		rest := s[n:]
		if strings.HasPrefix(rest[spaceRun(rest, true):], "@{") {
			l.emit(tActor, n, s[:n])
			return nil
		}
		if asAhead(rest) {
			l.push("ALIAS")
			l.emit(tActor, n, s[:n])
			return nil
		}
	}
	// [^<>:\n,;@]+ followed by the end of the line (or a ; or #).
	m := strings.IndexAny(s, "<>:\n,;@")
	if m < 0 {
		m = len(s)
	}
	for k := m; k > 0; k-- {
		if lineEndsAt(s[k:]) {
			l.pop()
			l.emit(tActor, k, strings.TrimSpace(s[:k]))
			return nil
		}
	}
	// Anything else up to the end of the line is invalid.
	e := lineEnd(s)
	if e == 0 {
		return l.errorf(s)
	}
	l.pop()
	l.emit(tInvalid, e, strings.TrimSpace(s[:e]))
	return nil
}

func (l *lexer) errorf(s string) error {
	return fmt.Errorf("sequence: lexical error on line %d: unrecognized text %q", l.line, s[:lineEnd(s)])
}

// actorName is the ACTOR rule: a name runs up to an arrow, a colon, a
// comma, a +, or the end of the line; hyphens are allowed inside it unless
// they start an arrow.
func actorName(s string) int {
	i := strings.IndexAny(s, "/\\+()<->:\n,;")
	if i < 0 {
		i = len(s)
	}
	if i == 0 {
		return 0
	}
	for {
		if prefixAny(s[i:], notArrow) {
			return i
		}
		j := i
		for j < len(s) && s[j] == '-' {
			j++
		}
		k := strings.IndexAny(s[j:], "+<->:\n,;")
		if k < 0 {
			k = len(s) - j
		}
		if k == 0 {
			return i
		}
		i = j + k
	}
}

func prefixAny(s string, list []string) bool {
	for _, p := range list {
		if hasFold(s, p) {
			return true
		}
	}
	return false
}

// number is the NUM rule: 12, 1.5 or .25, followed by a space or newline.
func number(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	ok := func(n int) bool { return n < len(s) && (s[n] == ' ' || s[n] == '\n') }
	if i < len(s) && s[i] == '.' {
		// Up to two decimals, the longest that is followed by a space.
		for d := 2; d >= 1; d-- {
			if i+1+d <= len(s) && allDigits(s[i+1:i+1+d]) && ok(i+1+d) {
				return i + 1 + d
			}
		}
	}
	if i > 0 && ok(i) {
		return i
	}
	return 0
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// word reports whether s starts with w, ignoring case, as a whole word.
func word(s, w string) bool {
	if !hasFold(s, w) {
		return false
	}
	if len(s) == len(w) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[len(w):])
	return !isWordRune(r)
}

func hasFold(s, p string) bool { return len(s) >= len(p) && strings.EqualFold(s[:len(p)], p) }

// isWordRune is \w (ASCII only, as in JavaScript).
func isWordRune(r rune) bool {
	return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// isSpace is JavaScript's \s.
func isSpace(r rune) bool { return unicode.IsSpace(r) || r == 0xfeff }

// spaceRun is the length of the white space s starts with; newlines too
// if withNewlines.
func spaceRun(s string, withNewlines bool) int {
	n := 0
	for _, r := range s {
		if !isSpace(r) || r == '\n' && !withNewlines {
			break
		}
		n += utf8.RuneLen(r)
	}
	return n
}

// lineEnd is the length of s up to its first newline.
func lineEnd(s string) int {
	if n := strings.IndexByte(s, '\n'); n >= 0 {
		return n
	}
	return len(s)
}

// asAhead is (?=\s+as\s).
func asAhead(s string) bool {
	n := spaceRun(s, true)
	if n == 0 || !hasFold(s[n:], "as") || len(s) <= n+2 {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[n+2:])
	return isSpace(r)
}

// lineEndsAt is (?=\s*[\n;#]|$).
func lineEndsAt(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		switch {
		case r == '\n' || r == ';' || r == '#':
			return true
		case !isSpace(r):
			return false
		}
	}
	return false
}

// linkAhead is the lookahead after links/link/properties/details:
// (?=[ \t]+[^\/\\\+\(\)<>:\n,;\s\-]).
func linkAhead(s string) bool {
	n := len(s) - len(strings.TrimLeft(s, " \t"))
	if n == 0 || n == len(s) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[n:])
	return !strings.ContainsRune(`/\+()<>:`+"\n,;-", r) && !isSpace(r)
}

// Package jison runs lexers written as jison's, for the diagrams whose
// grammars mermaid writes in jison (flowchart, class, state, ER,
// requirement). A diagram keeps its rule table, in its .jison's order, and
// its own token kinds; this package does what jison's generated lexer does
// with them:
//
//   - rules are tried in order and the first that matches wins (not the
//     longest);
//   - a pattern ending in a word character (not an escape) only matches a
//     whole word: jison appends \b to it;
//   - start conditions are exclusive and kept on a stack whose bottom,
//     INITIAL, never pops; a rule names the conditions it applies in
//     ("" INITIAL only, "*" all);
//   - patterns are JavaScript's: \s, \S and '.' mean what they do there;
//   - <<EOF>> rules, and every rule's one try at the end of the input;
//   - actions may change the state, the token's kind and text, or give
//     input back (jison's this.less).
package jison

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token is a token: its kind, its text, and where it starts (a byte offset
// and a 1-based line).
type Token[K comparable] struct {
	Kind K
	Text string
	Pos  int
	Line int
	end  bool // the token Lex ends with
}

// String describes the token for error messages.
func (t Token[K]) String() string {
	switch {
	case t.end:
		return "end of input"
	case t.Text != "" && strings.Trim(t.Text, "\r\n") == "":
		return "newline"
	}
	return strconv.Quote(Snippet(t.Text))
}

// Rule is one lexer rule. Build it with On, Lit or Func, then add what its
// action does with the methods; the action runs in their order: Pop, Push,
// Text, Do.
type Rule[K comparable] struct {
	states       string
	pattern      string
	literal, eof bool
	match        func(string) int
	// first is the bytes a match can start with; nil when it can be empty
	// or is not known (a Func).
	first           *[256]bool
	ahead, notAhead string
	kind            K
	pop             bool
	push            string
	text            *string
	do              func(*Scanner[K], *Token[K])
}

// On is a rule matching a JavaScript pattern, or "<<EOF>>", in states
// ("" INITIAL, "*" all, or a comma-separated list), making a token of kind
// (Options.Skip for none).
func On[K comparable](states, pattern string, kind K) Rule[K] {
	return Rule[K]{states: states, pattern: pattern, eof: pattern == "<<EOF>>", kind: kind}
}

// Lit is a rule matching a literal, jison's "quoted" pattern.
func Lit[K comparable](states, literal string, kind K) Rule[K] {
	return Rule[K]{states: states, pattern: literal, literal: true, kind: kind}
}

// Func is a rule matching with a function (the length matched at the start
// of its argument, or -1), for what Go's regexp cannot write. The function
// is called with "" at the end of the input.
func Func[K comparable](states string, match func(string) int, kind K) Rule[K] {
	return Rule[K]{states: states, match: match, kind: kind}
}

// Push makes the rule enter state (jison's this.begin).
func (r Rule[K]) Push(state string) Rule[K] { r.push = state; return r }

// Pop makes the rule leave the current state (this.popState).
func (r Rule[K]) Pop() Rule[K] { r.pop = true; return r }

// Switch makes the rule leave the current state and enter state.
func (r Rule[K]) Switch(state string) Rule[K] { r.pop, r.push = true, state; return r }

// Text makes the token's text s instead of what matched.
func (r Rule[K]) Text(s string) Rule[K] { r.text = &s; return r }

// Ahead makes the rule match only when pattern follows (jison's (?=...)
// after the rule's pattern; as there, no \b is added).
func (r Rule[K]) Ahead(pattern string) Rule[K] { r.ahead = pattern; return r }

// NotAhead makes the rule fail when pattern follows its match (jison's
// (?!...) after the rule's pattern; as there, no \b is added).
func (r Rule[K]) NotAhead(pattern string) Rule[K] { r.notAhead = pattern; return r }

// Do runs f last: it may change the token's kind and text, the state, or
// give input back.
func (r Rule[K]) Do(f func(s *Scanner[K], t *Token[K])) Rule[K] { r.do = f; return r }

// Options are a lexer's settings.
type Options[K comparable] struct {
	Skip K // the kind of rules that make no token
	EOF  K // the kind of the token Lex ends with
	// CaseInsensitive is %options case-insensitive.
	CaseInsensitive bool
}

// Lexer is a compiled rule table.
type Lexer[K comparable] struct {
	opts  Options[K]
	rules []Rule[K]
	// byState lists the rules of each state the table names, in order;
	// star the rules of all states (for a state only an action names).
	byState map[string][]*Rule[K]
	star    []*Rule[K]
}

// New compiles rules; it panics on a bad pattern.
func New[K comparable](opts Options[K], rules ...Rule[K]) *Lexer[K] {
	lx := &Lexer[K]{opts: opts, rules: append([]Rule[K](nil), rules...), byState: map[string][]*Rule[K]{}}
	flags := ""
	if opts.CaseInsensitive {
		flags = "(?i)"
	}
	names := []string{"INITIAL"}
	for i := range lx.rules {
		r := &lx.rules[i]
		if r.match == nil && !r.eof {
			r.match, r.first = compile(flags, r.pattern, r.literal, r.ahead, r.notAhead)
		}
		if r.states != "" && r.states != "*" {
			names = append(names, strings.Split(r.states, ",")...)
		}
		if r.push != "" {
			names = append(names, r.push)
		}
	}
	for _, n := range names {
		lx.byState[n] = nil
	}
	for i := range lx.rules {
		r := &lx.rules[i]
		switch r.states {
		case "*":
			lx.star = append(lx.star, r)
			for n := range lx.byState {
				lx.byState[n] = append(lx.byState[n], r)
			}
		case "":
			lx.byState["INITIAL"] = append(lx.byState["INITIAL"], r)
		default:
			for _, n := range strings.Split(r.states, ",") {
				lx.byState[n] = append(lx.byState[n], r)
			}
		}
	}
	return lx
}

// compile makes a matcher of a pattern.
func compile(flags, pattern string, literal bool, ahead, notAhead string) (func(string) int, *[256]bool) {
	if literal {
		pattern = regexp.QuoteMeta(pattern)
	}
	if ahead == "" && notAhead == "" && wordEnd.MatchString(pattern) && !escapeEnd.MatchString(pattern) {
		pattern += `\b`
	}
	re := regexp.MustCompile(flags + `^(?:` + jsRegex(pattern) + `)`)
	first := firstBytes(flags + jsRegex(pattern))
	match := func(s string) int {
		if m := re.FindStringIndex(s); m != nil {
			return m[1]
		}
		return -1
	}
	if ahead != "" {
		// The pattern is group 1, the lookahead's match after it.
		re := regexp.MustCompile(flags + `^(` + jsRegex(pattern) + `)(?:` + jsRegex(ahead) + `)`)
		match = func(s string) int {
			if m := re.FindStringSubmatchIndex(s); m != nil {
				return m[3]
			}
			return -1
		}
	}
	if notAhead == "" {
		return match, first
	}
	not := regexp.MustCompile(flags + `^(?:` + jsRegex(notAhead) + `)`)
	return func(s string) int {
		n := match(s)
		if n >= 0 && not.MatchString(s[n:]) {
			return -1
		}
		return n
	}, first
}

// firstBytes is the set of bytes a match of pattern can start with, nil
// when it can match the empty string. Non-ASCII runes count as every UTF-8
// lead byte.
func firstBytes(pattern string) *[256]bool {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil
	}
	var set [256]bool
	if first(re.Simplify(), &set) {
		return nil
	}
	return &set
}

// first adds the bytes re's matches can start with to set and reports
// whether re can match the empty string.
func first(re *syntax.Regexp, set *[256]bool) bool {
	add := func(lo, hi rune) {
		for r := lo; r <= hi && r < utf8.RuneSelf; r++ {
			set[r] = true
		}
		if hi >= utf8.RuneSelf {
			for b := 0xC0; b < 0x100; b++ {
				set[b] = true
			}
		}
	}
	switch re.Op {
	case syntax.OpNoMatch:
		return false
	case syntax.OpLiteral:
		if len(re.Rune) == 0 {
			return true
		}
		r := re.Rune[0]
		add(r, r)
		if re.Flags&syntax.FoldCase != 0 {
			for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
				add(f, f)
			}
		}
		return false
	case syntax.OpCharClass:
		for i := 0; i+1 < len(re.Rune); i += 2 {
			add(re.Rune[i], re.Rune[i+1])
		}
		return false
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		add(0, unicode.MaxRune)
		return false
	case syntax.OpCapture, syntax.OpPlus:
		return first(re.Sub[0], set)
	case syntax.OpStar, syntax.OpQuest:
		first(re.Sub[0], set)
		return true
	case syntax.OpRepeat:
		return first(re.Sub[0], set) || re.Min == 0
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			if !first(sub, set) {
				return false
			}
		}
		return true
	case syntax.OpAlternate:
		empty := false
		for _, sub := range re.Sub {
			empty = first(sub, set) || empty
		}
		return empty
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText,
		syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return true
	}
	add(0, unicode.MaxRune)
	return true
}

// jison's test for a pattern that gets \b.
var (
	wordEnd   = regexp.MustCompile(`\w$`)
	escapeEnd = regexp.MustCompile(`\\(?:[rfntvsb]|c[A-Z]|x[0-9A-F]{2}|u[a-fA-F0-9]{4}|[0-7]{1,3})$`)
)

// Scanner is a lexer at work, as actions see it.
type Scanner[K comparable] struct {
	src        string
	pos, start int // where the input goes on; where the current match began
	line       int
	stack      []string
	out        []Token[K]
	cur        *Token[K]
}

// State is the current start condition.
func (s *Scanner[K]) State() string { return s.stack[len(s.stack)-1] }

// Push enters a state.
func (s *Scanner[K]) Push(state string) { s.stack = append(s.stack, state) }

// Pop leaves the current state; INITIAL stays.
func (s *Scanner[K]) Pop() {
	if len(s.stack) > 1 {
		s.stack = s.stack[:len(s.stack)-1]
	}
}

// Less keeps only the first n bytes of the match (and of the token's
// text), giving the rest back to the input.
func (s *Scanner[K]) Less(n int) {
	s.pos = s.start + n
	s.cur.Text = s.src[s.start:s.pos]
}

// Tokens are the tokens made so far.
func (s *Scanner[K]) Tokens() []Token[K] { return s.out }

// Lex tokenizes src, up to and including a token of kind Options.EOF.
func (lx *Lexer[K]) Lex(src string) ([]Token[K], error) {
	s := &Scanner[K]{src: src, line: 1, stack: []string{"INITIAL"}}
	for done := false; ; {
		if s.pos >= len(src) {
			if done {
				break
			}
			done = true // as jison: the rules get one try at the end
		}
		matched := false
		for _, r := range lx.rulesOf(s.State()) {
			n := -1
			if !r.eof {
				if r.first != nil && (s.pos >= len(src) || !r.first[src[s.pos]]) {
					continue
				}
				n = r.match(src[s.pos:])
			} else if done {
				n = 0
			}
			if n < 0 {
				continue
			}
			depth, state, pos, line := len(s.stack), s.State(), s.pos, s.line
			t := Token[K]{Kind: r.kind, Text: src[pos : pos+n], Pos: pos, Line: line}
			s.start, s.pos, s.cur = pos, pos+n, &t
			if r.pop {
				s.Pop()
			}
			if r.push != "" {
				s.Push(r.push)
			}
			if r.text != nil {
				t.Text = *r.text
			}
			if r.do != nil {
				r.do(s, &t)
			}
			if s.pos == pos && !done && len(s.stack) == depth && s.State() == state {
				continue // an empty match that changes nothing would loop
			}
			s.line += strings.Count(src[pos:s.pos], "\n")
			if t.Kind != lx.opts.Skip {
				s.out = append(s.out, t)
			}
			matched = true
			break
		}
		if !matched {
			if done {
				break
			}
			return nil, fmt.Errorf("line %d: unrecognized text %s", s.line, strconv.Quote(Snippet(src[s.pos:])))
		}
	}
	return append(s.out, Token[K]{Kind: lx.opts.EOF, Pos: len(src), Line: s.line, end: true}), nil
}

func (lx *Lexer[K]) rulesOf(state string) []*Rule[K] {
	if rs, ok := lx.byState[state]; ok {
		return rs
	}
	return lx.star
}

// Snippet is the start of s, up to its first line break or 20 characters,
// for error messages.
func Snippet(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	if r := []rune(s); len(r) > 20 {
		s = string(r[:20]) + "…"
	}
	return s
}

// Regexp compiles a JavaScript pattern (as jsRegex rewrites it); it panics
// on a bad one.
func Regexp(pattern string) *regexp.Regexp { return regexp.MustCompile(jsRegex(pattern)) }

// spaces are what \s matches in JavaScript.
var spaces = []rune{'\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680,
	0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a,
	0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff}

// spaceClass is the inside of a character class of spaces but those in
// except.
func spaceClass(except string) string {
	var b strings.Builder
	for _, r := range spaces {
		if !strings.ContainsRune(except, r) {
			fmt.Fprintf(&b, `\x{%x}`, r)
		}
	}
	return b.String()
}

// classEscapes are the escapes [^\S...] may list after \S.
var classEscapes = map[byte]rune{'n': '\n', 'r': '\r', 't': '\t', 'v': '\v', 'f': '\f'}

// jsRegex rewrites a JavaScript pattern for Go: \s, \S and '.' as
// JavaScript has them, and [^\S...] (white space but the characters
// listed).
func jsRegex(p string) string {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c == '[' && !inClass && strings.HasPrefix(p[i:], `[^\S`):
			end := strings.IndexByte(p[i:], ']')
			except := ""
			for j := i + 4; j < i+end; j++ {
				if p[j] == '\\' {
					j++
					r, ok := classEscapes[p[j]]
					if !ok {
						panic("jison: unsupported escape in " + p)
					}
					except += string(r)
				} else {
					except += string(p[j])
				}
			}
			b.WriteString("[" + spaceClass(except) + "]")
			i += end
		case c == '\\' && i+1 < len(p):
			i++
			switch n := p[i]; {
			case n == 's' && inClass:
				b.WriteString(spaceClass(""))
			case n == 's':
				b.WriteString("[" + spaceClass("") + "]")
			case n == 'S' && !inClass:
				b.WriteString("[^" + spaceClass("") + "]")
			default:
				b.WriteByte('\\')
				b.WriteByte(n)
			}
		case c == '[' && !inClass:
			inClass = true
			b.WriteByte(c)
		case c == ']' && inClass:
			inClass = false
			b.WriteByte(c)
		case c == '.' && !inClass:
			b.WriteString(`[^\n\r\x{2028}\x{2029}]`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// Package lex tokenizes the diagrams whose grammars mermaid writes in
// Langium, with the terminals of its common.langium (STRING, NUMBER, ID,
// BOOLEAN, title/accTitle/accDescr, %% comments), for recursive-descent
// parsers.
package lex

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Kind is a token's kind.
type Kind uint8

const (
	EOF Kind = iota
	Newline
	ID
	String
	Number
	// Punct is one character of punctuation, or one of the multi-character
	// operators given to Lex.
	Punct
	Title
	AccTitle
	AccDescr
)

// Token is a token: its text (a String without its quotes and with escapes
// kept as written; a Title's or accessibility statement's value), the number
// for a Number, and its line (1-based).
type Token struct {
	Kind Kind
	Text string
	Num  float64
	Line int
}

func (t Token) String() string {
	switch t.Kind {
	case EOF:
		return "end of input"
	case Newline:
		return "newline"
	}
	return strconv.Quote(t.Text)
}

var (
	title    = regexp.MustCompile(`^[\t ]*title(?:[\t ]+([^\n\r]*?))?[\t ]*(?:%%[^\n\r]*)?(?:\r?\n|$)`)
	accTitle = regexp.MustCompile(`^[\t ]*accTitle[\t ]*:([^\n\r]*?)(?:%%[^\n\r]*)?(?:\r?\n|$)`)
	accDescr = regexp.MustCompile(`^[\t ]*accDescr(?:[\t ]*:([^\n\r]*?)(?:%%[^\n\r]*)?(?:\r?\n|$)|\s*\{([^}]*)\})`)
	number   = regexp.MustCompile(`^(?:[0-9]+\.[0-9]+|0|[1-9][0-9]*)`)
	str      = regexp.MustCompile(`^(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')`)
	id       = regexp.MustCompile(`^[\w](?:[-\w]*\w)?`)
)

// Lex tokenizes src. ops are multi-character operators to keep whole
// ("-->", "::"), longest first.
func Lex(src string, ops ...string) ([]Token, error) {
	var out []Token
	line := 1
	atLineStart := true
	for len(src) > 0 {
		// Line statements, at the start of a line.
		if atLineStart {
			if m := title.FindStringSubmatch(src); m != nil {
				out = append(out, Token{Kind: Title, Text: strings.TrimSpace(m[1]), Line: line})
				src, line = advance(src, len(m[0]), line)
				out = append(out, Token{Kind: Newline, Line: line - 1})
				continue
			}
			if m := accTitle.FindStringSubmatch(src); m != nil {
				out = append(out, Token{Kind: AccTitle, Text: strings.TrimSpace(m[1]), Line: line})
				src, line = advance(src, len(m[0]), line)
				out = append(out, Token{Kind: Newline, Line: line - 1})
				continue
			}
			if m := accDescr.FindStringSubmatch(src); m != nil {
				out = append(out, Token{Kind: AccDescr, Text: strings.TrimSpace(m[1] + m[2]), Line: line})
				src, line = advance(src, len(m[0]), line)
				continue
			}
		}
		c := src[0]
		switch {
		case c == '\n' || c == '\r':
			n := 1
			if c == '\r' && len(src) > 1 && src[1] == '\n' {
				n = 2
			}
			out = append(out, Token{Kind: Newline, Line: line})
			src, line = src[n:], line+1
			atLineStart = true
			continue
		case c == ' ' || c == '\t':
			src = src[1:]
			continue
		case strings.HasPrefix(src, "%%"):
			end := strings.IndexAny(src, "\r\n")
			if end < 0 {
				end = len(src)
			}
			src = src[end:]
			continue
		}
		atLineStart = false
		if m := str.FindString(src); m != "" {
			out = append(out, Token{Kind: String, Text: m[1 : len(m)-1], Line: line})
			src, line = advance(src, len(m), line)
			continue
		}
		if m := number.FindString(src); m != "" && !(len(m) < len(src) && (src[len(m)] == '.' || isWord(src[len(m)]))) {
			v, _ := strconv.ParseFloat(m, 64)
			out = append(out, Token{Kind: Number, Text: m, Num: v, Line: line})
			src = src[len(m):]
			continue
		}
		if m := id.FindString(src); m != "" {
			out = append(out, Token{Kind: ID, Text: m, Line: line})
			src = src[len(m):]
			continue
		}
		op := src[:1]
		for _, o := range ops {
			if strings.HasPrefix(src, o) && len(o) > len(op) {
				op = o
			}
		}
		if c >= 0x80 {
			return nil, fmt.Errorf("line %d: unexpected character %q", line, []rune(src)[0])
		}
		out = append(out, Token{Kind: Punct, Text: op, Line: line})
		src = src[len(op):]
	}
	return append(out, Token{Kind: EOF, Line: line}), nil
}

func isWord(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func advance(src string, n, line int) (string, int) {
	return src[n:], line + strings.Count(src[:n], "\n")
}

// Parser walks tokens.
type Parser struct {
	Toks []Token
	I    int
}

// Peek is the current token.
func (p *Parser) Peek() Token { return p.Toks[p.I] }

// Next takes the current token.
func (p *Parser) Next() Token {
	t := p.Toks[p.I]
	if t.Kind != EOF {
		p.I++
	}
	return t
}

// Is reports whether the current token is of a kind and, if text is not
// empty, has that text.
func (p *Parser) Is(k Kind, text string) bool {
	t := p.Peek()
	return t.Kind == k && (text == "" || t.Text == text)
}

// Accept takes the current token if it matches.
func (p *Parser) Accept(k Kind, text string) (Token, bool) {
	if p.Is(k, text) {
		return p.Next(), true
	}
	return Token{}, false
}

// Expect takes the current token, which must match.
func (p *Parser) Expect(k Kind, text string) (Token, error) {
	if t, ok := p.Accept(k, text); ok {
		return t, nil
	}
	want := text
	if want == "" {
		want = [...]string{"end of input", "newline", "identifier", "string", "number", "punctuation", "title", "accTitle", "accDescr"}[k]
	}
	return Token{}, p.Errorf("expected %s, got %s", want, p.Peek())
}

// SkipNewlines takes any newlines.
func (p *Parser) SkipNewlines() {
	for p.Is(Newline, "") {
		p.Next()
	}
}

// Errorf is an error at the current token's line.
func (p *Parser) Errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s", p.Peek().Line, fmt.Sprintf(format, args...))
}

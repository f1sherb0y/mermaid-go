package state

import (
	"fmt"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/jison"
)

type (
	token   = jison.Token[string]
	scanner = jison.Scanner[string]
)

// processID cuts an id at an inline %% comment, giving the comment back;
// false, and no token, when the text is all comment.
func processID(s *scanner, t *token) bool {
	i := strings.Index(t.Text, "%%")
	if i == 0 {
		t.Kind = ""
		return false
	}
	if i > 0 {
		s.Less(i)
	}
	return true
}

// shapeState is a state with a <<fork>>, [[join]], ... marker: the text
// before it, trimmed.
func shapeState(marker int) func(*scanner, *token) {
	return func(_ *scanner, t *token) { t.Text = strings.TrimSpace(t.Text[:len(t.Text)-marker]) }
}

// lexer is stateDiagram.jison's lexer.
var lexer = jison.New(jison.Options[string]{EOF: "EOF", CaseInsensitive: true},
	jison.On("", `click`, "CLICK"),
	jison.On("", `href`, "HREF"),
	jison.On("", `"[^"]*"`, "STRING"),
	jison.On("", `default`, "DEFAULT"),
	jison.On("", `.*direction\s+TB[^\n]*`, "direction_tb"),
	jison.On("", `.*direction\s+BT[^\n]*`, "direction_bt"),
	jison.On("", `.*direction\s+RL[^\n]*`, "direction_rl"),
	jison.On("", `.*direction\s+LR[^\n]*`, "direction_lr"),
	jison.On("", `[\n]+`, "NL"),
	jison.On("", `[\s]+`, ""),
	jison.On("ID,STATE,struct,LINE", `[^\S\n]+`, ""),
	jison.On("INITIAL,ID,STATE,struct,LINE", `#[^\n]*`, ""),
	jison.On("INITIAL,ID,STATE,struct,LINE", `%%(?:[^\n{][^\n]*)?`, ""), // %%(?!\{)[^\n]*
	jison.On("", `scale\s+`, "scale").Push("SCALE"),
	jison.On("SCALE", `\d+`, "WIDTH"),
	jison.On("SCALE", `\s+width`, "").Pop(),

	jison.On("", `accTitle\s*:\s*`, "acc_title").Push("acc_title"),
	jison.On("acc_title", `[^\n]*`, "acc_title_value").Pop(),
	jison.On("", `accDescr\s*:\s*`, "acc_descr").Push("acc_descr"),
	jison.On("acc_descr", `[^\n]*`, "acc_descr_value").Pop(),
	jison.On("", `accDescr\s*\{\s*`, "").Push("acc_descr_multiline"),
	jison.On("acc_descr_multiline", `[\}]`, "").Pop(),
	jison.On("acc_descr_multiline", `[^\}]*`, "acc_descr_multiline_value"),

	jison.On("INITIAL,struct", `classDef\s+`, "classDef").Push("CLASSDEF"),
	jison.On("CLASSDEF", `DEFAULT\s+`, "DEFAULT_CLASSDEF_ID").Switch("CLASSDEFID"),
	jison.On("CLASSDEF", `\w+\s+`, "CLASSDEF_ID").Switch("CLASSDEFID"),
	jison.On("CLASSDEFID", `[^\n]*`, "CLASSDEF_STYLEOPTS").Pop(),
	jison.On("INITIAL,struct", `class\s+`, "class").Push("CLASS"),
	jison.On("CLASS", `(\w+)+((,\s*\w+)*)`, "CLASSENTITY_IDS").Switch("CLASS_STYLE"),
	jison.On("CLASS_STYLE", `[^\n]*`, "STYLECLASS").Pop(),
	jison.On("INITIAL,struct", `style\s+`, "style").Push("STYLE"),
	jison.On("STYLE", `[\w,]+\s+`, "STYLE_IDS").Switch("STYLEDEF_STYLES"),
	jison.On("STYLEDEF_STYLES", `[^\n]*`, "STYLEDEF_STYLEOPTS").Pop(),

	jison.On("INITIAL,struct", `state\s+`, "").Push("STATE"),
	jison.On("STATE", `.*<<fork>>`, "FORK").Pop().Do(shapeState(8)),
	jison.On("STATE", `.*<<join>>`, "JOIN").Pop().Do(shapeState(8)),
	jison.On("STATE", `.*<<choice>>`, "CHOICE").Pop().Do(shapeState(10)),
	jison.On("STATE", `.*\[\[fork\]\]`, "FORK").Pop().Do(shapeState(8)),
	jison.On("STATE", `.*\[\[join\]\]`, "JOIN").Pop().Do(shapeState(8)),
	jison.On("STATE", `.*\[\[choice\]\]`, "CHOICE").Pop().Do(shapeState(10)),
	jison.On("struct", `.*direction\s+TB[^\n]*`, "direction_tb"),
	jison.On("struct", `.*direction\s+BT[^\n]*`, "direction_bt"),
	jison.On("struct", `.*direction\s+RL[^\n]*`, "direction_rl"),
	jison.On("struct", `.*direction\s+LR[^\n]*`, "direction_lr"),
	jison.On("STATE", `["]`, "").Push("STATE_STRING"),
	jison.On("STATE", `\s*as\s+`, "AS").Push("STATE_ID"),
	jison.On("STATE_ID", `[^\n\{]*`, "ID").Do(func(s *scanner, t *token) {
		if processID(s, t) {
			s.Pop()
		}
	}),
	jison.On("STATE_STRING", `["]`, "").Pop(),
	jison.On("STATE_STRING", `[^"]*`, "STATE_DESCR"),
	jison.On("STATE", `\w+\s+\w+.*?\{`, "").Do(func(_ *scanner, t *token) {
		panic(parseError{fmt.Errorf("state name must be a single word, found %q", strings.TrimSpace(t.Text))})
	}),
	jison.On("STATE", `[^\n\s\{]+`, "COMPOSIT_STATE"),
	jison.On("STATE", `\n`, "").Pop(),
	jison.On("INITIAL,STATE", `\{`, "STRUCT_START").Switch("struct"),
	jison.On("struct", `\}`, "STRUCT_STOP").Pop(),
	jison.On("struct", `[\n]`, ""),

	jison.On("INITIAL,struct", `note\s+`, "note").Push("NOTE"),
	jison.On("NOTE", `left of`, "left_of").Switch("NOTE_ID"),
	jison.On("NOTE", `right of`, "right_of").Switch("NOTE_ID"),
	jison.On("NOTE", `"`, "").Switch("FLOATING_NOTE"),
	jison.On("FLOATING_NOTE", `\s*as\s*`, "AS").Switch("FLOATING_NOTE_ID"),
	jison.On("FLOATING_NOTE", `["]`, ""),
	jison.On("FLOATING_NOTE", `[^"]*`, "NOTE_TEXT"),
	jison.On("FLOATING_NOTE_ID", `[^\n]*`, "ID").Do(func(s *scanner, t *token) {
		if processID(s, t) {
			s.Pop()
		}
	}),
	jison.On("NOTE_ID", `\s*[^:\n\s\-]+`, "ID").Do(func(s *scanner, t *token) {
		if processID(s, t) {
			s.Pop()
			s.Push("NOTE_TEXT")
		}
	}),
	jison.On("NOTE_TEXT", `\s*:[^:\n;]+`, "NOTE_TEXT").Pop().Do(func(_ *scanner, t *token) {
		r := []rune(t.Text)
		t.Text = strings.TrimSpace(string(r[min(2, len(r)):]))
	}),
	jison.On("NOTE_TEXT", `[\x00-\x{10FFFF}]*?\n\s*end note`, "NOTE_TEXT").Pop().Do(func(_ *scanner, t *token) {
		t.Text = strings.TrimSpace(t.Text[:len(t.Text)-len("end note")])
	}),

	jison.On("", `stateDiagram\s+`, "SD"),
	jison.On("", `stateDiagram-v2\s+`, "SD"),
	jison.On("", `hide empty description`, "HIDE_EMPTY"),
	jison.On("INITIAL,struct", `\[\*\]`, "EDGE_STATE"),
	jison.On("INITIAL,struct", `[^:\n\s\-\{]+`, "ID").Do(func(s *scanner, t *token) { processID(s, t) }),
	jison.On("INITIAL,struct", `\s*:(?:[^:\n;]|:[^:\n;])+`, "DESCR").Do(func(_ *scanner, t *token) {
		t.Text = strings.TrimSpace(t.Text)
	}),
	jison.On("INITIAL,struct", `-->`, "-->"),
	jison.On("struct", `--`, "CONCURRENT"),
	jison.On("INITIAL,struct", `:::`, "STYLE_SEPARATOR"),
	jison.On("", `<<EOF>>`, "NL"),
	jison.On("", `.`, "INVALID"),
)

// parser is stateDiagram.jison's grammar, recursive descent: it builds
// the statement tree the grammar's actions build.
type parser struct {
	toks     []token
	i        int
	dividers int // getDividerId's count
}

// parseError carries a syntax error up through the parser.
type parseError struct{ err error }

func (p *parser) fail(format string, args ...any) {
	line := p.peek().Line
	panic(parseError{fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, args...))})
}

func (p *parser) peek() token { return p.toks[p.i] }

func (p *parser) is(kinds ...string) bool {
	k := p.peek().Kind
	for _, want := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

func (p *parser) next() token {
	t := p.toks[p.i]
	if t.Kind != "EOF" {
		p.i++
	}
	return t
}

func (p *parser) accept(kinds ...string) (token, bool) {
	if p.is(kinds...) {
		return p.next(), true
	}
	return token{}, false
}

func (p *parser) expect(kinds ...string) token {
	if !p.is(kinds...) {
		t := p.peek()
		got := t.Kind
		if t.Text != "" && t.Text != t.Kind {
			got += fmt.Sprintf(" %q", jison.Snippet(t.Text))
		}
		p.fail("expected %s, got %s", strings.Join(kinds, " or "), got)
	}
	return p.next()
}

// root is start: newlines, the header, the root document.
func (p *parser) root() []*stmt {
	for p.is("NL") {
		p.next()
	}
	p.expect("SD")
	doc := p.document("EOF")
	p.expect("EOF")
	return doc
}

// document reads lines up to end (not taken). Statements the db ignores
// (scale, a bare `state X`, floating notes, clicks, ...) stay as empty
// placeholders: they still count when a composite splits at its dividers.
func (p *parser) document(end string) []*stmt {
	doc := []*stmt{}
	for !p.is(end) {
		if _, ok := p.accept("NL"); ok {
			continue
		}
		if p.is("EOF") {
			p.expect(end)
		}
		doc = append(doc, p.statement())
	}
	return doc
}

func (p *parser) statement() *stmt {
	switch t := p.next(); t.Kind {
	case "classDef":
		id := p.expect("CLASSDEF_ID", "DEFAULT").Text
		return &stmt{kind: "classDef", id: strings.TrimSpace(id), value: strings.TrimSpace(p.expect("CLASSDEF_STYLEOPTS").Text)}
	case "style":
		id := p.expect("STYLE_IDS").Text
		return &stmt{kind: "style", id: strings.TrimSpace(id), value: strings.TrimSpace(p.expect("STYLEDEF_STYLEOPTS").Text)}
	case "class":
		id := p.expect("CLASSENTITY_IDS").Text
		return &stmt{kind: "applyClass", id: strings.TrimSpace(id), value: strings.TrimSpace(p.expect("STYLECLASS").Text)}
	case "ID", "EDGE_STATE":
		s1 := p.idStatement(t)
		if d, ok := p.accept("DESCR"); ok {
			s1.descr = []string{trimColon(d.Text)}
			return s1
		}
		if _, ok := p.accept("-->"); !ok {
			return s1
		}
		r := &stmt{kind: "relation", s1: s1, s2: p.idStatement(p.expect("ID", "EDGE_STATE"))}
		if d, ok := p.accept("DESCR"); ok {
			r.value = trimColon(d.Text)
		}
		return r
	case "HIDE_EMPTY":
	case "scale":
		p.expect("WIDTH")
	case "COMPOSIT_STATE":
		if _, ok := p.accept("STRUCT_START"); !ok {
			return &stmt{} // `state X` alone declares nothing
		}
		return &stmt{kind: "state", id: t.Text, typ: "default", doc: p.body()}
	case "STATE_DESCR":
		p.expect("AS")
		id := p.expect("ID").Text
		if _, ok := p.accept("STRUCT_START"); ok {
			return &stmt{kind: "state", id: id, typ: "default", descr: []string{t.Text}, doc: p.body()}
		}
		s := &stmt{kind: "state", id: id, typ: "default", descr: []string{strings.TrimSpace(t.Text)}}
		if strings.Contains(id, ":") {
			parts := strings.Split(id, ":")
			s.id, s.descr = parts[0], append(s.descr, parts[1])
			s.descrList = true
		}
		return s
	case "FORK", "JOIN", "CHOICE":
		return &stmt{kind: "state", id: t.Text, typ: strings.ToLower(t.Kind)}
	case "CONCURRENT":
		p.dividers++
		return &stmt{kind: "state", id: fmt.Sprintf("divider-id-%d", p.dividers), typ: "divider"}
	case "note":
		if _, ok := p.accept("NOTE_TEXT"); ok {
			p.expect("AS")
			p.expect("ID")
			return &stmt{} // a floating note is not drawn
		}
		pos := p.expect("left_of", "right_of").Text
		id := p.expect("ID").Text
		text := p.expect("NOTE_TEXT").Text
		return &stmt{kind: "state", id: strings.TrimSpace(id),
			note: &note{position: strings.TrimSpace(pos), text: strings.TrimSpace(text)}}
	case "direction_tb", "direction_bt", "direction_rl", "direction_lr":
		return &stmt{kind: "dir", value: strings.ToUpper(strings.TrimPrefix(t.Kind, "direction_"))}
	case "acc_title":
		p.expect("acc_title_value")
	case "acc_descr":
		p.expect("acc_descr_value")
	case "acc_descr_multiline_value":
	case "CLICK":
		p.idStatement(p.expect("ID", "EDGE_STATE"))
		if _, ok := p.accept("HREF"); !ok {
			p.expect("STRING")
		}
		p.expect("STRING")
		p.expect("NL")
	default:
		p.i--
		p.fail("unexpected %s", t.Kind)
	}
	return &stmt{}
}

// body is a composite state's document, in braces (the opening one taken).
func (p *parser) body() []*stmt {
	doc := p.document("STRUCT_STOP")
	p.expect("STRUCT_STOP")
	return doc
}

// idStatement is a state reference: ID or [*], maybe with :::class.
func (p *parser) idStatement(t token) *stmt {
	s := &stmt{kind: "state", id: strings.TrimSpace(t.Text), typ: "default"}
	if _, ok := p.accept("STYLE_SEPARATOR"); ok {
		s.classes = []string{strings.TrimSpace(p.expect("ID").Text)}
	}
	return s
}

func trimColon(s string) string {
	if strings.HasPrefix(s, ":") {
		s = s[1:]
	}
	return strings.TrimSpace(s)
}

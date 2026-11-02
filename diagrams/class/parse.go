package class

import (
	"fmt"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/jison"
)

type token = jison.Token[string]

// nameGoesOn is what makes a keyword the start of a longer class name: a
// word character, a non-ASCII one, or one after '-' or '.'.
// classDiagram.jison only asks for a word's end (\b), so link-manager
// lexes as LINK and "-manager", and mermaid fails on it; we read a name.
const nameGoesOn = `[-.]?[\w\x{80}-\x{10FFFF}]`

// lexer is classDiagram.jison's lexer; the keyword rules also have
// NotAhead(nameGoesOn).
var lexer = jison.New(jison.Options[string]{EOF: "EOF"},
	jison.On("", `.*direction\s+TB[^\n]*`, "direction_tb"),
	jison.On("", `.*direction\s+BT[^\n]*`, "direction_bt"),
	jison.On("", `.*direction\s+RL[^\n]*`, "direction_rl"),
	jison.On("", `.*direction\s+LR[^\n]*`, "direction_lr"),
	jison.On("", `%%[^\n]*[\r\n]*`, ""),
	jison.On("", `accTitle\s*:\s*`, "acc_title").Push("acc_title"),
	jison.On("acc_title", `[^\n]*`, "acc_title_value").Pop(),
	jison.On("", `accDescr\s*:\s*`, "acc_descr").Push("acc_descr"),
	jison.On("acc_descr", `[^\n]*`, "acc_descr_value").Pop(),
	jison.On("", `accDescr\s*\{\s*`, "").Push("acc_descr_multiline"),
	jison.On("acc_descr_multiline", `\}`, "").Pop(),
	jison.On("acc_descr_multiline", `[^\}]*`, "acc_descr_multiline_value"),

	jison.On("", `\s*(\r?\n)+`, "NEWLINE"),
	jison.On("", `\s+`, ""),
	jison.On("", `classDiagram-v2`, "CLASS_DIAGRAM"),
	jison.On("", `classDiagram`, "CLASS_DIAGRAM"),
	jison.On("", `\[\*\]`, "EDGE_STATE"),

	jison.On("", `call[\s]+`, "").Push("callback_name"),
	jison.On("callback_name", `\([\s]*\)`, "").Pop(),
	jison.On("callback_name", `\(`, "").Switch("callback_args"),
	jison.On("callback_name", `[^(]*`, "CALLBACK_NAME"),
	jison.On("callback_args", `\)`, "").Pop(),
	jison.On("callback_args", `[^)]*`, "CALLBACK_ARGS"),

	jison.On("string", `["]`, "").Pop(),
	jison.On("string", `[^"]*`, "STR"),
	jison.On("*", `["]`, "").Push("string"),
	jison.On("", `style`, "STYLE").NotAhead(nameGoesOn),
	jison.On("", `classDef`, "CLASSDEF").NotAhead(nameGoesOn),

	jison.On("INITIAL,namespace,namespace-body", `namespace`, "NAMESPACE").Push("namespace").NotAhead(nameGoesOn),
	jison.On("namespace", `\s*(\r?\n)+`, "NEWLINE").Pop(),
	jison.On("namespace", `\s+`, ""),
	jison.On("namespace", `[{]`, "STRUCT_START").Push("namespace-body"),
	jison.On("namespace", `[}]`, "").Pop().Do(func(s *jison.Scanner[string], _ *token) { s.Less(0) }),
	jison.On("namespace-body", `[}]`, "STRUCT_STOP").Pop(),
	jison.On("namespace-body", `<<EOF>>`, "EOF_IN_STRUCT"),
	jison.On("namespace-body", `\s*(\r?\n)+`, "NEWLINE"),
	jison.On("namespace-body", `\s+`, ""),
	jison.On("namespace-body", `\[\*\]`, "EDGE_STATE"),

	jison.On("INITIAL,namespace-body", `class`, "CLASS").Push("class").NotAhead(nameGoesOn),
	jison.On("class", `\s*(\r?\n)+`, "NEWLINE").Pop(),
	jison.On("class", `\s+`, ""),
	jison.On("class", `[}]`, "STRUCT_STOP").Pop().Do(func(s *jison.Scanner[string], _ *token) { s.Pop() }),
	jison.On("class", `[{]`, "STRUCT_START").Push("class-body"),
	jison.On("class-body", `[}]`, "STRUCT_STOP").Pop(),
	jison.On("class-body", `<<EOF>>`, "EOF_IN_STRUCT"),
	jison.On("class-body", `\[\*\]`, "EDGE_STATE"),
	jison.On("class-body", `[{]`, "OPEN_IN_STRUCT"),
	jison.On("class-body", `[\n]`, ""),
	jison.On("class-body", `[^{}\n]*`, "MEMBER"),

	jison.On("*", `cssClass`, "CSSCLASS").NotAhead(nameGoesOn),
	jison.On("*", `callback`, "CALLBACK").NotAhead(nameGoesOn),
	jison.On("*", `link`, "LINK").NotAhead(nameGoesOn),
	jison.On("*", `click`, "CLICK").NotAhead(nameGoesOn),
	jison.On("*", `note for`, "NOTE_FOR").NotAhead(nameGoesOn),
	jison.On("*", `note`, "NOTE").NotAhead(nameGoesOn),
	jison.On("*", `<<`, "ANNOTATION_START"),
	jison.On("*", `>>`, "ANNOTATION_END"),
	jison.On("*", `href`, "HREF").NotAhead(nameGoesOn),

	jison.On("generic", `[~]`, "").Pop(),
	jison.On("generic", `[^~]*`, "GENERICTYPE"),
	jison.On("*", `~`, "").Push("generic"),
	jison.On("bqstring", "[`]", "").Pop(),
	jison.On("bqstring", "[^`]+", "BQUOTE_STR"),
	jison.On("*", "[`]", "").Push("bqstring"),

	jison.On("*", `_self`, "LINK_TARGET").NotAhead(nameGoesOn),
	jison.On("*", `_blank`, "LINK_TARGET").NotAhead(nameGoesOn),
	jison.On("*", `_parent`, "LINK_TARGET").NotAhead(nameGoesOn),
	jison.On("*", `_top`, "LINK_TARGET").NotAhead(nameGoesOn),

	jison.On("*", `\s*<\|`, "EXTENSION"),
	jison.On("*", `\s*\|>`, "EXTENSION"),
	jison.On("*", `\s*>`, "DEPENDENCY"),
	jison.On("*", `\s*<`, "DEPENDENCY"),
	jison.On("*", `\s*\*`, "COMPOSITION"),
	jison.On("*", `\s*o`, "AGGREGATION"),
	jison.On("*", `\s*\(\)`, "LOLLIPOP"),
	jison.On("*", `--`, "LINE"),
	jison.On("*", `\.\.`, "DOTTED_LINE"),
	jison.On("*", `:[^:\n;]+`, "LABEL"),
	jison.On("*", `:{3}`, "STYLE_SEPARATOR"),
	jison.On("*", `-`, "MINUS"),
	jison.On("*", `\.`, "DOT"),
	jison.On("*", `\+`, "PLUS"),
	jison.On("", `:`, "COLON"),
	jison.On("", `,`, "COMMA"),
	jison.On("", `#`, "BRKT"),
	jison.On("*", `%`, "PCT"),
	jison.On("*", `=`, "EQUALS"),
	jison.On("*", `\w+`, "ALPHA"),
	jison.On("*", `\[`, "SQS"),
	jison.On("*", `\]`, "SQE"),
	jison.On("*", "[!\"#$%&'*+,-.`?\\\\/]", "PUNCTUATION"),
	jison.On("*", `[0-9]+`, "NUM"),
	jison.On("*", unicodeText, "UNICODE_TEXT"),
	jison.On("*", `\s`, "SPACE"),
	jison.On("*", `<<EOF>>`, "EOF"),
)

// parser is classDiagram.jison's grammar, recursive descent; it runs the
// db's actions as jison would on each reduction.
type parser struct {
	toks []token
	i    int
	db   *db
}

// parseError carries a syntax or db error up through the parser.
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
		if t.Text != "" {
			got += fmt.Sprintf(" %q", jison.Snippet(t.Text))
		}
		p.fail("expected %s, got %s", strings.Join(kinds, " or "), got)
	}
	return p.next()
}

var (
	alphaNum     = []string{"UNICODE_TEXT", "NUM", "ALPHA", "MINUS"}
	nameStart    = append(alphaNum[:len(alphaNum):len(alphaNum)], "BQUOTE_STR")
	relationType = map[string]int{"AGGREGATION": aggregation, "EXTENSION": extension, "COMPOSITION": composition, "DEPENDENCY": dependency, "LOLLIPOP": lollipop}
	styleParts   = []string{"ALPHA", "NUM", "COLON", "UNIT", "SPACE", "BRKT", "STYLE", "PCT", "LABEL"}
)

// document is graphConfig: the header, then statements a line each.
func (p *parser) document() {
	p.expect("CLASS_DIAGRAM")
	p.expect("NEWLINE")
	p.statement()
	for p.is("NEWLINE") {
		p.next()
		if p.is("EOF") {
			break
		}
		p.statement()
	}
	p.expect("EOF")
}

func (p *parser) statement() {
	d := p.db
	switch t := p.peek(); t.Kind {
	case "direction_tb", "direction_bt", "direction_rl", "direction_lr":
		p.next()
		d.direction = strings.ToUpper(strings.TrimPrefix(t.Kind, "direction_"))
	case "acc_title":
		p.next()
		p.expect("acc_title_value")
	case "acc_descr":
		p.next()
		p.expect("acc_descr_value")
	case "acc_descr_multiline_value":
		p.next()
	case "NAMESPACE":
		p.namespace()
	case "CLASS":
		p.class()
	case "ANNOTATION_START":
		p.next()
		a := p.expect(alphaNum...).Text
		p.expect("ANNOTATION_END")
		d.addAnnotation(p.className(), a)
	case "CALLBACK", "LINK", "CLICK":
		p.click()
	case "STYLE":
		p.next()
		id := p.expect("ALPHA").Text
		d.setCSSStyle(id, p.styles())
	case "CSSCLASS":
		p.next()
		ids := p.expect("STR").Text
		d.setCSSClass(ids, p.expect("ALPHA").Text)
	case "NOTE_FOR", "NOTE":
		p.note()
	case "CLASSDEF":
		p.next()
		ids := []string{p.expect("ALPHA").Text}
		for p.is("COMMA") {
			p.next()
			ids = append(ids, p.expect("ALPHA").Text)
		}
		d.defineClass(ids, p.styles())
	case "MEMBER", "SEPARATOR":
		p.next()
	default:
		if !p.is(nameStart...) {
			p.fail("unexpected %s", t.Kind)
		}
		p.memberOrRelation()
	}
}

// memberOrRelation is memberStatement (`Name`, `Name : member`) or
// relationStatement (`A "1" <|-- "*" B : label`).
func (p *parser) memberOrRelation() {
	id1 := p.className()
	if t, ok := p.accept("LABEL"); ok {
		p.db.addMember(id1, cleanupLabel(t.Text))
		return
	}
	if !p.is("STR", "LINE", "DOTTED_LINE") && !p.isRelationType() {
		return // a bare class name declares nothing
	}
	r := &relation{id1: id1, title1: "none", title2: "none"}
	if t, ok := p.accept("STR"); ok {
		r.title1 = t.Text
	}
	r.type1, r.type2 = none, none
	if p.isRelationType() {
		r.type1 = relationType[p.next().Kind]
	}
	r.dotted = p.expect("LINE", "DOTTED_LINE").Kind == "DOTTED_LINE"
	if p.isRelationType() {
		r.type2 = relationType[p.next().Kind]
	}
	if t, ok := p.accept("STR"); ok {
		r.title2 = t.Text
	}
	r.id2 = p.className()
	if t, ok := p.accept("LABEL"); ok {
		r.title = cleanupLabel(t.Text)
	}
	p.db.addRelation(r)
}

func (p *parser) isRelationType() bool {
	_, ok := relationType[p.peek().Kind]
	return ok
}

func (p *parser) className() string { return p.name(true) }

// name is className (with generic) or namespaceName: alphaNumTokens run
// together ('.' across DOTs), maybe ending in a `backquoted` name; a class
// name may end in a ~generic~.
func (p *parser) name(generic bool) string {
	var b strings.Builder
	for {
		if t, ok := p.accept("BQUOTE_STR"); ok {
			b.WriteString(t.Text)
			break
		}
		b.WriteString(p.expect(alphaNum...).Text)
		if p.is("DOT") {
			p.next()
			b.WriteByte('.')
			continue
		}
		if generic && p.is("GENERICTYPE") {
			break
		}
		if !p.is(nameStart...) {
			return b.String()
		}
	}
	if generic {
		if t, ok := p.accept("GENERICTYPE"); ok {
			b.WriteString("~" + t.Text + "~")
		}
	}
	return b.String()
}

// label is classLabel: ["text"].
func (p *parser) label() (string, bool) {
	if _, ok := p.accept("SQS"); !ok {
		return "", false
	}
	s := p.expect("STR").Text
	p.expect("SQE")
	return s, true
}

// class is classStatement; it returns the class's name.
func (p *parser) class() string {
	d := p.db
	p.expect("CLASS")
	id := p.className()
	d.addClass(id)
	if l, ok := p.label(); ok {
		d.setClassLabel(id, l)
	}
	switch {
	case p.is("STYLE_SEPARATOR"):
		p.next()
		d.setCSSClass(id, p.expect(alphaNum...).Text)
		if p.is("STRUCT_START") {
			p.next()
			d.addMembers(id, p.members(false))
		}
	case p.is("ANNOTATION_START"):
		p.next()
		d.addAnnotation(id, p.expect(alphaNum...).Text)
		p.expect("ANNOTATION_END")
		if p.is("STRUCT_START") {
			p.next()
			d.addMembers(id, p.members(true))
		}
	case p.is("STRUCT_START"):
		p.next()
		d.addMembers(id, p.members(true))
	}
	return id
}

// members reads MEMBER lines up to STRUCT_STOP.
func (p *parser) members(emptyOK bool) []string {
	var ms []string
	for p.is("MEMBER") {
		ms = append(ms, p.next().Text)
	}
	if len(ms) == 0 && !emptyOK {
		p.expect("MEMBER")
	}
	p.expect("STRUCT_STOP")
	return ms
}

// namespace is namespaceStatement: classes, notes and namespaces in braces.
func (p *parser) namespace() {
	d := p.db
	p.expect("NAMESPACE")
	name := p.name(false)
	label, _ := p.label()
	id := d.addNamespace(name, label)
	p.expect("STRUCT_START")
	p.accept("NEWLINE")
	var classes, notes []string
	for {
		switch {
		case p.is("CLASS"):
			classes = append(classes, p.class())
		case p.is("NOTE", "NOTE_FOR"):
			notes = append(notes, p.note())
		case p.is("NAMESPACE"):
			p.namespace()
		default:
			p.fail("expected a class, note or namespace, got %s", p.peek().Kind)
		}
		if _, ok := p.accept("NEWLINE"); !ok || p.is("STRUCT_STOP") {
			break
		}
	}
	p.expect("STRUCT_STOP")
	d.addClassesToNamespace(id, classes, notes)
	d.closeNamespace()
}

// note is noteStatement; it returns the note's id.
func (p *parser) note() string {
	class := ""
	if _, ok := p.accept("NOTE_FOR"); ok {
		class = p.className()
	} else {
		p.expect("NOTE")
	}
	return p.db.addNote(p.expect("STR").Text, class)
}

// click is clickStatement: links, callbacks and tooltips are dropped, but
// they make the class "clickable".
func (p *parser) click() {
	kind := p.next().Kind
	id := p.className()
	switch kind {
	case "CALLBACK":
		p.expect("STR")
		p.accept("STR")
	case "LINK":
		p.expect("STR")
		p.linkTail()
	case "CLICK":
		if _, ok := p.accept("HREF"); ok {
			p.expect("STR")
			p.linkTail()
			break
		}
		p.expect("CALLBACK_NAME")
		p.accept("CALLBACK_ARGS")
		p.accept("STR")
	}
	p.db.setCSSClass(id, "clickable")
}

// linkTail is what may follow a link's URL: a tooltip, a target, or both.
func (p *parser) linkTail() {
	p.accept("STR")
	p.accept("LINK_TARGET")
}

// styles is stylesOpt: styles separated by commas, each its tokens' texts
// run together.
func (p *parser) styles() []string {
	var out []string
	for {
		var b strings.Builder
		b.WriteString(p.expect(styleParts...).Text)
		for p.is(styleParts...) {
			b.WriteString(p.next().Text)
		}
		out = append(out, b.String())
		if _, ok := p.accept("COMMA"); !ok {
			return out
		}
	}
}

func cleanupLabel(s string) string {
	s = strings.TrimPrefix(s, ":")
	return sanitize(strings.TrimSpace(s))
}

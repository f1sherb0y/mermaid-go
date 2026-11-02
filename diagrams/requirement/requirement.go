// Package requirement is mermaid's requirement diagram
// (diagrams/requirement): it parses requirementDiagram source into the nodes
// and edges the unified renderer draws, as requirementDb.getData() makes
// them.
package requirement

import (
	"fmt"
	"strings"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/render"
)

// Parse reads a requirement diagram (requirementDiagram.jison), code being
// what config.Preprocess leaves. Requirements and elements are
// requirementBox nodes with their fields in Extra under mermaid's names
// ("name", "type", "requirementId", "text", "risk", "verifyMethod";
// "docRef" for elements); relationships are edges labelled <<type>>.
func Parse(code string, _ *config.Config) (d *render.Data, err error) {
	toks, err := lexer.Lex(common.EncodeEntities(code))
	if err != nil {
		return nil, fmt.Errorf("requirement: %w", err)
	}
	p := &parser{toks: toks, db: newDB()}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(parseError)
			if !ok {
				panic(r)
			}
			d, err = nil, fmt.Errorf("requirement: %s", e.msg)
		}
	}()
	for p.directive() {
		p.accept(tNewline)
	}
	p.want(tHeader, "requirementDiagram")
	p.want(tNewline, "a newline")
	p.diagram()
	return p.db.data(), nil
}

type parseError struct{ msg string }

type parser struct {
	toks []token
	i    int
	db   *db
}

func (p *parser) peek() token { return p.toks[p.i] }

// next takes the next token; at the end, EOF again and again.
func (p *parser) next() token {
	t := p.toks[p.i]
	if p.i < len(p.toks)-1 {
		p.i++
	}
	return t
}

func (p *parser) accept(k kind) bool {
	if p.peek().Kind == k {
		p.next()
		return true
	}
	return false
}

func (p *parser) want(k kind, what string) token {
	t := p.next()
	if t.Kind != k {
		p.fail(t, what)
	}
	return t
}

func (p *parser) fail(t token, want string) {
	panic(parseError{fmt.Sprintf("line %d: unexpected %v, want %s", t.Line, t, want)})
}

// directive reads an accessibility statement, if one is next.
func (p *parser) directive() bool {
	switch p.peek().Kind {
	case tAccTitle:
		p.next()
		p.want(tAccTitleValue, "a title")
	case tAccDescr:
		p.next()
		p.want(tAccDescrValue, "a description")
	case tAccDescrMultiValue:
		p.next()
	default:
		return false
	}
	return true
}

func (p *parser) diagram() {
	for {
		if p.directive() {
			continue
		}
		t := p.next()
		switch t.Kind {
		case tEOF:
			return
		case tNewline:
		case tDirection:
			p.db.direction = t.Text
		case tReqType:
			p.requirement(t.Text)
		case tElement:
			p.element()
		case tQString, tUnqString:
			p.relationshipOrClass(t.Text)
		case tStyle:
			ids := p.idList()
			p.db.setStyle(ids, p.styles())
		case tClassDef:
			ids := p.idList()
			p.db.defineClass(ids, p.styles())
		case tClass:
			ids := p.idList()
			p.db.setClass(ids, p.idList())
		default:
			p.fail(t, "a statement")
		}
	}
}

// name reads a quoted or unquoted string.
func (p *parser) name() string {
	t := p.next()
	if t.Kind != tQString && t.Kind != tUnqString {
		p.fail(t, "a name")
	}
	return t.Text
}

// field is a `key: value` line of a requirement's or element's body.
type field struct {
	key  kind
	text string
}

// body reads `{` newline, fields of the given keys, and `}`; the value of
// each key is a name unless values gives its token kind.
func (p *parser) body(values map[kind]kind) []field {
	p.want(tStructStart, "'{'")
	p.want(tNewline, "a newline")
	var fields []field
	for {
		t := p.next()
		switch {
		case t.Kind == tStructStop:
			return fields
		case t.Kind == tNewline:
		default:
			vk, ok := values[t.Kind]
			if !ok {
				p.fail(t, "a field")
			}
			p.want(tColonSep, "':'")
			var v string
			if vk == tUnqString {
				v = p.name()
			} else {
				v = p.want(vk, "a value").Text
			}
			p.want(tNewline, "a newline")
			fields = append(fields, field{t.Kind, v})
		}
	}
}

// classes reads `:::a,b` if next.
func (p *parser) classes() []string {
	if p.accept(tStyleSep) {
		return p.idList()
	}
	return nil
}

func (p *parser) requirement(typ string) {
	name := p.name()
	classes := p.classes()
	fields := p.body(map[kind]kind{tID: tUnqString, tText: tUnqString, tRisk: tRiskLevel, tVerifyMethod: tVerifyType})
	n := &node{name: name, req: true, typ: typ}
	// mermaid sets the fields as the parser reduces them, last first: a
	// field given twice keeps its first value.
	for i := len(fields) - 1; i >= 0; i-- {
		f := fields[i]
		switch f.key {
		case tID:
			n.id = f.text
		case tText:
			n.text = f.text
		case tRisk:
			n.risk = f.text
		case tVerifyMethod:
			n.verify = f.text
		}
	}
	p.db.add(n)
	p.db.setClass([]string{name}, classes)
}

func (p *parser) element() {
	name := p.name()
	classes := p.classes()
	fields := p.body(map[kind]kind{tType: tUnqString, tDocRef: tUnqString})
	n := &node{name: name}
	for i := len(fields) - 1; i >= 0; i-- {
		if fields[i].key == tType {
			n.typ = fields[i].text
		} else {
			n.docRef = fields[i].text
		}
	}
	p.db.add(n)
	p.db.setClass([]string{name}, classes)
}

// relationshipOrClass reads what follows a name: `<- type - name`,
// `- type -> name` or `:::classes`.
func (p *parser) relationshipOrClass(name string) {
	t := p.next()
	switch t.Kind {
	case tArrowL:
		rel := p.want(tRelationship, "a relationship").Text
		p.want(tLine, "'-'")
		p.db.rels = append(p.db.rels, relation{rel, p.name(), name})
	case tLine:
		rel := p.want(tRelationship, "a relationship").Text
		p.want(tArrowR, "'->'")
		p.db.rels = append(p.db.rels, relation{rel, name, p.name()})
	case tStyleSep:
		p.db.setClass([]string{name}, p.idList())
	default:
		p.fail(t, "a relationship")
	}
}

func (p *parser) idList() []string {
	var ids []string
	for {
		t := p.next()
		if t.Kind != tAlpha && t.Kind != tQString && t.Kind != tUnqString {
			p.fail(t, "a name")
		}
		ids = append(ids, t.Text)
		if !p.accept(tComma) {
			return ids
		}
	}
}

// styles reads comma-separated styles, each its tokens run together.
func (p *parser) styles() []string {
	var out []string
	for {
		var s strings.Builder
		for {
			switch p.peek().Kind {
			case tAlpha, tNum, tColon, tBrkt, tMinus, tSemicolon:
				s.WriteString(p.next().Text)
				continue
			}
			break
		}
		if s.Len() == 0 {
			p.fail(p.peek(), "a style")
		}
		out = append(out, s.String())
		if !p.accept(tComma) {
			return out
		}
	}
}

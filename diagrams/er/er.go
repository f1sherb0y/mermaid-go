// Package er is mermaid's entity relationship diagram (diagrams/er): it
// parses erDiagram source into the nodes and edges the unified renderer
// draws, as erDb.getData() makes them.
package er

import (
	"fmt"
	"strings"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/render"
)

// Parse reads an ER diagram (erDiagram.jison), code being what
// config.Preprocess leaves. Entities are erBox nodes with their attributes
// and alias in Extra ("attributes", "alias"); subgraphs are groups; each
// relationship is an edge whose arrowTypeStart and arrowTypeEnd name the
// cardinalities at its ends.
func Parse(code string, _ *config.Config) (d *render.Data, err error) {
	toks, err := lexer.Lex(common.EncodeEntities(code))
	if err != nil {
		return nil, fmt.Errorf("er: %w", err)
	}
	p := &parser{toks: toks, db: newDB()}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(parseError)
			if !ok {
				panic(r)
			}
			d, err = nil, fmt.Errorf("er: %s", e.msg)
		}
	}()
	p.want(tHeader, "erDiagram")
	p.document(false)
	p.want(tEOF, "end of input")
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

func (p *parser) acceptChar(c string) bool {
	if t := p.peek(); t.Kind == tChar && t.Text == c {
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

// document reads statements up to the end, or a subgraph's up to its end;
// it returns what the statements name (a subgraph's members) and the last
// direction given inside a subgraph.
func (p *parser) document(inSubgraph bool) (members []string, dir string) {
	for {
		t := p.peek()
		switch t.Kind {
		case tEOF:
			if inSubgraph {
				p.fail(t, "end")
			}
			return members, dir
		case tEnd:
			if !inSubgraph {
				p.fail(t, "a statement")
			}
			p.next()
			return members, dir
		case tNewline:
			p.next()
		case tDirection:
			p.next()
			if inSubgraph {
				dir = t.Text
			} else {
				p.db.direction = t.Text
			}
		default:
			members = append(members, p.statement()...)
		}
	}
}

func (p *parser) statement() []string {
	t := p.next()
	switch t.Kind {
	case tEntityName, tUnicodeText, tNum, tDecimal, tEntityOne:
		return p.entity(entityName(t))
	case tAccTitle:
		p.want(tAccTitleValue, "a title")
	case tAccDescr:
		p.want(tAccDescrValue, "a description")
	case tAccDescrMultiValue:
	case tClassDef:
		ids := p.idList()
		p.db.addClass(ids, p.styles())
		p.separator()
	case tStyle:
		ids := p.idList()
		p.db.addStyles(ids, p.styles())
		p.separator()
	case tClass:
		ids := p.idList()
		p.db.setClass(ids, p.idList())
	case tSubgraph:
		return []string{p.subgraph()}
	default:
		p.fail(t, "a statement")
	}
	return nil
}

func isEntityName(k kind) bool {
	return k == tEntityName || k == tUnicodeText || k == tNum || k == tDecimal || k == tEntityOne
}

func entityName(t token) string {
	if t.Kind == tEntityName {
		return strings.ReplaceAll(t.Text, `"`, "")
	}
	return t.Text
}

func (p *parser) entityName() string {
	t := p.next()
	if !isEntityName(t.Kind) {
		p.fail(t, "an entity name")
	}
	return entityName(t)
}

// entity reads what follows an entity's name: an alias, classes, and then
// attributes or a relationship.
func (p *parser) entity(name string) []string {
	var alias string
	var classes []string
	if p.accept(tSQS) {
		alias = p.entityName()
		p.want(tSQE, "']'")
	}
	if p.accept(tStyleSep) {
		classes = p.idList()
	}
	if alias == "" && p.peek().Kind == tCard {
		spec := p.relSpec()
		other := p.entityName()
		var otherClasses []string
		if p.accept(tStyleSep) {
			otherClasses = p.idList()
		}
		p.want(tColon, "':'")
		role := p.role()
		p.db.addEntity(name, "")
		p.db.addEntity(other, "")
		p.db.addRelationship(name, role, other, spec)
		p.db.setClass([]string{name}, classes)
		p.db.setClass([]string{other}, otherClasses)
		return []string{name, other}
	}
	var attrs []attribute
	if p.accept(tBlockStart) {
		for !p.accept(tBlockStop) {
			attrs = append(attrs, p.attribute())
		}
	}
	p.db.addEntity(name, alias)
	p.db.addAttributes(name, attrs)
	p.db.setClass([]string{name}, classes)
	return []string{name}
}

// relSpec reads `card rel card`.
func (p *parser) relSpec() relSpec {
	b := p.want(tCard, "a cardinality").Text
	rel := p.want(tRel, "a relationship").Text
	a := p.want(tCard, "a cardinality").Text
	return relSpec{cardA: a, cardB: b, identifying: rel == "IDENTIFYING"}
}

func (p *parser) role() string {
	t := p.next()
	switch t.Kind {
	case tWord, tEntityName:
		return strings.ReplaceAll(t.Text, `"`, "")
	case tUnicodeText:
		return t.Text
	}
	p.fail(t, "a label")
	return ""
}

// attribute reads `type[?] name [KEY, ...] ["comment"]`.
func (p *parser) attribute() attribute {
	a := attribute{Type: p.want(tAttrWord, "an attribute type").Text, Keys: []string{}}
	if p.acceptChar("?") {
		a.Type += "?"
	}
	a.Name = p.want(tAttrWord, "an attribute name").Text
	if p.peek().Kind == tAttrKey {
		a.Keys = append(a.Keys, p.next().Text)
		for p.acceptChar(",") {
			a.Keys = append(a.Keys, p.want(tAttrKey, "PK, FK or UK").Text)
		}
	}
	if p.peek().Kind == tComment {
		a.Comment = strings.ReplaceAll(p.next().Text, `"`, "")
	}
	return a
}

// subgraph reads `subgraph id [title words]` and its statements up to end.
func (p *parser) subgraph() string {
	id := p.entityName()
	title := id
	if p.accept(tSQS) {
		words := []string{p.entityName()}
		for !p.accept(tSQE) {
			words = append(words, p.entityName())
		}
		title = strings.Join(words, " ")
	}
	p.separator()
	members, dir := p.document(true)
	return p.db.addSubgraph(id, title, members, dir)
}

func (p *parser) separator() {
	if t := p.next(); t.Kind != tSemi && t.Kind != tNewline && t.Kind != tEOF {
		p.fail(t, "a newline")
	}
}

func (p *parser) idList() []string {
	var ids []string
	for {
		t := p.next()
		if t.Kind != tUnicodeText && t.Kind != tStyleText {
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
			k := p.peek().Kind
			if k != tStyleText && k != tNum && k != tColon && k != tBrkt {
				break
			}
			s.WriteString(p.next().Text)
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

package flowchart

import (
	"fmt"
	"strconv"
	"strings"
)

// The parser is flow.jison's grammar, by recursive descent over the lexer's
// tokens; it calls the db's actions in the order jison reduces them.

type parser struct {
	toks []token
	i    int
	db   *db
}

// label is a FlowText: text and how it was written ("text", "string",
// "markdown").
type label struct{ text, typ string }

// item is what a statement puts in its document, for subgraphs: node ids,
// a subgraph's id, or a direction.
type item struct {
	id  string
	dir string
}

func (p *parser) peek() token        { return p.toks[p.i] }
func (p *parser) peekAt(n int) token { return p.toks[min(p.i+n, len(p.toks)-1)] }
func (p *parser) is(k kind) bool     { return p.toks[p.i].Kind == k }

func (p *parser) next() token {
	t := p.toks[p.i]
	if t.Kind != tEOF {
		p.i++
	}
	return t
}

func (p *parser) accept(k kind) bool {
	if p.is(k) {
		p.next()
		return true
	}
	return false
}

func (p *parser) expect(k kind, what string) (token, error) {
	if p.is(k) {
		return p.next(), nil
	}
	return token{}, p.errorf("expected %s, got %s", what, p.peek())
}

func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s", p.peek().Line, fmt.Sprintf(format, args...))
}

func (p *parser) spaces() bool {
	n := 0
	for p.accept(tSpace) {
		n++
	}
	return n > 0
}

// parse reads graphConfig document.
func (p *parser) parse() error {
	for p.is(tSpace) || p.is(tNewline) {
		p.next()
	}
	if _, err := p.expect(tGraph, "flowchart or graph"); err != nil {
		return err
	}
	switch {
	case p.accept(tNoDir):
		p.db.setDirection("TB")
	case p.is(tDir):
		p.db.setDirection(p.next().Text)
		// FirstStmtSeparator: SEMI | NEWLINE | spaceList NEWLINE.
		if !p.accept(tSemi) {
			p.spaces()
			if _, err := p.expect(tNewline, "newline"); err != nil {
				return err
			}
		}
	default:
		return p.errorf("expected a direction or newline, got %s", p.peek())
	}
	_, err := p.document(false)
	if err == nil && !p.is(tEOF) {
		err = p.errorf("unexpected %s", p.peek())
	}
	return err
}

// document reads lines up to the end of input or, in a subgraph, its end.
func (p *parser) document(sub bool) ([]item, error) {
	var items []item
	for {
		switch p.peek().Kind {
		case tEOF:
			if sub {
				return nil, p.errorf("expected end")
			}
			return items, nil
		case tEnd:
			if sub {
				return items, nil
			}
			return nil, p.errorf("unexpected end")
		case tSemi, tNewline, tSpace:
			p.next()
			continue
		}
		got, err := p.statement()
		if err != nil {
			return nil, err
		}
		items = append(items, got...)
	}
}

func (p *parser) statement() ([]item, error) {
	t := p.peek()
	switch t.Kind {
	case tSubgraph:
		return p.subgraph()
	case tDirection:
		p.next()
		return []item{{dir: t.Text}}, nil
	case tAccTitle, tAccDescr:
		p.next()
		p.accept(tAccTitleValue)
		p.accept(tAccDescrValue)
		return nil, nil
	case tAccDescrMultiValue:
		p.next()
		return nil, nil
	}
	var items []item
	var err error
	switch t.Kind {
	case tStyle:
		err = p.styleStatement()
	case tLinkStyle:
		err = p.linkStyleStatement()
	case tClassDef:
		err = p.classDefStatement()
	case tClass:
		err = p.classStatement()
	case tClick:
		// Interactions are not drawn: the statement is read and dropped,
		// but for the class mermaid gives the nodes.
		ids := p.next().Text
		for !p.is(tNewline) && !p.is(tSemi) && !p.is(tEOF) {
			p.next()
		}
		p.db.setClass(ids, "clickable")
	default:
		var ids []string
		ids, err = p.vertexStatement()
		for _, id := range ids {
			items = append(items, item{id: id})
		}
	}
	if err != nil {
		return nil, err
	}
	// separator: NEWLINE | SEMI | EOF. White space left before it (after
	// a class statement, say) fails in mermaid; we skip it.
	p.spaces()
	if !p.accept(tNewline) && !p.accept(tSemi) && !p.is(tEOF) {
		return nil, p.errorf("unexpected %s", p.peek())
	}
	return items, nil
}

// vertexStatement reads nodes joined by links; it returns the node ids,
// the last linked first, as jison's vertexStatement.nodes.
func (p *parser) vertexStatement() ([]string, error) {
	prev, err := p.node()
	if err != nil {
		return nil, err
	}
	nodes := prev
	for {
		if p.is(tShapeData) {
			p.db.addVertexData(prev[len(prev)-1], p.shapeData())
		}
		p.spaces()
		if !p.is(tLink) && !p.is(tStartLink) && !p.is(tLinkID) {
			return nodes, nil
		}
		l, err := p.link()
		if err != nil {
			return nil, err
		}
		next, err := p.node()
		if err != nil {
			return nil, err
		}
		if p.is(tShapeData) {
			p.db.addVertexData(next[len(next)-1], p.shapeData())
		}
		p.db.addLink(prev, next, l)
		nodes = append(append([]string{}, next...), nodes...)
		prev = next
	}
}

// node reads styledVertex (spaceList '&' spaceList styledVertex)*; a
// vertex's shape data may come before the '&'.
func (p *parser) node() ([]string, error) {
	var ids []string
	for {
		id, err := p.styledVertex()
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
		j := 0
		if p.peekAt(j).Kind == tShapeData {
			for p.peekAt(j).Kind == tShapeData {
				j++
			}
		}
		k := j
		for p.peekAt(k).Kind == tSpace {
			k++
		}
		if k == j || p.peekAt(k).Kind != tAmp || p.peekAt(k+1).Kind != tSpace {
			return ids, nil
		}
		if j > 0 {
			p.db.addVertexData(id, p.shapeData())
		}
		p.spaces()
		p.next() // &
		p.spaces()
	}
}

func (p *parser) shapeData() string {
	var b strings.Builder
	for p.is(tShapeData) {
		b.WriteString(p.next().Text)
	}
	return b.String()
}

func (p *parser) styledVertex() (string, error) {
	id, err := p.vertex()
	if err != nil {
		return "", err
	}
	if p.accept(tStyleSep) {
		class, err := p.idString()
		if err != nil {
			return "", err
		}
		p.db.setClass(id, class)
	}
	return id, nil
}

// shapes are the vertex shapes by their opening and closing tokens.
var shapes = []struct {
	open, close []kind
	typ         string
}{
	{[]kind{tSQS}, []kind{tSQE}, "square"},
	{[]kind{tDoubleCircleStart}, []kind{tDoubleCircleEnd}, "doublecircle"},
	{[]kind{tPS, tPS}, []kind{tPE, tPE}, "circle"},
	{[]kind{tEllipseStart}, []kind{tEllipseEnd}, "ellipse"},
	{[]kind{tStadiumStart}, []kind{tStadiumEnd}, "stadium"},
	{[]kind{tSubroutineStart}, []kind{tSubroutineEnd}, "subroutine"},
	{[]kind{tCylinderStart}, []kind{tCylinderEnd}, "cylinder"},
	{[]kind{tPS}, []kind{tPE}, "round"},
	{[]kind{tDiamondStart, tDiamondStart}, []kind{tDiamondStop, tDiamondStop}, "hexagon"},
	{[]kind{tDiamondStart}, []kind{tDiamondStop}, "diamond"},
	{[]kind{tTagEnd}, []kind{tSQE}, "odd"},
	{[]kind{tTrapStart}, []kind{tTrapEnd}, "trapezoid"},
	{[]kind{tInvTrapStart}, []kind{tInvTrapEnd}, "inv_trapezoid"},
	{[]kind{tTrapStart}, []kind{tInvTrapEnd}, "lean_right"},
	{[]kind{tInvTrapStart}, []kind{tTrapEnd}, "lean_left"},
}

// vertex reads idString and the shape around its text, if any.
func (p *parser) vertex() (string, error) {
	id, err := p.idString()
	if err != nil {
		return "", err
	}
	if p.accept(tVertexWithPropsStart) {
		// idString [| field : value | text ]
		if _, err := p.expect(tNodeString, "field"); err != nil {
			return "", err
		}
		if _, err := p.expect(tColon, "':'"); err != nil {
			return "", err
		}
		if _, err := p.expect(tNodeString, "value"); err != nil {
			return "", err
		}
		if _, err := p.expect(tPipe, "'|'"); err != nil {
			return "", err
		}
		t, err := p.text()
		if err != nil {
			return "", err
		}
		if _, err := p.expect(tSQE, "']'"); err != nil {
			return "", err
		}
		p.db.addVertex(id, &t, "rect")
		return id, nil
	}
	opened := false
	for _, s := range shapes {
		if !p.has(s.open) {
			continue
		}
		opened = true
		start := p.i
		p.i += len(s.open)
		t, err := p.text()
		if err != nil {
			return "", err
		}
		// Trapezoids share their openings: try the other closing.
		if !p.has(s.close) {
			p.i = start
			continue
		}
		p.i += len(s.close)
		p.db.addVertex(id, &t, s.typ)
		return id, nil
	}
	if opened {
		return "", p.errorf("unclosed shape at %s", p.peek())
	}
	p.db.addVertex(id, nil, "")
	return id, nil
}

// has reports whether the next tokens are ks; hexagons' and circles' doubled
// brackets are told from single ones by the token after the first.
func (p *parser) has(ks []kind) bool {
	for j, k := range ks {
		if p.peekAt(j).Kind != k {
			return false
		}
	}
	return true
}

// idString joins idStringTokens.
func (p *parser) idString() (string, error) {
	var b strings.Builder
	for {
		switch p.peek().Kind {
		case tNum, tNodeString, tDown, tMinus, tDefault, tComma, tColon, tAmp, tBrkt, tMult, tUnicodeText:
			b.WriteString(p.next().Text)
			continue
		}
		break
	}
	if b.Len() == 0 {
		return "", p.errorf("expected a node id, got %s", p.peek())
	}
	return b.String(), nil
}

// text is a vertex's or link's text: a string, a markdown string or text
// tokens, then more text tokens.
func (p *parser) text() (label, error) {
	var l label
	switch t := p.peek(); t.Kind {
	case tStr:
		l = label{t.Text, "string"}
	case tMdStr:
		l = label{t.Text, "markdown"}
	case tText, tTagStart, tTagEnd, tUnicodeText:
		l = label{t.Text, "text"}
	default:
		return l, p.errorf("expected text, got %s", t)
	}
	p.next()
	for p.is(tText) || p.is(tTagStart) || p.is(tTagEnd) || p.is(tUnicodeText) {
		l.text += p.next().Text
	}
	return l, nil
}

// link reads an arrow with its optional text and id.
func (p *parser) link() (*linkInfo, error) {
	id := ""
	if p.is(tLinkID) {
		id = strings.Replace(p.next().Text, "@", "", 1)
	}
	if p.is(tStartLink) {
		start := p.next().Text
		var t label
		switch tk := p.peek(); tk.Kind {
		case tStr:
			t = label{tk.Text, "string"}
		case tMdStr:
			t = label{tk.Text, "markdown"}
		case tEdgeText:
			t = label{tk.Text, "text"}
		default:
			return nil, p.errorf("expected link text, got %s", tk)
		}
		p.next()
		for p.is(tEdgeText) {
			t.text += p.next().Text
		}
		end, err := p.expect(tLink, "the rest of the link")
		if err != nil {
			return nil, err
		}
		l := destructLink(end.Text, start)
		l.text, l.id = &t, id
		return l, nil
	}
	tk, err := p.expect(tLink, "a link")
	if err != nil {
		return nil, err
	}
	l := destructLink(tk.Text, "")
	l.id = id
	if p.accept(tPipe) {
		t, err := p.text()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tPipe, "'|'"); err != nil {
			return nil, err
		}
		l.text = &t
		p.accept(tSpace)
	}
	return l, nil
}

// textNoTags is a subgraph's id or title.
func (p *parser) textNoTags() (label, error) {
	var l label
	switch t := p.peek(); {
	case t.Kind == tStr:
		l = label{t.Text, "text"}
	case t.Kind == tMdStr:
		l = label{t.Text, "markdown"}
	case noTagsToken(t.Kind):
		l = label{t.Text, "text"}
	default:
		return l, p.errorf("expected a subgraph title, got %s", t)
	}
	p.next()
	for noTagsToken(p.peek().Kind) {
		l.text += p.next().Text
	}
	return l, nil
}

func noTagsToken(k kind) bool {
	switch k {
	case tNum, tNodeString, tSpace, tMinus, tAmp, tUnicodeText, tColon, tMult, tBrkt, tStartLink,
		tStyle, tLinkStyle, tClassDef, tClass, tClick, tGraph, tDir, tSubgraph, tEnd, tDown, tUp:
		return true
	}
	return false
}

func (p *parser) subgraph() ([]item, error) {
	p.next()
	var id, title *label
	if p.accept(tSpace) {
		t, err := p.textNoTags()
		if err != nil {
			return nil, err
		}
		id, title = &t, &t
		if p.accept(tSQS) {
			tt, err := p.text()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tSQE, "']'"); err != nil {
				return nil, err
			}
			title = &tt
		}
	}
	p.spaces() // after a [title], mermaid fails on them
	if !p.accept(tNewline) && !p.accept(tSemi) && !p.is(tEOF) {
		return nil, p.errorf("unexpected %s", p.peek())
	}
	doc, err := p.document(true)
	if err != nil {
		return nil, err
	}
	p.next() // end
	if id == nil {
		// mermaid's addSubGraph reads the missing id's text and throws.
		return nil, p.errorf("subgraph without an id")
	}
	return []item{{id: p.db.addSubGraph(id, title, doc)}}, nil
}

// styles reads stylesOpt: styles of NUM, NODE_STRING, COLON, SPACE, BRKT
// and STYLE tokens, separated by commas.
func (p *parser) styles() ([]string, error) {
	var out []string
	for {
		var b strings.Builder
		for {
			switch p.peek().Kind {
			case tNum, tNodeString, tColon, tSpace, tBrkt, tStyle:
				b.WriteString(p.next().Text)
				continue
			}
			break
		}
		if b.Len() == 0 {
			return nil, p.errorf("expected a style, got %s", p.peek())
		}
		out = append(out, b.String())
		if !p.accept(tComma) {
			return out, nil
		}
	}
}

func (p *parser) styleStatement() error {
	p.next()
	if _, err := p.expect(tSpace, "space"); err != nil {
		return err
	}
	id, err := p.idString()
	if err != nil {
		return err
	}
	if _, err := p.expect(tSpace, "space"); err != nil {
		return err
	}
	st, err := p.styles()
	if err != nil {
		return err
	}
	p.db.addVertexStyles(id, st)
	return nil
}

func (p *parser) classDefStatement() error {
	p.next()
	if _, err := p.expect(tSpace, "space"); err != nil {
		return err
	}
	ids, err := p.idString()
	if err != nil {
		return err
	}
	if _, err := p.expect(tSpace, "space"); err != nil {
		return err
	}
	st, err := p.styles()
	if err != nil {
		return err
	}
	p.db.addClass(ids, st)
	return nil
}

func (p *parser) classStatement() error {
	p.next()
	if _, err := p.expect(tSpace, "space"); err != nil {
		return err
	}
	ids, err := p.idString()
	if err != nil {
		return err
	}
	if _, err := p.expect(tSpace, "space"); err != nil {
		return err
	}
	class, err := p.idString()
	if err != nil {
		return err
	}
	p.db.setClass(ids, class)
	return nil
}

// linkStyleStatement: linkStyle (default | n,n...) [interpolate curve] [styles].
func (p *parser) linkStyleStatement() error {
	p.next()
	if _, err := p.expect(tSpace, "space"); err != nil {
		return err
	}
	var pos []int // -1 is default
	if p.accept(tDefault) {
		pos = []int{-1}
	} else {
		for {
			t, err := p.expect(tNum, "a link number")
			if err != nil {
				return err
			}
			n, _ := strconv.Atoi(t.Text)
			pos = append(pos, n)
			if !p.accept(tComma) {
				break
			}
		}
	}
	if _, err := p.expect(tSpace, "space"); err != nil {
		return err
	}
	if p.accept(tInterpolate) {
		if _, err := p.expect(tSpace, "space"); err != nil {
			return err
		}
		var b strings.Builder
		for {
			switch p.peek().Kind {
			case tNum, tUnicodeText, tNodeString, tDir, tDown, tMinus, tComma, tColon, tAmp, tBrkt, tMult:
				b.WriteString(p.next().Text)
				continue
			}
			break
		}
		if b.Len() == 0 {
			return p.errorf("expected a curve, got %s", p.peek())
		}
		p.db.updateLinkInterpolate(pos, b.String())
		if !p.accept(tSpace) || p.is(tNewline) || p.is(tSemi) || p.is(tEOF) {
			return nil
		}
	}
	st, err := p.styles()
	if err != nil {
		return err
	}
	return p.db.updateLink(pos, st)
}

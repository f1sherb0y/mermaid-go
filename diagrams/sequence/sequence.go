// Package sequence is mermaid's sequence diagram (diagrams/sequence):
// participants in a row with their lifelines, messages between them from
// top to bottom, notes, activations, and loop/alt/opt/par/critical/break
// and rect blocks around them.
//
// Parse reads the source (sequenceDiagram.jison and sequenceDb.ts); Render
// lays it out and draws it as sequenceRenderer.ts and svgDraw.js do. Links,
// menus, properties and accessibility statements are parsed and dropped.
package sequence

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
	"github.com/f1sherb0y/mermaid-go/style"
)

// Kind is a message's kind: a signal's line type, or a note, an
// activation, a block's start/section/end, an autonumber setting
// (mermaid's LINETYPE).
type Kind int

const (
	Solid Kind = iota
	Dotted
	Note
	SolidCross
	DottedCross
	SolidOpen
	DottedOpen
	LoopStart
	LoopEnd
	AltStart
	AltElse
	AltEnd
	OptStart
	OptEnd
	ActiveStart
	ActiveEnd
	ParStart
	ParAnd
	ParEnd
	RectStart
	RectEnd
	SolidPoint
	DottedPoint
	Autonumber
	CriticalStart
	CriticalOption
	CriticalEnd
	BreakStart
	BreakEnd
	ParOverStart
	BidirectionalSolid
	BidirectionalDotted
	// Half arrows: a filled or stick head on the top or bottom side of the
	// line, at its end, or at its start (Reverse).
	SolidTop
	SolidBottom
	StickTop
	StickBottom
	SolidTopReverse
	SolidBottomReverse
	StickTopReverse
	StickBottomReverse
	SolidTopDotted
	SolidBottomDotted
	StickTopDotted
	StickBottomDotted
	SolidTopReverseDotted
	SolidBottomReverseDotted
	StickTopReverseDotted
	StickBottomReverseDotted
	// A dot on the lifeline where a message ends (()), starts, or both.
	CentralConnection
	CentralConnectionReverse
	CentralConnectionDual
)

// IsSignal reports a message drawn as a line between lifelines.
func (k Kind) IsSignal() bool {
	switch k {
	case Solid, Dotted, SolidCross, DottedCross, SolidOpen, DottedOpen, SolidPoint, DottedPoint,
		BidirectionalSolid, BidirectionalDotted:
		return true
	}
	return k >= SolidTop && k <= StickBottomReverseDotted
}

// Placement is where a note is.
type Placement int

const (
	NoPlacement Placement = iota
	LeftOf
	RightOf
	Over
)

// Participant types.
const (
	TypeParticipant = "participant"
	TypeActor       = "actor"
	TypeBoundary    = "boundary"
	TypeControl     = "control"
	TypeEntity      = "entity"
	TypeDatabase    = "database"
	TypeCollections = "collections"
	TypeQueue       = "queue"
)

// Diagram is a parsed sequence diagram.
type Diagram struct {
	Title string
	// Actors are the participants in order of first appearance.
	Actors []*Actor
	Boxes  []*Box
	// Messages are the statements that make the drawing, in order.
	Messages []*Message

	byID                   map[string]*Actor
	createdAt, destroyedAt map[string]int
	prev                   string
	box                    *Box
	created, destroyd      string // a create/destroy waiting for its message
	autoWrap               bool
}

// Actor is a participant.
type Actor struct {
	ID          string
	Description string
	Type        string
	Wrap        bool
	Box         *Box
	// Prev and Next are the ids of the actors declared before and after
	// it (mermaid's prevActor/nextActor).
	Prev, Next string
	// Created and Destroyed are the index of the message that creates or
	// destroys it, -1 for none.
	Created, Destroyed int
}

// Box groups participants under a title and fill.
type Box struct {
	Name   string
	Fill   string
	Wrap   bool
	Actors []string
}

// Message is a statement of the drawing.
type Message struct {
	Kind     Kind
	From, To string
	Text     string
	Wrap     bool
	// Placement is a note's.
	Placement Placement
	// Activate is set on a message that activates its receiver (+, ()).
	Activate bool
	// Central is the message's dots: CentralConnection, ...Reverse,
	// ...Dual, or 0.
	Central Kind
	// Start, Step and Visible are an autonumber statement's.
	Start, Step float64
	Visible     bool
}

// Actor returns the actor with an id, or nil.
func (d *Diagram) Actor(id string) *Actor { return d.byID[id] }

// Parse reads a sequence diagram. autoWrap is the wrap setting labels
// take when they do not say wrap: or nowrap:.
func Parse(code string, autoWrap bool) (*Diagram, error) {
	toks, err := lex(common.EncodeEntities(code))
	if err != nil {
		return nil, err
	}
	d := &Diagram{byID: map[string]*Actor{}, createdAt: map[string]int{}, destroyedAt: map[string]int{}, autoWrap: autoWrap}
	p := &parser{toks: toks, d: d}
	if err := p.start(); err != nil {
		return nil, err
	}
	for _, a := range d.Actors {
		if i, ok := d.createdAt[a.ID]; ok {
			a.Created = i
		}
		if i, ok := d.destroyedAt[a.ID]; ok {
			a.Destroyed = i
		}
	}
	return d, nil
}

type parser struct {
	toks []token
	i    int
	d    *Diagram
}

func (p *parser) peek() token { return p.toks[p.i] }

func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

func (p *parser) is(k kind, text string) bool {
	t := p.peek()
	return t.kind == k && (text == "" || t.text == text)
}

func (p *parser) expect(k kind, text string) (token, error) {
	if !p.is(k, text) {
		return token{}, p.unexpected()
	}
	return p.next(), nil
}

func (p *parser) unexpected() error {
	t := p.peek()
	what := strconv.Quote(t.text)
	switch t.kind {
	case tEOF:
		what = "end of input"
	case tNewline:
		what = "newline"
	}
	return fmt.Errorf("sequence: parse error on line %d: unexpected %s", t.line, what)
}

func (p *parser) start() error {
	for p.is(tNewline, "") {
		p.next()
	}
	if _, err := p.expect(tKeyword, "sequencediagram"); err != nil {
		return err
	}
	if err := p.document(); err != nil {
		return err
	}
	if !p.is(tEOF, "") {
		return p.unexpected()
	}
	return nil
}

// document reads lines up to a token that ends it (end, else, and,
// option or the end of input), which it leaves.
func (p *parser) document() error {
	for {
		t := p.peek()
		switch {
		case t.kind == tEOF:
			return nil
		case t.kind == tNewline, t.kind == tInvalid:
			p.next()
		case t.kind == tKeyword && (t.text == "end" || t.text == "else" || t.text == "and" || t.text == "option"):
			return nil
		default:
			if err := p.statement(); err != nil {
				return err
			}
		}
	}
}

func (p *parser) statement() error {
	t := p.peek()
	switch t.kind {
	case tActor:
		return p.signal()
	case tTitle:
		p.next()
		p.d.Title = strings.TrimSpace(common.SanitizeText(decode(t.text)))
		return nil
	case tAcc:
		p.next()
		if p.is(tAcc, "") {
			p.next()
		}
		return nil
	case tKeyword:
	default:
		return p.unexpected()
	}
	switch t.text {
	case "participant", "actor", "destroy":
		return p.participant(false)
	case "create":
		p.next()
		if !p.declares() {
			return p.unexpected()
		}
		return p.participant(true)
	case "box":
		p.next()
		rest, err := p.expect(tRest, "")
		if err != nil {
			return err
		}
		p.d.addBox(rest.text)
		for p.is(tNewline, "") || p.declares() {
			if p.is(tNewline, "") {
				p.next()
			} else if err := p.participant(false); err != nil {
				return err
			}
		}
		if _, err := p.expect(tKeyword, "end"); err != nil {
			return err
		}
		p.d.box = nil
		return nil
	case "autonumber":
		p.next()
		m := &Message{Kind: Autonumber, Visible: true}
		switch {
		case p.is(tNum, ""):
			m.Start, _ = strconv.ParseFloat(p.next().text, 64)
			m.Step = 1
			if p.is(tNum, "") {
				m.Step, _ = strconv.ParseFloat(p.next().text, 64)
			}
		case p.is(tKeyword, "off"):
			p.next()
			m.Visible = false
		}
		if _, err := p.expect(tNewline, ""); err != nil {
			return err
		}
		p.d.Messages = append(p.d.Messages, m)
		return nil
	case "activate", "deactivate":
		p.next()
		a, err := p.expect(tActor, "")
		if err != nil {
			return err
		}
		if _, err := p.expect(tNewline, ""); err != nil {
			return err
		}
		if err := p.d.addActor(a.text, nil, "", ""); err != nil {
			return err
		}
		k := ActiveStart
		if t.text == "deactivate" {
			k = ActiveEnd
		}
		return p.d.addSignal(&Message{Kind: k, From: a.text})
	case "note":
		return p.note()
	case "links", "link", "properties", "details":
		p.next()
		a, err := p.expect(tActor, "")
		if err != nil {
			return err
		}
		if _, err := p.expect(tText, ""); err != nil {
			return err
		}
		if _, err := p.expect(tNewline, ""); err != nil {
			return err
		}
		return p.d.addActor(a.text, nil, "", "")
	case "loop", "rect", "opt", "break", "alt", "par", "par_over", "critical":
		return p.block(t.text)
	}
	return p.unexpected()
}

// blocks are the block statements: their start, section and end kinds,
// and the keyword that starts a section.
var blocks = map[string]struct {
	start, section, end Kind
	word                string
}{
	"loop": {LoopStart, 0, LoopEnd, ""}, "rect": {RectStart, 0, RectEnd, ""},
	"opt": {OptStart, 0, OptEnd, ""}, "break": {BreakStart, 0, BreakEnd, ""},
	"alt": {AltStart, AltElse, AltEnd, "else"}, "par": {ParStart, ParAnd, ParEnd, "and"},
	"par_over": {ParOverStart, ParAnd, ParEnd, "and"}, "critical": {CriticalStart, CriticalOption, CriticalEnd, "option"},
}

// block reads loop/rect/opt/break/alt/par/par_over/critical, its sections
// (else, and, option) and end.
func (p *parser) block(kw string) error {
	b := blocks[kw]
	k := b.start
	for {
		p.next()
		rest, err := p.expect(tRest, "")
		if err != nil {
			return err
		}
		text, wrap := p.d.message(rest.text)
		p.d.Messages = append(p.d.Messages, &Message{Kind: k, Text: text, Wrap: wrap})
		if err := p.document(); err != nil {
			return err
		}
		if b.word == "" || !p.is(tKeyword, b.word) {
			break
		}
		k = b.section
	}
	if _, err := p.expect(tKeyword, "end"); err != nil {
		return err
	}
	p.d.Messages = append(p.d.Messages, &Message{Kind: b.end, Wrap: p.d.autoWrap})
	return nil
}

// declares reports a participant statement next.
func (p *parser) declares() bool {
	return p.is(tKeyword, "participant") || p.is(tKeyword, "actor") || p.is(tKeyword, "destroy")
}

// participant reads participant/actor/destroy statements.
func (p *parser) participant(create bool) error {
	kw := p.next().text
	a, err := p.expect(tActor, "")
	if err != nil {
		return err
	}
	meta := ""
	if p.is(tConfigOpen, "") {
		p.next()
		c, err := p.expect(tConfig, "")
		if err != nil {
			return err
		}
		meta = c.text
	}
	var desc *string
	if kw != "destroy" && p.is(tAs, "") {
		p.next()
		r, err := p.expect(tRest, "")
		if err != nil {
			return err
		}
		desc = &r.text
	}
	if _, err := p.expect(tNewline, ""); err != nil {
		return err
	}
	d := p.d
	typ := map[string]string{"participant": TypeParticipant, "actor": TypeActor}[kw]
	switch {
	case create:
		if d.byID[a.text] != nil {
			return fmt.Errorf("sequence: it is not possible to have actors with the same id, even if one is destroyed before the next is created (%s)", a.text)
		}
		d.created = a.text
		if err := d.addActor(a.text, desc, typ, meta); err != nil {
			return err
		}
		d.createdAt[a.text] = len(d.Messages)
		return nil
	case kw == "destroy":
		d.destroyd = a.text
		d.destroyedAt[a.text] = len(d.Messages)
		return nil
	}
	return d.addActor(a.text, desc, typ, meta)
}

func (p *parser) note() error {
	p.next()
	var placement Placement
	switch {
	case p.is(tLeftOf, ""):
		placement = LeftOf
	case p.is(tRightOf, ""):
		placement = RightOf
	case p.is(tKeyword, "over"):
		placement = Over
	default:
		return p.unexpected()
	}
	p.next()
	a, err := p.expect(tActor, "")
	if err != nil {
		return err
	}
	from, to := a.text, a.text
	if placement == Over && p.is(tComma, "") {
		p.next()
		b, err := p.expect(tActor, "")
		if err != nil {
			return err
		}
		to = b.text
	}
	txt, err := p.expect(tText, "")
	if err != nil {
		return err
	}
	if _, err := p.expect(tNewline, ""); err != nil {
		return err
	}
	p.d.addActor(from, nil, "", "")
	p.d.addActor(to, nil, "", "")
	text, wrap := p.d.message(strings.TrimSpace(txt.text)[1:])
	p.d.Messages = append(p.d.Messages, &Message{Kind: Note, From: from, To: to, Text: text, Wrap: wrap, Placement: placement})
	return nil
}

// signal reads a message: from, arrow, to (with +, - or the () dots) and
// its text.
func (p *parser) signal() error {
	from := p.next().text
	m := &Message{From: from}
	var after Kind // activeStart, activeEnd or central dots after the message
	if p.is(tCircle, "") {
		p.next()
		m.Central = CentralConnectionReverse
	}
	arrow, err := p.expect(tArrow, "")
	if err != nil {
		return err
	}
	m.Kind = arrow.arrow
	switch {
	case p.is(tCircle, ""):
		p.next()
		m.Activate = true
		if m.Central == CentralConnectionReverse {
			m.Central = CentralConnectionDual
		} else {
			m.Central = CentralConnection
		}
	case m.Central != 0:
	case p.is(tPlus, ""):
		p.next()
		m.Activate = true
		after = ActiveStart
	case p.is(tMinus, ""):
		p.next()
		after = ActiveEnd
	}
	to, err := p.expect(tActor, "")
	if err != nil {
		return err
	}
	m.To = to.text
	txt, err := p.expect(tText, "")
	if err != nil {
		return err
	}
	if _, err := p.expect(tNewline, ""); err != nil {
		return err
	}
	m.Text, m.Wrap = p.d.message(strings.TrimSpace(txt.text)[1:])
	d := p.d
	d.addActor(m.From, nil, "", "")
	d.addActor(m.To, nil, "", "")
	switch {
	case d.created != "":
		if m.To != d.created {
			return fmt.Errorf("sequence: the created participant %s does not have an associated creating message after its declaration", d.created)
		}
		d.created = ""
	case d.destroyd != "":
		if m.To != d.destroyd && m.From != d.destroyd {
			return fmt.Errorf("sequence: the destroyed participant %s does not have an associated destroying message after its declaration", d.destroyd)
		}
		d.destroyd = ""
	}
	if err := d.addSignal(m); err != nil {
		return err
	}
	switch {
	case after == ActiveStart:
		return d.addSignal(&Message{Kind: ActiveStart, From: m.To})
	case after == ActiveEnd:
		return d.addSignal(&Message{Kind: ActiveEnd, From: m.From})
	case m.Central == CentralConnection:
		return d.addSignal(&Message{Kind: CentralConnection, From: m.To})
	case m.Central == CentralConnectionReverse:
		return d.addSignal(&Message{Kind: CentralConnectionReverse, From: m.From})
	case m.Central == CentralConnectionDual:
		d.addSignal(&Message{Kind: CentralConnection, From: m.To})
		return d.addSignal(&Message{Kind: CentralConnectionReverse, From: m.From})
	}
	return nil
}

// addActor is the db's addActor: a participant is added the first time it
// is named, and a declaration (desc or typ given) replaces it in place.
func (d *Diagram) addActor(id string, desc *string, typ, meta string) error {
	box := d.box
	if meta != "" {
		var doc struct{ Type, Alias string }
		y := meta + "\n"
		if !strings.Contains(meta, "\n") {
			y = "{\n" + meta + "\n}"
		}
		if err := yaml.Unmarshal([]byte(y), &doc); err != nil {
			return fmt.Errorf("sequence: participant %s: %w", id, err)
		}
		if doc.Type != "" {
			typ = doc.Type
		}
		if doc.Alias != "" && (desc == nil || *desc == id) {
			desc = &doc.Alias
		}
	}
	old := d.byID[id]
	if old != nil {
		if d.box != nil && old.Box != nil && d.box != old.Box {
			return fmt.Errorf("sequence: a participant should only be defined in one box: %s can't be in '%s' and in '%s' at the same time", id, old.Box.Name, d.box.Name)
		}
		if old.Box != nil {
			box = old.Box
		}
		old.Box = box
		if desc == nil {
			return nil
		}
	}
	a := &Actor{ID: id, Description: id, Type: TypeParticipant, Wrap: d.autoWrap, Box: box, Prev: d.prev, Created: -1, Destroyed: -1}
	if desc != nil && typ != "" {
		a.Description, a.Wrap = d.message(*desc)
	}
	if typ != "" {
		a.Type = typ
	}
	if old != nil {
		*old = *a
		a = old
	} else {
		d.byID[id] = a
		d.Actors = append(d.Actors, a)
	}
	if prev := d.byID[d.prev]; prev != nil {
		prev.Next = id
	}
	if d.box != nil {
		d.box.Actors = append(d.box.Actors, id)
	}
	d.prev = id
	return nil
}

// addSignal adds a message; deactivating an inactive participant fails.
func (d *Diagram) addSignal(m *Message) error {
	if m.Kind == ActiveEnd {
		n := 0
		for _, o := range d.Messages {
			switch {
			case o.Kind == ActiveStart && o.From == m.From:
				n++
			case o.Kind == ActiveEnd && o.From == m.From:
				n--
			}
		}
		if n < 1 {
			return fmt.Errorf("sequence: trying to inactivate an inactive participant (%s)", m.From)
		}
	}
	d.Messages = append(d.Messages, m)
	return nil
}

var wrapPrefix = regexp.MustCompile(`^:?(no)?wrap:`)

// message is the db's parseMessage: the text trimmed, a wrap: or nowrap:
// prefix taken off and turned into the wrap setting.
func (d *Diagram) message(s string) (string, bool) {
	s = strings.TrimSpace(decode(s))
	wrap := d.autoWrap
	if m := wrapPrefix.FindStringSubmatch(s); m != nil {
		wrap = m[1] == ""
		s = strings.TrimSpace(s[len(m[0]):])
	}
	return s, wrap
}

var boxData = regexp.MustCompile(`^((?:rgba?|hsla?)\s*\(.*\)|\w*)(.*)$`)

// addBox is the db's parseBoxData and addBox: a color first (a name,
// rgb()/hsl(); not #hex, which the lexer takes for a comment), then the
// title.
func (d *Diagram) addBox(s string) {
	m := boxData.FindStringSubmatch(s)
	fill, title := "transparent", strings.TrimSpace(m[2])
	if c := strings.TrimSpace(m[1]); c != "" {
		fill = c
	}
	if !isColor(fill) {
		fill, title = "transparent", strings.TrimSpace(s)
	}
	b := &Box{Fill: fill}
	b.Name, b.Wrap = d.message(title)
	b.Name = common.SanitizeText(b.Name)
	d.Boxes = append(d.Boxes, b)
	d.box = b
}

func isColor(s string) bool {
	return strings.EqualFold(s, "transparent") || !style.Color(s).IsNone()
}

// placeholder is an entity common.EncodeEntities took out: #9829; or
// #infin; in the source.
var placeholder = regexp.MustCompile(`ﬂ°(°?)(\w+)¶ß`)

// decode turns entity placeholders back into their characters.
func decode(s string) string {
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		p := placeholder.FindStringSubmatch(m)
		e := "&" + p[2] + ";"
		if p[1] != "" {
			e = "&#" + p[2] + ";"
		}
		return html.UnescapeString(e)
	})
}

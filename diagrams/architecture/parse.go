package architecture

import (
	"fmt"
	"regexp"
	"strings"
)

// Diagram is a parsed architecture diagram, checked as mermaid's db checks
// it.
type Diagram struct {
	Title, AccTitle, AccDescr string
	Groups                    []*Group
	// Nodes are the services, then the junctions, each in declaration order
	// (mermaid's order).
	Nodes []*Node
	Edges []*Edge
	Hints []*Hint
}

// Group is a group of services, junctions and groups.
type Group struct {
	ID, Icon, Title string
	Parent          *Group
}

// Node is a service or a junction.
type Node struct {
	ID       string
	Junction bool
	// Icon is an icon's name ("database", "logos:aws-s3"); IconText the text
	// drawn on a blank icon instead. Neither: a dashed outline.
	Icon, IconText, Title string
	Parent                *Group
}

// Side is the side of a node an edge leaves from: 'L', 'R', 'T' or 'B'.
type Side byte

// Edge joins two nodes at given sides.
type Edge struct {
	From, To         *Node
	FromSide, ToSide Side
	// FromArrow and ToArrow put an arrowhead at that end.
	FromArrow, ToArrow bool
	// FromGroup and ToGroup ({group}) start that end at the border of the
	// node's group.
	FromGroup, ToGroup bool
	Label              string
}

// Hint is an `align row|column` directive: its members share a row (left to
// right) or a column (top to bottom).
type Hint struct {
	Column  bool
	Members []*Node
}

// The statements (architecture.langium), one per line.
const (
	id    = `[A-Za-z0-9_](?:[-A-Za-z0-9_]*[A-Za-z0-9_])?`
	icon  = `\(([\w:-]+)\)`
	title = `(\[(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|[^\[\]\r\n]+)\])`
	str   = `"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)'`
	in    = `(?:\s*\bin\s+(` + id + `))?`
)

var (
	groupRe    = regexp.MustCompile(`^group\s+(` + id + `)\s*(?:` + icon + `)?\s*` + title + `?` + in + `$`)
	serviceRe  = regexp.MustCompile(`^service\s+(` + id + `)\s*(?:` + str + `|` + icon + `)?\s*` + title + `?` + in + `$`)
	junctionRe = regexp.MustCompile(`^junction\s+(` + id + `)` + in + `$`)
	alignRe    = regexp.MustCompile(`^align\s+(row|column)((?:\s+` + id + `){2,})$`)
	edgeRe     = regexp.MustCompile(`^(` + id + `)\s*(\{group\})?\s*:\s*([LRTB])\s*(<)?\s*(?:--|-\s*` + title + `\s*-)\s*(>)?\s*([LRTB])\s*:\s*(` + id + `)\s*(\{group\})?$`)
	titleRe    = regexp.MustCompile(`^title(?:[\t ]+(.*))?$`)
	accTitleRe = regexp.MustCompile(`^accTitle\s*:(.*)$`)
	accDescrRe = regexp.MustCompile(`^accDescr\s*(?::(.*)|\{(.*))$`)
	headerRe   = regexp.MustCompile(`^architecture-beta(?:[\t ]+|$)`)
)

// keywords can't be ids (mermaid's lexer makes them keywords).
var keywords = map[string]bool{"group": true, "service": true, "junction": true, "in": true, "align": true, "row": true, "column": true}

type statement struct {
	line int
	m    []string
}

// Parse reads an architecture diagram. Ids that mermaid's lexer turns into
// a direction (any id starting with L, R, T or B, as "Lambda") are read as
// ids: mermaid fails on them, which is a lexer accident.
func Parse(code string) (*Diagram, error) {
	d := &Diagram{}
	var groups, services, junctions, edges, aligns []statement
	lines := strings.Split(code, "\n")
	started := false
	for n := 0; n < len(lines); n++ {
		l := strings.TrimSpace(stripComment(lines[n]))
		if !started {
			if l == "" {
				continue
			}
			loc := headerRe.FindStringIndex(l)
			if loc == nil {
				return nil, fmt.Errorf("architecture: line %d: expected architecture-beta", n+1)
			}
			started = true
			l = strings.TrimSpace(l[loc[1]:])
		}
		if l == "" {
			continue
		}
		word, _, _ := strings.Cut(l, " ")
		var m []string
		switch {
		case titleRe.MatchString(l):
			d.Title = strings.TrimSpace(titleRe.FindStringSubmatch(l)[1])
			continue
		case accTitleRe.MatchString(l):
			d.AccTitle = strings.TrimSpace(accTitleRe.FindStringSubmatch(l)[1])
			continue
		case accDescrRe.MatchString(l):
			sm := accDescrRe.FindStringSubmatch(l)
			if !strings.HasPrefix(strings.TrimSpace(l[len("accDescr"):]), "{") {
				d.AccDescr = strings.TrimSpace(sm[1])
				continue
			}
			// A block, to the closing brace.
			body := sm[2]
			for !strings.Contains(body, "}") && n+1 < len(lines) {
				n++
				body += "\n" + lines[n]
			}
			text, rest, ok := strings.Cut(body, "}")
			if !ok || strings.TrimSpace(rest) != "" {
				return nil, fmt.Errorf("architecture: line %d: unclosed accDescr", n+1)
			}
			d.AccDescr = strings.TrimSpace(text)
			continue
		case word == "group" || strings.HasPrefix(l, "group\t"):
			m = groupRe.FindStringSubmatch(l)
			groups = append(groups, statement{n + 1, m})
		case word == "service" || strings.HasPrefix(l, "service\t"):
			m = serviceRe.FindStringSubmatch(l)
			services = append(services, statement{n + 1, m})
		case word == "junction" || strings.HasPrefix(l, "junction\t"):
			m = junctionRe.FindStringSubmatch(l)
			junctions = append(junctions, statement{n + 1, m})
		case word == "align" || strings.HasPrefix(l, "align\t"):
			m = alignRe.FindStringSubmatch(l)
			aligns = append(aligns, statement{n + 1, m})
		default:
			m = edgeRe.FindStringSubmatch(l)
			edges = append(edges, statement{n + 1, m})
		}
		if m == nil {
			return nil, fmt.Errorf("architecture: line %d: can't read %q", n+1, l)
		}
	}
	if !started {
		return nil, fmt.Errorf("architecture: empty")
	}
	if err := d.build(groups, services, junctions, edges, aligns); err != nil {
		return nil, err
	}
	return d, nil
}

// stripComment drops a %% comment, which can't start inside a [title] or a
// quoted string.
func stripComment(l string) string {
	var close byte
	for i := 0; i < len(l); i++ {
		c := l[i]
		switch {
		case close != 0:
			if c == '\\' && close != ']' {
				i++
			} else if c == close {
				close = 0
			}
		case c == '[':
			close = ']'
		case c == '"' || c == '\'':
			close = c
		case c == '%' && i+1 < len(l) && l[i+1] == '%':
			return l[:i]
		}
	}
	return l
}

// build adds the statements in mermaid's order (populateDb): groups, then
// services, junctions, edges and alignments, checking each as the db does.
func (d *Diagram) build(groups, services, junctions, edges, aligns []statement) error {
	kinds := map[string]string{} // id → "node" or "group"
	byGroup := map[string]*Group{}
	byNode := map[string]*Node{}
	check := func(line int, what, id, parent string) (*Group, error) {
		if keywords[id] {
			return nil, fmt.Errorf("architecture: line %d: %q is a keyword, not an id", line, id)
		}
		if k, ok := kinds[id]; ok {
			return nil, fmt.Errorf("architecture: line %d: the %s id [%s] is already in use by another %s", line, what, id, k)
		}
		if parent == "" {
			return nil, nil
		}
		switch {
		case id == parent:
			return nil, fmt.Errorf("architecture: line %d: the %s [%s] cannot be placed within itself", line, what, id)
		case kinds[parent] == "":
			return nil, fmt.Errorf("architecture: line %d: the %s [%s]'s parent does not exist; create the parent before this %s", line, what, id, what)
		case kinds[parent] == "node":
			return nil, fmt.Errorf("architecture: line %d: the %s [%s]'s parent is not a group", line, what, id)
		}
		return byGroup[parent], nil
	}
	for _, s := range groups {
		m := s.m
		parent, err := check(s.line, "group", m[1], m[4])
		if err != nil {
			return err
		}
		g := &Group{ID: m[1], Icon: m[2], Title: archTitle(m[3]), Parent: parent}
		kinds[g.ID], byGroup[g.ID] = "group", g
		d.Groups = append(d.Groups, g)
	}
	for _, s := range services {
		m := s.m
		parent, err := check(s.line, "service", m[1], m[6])
		if err != nil {
			return err
		}
		n := &Node{ID: m[1], IconText: unescape(m[2] + m[3]), Icon: m[4], Title: archTitle(m[5]), Parent: parent}
		kinds[n.ID], byNode[n.ID] = "node", n
		d.Nodes = append(d.Nodes, n)
	}
	for _, s := range junctions {
		m := s.m
		parent, err := check(s.line, "junction", m[1], m[2])
		if err != nil {
			return err
		}
		n := &Node{ID: m[1], Junction: true, Parent: parent}
		kinds[n.ID], byNode[n.ID] = "node", n
		d.Nodes = append(d.Nodes, n)
	}
	for _, s := range edges {
		m := s.m
		var ends [2]*Node
		for i, id := range []string{m[1], m[8]} {
			switch kinds[id] {
			case "":
				return fmt.Errorf("architecture: line %d: [%s] does not exist yet; create the service or junction before an edge to it", s.line, id)
			case "group":
				return fmt.Errorf("architecture: line %d: [%s] is a group; an edge joins services or junctions ({group} starts it at the group's border)", s.line, id)
			}
			ends[i] = byNode[id]
		}
		e := &Edge{From: ends[0], To: ends[1], FromSide: Side(m[3][0]), ToSide: Side(m[7][0]),
			FromArrow: m[4] != "", ToArrow: m[6] != "", FromGroup: m[2] != "", ToGroup: m[9] != "", Label: archTitle(m[5])}
		sameGroup := e.From.Parent != nil && e.From.Parent == e.To.Parent
		if sameGroup && (e.FromGroup || e.ToGroup) {
			return fmt.Errorf("architecture: line %d: {group} on an edge between [%s] and [%s], which are in the same group", s.line, e.From.ID, e.To.ID)
		}
		d.Edges = append(d.Edges, e)
	}
	for _, s := range aligns {
		h := &Hint{Column: s.m[1] == "column"}
		seen := map[string]bool{}
		for _, id := range strings.Fields(s.m[2]) {
			if kinds[id] != "node" {
				return fmt.Errorf("architecture: line %d: align %s references [%s], which is not a service or junction", s.line, s.m[1], id)
			}
			if seen[id] {
				return fmt.Errorf("architecture: line %d: align %s lists [%s] more than once", s.line, s.m[1], id)
			}
			seen[id] = true
			h.Members = append(h.Members, byNode[id])
		}
		d.Hints = append(d.Hints, h)
	}
	return nil
}

// archTitle is an ARCH_TITLE's value: the brackets and outer quotes taken
// off, escaped quotes unescaped.
func archTitle(s string) string {
	if s == "" {
		return ""
	}
	s = strings.TrimSpace(s[1 : len(s)-1])
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		s = unescape(s[1 : len(s)-1])
	}
	return strings.TrimSpace(s)
}

func unescape(s string) string {
	return strings.NewReplacer(`\"`, `"`, `\'`, `'`).Replace(s)
}

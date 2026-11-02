package gitgraph

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/common"
)

// Commit kinds.
const (
	Normal = iota
	Reverse
	Highlight
	Merge
	CherryPick
)

// Graph is a parsed gitGraph: its commits in the order they were made and
// its branches in drawing order.
type Graph struct {
	Title    string
	Dir      string // "LR", "TB" or "BT"
	Commits  []*Commit
	Branches []string
}

// Commit is a commit. Type is its kind; Symbol the kind it is drawn as (a
// merge can be drawn as another kind).
type Commit struct {
	ID       string
	Seq      int
	Type     int
	Symbol   int
	CustomID bool
	Tags     []string
	Parents  []string
	Branch   string
}

// db is mermaid's gitGraphAst state.
type db struct {
	g        *Graph
	byID     map[string]*Commit
	head     *Commit
	heads    map[string]string // branch → its last commit's id ("" none)
	order    []string          // branches in creation order
	orders   map[string]float64
	explicit map[string]bool
	cur      string
	seq      int
}

// id is a commit's generated id: its sequence number and seven hex digits
// (random in mermaid; here a hash of the number, so drawings repeat).
func (d *db) id() string {
	x := uint64(d.seq) + 0x9e3779b97f4a7c15 // splitmix64
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	x ^= x >> 31
	return fmt.Sprintf("%d-%07x", d.seq, x&0xfffffff)
}

func (d *db) add(c *Commit) {
	c.Seq, c.Branch = d.seq, d.cur
	d.seq++
	if d.head != nil {
		c.Parents = append([]string{d.head.ID}, c.Parents...)
	}
	if old, ok := d.byID[c.ID]; ok {
		*old = *c // a repeated id replaces the commit, in its place
		c = old
	} else {
		d.byID[c.ID] = c
		d.g.Commits = append(d.g.Commits, c)
	}
	d.head = c
	d.heads[d.cur] = c.ID
}

func (d *db) commit(id string, typ int, tags []string) {
	c := &Commit{ID: id, Type: typ, Symbol: typ, Tags: tags}
	if id == "" {
		c.ID = d.id()
	}
	d.add(c)
}

func (d *db) branch(name string, order *float64) error {
	if _, ok := d.heads[name]; ok {
		return fmt.Errorf(`trying to create an existing branch (help: either use a new name if you want create a new branch or try using "checkout %s")`, name)
	}
	d.heads[name] = ""
	if d.head != nil {
		d.heads[name] = d.head.ID
	}
	d.order = append(d.order, name)
	if order != nil {
		d.orders[name], d.explicit[name] = *order, true
	}
	return d.checkout(name)
}

func (d *db) checkout(name string) error {
	id, ok := d.heads[name]
	if !ok {
		return fmt.Errorf(`trying to checkout branch which is not yet created (help: try using "branch %s")`, name)
	}
	d.cur, d.head = name, d.byID[id]
	return nil
}

func (d *db) merge(other, id string, typ int, tags []string) error {
	cur := d.byID[d.heads[d.cur]]
	otherHead, exists := d.heads[other]
	oc := d.byID[otherHead]
	switch {
	case cur != nil && oc != nil && cur.Branch == other, d.cur == other:
		return fmt.Errorf("incorrect usage of merge: cannot merge branch %q into itself", other)
	case cur == nil:
		return fmt.Errorf("incorrect usage of merge: current branch (%s) has no commits", d.cur)
	case !exists:
		return fmt.Errorf("incorrect usage of merge: branch to be merged (%s) does not exist", other)
	case oc == nil:
		return fmt.Errorf("incorrect usage of merge: branch to be merged (%s) has no commits", other)
	case cur == oc:
		return fmt.Errorf("incorrect usage of merge: both branches have the same head")
	case id != "" && d.byID[id] != nil:
		return fmt.Errorf("incorrect usage of merge: commit with id %s already exists, use a different custom id", id)
	}
	c := &Commit{ID: id, Type: Merge, Symbol: Merge, CustomID: id != "", Tags: tags, Parents: []string{otherHead}}
	if typ >= 0 {
		c.Symbol = typ
	}
	if id == "" {
		c.ID = d.id()
	}
	d.add(c)
	return nil
}

func (d *db) cherryPick(src, parent string, tags []string, tagsSet bool) error {
	sc := d.byID[src]
	if sc == nil {
		return fmt.Errorf("incorrect usage of cherry-pick: source commit id should exist and be provided")
	}
	if parent != "" && !contains(sc.Parents, parent) {
		return fmt.Errorf("invalid operation: the specified parent commit is not an immediate parent of the cherry-picked commit")
	}
	if sc.Type == Merge && parent == "" {
		return fmt.Errorf("incorrect usage of cherry-pick: if the source commit is a merge commit, an immediate parent commit must be specified")
	}
	if sc.Branch == d.cur {
		return fmt.Errorf("incorrect usage of cherry-pick: source commit is already on current branch")
	}
	if d.heads[d.cur] == "" {
		return fmt.Errorf("incorrect usage of cherry-pick: current branch (%s) has no commits", d.cur)
	}
	if !tagsSet {
		tag := "cherry-pick:" + sc.ID
		if sc.Type == Merge {
			tag += "|parent:" + parent
		}
		tags = []string{tag}
	}
	d.add(&Commit{ID: d.id(), Type: CherryPick, Symbol: CherryPick, Tags: tags, Parents: []string{sc.ID}})
	return nil
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// branches are the branches in drawing order: by their order, the ones
// without one at 0.<their index> (mermaid's parseFloat("0." + i)).
func (d *db) branches() []string {
	type b struct {
		name  string
		order float64
	}
	var bs []b
	for i, n := range d.order {
		o := d.orders[n]
		if !d.explicit[n] {
			o, _ = strconv.ParseFloat("0."+strconv.Itoa(i), 64)
		}
		bs = append(bs, b{n, o})
	}
	sort.SliceStable(bs, func(i, j int) bool { return bs[i].order < bs[j].order })
	out := make([]string, len(bs))
	for i, x := range bs {
		out[i] = x.name
	}
	return out
}

// scanner reads a statement (gitGraph.langium's terminals).
type scanner struct {
	s    string
	line int
}

var (
	refRe    = regexp.MustCompile(`^\w(?:[-./\w]*[-\w])?`)
	stringRe = regexp.MustCompile(`^(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')`)
	intRe    = regexp.MustCompile(`^(?:0|[1-9][0-9]*)`)
)

func (sc *scanner) errorf(format string, args ...any) error {
	return fmt.Errorf("gitGraph: line %d: %s", sc.line, fmt.Sprintf(format, args...))
}

func (sc *scanner) skip() { sc.s = strings.TrimLeft(sc.s, " \t\r") }

func (sc *scanner) eol() bool { sc.skip(); return sc.s == "" }

// word takes a keyword, not followed by more of a word.
func (sc *scanner) word(kw string) bool {
	sc.skip()
	if !strings.HasPrefix(sc.s, kw) {
		return false
	}
	rest := sc.s[len(kw):]
	if rest != "" && (rest[0] == '_' || rest[0] == '-' || unicode.IsLetter(rune(rest[0])) || unicode.IsDigit(rune(rest[0]))) {
		return false
	}
	sc.s = rest
	return true
}

// key takes `name:` (spaces allowed before the colon).
func (sc *scanner) key(name string) bool {
	save := sc.s
	if sc.word(name) {
		sc.skip()
		if strings.HasPrefix(sc.s, ":") {
			sc.s = sc.s[1:]
			return true
		}
	}
	sc.s = save
	return false
}

func (sc *scanner) str() (string, bool) {
	sc.skip()
	m := stringRe.FindString(sc.s)
	if m == "" {
		return "", false
	}
	sc.s = sc.s[len(m):]
	return m[1 : len(m)-1], true
}

func (sc *scanner) mustStr() (string, error) {
	s, ok := sc.str()
	if !ok {
		return "", sc.errorf("expected a string")
	}
	return common.SanitizeText(s), nil
}

// ref is a branch name: a reference or a string.
func (sc *scanner) ref() (string, error) {
	if s, ok := sc.str(); ok {
		return common.SanitizeText(s), nil
	}
	m := refRe.FindString(sc.s)
	if m == "" {
		return "", sc.errorf("expected a branch name")
	}
	sc.s = sc.s[len(m):]
	return common.SanitizeText(m), nil
}

func (sc *scanner) commitType() (int, error) {
	for i, t := range []string{"NORMAL", "REVERSE", "HIGHLIGHT"} {
		if sc.word(t) {
			return i, nil
		}
	}
	return 0, sc.errorf("expected NORMAL, REVERSE or HIGHLIGHT")
}

// Parse reads a gitGraph (gitGraph.langium) and plays its statements.
func Parse(code string, mainBranch string, mainOrder float64) (*Graph, error) {
	g := &Graph{Dir: "LR"}
	d := &db{g: g, byID: map[string]*Commit{}, heads: map[string]string{mainBranch: ""}, order: []string{mainBranch},
		orders: map[string]float64{mainBranch: mainOrder}, explicit: map[string]bool{mainBranch: true}, cur: mainBranch}
	started, inDescr := false, false
	for n, raw := range strings.Split(code, "\n") {
		if i := strings.Index(raw, "%%"); i >= 0 {
			raw = raw[:i]
		}
		if inDescr {
			if _, _, ok := strings.Cut(raw, "}"); ok {
				inDescr = false
			}
			continue
		}
		sc := &scanner{s: raw, line: n + 1}
		if sc.eol() {
			continue
		}
		if !started {
			if !sc.word("gitGraph") {
				return nil, sc.errorf("expected gitGraph")
			}
			for _, dir := range []string{"LR", "TB", "BT"} {
				if sc.word(dir) {
					g.Dir = dir
					break
				}
			}
			sc.skip()
			sc.s = strings.TrimPrefix(sc.s, ":")
			started = true
			if !sc.eol() {
				return nil, sc.errorf("unexpected %q", sc.s)
			}
			continue
		}
		var err error
		switch {
		case sc.word("title"):
			g.Title = strings.TrimSpace(sc.s)
			continue
		case sc.word("accTitle"):
			continue
		case sc.word("accDescr"):
			if !strings.HasPrefix(strings.TrimSpace(sc.s), ":") {
				inDescr = !strings.Contains(sc.s, "}")
			}
			continue
		case sc.word("commit"):
			id, typ, tags := "", Normal, []string(nil)
			for !sc.eol() && err == nil {
				switch {
				case sc.key("id"):
					id, err = sc.mustStr()
				case sc.key("msg"):
					_, err = sc.mustStr()
				case sc.key("tag"):
					var t string
					t, err = sc.mustStr()
					tags = append(tags, t)
				case sc.key("type"):
					typ, err = sc.commitType()
				default:
					if _, ok := sc.str(); !ok {
						err = sc.errorf("unexpected %q", sc.s)
					}
				}
			}
			if err == nil {
				d.commit(id, typ, tags)
			}
		case sc.word("branch"):
			var name string
			if name, err = sc.ref(); err != nil {
				break
			}
			var order *float64
			if sc.key("order") {
				sc.skip()
				m := intRe.FindString(sc.s)
				if m == "" {
					err = sc.errorf("expected a number")
					break
				}
				sc.s = sc.s[len(m):]
				v, _ := strconv.ParseFloat(m, 64)
				order = &v
			}
			err = d.branch(name, order)
		case sc.word("merge"):
			var name string
			if name, err = sc.ref(); err != nil {
				break
			}
			id, typ, tags := "", -1, []string(nil)
			for !sc.eol() && err == nil {
				switch {
				case sc.key("id"):
					id, err = sc.mustStr()
				case sc.key("tag"):
					var t string
					t, err = sc.mustStr()
					tags = append(tags, t)
				case sc.key("type"):
					typ, err = sc.commitType()
				default:
					err = sc.errorf("unexpected %q", sc.s)
				}
			}
			if err == nil {
				err = d.merge(name, id, typ, tags)
			}
		case sc.word("checkout"), sc.word("switch"):
			var name string
			if name, err = sc.ref(); err == nil {
				err = d.checkout(name)
			}
		case sc.word("cherry-pick"):
			id, parent, tags, tagsSet := "", "", []string(nil), false
			for !sc.eol() && err == nil {
				switch {
				case sc.key("id"):
					id, err = sc.mustStr()
				case sc.key("parent"):
					parent, err = sc.mustStr()
				case sc.key("tag"):
					var t string
					t, err = sc.mustStr()
					if t != "" {
						tags = append(tags, t)
					}
					tagsSet = true
				default:
					err = sc.errorf("unexpected %q", sc.s)
				}
			}
			if err == nil {
				err = d.cherryPick(id, parent, tags, tagsSet)
			}
		default:
			err = sc.errorf("unexpected %q", strings.TrimSpace(sc.s))
		}
		if err == nil && !sc.eol() {
			err = sc.errorf("unexpected %q", sc.s)
		}
		if err != nil {
			if !strings.HasPrefix(err.Error(), "gitGraph:") {
				err = sc.errorf("%v", err)
			}
			return nil, err
		}
	}
	if !started {
		return nil, fmt.Errorf("gitGraph: empty")
	}
	g.Branches = d.branches()
	return g, nil
}

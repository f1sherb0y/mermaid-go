package class

import (
	"fmt"
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/jison"
)

// member is an attribute as mermaid's ClassMember has it (classTypes.ts);
// the renderer reads text (the display text, ~generics~ made <...>, HTML
// escaped) and classifier ("$" static: underlined, "*" abstract: italic).
type member struct {
	MemberType string `json:"memberType"`
	Visibility string `json:"visibility"`
	Classifier string `json:"classifier"`
	Text       string `json:"text"`
	ID         string `json:"id"`
}

// method is a method: a member with parameters and a return type.
type method struct {
	member
	Parameters string `json:"parameters"`
	ReturnType string `json:"returnType"`
}

var methodRE = jison.Regexp(`([#+~-])?(.+)\((.*)\)([\s$*])?(.*)([$*])?`)

func parseMethod(in string) method {
	in = sanitize(in)
	m := method{member: member{MemberType: "method"}}
	g := methodRE.FindStringSubmatch(in)
	if g == nil {
		// mermaid's ClassMember fails here, reading the id it never set.
		panic(parseError{fmt.Errorf("method %q has no parameter list", in)})
	}
	m.Visibility = g[1]
	m.ID = g[2]
	m.Parameters = strings.TrimSpace(g[3])
	m.Classifier = strings.TrimSpace(g[4])
	m.ReturnType = strings.TrimSpace(g[5])
	if m.Classifier == "" {
		if n := len(m.ReturnType); n > 0 && strings.ContainsAny(m.ReturnType[n-1:], "$*") {
			m.Classifier = m.ReturnType[n-1:]
			m.ReturnType = m.ReturnType[:n-1]
		}
	}
	m.ID = trimID(m.ID)
	text := parseGenericTypes(m.ID) + "(" + parseGenericTypes(m.Parameters) + ")"
	if m.ReturnType != "" {
		text += " : " + parseGenericTypes(m.ReturnType)
	}
	m.Text = displayText(m.Visibility, text)
	return m
}

func parseAttribute(in string) member {
	in = sanitize(in)
	m := member{MemberType: "attribute"}
	if in != "" && strings.Contains("#+~-", in[:1]) {
		m.Visibility = in[:1]
	}
	id := in[len(m.Visibility):]
	if n := len(id); n > 0 && strings.ContainsAny(id[n-1:], "$*") {
		m.Classifier = id[n-1:]
		id = id[:n-1]
	}
	m.ID = trimID(id)
	m.Text = displayText(m.Visibility, parseGenericTypes(m.ID))
	return m
}

// trimID trims, keeping one leading space if there was any.
func trimID(s string) string {
	if strings.HasPrefix(s, " ") {
		return " " + strings.TrimSpace(s)
	}
	return strings.TrimSpace(s)
}

// displayText is the member's text: an escaped visibility, then the rest
// with < and > as entities.
func displayText(visibility, rest string) string {
	if visibility != "" {
		rest = `\` + visibility + rest
	}
	t := strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(rest)
	if strings.HasPrefix(t, `\&lt;`) {
		t = "~" + t[len(`\&lt;`):]
	}
	return t
}

// parseGenericTypes is common.parseGenericTypes: pairs of '~' become
// '<' and '>', also across a comma ("Map~K, V~").
func parseGenericTypes(s string) string {
	sets := splitKeepingCommas(s)
	var out []string
	for i := 0; i < len(sets); i++ {
		set := sets[i]
		if set == "," && i > 0 && i+1 < len(sets) {
			prev, next := sets[i-1], sets[i+1]
			if strings.Count(prev, "~") == 1 && strings.Count(next, "~") == 1 {
				set = prev + "," + next
				i++
				out = out[:len(out)-1]
			}
		}
		out = append(out, genericSet(set))
	}
	return strings.Join(out, "")
}

// splitKeepingCommas is JavaScript's s.split(/(,)/).
func splitKeepingCommas(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, 2*len(parts)-1)
	for i, p := range parts {
		if i > 0 {
			out = append(out, ",")
		}
		out = append(out, p)
	}
	return out
}

func genericSet(s string) string {
	n := strings.Count(s, "~")
	if n <= 1 {
		return s
	}
	lead := false
	if n%2 != 0 && strings.HasPrefix(s, "~") {
		s, lead = s[1:], true
	}
	r := []rune(s)
	for {
		first, last := indexRune(r, '~', false), indexRune(r, '~', true)
		if first < 0 || first == last {
			break
		}
		r[first], r[last] = '<', '>'
	}
	if lead {
		return "~" + string(r)
	}
	return string(r)
}

func indexRune(r []rune, c rune, last bool) int {
	idx := -1
	for i, x := range r {
		if x == c {
			idx = i
			if !last {
				break
			}
		}
	}
	return idx
}

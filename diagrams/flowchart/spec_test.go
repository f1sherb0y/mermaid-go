package flowchart

import (
	"fmt"
	"strings"
	"testing"

	"github.com/f1sherb0y/mermaid-go/render"
)

// Cases from mermaid's parser specs (diagrams/flowchart/parser/*.spec.js),
// with their expected values.

func parse(t *testing.T, src string) *render.Data {
	t.Helper()
	d, err := Parse(src, nil)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	return d
}

func labels(d *render.Data) string {
	var out []string
	for _, n := range d.Nodes {
		out = append(out, n.ID+"="+n.Label+":"+n.Shape)
	}
	return strings.Join(out, " ")
}

func TestEdges(t *testing.T) {
	regular := []struct{ start, end, stroke, typ string }{
		{"--", "--x", "normal", "arrow_cross"}, {"==", "==x", "thick", "arrow_cross"}, {"-.", ".-x", "dotted", "arrow_cross"},
		{"--", "--o", "normal", "arrow_circle"}, {"==", "==o", "thick", "arrow_circle"}, {"-.", ".-o", "dotted", "arrow_circle"},
		{"--", "-->", "normal", "arrow_point"}, {"==", "==>", "thick", "arrow_point"}, {"-.", ".->", "dotted", "arrow_point"},
		{"--", "----x", "normal", "arrow_cross"}, {"==", "====x", "thick", "arrow_cross"}, {"-.", "...-x", "dotted", "arrow_cross"},
		{"--", "---->", "normal", "arrow_point"}, {"==", "====>", "thick", "arrow_point"}, {"-.", "...->", "dotted", "arrow_point"},
	}
	double := []struct{ start, end, stroke, typ string }{
		{"x--", "--x", "normal", "double_arrow_cross"}, {"x==", "==x", "thick", "double_arrow_cross"}, {"x-.", ".-x", "dotted", "double_arrow_cross"},
		{"o--", "--o", "normal", "double_arrow_circle"}, {"o==", "==o", "thick", "double_arrow_circle"}, {"o-.", ".-o", "dotted", "double_arrow_circle"},
		{"<--", "-->", "normal", "double_arrow_point"}, {"<==", "==>", "thick", "double_arrow_point"}, {"<-.", ".->", "dotted", "double_arrow_point"},
	}
	check := func(src, id, typ, stroke, text string) {
		t.Helper()
		d := parse(t, src)
		if len(d.Edges) != 1 {
			t.Fatalf("%q: %d edges", src, len(d.Edges))
		}
		e := d.Edges[0]
		if e.ID != id || e.Type != typ || e.Thickness != stroke || e.Label != text || e.Start != "A" || e.End != "B" {
			t.Errorf("%q: got %s %s %s %q %s->%s", src, e.ID, e.Type, e.Thickness, e.Label, e.Start, e.End)
		}
	}
	for _, c := range regular {
		check(fmt.Sprintf("flowchart TD;\nA e1@%s%s B;", c.start, c.end), "e1", c.typ, c.stroke, "")
		check(fmt.Sprintf("graph TD;\nA %s%s B;", c.start, c.end), "L_A_B_0", c.typ, c.stroke, "")
	}
	for _, c := range double {
		check(fmt.Sprintf("graph TD;\nA %s%s B;", c.start, c.end), "L_A_B_0", c.typ, c.stroke, "")
		check(fmt.Sprintf("graph TD;\nA %s text %s B;", c.start, c.end), "L_A_B_0", c.typ, c.stroke, "text")
		check(fmt.Sprintf("flowchart TD;\nA e1@%s text %s B;", c.start, c.end), "e1", c.typ, c.stroke, "text")
	}
	check("graph TD;A---B;", "L_A_B_0", "arrow_open", "normal", "")
	check("graph TD;A--xB;", "L_A_B_0", "arrow_cross", "normal", "")
	check("graph TD;A--oB;", "L_A_B_0", "arrow_circle", "normal", "")
	check("graph TD;A--x|text with / should work|B;", "L_A_B_0", "arrow_cross", "normal", "text with / should work")
	check("graph TD;A --x|textNoSpace| B;", "L_A_B_0", "arrow_cross", "normal", "textNoSpace")
	check("graph TD;A ~~~ B;", "L_A_B_0", "arrow_open", "invisible", "")
	check("graph TD;A-- \"quoted\" -->B;", "L_A_B_0", "arrow_point", "normal", "quoted")

	// Lengths: minlen is the arrow's extra dashes, dots or equals.
	for src, want := range map[string]int{"A-->B": 1, "A--->B": 2, "A---->B": 3, "A-.->B": 1, "A-...->B": 3, "A===>B": 2, "A-- x ---->B": 3, "A----------------->B": 10} {
		if got := parse(t, "graph TD\n"+src).Edges[0].Minlen; got != want {
			t.Errorf("%s: minlen %d, want %d", src, got, want)
		}
	}
}

func TestKeywordsInIDs(t *testing.T) {
	keywords := []string{"graph", "flowchart", "flowchart-elk", "style", "default", "linkStyle", "interpolate",
		"classDef", "class", "href", "call", "click", "_self", "_blank", "_parent", "_top", "end", "subgraph", "kitty"}
	for _, k := range keywords {
		for _, id := range []string{"a-" + k + "-node", "a." + k + ".node", "a_" + k + "_node", "node_" + k, "node." + k, "node-" + k} {
			d := parse(t, "graph TD;"+id+";")
			if len(d.Nodes) != 1 || d.Nodes[0].ID != id || d.Nodes[0].Label != id {
				t.Errorf("%s: %s", id, labels(d))
			}
		}
	}
	// mermaid's spec asserts that these fail (its lexer takes the keyword
	// up to the '.', '-' or '/'); by the user's decision we read them as
	// ids, as for default, href, click and call.
	for _, k := range []string{"graph", "flowchart", "flowchart-elk", "style", "linkStyle", "interpolate", "classDef",
		"class", "_self", "_blank", "_parent", "_top", "end", "subgraph",
		"default", "href", "click", "call"} {
		d := parse(t, fmt.Sprintf("graph TD; %s.node;%s-node;%s/node;", k, k, k))
		if got, want := labels(d), fmt.Sprintf("%[1]s.node=%[1]s.node:squareRect %[1]s-node=%[1]s-node:squareRect %[1]s/node=%[1]s/node:squareRect", k); got != want {
			t.Errorf("%s: %s, want %s", k, got, want)
		}
	}
}

func TestShapeData(t *testing.T) {
	cases := []struct{ src, want string }{
		{"flowchart TB\n D@{ shape: rounded } & E@{ shape: rounded } --> F{ shape: rounded } & G{ shape: rounded }    ",
			"D=D:rounded E=E:rounded F=shape: rounded:diamond G=shape: rounded:diamond"},
		{"flowchart TB\n      D@{shape: rounded}", "D=D:rounded"},
		{"flowchart TB\n      D@{ shape: rounded , label: \"DD\"}", "D=DD:rounded"},
		{"flowchart TB\n      A --> D@{\n        shape: circle\n        other: \"clock\"\n     }\n\n      ", "A=A:squareRect D=D:circle"},
		{"flowchart TB\n      A[hello]\n      B@{\n        shape: circle\n        other: \"clock\"\n     }\n      C[Hello]@{\n        shape: circle\n        other: \"clock\"\n     }\n      ",
			"A=hello:squareRect B=B:circle C=Hello:circle"},
		{"flowchart TB\n      A@{\n        label: \"This is }\"\n        other: \"clock\"\n     }\n      ", "A=This is }:squareRect"},
		{"flowchart TB\n      A@{\n        label: |\n          This is a\n          multiline string\n        other: \"clock\"\n     }\n      ", "A=This is a\nmultiline string\n:squareRect"},
		{"flowchart TB\n      A@{\n        label: \"This is a\n          multiline string\"\n        other: \"clock\"\n     }\n      ", "A=This is a<br/>multiline string:squareRect"},
		{"flowchart TB\n       n2[\"label for n2\"] &   n4@{ label: \"label for n4\"}   & n5@{ label: \"label for n5\"}\n      ",
			"n2=label for n2:squareRect n4=label for n4:squareRect n5=label for n5:squareRect"},
		{"flowchart TD\n    A[\"A\"] --> B[\"for B\"] &    C@{ label: \"for c\"} & E@{label : \"for E\"}\n    D@{label: \"for D\"}\n      ",
			"A=A:squareRect B=for B:squareRect C=for c:squareRect E=for E:squareRect D=for D:squareRect"},
		{"flowchart TD\n    A[\"@A@\"] --> B[\"@for@ B@\"] & C@{ label: \"@for@ c@\"} & E{\"`@for@ E@`\"} & D((\"@for@ D@\"))\n      H1{{\"@for@ H@\"}}\n      H2{{\"`@for@ H@`\"}}\n      AS1>\"@for@ AS@\"]\n",
			"A=@A@:squareRect B=@for@ B@:squareRect C=@for@ c@:squareRect E=@for@ E@:diamond D=@for@ D@:circle H1=@for@ H@:hexagon H2=@for@ H@:hexagon AS1=@for@ AS@:odd"},
	}
	for _, c := range cases {
		if got := labels(parse(t, c.src)); got != c.want {
			t.Errorf("%q:\n got %s\nwant %s", c.src, got, c.want)
		}
	}
	for _, src := range []string{"flowchart TB\n      A@{ shape: this-shape-does-not-exist }\n", "flowchart TB\n      A@{ shape: rect_left_inv_arrow }\n"} {
		if _, err := Parse(src, nil); err == nil {
			t.Errorf("%q parsed", src)
		}
	}

	ids := func(src string) string {
		var out []string
		for _, e := range parse(t, src).Edges {
			out = append(out, fmt.Sprintf("%s:%v", e.ID, e.Animate))
		}
		return strings.Join(out, " ")
	}
	if got := ids("flowchart TD\n A & B e1@--> C & D\n    A1 e2@--> C1 & D1\n"); got != "L_A_C_0:false L_A_D_0:false e1:false L_B_D_0:false e2:false L_A1_D1_0:false" {
		t.Error(got)
	}
	if got := ids("flowchart TD\n A & B e1@--> C & D\n    A1 e1@--> C1 & D1\n"); got != "L_A_C_0:false L_A_D_0:false e1:false L_B_D_0:false L_A1_C1_0:false L_A1_D1_0:false" {
		t.Error(got)
	}
	if got := ids("flowchart TD\n A e1@--> B\n C e2@--> D\n E e3@--> F\n e1@{ animate: true }\n e2@{ animate: false }\n e3@{ animate: true }\n e3@{ animate: false }\n"); got != "e1:true e2:false e3:false" {
		t.Error(got)
	}
}

// Subgraphs nest without a depth limit.
func TestDeepNesting(t *testing.T) {
	const depth = 300
	var b strings.Builder
	b.WriteString("flowchart TB\n")
	for i := range depth {
		fmt.Fprintf(&b, "subgraph s%d\n", i)
	}
	b.WriteString("a --> b\n")
	for range depth {
		b.WriteString("end\n")
	}
	d := parse(t, b.String())
	parents := map[string]string{}
	for _, n := range d.Nodes {
		parents[n.ID] = n.ParentID
	}
	n, id := 0, "a"
	for parents[id] != "" {
		id = parents[id]
		n++
	}
	if n != depth || id != "s0" || d.Nodes[0].ID != "s0" {
		t.Errorf("a is %d deep under %s; first node %s", n, id, d.Nodes[0].ID)
	}
}

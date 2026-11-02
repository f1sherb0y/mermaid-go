package flowchart

import (
	"fmt"
	"strings"
	"testing"
)

// Input mermaid rejects only by a lexer accident, which we draw.

// An '@' in a node's or edge's text is text: mermaid's LINK_ID takes
// A[foo@bar] for the edge id "A[foo".
func TestAtInText(t *testing.T) {
	edges := func(src string) string {
		var out []string
		for _, e := range parse(t, src).Edges {
			out = append(out, fmt.Sprintf("%s:%s->%s:%s", e.ID, e.Start, e.End, e.Label))
		}
		return strings.Join(out, " ")
	}
	cases := []struct{ src, nodes, edges string }{
		{"flowchart TD\n    A[foo@bar] --> B\n", "A=foo@bar:squareRect B=B:squareRect", "L_A_B_0:A->B:"},
		{"flowchart TD\n    A[foo@bar]-->B\n", "A=foo@bar:squareRect B=B:squareRect", "L_A_B_0:A->B:"},
		{"flowchart TD\n    A(user@example.com) --> B{a@b}\n", "A=user@example.com:roundedRect B=a@b:diamond", "L_A_B_0:A->B:"},
		{"flowchart TD\n    A-->|x@y|B\n", "A=A:squareRect B=B:squareRect", "L_A_B_0:A->B:x@y"},
		{"flowchart TD\n    A>x@y] --> B[[a@b]]\n", "A=x@y:odd B=a@b:subroutine", "L_A_B_0:A->B:"},
		{"flowchart LR\n    A((a@b)) --> B[(db@host)] --> C{{x@y}} --> D[/p@q/]\n",
			"A=a@b:circle B=db@host:cylinder C=x@y:hexagon D=p@q:lean_right", "L_A_B_0:A->B: L_B_C_0:B->C: L_C_D_0:C->D:"},
		{"flowchart LR\n    A[a@b] e1@--> B[c@d]\n", "A=a@b:squareRect B=c@d:squareRect", "e1:A->B:"},
		{"flowchart TD\n    subgraph S[team@corp]\n    A\n    end\n", "S=team@corp:rect A=A:squareRect", ""},
	}
	for _, c := range cases {
		d := parse(t, c.src)
		if got := labels(d); got != c.nodes {
			t.Errorf("%q: nodes %s, want %s", c.src, got, c.nodes)
		}
		if got := edges(c.src); got != c.edges {
			t.Errorf("%q: edges %s, want %s", c.src, got, c.edges)
		}
	}
	// An '@' in a bare node id is still an error, as in mermaid.
	if _, err := Parse("flowchart TD\n    A --> foo@bar\n", nil); err == nil {
		t.Error("foo@bar parsed")
	}
}

// White space at the end of a line changes nothing. mermaid fails on it
// after a subgraph's [title] and a class statement; after shape data it
// reads it, and so do we.
func TestTrailingSpace(t *testing.T) {
	for _, src := range []string{
		"flowchart TD\n    subgraph one [My Group]\n    A --> B\n    end\n",
		"flowchart TD\n    A --> B\n    class A green\n    classDef green fill:#0f0\n",
		"flowchart TD\n    A@{ shape: rounded, label: \"x\" }\n    B e1@--> C\n    e1@{ animate: true }\n",
	} {
		want := parse(t, src)
		for _, ws := range []string{"  ", "\t"} {
			got := parse(t, strings.ReplaceAll(src, "\n", ws+"\n"))
			if labels(got) != labels(want) || len(got.Edges) != len(want.Edges) {
				t.Errorf("%q with %q: %s, want %s", src, ws, labels(got), labels(want))
			}
		}
	}
}

// A node id may start with a keyword (end-user, class-a, graph-1): an id
// character after the keyword makes it an id. The keywords still work.
func TestKeywordIDs(t *testing.T) {
	d := parse(t, "flowchart TD\n    end-user --> class-a --> style-x --> graph-1\n    subgraph-a --> click-me --> default-x --> end用户\n")
	if got, want := labels(d), "end-user=end-user:squareRect class-a=class-a:squareRect style-x=style-x:squareRect graph-1=graph-1:squareRect "+
		"subgraph-a=subgraph-a:squareRect click-me=click-me:squareRect default-x=default-x:squareRect end用户=end用户:squareRect"; got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
	d = parse(t, "flowchart TD\n    subgraph X\n    direction LR\n    A --> B\n    end\n    subgraph Y\n    C\n    end  \n    class A foo\n    classDef foo fill:#f00\n"+
		"    style B fill:#0f0\n    linkStyle 0 stroke:#00f\n    click A href \"https://x\" _blank\n")
	if got, want := labels(d), "Y=Y:rect X=X:rect A=A:squareRect B=B:squareRect C=C:squareRect"; got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

package class

import (
	"strings"
	"testing"

	"github.com/f1sherb0y/mermaid-go/config"
)

// A class name may start with a keyword and go on with '-' or '.':
// mermaid lexes link-manager as LINK and "-manager" and fails.
func TestKeywordNames(t *testing.T) {
	d, err := Parse("classDiagram\n    class link-manager\n    link-manager --> note-service : uses\n    note-service : +send() void\n    click-handler <|-- style-sheet\n    class namespace.x\n", config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	var ids, edges []string
	for _, n := range d.Nodes {
		ids = append(ids, n.ID)
	}
	for _, e := range d.Edges {
		edges = append(edges, e.Start+">"+e.End+":"+e.Label)
	}
	if got, want := strings.Join(ids, " "), "link-manager note-service click-handler style-sheet namespace.x"; got != want {
		t.Errorf("nodes %q, want %q", got, want)
	}
	if got, want := strings.Join(edges, " "), "link-manager>note-service:uses click-handler>style-sheet:"; got != want {
		t.Errorf("edges %q, want %q", got, want)
	}
	// The keywords themselves still work.
	if _, err := Parse("classDiagram\n    class A\n    link A \"https://x\" _blank\n    note for A \"n\"\n    style A fill:#f9f\n", config.Defaults()); err != nil {
		t.Error(err)
	}
}

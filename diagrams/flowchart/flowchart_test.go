package flowchart

import (
	"testing"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/datatest"
)

// allowed are the accepted differences from mermaid, by case.
var allowed = map[string][]string{}

func TestGetData(t *testing.T) {
	cases := datatest.Cases([]string{"testdata/flowchart", "testdata/examples", "testdata/nested",
		"../../../../data/mermaid-port/corpus-real/real"}, "flowchart", "graph", "flowchart-elk")
	datatest.Run(t, Parse, cases, allowed)
}

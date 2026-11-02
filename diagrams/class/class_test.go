package class

import (
	"testing"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/datatest"
)

func TestGetData(t *testing.T) {
	cases := datatest.Cases([]string{"testdata/classDiagram", "testdata/nested", "../../../../data/mermaid-port/corpus-real/real"}, "classDiagram", "classDiagram-v2")
	datatest.Run(t, Parse, cases, allowed)
}

// allowed are the differences from mermaid accepted, by case.
var allowed = map[string][]string{
	// The front matter sets class.hierarchicalNamespaces false, which the
	// dumper (getDiagramFromText, not render) does not apply.
	"classDiagram/22": {"nodes", "node label", "node parentId"},
}

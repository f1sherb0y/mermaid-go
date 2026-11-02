package requirement

import (
	"testing"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/datatest"
)

// testdata/edge 03, 04 and 07-09 are input mermaid fails on by a lexer
// accident (trailing white space, a field value that is or starts with a
// keyword, has '-', ',' or ':', or starts with a non-ASCII letter), which
// we draw: their dumps are mermaid's for the same diagram with the values
// quoted and no trailing white space. 10 and 11 still fail, as in mermaid.
func TestGetData(t *testing.T) {
	cases := datatest.Cases([]string{"testdata/requirementDiagram", "testdata/edge", "../../../../data/mermaid-port/corpus-real/real"}, "requirementDiagram")
	datatest.Run(t, Parse, cases, nil)
}

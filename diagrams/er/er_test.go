package er

import (
	"testing"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/datatest"
)

// testdata/edge 02, 11-16 and 18 are input mermaid fails on (or, for 02
// and 18, misreads: a style line's trailing white space runs it into the
// next line) by a lexer accident, which we draw: their dumps are mermaid's
// for the same diagram written so that it reads it (names quoted, no
// trailing white space). 17 checks that the cardinality words still work.
func TestGetData(t *testing.T) {
	cases := datatest.Cases([]string{"testdata/entityRelationshipDiagram", "testdata/edge", datatest.Data("mermaid-port/corpus-real/real")}, "erDiagram")
	datatest.Run(t, Parse, cases, nil)
}

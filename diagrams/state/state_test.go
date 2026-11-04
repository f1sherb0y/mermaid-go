package state

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagrams/internal/datatest"
)

func TestGetData(t *testing.T) {
	cases := datatest.Cases([]string{"testdata/stateDiagram", "testdata/nested", datatest.Data("mermaid-port/corpus-real/real")}, "stateDiagram", "stateDiagram-v2")
	tmp := t.TempDir()
	for i, c := range cases {
		if c.Dump == "" {
			continue
		}
		b, err := os.ReadFile(c.Dump)
		if err != nil {
			t.Fatal(err)
		}
		cases[i].Dump = filepath.Join(tmp, strconv.Itoa(i)+".json")
		if err := os.WriteFile(cases[i].Dump, renameRegions(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	datatest.Run(t, Parse, cases, allowed)
}

// randomID is mermaid's generateId: id-<random>-<counter>.
var randomID = regexp.MustCompile(`\bid-[0-9a-z]{4,12}-(\d+)`)

// renameRegions gives the concurrent regions mermaid names with random ids
// (generateId) our names, id-region-1, 2, ... in the order mermaid made
// them (its counter's).
func renameRegions(dump []byte) []byte {
	seen := map[string]int{}
	for _, m := range randomID.FindAllSubmatch(dump, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		seen[string(m[0])] = n
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return seen[ids[i]] < seen[ids[j]] })
	ours := map[string]string{}
	for i, id := range ids {
		ours[id] = "id-region-" + strconv.Itoa(i+1)
	}
	return randomID.ReplaceAllFunc(dump, func(m []byte) []byte { return []byte(ours[string(m)]) })
}

// allowed are the differences from mermaid accepted, by case.
var allowed = map[string][]string{}

// An id's trailing white space is not part of it (stateDb trims ids).
func TestTrailingSpace(t *testing.T) {
	d, err := Parse("stateDiagram-v2\n    state \"This is a state description\" as s2  \n    s2 --> s3  \n", config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range d.Nodes {
		ids = append(ids, n.ID)
	}
	if got := strings.Join(ids, " "); got != "s2 s3" {
		t.Errorf("nodes %q", got)
	}
}

package architecture

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/diagram"
	"github.com/f1sherb0y/mermaid-go/scene"
	"github.com/f1sherb0y/mermaid-go/text"
	"github.com/f1sherb0y/mermaid-go/theme"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/datatest"
)

type vars struct{ v *theme.Variables }

func (t vars) Get(name string) string { return t.v.Str(strings.Split(name, ".")...) }

// context is the default theme's, with a font for the theme's family: the
// one mermaid.live draws (Trebuchet MS) where there is one, else Recursive.
func context(t *testing.T) *diagram.Context {
	fs := text.NewFonts()
	if fs.AddFile("trebuchet ms", "C:/Windows/Fonts/trebuc.ttf") != nil &&
		fs.AddFile("trebuchet ms", datatest.Data("ref/fonts/RecursiveSansLinear-VF.ttf")) != nil {
		t.Skip("no font")
	}
	v, err := theme.Get("default", nil)
	if err != nil {
		t.Fatal(err)
	}
	return &diagram.Context{Config: config.Defaults(), Theme: vars{v}, Fonts: fs, Look: "classic", ID: "mermaid-svg"}
}

func cases(t *testing.T) map[string]string {
	out := map[string]string{}
	for _, glob := range []string{"testdata/*.mmd", "../../testdata/corpus/architecture/*.mmd", "../../testdata/corpus/nested/*architecture*.mmd"} {
		files, _ := filepath.Glob(glob)
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := config.Preprocess(string(b))
			if err != nil {
				t.Fatal(err)
			}
			out[f] = doc.Code
		}
	}
	if len(out) < 20 {
		t.Fatalf("only %d cases", len(out))
	}
	return out
}

// TestRender draws every case and checks the layout: no two services'
// boxes (icon and label) overlap, every group's box holds its members, and
// siblings' boxes keep apart.
func TestRender(t *testing.T) {
	ctx := context(t)
	for name, src := range cases(t) {
		if _, err := Render(src, ctx); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		d, _ := Parse(src)
		l := newLayout(d, ctx)
		l.place()
		if err := l.check(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestRandom lays out random diagrams: deep nesting, edges between any
// nodes in any directions, hints. Edges almost all keep clear of nodes,
// labels and titles (in these crowded, contradictory diagrams a few that
// start inside another node's clearance do not).
func TestRandom(t *testing.T) {
	bad, total := 0, 0
	defer func() {
		if t.Logf("%d of %d edges not clear", bad, total); bad*100 > 3*total {
			t.Errorf("%d of %d edges not clear", bad, total)
		}
	}()
	ctx := context(t)
	rng := rand.New(rand.NewSource(1))
	sides := "LRTB"
	for k := range 300 {
		var b strings.Builder
		b.WriteString("architecture-beta\n")
		ng := rng.Intn(14)
		for g := range ng {
			fmt.Fprintf(&b, "group g%d(cloud)[Group %d]", g, g)
			if g > 0 && rng.Intn(3) > 0 {
				fmt.Fprintf(&b, " in g%d", rng.Intn(g))
			}
			b.WriteString("\n")
		}
		nn := 1 + rng.Intn(30)
		for n := range nn {
			kind := "service"
			if rng.Intn(6) == 0 {
				kind = "junction"
			}
			fmt.Fprintf(&b, "%s n%d", kind, n)
			if kind == "service" {
				fmt.Fprintf(&b, "(server)[Node %d]", n)
			}
			if ng > 0 && rng.Intn(4) > 0 {
				fmt.Fprintf(&b, " in g%d", rng.Intn(ng))
			}
			b.WriteString("\n")
		}
		for range rng.Intn(2 * nn) {
			a, c := rng.Intn(nn), rng.Intn(nn)
			label := ""
			if rng.Intn(4) == 0 {
				label = "[label]"
			}
			arrow := "--"
			if label != "" {
				arrow = "-" + label + "-"
			}
			fmt.Fprintf(&b, "n%d:%c %s> %c:n%d\n", a, sides[rng.Intn(4)], arrow, sides[rng.Intn(4)], c)
		}
		if nn >= 3 && rng.Intn(3) == 0 {
			fmt.Fprintf(&b, "align row n0 n1 n2\n")
		}
		src := b.String()
		d, err := Parse(src)
		if err != nil {
			continue // a {group} rule or the like
		}
		l := newLayout(d, ctx)
		l.place()
		if err := l.check(); err != nil {
			t.Fatalf("case %d: %v\n%s", k, err, src)
		}
		obs := l.obstacles()
		for _, e := range d.Edges {
			if !l.fits(e, l.routes[e], obs) {
				bad++
				if testing.Verbose() && bad <= 3 {
					t.Logf("case %d: %s:%c -- %c:%s runs through something: %v", k, e.From.ID, e.FromSide, e.ToSide, e.To.ID, l.routes[e])
				}
			}
			total++
		}
		if _, err := Render(src, ctx); err != nil {
			t.Fatalf("case %d: %v", k, err)
		}
	}
}

// check verifies the layout's boxes.
func (l *layout) check() error {
	inner := func(a, b scene.Rect) bool { // a strictly inside b
		return a.X > b.X && a.Y > b.Y && a.X+a.W < b.X+b.W && a.Y+a.H < b.Y+b.H
	}
	apart := func(a, b scene.Rect) bool {
		return a.X+a.W <= b.X+1e-6 || b.X+b.W <= a.X+1e-6 || a.Y+a.H <= b.Y+1e-6 || b.Y+b.H <= a.Y+1e-6
	}
	name := func(it item) string {
		if it.group != nil {
			return "group " + it.group.ID
		}
		return "node " + it.node.ID
	}
	for g, ks := range l.kids {
		for i, a := range ks {
			ra := l.itemBox(a)
			if g != nil && !inner(ra, l.box[g]) {
				return fmt.Errorf("%s %v is not inside group %s %v", name(a), ra, g.ID, l.box[g])
			}
			for _, b := range ks[i+1:] {
				if rb := l.itemBox(b); !apart(ra, rb) {
					return fmt.Errorf("%s %v overlaps %s %v", name(a), ra, name(b), rb)
				}
			}
		}
	}
	// Services anywhere: no overlaps (implied by the above, checked plainly).
	for i, a := range l.nodes {
		for _, b := range l.nodes[i+1:] {
			if ra, rb := l.itemBox(item{node: a}), l.itemBox(item{node: b}); !apart(ra, rb) {
				return fmt.Errorf("node %s %v overlaps node %s %v", a.ID, ra, b.ID, rb)
			}
		}
	}
	return nil
}

// big is a large diagram as people draw them: groups of groups of
// services, chains inside, edges between neighbors and across groups.
func big(groups, subgroups, services int) string {
	var b strings.Builder
	b.WriteString("architecture-beta\n")
	icons := []string{"server", "database", "disk", "cloud", "internet"}
	for g := range groups {
		fmt.Fprintf(&b, "group g%d(cloud)[Region %d]\n", g, g)
		for s := range subgroups {
			fmt.Fprintf(&b, "group g%ds%d(server)[Zone %d.%d] in g%d\n", g, s, g, s, g)
			for n := range services {
				fmt.Fprintf(&b, "service n%d_%d_%d(%s)[Service %d.%d.%d] in g%ds%d\n", g, s, n, icons[(g+s+n)%len(icons)], g, s, n, g, s)
			}
		}
	}
	for g := range groups {
		for s := range subgroups {
			for n := 1; n < services; n++ {
				if n%4 == 0 {
					fmt.Fprintf(&b, "n%d_%d_%d:B --> T:n%d_%d_%d\n", g, s, n-4, g, s, n)
				} else {
					fmt.Fprintf(&b, "n%d_%d_%d:R --> L:n%d_%d_%d\n", g, s, n-1, g, s, n)
				}
			}
			if s > 0 {
				fmt.Fprintf(&b, "n%d_%d_0:B --> T:n%d_%d_0\n", g, s-1, g, s)
			}
		}
		if g > 0 {
			fmt.Fprintf(&b, "n%d_0_%d:R --> L:n%d_0_0\n", g-1, services-1, g)
		}
	}
	return b.String()
}

// TestBig lays out 200 services in 5 groups of 4 groups.
func TestBig(t *testing.T) {
	ctx := context(t)
	d, err := Parse(big(5, 4, 10))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	l := newLayout(d, ctx)
	l.place()
	if dt := time.Since(start); dt > 500*time.Millisecond {
		t.Errorf("%d services: %v", len(d.Nodes), dt)
	}
	if err := l.check(); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkBig(b *testing.B) {
	ctx := context(&testing.T{})
	src := big(5, 4, 10)
	for b.Loop() {
		if _, err := Render(src, ctx); err != nil {
			b.Fatal(err)
		}
	}
}

package architecture

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/f1sherb0y/mermaid-go/config"
	"github.com/f1sherb0y/mermaid-go/scene"
)

// laidOut lays out a test file with our grid layout (these tests are
// about it; which layout a diagram takes is TestChoice's).
func laidOut(t *testing.T, file string) (*layout, map[string]scene.Pt) {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := config.Preprocess(string(b))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Parse(doc.Code)
	if err != nil {
		t.Fatal(err)
	}
	l := newLayout(d, context(t))
	l.placeGrid()
	l.route()
	at := map[string]scene.Pt{}
	for _, n := range d.Nodes {
		at[n.ID] = l.at[n]
	}
	return l, at
}

// TestEdgesClear: every edge leaves and enters by its sides and passes no
// other node, label, junction or group title, in every case (a cycle of
// contradictory directions included).
func TestEdgesClear(t *testing.T) {
	ctx := context(t)
	for name, src := range cases(t) {
		d, _ := Parse(src)
		l := newLayout(d, ctx)
		l.place()
		obs := l.obstacles()
		for _, e := range d.Edges {
			if pts := l.routes[e]; !l.fits(e, pts, obs) {
				t.Errorf("%s: %s:%c -- %c:%s runs through something: %v", name, e.From.ID, e.FromSide, e.ToSide, e.To.ID, pts)
			}
		}
	}
}

// TestCycle: an edge whose sides contradict the others' goes around.
func TestCycle(t *testing.T) {
	l, _ := laidOut(t, "testdata/11-cycle-conflict.mmd")
	routed := 0
	for _, e := range l.d.Edges {
		if len(l.routes[e]) > 3 {
			routed++
		}
	}
	if routed != 1 {
		t.Errorf("%d edges go around, want 1", routed)
	}
}

// TestColumns: nodes line up only where mermaid's constraints say (docs
// case 04: Source C over DB Three, its own column, not Analyst's).
func TestColumns(t *testing.T) {
	_, at := laidOut(t, "../../testdata/corpus/architecture/04.mmd")
	if at["src_c"].X != at["db_three"].X || at["src_b"].X != at["brief"].X || at["src_a"].X != at["db_one"].X {
		t.Errorf("columns: %v", at)
	}
	if at["src_c"].X >= at["analyst"].X-1 {
		t.Errorf("Source C at %v, not left of Analyst at %v", at["src_c"].X, at["analyst"].X)
	}
	if d := at["analyst"].X - at["brief"].X; d != 200 {
		t.Errorf("Brief to Analyst %v, want the ideal edge length 200", d)
	}
}

// TestSpacing: distances as mermaid.live draws them, about: an edge's ends
// a pitch apart, unconnected neighbors closer, groups nodeSeparation apart.
func TestSpacing(t *testing.T) {
	_, at := laidOut(t, "testdata/17-group-to-group.mmd")
	if d := at["prom"].X - at["api2"].X; d != 260 {
		t.Errorf("groups side by side: %v apart, want 260 (mermaid.live 267)", d)
	}
	if d := at["api1"].Y - at["fe1"].Y; d != 260 {
		t.Errorf("groups stacked: %v apart, want 260 (mermaid.live 255)", d)
	}
	_, at = laidOut(t, "testdata/14-split-direction.mmd")
	if d := at["serv1"].Y - at["disk"].Y; d != 200 {
		t.Errorf("edge: %v, want 200", d)
	}
	if d := at["db"].Y - at["serv1"].Y; d != 128 {
		t.Errorf("unconnected neighbors: %v, want 128", d)
	}
	_, at = laidOut(t, "testdata/13-align-only.mmd")
	if d := at["b"].X - at["a"].X; d != 160 {
		t.Errorf("align row: %v, want 160", d)
	}
}

// TestFan: services with edges to one outside node stack across them; a
// part of a group with no edge inside it sits where mermaid's spatial map
// says.
func TestFan(t *testing.T) {
	_, at := laidOut(t, "testdata/20-microservices.mmd")
	if at["web"].X != at["mobile"].X || at["web"].Y == at["mobile"].Y {
		t.Errorf("web %v, mobile %v: not stacked", at["web"], at["mobile"])
	}
	_, at = laidOut(t, "testdata/19-web-app.mmd")
	if at["queue"].Y != at["app3"].Y || at["queue"].X <= at["app3"].X {
		t.Errorf("queue %v, app3 %v: not in its row", at["queue"], at["app3"])
	}
}

// TestGroupTitles: a group whose title an edge would cross is padded on the
// left until the title ends before it.
func TestGroupTitles(t *testing.T) {
	l, _ := laidOut(t, "../../testdata/corpus/architecture/04.mmd")
	for _, g := range l.d.Groups {
		r, _ := l.titleBox(g)
		for _, e := range l.d.Edges {
			pts := l.routes[e]
			for i := 1; i < len(pts); i++ {
				if crosses(pts[i-1], pts[i], r) {
					t.Errorf("%s:%s crosses the title of %s", e.From.ID, e.To.ID, g.ID)
				}
			}
		}
	}
	if l.pad[l.d.Groups[1]].l <= l.padding+2.5 {
		t.Errorf("Storage not padded: %v", *l.pad[l.d.Groups[1]])
	}
}

// TestArrow: an arrowhead points along its line, slanted or not.
func TestArrow(t *testing.T) {
	l := &layout{iconSize: 80}
	for _, c := range []struct{ q, p scene.Pt }{{scene.Pt{X: 0, Y: 0}, scene.Pt{X: 30, Y: 40}}, {scene.Pt{X: 0, Y: 0}, scene.Pt{X: -50, Y: 0}}} {
		a := l.arrow(c.q, c.p, 'L', scene.RGB(0, 0, 0)).(*scene.Path)
		tip := a.Segs[0].P[0]
		dx, dy := c.p.X-c.q.X, c.p.Y-c.q.Y
		n := math.Hypot(dx, dy)
		if math.Abs(tip.X-(c.p.X+2*dx/n)) > 1e-9 || math.Abs(tip.Y-(c.p.Y+2*dy/n)) > 1e-9 {
			t.Errorf("tip %v for a line %v to %v", tip, c.q, c.p)
		}
		// The base's middle is on the line.
		b1, b2 := a.Segs[1].P[0], a.Segs[2].P[0]
		mx, my := (b1.X+b2.X)/2, (b1.Y+b2.Y)/2
		if math.Abs((mx-c.q.X)*dy-(my-c.q.Y)*dx) > 1e-6 {
			t.Errorf("base %v %v off the line %v to %v", b1, b2, c.q, c.p)
		}
	}
}

// TestStraight: placements that keep edges straight as their sides say,
// over going around (28: Media's edge out of its group would pass Ads; 20:
// Notifications' would pass Search; 27: the VPN link stays level), and a
// single edge whose ends cannot line up goes out and in square (19: Queue
// to Worker, in another group), where mermaid slants it.
func TestStraight(t *testing.T) {
	for _, c := range []struct{ file, from, to string }{
		{"testdata/28-platform.mmd", "media", "objects"},
		{"testdata/28-platform.mmd", "ads", "olap"},
		{"testdata/20-microservices.mmd", "notify", "ordersdb"},
		{"testdata/20-microservices.mmd", "search", "index"},
		{"testdata/27-hybrid.mmd", "vpn", "cvpn"},
	} {
		l, at := laidOut(t, c.file)
		if at[c.from].Y != at[c.to].Y {
			t.Errorf("%s: %s %v, %s %v: not level", c.file, c.from, at[c.from], c.to, at[c.to])
		}
		for _, e := range l.d.Edges {
			if e.From.ID == c.from && e.To.ID == c.to && len(l.routes[e]) != 2 {
				t.Errorf("%s: %s to %s goes around: %v", c.file, c.from, c.to, l.routes[e])
			}
		}
	}
	l, _ := laidOut(t, "testdata/19-web-app.mmd")
	for _, e := range l.d.Edges {
		if e.From.ID != "queue" || e.To.ID != "worker" {
			continue
		}
		pts := l.routes[e]
		for i := 1; i < len(pts); i++ {
			if pts[i].X != pts[i-1].X && pts[i].Y != pts[i-1].Y {
				t.Errorf("queue to worker slants: %v", pts)
			}
		}
	}
}

// TestUnconnected: parts no edge ties to the rest go as fcose packs them:
// beside the first, centered on it, or under it when that is smaller (08's
// Client to the right; 12's group under its row, centered).
func TestUnconnected(t *testing.T) {
	_, at := laidOut(t, "testdata/08-azure.mmd")
	lo, hi := math.Inf(1), math.Inf(-1)
	right := math.Inf(-1)
	for id, p := range at {
		if id != "client" {
			lo, hi, right = math.Min(lo, p.Y), math.Max(hi, p.Y), math.Max(right, p.X)
		}
	}
	if c := at["client"]; c.X <= right || math.Abs(c.Y-(lo+hi)/2) > 100 {
		t.Errorf("client %v: not right of the rest (to %v), centered on %v..%v", c, right, lo, hi)
	}
	_, at = laidOut(t, "testdata/12-title-text.mmd")
	row := (at["q"].X + at["n"].X) / 2
	group := (at["long"].X + at["cjk"].X) / 2
	if at["long"].Y <= at["q"].Y || math.Abs(row-group) > 60 {
		t.Errorf("group at %v, %v: not under the row at %v, %v", at["long"], at["cjk"], at["q"], at["n"])
	}
}

// TestChoice: diagrams that fcose lays out well keep mermaid's layout
// (mermaid's positions, TestFcoseMatchesMermaid); where fcose overlaps
// nodes, runs edges through them or scatters the diagram, ours is taken
// (choose.go).
func TestChoice(t *testing.T) {
	grid := map[string]bool{"08-azure": true, "15-fanout": true, "19-web-app": true, "20-microservices": true,
		"24-long-labels": true, "26-event-pipeline": true, "27-hybrid": true, "28-platform": true, "29-ml-platform": true}
	ctx := context(t)
	for name, src := range cases(t) {
		d, _ := Parse(src)
		l := newLayout(d, ctx)
		l.place()
		base := strings.TrimSuffix(filepath.Base(name), ".mmd")
		want := "fcose"
		if grid[base] && strings.HasPrefix(filepath.ToSlash(name), "testdata/") {
			want = "grid"
		}
		if l.choice != want {
			t.Errorf("%s: took %s (fcose %+v, grid %+v), want %s", name, l.choice, l.qFcose, l.qGrid, want)
		}
	}
}

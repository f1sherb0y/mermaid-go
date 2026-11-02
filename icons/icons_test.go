package icons

import (
	"testing"

	"github.com/f1sherb0y/mermaid-go/scene"
)

const set = `{
	"prefix": "test",
	"width": 24, "height": 24,
	"icons": {
		"box": {"body": "<path fill=\"currentColor\" fill-rule=\"evenodd\" d=\"M2 2h20v20H2zm4 4v12h12V6z\"/>"},
		"wide": {"body": "<rect width=\"48\" height=\"24\" fill=\"#f00\"/>", "width": 48},
		"grad": {"body": "<defs><linearGradient id=\"a\"><stop offset=\"0\" stop-color=\"#000\"/><stop offset=\"1\" stop-color=\"#fff\"/></linearGradient></defs><g transform=\"translate(2 3)\" opacity=\".5\"><circle cx=\"5\" cy=\"5\" r=\"4\" fill=\"url(#a)\"/><use href=\"#l\" x=\"1\"/></g><line id=\"l\" x1=\"0\" y1=\"0\" x2=\"4\" y2=\"0\" stroke=\"#00f\" style=\"stroke-width: 2px\"/>"}
	},
	"aliases": {
		"box-flipped": {"parent": "box", "hFlip": true},
		"box-turned": {"parent": "box-flipped", "rotate": 1}
	}
}`

func TestLookup(t *testing.T) {
	if err := Register("t", []byte(set)); err != nil {
		t.Fatal(err)
	}
	if err := Register("bad", []byte(`{"icons": {}}`)); err == nil {
		t.Error("an empty set: no error")
	}
	if _, ok := Lookup("t:nope", ""); ok {
		t.Error("t:nope found")
	}
	if _, ok := Lookup("nope:box", ""); ok {
		t.Error("nope:box found")
	}
	ic, ok := Lookup("box", "t")
	if !ok || ic.Width != 24 || ic.Height != 24 {
		t.Fatalf("box: %+v %v", ic, ok)
	}
	if ic, _ := Lookup("t:wide", ""); ic.Width != 48 || ic.Height != 24 {
		t.Errorf("wide: %+v", ic)
	}
	// A flip then a quarter turn: (2, 2), the top left of the box, goes to
	// (22, 2) flipped and to (22, 22) turned.
	ic, ok = Lookup("t:box-turned", "")
	if !ok {
		t.Fatal("box-turned not found")
	}
	if x, y := ic.Transform.Apply(2, 2); !near(x, 22) || !near(y, 22) {
		t.Errorf("turned (2, 2) = (%v, %v)", x, y)
	}
}

func near(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

func TestDraw(t *testing.T) {
	Register("t", []byte(set))
	// 24 units into 48 px: scaled 2, at (10, 20).
	ic, _ := Lookup("t:box", "")
	g := ic.Draw(10, 20, 48, 48, scene.RGB(1, 2, 3)).(*scene.Group)
	if x, y := g.Transform.Apply(2, 2); !near(x, 14) || !near(y, 24) {
		t.Errorf("(2, 2) drawn at (%v, %v)", x, y)
	}
	p := g.Items[0].(*scene.Path)
	if p.Fill != scene.RGB(0, 0, 0) || !p.EvenOdd || len(p.Segs) != 10 {
		t.Errorf("box: fill %v evenodd %v %d segs", p.Fill, p.EvenOdd, len(p.Segs))
	}
	// A wide icon in a square: centered.
	ic, _ = Lookup("t:wide", "")
	g = ic.Draw(0, 0, 48, 48, scene.None).(*scene.Group)
	if x, y := g.Transform.Apply(0, 0); !near(x, 0) || !near(y, 12) {
		t.Errorf("wide at (%v, %v)", x, y)
	}
	// Gradient as its mean, opacity, transforms, <use>.
	ic, _ = Lookup("t:grad", "")
	g = ic.Draw(0, 0, 24, 24, scene.None).(*scene.Group)
	inner := g.Items[0].(*scene.Group)
	c := inner.Items[0].(*scene.Path)
	if c.Fill.R != 128 || !near(c.Fill.A, 0.5) {
		t.Errorf("gradient fill %+v", c.Fill)
	}
	if x, _ := inner.Transform.Apply(0, 0); !near(x, 2) {
		t.Errorf("group transform %+v", inner.Transform)
	}
	use := inner.Items[1].(*scene.Group)
	if l := use.Items[0].(*scene.Path); l.Stroke != (scene.Color{B: 255, A: 0.5}) || l.StrokeWidth != 2 || !l.Fill.IsNone() {
		t.Errorf("used line %+v", l)
	}
	if len(g.Items) != 2 {
		t.Errorf("%d items, want the group and the line", len(g.Items))
	}
}

func TestUnknown(t *testing.T) {
	g := Unknown.Draw(0, 0, 80, 80, scene.None).(*scene.Group)
	items := g.Items
	if len(items) != 2 {
		t.Fatalf("%d items", len(items))
	}
	txt := items[1].(*scene.Group).Items[0].(*scene.Text)
	if txt.Runs[0].Text != "?" || txt.Size != 67.75 || txt.Color != scene.RGB(255, 255, 255) {
		t.Errorf("%+v", txt)
	}
}

func TestTransform(t *testing.T) {
	m, _ := transform("translate(10 20) rotate(90) scale(2)")
	if x, y := m.Apply(1, 0); !near(x, 10) || !near(y, 22) {
		t.Errorf("(1, 0) = (%v, %v)", x, y)
	}
	if _, ok := transform(""); ok {
		t.Error("empty transform")
	}
}

// TestCache: a body is drawn once per fill, and its items are shared.
func TestCache(t *testing.T) {
	ic := Icon{Body: `<path d="M0 0h4v4z"/>`, Width: 4, Height: 4}
	red, blue := scene.RGB(255, 0, 0), scene.RGB(0, 0, 255)
	a := ic.Draw(0, 0, 8, 8, red).(*scene.Group)
	b := ic.Draw(10, 0, 8, 8, red).(*scene.Group)
	c := ic.Draw(0, 0, 8, 8, blue).(*scene.Group)
	if len(a.Items) != 1 || a.Items[0] != b.Items[0] {
		t.Fatalf("not shared: %v %v", a.Items, b.Items)
	}
	if a.Transform == b.Transform {
		t.Error("same transform at different places")
	}
	if c.Items[0].(*scene.Path).Fill != blue || a.Items[0].(*scene.Path).Fill != red {
		t.Errorf("fills: %v %v", a.Items[0].(*scene.Path).Fill, c.Items[0].(*scene.Path).Fill)
	}
	if cap(a.Items) != len(a.Items) {
		t.Error("shared items can be appended to in place")
	}
}

// TestCurrentColor: currentColor is the element's own color, whatever the
// attributes' order.
func TestCurrentColor(t *testing.T) {
	for range 20 {
		items := drawBody(`<path color="#0f0" fill="currentColor" stroke="currentColor" d="M0 0h4v4z"/>`, scene.RGB(0, 0, 0))
		if p := items[0].(*scene.Path); p.Fill != scene.RGB(0, 255, 0) || p.Stroke != scene.RGB(0, 255, 0) {
			t.Fatalf("fill %v stroke %v", p.Fill, p.Stroke)
		}
	}
}

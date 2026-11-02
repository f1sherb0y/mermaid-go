package config

import "testing"

func TestPreprocess(t *testing.T) {
	doc, err := Preprocess("---\ntitle: Node\nconfig:\n  theme: forest\n  flowchart:\n    curve: linear\n---\n%%{init: {'look': 'handDrawn'}}%%\n%% a comment\nflowchart LR\n  id\n")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "Node" || doc.Code != "flowchart LR\n  id\n" {
		t.Fatalf("doc %+v", doc)
	}
	c := Defaults().With(doc.Overrides)
	if c.Str("theme", "") != "forest" || c.Str("flowchart.curve", "") != "linear" || c.Num("flowchart.nodeSpacing", 0) != 50 || c.Str("look", "") != "handDrawn" {
		t.Errorf("config %v %v %v %v", c.Get("theme"), c.Get("flowchart.curve"), c.Get("flowchart.nodeSpacing"), c.Get("look"))
	}
	theme, look, layout := Appearance("flowchart", nil, nil)
	if theme != "redux-color" || look != "neo" || layout != "elk" {
		t.Errorf("flowchart appearance %s %s %s", theme, look, layout)
	}
	theme, _, _ = Appearance("pie", nil, nil)
	if theme != "default" {
		t.Errorf("pie theme %s", theme)
	}
	theme, look, _ = Appearance("flowchart", doc.Overrides, nil)
	if theme != "forest" || look != "handDrawn" {
		t.Errorf("overridden %s %s", theme, look)
	}
}

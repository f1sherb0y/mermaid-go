package architecture

import (
	"strings"
	"testing"
)

// Cases from mermaid's architecture.spec.ts and the parser package's
// architecture.test.ts.

func TestParse(t *testing.T) {
	ok := []string{
		"architecture-beta",
		"  architecture-beta  ",
		"\tarchitecture-beta\t",
		"\n        \tarchitecture-beta\n        ",
		"architecture-beta\n            service db\n            ",
	}
	for _, src := range ok {
		if _, err := Parse(src); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}

func TestTitleAndAccessibility(t *testing.T) {
	for _, src := range []string{
		"architecture-beta title sample title",
		"  architecture-beta  title sample title  ",
		"\tarchitecture-beta\ttitle sample title\t",
		"architecture-beta\n            \ttitle sample title\n            ",
		"architecture-beta\n            title sample title",
	} {
		d, err := Parse(src)
		if err != nil || d.Title != "sample title" {
			t.Errorf("%q: title %q, %v", src, d.Title, err)
		}
	}
	d, err := Parse("architecture-beta\n title sample title\n accTitle: sample accTitle\n accDescr: sample accDescr\n")
	if err != nil || d.AccTitle != "sample accTitle" || d.AccDescr != "sample accDescr" {
		t.Errorf("acc: %+v %v", d, err)
	}
	d, err = Parse("architecture-beta\n accDescr {\n    Accessibility Description\n }\n service a")
	if err != nil || d.AccDescr != "Accessibility Description" || len(d.Nodes) != 1 {
		t.Errorf("accDescr block: %+v %v", d, err)
	}
}

func TestTitles(t *testing.T) {
	for in, want := range map[string]string{
		`["Database"]`:         "Database",
		`['Database']`:         "Database",
		`[Database]`:           "Database",
		`["John's Database"]`:  "John's Database",
		`["The \"Main\" API"]`: `The "Main" API`,
		`[采集器]`:                "采集器",
		`[Hello 世界]`:           "Hello 世界",
		`[Сервер]`:             "Сервер",
		`[Structure + Src]`:    "Structure + Src",
		`[Multi-Agent]`:        "Multi-Agent",
		`[A, B, C]`:            "A, B, C",
		`[TCP/IP: v6]`:         "TCP/IP: v6",
		`[100%% done]`:         "100%% done",
	} {
		d, err := Parse("architecture-beta\n group g1(cloud)\n service db(database)" + in + " in g1 %% a comment")
		if err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if got := d.Nodes[0].Title; got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
	d, err := Parse("architecture-beta\n group g1(server)[你好世界]")
	if err != nil || d.Groups[0].Title != "你好世界" || d.Groups[0].Icon != "server" {
		t.Errorf("group title: %+v %v", d, err)
	}
}

func TestStatements(t *testing.T) {
	d, err := Parse(`architecture-beta
    group api(cloud)[API]
    group inner[Inner] in api
    service db(logos:aws-aurora)[Database] in inner
    service txt "Hi there"[Text icon]
    service 20(server)[B]
    service Lambda(server)
    junction j in api
    db:L -- R:txt
    db{group}:B <--> T:20{group}
    txt:R -[a label]-> L:j
    Lambda : T<--B : j
    align row db txt
    align column 20 Lambda j
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Groups) != 2 || d.Groups[1].Parent != d.Groups[0] {
		t.Errorf("groups: %+v", d.Groups)
	}
	var ids []string
	for _, n := range d.Nodes {
		ids = append(ids, n.ID)
	}
	if got := strings.Join(ids, " "); got != "db txt 20 Lambda j" {
		t.Errorf("nodes %q", got)
	}
	if n := d.Nodes[0]; n.Icon != "logos:aws-aurora" || n.Parent != d.Groups[1] {
		t.Errorf("db: %+v", n)
	}
	if n := d.Nodes[1]; n.IconText != "Hi there" || n.Icon != "" {
		t.Errorf("txt: %+v", n)
	}
	if n := d.Nodes[4]; !n.Junction || n.Parent != d.Groups[0] {
		t.Errorf("j: %+v", n)
	}
	e := d.Edges[1]
	if !e.FromGroup || !e.ToGroup || !e.FromArrow || !e.ToArrow || e.FromSide != 'B' || e.ToSide != 'T' {
		t.Errorf("group edge: %+v", e)
	}
	if e := d.Edges[2]; e.Label != "a label" || e.FromArrow || !e.ToArrow {
		t.Errorf("labelled edge: %+v", e)
	}
	if e := d.Edges[3]; e.From.ID != "Lambda" || !e.FromArrow || e.ToArrow || e.FromSide != 'T' {
		t.Errorf("spaced edge: %+v", e)
	}
	if len(d.Hints) != 2 || !d.Hints[1].Column || len(d.Hints[1].Members) != 3 {
		t.Errorf("hints: %+v", d.Hints)
	}
}

func TestErrors(t *testing.T) {
	for src, want := range map[string]string{
		// A parent that does not exist (mermaid's prototype-pollution case).
		"group m(cloud)[Marker]\n service a(server)[A] in __proto__\n service b(server)[B] in m\n a:R -- L:b": "parent does not exist",
		"service a(server)[A]\n service b\n align row a b ghost":                                              "ghost",
		"service a(server)[A]\n align row a a":                                                                "more than once",
		"service row(database)[Reserved]":                                                                     "keyword",
		"service align(database)[Reserved]":                                                                   "keyword",
		"service column(database)[Reserved]":                                                                  "keyword",
		"group g1[Group]\n service svc(server) in g1\n junction svc":                                          "already in use",
		"group g1[Group]\n junction g1":                                                                       "already in use",
		"junction j1 in nonexistent":                                                                          "parent does not exist",
		"service app(server)\n junction j1 in app":                                                            "not a group",
		"junction j1 in j1":                                                                                   "within itself",
		"group g in g":                                                                                        "within itself",
		"group a in b\n group b":                                                                              "parent does not exist",
		"service a\n a:R -- L:b":                                                                              "does not exist",
		"group g\n service a in g\n service b in g\n a{group}:R -- L:b":                                       "same group",
		"group g\n service a\n a:R -- L:g":                                                                    "is a group",
		"service a\n a:X -- L:a":                                                                              "can't read",
		"service a\n align row a":                                                                             "can't read",
	} {
		_, err := Parse("architecture-beta\n" + src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", src, err, want)
		}
	}
	if _, err := Parse("graph TD"); err == nil {
		t.Error("not architecture: no error")
	}
}

// TestAccepted: mermaid's lexer reads an id starting with L, R, T or B as
// a direction and fails; these are ids here.
func TestAccepted(t *testing.T) {
	d, err := Parse("architecture-beta\n group Lambda(cloud)\n service Rabbit(server) in Lambda\n service Tasks\n service B\n Rabbit:R -- L:Tasks\n Tasks:B -- T:B\n align row rowspan columnar")
	if err == nil {
		t.Fatalf("align with unknown members: no error")
	}
	d, err = Parse("architecture-beta\n group Lambda(cloud)\n service Rabbit(server) in Lambda\n service Tasks\n service B\n service rowspan\n service columnar\n Rabbit:R -- L:Tasks\n Tasks:B -- T:B\n align row rowspan columnar")
	if err != nil {
		t.Fatal(err)
	}
	if d.Edges[1].To.ID != "B" || d.Hints[0].Members[0].ID != "rowspan" {
		t.Errorf("%+v", d)
	}
}

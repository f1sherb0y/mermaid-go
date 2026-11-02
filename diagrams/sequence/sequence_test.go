package sequence

import (
	"fmt"
	"strings"
	"testing"
)

var kindNames = map[Kind]string{
	Solid: "Solid", Dotted: "Dotted", Note: "Note", SolidCross: "SolidCross", DottedCross: "DottedCross",
	SolidOpen: "SolidOpen", DottedOpen: "DottedOpen", LoopStart: "LoopStart", LoopEnd: "LoopEnd",
	AltStart: "AltStart", AltElse: "AltElse", AltEnd: "AltEnd", OptStart: "OptStart", OptEnd: "OptEnd",
	ActiveStart: "ActiveStart", ActiveEnd: "ActiveEnd", ParStart: "ParStart", ParAnd: "ParAnd", ParEnd: "ParEnd",
	RectStart: "RectStart", RectEnd: "RectEnd", SolidPoint: "SolidPoint", DottedPoint: "DottedPoint",
	Autonumber: "Autonumber", CriticalStart: "CriticalStart", CriticalOption: "CriticalOption",
	CriticalEnd: "CriticalEnd", BreakStart: "BreakStart", BreakEnd: "BreakEnd", ParOverStart: "ParOverStart",
	BidirectionalSolid: "BidirectionalSolid", BidirectionalDotted: "BidirectionalDotted",
	SolidTop: "SolidTop", SolidBottom: "SolidBottom", StickTop: "StickTop", StickBottom: "StickBottom",
	SolidTopReverse: "SolidTopReverse", SolidBottomReverse: "SolidBottomReverse",
	StickTopReverse: "StickTopReverse", StickBottomReverse: "StickBottomReverse",
	SolidTopDotted: "SolidTopDotted", StickBottomReverseDotted: "StickBottomReverseDotted",
	CentralConnection: "CentralConnection", CentralConnectionReverse: "CentralConnectionReverse",
	CentralConnectionDual: "CentralConnectionDual",
}

// summary writes a message as "Kind from>to: text", with its flags.
func summary(m *Message) string {
	s := kindNames[m.Kind]
	if m.From != "" || m.To != "" {
		s += " " + m.From + ">" + m.To
	}
	if m.Text != "" {
		s += ": " + m.Text
	}
	if m.Wrap {
		s += " [wrap]"
	}
	if m.Activate {
		s += " [activate]"
	}
	if m.Central != 0 {
		s += " [" + kindNames[m.Central] + "]"
	}
	if m.Kind == Autonumber {
		s += fmt.Sprintf(" %g %g %v", m.Start, m.Step, m.Visible)
	}
	return s
}

// TestParse checks the parser on cases from mermaid's sequenceDiagram.spec.js
// and the syntax docs.
func TestParse(t *testing.T) {
	for _, c := range []struct {
		name, src string
		actors    string // id=description:type, in order
		messages  []string
	}{
		{"basic", "sequenceDiagram\nAlice->Bob:Hello Bob, how are you?\nNote right of Bob: Bob thinks\nBob-->Alice: I am good thanks!",
			"Alice=Alice:participant Bob=Bob:participant",
			[]string{"SolidOpen Alice>Bob: Hello Bob, how are you?", "Note Bob>Bob: Bob thinks", "DottedOpen Bob>Alice: I am good thanks!"}},
		{"spaces and dashes in names", "sequenceDiagram\nAlice-in-Wonderland->Bob:Hello Bob, how are - you?\nBob-->Alice-in-Wonderland:I am good thanks!",
			"Alice-in-Wonderland=Alice-in-Wonderland:participant Bob=Bob:participant",
			[]string{"SolidOpen Alice-in-Wonderland>Bob: Hello Bob, how are - you?", "DottedOpen Bob>Alice-in-Wonderland: I am good thanks!"}},
		{"equals in names", "sequenceDiagram\nparticipant Alice=Wonderland\nparticipant Bob\nAlice=Wonderland->Bob:Hi",
			"Alice=Wonderland=Alice=Wonderland:participant Bob=Bob:participant",
			[]string{"SolidOpen Alice=Wonderland>Bob: Hi"}},
		{"aliases", "sequenceDiagram\nparticipant A as Alice\nactor B as Bob\nA->B:Hello\nB->>A: Hi",
			"A=Alice:participant B=Bob:actor",
			[]string{"SolidOpen A>B: Hello", "Solid B>A: Hi"}},
		{"redeclared", "sequenceDiagram\nparticipant Alice\nparticipant Alice\nparticipant Alice\n",
			"Alice=Alice:participant", nil},
		{"arrows", "sequenceDiagram\nA-)B: a\nA--)B: b\nA->>B: c\nA-->>B: d\nA-xB: e\nA--xB: f\nA<<->>B: g\nA<<-->>B: h\nA-|\\B: i\nA//-B: j\nA--|\\B: k\nA\\\\--B: l",
			"A=A:participant B=B:participant",
			[]string{"SolidPoint A>B: a", "DottedPoint A>B: b", "Solid A>B: c", "Dotted A>B: d", "SolidCross A>B: e",
				"DottedCross A>B: f", "BidirectionalSolid A>B: g", "BidirectionalDotted A>B: h", "SolidTop A>B: i",
				"StickTopReverse A>B: j", "SolidTopDotted A>B: k", "StickBottomReverseDotted A>B: l"}},
		{"activations", "sequenceDiagram\nAlice-->>Bob: Hello\nactivate Bob\nAlice->>+Bob: again\nBob-->>-Alice: ok\ndeactivate Bob",
			"Alice=Alice:participant Bob=Bob:participant",
			[]string{"Dotted Alice>Bob: Hello", "ActiveStart Bob>", "Solid Alice>Bob: again [activate]", "ActiveStart Bob>",
				"Dotted Bob>Alice: ok", "ActiveEnd Bob>", "ActiveEnd Bob>"}},
		{"central", "sequenceDiagram\nAlice->>()John: a\nAlice()->>John: b\nJohn()<<->>()Alice: c",
			"Alice=Alice:participant John=John:participant",
			[]string{"Solid Alice>John: a [activate] [CentralConnection]", "CentralConnection John>",
				"Solid Alice>John: b [CentralConnectionReverse]", "CentralConnectionReverse Alice>",
				"BidirectionalSolid John>Alice: c [activate] [CentralConnectionDual]", "CentralConnection Alice>", "CentralConnectionReverse John>"}},
		{"comments and semicolons", "sequenceDiagram;Alice->Bob: Hello;%% comment\n# another\nBob-->Alice: ok",
			"Alice=Alice:participant Bob=Bob:participant",
			[]string{"SolidOpen Alice>Bob: Hello", "DottedOpen Bob>Alice: ok"}},
		{"empty message", "sequenceDiagram\nAlice->>Bob:\nBob->>Alice:Got it!",
			"Alice=Alice:participant Bob=Bob:participant",
			[]string{"Solid Alice>Bob", "Solid Bob>Alice: Got it!"}},
		{"wrap", "sequenceDiagram\nAlice->Bob:wrap: Hello\nNote over Alice,Bob: wrap:Shared\nBob->Alice: nowrap:Bye",
			"Alice=Alice:participant Bob=Bob:participant",
			[]string{"SolidOpen Alice>Bob: Hello [wrap]", "Note Alice>Bob: Shared [wrap]", "SolidOpen Bob>Alice: Bye"}},
		{"blocks", "sequenceDiagram\nloop Every minute\nA->B: x\nend\nalt is sick\nB->A: no\nelse is well\nB->A: yes\nelse\nB->A: ?\nend\nopt\nA->B: y\nend\ncritical c\nA->B: z\noption o\nA->A: w\nend\nbreak b\nA->B: v\nend\npar p\nA->B: 1\nand q\nA->B: 2\nend\npar_over po\nA->B: 3\nend\nrect rgb(1, 2, 3)\nA->B: 4\nend",
			"A=A:participant B=B:participant",
			[]string{"LoopStart: Every minute", "SolidOpen A>B: x", "LoopEnd", "AltStart: is sick", "SolidOpen B>A: no",
				"AltElse: is well", "SolidOpen B>A: yes", "AltElse", "SolidOpen B>A: ?", "AltEnd", "OptStart", "SolidOpen A>B: y", "OptEnd",
				"CriticalStart: c", "SolidOpen A>B: z", "CriticalOption: o", "SolidOpen A>A: w", "CriticalEnd",
				"BreakStart: b", "SolidOpen A>B: v", "BreakEnd", "ParStart: p", "SolidOpen A>B: 1", "ParAnd: q", "SolidOpen A>B: 2", "ParEnd",
				"ParOverStart: po", "SolidOpen A>B: 3", "ParEnd", "RectStart: rgb(1, 2, 3)", "SolidOpen A>B: 4", "RectEnd"}},
		{"autonumber", "sequenceDiagram\nautonumber\nA->B: 1\nautonumber 10 5\nA->B: 2\nautonumber 1.25\nautonumber off\nA->B: 3",
			"A=A:participant B=B:participant",
			[]string{"Autonumber 0 0 true", "SolidOpen A>B: 1", "Autonumber 10 5 true", "SolidOpen A>B: 2", "Autonumber 1.25 1 true",
				"Autonumber 0 0 false", "SolidOpen A>B: 3"}},
		{"title, accessibility, links", "sequenceDiagram\ntitle: Diagram Title\naccTitle: A title\naccDescr {\nmultiline\n}\nparticipant A\nlink A: Dashboard @ https://x.com\nlinks A: {\"Wiki\": \"https://y.com\"}\nA->A: self",
			"A=A:participant", []string{"SolidOpen A>A: self"}},
		{"types", "sequenceDiagram\nparticipant A@{ \"type\": \"boundary\" }\nactor B@{ type: database }\nparticipant C@{ \"type\": \"queue\", \"alias\": \"Jobs\" }\nparticipant D@{ \"alias\": \"Inner\" } as Outer\nactor lead-actor@{ \"type\" : \"control\" } as Kebab-Cased",
			"A=A:boundary B=B:database C=Jobs:queue D=Outer:participant lead-actor=Kebab-Cased:control", nil},
		{"create and destroy", "sequenceDiagram\nAlice->>Bob: Hello\ncreate participant Carl\nAlice->>Carl: Hi\ndestroy Carl\nAlice-xCarl: bye\ndestroy Bob\nBob->>Alice: ok",
			"Alice=Alice:participant Bob=Bob:participant Carl=Carl:participant",
			[]string{"Solid Alice>Bob: Hello", "Solid Alice>Carl: Hi", "SolidCross Alice>Carl: bye", "Solid Bob>Alice: ok"}},
		{"entities", "sequenceDiagram\nA->>B: I #9829; you #infin; times #35;", "A=A:participant B=B:participant",
			[]string{"Solid A>B: I ♥ you ∞ times #"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, err := Parse(c.src, false)
			if err != nil {
				t.Fatal(err)
			}
			var actors []string
			for _, a := range d.Actors {
				actors = append(actors, a.ID+"="+a.Description+":"+a.Type)
			}
			if got := strings.Join(actors, " "); got != c.actors {
				t.Errorf("actors\n got %s\nwant %s", got, c.actors)
			}
			var msgs []string
			for _, m := range d.Messages {
				msgs = append(msgs, summary(m))
			}
			if got, want := strings.Join(msgs, "\n"), strings.Join(c.messages, "\n"); got != want {
				t.Errorf("messages\n got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestParseDetails(t *testing.T) {
	d, err := Parse("sequenceDiagram\ntitle My title\nbox Purple Alice & John\nparticipant A\nparticipant J\nend\nbox Another Group\nparticipant B\nend\nbox rgb(10, 20, 30)\nparticipant C\nend\ncreate participant X\nA->>X: hi\ndestroy X\nX->>A: bye", false)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "My title" {
		t.Errorf("title %q", d.Title)
	}
	var boxes []string
	for _, b := range d.Boxes {
		boxes = append(boxes, fmt.Sprintf("%s|%s|%s", b.Fill, b.Name, strings.Join(b.Actors, ",")))
	}
	if got, want := strings.Join(boxes, " "), "Purple|Alice & John|A,J transparent|Another Group|B rgb(10, 20, 30)||C"; got != want {
		t.Errorf("boxes\n got %s\nwant %s", got, want)
	}
	x := d.Actor("X")
	if x.Created != 0 || x.Destroyed != 1 || d.Actor("A").Created != -1 {
		t.Errorf("created %d destroyed %d", x.Created, x.Destroyed)
	}
	if a, j := d.Actor("A"), d.Actor("J"); a.Next != "J" || j.Prev != "A" {
		t.Errorf("A.Next %q J.Prev %q", a.Next, j.Prev)
	}
	// The diagram's wrap setting is the default.
	d, _ = Parse("sequenceDiagram\nA->>B: x\nA->>B: nowrap: y", true)
	if !d.Messages[0].Wrap || d.Messages[1].Wrap || !d.Actor("A").Wrap {
		t.Error("autoWrap not applied")
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{
		"sequenceDiagram\nAlice->>Bob: Hello\ndeactivate Bob",               // inactive participant
		"sequenceDiagram\nAlice->>Bob: Hello\nBob-->>-Alice: x",             // the same, with -
		"sequenceDiagram\nparticipant SAS MyService <br> description",       // < in a name
		"sequenceDiagram\nparticipant X asAAAA:AAAA",                        // as without a space
		"sequenceDiagram\nparticipant C@{ \"type\" \"control\" }\nC->>C: a", // bad config
		"sequenceDiagram\nparticipant E@{ \"type\": \"entity\"\nE->>E: a",   // unclosed config
		"sequenceDiagram\ncreate participant C\nA->>B: not creating C",
		"sequenceDiagram\nA->>B: x\ncreate participant A\nA->>B: y", // created twice
		"sequenceDiagram\nloop x\nA->>B: y",                         // no end
		"sequenceDiagram\nA->>B: y\nend",
		"sequenceDiagram\nbox Aqua\nparticipant A\nend\nbox Blue\nparticipant A\nend", // two boxes
		"sequenceDiagram\nautonumber 1.234\nA->>B: x",
		"flowchart\nA-->B",
	} {
		if _, err := Parse(src, false); err == nil {
			t.Errorf("no error for %q", src)
		}
	}
}

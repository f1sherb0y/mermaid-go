package gantt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/f1sherb0y/mermaid-go"
	"github.com/f1sherb0y/mermaid-go/text"
)

func day(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }

func TestDates(t *testing.T) {
	for _, c := range []struct {
		s, format string
		want      time.Time
		ok        bool
	}{
		{"2014-01-06", "YYYY-MM-DD", day(2014, 1, 6), true},
		{"2014-1-6", "YYYY-MM-DD", time.Time{}, false}, // strict
		{"2014-02-30", "YYYY-MM-DD", time.Time{}, false},
		{"17:49", "HH:mm", refDay.Add(17*time.Hour + 49*time.Minute), true},
		{"1900", "YYYY", day(1900, 1, 1), true},
		{"71", "X", time.Unix(71, 0).UTC(), true},
		{"06/01/2014 3:04 PM", "DD/MM/YYYY h:mm A", day(2014, 1, 6).Add(15*time.Hour + 4*time.Minute), true},
		{"Jan 6th 2014", "MMM Do YYYY", day(2014, 1, 6), true},
	} {
		got, ok := parseDate(c.s, c.format)
		if ok != c.ok || ok && !got.Equal(c.want) {
			t.Errorf("parseDate(%q, %q) = %v %v, want %v %v", c.s, c.format, got, ok, c.want, c.ok)
		}
	}
	if s := formatDate(day(2014, 1, 6), "dddd, MMMM Do YYYY [at] HH:mm"); s != "Monday, January 6th 2014 at 00:00" {
		t.Errorf("formatDate = %q", s)
	}
	for _, c := range []struct {
		from time.Time
		d    string
		want time.Time
	}{
		{day(2024, 1, 31), "1M", day(2024, 2, 29)},
		{day(2024, 1, 1), "1.5d", day(2024, 1, 3)},
		{day(2024, 1, 1), "1w", day(2024, 1, 8)},
		{day(2024, 1, 1), "36h", day(2024, 1, 2).Add(12 * time.Hour)},
	} {
		if got, ok := addDuration(c.from, c.d); !ok || !got.Equal(c.want) {
			t.Errorf("addDuration(%v, %s) = %v", c.from, c.d, got)
		}
	}
}

func TestParse(t *testing.T) {
	g, err := Parse(`gantt
    dateFormat  YYYY-MM-DD
    title       Adding GANTT diagram functionality to mermaid
    excludes    weekends
    section A section
    Completed task            :done,    des1, 2014-01-06,2014-01-08
    Active task               :active,  des2, 2014-01-09, 3d
    Future task               :         des3, after des2, 5d
    section Critical tasks
    Add to mermaid            :until isadded
    Functionality added       :milestone, isadded, 2014-01-25, 0d
    Marker                    :vert, 2014-01-20, 0d`)
	if err != nil {
		t.Fatal(err)
	}
	if g.Title != "Adding GANTT diagram functionality to mermaid" || strings.Join(g.Excludes, ",") != "weekends" || len(g.Tasks) != 6 {
		t.Fatalf("got %+v", g)
	}
	byName := map[string]*Task{}
	for _, tk := range g.Tasks {
		byName[tk.Name] = tk
	}
	// Times as mermaid.js draws gantt/02: the weekend inside "Active task"
	// pushes its end two days on.
	for _, c := range []struct {
		name       string
		start, end time.Time
	}{
		{"Completed task", day(2014, 1, 6), day(2014, 1, 8)},
		{"Active task", day(2014, 1, 9), day(2014, 1, 14)},
		{"Future task", day(2014, 1, 14), day(2014, 1, 21)},
		{"Add to mermaid", day(2014, 1, 21), day(2014, 1, 27)}, // drawn to the 25th
		{"Functionality added", day(2014, 1, 25), day(2014, 1, 25)},
	} {
		tk := byName[c.name]
		if !tk.Start.Equal(c.start) || !tk.End.Equal(c.end) {
			t.Errorf("%s: %v..%v, want %v..%v", c.name, tk.Start, tk.End, c.start, c.end)
		}
	}
	if r := byName["Add to mermaid"].RenderEnd; r == nil || !r.Equal(day(2014, 1, 25)) {
		t.Errorf("Add to mermaid is drawn to %v", r)
	}
	if a := byName["Active task"]; !a.Active || a.ID != "des2" || a.Section != "A section" || a.Order != 1 {
		t.Errorf("active task = %+v", a)
	}
	if m := byName["Marker"]; !m.Vert || m.Order != -1 {
		t.Errorf("marker = %+v", m)
	}
	if _, err := Parse("gantt\n  dateFormat YYYY-MM-DD\n  a : 2014-13-45, 1d"); err == nil {
		t.Error("an invalid date parsed")
	}
}

// TestCorpus renders every gantt chart of the corpus.
func TestCorpus(t *testing.T) {
	fs := text.NewFonts()
	if err := fs.AddFile("Recursive Variable", "../../../../data/ref/fonts/RecursiveSansLinear-VF.ttf"); err != nil {
		t.Skip("no fonts:", err)
	}
	fs.AddFile("trebuchet ms", "C:/Windows/Fonts/trebuc.ttf")
	files, _ := filepath.Glob("../../testdata/corpus/gantt/*.mmd")
	if len(files) == 0 {
		t.Fatal("no corpus")
	}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		s, err := mermaid.Render(string(b), mermaid.Options{Fonts: fs})
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if len(s.Items) == 0 || !(s.ViewBox.W > 0 && s.ViewBox.H > 0) || strings.Contains(s.SVG(), "NaN") {
			t.Errorf("%s: empty or broken drawing (view box %+v)", f, s.ViewBox)
		}
	}
}

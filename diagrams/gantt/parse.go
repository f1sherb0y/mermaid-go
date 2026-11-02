package gantt

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Gantt is a parsed gantt chart with its tasks' times worked out.
type Gantt struct {
	Title        string
	DateFormat   string
	AxisFormat   string
	TickInterval string
	TopAxis      bool
	Weekday      string
	Weekend      string // "saturday" (Saturday and Sunday) or "friday"
	Includes     []string
	Excludes     []string
	Sections     []string
	Tasks        []*Task
}

// Task is a bar, a milestone or a vertical marker.
type Task struct {
	ID, Name, Section string
	Start, End        time.Time
	// RenderEnd, when set, is where the bar ends: before the excluded days
	// that pushed End later.
	RenderEnd                           *time.Time
	Active, Done, Crit, Milestone, Vert bool
	// Order is the task's row (-1 for markers).
	Order int

	start, end string // the raw start and end
	prev       string // the task before it, for a start left out
	resolved   bool
}

// keywordRe matches the statements (gantt.jison's lexer, case-insensitive).
// Groups: 1 a keyword with its value 2; 3 a flag; 4 a weekday; 5 a
// weekend; 6 the header and the statements that are dropped.
var keywordRe = regexp.MustCompile(`(?i)^(?:(dateFormat|axisFormat|tickInterval|includes|excludes|todayMarker|title|accDescription|section)\s(.*)$|(inclusiveEndDates|topAxis)\b|weekday\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\b|weekend\s+(friday|saturday)\b|(gantt|click|accTitle\s*:|accDescr\s*:|accDescr\s*\{))`)

// Parse reads a gantt chart (gantt.jison) and works out its tasks' times.
func Parse(code string) (*Gantt, error) {
	g := &Gantt{Weekday: "sunday", Weekend: "saturday"}
	started, inDescr := false, false
	section := ""
	inclusive := false
	taskCount := 0
	lastID := ""
	for n, raw := range strings.Split(code, "\n") {
		if inDescr {
			if strings.Contains(raw, "}") {
				inDescr = false
			}
			continue
		}
		l := strings.TrimSpace(raw)
		if l == "" || strings.HasPrefix(l, "%%") || strings.HasPrefix(l, "#") {
			continue
		}
		m := keywordRe.FindStringSubmatch(l)
		if !started {
			if m == nil || !strings.EqualFold(m[6], "gantt") {
				return nil, fmt.Errorf("gantt: line %d: expected gantt", n+1)
			}
			started = true
			continue
		}
		upTo := func(s string) string { // a value ends at # or ;
			if i := strings.IndexAny(s, "#;"); i >= 0 {
				s = s[:i]
			}
			return s
		}
		if m != nil {
			kw := strings.ToLower(m[1] + m[3] + m[6])
			switch {
			case kw == "dateformat":
				g.DateFormat = strings.TrimSpace(upTo(m[2]))
			case kw == "axisformat":
				g.AxisFormat = strings.TrimSpace(upTo(m[2]))
			case kw == "tickinterval":
				g.TickInterval = strings.TrimSpace(upTo(m[2]))
			case kw == "includes":
				g.Includes = mergeTokens(g.Includes, upTo(m[2]))
			case kw == "excludes":
				g.Excludes = mergeTokens(g.Excludes, upTo(m[2]))
			case kw == "title":
				g.Title = strings.TrimSpace(m[2])
			case kw == "section":
				section = strings.TrimSpace(m[2])
				g.Sections = append(g.Sections, section)
			case kw == "inclusiveenddates":
				inclusive = true
			case kw == "topaxis":
				g.TopAxis = true
			case m[4] != "":
				g.Weekday = strings.ToLower(m[4])
			case m[5] != "":
				g.Weekend = strings.ToLower(m[5])
			case strings.HasPrefix(kw, "accdescr") && strings.HasSuffix(kw, "{"):
				inDescr = !strings.Contains(l, "}")
			}
			// todayMarker, accDescription, accTitle, accDescr, click: dropped.
			continue
		}
		name, data, ok := strings.Cut(l, ":")
		if !ok {
			return nil, fmt.Errorf("gantt: line %d: expected `task : data`, got %q", n+1, l)
		}
		data = upTo(data)
		t := &Task{Name: strings.TrimSpace(name), Section: section, prev: lastID}
		fields := strings.Split(data, ",")
		// Leading tags: active, done, crit, milestone, vert.
		for len(fields) > 0 {
			f := strings.TrimSpace(fields[0])
			switch f {
			case "active":
				t.Active = true
			case "done":
				t.Done = true
			case "crit":
				t.Crit = true
			case "milestone":
				t.Milestone = true
			case "vert":
				t.Vert = true
			default:
				goto tagsDone
			}
			fields = fields[1:]
		}
	tagsDone:
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		newID := func() string { taskCount++; return fmt.Sprintf("task%d", taskCount) }
		switch len(fields) {
		case 1:
			t.ID, t.end = newID(), fields[0]
		case 2:
			t.ID, t.start, t.end = newID(), fields[0], fields[1]
		case 3:
			t.ID, t.start, t.end = fields[0], fields[1], fields[2]
		default:
			t.ID = newID()
		}
		t.Order = -1
		if !t.Vert {
			t.Order = len(nonVert(g.Tasks))
		}
		g.Tasks = append(g.Tasks, t)
		lastID = t.ID
	}
	if !started {
		return nil, fmt.Errorf("gantt: empty")
	}
	if err := g.compile(inclusive); err != nil {
		return nil, err
	}
	return g, nil
}

func nonVert(ts []*Task) []*Task {
	var out []*Task
	for _, t := range ts {
		if !t.Vert {
			out = append(out, t)
		}
	}
	return out
}

func mergeTokens(have []string, s string) []string {
	for _, t := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		if !contains(have, t) {
			have = append(have, t)
		}
	}
	return have
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func (g *Gantt) task(id string) *Task {
	var found *Task
	for _, t := range g.Tasks {
		if t.ID == id {
			found = t // the last one with the id, as mermaid's index keeps
		}
	}
	return found
}

var (
	afterRe = regexp.MustCompile(`^after\s+([\d\w\- ]+)`)
	untilRe = regexp.MustCompile(`^until\s+([\d\w\- ]+)`)
)

// compile works the tasks' times out, in passes, as tasks can refer to
// later ones.
func (g *Gantt) compile(inclusive bool) error {
	for pass := 0; pass < 11; pass++ {
		done := true
		for _, t := range g.Tasks {
			if t.resolved {
				continue
			}
			ok, err := g.resolve(t, inclusive)
			if err != nil {
				return err
			}
			done = done && ok
		}
		if done {
			break
		}
	}
	// Tasks never worked out (a cycle of references) are left out.
	var kept []*Task
	for _, t := range g.Tasks {
		if t.resolved {
			kept = append(kept, t)
		}
	}
	g.Tasks = kept
	return nil
}

func (g *Gantt) resolve(t *Task, inclusive bool) (bool, error) {
	var start time.Time
	if t.start == "" {
		p := g.task(t.prev)
		if p == nil {
			start = refDay
		} else if !p.resolved {
			return false, nil
		} else {
			start = p.End
		}
	} else {
		var ok bool
		var err error
		if start, ok, err = g.startDate(t.start); err != nil || !ok {
			return false, err
		}
	}
	end, ok := g.endDate(start, t.end, inclusive)
	if !ok {
		return false, nil
	}
	t.Start, t.End, t.resolved = start, end, true
	if _, manual := parseDate(t.end, "YYYY-MM-DD"); len(g.Excludes) > 0 && !manual {
		t.End, t.RenderEnd = g.skipExcluded(start.AddDate(0, 0, 1), end)
	}
	return true, nil
}

// startDate is a task's start: a timestamp, `after id ...`, a date in the
// date format, or an ISO date; ok is false while a referred task is not
// worked out yet.
func (g *Gantt) startDate(s string) (time.Time, bool, error) {
	if (g.DateFormat == "x" || g.DateFormat == "X") && isDigits(s) {
		var ms int64
		fmt.Sscan(s, &ms)
		return time.UnixMilli(ms).UTC(), true, nil
	}
	if m := afterRe.FindStringSubmatch(s); m != nil {
		var latest *Task
		for _, id := range strings.Fields(m[1]) {
			if t := g.task(id); t != nil {
				if !t.resolved {
					return time.Time{}, false, nil
				}
				if latest == nil || t.End.After(latest.End) {
					latest = t
				}
			}
		}
		if latest != nil {
			return latest.End, true, nil
		}
		return refDay, true, nil
	}
	if t, ok := parseDate(s, g.DateFormat); ok {
		return t, true, nil
	}
	if t, ok := jsDate(s); ok {
		return t, true, nil
	}
	return time.Time{}, false, dateErr(s)
}

// endDate is a task's end: `until id ...`, a date, or a duration from the
// start (none: the start).
func (g *Gantt) endDate(start time.Time, s string, inclusive bool) (time.Time, bool) {
	if m := untilRe.FindStringSubmatch(s); m != nil {
		var earliest *Task
		for _, id := range strings.Fields(m[1]) {
			if t := g.task(id); t != nil {
				if !t.resolved {
					return time.Time{}, false
				}
				if earliest == nil || t.Start.Before(earliest.Start) {
					earliest = t
				}
			}
		}
		if earliest != nil {
			return earliest.Start, true
		}
		return refDay, true
	}
	if t, ok := parseDate(s, g.DateFormat); ok {
		if inclusive {
			t = t.AddDate(0, 0, 1)
		}
		return t, true
	}
	end, _ := addDuration(start, s)
	return end, true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// excluded reports a day the excludes take out (and the includes don't
// put back).
func (g *Gantt) excluded(t time.Time) bool {
	formatted, dateOnly := formatDate(t, g.DateFormat), formatDate(t, "YYYY-MM-DD")
	if contains(g.Includes, formatted) || contains(g.Includes, dateOnly) {
		return false
	}
	if contains(g.Excludes, "weekends") {
		first := time.Saturday
		if g.Weekend == "friday" {
			first = time.Friday
		}
		if wd := t.Weekday(); wd == first || wd == (first+1)%7 {
			return true
		}
	}
	return contains(g.Excludes, strings.ToLower(t.Weekday().String())) ||
		contains(g.Excludes, formatted) || contains(g.Excludes, dateOnly)
}

// skipExcluded pushes a task's end past the excluded days it spans; the
// bar is drawn to where the last counted day ends.
func (g *Gantt) skipExcluded(day, end time.Time) (time.Time, *time.Time) {
	var render *time.Time
	invalid := false
	limit := end.AddDate(0, 0, 10000)
	for !day.After(end) {
		if !invalid {
			e := end
			render = &e
		}
		invalid = g.excluded(day)
		if invalid {
			end = end.AddDate(0, 0, 1)
			if end.After(limit) {
				break
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return end, render
}

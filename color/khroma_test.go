package color

import (
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"testing"
)

// testdata/khroma.json.gz: khroma (as bundled into mermaid 12.2.0) run by tools/theme/khroma.mjs.
func TestKhroma(t *testing.T) {
	f, err := os.Open("testdata/khroma.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Fn   string            `json:"fn"`
		Args []json.RawMessage `json:"args"`
		Out  *string           `json:"out"`
		Num  *string           `json:"num"`
		Err  string            `json:"err"`
	}
	if err := json.NewDecoder(z).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	str := func(m json.RawMessage) string { var s string; json.Unmarshal(m, &s); return s }
	num := func(m json.RawMessage) float64 {
		var f *float64
		json.Unmarshal(m, &f)
		if f == nil {
			return math.NaN() // undefined
		}
		return *f
	}
	chans := map[string]Channel{"r": R, "g": G, "b": B, "h": H, "s": S, "l": L, "a": A}
	fails := 0
	for _, c := range cases {
		var out string
		var err error
		boolOut := func(b bool, e error) { out, err = map[bool]string{true: "true", false: "false"}[b], e }
		numOut := func(f float64, e error) { out, err = FormatNumber(f), e }
		switch c.Fn {
		case "darken":
			out, err = Darken(str(c.Args[0]), num(c.Args[1]))
		case "lighten":
			out, err = Lighten(str(c.Args[0]), num(c.Args[1]))
		case "transparentize":
			out, err = Transparentize(str(c.Args[0]), num(c.Args[1]))
		case "adjust":
			var d map[string]float64
			json.Unmarshal(c.Args[1], &d)
			out, err = Adjust(str(c.Args[0]), Delta{d["r"], d["g"], d["b"], d["h"], d["s"], d["l"], d["a"]})
		case "invert":
			w := 100.0
			if len(c.Args) > 1 {
				w = num(c.Args[1])
			}
			out, err = Invert(str(c.Args[0]), w)
		case "mix":
			out, err = Mix(str(c.Args[0]), str(c.Args[1]), num(c.Args[2]))
		case "rgba":
			if len(c.Args) == 2 {
				out, err = SetAlpha(str(c.Args[0]), num(c.Args[1]))
			} else {
				out = RGBA(num(c.Args[0]), num(c.Args[1]), num(c.Args[2]), num(c.Args[3]))
			}
		case "channel":
			numOut(ChannelOf(str(c.Args[0]), chans[str(c.Args[1])]))
		case "luminance":
			numOut(Luminance(str(c.Args[0])))
		case "is_dark":
			boolOut(IsDark(str(c.Args[0])))
		case "is_light":
			boolOut(IsLight(str(c.Args[0])))
		default:
			t.Fatalf("unknown fn %q", c.Fn)
		}
		var want string
		switch {
		case c.Out != nil:
			want = *c.Out
		case c.Num != nil:
			want = *c.Num
		}
		if (c.Err != "") != (err != nil) || err == nil && out != want {
			if fails++; fails <= 30 {
				t.Errorf("%s(%s): got %q, %v; want %q, %q", c.Fn, c.Args, out, err, want, c.Err)
			}
		}
	}
	if fails > 0 {
		t.Errorf("%d of %d cases differ", fails, len(cases))
	}
	t.Logf("%d cases", len(cases))
}

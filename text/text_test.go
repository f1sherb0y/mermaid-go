package text

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// testFonts registers what mermaid.live has on Windows: Recursive Sans
// Linear (tools/fonts.py; MERMAID_FONTS names its directory) and the
// system's fallbacks. Skips when the fonts are missing.
func testFonts(t testing.TB) *Fonts {
	dir := os.Getenv("MERMAID_FONTS")
	if dir == "" {
		dir = "../../../data/ref/fonts"
	}
	fs := NewFonts()
	if err := fs.AddFile("Recursive Variable", filepath.Join(dir, "RecursiveSansLinear-VF.ttf")); err != nil {
		t.Skip("no Recursive font:", err)
	}
	for name, path := range map[string]string{
		"arial":           "C:/Windows/Fonts/arial.ttf",
		"Microsoft YaHei": "C:/Windows/Fonts/msyh.ttc",
		"Segoe UI Symbol": "C:/Windows/Fonts/seguisym.ttf",
		"Segoe UI Emoji":  "C:/Windows/Fonts/seguiemj.ttf",
	} {
		if err := fs.AddFile(name, path); err != nil {
			t.Skip("no system font:", err)
		}
	}
	fs.AddFile("arial", "C:/Windows/Fonts/arialbd.ttf")
	fs.Fallback("Microsoft YaHei", "Segoe UI Emoji", "Segoe UI Symbol")
	return fs
}

// chrome.json: text measured in Chrome (node tools/ref.mjs measure
// testdata/measure-in.json testdata/chrome.json).
type measured struct {
	Text   string  `json:"text"`
	Family string  `json:"family"`
	Size   float64 `json:"size"`
	Weight int     `json:"weight"`
	HTMLW  float64 `json:"htmlW"`
	SVGH   float64 `json:"svgH"`
}

func TestWidthsAreChromes(t *testing.T) {
	fs := testFonts(t)
	data, err := os.ReadFile("testdata/chrome.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []measured
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	// Where Chrome differs for reasons outside the font: mermaid.live's
	// webfont comes in unicode-range subsets, some of which (basic Cyrillic,
	// some accented letters) fall to arial there; symbols and emoji take
	// whichever system font the browser picks.
	elsewhere := regexp.MustCompile(`[\x{00C0}-\x{024F}\x{0400}-\x{04FF}\x{1E00}-\x{1EFF}\x{2600}-\x{27BF}\x{1F300}-\x{1FAFF}]`)
	bad := 0
	for _, c := range cases {
		st := Style{Family: c.Family, Size: c.Size, Weight: c.Weight}
		// Labels collapse white space as HTML does before measuring.
		s := strings.Join(strings.Fields(c.Text), " ")
		w := fs.Width(s, st)
		if math.Abs(w-c.HTMLW) > 1.0/64+1e-9 {
			if elsewhere.MatchString(s) {
				t.Logf("%q %v %d: width %.4f, Chrome %.4f (not the font's)", c.Text, c.Size, c.Weight, w, c.HTMLW)
				continue
			}
			bad++
			t.Errorf("%q %v %d: width %.4f, Chrome %.4f", c.Text, c.Size, c.Weight, w, c.HTMLW)
		}
		a, d := fs.Extents(st)
		if a+d != c.SVGH && bad < 40 {
			t.Logf("%q %v: height %v, Chrome %v", c.Text, c.Size, a+d, c.SVGH)
		}
	}
	t.Logf("%d of %d widths off", bad, len(cases))
}

func TestWrapBreaksAtSpacesAndBetweenCJK(t *testing.T) {
	fs := testFonts(t)
	st := Style{Family: `"Recursive Variable", arial, sans-serif`, Size: 14}
	got := fs.Wrap("The quick brown fox jumps over the lazy dog", st, 120)
	for _, l := range got {
		if fs.Width(l, st) > 120 {
			t.Errorf("line %q wider than 120", l)
		}
	}
	if len(got) < 3 {
		t.Errorf("lines %q", got)
	}
	cjk := fs.Wrap("分布式键值存储系统的分片和复制", st, 60)
	if len(cjk) < 3 || fs.Width(cjk[0], st) > 60 {
		t.Errorf("CJK lines %q", cjk)
	}
}

func BenchmarkWidth(b *testing.B) {
	fs := testFonts(b)
	st := Style{Family: `"Recursive Variable", arial, sans-serif`, Size: 14}
	words := []string{"Load Balancer", "API 网关", "Sharded KV Store", "Redis Cluster", "需要统计?"}
	b.Run("cached", func(b *testing.B) {
		for i := 0; b.Loop(); i++ {
			fs.Width(words[i%len(words)], st)
		}
	})
	b.Run("uncached", func(b *testing.B) {
		for i := 0; b.Loop(); i++ {
			fs.widths = map[widthKey]float64{}
			fs.Width(words[i%len(words)], st)
		}
	})
}

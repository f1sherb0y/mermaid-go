// Command mermaid renders Mermaid diagrams to SVG.
//
//	mermaid [-fonts dir] <in.mmd | dir> [out.svg | dir]
//
// With directories, every .mmd under the input becomes .svg (or .err) at the
// same place under the output, for tools/ref.mjs cmp. -fonts is a directory
// with RecursiveSansLinear-VF.ttf (tools/fonts.py); system fonts (arial, a
// CJK font, an emoji font) are added as fallbacks when found.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/f1sherb0y/mermaid-go"
	_ "github.com/f1sherb0y/mermaid-go/diagrams"
	"github.com/f1sherb0y/mermaid-go/text"
)

func main() {
	fontDir := flag.String("fonts", "", "directory with RecursiveSansLinear-VF.ttf")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: mermaid [-fonts dir] <in.mmd | dir> [out.svg | dir]")
		os.Exit(2)
	}
	fonts, err := loadFonts(*fontDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	in := flag.Arg(0)
	st, err := os.Stat(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !st.IsDir() {
		svg, err := renderFile(in, fonts)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if flag.NArg() > 1 {
			err = os.WriteFile(flag.Arg(1), []byte(svg), 0o644)
		} else {
			_, err = os.Stdout.WriteString(svg)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	out := "out"
	if flag.NArg() > 1 {
		out = flag.Arg(1)
	}
	var files []string
	filepath.WalkDir(in, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".mmd") {
			files = append(files, p)
		}
		return nil
	})
	start := time.Now()
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, failed := 0, 0
	jobs := make(chan string)
	for range runtime.NumCPU() {
		wg.Go(func() {
			for f := range jobs {
				rel, _ := filepath.Rel(in, f)
				dst := filepath.Join(out, strings.TrimSuffix(rel, ".mmd"))
				os.MkdirAll(filepath.Dir(dst), 0o755)
				os.Remove(dst + ".svg")
				os.Remove(dst + ".err")
				svg, err := renderFile(f, fonts)
				mu.Lock()
				if err != nil {
					failed++
					os.WriteFile(dst+".err", []byte(err.Error()), 0o644)
				} else {
					ok++
					os.WriteFile(dst+".svg", []byte(svg), 0o644)
				}
				mu.Unlock()
			}
		})
	}
	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	wg.Wait()
	fmt.Printf("rendered %d, failed %d, in %v\n", ok, failed, time.Since(start).Round(time.Millisecond))
}

func renderFile(path string, fonts *text.Fonts) (svg string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s, err := mermaid.Render(string(src), mermaid.Options{Fonts: fonts})
	if err != nil {
		return "", err
	}
	return s.SVG(), nil
}

// loadFonts registers Recursive Variable and whichever system fallbacks are
// there.
func loadFonts(dir string) (*text.Fonts, error) {
	fs := text.NewFonts()
	if dir != "" {
		if err := fs.AddFile("Recursive Variable", filepath.Join(dir, "RecursiveSansLinear-VF.ttf")); err != nil {
			return nil, err
		}
	}
	var fallbacks []string
	for _, f := range []struct{ name, path string }{
		{"arial", "C:/Windows/Fonts/arial.ttf"},
		{"arial", "C:/Windows/Fonts/arialbd.ttf"},
		{"trebuchet ms", "C:/Windows/Fonts/trebuc.ttf"},
		{"trebuchet ms", "C:/Windows/Fonts/trebucbd.ttf"},
		{"verdana", "C:/Windows/Fonts/verdana.ttf"},
		{"Microsoft YaHei", "C:/Windows/Fonts/msyh.ttc"},
		{"Segoe UI Emoji", "C:/Windows/Fonts/seguiemj.ttf"},
		{"Segoe UI Symbol", "C:/Windows/Fonts/seguisym.ttf"},
		{"Noto Sans CJK SC", "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"},
		{"DejaVu Sans", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"},
	} {
		if fs.AddFile(f.name, f.path) == nil {
			switch f.name {
			case "Microsoft YaHei", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Sans CJK SC", "DejaVu Sans":
				fallbacks = append(fallbacks, f.name)
			}
		}
	}
	fs.Fallback(fallbacks...)
	return fs, nil
}

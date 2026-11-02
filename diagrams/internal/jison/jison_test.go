package jison

import (
	"fmt"
	"strings"
	"testing"
)

func kinds(t *testing.T, lx *Lexer[string], src string) string {
	t.Helper()
	toks, err := lx.Lex(src)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	var out []string
	for _, tk := range toks {
		out = append(out, fmt.Sprintf("%s:%s", tk.Kind, tk.Text))
	}
	return strings.Join(out, " ")
}

func TestLexer(t *testing.T) {
	lx := New(Options[string]{EOF: "EOF"},
		On("", `\s+`, ""),
		Lit("", "end", "END"), // whole word only
		On("", `"`, "").Push("str"),
		On("str", `"`, "").Pop(),
		On("str", `[^"]*`, "STR"),
		Lit("", "one", "ONE").NotAhead(`-\w`),
		Lit("", "1", "CARD").Ahead(`\s+x`),
		On("", `\w[\w-]*`, "WORD"),
		On("", `\.`, "DOT").Do(func(s *Scanner[string], tk *Token[string]) { s.Pop(); s.Pop() }), // INITIAL stays
		On("", `@@`, "AT").Do(func(s *Scanner[string], tk *Token[string]) { s.Less(1) }),
		On("", `@`, "AT1"),
		On("", `<<EOF>>`, "END_OF_INPUT"),
	)
	for _, c := range []struct{ src, want string }{
		{`end endless`, "END:end WORD:endless END_OF_INPUT: EOF:"},
		{`"a b" one one-x`, "STR:a b ONE:one WORD:one-x END_OF_INPUT: EOF:"},
		{`1 x 1 y`, "CARD:1 WORD:x WORD:1 WORD:y END_OF_INPUT: EOF:"},
		{`. @@`, "DOT:. AT:@ AT1:@ END_OF_INPUT: EOF:"},
	} {
		if got := kinds(t, lx, c.src); got != c.want {
			t.Errorf("%q: %s, want %s", c.src, got, c.want)
		}
	}
	if _, err := lx.Lex("a ~"); err == nil || !strings.Contains(err.Error(), `line 1: unrecognized text "~"`) {
		t.Errorf("error %v", err)
	}
}

func TestJSRegex(t *testing.T) {
	for _, c := range []struct{ pattern, in string }{
		{`\s`, "\u00a0"}, {`[\s]`, "\u3000"}, {`\S`, "x"}, {`.`, "x"}, {`[^\S\n]+`, "\t\v\u00a0"},
	} {
		if !Regexp(`^(?:` + c.pattern + `)$`).MatchString(c.in) {
			t.Errorf("%s does not match %q", c.pattern, c.in)
		}
	}
	for _, c := range []struct{ pattern, in string }{{`.`, "\n"}, {`.`, "\u2028"}, {`[^\S\n]`, "\n"}, {`\S`, "\u00a0"}} {
		if Regexp(`^(?:` + c.pattern + `)$`).MatchString(c.in) {
			t.Errorf("%s matches %q", c.pattern, c.in)
		}
	}
}

func TestCaseInsensitive(t *testing.T) {
	lx := New(Options[string]{EOF: "EOF", CaseInsensitive: true}, Lit("", "end", "END"), On("", `\s+`, ""))
	if got := kinds(t, lx, "END End"); got != "END:END END:End EOF:" {
		t.Error(got)
	}
}

package flowchart

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/jison"
)

type kind uint8

const (
	tEOF kind = iota
	tNewline
	tSpace
	tSemi
	tComma
	tColon
	tAmp
	tMult
	tBrkt
	tStyleSep // :::
	tNum
	tNodeString
	tUnicodeText
	tMinus
	tDown // v
	tUp   // ^
	tTagStart
	tTagEnd
	tSep
	tQuote
	tStr
	tMdStr
	tText
	tEdgeText
	tLink
	tStartLink
	tLinkID
	tPipe
	tPS
	tPE
	tSQS
	tSQE
	tDiamondStart
	tDiamondStop
	tStadiumStart
	tStadiumEnd
	tSubroutineStart
	tSubroutineEnd
	tCylinderStart
	tCylinderEnd
	tDoubleCircleStart
	tDoubleCircleEnd
	tTrapStart
	tTrapEnd
	tInvTrapStart
	tInvTrapEnd
	tEllipseStart // (-
	tEllipseEnd   // -)
	tVertexWithPropsStart
	tShapeData
	tGraph
	tDir
	tNoDir
	tSubgraph
	tEnd
	tStyle
	tDefault
	tLinkStyle
	tInterpolate
	tClassDef
	tClass
	tClick
	tHref
	tCallbackName
	tCallbackArgs
	tLinkTarget
	tDirection // direction_tb etc.; text is the direction
	tAccTitle
	tAccTitleValue
	tAccDescr
	tAccDescrValue
	tAccDescrMultiValue
	tSkip // internal: the rule makes no token
)

type (
	token   = jison.Token[kind]
	scanner = jison.Scanner[kind]
)

// idGoesOn is what makes a keyword the start of a longer node id: a word
// character or letter, or one after '-', '.' or '/'. flow.jison only asks
// for a word's end (\b), so end-user lexes as END and "-user", and mermaid
// fails on it (its parser spec even asserts so); we read an id.
const idGoesOn = `[-./]?[\w\p{L}]`

// lexer is flow.jison's lexer, in its order; the keyword rules also have
// NotAhead(idGoesOn).
var lexer = jison.New(jison.Options[kind]{Skip: tSkip, EOF: tEOF},
	jison.On("", `accTitle\s*:\s*`, tAccTitle).Push("acc_title"),
	jison.On("acc_title", `[^\n]*`, tAccTitleValue).Pop(),
	jison.On("", `accDescr\s*:\s*`, tAccDescr).Push("acc_descr"),
	jison.On("acc_descr", `[^\n]*`, tAccDescrValue).Pop(),
	jison.On("", `accDescr\s*\{\s*`, tSkip).Push("acc_descr_multiline"),
	jison.Lit("acc_descr_multiline", "}", tSkip).Pop(),
	jison.On("acc_descr_multiline", `[^}]*`, tAccDescrMultiValue),

	jison.Lit("", "@{", tShapeData).Push("shapeData").Text(""),
	jison.Lit("shapeData", `"`, tShapeData).Push("shapeDataStr"),
	jison.Lit("shapeDataStr", `"`, tShapeData).Pop(),
	jison.On("shapeDataStr", `[^"]+`, tShapeData).Do(func(_ *scanner, t *token) {
		t.Text = newlineIndent.ReplaceAllString(t.Text, "<br/>")
	}),
	jison.On("shapeData", `[^}^"]+`, tShapeData),
	jison.Lit("shapeData", "}", tSkip).Pop(),

	jison.On("", `call\s+`, tSkip).Push("callbackname"),
	jison.On("callbackname", `\(\s*\)`, tSkip).Pop(),
	jison.Lit("callbackname", "(", tSkip).Switch("callbackargs"),
	jison.On("callbackname", `[^(]*`, tCallbackName),
	jison.Lit("callbackargs", ")", tSkip).Pop(),
	jison.On("callbackargs", `[^)]*`, tCallbackArgs),

	jison.On("md_string", "[^`\"]+", tMdStr),
	jison.Lit("md_string", "`\"", tSkip).Pop(),
	jison.Lit("*", "\"`", tSkip).Push("md_string"),
	jison.On("string", `[^"]+`, tStr),
	jison.Lit("string", `"`, tSkip).Pop(),
	jison.Lit("*", `"`, tSkip).Push("string"),

	jison.Lit("", "style", tStyle).NotAhead(idGoesOn),
	jison.Lit("", "default", tDefault),
	jison.Lit("", "linkStyle", tLinkStyle).NotAhead(idGoesOn),
	jison.Lit("", "interpolate", tInterpolate).NotAhead(idGoesOn),
	jison.Lit("", "classDef", tClassDef).NotAhead(idGoesOn),
	jison.Lit("", "class", tClass).NotAhead(idGoesOn),
	jison.On("", `href[\s]`, tHref),
	jison.On("", `click[\s]+`, tSkip).Push("click"),
	jison.On("click", `[\s\n]`, tSkip).Pop(),
	jison.On("click", `[^\s\n]*`, tClick),

	jison.Lit("", "flowchart-elk", tGraph).Do(graph).NotAhead(idGoesOn),
	jison.Lit("", "swimlane-beta", tGraph).Do(graph).NotAhead(idGoesOn),
	jison.Lit("", "graph", tGraph).Do(graph).NotAhead(idGoesOn),
	jison.Lit("", "flowchart", tGraph).Do(graph).NotAhead(idGoesOn),
	jison.Lit("", "subgraph", tSubgraph).NotAhead(idGoesOn),
	jison.Lit("", "end", tEnd).NotAhead(idGoesOn), // end\b\s*; the white space after is skipped anyway
	jison.On("", `_self|_blank|_parent|_top`, tLinkTarget).NotAhead(idGoesOn),

	jison.On("dir", `(?:\r?\n)*\s*\n`, tNoDir).Pop(),
	jison.On("dir", `\s*(?:(?:LR|RL|TB|BT|TD|BR|v)\b|[<>^])`, tDir).Pop(),

	jison.On("", `.*direction\s+TB[^\n]*`, tDirection).Text("TB"),
	jison.On("", `.*direction\s+BT[^\n]*`, tDirection).Text("BT"),
	jison.On("", `.*direction\s+RL[^\n]*`, tDirection).Text("RL"),
	jison.On("", `.*direction\s+LR[^\n]*`, tDirection).Text("LR"),
	jison.On("", `.*direction\s+TD[^\n]*`, tDirection).Text("TD"),

	jison.Func("", linkID, tLinkID),
	jison.On("", `[0-9]+`, tNum),
	jison.Lit("", "#", tBrkt),
	jison.Lit("", ":::", tStyleSep),
	jison.Lit("", ":", tColon),
	jison.Lit("", "&", tAmp),
	jison.Lit("", ";", tSemi),
	jison.Lit("", ",", tComma),
	jison.Lit("", "*", tMult),

	jison.On("INITIAL,edgeText", `\s*[xo<]?--+[-xo>]\s*`, tLink).Pop(),
	jison.On("", `\s*[xo<]?--\s*`, tStartLink).Push("edgeText"),
	jison.Func("edgeText", edgeTextChar('-'), tEdgeText),
	jison.On("INITIAL,thickEdgeText", `\s*[xo<]?==+[=xo>]\s*`, tLink).Pop(),
	jison.On("", `\s*[xo<]?==\s*`, tStartLink).Push("thickEdgeText"),
	jison.Func("thickEdgeText", edgeTextChar('='), tEdgeText),
	jison.On("INITIAL,dottedEdgeText", `\s*[xo<]?-?\.+-[xo>]?\s*`, tLink).Pop(),
	jison.On("", `\s*[xo<]?-\.\s*`, tStartLink).Push("dottedEdgeText"),
	jison.Func("dottedEdgeText", dottedTextChar, tEdgeText),
	jison.On("*", `\s*~~~+\s*`, tLink),

	jison.On("ellipseText", `[-/)]\)`, tEllipseEnd).Pop(),
	jison.On("ellipseText", `[^()\[\]{}]`, tText),
	jison.Lit("*", "(-", tEllipseStart).Push("ellipseText"),
	jison.Lit("text", "])", tStadiumEnd).Pop(),
	jison.Lit("*", "([", tStadiumStart).Push("text"),
	jison.Lit("text", "]]", tSubroutineEnd).Pop(),
	jison.Lit("*", "[[", tSubroutineStart).Push("text"),
	jison.Lit("", "[|", tVertexWithPropsStart),
	jison.Lit("", ">", tTagEnd).Push("text"),
	jison.Lit("text", ")]", tCylinderEnd).Pop(),
	jison.Lit("*", "[(", tCylinderStart).Push("text"),
	jison.Lit("text", ")))", tDoubleCircleEnd).Pop(),
	jison.Lit("*", "(((", tDoubleCircleStart).Push("text"),
	jison.On("trapText", `[\\(?=\])]\]`, tTrapEnd).Pop(),
	jison.Lit("trapText", "/]", tInvTrapEnd).Pop(),
	jison.Func("trapText", trapText, tText),
	jison.Lit("*", "[/", tTrapStart).Push("trapText"),
	jison.Lit("*", `[\`, tInvTrapStart).Push("trapText"),

	jison.Lit("", "<", tTagStart),
	jison.Lit("", "^", tUp),
	jison.Lit("", `\|`, tSep),
	jison.Lit("", "v", tDown),
	jison.Func("", nodeString, tNodeString),
	jison.Lit("", "-", tMinus),
	jison.Func("", unicodeText, tUnicodeText),

	jison.Lit("text", "|", tPipe).Pop(),
	jison.Lit("*", "|", tPipe).Push("text"),
	jison.Lit("text", ")", tPE).Pop(),
	jison.Lit("*", "(", tPS).Push("text"),
	jison.Lit("text", "]", tSQE).Pop(),
	jison.Lit("*", "[", tSQS).Push("text"),
	jison.Lit("text", "}", tDiamondStop).Pop(),
	jison.Lit("*", "{", tDiamondStart).Push("text"),
	jison.On("text", `[^\[\](){}|"]+`, tText),

	jison.On("", `(?:\r?\n)+`, tNewline),
	jison.On("", `[^\S\n\r]+`, tSpace),
)

// graph is the header's action: the first one is followed by a direction
// (flowDb's firstGraph).
func graph(s *scanner, _ *token) {
	for _, t := range s.Tokens() {
		if t.Kind == tGraph {
			return
		}
	}
	s.Push("dir")
}

var newlineIndent = jison.Regexp(`\n\s*`)

// linkID is [^\s"]+@(?=[^{"]), up to the last '@' that a character other
// than '{' or '"' follows, but without brackets or pipes: mermaid takes
// A[foo@bar] for the edge id "A[foo" and fails, we read the '@' as the
// node's text.
func linkID(s string) int {
	run := strings.IndexFunc(s, func(r rune) bool {
		return r == '"' || unicode.IsSpace(r) || r == 0xfeff || strings.ContainsRune("[](){}|<>", r)
	})
	if run < 0 {
		run = len(s)
	}
	for i := run - 1; i >= 1; i-- {
		if s[i] == '@' && i+1 < len(s) && s[i+1] != '{' && s[i+1] != '"' {
			return i + 1
		}
	}
	return -1
}

// edgeTextChar is [^c]|c(?!c): one character of an edge's text.
func edgeTextChar(c byte) func(string) int {
	return func(s string) int {
		if s == "" || s[0] == c && len(s) > 1 && s[1] == c {
			return -1
		}
		return 1
	}
}

// dottedTextChar is [^.]|\.(?!-).
func dottedTextChar(s string) int {
	if s == "" || s[0] == '.' && len(s) > 1 && s[1] == '-' {
		return -1
	}
	return 1
}

// trapText is \/(?!\])|\\(?!\])|[^\\\[\](){}\/]+.
func trapText(s string) int {
	if s == "" {
		return -1
	}
	if s[0] == '/' || s[0] == '\\' {
		if len(s) > 1 && s[1] == ']' {
			return -1
		}
		return 1
	}
	n := 0
	for n < len(s) && !strings.ContainsRune(`\[](){}/`, rune(s[n])) {
		n++
	}
	if n == 0 {
		return -1
	}
	return n
}

// nodeString is ([A-Za-z0-9!"#$%&'*+.`?\\_/]|-(?=[^>\-.])|=(?!=))+.
func nodeString(s string) int {
	n := 0
	for n < len(s) {
		c := s[n]
		switch {
		case isWord(c) || strings.IndexByte("!\"#$%&'*+.`?\\/", c) >= 0:
		case c == '-' && n+1 < len(s) && strings.IndexByte(">-.", s[n+1]) < 0:
		case c == '=' && (n+1 == len(s) || s[n+1] != '='):
		default:
			goto done
		}
		n++
	}
done:
	if n == 0 {
		return -1
	}
	return n
}

// unicodeText is one letter outside ASCII (flow.jison lists the Basic
// Multilingual Plane's letters).
func unicodeText(s string) int {
	r, n := utf8.DecodeRuneInString(s)
	if r < 0x80 || r > 0xffff || !unicode.IsLetter(r) {
		return -1
	}
	return n
}

func isWord(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

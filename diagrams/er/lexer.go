package er

import "github.com/f1sherb0y/mermaid-go/diagrams/internal/jison"

type kind uint8

const (
	tEOF kind = iota
	tNewline
	tEntityName // a quoted name
	tWord       // a quoted text that is not a name
	tHeader
	tBlockStart
	tBlockStop
	tBrkt
	tComma
	tStyleSep
	tColon
	tSQS
	tSQE
	tAttrKey
	tAttrWord
	tComment
	tStyle
	tClassDef
	tClass
	tSubgraph
	tEnd
	tCard // text: the cardinality, as erDb's Cardinality
	tRel  // text: IDENTIFYING or NON_IDENTIFYING
	tDecimal
	tEntityOne
	tNum
	tStyleText
	tSemi
	tUnicodeText
	tChar      // any other character, as itself
	tDirection // text: TB, BT, RL or LR
	tAccTitle
	tAccTitleValue
	tAccDescr
	tAccDescrValue
	tAccDescrMultiValue
	tSkip // internal: the rule makes no token
)

type token = jison.Token[kind]

// nameGoesOn is what makes a keyword, a cardinality or a number the start
// of a longer name (UNICODE_TEXT, ATTRIBUTE_WORD): a word character, a
// non-ASCII one, or one after '-' or '.'. erDiagram.jison only asks for a
// word's end (\b), so many-to-many lexes as "many" (a cardinality) and
// "-to-many", end-user as END and "-user", 2FA as NUM and "FA", and mermaid
// fails on them; we read them as names.
const nameGoesOn = `[-.]?[\w\x{80}-\x{10FFFF}]`

// lexer is erDiagram.jison's lexer (%options case-insensitive), in its
// order; the word rules also have NotAhead(nameGoesOn).
var lexer = jison.New(jison.Options[kind]{Skip: tSkip, EOF: tEOF, CaseInsensitive: true},
	jison.On("", `accTitle\s*:\s*`, tAccTitle).Push("acc_title"),
	jison.On("acc_title", `[^\n]*`, tAccTitleValue).Pop(),
	jison.On("", `accDescr\s*:\s*`, tAccDescr).Push("acc_descr"),
	jison.On("acc_descr", `[^\n]*`, tAccDescrValue).Pop(),
	jison.On("", `accDescr\s*\{\s*`, tSkip).Push("acc_descr_multiline"),
	jison.Lit("acc_descr_multiline", "}", tSkip).Pop(),
	jison.On("acc_descr_multiline", `[^}]*`, tAccDescrMultiValue),
	jison.On("", `.*direction\s+TB[^\n]*`, tDirection).Text("TB"),
	jison.On("", `.*direction\s+BT[^\n]*`, tDirection).Text("BT"),
	jison.On("", `.*direction\s+RL[^\n]*`, tDirection).Text("RL"),
	jison.On("", `.*direction\s+LR[^\n]*`, tDirection).Text("LR"),
	jison.On("", `[ \t\r]+`, tSkip),
	jison.On("", `\n+`, tNewline),
	jison.On("", `"[^"%\r\n\v\x08\\]+"`, tEntityName),
	jison.On("", `"[^"]*"`, tWord),
	jison.Lit("", "erDiagram", tHeader),
	jison.Lit("", "{", tBlockStart).Push("block"),
	jison.Lit("", "#", tBrkt),
	jison.Lit("", ",", tComma),
	jison.Lit("", ":::", tStyleSep),
	jison.Lit("", ":", tColon),

	jison.On("block", `\s+`, tSkip),
	jison.On("block", `\b(?:PK|FK|UK)`, tAttrKey).NotAhead(nameGoesOn),
	jison.On("block", `[^\s]*~.*~[^\s]*`, tAttrWord),
	jison.On("block", `[\*A-Za-z_\x{C0}-\x{10FFFF}][A-Za-z0-9\-_\[\]\(\)\.,\x{C0}-\x{10FFFF}\*]*`, tAttrWord),
	jison.Lit("block", "`", tSkip).Push("BLOCK_BQ"),
	jison.On("BLOCK_BQ", "[^`]+", tAttrWord),
	jison.Lit("BLOCK_BQ", "`", tSkip).Pop(),
	jison.On("block", `"[^"]*"`, tComment),
	jison.On("block", `\n+`, tSkip),
	jison.Lit("block", "}", tBlockStop).Pop(),
	jison.On("block", `.`, tChar),
	jison.Lit("", "[", tSQS),
	jison.Lit("", "]", tSQE),

	jison.Lit("", "style", tStyle).Push("style").NotAhead(nameGoesOn),
	jison.On("style", `\n+`, tNewline).Pop(),
	// \s+ in the .jison: it takes the line break after trailing white
	// space, and the style statement runs on into the next line.
	jison.On("style", `[^\S\n]+`, tSkip),
	jison.Lit("style", ":", tColon),
	jison.Lit("style", ",", tComma),
	jison.Lit("style", "#", tBrkt),
	jison.Lit("", "classDef", tClassDef).Push("style").NotAhead(nameGoesOn),
	jison.Lit("", "class", tClass).NotAhead(nameGoesOn),
	jison.Lit("", "subgraph", tSubgraph).NotAhead(nameGoesOn),
	jison.Lit("", "end", tEnd).NotAhead(nameGoesOn), // end\b\s*; the newline after is skipped anyway

	jison.Lit("", "one or zero", tCard).Text(zeroOrOne).NotAhead(nameGoesOn),
	jison.Lit("", "one or more", tCard).Text(oneOrMore).NotAhead(nameGoesOn),
	jison.Lit("", "one or many", tCard).Text(oneOrMore).NotAhead(nameGoesOn),
	jison.Lit("", "1+", tCard).Text(oneOrMore),
	jison.On("", `\|o`, tCard).Text(zeroOrOne),
	jison.Lit("", "zero or one", tCard).Text(zeroOrOne).NotAhead(nameGoesOn),
	jison.Lit("", "zero or more", tCard).Text(zeroOrMore).NotAhead(nameGoesOn),
	jison.Lit("", "zero or many", tCard).Text(zeroOrMore).NotAhead(nameGoesOn),
	jison.Lit("", "0+", tCard).Text(zeroOrMore),
	jison.On("", `\}o`, tCard).Text(zeroOrMore),
	jison.Lit("", "many(0)", tCard).Text(zeroOrMore),
	jison.Lit("", "many(1)", tCard).Text(oneOrMore),
	jison.Lit("", "many", tCard).Text(zeroOrMore).NotAhead(nameGoesOn),
	jison.On("", `\}\|`, tCard).Text(oneOrMore),
	jison.Lit("", "one", tCard).Text(onlyOne).NotAhead(nameGoesOn),
	jison.Lit("", "only one", tCard).Text(onlyOne).NotAhead(nameGoesOn),
	jison.On("", `[0-9]+\.[0-9]+`, tDecimal).NotAhead(nameGoesOn),
	jison.Lit("", "1", tCard).Text(onlyOne).Ahead(`\s+[A-Za-z_"']`),
	jison.Lit("", "1", tCard).Text(onlyOne).Ahead(`\s+[0-9]`),
	jison.Lit("", "1", tCard).Text(onlyOne).Ahead(`--|\.\.|\.-|-\.`),
	jison.Lit("", "1", tEntityOne).NotAhead(nameGoesOn),
	jison.On("", `[0-9]+`, tNum).NotAhead(nameGoesOn),
	jison.On("", `\|\|`, tCard).Text(onlyOne),
	jison.On("", `o\|`, tCard).Text(zeroOrOne),
	jison.On("", `o\{`, tCard).Text(zeroOrMore),
	jison.On("", `\|\{`, tCard).Text(oneOrMore),
	jison.Lit("", "u", tCard).Text("MD_PARENT").Ahead(`[.\-|]`).NotAhead(nameGoesOn),
	jison.On("", `\.\.`, tRel).Text("NON_IDENTIFYING"),
	jison.On("", `--`, tRel).Text("IDENTIFYING"),
	jison.Lit("", "to", tRel).Text("IDENTIFYING").NotAhead(nameGoesOn),
	jison.Lit("", "optionally to", tRel).Text("NON_IDENTIFYING").NotAhead(nameGoesOn),
	jison.On("", `\.-`, tRel).Text("NON_IDENTIFYING"),
	jison.On("", `-\.`, tRel).Text("NON_IDENTIFYING"),

	jison.On("style", `(?:[^\x00-\x7F]|\w|-|\*)+`, tStyleText),
	jison.Lit("style", ";", tSemi),
	jison.On("", `(?:[^\x00-\x7F]|\w|-|\*|\.)+`, tUnicodeText),
	jison.On("", `.`, tChar),
)

// erDb's Cardinality.
const (
	zeroOrOne  = "ZERO_OR_ONE"
	zeroOrMore = "ZERO_OR_MORE"
	oneOrMore  = "ONE_OR_MORE"
	onlyOne    = "ONLY_ONE"
)

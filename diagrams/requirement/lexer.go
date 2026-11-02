package requirement

import (
	"strings"

	"github.com/f1sherb0y/mermaid-go/diagrams/internal/jison"
)

type kind uint8

const (
	tEOF kind = iota
	tNewline
	tTitle
	tHeader
	tStructStart
	tStructStop
	tStyleSep
	tColonSep
	tID
	tText
	tRisk
	tVerifyMethod
	tReqType // text: the requirement type, as requirementDb's RequirementType
	tRiskLevel
	tVerifyType
	tElement
	tRelationship // text: contains, copies, ...
	tType
	tDocRef
	tStyle
	tClassDef
	tClass
	tAlpha
	tColon
	tSemicolon
	tPercent
	tMinus
	tBrkt
	tArrowL // <-
	tArrowR // ->
	tLine   // -
	tQString
	tUnqString
	tNum
	tComma
	tDirection // text: TB, BT, RL or LR
	tAccTitle
	tAccTitleValue
	tAccDescr
	tAccDescrValue
	tAccDescrMultiValue
	tSkip // internal: the rule makes no token
)

type token = jison.Token[kind]

// lexer is requirementDiagram.jison's lexer (%options case-insensitive),
// in its order; its <*> rules come last.
var lexer = jison.New(jison.Options[kind]{Skip: tSkip, EOF: tEOF, CaseInsensitive: true}, rules()...)

func rules() []jison.Rule[kind] {
	rs := []jison.Rule[kind]{
		jison.On("", `title\s[^#\n;]+`, tTitle),
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
		jison.On("", `(?:\r?\n)+`, tNewline),
		// \s+ in the .jison: it takes the line break after trailing white
		// space, and the grammar wants a NEWLINE there.
		jison.On("", `[^\S\n]+`, tSkip),
		jison.On("", `#[^\n]*`, tSkip),
		jison.On("", `%%[^\n]*`, tSkip),

		jison.Lit("", "requirementDiagram", tHeader),
		jison.Lit("", "{", tStructStart),
		jison.Lit("", "}", tStructStop),
		jison.Lit("", ":::", tStyleSep),
		jison.Lit("", ":", tColonSep),
	}
	// A field whose value is a string (id, text, type, docref) takes the
	// rest of its line, unless quoted. In the .jison the value is
	// unqString, \w[^:,\r\n{<>\-=]*, after the keywords: it fails on a
	// value that is a keyword (text: test), starts with one (low
	// priority), has '-', ',' or ':' (id: REQ-001), or starts with a
	// non-ASCII letter.
	for _, f := range []struct {
		w string
		k kind
	}{{"id", tID}, {"text", tText}, {"type", tType}, {"docref", tDocRef}} {
		rs = append(rs, jison.Lit("", f.w, f.k).Ahead(`[^\S\n]*:`).Push("field"))
	}
	rs = append(rs,
		jison.On("field", `[^\S\n]+`, tSkip),
		jison.Lit("field", ":", tColonSep).Switch("value"),
		jison.On("value", `[^\S\n]+`, tSkip),
		jison.Lit("value", `"`, tSkip).Push("string"),
		jison.On("value", `(?:\r?\n)+`, tNewline).Pop(),
		jison.On("value", `[^\r\n]+`, tUnqString).Do(trim),
	)
	words := []struct {
		w, v string
		k    kind
	}{
		{"id", "", tID},
		{"text", "", tText},
		{"risk", "", tRisk},
		{"verifyMethod", "", tVerifyMethod},
		{"requirement", "Requirement", tReqType},
		{"functionalRequirement", "Functional Requirement", tReqType},
		{"interfaceRequirement", "Interface Requirement", tReqType},
		{"performanceRequirement", "Performance Requirement", tReqType},
		{"physicalRequirement", "Physical Requirement", tReqType},
		{"designConstraint", "Design Constraint", tReqType},
		{"low", "Low", tRiskLevel},
		{"medium", "Medium", tRiskLevel},
		{"high", "High", tRiskLevel},
		{"analysis", "Analysis", tVerifyType},
		{"demonstration", "Demonstration", tVerifyType},
		{"inspection", "Inspection", tVerifyType},
		{"test", "Test", tVerifyType},
		{"element", "", tElement},
		{"contains", "contains", tRelationship},
		{"copies", "copies", tRelationship},
		{"derives", "derives", tRelationship},
		{"satisfies", "satisfies", tRelationship},
		{"verifies", "verifies", tRelationship},
		{"refines", "refines", tRelationship},
		{"traces", "traces", tRelationship},
		{"type", "", tType},
		{"docref", "", tDocRef},
	}
	for _, w := range words {
		r := jison.Lit("", w.w, w.k)
		if w.v != "" {
			r = r.Text(w.v)
		}
		rs = append(rs, r)
	}
	return append(rs,
		jison.Lit("", "style", tStyle).Push("style"),
		jison.On("style", `\w+`, tAlpha),
		jison.Lit("style", ":", tColon),
		jison.Lit("style", ";", tSemicolon),
		jison.Lit("style", "%", tPercent),
		jison.Lit("style", "-", tMinus),
		jison.Lit("style", "#", tBrkt),
		jison.On("style", `[^\S\n]`, tSkip), // " " in the .jison: a tab fails there
		jison.Lit("style", `"`, tSkip).Push("string"),
		jison.Lit("style", "\n", tSkip).Pop(),

		jison.Lit("", "classDef", tClassDef).Push("style"),
		jison.Lit("", "class", tClass).Push("style"),

		jison.Lit("", "<-", tArrowL),
		jison.Lit("", "->", tArrowR),
		jison.Lit("", "-", tLine),

		jison.Lit("", `"`, tSkip).Push("string"),
		jison.Lit("string", `"`, tSkip).Pop(),
		jison.On("string", `[^"]*`, tQString),

		// [\w] in the .jison: a name may start with a non-ASCII letter too.
		jison.On("", `[\w\x{80}-\x{10FFFF}][^:,\r\n{<>\-=]*`, tUnqString).Do(trim),

		jison.On("*", `\w+`, tAlpha),
		jison.On("*", `[0-9]+`, tNum),
		jison.Lit("*", ",", tComma),
	)
}

func trim(_ *jison.Scanner[kind], t *token) { t.Text = strings.TrimSpace(t.Text) }

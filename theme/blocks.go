package theme

import (
	"math"

	"github.com/f1sherb0y/mermaid-go/color"
)

// Blocks of the themes' constructors and updateColors that several have word for word.

// architectureInit is the constructor's architecture diagram block.
func architectureInit(v vars) {
	v.set("archEdgeColor", "calculated")
	v.set("archEdgeArrowColor", "calculated")
	v.set("archEdgeWidth", "3")
	v.set("archGroupBorderColor", v["primaryBorderColor"])
	v.set("archGroupBorderWidth", "2px")
}

// mainColors derives the base colors from primaryColor and background; ink is the text
// color outside dark mode, noteBkg the note fill if unset.
func mainColors(v vars, ink, noteBkg string) {
	dark := v.dark()
	v.or("primaryTextColor", pick(dark, "#eee", ink))
	v.or("secondaryColor", adjust(v["primaryColor"], dh(-120)))
	v.or("tertiaryColor", adjust(v["primaryColor"], dhl(180, 5)))
	v.or("primaryBorderColor", mkBorder(v["primaryColor"], dark))
	v.or("secondaryBorderColor", mkBorder(v["secondaryColor"], dark))
	v.or("tertiaryBorderColor", mkBorder(v["tertiaryColor"], dark))
	v.or("noteBorderColor", mkBorder(v["noteBkgColor"], dark))
	v.or("noteBkgColor", noteBkg)
	v.or("noteTextColor", ink)
	v.or("secondaryTextColor", invert(v["secondaryColor"]))
	v.or("tertiaryTextColor", invert(v["tertiaryColor"]))
	v.or("lineColor", invert(v["background"]))
	v.or("arrowheadColor", invert(v["background"]))
	v.or("textColor", v["primaryTextColor"])
	v.or("border2", v["tertiaryBorderColor"])
}

// labelOnSecondary is the edge label background: secondaryColor, darkened in dark mode.
func labelOnSecondary(v vars) any {
	return pick(v.dark(), darken(v["secondaryColor"], 30), v["secondaryColor"])
}

// flowchartColors is the flowchart (and agentflow) block, nodeBorder's default given.
func flowchartColors(v vars, nodeBorder any) {
	v.or("nodeBkg", v["primaryColor"])
	v.or("mainBkg", v["primaryColor"])
	v.or("nodeBorder", nodeBorder)
	v.or("clusterBkg", v["tertiaryColor"])
	v.or("clusterBorder", v["tertiaryBorderColor"])
	v.or("defaultLinkColor", v["lineColor"])
	v.or("titleColor", v["tertiaryTextColor"])
	v.or("edgeLabelBackground", labelOnSecondary(v))
	v.or("nodeTextColor", v["primaryTextColor"])
	v.or("flowContainerStroke", v["secondaryBorderColor"])
}

// sequenceColors is the sequence diagram block (but rectBkgColor, which varies).
func sequenceColors(v vars) {
	v.or("actorBorder", v["primaryBorderColor"])
	v.or("actorBkg", v["mainBkg"])
	v.or("actorTextColor", v["primaryTextColor"])
	v.or("actorLineColor", v["actorBorder"])
	v.or("labelBoxBkgColor", v["actorBkg"])
	v.or("signalColor", v["textColor"])
	v.or("signalTextColor", v["textColor"])
	v.or("labelBoxBorderColor", v["actorBorder"])
	v.or("labelTextColor", v["actorTextColor"])
	v.or("loopTextColor", v["actorTextColor"])
	v.or("activationBorderColor", darken(v["secondaryColor"], 10))
	v.or("activationBkgColor", v["secondaryColor"])
	v.or("sequenceNumberColor", invert(v["lineColor"]))
}

// cScaleWheel defaults cScale0..11 to p, s, t and then p around the color wheel.
func cScaleWheel(v vars, p, s, t any) {
	v.or("cScale0", p)
	v.or("cScale1", s)
	v.or("cScale2", t)
	for i, d := range []color.Delta{dh(30), dh(60), dh(90), dh(120), dh(150), dhl(210, 150), dh(270), dh(300), dh(330)} {
		v.or(key("cScale", i+3), adjust(p, d))
	}
}

// cScaleDarken darkens every cScale color by 25, or by 75 in dark mode.
func cScaleDarken(v vars) {
	for i := range v.limit() {
		v.set(key("cScale", i), darken(v[key("cScale", i)], pick(v.dark(), 75.0, 25.0).(float64)))
	}
}

// fillTypes is the user journey's fills from p and s: defaults, or set outright.
func fillTypes(v vars, p, s any, outright bool) {
	set := v.or
	if outright {
		set = v.set
	}
	set("fillType0", p)
	set("fillType1", s)
	for i, deg := range []float64{64, -64, 128} {
		set(key("fillType", 2*i+2), adjust(p, dh(deg)))
		set(key("fillType", 2*i+3), adjust(s, dh(deg)))
	}
}

// pieWheel defaults pie1..12 to shades of p, s and t.
func pieWheel(v vars, p, s, t any) {
	v.or("pie1", p)
	v.or("pie2", s)
	v.or("pie3", t)
	v.or("pie4", adjust(p, dl(-10)))
	v.or("pie5", adjust(s, dl(-10)))
	v.or("pie6", adjust(t, dl(-10)))
	v.or("pie7", adjust(p, dhl(60, -10)))
	v.or("pie8", adjust(p, dhl(-60, -10)))
	v.or("pie9", adjust(p, dh(120)))
	v.or("pie10", adjust(p, dhl(60, -20)))
	v.or("pie11", adjust(p, dhl(-60, -20)))
	v.or("pie12", adjust(p, dhl(120, -10)))
}

// requirement is the requirement diagram block.
func requirement(v vars, bkg, labelBkg any) {
	v.or("requirementBackground", bkg)
	v.or("requirementBorderColor", v["primaryBorderColor"])
	v.or("requirementBorderSize", "1")
	v.or("requirementTextColor", v["primaryTextColor"])
	v.or("relationColor", v["lineColor"])
	v.or("relationLabelBackground", labelBkg)
	v.or("relationLabelColor", v["actorTextColor"])
}

// packet is the packet diagram's colors, set outright.
func packet(v vars, blockFill any) {
	m := map[string]any{"blockFillColor": blockFill}
	for _, k := range []string{"startByteColor", "endByteColor", "labelColor", "titleColor", "blockStrokeColor"} {
		m[k] = v["primaryTextColor"]
	}
	for k, x := range m {
		if x == nil {
			delete(m, k)
		}
	}
	v.set("packet", m)
}

// Deltas for adjust: hue, hue and lightness, lightness, and all of red, green and blue.
func dh(deg float64) color.Delta         { return color.Delta{H: deg} }
func dhl(deg, light float64) color.Delta { return color.Delta{H: deg, L: light} }
func dl(light float64) color.Delta       { return color.Delta{L: light} }
func drgb(x float64) color.Delta         { return color.Delta{R: x, G: x, B: x} }

// nan is the amount of khroma's lighten(c) and darken(c) called without one.
var nan = math.NaN()

// classicScale is the classic themes' color scale: primaryColor around the wheel, darkened,
// with darker peers and hue-turned inverses.
func classicScale(v vars) {
	v.or("cScale0", v["primaryColor"])
	v.or("cScale1", v["secondaryColor"])
	v.or("cScale2", v["tertiaryColor"])
	for i, deg := range []float64{30, 60, 90, 120, 150, 210, 270, 300, 330} {
		v.or(key("cScale", i+3), adjust(v["primaryColor"], dh(deg)))
	}
	v.or("cScalePeer1", darken(v["secondaryColor"], 45))
	v.or("cScalePeer2", darken(v["tertiaryColor"], 40))
	for i := range v.limit() {
		v.set(key("cScale", i), darken(v[key("cScale", i)], 10))
		v.or(key("cScalePeer", i), darken(v[key("cScale", i)], 25))
	}
	for i := range v.limit() {
		v.or(key("cScaleInv", i), adjust(v[key("cScale", i)], dh(180)))
	}
}

// cScaleInverses derives each cScale color's inverse and peer (a border shade).
func cScaleInverses(v vars) {
	for i := range v.limit() {
		v.or(key("cScaleInv", i), invert(v[key("cScale", i)]))
	}
	for i := range v.limit() {
		if v.dark() {
			v.or(key("cScalePeer", i), lighten(v[key("cScale", i)], 10))
		} else {
			v.or(key("cScalePeer", i), darken(v[key("cScale", i)], 10))
		}
	}
}

// cScaleSets derives each cScale color's inverse, peer and label color.
func cScaleSets(v vars) {
	cScaleInverses(v)
	v.or("scaleLabelColor", v["labelTextColor"])
	for i := range v.limit() {
		v.or(key("cScaleLabel", i), v["scaleLabelColor"])
	}
}

// surfaces derives surface0..4 and surfacePeer0..4 from mainBkg.
func surfaces(v vars) {
	m := pick(v.dark(), -4.0, -1.0).(float64)
	for i := range 5 {
		fi := float64(i)
		v.or(key("surface", i), adjust(v["mainBkg"], color.Delta{H: 180, S: -15, L: m * (5 + fi*3)}))
		v.or(key("surfacePeer", i), adjust(v["mainBkg"], color.Delta{H: 180, S: -15, L: m * (8 + fi*3)}))
	}
}

// vennWheel defaults the venn sets to shades of the base colors; l3 is the third's lightness.
func vennWheel(v vars, l3 float64) {
	v.ifNull("venn1", adjust(v["primaryColor"], dl(-30)))
	v.ifNull("venn2", adjust(v["secondaryColor"], dl(-30)))
	v.ifNull("venn3", adjust(v["tertiaryColor"], dl(l3)))
	v.ifNull("venn4", adjust(v["primaryColor"], dhl(60, -30)))
	v.ifNull("venn5", adjust(v["primaryColor"], dhl(-60, -30)))
	v.ifNull("venn6", adjust(v["secondaryColor"], dhl(60, -30)))
	v.ifNull("venn7", adjust(v["primaryColor"], dhl(120, -30)))
	v.ifNull("venn8", adjust(v["secondaryColor"], dhl(120, -30)))
	v.ifNull("vennTitleTextColor", v["titleColor"])
	v.ifNull("vennSetTextColor", v["textColor"])
}

func pieText(v vars) {
	v.or("pieTitleTextSize", "25px")
	v.or("pieTitleTextColor", v["taskTextDarkColor"])
	v.or("pieSectionTextSize", "17px")
	v.or("pieSectionTextColor", v["textColor"])
	v.or("pieLegendTextSize", "17px")
	v.or("pieLegendTextColor", v["taskTextDarkColor"])
	v.or("pieStrokeColor", "black")
	v.or("pieStrokeWidth", "2px")
	v.or("pieOuterStrokeWidth", "2px")
	v.or("pieOuterStrokeColor", "black")
	v.or("pieOpacity", "0.7")
}

func cynefin(v vars) {
	v.nested("cynefin",
		field{"domainFontSize", 16},
		field{"itemFontSize", 12},
		field{"boundaryColor", v["lineColor"]},
		field{"boundaryWidth", 2},
		field{"cliffColor", "#8B0000"},
		field{"cliffWidth", 4},
		field{"arrowColor", v["lineColor"]},
		field{"arrowWidth", 2},
		field{"complexBg", "#E8F5E9"},
		field{"complicatedBg", "#E3F2FD"},
		field{"chaoticBg", "#FBE9E7"},
		field{"clearBg", "#FFF8E1"},
		field{"confusionBg", "#F3E5F5"},
		field{"textColor", v["textColor"]},
		field{"labelColor", v["primaryTextColor"]},
	)
}

func radar(v vars) {
	v.nested("radar",
		field{"axisColor", v["lineColor"]},
		field{"axisStrokeWidth", 2},
		field{"axisLabelFontSize", 12},
		field{"curveOpacity", 0.5},
		field{"curveStrokeWidth", 2},
		field{"graticuleColor", "#DEDEDE"},
		field{"graticuleStrokeWidth", 1},
		field{"graticuleOpacity", 0.3},
		field{"legendBoxSize", 12},
		field{"legendFontSize", 12},
	)
}

func wardley(v vars) {
	v.or("wardleyEvolutionColor", "#dc3545")
	v.nested("wardley",
		field{"backgroundColor", v["background"]},
		field{"axisColor", v["lineColor"]},
		field{"axisTextColor", v["primaryTextColor"]},
		field{"gridColor", v["gridColor"]},
		field{"componentFill", v["background"]},
		field{"componentStroke", v["lineColor"]},
		field{"componentLabelColor", v["primaryTextColor"]},
		field{"linkStroke", v["lineColor"]},
		field{"evolutionStroke", v["wardleyEvolutionColor"]},
		field{"annotationStroke", v["lineColor"]},
		field{"annotationTextColor", v["primaryTextColor"]},
		field{"annotationFill", v["background"]},
	)
}

// quadrant is the quadrant chart's colors, its fills from fill.
func quadrant(v vars, fill any) {
	v.or("quadrant1Fill", fill)
	v.or("quadrant2Fill", adjust(fill, drgb(5)))
	v.or("quadrant3Fill", adjust(fill, drgb(10)))
	v.or("quadrant4Fill", adjust(fill, drgb(15)))
	v.or("quadrant1TextFill", v["primaryTextColor"])
	v.or("quadrant2TextFill", adjust(v["primaryTextColor"], drgb(-5)))
	v.or("quadrant3TextFill", adjust(v["primaryTextColor"], drgb(-10)))
	v.or("quadrant4TextFill", adjust(v["primaryTextColor"], drgb(-15)))
	// mermaid's "this.quadrantPointFill || isDark(q1) ? lighten(q1) : darken(q1)" parses as
	// "(…) ? … : …", and both branches pass no amount: the lightness becomes NaN either way.
	v.set("quadrantPointFill", lighten(v["quadrant1Fill"], nan))
	v.or("quadrantPointTextFill", v["primaryTextColor"])
	v.or("quadrantXAxisTextFill", v["primaryTextColor"])
	v.or("quadrantYAxisTextFill", v["primaryTextColor"])
	v.or("quadrantInternalBorderStrokeFill", v["primaryBorderColor"])
	v.or("quadrantExternalBorderStrokeFill", v["primaryBorderColor"])
	v.or("quadrantTitleFill", v["primaryTextColor"])
}

// xyChart is the xychart's nested colors; the neo and redux themes have no dataLabelColor.
func xyChart(v vars, palette string, dataLabel bool) {
	t := v["primaryTextColor"]
	fields := []field{
		{"backgroundColor", v["background"]},
		{"titleColor", t},
	}
	if dataLabel {
		fields = append(fields, field{"dataLabelColor", t})
	}
	for _, f := range []string{"legendTextColor", "xAxisTitleColor", "xAxisLabelColor", "xAxisTickColor", "xAxisLineColor",
		"yAxisTitleColor", "yAxisLabelColor", "yAxisTickColor", "yAxisLineColor"} {
		fields = append(fields, field{f, t})
	}
	v.nested("xyChart", append(fields, field{"plotColorPalette", palette})...)
}

// gitWheel defaults git0..7 to p, s, t and then p around the color wheel.
func gitWheel(v vars, p, s, t any) {
	v.or("git0", p)
	v.or("git1", s)
	v.or("git2", t)
	for i, deg := range []float64{-30, -60, -90, 60, 120} {
		v.or(key("git", i+3), adjust(p, dh(deg)))
	}
}

// gitShade darkens git0..7 by 25, or lightens them in dark mode.
func gitShade(v vars) {
	for i := range 8 {
		k := key("git", i)
		if v.dark() {
			v.set(k, lighten(v[k], 25))
		} else {
			v.set(k, darken(v[k], 25))
		}
	}
}

func gitInverses(v vars) {
	for i := range 8 {
		v.or(key("gitInv", i), invert(v[key("git", i)]))
	}
}

// gitLabels is the gitGraph's branch, tag and commit label colors.
func gitLabels(v vars) {
	v.or("branchLabelColor", pick(v.dark(), "black", v["labelTextColor"]))
	for i := range 8 {
		v.or(key("gitBranchLabel", i), v["branchLabelColor"])
	}
	gitTagLabels(v)
}

// classicBranchLabels is the classic themes' branch labels: the label color, inverted on
// branches 0 and 3.
func classicBranchLabels(v vars) {
	for i := range 8 {
		if i == 0 || i == 3 {
			v.or(key("gitBranchLabel", i), invert(v["labelTextColor"]))
		} else {
			v.or(key("gitBranchLabel", i), v["labelTextColor"])
		}
	}
}

// gitTagLabels is the gitGraph's tag and commit label colors.
func gitTagLabels(v vars) {
	v.or("tagLabelColor", v["primaryTextColor"])
	v.or("tagLabelBackground", v["primaryColor"])
	v.set("tagLabelBorder", first(v["tagBorder"], v["primaryBorderColor"]))
	v.or("tagLabelFontSize", "10px")
	v.or("commitLabelColor", v["secondaryTextColor"])
	v.or("commitLabelBackground", v["secondaryColor"])
	v.or("commitLabelFontSize", "10px")
}

// eventModeling is the event modeling diagram's colors.
func eventModeling(v vars) {
	for _, kv := range [][2]string{
		{"emUiFill", "white"}, {"emUiStroke", "#dbdada"},
		{"emProcessorFill", "#edb3f6"}, {"emProcessorStroke", "#b88cbf"},
		{"emReadModelFill", "#d3f1a2"}, {"emReadModelStroke", "#a3b732"},
		{"emCommandFill", "#bcd6fe"}, {"emCommandStroke", "#679ac3"},
		{"emEventFill", "#ffb778"}, {"emEventStroke", "#c19a0f"},
		{"emSwimlaneBackgroundOdd", "rgb(250,250,250)"}, {"emSwimlaneBackgroundStroke", "rgb(240,240,240)"},
	} {
		v.or(kv[0], kv[1])
	}
	v.or("emArrowhead", v["lineColor"])
	v.or("emRelationStroke", v["lineColor"])
}

// erAttributes is the ER diagram's attribute row colors (erDiagram-oldHardcodedValues).
func erAttributes(v vars) {
	v.or("attributeBackgroundColorOdd", "#ffffff")
	v.or("attributeBackgroundColorEven", "#f2f2f2")
}

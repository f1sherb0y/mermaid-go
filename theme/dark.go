package theme

import "github.com/f1sherb0y/mermaid-go/color"

// theme-dark.js.
func init() {
	defs["dark"] = def{init: darkInit, update: darkUpdate}
}

func darkInit(v vars) {
	v.set("background", "#333")
	v.set("primaryColor", "#1f2020")
	v.set("secondaryColor", lighten(v["primaryColor"], 16))
	v.set("tertiaryColor", adjust(v["primaryColor"], dh(-160)))
	v.set("primaryBorderColor", invert(v["background"]))
	v.set("secondaryBorderColor", mkBorder(v["secondaryColor"], v.dark()))
	v.set("tertiaryBorderColor", mkBorder(v["tertiaryColor"], v.dark()))
	v.set("primaryTextColor", invert(v["primaryColor"]))
	v.set("secondaryTextColor", invert(v["secondaryColor"]))
	v.set("tertiaryTextColor", invert(v["tertiaryColor"]))
	v.set("lineColor", invert(v["background"]))
	v.set("textColor", invert(v["background"]))

	v.set("mainBkg", "#1f2020")
	v.set("secondBkg", "calculated")
	v.set("mainContrastColor", "lightgrey")
	v.set("darkTextColor", lighten(invert("#323D47"), 10))
	v.set("lineColor", "calculated")
	v.set("border1", "#ccc")
	v.set("border2", rgba(255, 255, 255, 0.25))
	v.set("arrowheadColor", "calculated")
	v.set("fontFamily", `"trebuchet ms", verdana, arial, sans-serif`)
	v.set("fontSize", "16px")
	v.set("labelBackground", "#181818")
	v.set("textColor", "#ccc")
	v.set("THEME_COLOR_LIMIT", 12)
	v.set("radius", 5)
	v.set("strokeWidth", 1)

	for _, k := range []string{"nodeBkg", "nodeBorder", "clusterBkg", "clusterBorder", "defaultLinkColor"} {
		v.set(k, "calculated")
	}
	v.set("titleColor", "#F9FFFE")
	v.set("edgeLabelBackground", "calculated")

	for _, k := range []string{"actorBorder", "actorBkg", "actorTextColor", "actorLineColor", "signalColor",
		"signalTextColor", "labelBoxBkgColor", "labelBoxBorderColor", "labelTextColor", "loopTextColor",
		"noteBorderColor"} {
		v.set(k, "calculated")
	}
	v.set("noteBkgColor", "#fff5ad")
	v.set("noteTextColor", "calculated")
	v.set("activationBorderColor", "calculated")
	v.set("activationBkgColor", "calculated")
	v.set("sequenceNumberColor", "black")
	v.set("clusterBkg", "#302F3D")

	v.set("sectionBkgColor", darken("#EAE8D9", 30))
	v.set("altSectionBkgColor", "calculated")
	v.set("sectionBkgColor2", "#EAE8D9")
	v.set("excludeBkgColor", darken(v["sectionBkgColor"], 10))
	v.set("taskBorderColor", rgba(255, 255, 255, 70))
	for _, k := range []string{"taskBkgColor", "taskTextColor", "taskTextLightColor", "taskTextOutsideColor"} {
		v.set(k, "calculated")
	}
	v.set("taskTextClickableColor", "#003163")
	v.set("activeTaskBorderColor", rgba(255, 255, 255, 50))
	v.set("activeTaskBkgColor", "#81B1DB")
	v.set("gridColor", "calculated")
	v.set("doneTaskBkgColor", "calculated")
	v.set("doneTaskBorderColor", "grey")
	v.set("critBorderColor", "#E83737")
	v.set("critBkgColor", "#E83737")
	v.set("taskTextDarkColor", "calculated")
	v.set("todayLineColor", "#DB5757")
	v.set("vertLineColor", "#00BFFF")

	v.set("personBorder", v["primaryBorderColor"])
	v.set("personBkg", v["mainBkg"])

	architectureInit(v)

	v.or("rowOdd", first(lighten(v["mainBkg"], 5), "#ffffff"))
	v.or("rowEven", darken(v["mainBkg"], 10))

	v.set("labelColor", "calculated")
	v.set("errorBkgColor", "#a44141")
	v.set("errorTextColor", "#ddd")
	v.set("useGradient", true)
	v.set("gradientStart", v["primaryBorderColor"])
	v.set("gradientStop", v["secondaryBorderColor"])
	v.set("dropShadow", "drop-shadow( 1px 2px 2px rgba(185,185,185,1))")
	v.or("noteFontWeight", "normal")
	v.or("fontWeight", "normal")
}

func darkUpdate(v vars) {
	contrast := v["mainContrastColor"]
	v.set("secondBkg", lighten(v["mainBkg"], 16))
	v.set("lineColor", contrast)
	v.set("arrowheadColor", contrast)

	v.set("nodeBkg", v["mainBkg"])
	v.set("nodeBorder", v["border1"])
	v.set("clusterBkg", v["secondBkg"])
	v.set("clusterBorder", v["border2"])
	v.set("defaultLinkColor", v["lineColor"])
	v.set("edgeLabelBackground", lighten(v["labelBackground"], 25))
	v.or("flowContainerStroke", v["secondaryBorderColor"])

	v.set("actorBorder", v["border1"])
	v.set("actorBkg", v["mainBkg"])
	v.set("actorTextColor", contrast)
	v.set("actorLineColor", v["actorBorder"])
	v.set("signalColor", contrast)
	v.set("signalTextColor", contrast)
	v.set("labelBoxBkgColor", v["actorBkg"])
	v.set("labelBoxBorderColor", v["actorBorder"])
	v.set("labelTextColor", contrast)
	v.set("loopTextColor", contrast)
	v.set("noteBorderColor", v["secondaryBorderColor"])
	v.set("noteBkgColor", v["secondBkg"])
	v.set("noteTextColor", v["secondaryTextColor"])
	v.set("activationBorderColor", v["border1"])
	v.set("activationBkgColor", v["secondBkg"])
	v.or("rectBkgColor", v["tertiaryColor"])

	v.set("altSectionBkgColor", v["background"])
	v.set("taskBkgColor", lighten(v["mainBkg"], 23))
	v.set("taskTextColor", v["darkTextColor"])
	v.set("taskTextLightColor", contrast)
	v.set("taskTextOutsideColor", v["taskTextLightColor"])
	v.set("gridColor", contrast)
	v.set("doneTaskBkgColor", contrast)
	v.set("taskTextDarkColor", invert(v["doneTaskBkgColor"]))

	v.set("archEdgeColor", v["lineColor"])
	v.set("archEdgeArrowColor", v["lineColor"])

	v.or("transitionColor", v["lineColor"])
	v.or("transitionLabelColor", v["textColor"])
	v.or("stateLabelColor", first(v["stateBkg"], v["primaryTextColor"]))
	v.or("stateBkg", v["mainBkg"])
	v.or("labelBackgroundColor", v["stateBkg"])
	v.or("compositeBackground", first(v["background"], v["tertiaryColor"]))
	v.or("altBackground", "#555")
	v.or("compositeTitleBackground", v["mainBkg"])
	v.or("compositeBorder", v["nodeBorder"])
	v.set("innerEndBackground", v["primaryBorderColor"])
	v.set("specialStateColor", "#f4f4f4")
	v.or("errorBkgColor", v["tertiaryColor"])
	v.or("errorTextColor", v["tertiaryTextColor"])

	fillTypes(v, v["primaryColor"], v["secondaryColor"], true)

	// cScale1..12 get fixed colors, so of the primaryColor-derived defaults mermaid then
	// gives cScale0..11 only cScale0's can apply.
	for i, c := range []string{"#0b0000", "#4d1037", "#3f5258", "#4f2f1b", "#6e0a0a", "#3b0048",
		"#995a01", "#154706", "#161722", "#00296f", "#01629c", "#010029"} {
		v.or(key("cScale", i+1), c)
	}
	v.or("cScale0", v["primaryColor"])
	for i := range v.limit() {
		v.or(key("cScaleInv", i), invert(v[key("cScale", i)]))
	}
	for i := range v.limit() {
		v.or(key("cScalePeer", i), lighten(v[key("cScale", i)], 10))
	}
	for i := range 5 {
		fi := float64(i)
		v.or(key("surface", i), adjust(v["mainBkg"], color.Delta{H: 30, S: -30, L: -(-10 + fi*4)}))
		v.or(key("surfacePeer", i), adjust(v["mainBkg"], color.Delta{H: 30, S: -30, L: -(-7 + fi*4)}))
	}
	v.or("scaleLabelColor", pick(v.dark(), "black", v["labelTextColor"]))
	for i := range v.limit() {
		v.or(key("cScaleLabel", i), v["scaleLabelColor"])
	}

	// pie0..11, not pie1..12.
	for i := range v.limit() {
		v.set(key("pie", i), v[key("cScale", i)])
	}
	v.or("pieTitleTextSize", "25px")
	v.or("pieTitleTextColor", contrast)
	v.or("pieSectionTextSize", "17px")
	v.or("pieSectionTextColor", v["textColor"])
	v.or("pieLegendTextSize", "17px")
	v.or("pieLegendTextColor", contrast)
	v.or("pieStrokeColor", "black")
	v.or("pieStrokeWidth", "2px")
	v.or("pieOuterStrokeWidth", "2px")
	v.or("pieOuterStrokeColor", "black")
	v.or("pieOpacity", "0.7")

	for i := range 8 {
		v.ifNull(key("venn", i+1), lighten(v[key("cScale", i)], 30))
	}
	v.ifNull("vennTitleTextColor", v["titleColor"])
	v.ifNull("vennSetTextColor", v["textColor"])

	v.nested("cynefin",
		field{"domainFontSize", 16},
		field{"itemFontSize", 12},
		field{"boundaryColor", v["lineColor"]},
		field{"boundaryWidth", 2},
		field{"cliffColor", "#FF6B6B"},
		field{"cliffWidth", 4},
		field{"arrowColor", v["lineColor"]},
		field{"arrowWidth", 2},
		field{"complexBg", "#1B5E20"},
		field{"complicatedBg", "#0D47A1"},
		field{"chaoticBg", "#BF360C"},
		field{"clearBg", "#F57F17"},
		field{"confusionBg", "#4A148C"},
		field{"textColor", v["textColor"]},
		field{"labelColor", v["primaryTextColor"]},
	)

	quadrant(v, v["primaryColor"])
	xyChart(v, "#3498db,#2ecc71,#e74c3c,#f1c40f,#bdc3c7,#ffffff,#34495e,#9b59b6,#1abc9c,#e67e22", true)
	packet(v, v["background"])
	radar(v)

	v.or("wardleyEvolutionColor", "#ff6b6b")
	v.nested("wardley",
		field{"backgroundColor", v["background"]},
		field{"axisColor", v["lineColor"]},
		field{"axisTextColor", v["primaryTextColor"]},
		field{"gridColor", v["gridColor"]},
		field{"componentFill", v["mainBkg"]},
		field{"componentStroke", v["lineColor"]},
		field{"componentLabelColor", v["primaryTextColor"]},
		field{"linkStroke", v["lineColor"]},
		field{"evolutionStroke", v["wardleyEvolutionColor"]},
		field{"annotationStroke", v["lineColor"]},
		field{"annotationTextColor", v["primaryTextColor"]},
		field{"annotationFill", v["mainBkg"]},
	)

	v.set("classText", v["primaryTextColor"])

	requirement(v, v["primaryColor"], labelOnSecondary(v))

	// git0 lightens secondaryColor, git1..7 the pie colors (which are cScale1..7 here).
	v.set("git0", lighten(v["secondaryColor"], 20))
	v.set("git1", lighten(first(v["pie2"], v["secondaryColor"]), 20))
	v.set("git2", lighten(first(v["pie3"], v["tertiaryColor"]), 20))
	for i, g := range []struct{ deg, amount float64 }{{-30, 20}, {-60, 20}, {-90, 10}, {60, 10}, {120, 20}} {
		v.set(key("git", i+3), lighten(first(v[key("pie", i+4)], adjust(v["primaryColor"], dh(g.deg))), g.amount))
	}
	gitInverses(v)
	classicBranchLabels(v)
	gitTagLabels(v)

	v.or("emUiFill", "#2d2d2d")
	v.or("emUiStroke", "#555")
	v.or("emProcessorFill", lighten("#5a3d5c", 10))
	v.or("emProcessorStroke", "#8a6d8c")
	v.or("emReadModelFill", lighten("#3d5a2d", 10))
	v.or("emReadModelStroke", "#6d8c5c")
	v.or("emCommandFill", lighten("#2d3d5a", 10))
	v.or("emCommandStroke", "#5c6d8c")
	v.or("emEventFill", lighten("#5a452d", 10))
	v.or("emEventStroke", "#8c755c")
	v.or("emSwimlaneBackgroundOdd", lighten(v["background"], 5))
	v.or("emSwimlaneBackgroundStroke", lighten(v["background"], 12))
	v.or("emArrowhead", v["lineColor"])
	v.or("emRelationStroke", v["lineColor"])

	v.or("attributeBackgroundColorOdd", lighten(v["background"], 12))
	v.or("attributeBackgroundColorEven", lighten(v["background"], 2))

	v.or("nodeBorder", "#999")
}

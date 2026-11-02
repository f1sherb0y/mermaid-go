package theme

import "github.com/f1sherb0y/mermaid-go/color"

// theme-forest.js.
func init() {
	defs["forest"] = def{init: forestInit, update: forestUpdate}
}

func forestInit(v vars) {
	v.set("background", "#f4f4f4")
	v.set("primaryColor", "#cde498")
	v.set("secondaryColor", "#cdffb2")
	v.set("background", "white")
	v.set("mainBkg", "#cde498")
	v.set("secondBkg", "#cdffb2")
	v.set("lineColor", "green")
	v.set("border1", "#13540c")
	v.set("border2", "#6eaa49")
	v.set("arrowheadColor", "green")
	v.set("fontFamily", `"trebuchet ms", verdana, arial, sans-serif`)
	v.set("fontSize", "16px")

	v.set("tertiaryColor", lighten("#cde498", 10))
	v.set("primaryBorderColor", mkBorder(v["primaryColor"], v.dark()))
	v.set("secondaryBorderColor", mkBorder(v["secondaryColor"], v.dark()))
	v.set("tertiaryBorderColor", mkBorder(v["tertiaryColor"], v.dark()))
	v.set("primaryTextColor", invert(v["primaryColor"]))
	v.set("secondaryTextColor", invert(v["secondaryColor"]))
	v.set("tertiaryTextColor", invert(v["primaryColor"]))
	v.set("lineColor", invert(v["background"]))
	v.set("textColor", invert(v["background"]))
	v.set("THEME_COLOR_LIMIT", 12)
	v.set("radius", 5)
	v.set("strokeWidth", 1)

	for _, k := range []string{"nodeBkg", "nodeBorder", "clusterBkg", "clusterBorder", "defaultLinkColor"} {
		v.set(k, "calculated")
	}
	v.set("titleColor", "#333")
	v.set("edgeLabelBackground", "#e8e8e8")

	v.set("actorBorder", "calculated")
	v.set("actorBkg", "calculated")
	v.set("actorTextColor", "black")
	v.set("actorLineColor", "calculated")
	v.set("signalColor", "#333")
	v.set("signalTextColor", "#333")
	v.set("labelBoxBkgColor", "calculated")
	v.set("labelBoxBorderColor", "#326932")
	v.set("labelTextColor", "calculated")
	v.set("loopTextColor", "calculated")
	v.set("noteBorderColor", "calculated")
	v.set("noteBkgColor", "#fff5ad")
	v.set("noteTextColor", "calculated")
	v.set("activationBorderColor", "#666")
	v.set("activationBkgColor", "#f4f4f4")
	v.set("sequenceNumberColor", "white")

	v.set("sectionBkgColor", "#6eaa49")
	v.set("altSectionBkgColor", "white")
	v.set("sectionBkgColor2", "#6eaa49")
	v.set("excludeBkgColor", "#eeeeee")
	v.set("taskBorderColor", "calculated")
	v.set("taskBkgColor", "#487e3a")
	v.set("taskTextLightColor", "white")
	v.set("taskTextColor", "calculated")
	v.set("taskTextDarkColor", "black")
	v.set("taskTextOutsideColor", "calculated")
	v.set("taskTextClickableColor", "#003163")
	v.set("activeTaskBorderColor", "calculated")
	v.set("activeTaskBkgColor", "calculated")
	v.set("gridColor", "lightgrey")
	v.set("doneTaskBkgColor", "lightgrey")
	v.set("doneTaskBorderColor", "grey")
	v.set("critBorderColor", "#ff8888")
	v.set("critBkgColor", "red")
	v.set("todayLineColor", "red")
	v.set("vertLineColor", "#00BFFF")

	v.set("personBorder", v["primaryBorderColor"])
	v.set("personBkg", v["mainBkg"])

	architectureInit(v)

	v.set("noteFontWeight", "normal")
	v.set("fontWeight", "normal")
	v.set("labelColor", "black")
	v.set("errorBkgColor", "#552222")
	v.set("errorTextColor", "#552222")
	v.set("useGradient", true)
	v.set("gradientStart", v["primaryBorderColor"])
	v.set("gradientStop", v["secondaryBorderColor"])
	v.set("dropShadow", "drop-shadow( 1px 2px 2px rgba(185,185,185,0.5))")
}

func forestUpdate(v vars) {
	v.set("actorBorder", darken(v["mainBkg"], 20))
	v.set("actorBkg", v["mainBkg"])
	v.set("labelBoxBkgColor", v["actorBkg"])
	v.set("labelTextColor", v["actorTextColor"])
	v.set("loopTextColor", v["actorTextColor"])
	v.set("noteBorderColor", v["border2"])
	v.set("noteTextColor", v["actorTextColor"])
	v.set("actorLineColor", v["actorBorder"])
	v.or("rectBkgColor", v["tertiaryColor"])

	classicScale(v)
	if s := v["scaleLabelColor"]; s == "calculated" || !truthy(s) {
		v.set("scaleLabelColor", v["labelTextColor"])
	}
	for i := range v.limit() {
		v.or(key("cScaleLabel", i), v["scaleLabelColor"])
	}
	for i := range 5 {
		fi := float64(i)
		v.or(key("surface", i), adjust(v["mainBkg"], color.Delta{H: 30, S: -30, L: -(5 + fi*5)}))
		v.or(key("surfacePeer", i), adjust(v["mainBkg"], color.Delta{H: 30, S: -30, L: -(8 + fi*5)}))
	}

	v.set("nodeBkg", v["mainBkg"])
	v.set("nodeBorder", v["border1"])
	v.set("clusterBkg", v["secondBkg"])
	v.set("clusterBorder", v["border2"])
	v.set("defaultLinkColor", v["lineColor"])
	v.or("flowContainerStroke", v["secondaryBorderColor"])

	v.set("taskBorderColor", v["border1"])
	v.set("taskTextColor", v["taskTextLightColor"])
	v.set("taskTextOutsideColor", v["taskTextDarkColor"])
	v.set("activeTaskBorderColor", v["taskBorderColor"])
	v.set("activeTaskBkgColor", v["mainBkg"])

	v.set("archEdgeColor", v["lineColor"])
	v.set("archEdgeArrowColor", v["lineColor"])

	v.or("rowOdd", first(lighten(v["mainBkg"], 75), "#ffffff"))
	v.or("rowEven", lighten(v["mainBkg"], 20))

	v.or("transitionColor", v["lineColor"])
	v.or("transitionLabelColor", v["textColor"])
	v.or("stateLabelColor", first(v["stateBkg"], v["primaryTextColor"]))
	v.or("stateBkg", v["mainBkg"])
	v.or("labelBackgroundColor", v["stateBkg"])
	v.or("compositeBackground", first(v["background"], v["tertiaryColor"]))
	v.or("altBackground", "#f0f0f0")
	v.or("compositeTitleBackground", v["mainBkg"])
	v.or("compositeBorder", v["nodeBorder"])
	v.set("innerEndBackground", v["primaryBorderColor"])
	v.set("specialStateColor", v["lineColor"])
	v.or("errorBkgColor", v["tertiaryColor"])
	v.or("errorTextColor", v["tertiaryTextColor"])

	v.set("classText", v["primaryTextColor"])
	fillTypes(v, v["primaryColor"], v["secondaryColor"], true)

	v.or("pie1", v["primaryColor"])
	v.or("pie2", v["secondaryColor"])
	v.or("pie3", v["tertiaryColor"])
	v.or("pie4", adjust(v["primaryColor"], dl(-30)))
	v.or("pie5", adjust(v["secondaryColor"], dl(-30)))
	v.or("pie6", adjust(v["tertiaryColor"], dhl(40, -40)))
	v.or("pie7", adjust(v["primaryColor"], dhl(60, -10)))
	v.or("pie8", adjust(v["primaryColor"], dhl(-60, -10)))
	v.or("pie9", adjust(v["primaryColor"], dhl(120, 0)))
	v.or("pie10", adjust(v["primaryColor"], dhl(60, -50)))
	v.or("pie11", adjust(v["primaryColor"], dhl(-60, -50)))
	v.or("pie12", adjust(v["primaryColor"], dhl(120, -50)))
	pieText(v)

	vennWheel(v, -30)

	v.nested("cynefin",
		field{"domainFontSize", 16},
		field{"itemFontSize", 12},
		field{"boundaryColor", v["lineColor"]},
		field{"boundaryWidth", 2},
		field{"cliffColor", "#8B4513"},
		field{"cliffWidth", 4},
		field{"arrowColor", v["lineColor"]},
		field{"arrowWidth", 2},
		field{"complexBg", "#C8E6C9"},
		field{"complicatedBg", "#DCEDC8"},
		field{"chaoticBg", "#FFE0B2"},
		field{"clearBg", "#FFF9C4"},
		field{"confusionBg", "#D7CCC8"},
		field{"textColor", v["textColor"]},
		field{"labelColor", v["primaryTextColor"]},
	)

	quadrant(v, v["primaryColor"])
	packet(v, v["mainBkg"])
	radar(v)
	wardley(v)
	xyChart(v, "#CDE498,#FF6B6B,#A0D2DB,#D7BDE2,#F0F0F0,#FFC3A0,#7FD8BE,#FF9A8B,#FAF3E0,#FFF176", true)
	requirement(v, v["primaryColor"], v["edgeLabelBackground"])

	gitWheel(v, v["primaryColor"], v["secondaryColor"], v["tertiaryColor"])
	gitShade(v)
	gitInverses(v)
	classicBranchLabels(v)
	gitTagLabels(v)

	eventModeling(v)
	erAttributes(v)
}

package theme

// theme-default.js: mermaid's classic lavender theme. Its constructor runs updateColors
// once while many values are still 'calculated'; calculate() then clears those and runs it
// again, so some colors (the cScale ones) are darkened twice.
func init() {
	defs["default"] = def{init: defaultInit, update: defaultUpdate, clearCalculated: true}
}

func defaultInit(v vars) {
	v.set("primaryColor", "#ECECFF")
	v.set("secondaryColor", "#ffffde")
	v.set("tertiaryColor", adjust(v["primaryColor"], dh(-160)))
	v.set("primaryBorderColor", mkBorder(v["primaryColor"], false))
	v.set("secondaryBorderColor", mkBorder(v["secondaryColor"], false))
	v.set("tertiaryBorderColor", mkBorder(v["tertiaryColor"], false))
	v.set("primaryTextColor", invert(v["primaryColor"]))
	v.set("secondaryTextColor", invert(v["secondaryColor"]))
	v.set("tertiaryTextColor", invert(v["tertiaryColor"]))

	for _, kv := range [][2]string{
		{"background", "white"}, {"mainBkg", "#ECECFF"}, {"secondBkg", "#ffffde"}, {"lineColor", "#333333"},
		{"border1", "#9370DB"}, {"border2", "#aaaa33"}, {"arrowheadColor", "#333333"},
		{"fontFamily", `"trebuchet ms", verdana, arial, sans-serif`}, {"fontSize", "16px"},
		{"labelBackground", "rgba(232,232,232, 0.8)"}, {"textColor", "#333"},
		// Sequence
		{"actorTextColor", "black"}, {"noteBkgColor", "#fff5ad"}, {"activationBorderColor", "#666"},
		{"activationBkgColor", "#f4f4f4"}, {"sequenceNumberColor", "white"}, {"clusterBkg", "#FBFBFF"},
		// Gantt
		{"excludeBkgColor", "#eeeeee"}, {"altSectionBkgColor", "white"}, {"sectionBkgColor2", "#fff400"},
		{"taskBorderColor", "#534fbc"}, {"taskBkgColor", "#8a90dd"}, {"taskTextLightColor", "white"},
		{"taskTextDarkColor", "black"}, {"taskTextClickableColor", "#003163"}, {"activeTaskBorderColor", "#534fbc"},
		{"activeTaskBkgColor", "#bfc7ff"}, {"gridColor", "lightgrey"}, {"doneTaskBkgColor", "lightgrey"},
		{"doneTaskBorderColor", "grey"}, {"critBorderColor", "#ff8888"}, {"critBkgColor", "red"},
		{"todayLineColor", "red"}, {"vertLineColor", "navy"},
		{"noteFontWeight", "normal"}, {"fontWeight", "normal"},
		{"archEdgeWidth", "3"}, {"archGroupBorderWidth", "2px"},
		{"labelColor", "black"}, {"errorBkgColor", "#552222"}, {"errorTextColor", "#552222"},
		{"dropShadow", "drop-shadow(1px 2px 2px rgba(185, 185, 185, 1))"},
	} {
		v.set(kv[0], kv[1])
	}
	for _, k := range []string{
		"nodeBkg", "nodeBorder", "clusterBorder", "defaultLinkColor", "titleColor", "edgeLabelBackground",
		"actorBorder", "actorBkg", "actorLineColor", "signalColor", "signalTextColor", "labelBoxBkgColor",
		"labelBoxBorderColor", "labelTextColor", "loopTextColor", "noteBorderColor", "noteTextColor",
		"taskTextColor", "taskTextOutsideColor", "archEdgeColor", "archEdgeArrowColor", "rowOdd", "rowEven",
	} {
		v.set(k, "calculated")
	}
	v.set("THEME_COLOR_LIMIT", 12)
	v.set("radius", 5)
	v.set("strokeWidth", 1)
	v.set("sectionBkgColor", rgba(102, 102, 255, 0.49))
	v.set("personBorder", v["primaryBorderColor"])
	v.set("personBkg", v["mainBkg"])
	v.set("archGroupBorderColor", v["primaryBorderColor"])
	v.set("useGradient", false)
	v.set("gradientStart", v["primaryBorderColor"])
	v.set("gradientStop", v["secondaryBorderColor"])
	defaultUpdate(v)
}

func defaultUpdate(v vars) {
	// Color scale
	classicScale(v)
	for i := range 5 {
		fi := float64(i)
		v.or(key("surface", i), adjust(v["mainBkg"], dhl(30, -(5+fi*5))))
		v.or(key("surfacePeer", i), adjust(v["mainBkg"], dhl(30, -(7+fi*5))))
	}
	if sl := v["scaleLabelColor"]; !truthy(sl) || sl == "calculated" {
		v.set("scaleLabelColor", v["labelTextColor"])
	}
	if label := v["labelTextColor"]; label != "calculated" {
		v.or("cScaleLabel0", invert(label))
		v.or("cScaleLabel3", invert(label))
		for i := range v.limit() {
			v.or(key("cScaleLabel", i), label)
		}
	}

	// Flowchart
	v.set("nodeBkg", v["mainBkg"])
	v.set("nodeBorder", v["border1"])
	v.set("clusterBkg", v["secondBkg"])
	v.set("clusterBorder", v["border2"])
	v.set("defaultLinkColor", v["lineColor"])
	v.set("titleColor", v["textColor"])
	v.set("edgeLabelBackground", v["labelBackground"])
	v.or("flowContainerStroke", v["secondaryBorderColor"])

	// Sequence
	v.set("actorBorder", v["border1"])
	v.set("actorBkg", v["mainBkg"])
	v.set("labelBoxBkgColor", v["actorBkg"])
	v.set("signalColor", v["textColor"])
	v.set("signalTextColor", v["textColor"])
	v.set("labelBoxBorderColor", v["actorBorder"])
	v.set("labelTextColor", v["actorTextColor"])
	v.set("loopTextColor", v["actorTextColor"])
	v.set("noteBorderColor", v["border2"])
	v.set("noteTextColor", v["actorTextColor"])
	v.set("actorLineColor", v["actorBorder"])
	v.or("rectBkgColor", v["tertiaryColor"])

	// Gantt, architecture
	v.set("taskTextColor", v["taskTextLightColor"])
	v.set("taskTextOutsideColor", v["taskTextDarkColor"])
	v.set("archEdgeColor", v["lineColor"])
	v.set("archEdgeArrowColor", v["lineColor"])

	// ER
	v.or("rowOdd", first(lighten(v["primaryColor"], 75), "#ffffff"))
	v.or("rowEven", lighten(v["primaryColor"], 1))

	// State
	v.or("transitionColor", v["lineColor"])
	v.or("transitionLabelColor", v["textColor"])
	v.or("stateLabelColor", first(v["stateBkg"], v["primaryTextColor"]))
	v.or("stateBkg", v["mainBkg"])
	v.or("labelBackgroundColor", v["stateBkg"])
	v.or("compositeBackground", first(v["background"], v["tertiaryColor"]))
	v.or("altBackground", "#f0f0f0")
	v.or("compositeTitleBackground", v["mainBkg"])
	v.or("compositeBorder", v["nodeBorder"])
	v.set("innerEndBackground", v["nodeBorder"])
	v.set("specialStateColor", v["lineColor"])
	v.or("errorBkgColor", v["tertiaryColor"])
	v.or("errorTextColor", v["tertiaryTextColor"])

	v.set("classText", v["primaryTextColor"])

	fillTypes(v, v["primaryColor"], v["secondaryColor"], true)

	// Pie
	v.or("pie1", v["primaryColor"])
	v.or("pie2", v["secondaryColor"])
	v.or("pie3", adjust(v["tertiaryColor"], dl(-40)))
	v.or("pie4", adjust(v["primaryColor"], dl(-10)))
	v.or("pie5", adjust(v["secondaryColor"], dl(-30)))
	v.or("pie6", adjust(v["tertiaryColor"], dl(-20)))
	v.or("pie7", adjust(v["primaryColor"], dhl(60, -20)))
	v.or("pie8", adjust(v["primaryColor"], dhl(-60, -40)))
	v.or("pie9", adjust(v["primaryColor"], dhl(120, -40)))
	v.or("pie10", adjust(v["primaryColor"], dhl(60, -40)))
	v.or("pie11", adjust(v["primaryColor"], dhl(-90, -40)))
	v.or("pie12", adjust(v["primaryColor"], dhl(120, -30)))
	pieText(v)

	vennWheel(v, -40)

	cynefin(v)
	quadrant(v, v["primaryColor"])
	radar(v)
	wardley(v)
	xyChart(v, "#ECECFF,#8493A6,#FFC3A0,#DCDDE1,#B8E994,#D1A36F,#C3CDE6,#FFB6C1,#496078,#F8F3E3", true)

	requirement(v, v["primaryColor"], v["labelBackground"])

	gitWheel(v, v["primaryColor"], v["secondaryColor"], v["tertiaryColor"])
	gitShade(v)
	v.or("gitInv0", darken(invert(v["git0"]), 25))
	for i := 1; i < 8; i++ {
		v.or(key("gitInv", i), invert(v[key("git", i)]))
	}
	classicBranchLabels(v)
	gitTagLabels(v)

	eventModeling(v)
	erAttributes(v)
}

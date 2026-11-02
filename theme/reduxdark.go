package theme

// theme-redux-dark.js: the redux look on a dark canvas.
func init() {
	defs["redux-dark"] = def{init: reduxDarkInit, update: reduxDarkUpdate}
}

// reduxDarkInit is the constructor; redux-dark-color's adds its arrays to it.
func reduxDarkInit(v vars) {
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
	v.set("mainBkg", "#111113")
	v.set("secondBkg", "calculated")
	v.set("mainContrastColor", "lightgrey")
	v.set("darkTextColor", lighten(invert("#323D47"), 10))
	v.set("border1", "#ccc")
	v.set("border2", rgba(255, 255, 255, 0.25))
	v.set("arrowheadColor", invert(v["background"]))
	v.set("fontFamily", `"Recursive Variable", arial, sans-serif`)
	v.set("fontSize", "14px")
	v.set("labelBackground", "#111113")
	v.set("textColor", "#ccc")
	v.set("THEME_COLOR_LIMIT", 12)
	v.set("radius", 12)
	v.set("strokeWidth", 2)
	v.ifNull("noteBkgColor", "#FEF9C3")
	v.ifNull("noteTextColor", "#28253D")
	v.set("nodeBorder", "#FFFFFF")
	v.set("stateBorder", "#FFFFFF")
	v.set("useGradient", false)
	v.set("gradientStart", "#0042eb")
	v.set("gradientStop", "#eb0042")
	v.set("dropShadow", "url(#drop-shadow)")
	v.set("nodeShadow", true)
	v.set("archEdgeColor", "calculated")
	v.set("archEdgeArrowColor", "calculated")
	v.set("archEdgeWidth", "3")
	v.set("archGroupBorderColor", v["primaryBorderColor"])
	v.set("archGroupBorderWidth", "2px")
	v.set("clusterBkg", "#1E1A2E")
	v.set("clusterBorder", "#BDBCCC")
	v.set("noteBorderColor", "#FACC15")
	v.set("noteFontWeight", 600)
	v.set("filterColor", "#FFFFFF")
}

// reduxDarkTop is the start of both dark redux themes' updateColors, through the sequence
// diagram's colors.
func reduxDarkTop(v vars) {
	mainColors(v, "#FFFFFF", "#fff5ad")
	flowchartColors(v, v["border1"])
	// The constructor's 'calculated', which mermaid resolves here.
	if v["secondBkg"] == "calculated" {
		v.set("secondBkg", lighten(v["mainBkg"], 16))
	}
	v.set("actorBorder", "#FFFFFF")
	v.set("signalColor", "#FFFFFF")
	v.set("labelBoxBorderColor", "#BDBCCC")
	sequenceColors(v)
	v.or("rectBkgColor", v["tertiaryColor"])
}

func reduxDarkUpdate(v vars) {
	reduxDarkTop(v)

	// Gantt
	v.or("sectionBkgColor", v["tertiaryColor"])
	v.or("altSectionBkgColor", "white")
	v.or("sectionBkgColor2", v["primaryColor"])
	v.or("excludeBkgColor", "#eeeeee")
	v.or("taskBorderColor", v["primaryBorderColor"])
	v.or("taskBkgColor", v["primaryColor"])
	v.or("activeTaskBorderColor", v["primaryColor"])
	v.or("activeTaskBkgColor", lighten(v["primaryColor"], 23))
	v.or("gridColor", "lightgrey")
	v.or("doneTaskBkgColor", v["secondBkg"])
	v.or("doneTaskBorderColor", "grey")
	v.or("critBorderColor", "#ff8888")
	v.or("critBkgColor", "red")
	v.or("todayLineColor", "red")
	v.or("taskTextColor", v["textColor"])
	v.or("taskTextOutsideColor", v["textColor"])
	v.or("taskTextLightColor", v["textColor"])
	v.or("taskTextDarkColor", v["textColor"])
	v.or("taskTextClickableColor", "#003163")

	v.set("archEdgeColor", v["lineColor"])
	v.set("archEdgeArrowColor", v["lineColor"])
	v.or("personBorder", v["primaryBorderColor"])
	v.or("personBkg", v["mainBkg"])

	// State
	v.or("transitionColor", v["lineColor"])
	v.or("transitionLabelColor", v["textColor"])
	v.or("stateLabelColor", first(v["stateBkg"], v["primaryTextColor"]))
	v.or("vertLineColor", v["primaryBorderColor"])
	for _, k := range []string{"compositeBackground", "altBackground", "compositeTitleBackground", "stateEdgeLabelBackground"} {
		v.set(k, "#16141F")
	}
	v.set("fontWeight", 600)
	v.or("stateBkg", v["mainBkg"])
	v.or("labelBackgroundColor", v["stateBkg"])
	v.or("compositeBorder", v["nodeBorder"])
	v.set("innerEndBackground", v["nodeBorder"])
	v.or("errorBkgColor", v["tertiaryColor"])
	v.or("errorTextColor", v["tertiaryTextColor"])
	v.set("specialStateColor", v["lineColor"])

	// Color scale, as theme-base's.
	cScaleWheel(v, v["primaryColor"], v["secondaryColor"], v["tertiaryColor"])
	cScaleDarken(v)
	cScaleSets(v)
	surfaces(v)
	v.or("classText", v["textColor"])

	fillTypes(v, v["primaryColor"], v["secondaryColor"], false)

	pieWheel(v, v["primaryColor"], v["secondaryColor"], v["tertiaryColor"])
	pieText(v)

	v.ifNull("vennTitleTextColor", v["titleColor"])
	v.ifNull("vennSetTextColor", v["textColor"])
	reduxDarkBottom(v)
}

// reduxDarkBottom is the end of both dark redux themes' updateColors, from the quadrant chart.
func reduxDarkBottom(v vars) {
	quadrant(v, v["primaryColor"])
	xyChart(v, "#FFF4DD,#FFD8B1,#FFA07A,#ECEFF1,#D6DBDF,#C3E0A8,#FFB6A4,#FFD74D,#738FA7,#FFFFF0", false)
	requirement(v, v["primaryColor"], labelOnSecondary(v))
	v.set("requirementEdgeLabelBackground", "#16141F")
	gitWheel(v, v["primaryColor"], v["secondaryColor"], v["tertiaryColor"])
	gitShade(v)
	gitInverses(v)
	gitLabels(v)
	v.ifNull("commitLineColor", "#BDBCCC")
	v.set("erEdgeLabelBackground", "#16141F")
	erAttributes(v)
}

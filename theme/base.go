package theme

// theme-base.js: everything derived from a few base colors; the theme meant for customizing.
func init() {
	defs["base"] = def{init: baseInit, update: baseUpdate, gradientYieldsToNodeBorder: true}
}

func baseInit(v vars) {
	v.set("background", "#f4f4f4")
	v.set("primaryColor", "#fff4dd")
	v.set("noteBkgColor", "#fff5ad")
	v.set("noteTextColor", "#333")
	v.set("THEME_COLOR_LIMIT", 12)
	v.set("radius", 5)
	v.set("strokeWidth", 1)
	v.set("fontFamily", `"trebuchet ms", verdana, arial, sans-serif`)
	v.set("fontSize", "16px")
	v.set("useGradient", true)
	v.set("dropShadow", "drop-shadow( 1px 2px 2px rgba(185,185,185,1))")
}

func baseUpdate(v vars) {
	dark := v.dark()
	mainColors(v, "#333", "#fff5ad")

	flowchartColors(v, v["primaryBorderColor"])

	sequenceColors(v)
	v.or("rectBkgColor", v["tertiaryColor"])

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
	v.or("doneTaskBkgColor", "lightgrey")
	v.or("doneTaskBorderColor", "grey")
	v.or("critBorderColor", "#ff8888")
	v.or("critBkgColor", "red")
	v.or("todayLineColor", "red")
	v.or("vertLineColor", "navy")
	v.or("taskTextColor", v["textColor"])
	v.or("taskTextOutsideColor", v["textColor"])
	v.or("taskTextLightColor", v["textColor"])
	v.or("taskTextDarkColor", v["textColor"])
	v.or("taskTextClickableColor", "#003163")
	v.or("noteFontWeight", "normal")
	v.or("fontWeight", "normal")

	// C4
	v.or("personBorder", v["primaryBorderColor"])
	v.or("personBkg", v["mainBkg"])

	// ER
	if dark {
		v.or("rowOdd", first(darken(v["mainBkg"], 5), "#ffffff"))
		v.or("rowEven", darken(v["mainBkg"], 10))
	} else {
		v.or("rowOdd", first(lighten(v["mainBkg"], 75), "#ffffff"))
		v.or("rowEven", lighten(v["mainBkg"], 5))
	}

	// State
	v.or("transitionColor", v["lineColor"])
	v.or("transitionLabelColor", v["textColor"])
	v.or("stateLabelColor", first(v["stateBkg"], v["primaryTextColor"]))
	v.or("stateBkg", v["mainBkg"])
	v.or("labelBackgroundColor", v["stateBkg"])
	v.or("compositeBackground", first(v["background"], v["tertiaryColor"]))
	v.or("altBackground", v["tertiaryColor"])
	v.or("compositeTitleBackground", v["mainBkg"])
	v.or("compositeBorder", v["nodeBorder"])
	v.set("innerEndBackground", v["nodeBorder"])
	v.or("errorBkgColor", v["tertiaryColor"])
	v.or("errorTextColor", v["tertiaryTextColor"])
	v.set("specialStateColor", v["lineColor"])

	// Color scale: the base colors, then the primary color around the wheel.
	cScaleWheel(v, v["primaryColor"], v["secondaryColor"], v["tertiaryColor"])
	cScaleDarken(v)
	cScaleSets(v)
	surfaces(v)

	v.or("classText", v["textColor"])

	fillTypes(v, v["primaryColor"], v["secondaryColor"], false)

	pieWheel(v, v["primaryColor"], v["secondaryColor"], v["tertiaryColor"])
	pieText(v)

	vennWheel(v, -30)

	cynefin(v)
	radar(v)
	wardley(v)

	// Architecture
	v.or("archEdgeColor", "#777")
	v.or("archEdgeArrowColor", "#777")
	v.or("archEdgeWidth", "3")
	v.or("archGroupBorderColor", "#000")
	v.or("archGroupBorderWidth", "2px")

	quadrant(v, v["primaryColor"])
	xyChart(v, "#FFF4DD,#FFD8B1,#FFA07A,#ECEFF1,#D6DBDF,#C3E0A8,#FFB6A4,#FFD74D,#738FA7,#FFFFF0", true)

	requirement(v, v["primaryColor"], labelOnSecondary(v))

	gitWheel(v, v["primaryColor"], v["secondaryColor"], v["tertiaryColor"])
	gitShade(v)
	gitInverses(v)
	gitLabels(v)

	eventModeling(v)
	erAttributes(v)
	v.set("gradientStart", v["primaryBorderColor"])
	v.set("gradientStop", v["secondaryBorderColor"])
}

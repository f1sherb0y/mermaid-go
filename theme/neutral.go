package theme

// theme-neutral.js: grays.
func init() {
	defs["neutral"] = def{init: neutralInit, update: neutralUpdate}
}

func neutralInit(v vars) {
	v.set("primaryColor", "#eee")
	v.set("contrast", "#707070")
	v.set("secondaryColor", lighten(v["contrast"], 55))
	v.set("background", "#ffffff")

	v.set("tertiaryColor", adjust(v["primaryColor"], dh(-160)))
	v.set("primaryBorderColor", mkBorder(v["primaryColor"], v.dark()))
	v.set("secondaryBorderColor", mkBorder(v["secondaryColor"], v.dark()))
	v.set("tertiaryBorderColor", mkBorder(v["tertiaryColor"], v.dark()))
	v.set("primaryTextColor", invert(v["primaryColor"]))
	v.set("secondaryTextColor", invert(v["secondaryColor"]))
	v.set("tertiaryTextColor", invert(v["tertiaryColor"]))
	v.set("lineColor", invert(v["background"]))
	v.set("textColor", invert(v["background"]))

	v.set("mainBkg", "#eee")
	v.set("secondBkg", "calculated")
	v.set("lineColor", "#666")
	v.set("border1", "#999")
	v.set("border2", "calculated")
	v.set("note", "#ffa")
	v.set("text", "#333")
	v.set("critical", "#d42")
	v.set("done", "#bbb")
	v.set("arrowheadColor", "#333333")
	v.set("fontFamily", `"trebuchet ms", verdana, arial, sans-serif`)
	v.set("fontSize", "16px")
	v.set("THEME_COLOR_LIMIT", 12)
	v.set("radius", 5)
	v.set("strokeWidth", 1)

	for _, k := range []string{"nodeBkg", "nodeBorder", "clusterBkg", "clusterBorder", "defaultLinkColor", "titleColor"} {
		v.set(k, "calculated")
	}
	v.set("edgeLabelBackground", "white")

	v.set("actorBorder", "calculated")
	v.set("actorBkg", "calculated")
	v.set("actorTextColor", "calculated")
	v.set("actorLineColor", v["actorBorder"])
	for _, k := range []string{"signalColor", "signalTextColor", "labelBoxBkgColor", "labelBoxBorderColor",
		"labelTextColor", "loopTextColor", "noteBorderColor", "noteBkgColor", "noteTextColor"} {
		v.set(k, "calculated")
	}
	v.set("activationBorderColor", "#666")
	v.set("activationBkgColor", "#f4f4f4")
	v.set("sequenceNumberColor", "white")

	v.set("sectionBkgColor", "calculated")
	v.set("altSectionBkgColor", "white")
	v.set("sectionBkgColor2", "calculated")
	v.set("excludeBkgColor", "#eeeeee")
	v.set("taskBorderColor", "calculated")
	v.set("taskBkgColor", "calculated")
	v.set("taskTextLightColor", "white")
	v.set("taskTextColor", "calculated")
	v.set("taskTextDarkColor", "calculated")
	v.set("taskTextOutsideColor", "calculated")
	v.set("taskTextClickableColor", "#003163")
	for _, k := range []string{"activeTaskBorderColor", "activeTaskBkgColor", "gridColor", "doneTaskBkgColor",
		"doneTaskBorderColor", "critBkgColor", "critBorderColor", "todayLineColor", "vertLineColor"} {
		v.set(k, "calculated")
	}

	v.set("personBorder", v["primaryBorderColor"])
	v.set("personBkg", v["mainBkg"])

	architectureInit(v)

	v.set("noteFontWeight", "normal")
	v.set("fontWeight", "normal")

	v.or("rowOdd", first(lighten(v["mainBkg"], 75), "#ffffff"))
	v.or("rowEven", "#f4f4f4")

	v.set("labelColor", "black")
	v.set("errorBkgColor", "#552222")
	v.set("errorTextColor", "#552222")
	v.set("useGradient", true)
	v.set("gradientStart", v["primaryBorderColor"])
	v.set("gradientStop", v["secondaryBorderColor"])
	v.set("dropShadow", "drop-shadow( 1px 2px 2px rgba(185,185,185,1))")
}

func neutralUpdate(v vars) {
	text := v["text"]
	v.set("secondBkg", lighten(v["contrast"], 55))
	v.set("border2", v["contrast"])

	v.set("actorBorder", lighten(v["border1"], 23))
	v.set("actorBkg", v["mainBkg"])
	v.set("actorTextColor", text)
	v.set("actorLineColor", v["actorBorder"])
	v.or("rectBkgColor", v["tertiaryColor"])
	v.set("signalColor", text)
	v.set("signalTextColor", text)
	v.set("labelBoxBkgColor", v["actorBkg"])
	v.set("labelBoxBorderColor", v["actorBorder"])
	v.set("labelTextColor", text)
	v.set("loopTextColor", text)
	v.set("noteBorderColor", "#999")
	v.set("noteBkgColor", "#666")
	v.set("noteTextColor", "#fff")

	for i, c := range []string{"#555", "#F4F4F4", "#555", "#BBB", "#777", "#999", "#DDD", "#FFF", "#DDD", "#BBB", "#999", "#777"} {
		v.or(key("cScale", i), c)
	}
	cScaleInverses(v)
	v.or("scaleLabelColor", pick(v.dark(), "black", v["labelTextColor"]))
	v.or("cScaleLabel0", v["cScale1"])
	v.or("cScaleLabel2", v["cScale1"])
	for i := range v.limit() {
		v.or(key("cScaleLabel", i), v["scaleLabelColor"])
	}
	for i := range 5 {
		fi := float64(i)
		v.or(key("surface", i), adjust(v["mainBkg"], dl(-(5+fi*5))))
		v.or(key("surfacePeer", i), adjust(v["mainBkg"], dl(-(8+fi*5))))
	}

	v.set("nodeBkg", v["mainBkg"])
	v.set("nodeBorder", v["border1"])
	v.set("clusterBkg", v["secondBkg"])
	v.set("clusterBorder", v["border2"])
	v.set("defaultLinkColor", v["lineColor"])
	v.set("titleColor", text)
	v.or("flowContainerStroke", v["secondaryBorderColor"])

	v.set("sectionBkgColor", lighten(v["contrast"], 30))
	v.set("sectionBkgColor2", lighten(v["contrast"], 30))
	v.set("taskBorderColor", darken(v["contrast"], 10))
	v.set("taskBkgColor", v["contrast"])
	v.set("taskTextColor", v["taskTextLightColor"])
	v.set("taskTextDarkColor", text)
	v.set("taskTextOutsideColor", v["taskTextDarkColor"])
	v.set("activeTaskBorderColor", v["taskBorderColor"])
	v.set("activeTaskBkgColor", v["mainBkg"])
	v.set("gridColor", lighten(v["border1"], 30))
	v.set("doneTaskBkgColor", v["done"])
	v.set("doneTaskBorderColor", v["lineColor"])
	v.set("critBkgColor", v["critical"])
	v.set("critBorderColor", darken(v["critBkgColor"], 10))
	v.set("todayLineColor", v["critBkgColor"])
	v.set("vertLineColor", v["critBkgColor"])

	v.set("archEdgeColor", v["lineColor"])
	v.set("archEdgeArrowColor", v["lineColor"])

	v.or("transitionColor", "#000")
	v.or("transitionLabelColor", v["textColor"])
	v.or("stateLabelColor", first(v["stateBkg"], v["primaryTextColor"]))
	v.or("stateBkg", v["mainBkg"])
	v.or("labelBackgroundColor", v["stateBkg"])
	v.or("compositeBackground", first(v["background"], v["tertiaryColor"]))
	v.or("altBackground", "#f4f4f4")
	v.or("compositeTitleBackground", v["mainBkg"])
	v.or("stateBorder", "#000")
	v.set("innerEndBackground", v["primaryBorderColor"])
	v.set("specialStateColor", "#222")
	v.or("errorBkgColor", v["tertiaryColor"])
	v.or("errorTextColor", v["tertiaryTextColor"])

	v.set("classText", v["primaryTextColor"])
	fillTypes(v, v["primaryColor"], v["secondaryColor"], true)

	// pie0..11 from the scale, and pie12 is pie0 again.
	for i := range v.limit() {
		v.set(key("pie", i), v[key("cScale", i)])
	}
	v.set("pie12", v["pie0"])
	pieText(v)

	for i := range 8 {
		v.ifNull(key("venn", i+1), v[key("cScale", i)])
	}
	v.ifNull("vennTitleTextColor", v["titleColor"])
	v.ifNull("vennSetTextColor", v["textColor"])

	cynefin(v)
	quadrant(v, v["primaryColor"])
	xyChart(v, "#EEE,#6BB8E4,#8ACB88,#C7ACD6,#E8DCC2,#FFB2A8,#FFF380,#7E8D91,#FFD8B1,#FAF3E0", true)
	radar(v)
	wardley(v)
	requirement(v, v["primaryColor"], v["edgeLabelBackground"])

	// The git colors are the pie colors, set outright; only git0 is darkened.
	v.set("git0", first(darken(v["pie1"], 25), v["primaryColor"]))
	v.set("git1", first(v["pie2"], v["secondaryColor"]))
	v.set("git2", first(v["pie3"], v["tertiaryColor"]))
	for i, deg := range []float64{-30, -60, -90, 60, 120} {
		v.set(key("git", i+3), first(v[key("pie", i+4)], adjust(v["primaryColor"], dh(deg))))
	}
	gitInverses(v)
	v.or("branchLabelColor", v["labelTextColor"])
	for i := range 8 {
		v.set(key("gitBranchLabel", i), pick(i == 1 || i == 3, "white", v["branchLabelColor"]))
	}
	gitTagLabels(v)

	eventModeling(v)
	erAttributes(v)
}

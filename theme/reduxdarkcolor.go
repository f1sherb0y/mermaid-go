package theme

// theme-redux-dark-color.js: redux-color's palette on redux-dark's canvas.
func init() {
	defs["redux-dark-color"] = def{init: reduxDarkColorInit, update: reduxDarkColorUpdate}
}

func reduxDarkColorInit(v vars) {
	reduxDarkInit(v)
	v.set("borderColorArray", list(reduxBorderColors...))
	v.set("bkgColorArray", list())
	v.set("usecaseActorBorder", "#A78BFA")
	v.set("usecaseBorder", "#2DD4BF")
	v.set("usecaseBoundaryBorder", "#BDBCCC")
	v.set("usecaseIncludeLine", "#38BDF8")
	v.set("usecaseExtendLine", "#FB923C")
}

func reduxDarkColorUpdate(v vars) {
	reduxDarkTop(v)
	v.set("rootLabelColor", "#FFFFFF")

	// Gantt
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
	v.or("vertLineColor", v["primaryBorderColor"])
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
	v.or("stateEdgeLabelBackground", "#16141F")
	v.or("stateBkg", v["mainBkg"])
	v.or("labelBackgroundColor", v["stateBkg"])
	v.or("compositeBackground", "#16141F")
	v.or("altBackground", "#16141F")
	v.or("compositeTitleBackground", "#16141F")
	v.or("compositeBorder", v["nodeBorder"])
	v.set("innerEndBackground", v["nodeBorder"])
	v.or("errorBkgColor", v["tertiaryColor"])
	v.or("errorTextColor", v["tertiaryTextColor"])
	v.set("specialStateColor", v["lineColor"])

	for i, c := range reduxScale {
		v.or(key("cScale", i), c)
	}
	v.or("sectionBkgColor", v["cScale0"])
	v.or("sectionBkgColor2", v["cScale1"])
	v.or("altSectionBkgColor", v["background"])
	cScaleInverses(v)
	v.or("scaleLabelColor", v["labelTextColor"])
	// Unlike the other themes, the labels ignore overrides and scaleLabelColor.
	for i := range v.limit() {
		v.set(key("cScaleLabel", i), darken(v[key("cScale", i)], 75))
	}
	surfaces(v)
	v.or("classText", v["textColor"])

	for i, c := range []string{"#701a75", "#134e4a", "#7c2d12", "#581c87", "#14532d", "#4c1d95", "#7f1d1d", "#713f12"} {
		v.or(key("fillType", i), c)
	}
	reduxPie(v)
	v.or("pieSectionTextColor", "#28253D") // where pieText has textColor
	pieText(v)
	reduxVenn(v)
	reduxDarkBottom(v)
	v.set("fontWeight", 600)
}

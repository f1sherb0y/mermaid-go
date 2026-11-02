package theme

// theme-redux-color.js: the redux look on white with a categorical palette; mermaid.live's
// default for flowchart, class, state, ER, sequence and other diagrams.
func init() {
	defs["redux-color"] = def{init: reduxColorInit, update: reduxColorUpdate}
}

// The Tailwind 400 and 50 shades the color themes use for borders and fills.
var reduxBorderColors = []string{
	"#E879F9", "#2DD4BF", "#FB923C", "#22D3EE", "#4ADE80", "#A78BFA",
	"#F87171", "#FACC15", "#818CF8", "#A3E635 ", "#38BDF8", "#FB7185", // sic: "#A3E635 "
}

// The Tailwind 300 shades the color themes' cScale starts from.
var reduxScale = []string{
	"#f4a8ff", "#46ecd5", "#ffb86a", "#dab2ff", "#7bf1a8", "#c4b4ff",
	"#ffa2a2", "#ffdf20", "#a3b3ff", "#bbf451", "#74d4ff", "#ffa1ad",
}

func reduxColorInit(v vars) {
	v.set("background", "#ffffff")
	v.set("primaryColor", "#cccccc")
	v.set("mainBkg", "#ffffff")
	v.set("noteBkgColor", "#fff5ad")
	v.set("noteTextColor", "#28253D")
	v.set("THEME_COLOR_LIMIT", 12)
	v.set("radius", 12)
	v.set("strokeWidth", 2)
	v.set("primaryBorderColor", mkBorder("#28253D", v.dark()))
	v.set("fontFamily", `"Recursive Variable", arial, sans-serif`)
	v.set("fontSize", "14px")
	v.set("nodeBorder", "#28253D")
	v.set("stateBorder", "#28253D")
	v.set("useGradient", false)
	v.set("gradientStart", "#0042eb")
	v.set("gradientStop", "#eb0042")
	v.set("dropShadow", "url(#drop-shadow)")
	v.set("nodeShadow", true)
	v.set("tertiaryColor", "#ffffff")
	v.set("clusterBkg", "#F9F9FB")
	v.set("clusterBorder", "#BDBCCC")
	architectureInit(v)
	v.set("actorBorder", "#28253D")
	v.set("noteBorderColor", "#FACC15")
	v.set("noteFontWeight", 600)
	v.set("borderColorArray", list(reduxBorderColors...))
	v.set("bkgColorArray", list("#FDF4FF", "#F0FDFA", "#FFF7ED", "#ECFEFF", "#F0FDF4", "#F5F3FF",
		"#FEF2F2", "#FEFCE8", "#EEF2FF", "#F7FEE7", "#F0F9FF", "#FFF1F2"))
	v.set("usecaseActorBorder", "#A78BFA")
	v.set("usecaseActorBkg", "#F5F3FF")
	v.set("usecaseBorder", "#2DD4BF")
	v.set("usecaseBkg", "#F0FDFA")
	v.set("usecaseBoundaryBorder", "#BDBCCC")
	v.set("usecaseBoundaryBkg", "#FAFAFC")
	v.set("usecaseIncludeLine", "#38BDF8")
	v.set("usecaseExtendLine", "#FB923C")
	v.set("filterColor", "#000000")
}

func reduxColorUpdate(v vars) {
	mainColors(v, "#28253D", "#fff5ad")
	flowchartColors(v, v["primaryBorderColor"])
	sequenceColors(v)
	v.or("rectBkgColor", shade(v["background"], 4))

	// Gantt, from colors of its own.
	const primary, secondary = "#ECECFE", "#E9E9F1"
	tertiary := adjust(primary, dhl(180, 5))
	v.or("excludeBkgColor", "#eeeeee")
	v.or("taskBorderColor", v["primaryBorderColor"])
	v.or("taskBkgColor", primary)
	v.or("activeTaskBorderColor", primary)
	v.or("activeTaskBkgColor", lighten(primary, 23))
	v.or("gridColor", "lightgrey")
	v.or("doneTaskBkgColor", "lightgrey")
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
	v.or("stateEdgeLabelBackground", "#FFFFFF")
	v.or("stateBkg", v["mainBkg"])
	v.or("labelBackgroundColor", v["stateBkg"])
	v.or("compositeBackground", first(v["background"], v["tertiaryColor"]))
	v.or("altBackground", "#F9F9FB")
	v.or("compositeTitleBackground", "#F9F9FB")
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
	v.or("altSectionBkgColor", "white")
	cScaleSets(v)
	surfaces(v)
	v.or("classText", v["textColor"])

	for i := range 8 {
		v.or(key("fillType", i), v.elem("bkgColorArray", i))
	}
	reduxPie(v)
	pieText(v)
	reduxVenn(v)
	quadrant(v, primary)
	xyChart(v, "#FFF4DD,#FFD8B1,#FFA07A,#ECEFF1,#D6DBDF,#C3E0A8,#FFB6A4,#FFD74D,#738FA7,#FFFFF0", false)
	requirement(v, primary, labelOnSecondary(v))
	v.set("requirementEdgeLabelBackground", "#FFFFFF")

	// Git, from the gantt colors.
	gitWheel(v, primary, secondary, tertiary)
	gitShade(v)
	gitInverses(v)
	gitLabels(v)
	v.ifNull("commitLineColor", "#BDBCCC")
	v.set("fontWeight", 600)

	v.set("erEdgeLabelBackground", "#FFFFFF")
	erAttributes(v)
}

// reduxPie takes the pie slices from the color scale.
func reduxPie(v vars) {
	for i := range v.limit() {
		v.or(key("pie", i+1), v[key("cScale", i)])
	}
}

// reduxVenn takes the venn sets from borderColorArray.
func reduxVenn(v vars) {
	n := v.length("borderColorArray")
	for i := range 8 {
		var c any
		if n > 0 {
			c = v.elem("borderColorArray", i%n)
		}
		v.ifNull(key("venn", i+1), c)
	}
	v.ifNull("vennTitleTextColor", v["titleColor"])
	v.ifNull("vennSetTextColor", v["textColor"])
}

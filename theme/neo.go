package theme

// theme-neo.js, and the updateColors that theme-neo-dark.js and theme-redux.js are copies of.
func init() {
	defs["neo"] = def{init: neoInit, update: func(v vars) { neoUpdate(v, neoVariantNeo) }}
}

func neoInit(v vars) {
	v.set("background", "#ffffff")
	v.set("primaryColor", "#cccccc")
	v.set("mainBkg", "#ffffff")
	v.set("noteBkgColor", "#fff5ad")
	v.set("noteTextColor", "#333")
	v.set("THEME_COLOR_LIMIT", 12)
	v.set("radius", 3)
	v.set("strokeWidth", 2)
	v.set("primaryBorderColor", mkBorder(v["primaryColor"], v.dark()))
	v.set("fontFamily", "arial, sans-serif")
	v.set("fontSize", "14px")
	v.set("nodeBorder", "#000000")
	v.set("stateBorder", "#000000")
	v.set("useGradient", true)
	v.set("gradientStart", "#0042eb")
	v.set("gradientStop", "#eb0042")
	v.set("dropShadow", "drop-shadow( 0px 1px 2px rgba(0, 0, 0, 0.25));")
	v.set("tertiaryColor", "#ffffff")
	architectureInit(v)
	v.set("noteFontWeight", "normal")
	v.set("fontWeight", "normal")
}

type neoVariant int

const (
	neoVariantNeo neoVariant = iota
	neoVariantDark
	neoVariantRedux
)

func neoUpdate(v vars, variant neoVariant) {
	ink, noteBkg := "#333", "#fff5ad"
	if variant == neoVariantRedux {
		ink, noteBkg = "#28253D", "#FEF9C3"
	}

	mainColors(v, ink, noteBkg)

	if variant == neoVariantDark {
		flowchartColors(v, v["border1"])
		// The constructor's 'calculated' placeholder, resolved.
		if v["secondBkg"] == "calculated" {
			v.set("secondBkg", lighten(v["mainBkg"], 16))
		}
	} else {
		flowchartColors(v, v["primaryBorderColor"])
	}

	// Sequence
	if variant == neoVariantRedux {
		v.set("noteFontWeight", 600)
	}
	sequenceColors(v)
	if variant == neoVariantDark {
		v.or("rectBkgColor", v["tertiaryColor"])
	} else {
		v.or("rectBkgColor", shade(v["background"], 4))
	}

	// Gantt. neo and neoVariantRedux use fixed colors here and below where neo-dark uses its own.
	p, s, t := v["primaryColor"], v["secondaryColor"], v["tertiaryColor"]
	if variant != neoVariantDark {
		p, s = "#ECECFE", "#E9E9F1"
		t = adjust(p, dhl(180, 5))
	}
	v.or("sectionBkgColor", t)
	v.or("altSectionBkgColor", "white")
	v.or("sectionBkgColor", s)
	v.or("sectionBkgColor2", p)
	v.or("excludeBkgColor", "#eeeeee")
	v.or("taskBorderColor", v["primaryBorderColor"])
	v.or("taskBkgColor", p)
	v.or("activeTaskBorderColor", p)
	v.or("activeTaskBkgColor", lighten(p, 23))
	v.or("gridColor", "lightgrey")
	v.or("doneTaskBkgColor", pick(variant == neoVariantDark, v["secondBkg"], "lightgrey"))
	v.or("doneTaskBorderColor", "grey")
	v.or("critBorderColor", "#ff8888")
	v.or("critBkgColor", "red")
	v.or("todayLineColor", "red")
	v.or("taskTextColor", v["textColor"])
	v.or("taskTextOutsideColor", v["textColor"])
	v.or("vertLineColor", v["primaryBorderColor"])
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
	if variant == neoVariantRedux {
		v.set("compositeTitleBackground", "#F9F9FB")
		v.set("altBackground", "#F9F9FB")
		v.set("stateEdgeLabelBackground", "#FFFFFF")
		v.set("fontWeight", 600)
	}
	v.or("stateBkg", v["mainBkg"])
	v.or("labelBackgroundColor", v["stateBkg"])
	v.or("compositeBackground", first(v["background"], v["tertiaryColor"]))
	v.or("altBackground", "#f0f0f0")
	v.or("compositeTitleBackground", v["mainBkg"])
	v.or("compositeBorder", v["nodeBorder"])
	v.set("innerEndBackground", v["nodeBorder"])
	v.or("errorBkgColor", v["tertiaryColor"])
	v.or("errorTextColor", v["tertiaryTextColor"])
	v.set("specialStateColor", v["lineColor"])

	// Color scale. neoVariantRedux paints every one mainBkg, overriding the user's.
	if variant == neoVariantRedux {
		for i := range v.limit() {
			v.set(key("cScale", i), v["mainBkg"])
		}
	} else {
		cScaleWheel(v, p, s, t)
	}
	cScaleDarken(v)
	cScaleSets(v)
	surfaces(v)

	v.or("classText", v["textColor"])

	fillTypes(v, p, s, false)

	pieWheel(v, p, s, t)
	pieText(v)

	v.ifNull("vennTitleTextColor", v["titleColor"])
	v.ifNull("vennSetTextColor", v["textColor"])

	quadrant(v, p)
	xyChart(v, "#FFF4DD,#FFD8B1,#FFA07A,#ECEFF1,#D6DBDF,#C3E0A8,#FFB6A4,#FFD74D,#738FA7,#FFFFF0", false)

	requirement(v, p, labelOnSecondary(v))
	if variant == neoVariantRedux {
		v.set("requirementEdgeLabelBackground", "#FFFFFF")
	}

	// Git
	if variant == neoVariantDark {
		for i, c := range []string{"#0b0000", "#4d1037", "#3f5258", "#4f2f1b", "#6e0a0a", "#3b0048", "#995a01", "#154706"} {
			v.or(key("git", i), c)
		}
		// Always lightened, whatever darkMode says.
		v.set("gitDarkMode", true)
		for i := range 8 {
			v.set(key("git", i), lighten(v[key("git", i)], 25))
		}
	} else {
		gitWheel(v, p, s, t)
		gitShade(v)
	}
	gitInverses(v)
	gitLabels(v)
	if variant == neoVariantRedux {
		v.ifNull("commitLineColor", "#BDBCCC")
		v.set("erEdgeLabelBackground", "#FFFFFF")
	}
	erAttributes(v)
}

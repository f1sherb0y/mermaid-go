package theme

// theme-redux.js: neo's updateColors with a darker ink, a grey palette and Recursive.
func init() {
	defs["redux"] = def{init: reduxInit, update: func(v vars) { neoUpdate(v, neoVariantRedux) }}
}

func reduxInit(v vars) {
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
	v.set("noteBorderColor", "#FACC15")
	architectureInit(v)
	v.set("actorBorder", "#28253D")
	v.set("filterColor", "#000000")
}

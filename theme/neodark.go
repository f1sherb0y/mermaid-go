package theme

// theme-neo-dark.js: theme-dark's base colors under neo's updateColors.
func init() {
	defs["neo-dark"] = def{init: neoDarkInit, update: func(v vars) { neoUpdate(v, neoVariantDark) }}
}

func neoDarkInit(v vars) {
	dark := v.dark()
	v.set("background", "#333")
	v.set("primaryColor", "#1f2020")
	v.set("secondaryColor", lighten(v["primaryColor"], 16))
	v.set("tertiaryColor", adjust(v["primaryColor"], dh(-160)))
	v.set("primaryBorderColor", invert(v["background"]))
	v.set("secondaryBorderColor", mkBorder(v["secondaryColor"], dark))
	v.set("tertiaryBorderColor", mkBorder(v["tertiaryColor"], dark))
	v.set("primaryTextColor", invert(v["primaryColor"]))
	v.set("secondaryTextColor", invert(v["secondaryColor"]))
	v.set("tertiaryTextColor", invert(v["tertiaryColor"]))
	v.set("mainBkg", "#2a2020")
	v.set("secondBkg", "calculated")
	v.set("mainContrastColor", "lightgrey")
	v.set("darkTextColor", lighten(invert("#323D47"), 10))
	v.set("border1", "#ccc")
	v.set("border2", rgba(255, 255, 255, 0.25))
	v.set("arrowheadColor", invert(v["background"]))
	v.set("fontFamily", "arial, sans-serif")
	v.set("fontSize", "14px")
	v.set("labelBackground", "#181818")
	v.set("textColor", "#ccc")
	v.set("THEME_COLOR_LIMIT", 12)
	v.set("radius", 3)
	v.set("strokeWidth", 1)
	v.set("noteBkgColor", "#fff5ad")
	v.set("noteTextColor", "#333")
	v.set("useGradient", true)
	v.set("gradientStart", "#0042eb")
	v.set("gradientStop", "#eb0042")
	v.set("dropShadow", "drop-shadow( 1px 2px 2px rgba(185,185,185,0.2))")
	architectureInit(v)
	v.set("noteFontWeight", "normal")
	v.set("fontWeight", "normal")
}

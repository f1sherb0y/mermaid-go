package render

import (
	"math"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// Edge markers (markers.js, edgeMarker.ts), drawn in place: each marker is
// its content transformed to the path's end as SVG's orient="auto" marker
// would be.

type markerDef struct {
	vb         *scene.Rect
	refX, refY float64
	w, h       float64
	// strokeUnits scales the marker by the path's stroke width
	// (markerUnits strokeWidth, the default).
	strokeUnits bool
	parts       []markerPart
	// cross markers get the .marker.cross stroke.
	cross bool
}

type markerPart struct {
	segs []scene.Seg
	// width is the part's stroke width (its inline style or attribute); -1
	// for the inherited default.
	width float64
	// fill: "" takes the stylesheet's marker fill; "none" none; else the
	// color the part's own style gives it (inline styles beat the sheet).
	fill string
	// stroke likewise.
	stroke string
	// filled: the part takes the edge's color as fill when colored
	// (arrowTypesMap's fill).
	filled bool
}

func mp(d string, width float64) markerPart { return markerPart{segs: parsePath(d), width: width} }

func circlePart(cx, cy, r, width float64) markerPart {
	return markerPart{segs: scene.Ellipse(cx, cy, r, r), width: width}
}

func vb(x, y, w, h float64) *scene.Rect { return &scene.Rect{X: x, Y: y, W: w, H: h} }

// markerDefs are the markers by mermaid's id suffix ("pointEnd",
// "pointEnd-margin", ...), for the classic and neo looks.
var markerDefs = map[string]*markerDef{
	// point
	"pointEnd":          {vb: vb(0, 0, 10, 10), refX: 5, refY: 5, w: 8, h: 8, parts: []markerPart{mp("M 0 0 L 10 5 L 0 10 z", 1)}},
	"pointStart":        {vb: vb(0, 0, 10, 10), refX: 4.5, refY: 5, w: 8, h: 8, parts: []markerPart{mp("M 0 5 L 10 10 L 10 0 z", 1)}},
	"pointEnd-margin":   {vb: vb(0, 0, 11.5, 14), refX: 11.5, refY: 7, w: 10.5, h: 14, parts: []markerPart{mp("M 0 0 L 11.5 7 L 0 14 z", 0)}},
	"pointStart-margin": {vb: vb(0, 0, 11.5, 14), refX: 1, refY: 7, w: 11.5, h: 14, parts: []markerPart{mp("M 0,7 L 11.5,14 L 11.5,0 Z", 0)}},
	// circle
	"circleEnd":          {vb: vb(0, 0, 10, 10), refX: 11, refY: 5, w: 11, h: 11, parts: []markerPart{circlePart(5, 5, 5, 1)}},
	"circleStart":        {vb: vb(0, 0, 10, 10), refX: -1, refY: 5, w: 11, h: 11, parts: []markerPart{circlePart(5, 5, 5, 1)}},
	"circleEnd-margin":   {vb: vb(0, 0, 10, 10), refX: 12.25, refY: 5, w: 14, h: 14, parts: []markerPart{circlePart(5, 5, 5, 0)}},
	"circleStart-margin": {vb: vb(0, 0, 10, 10), refX: -2, refY: 5, w: 14, h: 14, parts: []markerPart{circlePart(5, 5, 5, 0)}},
	// cross
	"crossEnd":          {vb: vb(0, 0, 11, 11), refX: 12, refY: 5.2, w: 11, h: 11, cross: true, parts: []markerPart{mp("M 1,1 l 9,9 M 10,1 l -9,9", 2)}},
	"crossStart":        {vb: vb(0, 0, 11, 11), refX: -1, refY: 5.2, w: 11, h: 11, cross: true, parts: []markerPart{mp("M 1,1 l 9,9 M 10,1 l -9,9", 2)}},
	"crossEnd-margin":   {vb: vb(0, 0, 15, 15), refX: 17.7, refY: 7.5, w: 12, h: 12, cross: true, parts: []markerPart{mp("M 1,1 L 14,14 M 1,14 L 14,1", 2.5)}},
	"crossStart-margin": {vb: vb(0, 0, 15, 15), refX: -3.5, refY: 7.5, w: 12, h: 12, cross: true, parts: []markerPart{mp("M 1,1 L 14,14 M 1,14 L 14,1", 2.5)}},
	// class diagram
	"extensionStart":          {refX: 18, refY: 7, w: 20, h: 28, parts: []markerPart{mp("M 1,7 L18,13 V 1 Z", -1)}},
	"extensionEnd":            {refX: 1, refY: 7, w: 20, h: 28, parts: []markerPart{mp("M 1,1 V 13 L18,7 Z", -1)}},
	"extensionStart-margin":   {vb: vb(0, 0, 20, 14), refX: 18, refY: 7, w: 20, h: 28, parts: []markerPart{mp("M 10,7 L 18,13 L 18,1 Z", 2)}},
	"extensionEnd-margin":     {vb: vb(0, 0, 20, 14), refX: 9, refY: 7, w: 20, h: 28, parts: []markerPart{mp("M 10,1 L 10,13 L 18,7 Z", 2)}},
	"compositionStart":        {refX: 18, refY: 7, w: 190, h: 240, parts: []markerPart{mp("M 18,7 L9,13 L1,7 L9,1 Z", -1)}},
	"compositionEnd":          {refX: 1, refY: 7, w: 20, h: 28, parts: []markerPart{mp("M 18,7 L9,13 L1,7 L9,1 Z", -1)}},
	"compositionStart-margin": {refX: 15, refY: 7, w: 190, h: 240, parts: []markerPart{mp("M 18,7 L9,13 L1,7 L9,1 Z", 0)}},
	"compositionEnd-margin":   {refX: 3.5, refY: 7, w: 20, h: 28, parts: []markerPart{mp("M 18,7 L9,13 L1,7 L9,1 Z", 0)}},
	"aggregationStart":        {refX: 18, refY: 7, w: 190, h: 240, parts: []markerPart{mp("M 18,7 L9,13 L1,7 L9,1 Z", -1)}},
	"aggregationEnd":          {refX: 1, refY: 7, w: 20, h: 28, parts: []markerPart{mp("M 18,7 L9,13 L1,7 L9,1 Z", -1)}},
	"aggregationStart-margin": {refX: 15, refY: 7, w: 190, h: 240, parts: []markerPart{mp("M 18,7 L9,13 L1,7 L9,1 Z", 2)}},
	"aggregationEnd-margin":   {refX: 1, refY: 7, w: 20, h: 28, parts: []markerPart{mp("M 18,7 L9,13 L1,7 L9,1 Z", 2)}},
	"dependencyStart":         {refX: 6, refY: 7, w: 190, h: 240, parts: []markerPart{mp("M 5,7 L9,13 L1,7 L9,1 Z", -1)}},
	"dependencyEnd":           {refX: 13, refY: 7, w: 20, h: 28, parts: []markerPart{mp("M 18,7 L9,13 L14,7 L9,1 Z", -1)}},
	"dependencyStart-margin":  {refX: 4, refY: 7, w: 190, h: 240, parts: []markerPart{mp("M 5,7 L9,13 L1,7 L9,1 Z", 0)}},
	"dependencyEnd-margin":    {refX: 16, refY: 7, w: 20, h: 28, parts: []markerPart{mp("M 18,7 L9,13 L14,7 L9,1 Z", 0)}},
	"lollipopStart":           {refX: 13, refY: 7, w: 190, h: 240, parts: []markerPart{{segs: scene.Ellipse(7, 7, 6, 6), width: -1, fill: "transparent-attr"}}},
	"lollipopEnd":             {refX: 1, refY: 7, w: 190, h: 240, parts: []markerPart{{segs: scene.Ellipse(7, 7, 6, 6), width: -1, fill: "transparent-attr"}}},
	"lollipopStart-margin":    {refX: 13, refY: 7, w: 190, h: 240, parts: []markerPart{{segs: scene.Ellipse(7, 7, 6, 6), width: 2, fill: "transparent-attr"}}},
	"lollipopEnd-margin":      {refX: 1, refY: 7, w: 190, h: 240, parts: []markerPart{{segs: scene.Ellipse(7, 7, 6, 6), width: 2, fill: "transparent-attr"}}},
	"hierarchyEnd":            {vb: vb(0, 0, 12, 10), refX: 10, refY: 5, w: 12, h: 10, parts: []markerPart{{segs: parsePath("M 0 0 L 6 5 L 0 10 M 4 0 L 10 5 L 4 10"), width: 1, fill: "none"}}},
	"hierarchyStart":          {vb: vb(0, 0, 12, 10), refX: 2, refY: 5, w: 12, h: 10, parts: []markerPart{{segs: parsePath("M 12 0 L 6 5 L 12 10 M 8 0 L 2 5 L 8 10"), width: 1, fill: "none"}}},
	// state diagram (barbNeo; the classic barb has no margin variant)
	"barbEnd":        {refX: 19, refY: 7, w: 20, h: 14, strokeUnits: true, parts: []markerPart{mp("M 19,7 L11,14 L13,7 L11,0 Z", -1)}},
	"barbEnd-margin": {refX: 17, refY: 7, w: 20, h: 14, parts: []markerPart{{segs: parsePath("M 19,7 L11,14 L13,7 L11,0 Z"), width: -1, fill: "transitionColor-attr"}}},
}

// ER cardinalities and requirement relations (the neo variants; used for
// both looks).
func init() {
	none := func(p markerPart) markerPart { p.fill = "none"; return p }
	for k, d := range map[string]*markerDef{
		"onlyOneStart":    {refX: 0, refY: 9, w: 18, h: 18, parts: []markerPart{mp("M9,0 L9,18 M15,0 L15,18", 2)}},
		"onlyOneEnd":      {refX: 18, refY: 9, w: 18, h: 18, parts: []markerPart{mp("M3,0 L3,18 M9,0 L9,18", 2)}},
		"zeroOrOneStart":  {refX: 0, refY: 9, w: 30, h: 18, parts: []markerPart{circlePart(21, 9, 6, 2), mp("M9,0 L9,18", 2)}},
		"zeroOrOneEnd":    {refX: 30, refY: 9, w: 30, h: 18, parts: []markerPart{circlePart(9, 9, 6, 2), mp("M21,0 L21,18", 2)}},
		"oneOrMoreStart":  {refX: 18, refY: 18, w: 45, h: 36, parts: []markerPart{mp("M0,18 Q 18,0 36,18 Q 18,36 0,18 M42,9 L42,27", 2)}},
		"oneOrMoreEnd":    {refX: 27, refY: 18, w: 45, h: 36, parts: []markerPart{mp("M3,9 L3,27 M9,18 Q27,0 45,18 Q27,36 9,18", 2)}},
		"zeroOrMoreStart": {refX: 18, refY: 18, w: 57, h: 36, parts: []markerPart{circlePart(45.5, 18, 6, 2), mp("M0,18 Q18,0 36,18 Q18,36 0,18", 2)}},
		"zeroOrMoreEnd":   {refX: 39, refY: 18, w: 57, h: 36, parts: []markerPart{circlePart(11, 18, 6, 2), mp("M21,18 Q39,0 57,18 Q39,36 21,18", 2)}},
		"requirement_arrowEnd": {vb: vb(0, 0, 25, 20), refX: 20, refY: 10, w: 20, h: 20,
			parts: []markerPart{none(mp("M0,0 L20,10 M20,10 L0,20", 2))}},
		"requirement_containsStart": {refX: 0, refY: 10, w: 20, h: 20,
			parts: []markerPart{none(circlePart(10, 10, 9, 2)), none(mp("M1,10 L19,10 M10,1 L10,19", 2))}},
	} {
		markerDefs[k] = d
	}
}

// classicBarb is the classic look's barbEnd.
var classicBarb = &markerDef{refX: 19, refY: 7, w: 20, h: 14, parts: []markerPart{mp("M 19,7 L9,13 L14,7 L9,1 Z", -1)}}

// arrowTypes maps edge arrow types to marker names and whether the colored
// variant fills them (edgeMarker.ts).
var arrowTypes = map[string]struct {
	name   string
	filled bool
	margin bool
}{
	"arrow_cross":          {"cross", false, true},
	"arrow_point":          {"point", true, true},
	"arrow_barb":           {"barb", true, true},
	"arrow_barb_neo":       {"barb", true, true},
	"arrow_circle":         {"circle", false, true},
	"arrow_hierarchy":      {"hierarchy", false, false},
	"aggregation":          {"aggregation", false, true},
	"extension":            {"extension", false, true},
	"composition":          {"composition", true, true},
	"dependency":           {"dependency", true, true},
	"lollipop":             {"lollipop", false, true},
	"only_one":             {"onlyOne", false, false},
	"zero_or_one":          {"zeroOrOne", false, false},
	"one_or_more":          {"oneOrMore", false, false},
	"zero_or_more":         {"zeroOrMore", false, false},
	"requirement_arrow":    {"requirement_arrow", false, false},
	"requirement_contains": {"requirement_contains", false, false},
}

// marker finds the marker an edge end gets: nil when none (or when the
// diagram defines none of that name).
func (r *renderer) marker(arrowType string, start, margin bool) (*markerDef, bool, string) {
	at, ok := arrowTypes[arrowType]
	if !ok || arrowType == "none" {
		return nil, false, ""
	}
	id := at.name
	if start {
		id += "Start"
	} else {
		id += "End"
	}
	if margin && at.margin {
		id += "-margin"
	}
	if d := r.sheet.markers[id]; d != nil {
		return d, at.filled, at.name
	}
	return nil, false, ""
}

// drawMarker places a marker at a path's end p, pointing along angle a
// (radians), for a path of stroke width sw and color c (the edge's own
// stroke color, None when it has none).
func (r *renderer) drawMarker(m *markerDef, name string, p P, a, sw float64, c scene.Color, filled bool) scene.Item {
	s := 1.0
	if m.vb != nil {
		s = math.Min(m.w/m.vb.W, m.h/m.vb.H)
	}
	if m.strokeUnits {
		s *= sw
	}
	t := scene.Translate(-m.refX, -m.refY).Then(scene.Scale(s, s)).Then(scene.Rotate(a * 180 / math.Pi)).Then(scene.Translate(p.X, p.Y))
	g := &scene.Group{Transform: t}
	fill, stroke := r.sheet.markerColors(name)
	if m.cross {
		stroke = r.sheet.markerCrossStroke
	}
	for _, part := range m.parts {
		pp := paint{Fill: fill, Stroke: stroke, Width: 1}
		if part.width >= 0 {
			pp.Width = part.width
		}
		if !c.IsNone() {
			pp.Stroke = c
			if filled {
				pp.Fill = c
			}
		}
		switch part.fill {
		case "none":
			pp.Fill = scene.None
		case "transparent-attr", "":
		case "transitionColor-attr":
		default:
			pp.Fill = r.sheet.resolve(part.fill)
		}
		if part.stroke != "" {
			pp.Stroke = r.sheet.resolve(part.stroke)
		}
		g.Items = append(g.Items, pp.path(part.segs))
	}
	return g
}

// endAngle is the direction a path arrives at its last point (or leaves
// its first, for start): the marker's orientation.
func endAngle(pts []P, start bool) float64 {
	n := len(pts)
	if n < 2 {
		return 0
	}
	if start {
		for i := 1; i < n; i++ {
			if pts[i] != pts[0] {
				return math.Atan2(pts[i].Y-pts[0].Y, pts[i].X-pts[0].X)
			}
		}
		return 0
	}
	for i := n - 2; i >= 0; i-- {
		if pts[i] != pts[n-1] {
			return math.Atan2(pts[n-1].Y-pts[i].Y, pts[n-1].X-pts[i].X)
		}
	}
	return 0
}

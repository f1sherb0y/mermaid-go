// Package scene is a drawing: filled and stroked paths, text, and groups
// with a transform, in CSS pixels. Styles are explicit; there are no
// classes. It writes itself as SVG, and a native painter draws it directly.
package scene

import "math"

// Scene is a whole diagram.
type Scene struct {
	// ViewBox is the area to show.
	ViewBox Rect
	// Background, if not None, fills the view box.
	Background Color
	Items      []Item
}

// Rect is an axis-aligned rectangle.
type Rect struct{ X, Y, W, H float64 }

// Item is a Path, a Text or a Group.
type Item interface{ item() }

// Group is items drawn with a transform and an opacity.
type Group struct {
	Transform Matrix
	// Opacity is 0..1; 0 means 1 (unset).
	Opacity float64
	// Blend is a CSS mix-blend-mode ("multiply"); "" is normal.
	Blend string
	Items []Item
}

// Path is a shape.
type Path struct {
	Segs        []Seg
	Fill        Color
	Stroke      Color
	StrokeWidth float64
	Dash        []float64
	Cap         Cap
	Join        Join
	// Shadow, if not nil, is drawn under the shape.
	Shadow *Shadow
	// StrokeGradient, if not nil, strokes instead of Stroke's color.
	StrokeGradient *Gradient
	// EvenOdd fills with the even-odd rule instead of nonzero.
	EvenOdd bool
}

// Gradient is a linear gradient in the drawing's coordinates.
type Gradient struct {
	X1, Y1, X2, Y2 float64
	Stops          []Stop
}

// Stop is a gradient's color at an offset 0..1.
type Stop struct {
	Offset float64
	Color  Color
}

// Text is one line of text: runs set one after another from a point.
type Text struct {
	X, Y     float64
	Anchor   Anchor
	Baseline Baseline
	// Family is a CSS font-family list; Size in px.
	Family string
	Size   float64
	Color  Color
	Runs   []Span
	// Rotate, in degrees, turns the text about (X, Y).
	Rotate float64
}

// Span is a run of text with its own weight and slant.
type Span struct {
	Text      string
	Weight    int
	Italic    bool
	Underline bool
	// Color, if not None, overrides the text's.
	Color Color
}

// Shadow is a drop shadow (CSS drop-shadow / SVG feDropShadow).
type Shadow struct {
	DX, DY, Blur float64
	Color        Color
}

func (*Group) item() {}
func (*Path) item()  {}
func (*Text) item()  {}

// Anchor is where a text's point is along it.
type Anchor uint8

const (
	Start Anchor = iota
	Middle
	End
)

// Baseline is where a text's point is across it.
type Baseline uint8

const (
	Alphabetic Baseline = iota
	// Central is the middle of the em box (SVG dominant-baseline central).
	Central
	// MiddleBaseline is half the x-height above the alphabetic baseline (SVG
	// dominant-baseline middle).
	MiddleBaseline
	Hanging
)

// Cap is a stroke's line cap.
type Cap uint8

const (
	ButtCap Cap = iota
	RoundCap
	SquareCap
)

// Join is a stroke's line join.
type Join uint8

const (
	MiterJoin Join = iota
	RoundJoin
	BevelJoin
)

// Color is 8-bit RGB and an alpha 0..1. The zero Color is None (nothing
// drawn); an opaque color has A 1.
type Color struct {
	R, G, B uint8
	A       float64
}

// None is no color.
var None = Color{}

// RGB is an opaque color.
func RGB(r, g, b uint8) Color { return Color{r, g, b, 1} }

// IsNone reports a color that draws nothing.
func (c Color) IsNone() bool { return c.A <= 0 }

// Matrix is an affine transform [a c e; b d f], as SVG's matrix(a b c d e f).
type Matrix struct{ A, B, C, D, E, F float64 }

// Identity is no transform.
var Identity = Matrix{A: 1, D: 1}

// Translate is a translation.
func Translate(x, y float64) Matrix { return Matrix{A: 1, D: 1, E: x, F: y} }

// Scale is a scaling.
func Scale(sx, sy float64) Matrix { return Matrix{A: sx, D: sy} }

// Rotate is a rotation by degrees, clockwise on screen.
func Rotate(deg float64) Matrix {
	s, c := math.Sincos(deg * math.Pi / 180)
	return Matrix{A: c, B: s, C: -s, D: c}
}

// Then is m followed by n.
func (m Matrix) Then(n Matrix) Matrix {
	return Matrix{
		A: n.A*m.A + n.C*m.B, B: n.B*m.A + n.D*m.B,
		C: n.A*m.C + n.C*m.D, D: n.B*m.C + n.D*m.D,
		E: n.A*m.E + n.C*m.F + n.E, F: n.B*m.E + n.D*m.F + n.F,
	}
}

// Apply maps a point.
func (m Matrix) Apply(x, y float64) (float64, float64) {
	return m.A*x + m.C*y + m.E, m.B*x + m.D*y + m.F
}

// IsIdentity reports no transform.
func (m Matrix) IsIdentity() bool { return m == Identity || m == Matrix{} }

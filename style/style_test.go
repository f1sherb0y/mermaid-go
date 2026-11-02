package style

import (
	"testing"

	"github.com/f1sherb0y/mermaid-go/color"
	"github.com/f1sherb0y/mermaid-go/scene"
)

func TestColor(t *testing.T) {
	for in, want := range map[string]scene.Color{
		"#ECECFF":                       {236, 236, 255, 1},
		"black":                         {0, 0, 0, 1},
		"hsl(80, 100%, 56.2745098039%)": {181, 255, 32, 1},
		"rgba(0, 0, 0, 0.5)":            {0, 0, 0, 0.5},
		"#f9f":                          {255, 153, 255, 1},
		"none":                          {},
	} {
		if got := Color(in); got != want {
			a, err := color.ChannelOf(in, color.A)
			t.Errorf("%s: %v, want %v (alpha %v %v)", in, got, want, a, err)
		}
	}
}

func TestDropShadow(t *testing.T) {
	for in, want := range map[string]*scene.Shadow{
		"drop-shadow(1px 2px 2px rgba(185, 185, 185, 1))": {DX: 1, DY: 2, Blur: 2, Color: scene.RGB(185, 185, 185)},
		"drop-shadow( 0px 1px 2px rgba(0, 0, 0, 0.25));":  {DX: 0, DY: 1, Blur: 2, Color: scene.Color{A: 0.25}},
		"drop-shadow(3px 5px)":                            {DX: 3, DY: 5, Color: scene.RGB(0, 0, 0)},
	} {
		if got := DropShadow(in); got == nil || *got != *want {
			t.Errorf("DropShadow(%q) = %+v, want %+v", in, got, want)
		}
	}
	for _, in := range []string{"url(#drop-shadow)", "none", ""} {
		if got := DropShadow(in); got != nil {
			t.Errorf("DropShadow(%q) = %+v, want nil", in, got)
		}
	}
}

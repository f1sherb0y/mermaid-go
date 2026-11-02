// Package icons is mermaid's icon registry (rendering-util/icons.ts): icon
// sets in Iconify's JSON format registered under a prefix, looked up as
// "prefix:name", and drawn as scene items. Nothing but mermaid's own
// built-ins is registered by default (mermaid.live registers no packs), so
// "logos:aws-s3" is the unknown icon until a host registers "logos".
package icons

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/f1sherb0y/mermaid-go/scene"
)

// Icon is an icon's SVG body (the markup inside <svg>) and its view box.
type Icon struct {
	Body                     string
	Left, Top, Width, Height float64
	// Transform, if set, maps the body into the view box (Iconify's
	// rotate, hFlip, vFlip).
	Transform scene.Matrix
}

// Set is an Iconify icon set (IconifyJSON).
type Set struct {
	Prefix  string           `json:"prefix"`
	Icons   map[string]Entry `json:"icons"`
	Aliases map[string]Entry `json:"aliases"`
	// The set's defaults for its icons' boxes.
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Entry is an icon (Body) or an alias of one (Parent), with its box and turns.
type Entry struct {
	Body   string   `json:"body"`
	Parent string   `json:"parent"`
	Left   *float64 `json:"left"`
	Top    *float64 `json:"top"`
	Width  *float64 `json:"width"`
	Height *float64 `json:"height"`
	Rotate int      `json:"rotate"`
	HFlip  bool     `json:"hFlip"`
	VFlip  bool     `json:"vFlip"`
}

var (
	mu   sync.RWMutex
	sets = map[string]*Set{}
)

// Register adds an Iconify icon set, as the @iconify-json packages ship it
// (icons.json), under name: its icons are then "name:icon". It replaces a set
// registered under the same name.
func Register(name string, iconifyJSON []byte) error {
	var s Set
	if err := json.Unmarshal(iconifyJSON, &s); err != nil {
		return fmt.Errorf("icons: %s: %w", name, err)
	}
	if len(s.Icons) == 0 {
		return fmt.Errorf("icons: %s: no icons", name)
	}
	AddSet(name, &s)
	return nil
}

// AddSet adds a set under name.
func AddSet(name string, s *Set) {
	mu.Lock()
	defer mu.Unlock()
	sets[name] = s
}

// Lookup finds an icon by mermaid's name: "prefix:name", or a bare "name" in
// fallbackPrefix's set. ok is false when it is not registered.
func Lookup(name, fallbackPrefix string) (Icon, bool) {
	prefix, icon, found := strings.Cut(name, ":")
	if !found {
		prefix, icon = fallbackPrefix, name
	}
	mu.RLock()
	s := sets[prefix]
	mu.RUnlock()
	if s == nil {
		return Icon{}, false
	}
	return s.icon(icon)
}

// icon resolves an icon or alias (Iconify's getIconData): the alias chain's
// boxes and turns apply over the parent's.
func (s *Set) icon(name string) (Icon, bool) {
	var chain []Entry
	for range 10 {
		if e, ok := s.Icons[name]; ok {
			chain = append(chain, e)
			break
		}
		e, ok := s.Aliases[name]
		if !ok {
			return Icon{}, false
		}
		chain = append(chain, e)
		name = e.Parent
	}
	base := chain[len(chain)-1]
	if base.Body == "" {
		return Icon{}, false
	}
	ic := Icon{Body: base.Body, Left: s.Left, Top: s.Top, Width: or(s.Width, 16), Height: or(s.Height, 16)}
	rot, hflip, vflip := 0, false, false
	// The base first, then each alias outwards.
	for i := len(chain) - 1; i >= 0; i-- {
		e := chain[i]
		for _, f := range []struct {
			v   *float64
			dst *float64
		}{{e.Left, &ic.Left}, {e.Top, &ic.Top}, {e.Width, &ic.Width}, {e.Height, &ic.Height}} {
			if f.v != nil {
				*f.dst = *f.v
			}
		}
		rot += e.Rotate
		hflip = hflip != e.HFlip
		vflip = vflip != e.VFlip
	}
	ic.transform(rot%4, hflip, vflip)
	return ic, true
}

// transform applies Iconify's flips then quarter turns, keeping the result
// in a box at the origin.
func (ic *Icon) transform(rot int, hflip, vflip bool) {
	if rot == 0 && !hflip && !vflip {
		return
	}
	l, t, w, h := ic.Left, ic.Top, ic.Width, ic.Height
	m := scene.Translate(-l, -t)
	if hflip {
		m = m.Then(scene.Scale(-1, 1)).Then(scene.Translate(w, 0))
	}
	if vflip {
		m = m.Then(scene.Scale(1, -1)).Then(scene.Translate(0, h))
	}
	for range rot {
		// A quarter turn clockwise keeps the box at the origin.
		m = m.Then(scene.Rotate(90)).Then(scene.Translate(h, 0))
		w, h = h, w
	}
	ic.Left, ic.Top, ic.Width, ic.Height, ic.Transform = 0, 0, w, h, m
}

func or(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}

// Unknown is mermaid's unknown icon: a "?" on blue.
var Unknown = Icon{
	Body:  `<g><rect width="80" height="80" style="fill: #087ebf; stroke-width: 0px;"/><text transform="translate(21.16 64.67)" style="fill: #fff; font-family: ArialMT, Arial; font-size: 67.75px;"><tspan x="0" y="0">?</tspan></text></g>`,
	Width: 80, Height: 80,
}

// Draw is the icon in the box (x, y, w, h), scaled to fit and centered as
// an <svg> with a view box. fill is what an unpainted shape is filled with
// (mermaid's SVG inherits the diagram's text color).
func (ic Icon) Draw(x, y, w, h float64, fill scene.Color) scene.Item {
	s := min(w/ic.Width, h/ic.Height)
	m := ic.Transform
	if m == (scene.Matrix{}) {
		m = scene.Identity
	}
	m = m.Then(scene.Translate(-ic.Left, -ic.Top)).Then(scene.Scale(s, s)).
		Then(scene.Translate(x+(w-ic.Width*s)/2, y+(h-ic.Height*s)/2))
	return &scene.Group{Transform: m, Items: drawn(ic.Body, fill)}
}

// drawKey is a body drawn with a fill.
type drawKey struct {
	body string
	fill scene.Color
}

// cache holds each body's items by fill: a body is parsed once, and every
// draw shares its items (scene items are not changed once made).
var cache sync.Map // drawKey → []scene.Item

// drawn is drawBody, cached.
func drawn(body string, fill scene.Color) []scene.Item {
	k := drawKey{body, fill}
	if v, ok := cache.Load(k); ok {
		return v.([]scene.Item)
	}
	// Clipped, so that appending to a group's items never writes into the
	// shared array.
	items := slices.Clip(drawBody(body, fill))
	v, _ := cache.LoadOrStore(k, items)
	return v.([]scene.Item)
}

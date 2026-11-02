# Notices

mermaid-go is MIT licensed (see [LICENSE](LICENSE)). It is a Go port of parts of the projects below; their copyright
notices and licenses apply to the code ported from them and are kept in [LICENSES/](LICENSES).

| Project | Used for | License | Copyright |
|---|---|---|---|
| [mermaid](https://github.com/mermaid-js/mermaid) 12.2.0 | parsers, renderers, themes, styles, config schema, built-in icons; the examples in `testdata/corpus` come from its docs | MIT | Knut Sveidqvist ([text](LICENSES/mermaid.txt)) |
| [dagre-d3-es](https://github.com/tbo47/dagre-es) 7.0.14 (dagre, graphlib) | `dagre`, `graph` | MIT | Chris Pettitt and contributors ([text](LICENSES/dagre-d3-es.txt)) |
| [cytoscape-fcose](https://github.com/iVis-at-Bilkent/cytoscape.js-fcose) 2.2.0, [cose-base](https://github.com/iVis-at-Bilkent/cose-base) 2.2.0, [layout-base](https://github.com/iVis-at-Bilkent/layout-base) 2.0.1 | `fcose` (architecture layout) | MIT | iVis@Bilkent ([fcose](LICENSES/cytoscape-fcose.txt), [cose-base](LICENSES/cose-base.txt), [layout-base](LICENSES/layout-base.txt)) |
| [cytoscape-cose-bilkent](https://github.com/cytoscape/cytoscape.js-cose-bilkent) | `diagrams/mindmap` (not registered yet) | MIT | iVis@Bilkent |
| [d3](https://github.com/d3/d3) (d3-format, d3-scale, d3-time, d3-time-format, d3-hierarchy) | `diagrams/internal/d3` | ISC | Mike Bostock ([text](LICENSES/d3.txt)) |
| [d3-sankey](https://github.com/d3/d3-sankey) 0.12.3 | `diagrams/sankey` | BSD-3-Clause | Mike Bostock ([text](LICENSES/d3-sankey.txt)) |
| [khroma](https://github.com/fabiospampinato/khroma) 2.1 | `color` | MIT | Fabio Spampinato, Andrew Maney ([text](LICENSES/khroma.txt)) |
| [go-text/typesetting](https://github.com/go-text/typesetting) 0.3.5 | `internal/typesetting`: a copy with one fix (below) | Unlicense OR BSD-3-Clause | go-text authors ([text](internal/typesetting/LICENSE)) |

## The typesetting fix

`internal/typesetting` is go-text/typesetting v0.3.5 with one change, in `harfbuzz/set_digest.go`: `setDigest.addRange`
no longer returns early when one of its three masks is full. The early return left the other masks without the
range, so a lookup's digest could reject glyphs it covers and kerning pairs were missed. It is vendored (not a
`replace`) so that programs importing mermaid-go get the fixed shaping too.

## Dependencies

- [d2lang/elk-go](https://github.com/d2lang/elk-go) (EPL-2.0): the ELK layout for flowchart, class, state, ER and
  requirement diagrams.
- golang.org/x/image (BSD-3-Clause), gopkg.in/yaml.v3 (MIT and Apache-2.0).

No font is included. To look like mermaid.live, use Recursive (SIL OFL 1.1; `tools/fonts.py` makes the instance
mermaid.live shows).

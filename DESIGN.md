# mermaid (Go)

Renders Mermaid source to a drawing (shapes and text) and to SVG, with the same
layout and look as mermaid.js 12.2.0 on mermaid.live. Pure Go, no browser.

Reference: mermaid.js 12.2.0 (tag `mermaid@12.2.0`), dagre-d3-es 7.0.14,
cytoscape-fcose 2.2.0 with cose-base 2.2.0 and layout-base 2.0.1 (the same
code is bundled in mermaid.js). mermaid.live renders with each diagram type's
default theme and look (`redux-color` + `neo` for most; see mermaid's
`schemas/config.schema.yaml`) and the Recursive Variable font. The tools
expect local checkouts of these under a reference directory of your choice.

## Scope

Rendering only: parse, lay out, draw. Not ported: clicks, links, callbacks,
tooltips, accessibility (`accTitle`/`accDescr` are parsed and dropped),
security levels, the `handDrawn` look's randomness (later, seeded), KaTeX,
ELK and other optional layout engines unless a default needs them.

The port is not a transliteration. Keep mermaid's results (sizes, positions,
colors); write the code the short, plain Go way. mermaid's code has a lot of
duplication and dead paths: do not carry them over.

## Layout

```
mermaid.go        Render(src, Options) (*Diagram, error): detect type, parse, draw
config/           mermaid's config: defaults (config.schema.yaml), front matter,
                  %%{init}%% directives, per-diagram theme/look defaults
scene/            the drawing: paths, text, groups; and its SVG writer
icons/            mermaid's icon registry: Iconify sets, icon SVG bodies drawn as paths
text/             fonts (switchable, CSS-like fallback), shaping, measuring, wrapping,
                  label markup (markdown, <br>, entities)
color/            khroma's color functions
theme/            mermaid's themes and their variables
graph/            graphlib (dagre's graph)
dagre/            dagre's layout
fcose/            cytoscape-fcose's layout as mermaid's architecture diagram runs it
render/           mermaid's unified renderer (rendering-util): node shapes, edges,
                  markers, clusters, labels, the dagre wrapper
diagrams/<type>/  one package per diagram type: parser, model, drawing
cmd/mermaid/      CLI: renders a corpus directory to SVG for the harness
tools/            ref.mjs (reference and score), corpus.py, fonts.py
testdata/corpus/  every example of mermaid's syntax docs, by page
```

## Conventions

- Units are CSS pixels, float64. Text widths come from `text` only (never
  guessed constants or metric tables): fonts are shaped (HarfBuzz, via
  go-text) and widths rounded up to 1/64 px as Chrome does.
- No font is embedded. The caller registers fonts by family name (bytes or
  files); a theme's CSS font-family list is resolved against them, with
  per-character fallback, as a browser does. To match mermaid.live use
  Recursive Sans Linear: `tools/fonts.py` makes it from Google Fonts'
  Recursive variable font (OFL), pinned to MONO 0, CASL 0, slnt 0, CRSV 0.5.
- A diagram package exposes `Detect(src string) bool` and
  `Render(src string, cfg *config.Config) (*scene.Scene, error)`.
- Styles are resolved when drawing: the scene has explicit colors, widths and
  fonts, no CSS classes.
- Tests: `go test ./...` must pass; each package tests against values taken
  from mermaid.js (dumped by a script into testdata), not against itself.

## Checking

`node tools/ref.mjs ref testdata/corpus <ref>` renders the references with
mermaid.js in Chrome. `go run ./cmd/mermaid testdata/corpus <ours>` renders
ours. `node tools/ref.mjs cmp testdata/corpus <ref> <ours> <report>` scores
each diagram (share of inked pixels that differ) and writes a side-by-side
report.

# render: status

mermaid 12.2.0's unified renderer (rendering-util) for flowchart, class,
state, ER and requirement diagrams. Goal: looks like mermaid.live (theme
redux-color, look neo), not pixel-exact.

## API

```go
s, err := render.Render(d *render.Data, ctx *diagram.Context, render.Options{})
```

- `Data` is mermaid's `db.getData()` (`data.go`, mermaid's field names; type
  specific fields stay in `Node.Extra` / `Edge.Extra`, read with
  `ExtraString` / `ExtraValue`). `LoadData` reads the JSON dumps.
  `Data.DiagramType` (mermaid's id: `flowchart-v2`, `classDiagram`,
  `stateDiagram`, `er`, `requirement`) picks the stylesheet and markers.
- `ctx.Layout` "dagre" uses package dagre; anything else ELK. ELK is
  d2lang/elk-go (`elkengine.go`, `LayoutELK`); `Options.ELK` replaces it for
  one render (the tests replay mermaid's captured answers with it). A failing
  or panicking ELK run falls back to dagre.
- `ctx.Look`, `ctx.Theme`, `ctx.Config`, `ctx.Fonts` as for the other
  diagram packages; `Data.Title` is drawn above.

data.go changes for the parser agent: none so far (still additive-only).

## Supported

- Node shapes: every flowchart shape (shapes.go, shapes_flow.go: all of
  shapes.ts's flowchart names and aliases; icons/images draw frame and label
  only, no glyph or picture), classBox, erBox, requirementBox, state shapes
  (start, end, choice, fork/join, note, rounded states).
- Clusters: rect (flowchart subgraphs, class namespaces), roundedWithTitle
  (composite states, inner/outer rects, alt fill), divider (concurrent
  regions), noteGroup (invisible), collapsed subgraphs.
- Per-type stylesheet (sheet.go): neo shadows, color-theme palette slots
  (clusters; class outlines and dividers; every path of ER and requirement
  boxes), note colors, state start/fork fill, edge label backgrounds (flowchart
  and state #ccc, class/ER/requirement white), class end labels at 11px,
  titles (18px only for flowchart and state, as mermaid's CSS).
- Markers: point, circle, cross (classic and neo margin variants), class
  (extension, composition, aggregation, dependency, lollipop), state barb,
  ER cardinalities, requirement contains/arrow; colored by the edge's stroke.
- Edges: ELK routes post-processed as mermaid (clip to outlines, short
  marker stubs, terminal jog straightening, end labels kept off frames),
  rounded corners; dagre routes with the configured curve (basis default),
  clipped to shapes and cut at cluster frames.
- Labels: HTML label model (wrapping at wrappingWidth, min width, markdown
  bold/italic, `<br>`, `white-space: nowrap` styles); htmlLabels:false gives
  SVG-text sizing (1.1em lines, translucent edge label background).
- Nesting: arbitrary depth with both engines; cluster directions with ELK.
- elk-go reserves no room for cluster titles: `reserveTitles` adds the
  label's height (+5) to the top padding before laying out.

## Scores

`node tools/ref.mjs cmp` (mean similarity, count ≥0.9), corpus testdata/corpus,
after this session (2026-11-02):

| type | replayed ELK (mermaid's layout) | elk-go / dagre |
|---|---|---|
| flowchart (114) | 0.939, 95 | 0.939, 93 |
| class (38) | 0.880, 28 | 0.857, 24 |
| state (22) | 0.849, 13 | 0.852, 10 |
| ER (25) | 0.890, 22 | 0.912, 21 |
| requirement (9) | 0.705, 1 | 0.743, 1 |
| real corpus (166) | 0.964, 158 | 0.649, 27 |

The pixel score is harsh on text anti-aliasing and 1–2px shifts; most
cases under 0.9 look the same side by side. Engine-mode differences are
elk-go's (ELK 0.12) layouts, accepted. ELK input built from getData matches
mermaid's: flowchart 109/113, real 149/159, class 36/37, ER 22/23,
requirement 8/8, state 8/21 (the state misses are the end circle's
0.013px roughjs overhang and dump ids), node sizes 5408/5526.

## Testing

```
cd libs/mermaid
MERMAID_OUT=D:/.../ours go test ./render -count=1 -run TestElkInputs   # replay mermaid's ELK answers
LAYOUT=engine MERMAID_OUT=D:/.../ours go test ./render -count=1 -run TestElkInputs   # elk-go / dagre
ONLY=flowchart node tools/ref.mjs cmp testdata/corpus ../../data/mermaid-port/ref D:/.../ours D:/.../report
```

VERBOSE=1 lists ELK input differences. Dump artifacts (not bugs): the
getData dumps were made with the default config, so classDiagram/22
(hierarchicalNamespaces false) and stateDiagram/17 (random ids) differ;
class note colors are patched in the test loader.

## Left

- dagre: clusters keep the diagram direction (mermaid lays out isolated
  clusters recursively with their own `dir`); no extra room under cluster
  titles; self-loops and edge labels are dagre's.
- Not drawn: icon glyphs, images, links/clicks (mermaid drops the slot colors
  of linked class nodes; we keep them), handDrawn look (draws as classic/neo).
- ER/requirement markers use the neo shapes in the classic look too.
- Small: the state end circle is 14 instead of 14.013 wide; diamonds whose
  labels contain entity-escaped `<br>` size differently; labels with glyphs
  from fallback fonts (CJK, emoji) can differ by a few px in width.
- Other diagram types that use the unified renderer (usecase, agentflow,
  swimlanes, kanban, mindmap) render but have no stylesheet of their own.

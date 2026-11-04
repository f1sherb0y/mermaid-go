// Loads dagre-d3-es 7.0.14 from data/ref (its sources import the bare
// specifier 'lodash-es', resolved here to this directory's node_modules) and
// runs layouts in the shape the Go tests read.
import { registerHooks } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const here = path.dirname(fileURLToPath(import.meta.url));
// The reference data (mermaid.js, fonts, real diagrams): $MERMAID_DATA, else
// the data folder of a MeetBuddy checkout beside this repository.
const DATA = process.env.MERMAID_DATA ?? path.join(here, "../../../shitou/data");
const lodash = pathToFileURL(path.join(here, 'node_modules/lodash-es/lodash.js')).href;
registerHooks({
  resolve(specifier, context, next) {
    if (specifier === 'lodash-es') return { url: lodash, shortCircuit: true };
    return next(specifier, context);
  },
});

export const src = pathToFileURL(
  path.resolve(here, '../../../../data/ref/pkg/dagre-d3-es-7.0.14/package/src') + '/',
).href;
export const { Graph } = await import(src + 'graphlib/index.js');
export const { layout } = await import(src + 'dagre/index.js');
export const _ = await import(lodash);

const edgeKeys = ['minlen', 'weight', 'width', 'height', 'labelpos', 'labeloffset'];

// build makes the input graph of a case: nodes in order (each followed by
// setParent, as mermaid does), then edges.
export function build(c) {
  const g = new Graph({ multigraph: !!c.multigraph, compound: !!c.compound }).setGraph({ ...c.opts });
  for (const n of c.nodes) {
    if (n.nolabel) g.setNode(n.id);
    else g.setNode(n.id, { width: n.width, height: n.height });
    if (n.parent !== undefined) g.setParent(n.id, n.parent);
  }
  for (const e of c.edges) {
    const label = {};
    for (const k of edgeKeys) if (k in e) label[k] = e[k];
    g.setEdge(e.v, e.w, label, e.name);
  }
  return g;
}

// run lays a case out and returns the lodash uniqueId counter it started
// from plus the results.
export function run(c) {
  const g = build(c);
  const idStart = Number(_.uniqueId());
  layout(g);
  const nodes = {};
  for (const v of g.nodes()) {
    const n = g.node(v);
    if (n) nodes[v] = [n.x, n.y, n.width, n.height];
  }
  const edges = g.edges().map((e) => {
    const l = g.edge(e);
    const out = { v: e.v, w: e.w, points: (l.points || []).flatMap((p) => [p.x, p.y]) };
    if (e.name !== undefined) out.name = e.name;
    if (Object.prototype.hasOwnProperty.call(l, 'x')) out.xy = [l.x, l.y];
    return out;
  });
  return { idStart, width: g.graph().width, height: g.graph().height, nodes, edges };
}

// JSON has no NaN or Infinity; write them as strings.
export const replacer = (k, v) => (typeof v === 'number' && !Number.isFinite(v) ? String(v) : v);

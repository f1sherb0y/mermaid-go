// Generates the reference data for the Go graph and dagre packages:
//
//   graph/testdata/ops.json.gz       random graphlib operations and the
//                                    resulting listings (order matters)
//   dagre/testdata/jssort.json.gz    V8's sort with dagre's comparator on
//                                    NaN barycenters
//   dagre/testdata/layouts.json.gz   layout inputs and dagre's results
//   dagre/testdata/bench.json.gz     the benchmark graphs and their results
//
// Usage: node gen.mjs   (from anywhere; paths are relative to this file)
import { writeFileSync } from 'node:fs';
import { gzipSync } from 'node:zlib';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { Graph, run, replacer } from './ref.mjs';
import { flowchart, compound, nested, flat, idCases, edgeCases, benchSpecs, benchCase, mulberry } from './cases.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const lib = path.resolve(here, '../..');

function write(rel, data) {
  const file = path.join(lib, rel);
  const buf = gzipSync(JSON.stringify(data, replacer));
  writeFileSync(file, buf);
  console.log(`${rel}: ${Array.isArray(data) ? data.length + ' entries, ' : ''}${buf.length} bytes`);
}

// --- graphlib operations ---

function opsCases() {
  const rng = mulberry(7);
  const pick = (a) => a[Math.floor(rng() * a.length)];
  const pool = ['a', 'b', 'c', 'd', 'e', '0', '1', '2', '10', '7', '01', '-1', 'x y', '\u{1F600}', '！', 'length'];
  const out = [];
  for (let t = 0; t < 300; t++) {
    const opt = { directed: rng() < 0.75, multigraph: rng() < 0.5, compound: rng() < 0.6 };
    const g = new Graph(opt);
    const ops = [];
    const nops = 5 + Math.floor(rng() * 60);
    let label = 0;
    for (let i = 0; i < nops; i++) {
      const r = rng();
      const v = pick(pool), w = pick(pool);
      if (r < 0.25) {
        if (rng() < 0.3) { g.setNode(v); ops.push(['n', v]); }
        else { g.setNode(v, ++label); ops.push(['n', v, label]); }
      } else if (r < 0.35) {
        g.removeNode(v); ops.push(['rn', v]);
      } else if (r < 0.7) {
        const name = opt.multigraph && rng() < 0.4 ? pick(['x', 'y', '1']) : undefined;
        g.setEdge(v, w, ++label, name); ops.push(['e', v, w, name ?? '', label]);
      } else if (r < 0.82) {
        const es = g.edges();
        if (es.length) {
          const e = pick(es);
          g.removeEdge(e); ops.push(['re', e.v, e.w, e.name ?? '']);
        }
      } else if (opt.compound) {
        const p = rng() < 0.2 ? undefined : w;
        let ok = true;
        for (let a = p; a !== undefined; a = g.parent(a)) if (a === v) ok = false;
        if (ok) { g.setParent(v, p); ops.push(['p', v, p ?? '']); }
      }
    }
    const nodes = {};
    for (const v of g.nodes()) {
      nodes[v] = {
        label: g.node(v) ?? null,
        preds: g.predecessors(v), sucs: g.successors(v), neighbors: g.neighbors(v),
        in: g.inEdges(v).map(ek), out: g.outEdges(v).map(ek), nodeEdges: g.nodeEdges(v).map(ek),
        children: g.children(v), parent: g.parent(v) ?? '',
      };
    }
    out.push({
      opt, ops,
      nodes: g.nodes(), edges: g.edges().map((e) => [...ek(e), g.edge(e)]),
      roots: g.children(), sources: g.sources(), sinks: g.sinks(), detail: nodes,
    });
  }
  return out;
}
const ek = (e) => [e.v, e.w, e.name ?? ''];

// --- V8 sort ---

function sortCases() {
  const rng = mulberry(11);
  const out = [];
  for (let t = 0; t < 400; t++) {
    const n = t < 100 ? t : Math.floor(rng() * (t < 300 ? 200 : 1500));
    const levels = 1 + Math.floor(rng() * 12);
    const nan = rng() * 0.5;
    const bias = rng() < 0.5;
    const bc = [];
    for (let i = 0; i < n; i++) bc.push(rng() < nan ? NaN : Math.floor(rng() * levels));
    // Mostly random order; some cases presorted or reversed runs.
    if (rng() < 0.2) bc.sort((a, b) => a - b);
    else if (rng() < 0.2) bc.sort((a, b) => b - a);
    const entries = bc.map((b, i) => ({ b, i }));
    entries.sort((x, y) => {
      if (x.b < y.b) return -1;
      if (x.b > y.b) return 1;
      return bias ? y.i - x.i : x.i - y.i;
    });
    out.push({ bias, bc, order: entries.map((e) => e.i) });
  }
  return out;
}

// --- layouts ---

function layoutCases(cases) {
  const out = [];
  let thrown = 0;
  for (const c of cases) {
    try {
      out.push({ ...c, want: run(c) });
    } catch (err) {
      if (!c.mayThrow) console.log(`  ${c.name} throws: ${err.message}`);
      out.push({ ...c, throws: err.message });
      thrown++;
    }
  }
  console.log(`  ${out.length} layouts, ${thrown} throw in dagre`);
  return out;
}

write('graph/testdata/ops.json.gz', opsCases());
write('dagre/testdata/jssort.json.gz', sortCases());
write('dagre/testdata/layouts.json.gz', layoutCases([
  ...edgeCases(),
  ...idCases(),
  ...flat(),
  ...compound(),
  ...nested(),
  ...flowchart(),
]));
write('dagre/testdata/bench.json.gz', benchSpecs.map(([name, seed, ...spec]) => {
  const c = benchCase(name, seed, ...spec);
  return { ...c, want: run(c) };
}));

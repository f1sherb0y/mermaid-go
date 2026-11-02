// Dumps dagre's layout graph after every step of runLayout for chosen cases,
// to find where the Go port first differs:
//
//   node trace.mjs flat-156 [more case names]
//   DAGRE_TRACE=1 go test ./dagre -run TestTrace
//
// Writes dagre/testdata/trace.json (not checked in).
import { registerHooks } from 'node:module';
import { writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Make runLayout report each step: its `time` wrapper is replaced by one
// that calls globalThis.__step with the step name and the layout graph.
registerHooks({
  load(url, context, next) {
    const r = next(url, context);
    if (url.endsWith('/dagre/layout.js')) {
      const src = String(r.source).replace(
        'function runLayout(g, time) {',
        'function runLayout(g, time) { time = (name, fn) => { const r = fn(); globalThis.__step(name.trim(), g); return r; };',
      );
      return { ...r, source: src };
    }
    if (url.endsWith('/position/bk.js')) {
      const src = String(r.source)
        .replace("      xss[vert + horiz] = xs;", "      xss[vert + horiz] = xs; globalThis.__bk?.(vert + horiz, { root: align.root, align: align.align, xs, layering: adjustedLayering });")
        .replace("  var xss = {};", "  var xss = {}; globalThis.__bk?.('conflicts', conflicts);");
      return { ...r, source: src };
    }
    return r;
  },
});

const { run, _ } = await import('./ref.mjs');
const { flat, compound, nested, flowchart, idCases, edgeCases } = await import('./cases.mjs');

const all = [...edgeCases(), ...idCases(), ...flat(), ...compound(), ...nested(), ...flowchart()];
const out = [];
for (const name of process.argv.slice(2)) {
  const c = all.find((c) => c.name === name);
  const steps = [];
  const bk = [];
  globalThis.__bk = (name, data) => bk.push([name, JSON.parse(JSON.stringify(data, (k, v) => (typeof v === 'number' && !Number.isFinite(v) ? String(v) : v)))]);
  globalThis.__step = (step, g) => {
    steps.push({
      step,
      nodes: g.nodes().map((v) => {
        const n = g.node(v) || {};
        return [v, n.rank ?? null, n.order ?? null, n.x ?? null, n.y ?? null, n.width ?? null, n.height ?? null, g.parent(v) ?? ''];
      }),
      edges: g.edges().map((e) => [e.v, e.w, e.name ?? '']),
    });
  };
  let err = '';
  const idStart = Number(_.uniqueId()) + 1;
  try { run(c); } catch (e) { err = e.message; }
  out.push({ case: { ...c, want: { idStart } }, steps, err, bk });
}
const here = path.dirname(fileURLToPath(import.meta.url));
writeFileSync(path.resolve(here, '../../dagre/testdata/trace.json'), JSON.stringify(out, (k, v) => (typeof v === 'number' && !Number.isFinite(v) ? String(v) : v)));
console.log(out.map((o) => `${o.case.name}: ${o.steps.length} steps ${o.err}`).join('\n'));

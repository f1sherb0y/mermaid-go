// Times dagre-d3-es on the benchmark graphs (dagre/testdata/bench.json.gz),
// the same ones BenchmarkLayout uses. Graph building is not timed.
//
// Usage: node bench.mjs [seconds per graph, default 3]
import { readFileSync } from 'node:fs';
import { gunzipSync } from 'node:zlib';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { build, layout } from './ref.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const cases = JSON.parse(gunzipSync(readFileSync(path.resolve(here, '../../dagre/testdata/bench.json.gz'))));
const budget = Number(process.argv[2] || 3) * 1000;

for (const c of cases) {
  for (let i = 0; i < 3; i++) layout(build(c)); // warm up
  let n = 0, total = 0;
  while (total < budget) {
    const g = build(c);
    const t0 = performance.now();
    layout(g);
    total += performance.now() - t0;
    n++;
  }
  console.log(`${c.name.padEnd(20)} ${(total / n).toFixed(2).padStart(9)} ms/op  (${n} runs)`);
}

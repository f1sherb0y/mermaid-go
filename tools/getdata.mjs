// Dumps what mermaid.js's db.getData() returns (LayoutData) for each diagram of a corpus,
// for the unified renderer's tests: node getdata <corpus> <out> [type substring].
// Writes <out>/<rel>.json; diagrams without getData are skipped.
import { chromium } from "playwright-core";
import fs from "node:fs";
import path from "node:path";

const here = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"));
const MERMAID_JS = process.env.MERMAID_JS ?? path.join(here, "../../../data/ref/pkg/mermaid-12.2.0/package/dist/mermaid.min.js");
const [corpus, out] = process.argv.slice(2);

function walk(dir, base = dir) {
  let r = [];
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) r = r.concat(walk(p, base));
    else if (e.name.endsWith(".mmd")) r.push(path.relative(base, p).replace(/\\/g, "/").replace(/\.mmd$/, ""));
  }
  return r.sort().filter((x) => !process.env.ONLY || x.includes(process.env.ONLY));
}

const b = await chromium.launch({ executablePath: process.env.CHROME ?? "C:/Program Files/Google/Chrome/Application/chrome.exe" });
const page = await b.newPage();
await page.setContent("<html><body></body></html>");
await page.addScriptTag({ path: MERMAID_JS });
let n = 0;
for (const rel of walk(corpus)) {
  const src = fs.readFileSync(path.join(corpus, rel + ".mmd"), "utf8");
  const r = await page.evaluate(async (src) => {
    try {
      mermaid.initialize({ startOnLoad: false });
      const d = await mermaid.mermaidAPI.getDiagramFromText(src);
      if (!d.db.getData) return { skip: true };
      const data = d.db.getData();
      const seen = new WeakSet();
      // Plain data only: functions, DOM and config objects out.
      const json = JSON.stringify(data, (k, v) => {
        if (typeof v === "function" || k === "config" || k === "intersect" || k === "calcIntersect") return undefined;
        if (v && typeof v === "object") {
          if (seen.has(v)) return undefined;
          seen.add(v);
        }
        return v;
      });
      return { type: d.type, json };
    } catch (e) {
      return { err: String(e.message ?? e) };
    }
  }, src);
  if (r.skip || r.err) continue;
  const f = path.join(out, rel + ".json");
  fs.mkdirSync(path.dirname(f), { recursive: true });
  fs.writeFileSync(f, JSON.stringify({ type: r.type, data: JSON.parse(r.json) }, null, 1));
  n++;
}
await b.close();
console.log("dumped", n);

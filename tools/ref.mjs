// The reference and the score: mermaid.js itself, in headless Chrome.
//
//   node tools/ref.mjs ref <corpus> <out>          mermaid.js renders every <corpus>/**/*.mmd
//                                                   to <out>/<rel>.svg and .png (each diagram type's default theme and look,
//                                                   as mermaid.live).
//   node tools/ref.mjs cmp <corpus> <ref> <ours> <report>
//                                                   rasterizes <ours>/<rel>.svg the same way and
//                                                   scores it against <ref>/<rel>.png:
//                                                   <report>/scores.json and index.html.
//   node tools/ref.mjs measure <in.json> <out.json> text widths and heights in Chrome, for the
//                                                   text measurer's tests.
//
// Env: MERMAID_JS (default $MERMAID_DATA/ref/pkg/mermaid-12.2.0/package/dist/mermaid.min.js),
// CHROME (default the Windows install), ONLY=<substring> to restrict the files; CAPTURE_ELK and
// CAPTURE_FCOSE (with FCOSE_TRACE=<n> for the first n iterations' rects) record layout input and
// output for the ELK and fcose ports' tests (see below).
import { chromium } from "playwright-core";
import fs from "node:fs";
import path from "node:path";

const here = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"));
// The reference data (mermaid.js, fonts, real diagrams): $MERMAID_DATA, else
// the data folder of a MeetBuddy checkout beside this repository.
const DATA = process.env.MERMAID_DATA ?? path.join(here, "../../shitou/data");
const MERMAID_JS = process.env.MERMAID_JS ?? path.join(DATA, "ref/pkg/mermaid-12.2.0/package/dist/mermaid.min.js");
const CHROME = process.env.CHROME ?? "C:/Program Files/Google/Chrome/Application/chrome.exe";

function walk(dir, base = dir) {
  let out = [];
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) out = out.concat(walk(p, base));
    else if (e.name.endsWith(".mmd")) out.push(path.relative(base, p).replace(/\\/g, "/").replace(/\.mmd$/, ""));
  }
  return out.sort().filter((r) => !process.env.ONLY || r.includes(process.env.ONLY));
}

const WORKERS = Number(process.env.WORKERS ?? 8);

// CAPTURE_ELK=<dir>: also records every graph mermaid hands ELK, and ELK's answer, as
// <dir>/<rel>.<n>.json ({input, output}). Done by wrapping the one elk.layout call in the
// bundle; for testing the ELK port only.
const CAPTURE_ELK = process.env.CAPTURE_ELK;
const ELK_CALL = "l=await t.layout(r)";
// CAPTURE_FCOSE=<dir>: records each architecture diagram's fcose runs as <dir>/<rel>.json: per
// run (mermaid runs fcose twice) the cose-base graph fcose builds (nodes in getAllNodes order with
// their rects and paddings, edges with ideal lengths and elasticities), the constraints, the
// elements' bounding box fcose recenters on, the rects after the layout and the number of
// Math.random calls; then cytoscape's final positions and bounding boxes. Done by wrapping the
// runLayout call in fcose's coseLayout and the architecture renderer's cy.ready; for testing
// the fcose port only.
const CAPTURE_FCOSE = process.env.CAPTURE_FCOSE;
const FCOSE_CALL = "te(z,M),z.runLayout(),J},\"coseLayout\")";
const FCOSE_READY = "b.ready(I=>{Pe.info(\"Ready\",I),p(b)})";
const mermaidScript = () => {
  let js = fs.readFileSync(MERMAID_JS, "utf8");
  if (CAPTURE_ELK) {
    if (!js.includes(ELK_CALL)) throw new Error("CAPTURE_ELK: the elk.layout call was not found in " + MERMAID_JS);
    js = js.replace(ELK_CALL, "l=await(async()=>{const i=JSON.stringify(r);const o=await t.layout(r);(globalThis.__elk??=[]).push({input:JSON.parse(i),output:JSON.parse(JSON.stringify(o))});return o})()");
  }
  if (CAPTURE_FCOSE) {
    if (!js.includes(FCOSE_CALL) || !js.includes(FCOSE_READY)) throw new Error("CAPTURE_FCOSE: the fcose calls were not found in " + MERMAID_JS);
    js = js.replace(FCOSE_CALL, "te(z,M),(globalThis.__fcoseRun?globalThis.__fcoseRun(z,M,()=>z.runLayout()):z.runLayout()),J},\"coseLayout\")");
    js = js.replace(FCOSE_READY, "b.ready(I=>{globalThis.__fcoseDone&&globalThis.__fcoseDone(b),Pe.info(\"Ready\",I),p(b)})");
  }
  return js;
};

// The page side of CAPTURE_FCOSE: fills globalThis.__fcose for one render.
const fcoseHooks = () => {
  const cap = (globalThis.__fcose = { runs: [] });
  const rect = (n) => ({ x: n.rect.x, y: n.rect.y, w: n.rect.width, h: n.rect.height });
  globalThis.__fcoseRun = (layout, options, run) => {
    const gm = layout.graphManager;
    const nodes = [], edges = [];
    for (const g of gm.graphs) {
      for (const n of g.nodes) nodes.push({ id: n.id, parent: g.parent && g.parent.id !== undefined ? g.parent.id : null, compound: n.child != null, pad: n.paddingLeft, rep: n.nodeRepulsion, ...rect(n) });
      for (const e of g.edges) edges.push({ id: e.id, s: e.source.id, t: e.target.id, ideal: e.idealLength, elas: e.edgeElasticity, inter: false });
    }
    for (const e of gm.edges) edges.push({ id: e.id, s: e.source.id, t: e.target.id, ideal: e.idealLength, elas: e.edgeElasticity, inter: true });
    const bb = options.eles.boundingBox();
    const r = {
      nodes, edges, bbox: { x1: bb.x1, y1: bb.y1, x2: bb.x2, y2: bb.y2 },
      cy: options.eles.nodes().map((n) => ({ id: n.id(), x: n.position("x"), y: n.position("y"), parent: n.isParent(), dims: n.layoutDimensions({ nodeDimensionsIncludeLabels: false }) })),
      cyEdges: options.eles.edges().map((e) => ({ id: e.id(), s: e.source().id(), t: e.target().id() })),
      alignment: JSON.parse(JSON.stringify(options.alignmentConstraint ?? null)),
      relative: JSON.parse(JSON.stringify(options.relativePlacementConstraint ?? null)),
    };
    cap.runs.push(r);
    if (globalThis.__fcoseTrace) {
      // FCOSE_TRACE: every node's rect and forces before each of the first iterations.
      r.trace = [];
      const tick = layout.tick;
      layout.tick = function () {
        if (this.totalIterations < globalThis.__fcoseTrace)
          r.trace.push({ it: this.totalIterations, cf: this.coolingFactor, nodes: this.graphManager.getAllNodes().map((n) => [n.id, n.rect.x, n.rect.y, n.rect.width, n.rect.height]) });
        return tick.call(this);
      };
      const ud = layout.updateDisplacements;
      layout.updateDisplacements = function () {
        const t = this.totalIterations < globalThis.__fcoseTrace;
        const f = t && this.graphManager.getAllNodes().map((n) => [n.id, n.springForceX, n.springForceY, n.repulsionForceX, n.repulsionForceY, n.gravitationForceX, n.gravitationForceY, n.displacementX, n.displacementY]);
        const res = ud.call(this);
        if (t) r.trace.push({ it: this.totalIterations, forces: f, after: this.graphManager.getAllNodes().map((n) => [n.id, n.displacementX, n.displacementY]), temp: this.nodeToTempPositionMapHorizontal ? [...this.nodeToTempPositionMapHorizontal] : null });
        return res;
      };
    }
    const orig = Math.random;
    let calls = 0;
    r.rnd = [];
    Math.random = function () { calls++; const v = orig(); if (r.rnd.length < 20) r.rnd.push(v); return v; };
    try {
      return run();
    } catch (e) {
      r.err = String(e && e.message ? e.message : e);
      throw e;
    } finally {
      Math.random = orig;
      r.randoms = calls;
      r.iterations = layout.totalIterations;
      r.idealEdgeLength = layout.idealEdgeLength;
      r.out = gm.getAllNodes().map((n) => ({ id: n.id, ...rect(n) }));
    }
  };
  globalThis.__fcoseDone = (cy) => {
    cap.final = cy.nodes().map((n) => {
      const bb = n.boundingBox();
      return { id: n.id(), parent: n.isParent(), x: n.position("x"), y: n.position("y"), bb: { x1: bb.x1, y1: bb.y1, x2: bb.x2, y2: bb.y2 } };
    });
    cap.edges = cy.edges().map((e) => {
      const s = e.sourceEndpoint(), m = e.midpoint(), t = e.targetEndpoint();
      return { id: e.id(), s: [s.x, s.y], m: [m.x, m.y], t: [t.x, t.y] };
    });
  };
};

async function browser() {
  return chromium.launch({ executablePath: CHROME, headless: true });
}

const newPage = (b) => b.newPage({ viewport: { width: 1600, height: 1200 }, deviceScaleFactor: 1 });

// Runs job(item, worker) over items with WORKERS workers; setup(b) makes a worker's state.
async function pool(items, setup, job) {
  const b = await browser();
  let next = 0, done = 0;
  await Promise.all(Array.from({ length: Math.min(WORKERS, items.length) }, async () => {
    const w = await setup(b);
    while (next < items.length) {
      const it = items[next++];
      await job(it, w);
      if (++done % 100 === 0) console.log(done, "/", items.length);
    }
  }));
  await b.close();
}

// The fonts mermaid.live loads: Recursive Variable (@fontsource-variable/recursive, crsv.css), inlined
// so an about:blank page can use them. RECURSIVE: that package's directory.
const RECURSIVE = process.env.RECURSIVE ?? path.join(DATA, "ref/pkg/recursive/package");
const FONTS = fs.readFileSync(path.join(RECURSIVE, "crsv.css"), "utf8").replace(/url\(\.\/files\/([^)]+)\)/g,
  (_, f) => `url(data:font/woff2;base64,${fs.readFileSync(path.join(RECURSIVE, "files", f)).toString("base64")})`);

// The page every SVG is drawn in: white, no margins, the SVG at its viewBox size.
const SHELL = `<html><head><style>${FONTS}html,body{margin:0;background:#fff}#c{display:inline-block}</style></head><body><div id="c"></div></body></html>`;

// Waits for the fonts, as mermaid.live has them before it renders.
const fontsReady = (page) => page.evaluate(async () => {
  for (const w of [400, 600, 700]) await document.fonts.load(`${w} 14px "Recursive Variable"`, "Aa");
  await document.fonts.ready;
});

async function rasterize(page, svg, png) {
  await page.setContent(SHELL);
  const size = await page.evaluate((svg) => {
    const c = document.getElementById("c");
    c.innerHTML = svg;
    const s = c.querySelector(":scope > svg");
    if (!s) return null;
    const vb = s.viewBox && s.viewBox.baseVal;
    let w = vb && vb.width, h = vb && vb.height;
    if (!w || !h) {
      const r = s.getBoundingClientRect();
      w = r.width; h = r.height;
    }
    s.removeAttribute("style");
    s.setAttribute("width", w);
    s.setAttribute("height", h);
    return { w: Math.ceil(w), h: Math.ceil(h) };
  }, svg);
  if (!size || !size.w || !size.h) return null;
  await page.setViewportSize({ width: Math.min(Math.max(size.w, 16), 8000), height: Math.min(Math.max(size.h, 16), 8000) });
  await fontsReady(page);
  await page.locator("#c > svg").screenshot({ path: png, omitBackground: false });
  return size;
}

async function ref(corpus, out) {
  const files = walk(corpus);
  const fails = {};
  let n = 0;
  await pool(files, async (b) => {
    const render = await newPage(b), raster = await newPage(b);
    await render.setContent(SHELL);
    await render.addScriptTag({ content: mermaidScript() });
    await fontsReady(render);
    return { render, raster };
  }, async (rel, { render, raster }) => {
    const src = fs.readFileSync(path.join(corpus, rel + ".mmd"), "utf8");
    // mermaid.render resets the config a diagram's front matter set.
    if (CAPTURE_FCOSE) await render.evaluate(fcoseHooks);
    if (CAPTURE_FCOSE && process.env.FCOSE_TRACE) await render.evaluate((n) => { globalThis.__fcoseTrace = n; }, Number(process.env.FCOSE_TRACE));
    const r = await render.evaluate(async (src) => {
      globalThis.__elk = [];
      try {
        mermaid.initialize({ startOnLoad: false });
        const { svg } = await mermaid.render("mermaid-svg", src);
        return { svg, elk: globalThis.__elk, fcose: globalThis.__fcose };
      } catch (e) {
        return { err: String(e && e.message ? e.message : e), fcose: globalThis.__fcose };
      }
    }, src);
    if (CAPTURE_FCOSE && r.fcose && r.fcose.runs.length) {
      const f = path.join(CAPTURE_FCOSE, `${rel}.json`);
      fs.mkdirSync(path.dirname(f), { recursive: true });
      fs.writeFileSync(f, JSON.stringify({ src, err: r.err, ...r.fcose }));
    }
    if (CAPTURE_ELK && r.elk) {
      r.elk.forEach((c, i) => {
        const f = path.join(CAPTURE_ELK, `${rel}.${i}.json`);
        fs.mkdirSync(path.dirname(f), { recursive: true });
        fs.writeFileSync(f, JSON.stringify(c));
      });
    }
    const dst = path.join(out, rel);
    fs.mkdirSync(path.dirname(dst), { recursive: true });
    if (r.err) {
      fails[rel] = r.err.slice(0, 300);
      fs.writeFileSync(dst + ".err", r.err);
      return;
    }
    fs.writeFileSync(dst + ".svg", r.svg);
    if (!(await rasterize(raster, r.svg, dst + ".png"))) fails[rel] = "no size";
    else n++;
  });
  console.log("rendered", n, "of", files.length, "failed", Object.keys(fails).length);
  fs.writeFileSync(path.join(out, "failures.json"), JSON.stringify(fails, null, 1));
}

// Scores two PNGs in the page: the share of inked pixels (in either) that differ.
async function score(page, a, b) {
  return page.evaluate(async ([a, b]) => {
    const load = (src) => new Promise((ok, no) => { const i = new Image(); i.onload = () => ok(i); i.onerror = no; i.src = src; });
    const [ia, ib] = await Promise.all([load(a), load(b)]);
    const w = Math.max(ia.width, ib.width), h = Math.max(ia.height, ib.height);
    const px = (img) => {
      const c = document.createElement("canvas"); c.width = w; c.height = h;
      const x = c.getContext("2d"); x.fillStyle = "#fff"; x.fillRect(0, 0, w, h); x.drawImage(img, 0, 0);
      return x.getImageData(0, 0, w, h).data;
    };
    const pa = px(ia), pb = px(ib);
    let ink = 0, diff = 0;
    for (let i = 0; i < pa.length; i += 4) {
      const inkA = pa[i] < 235 || pa[i + 1] < 235 || pa[i + 2] < 235;
      const inkB = pb[i] < 235 || pb[i + 1] < 235 || pb[i + 2] < 235;
      if (!inkA && !inkB) continue;
      ink++;
      if (Math.abs(pa[i] - pb[i]) > 48 || Math.abs(pa[i + 1] - pb[i + 1]) > 48 || Math.abs(pa[i + 2] - pb[i + 2]) > 48) diff++;
    }
    return { score: ink ? 1 - diff / ink : 1, ref: [ia.width, ia.height], ours: [ib.width, ib.height] };
  }, [a, b]);
}

const dataURL = (p) => "data:image/png;base64," + fs.readFileSync(p).toString("base64");

async function cmp(corpus, refDir, ours, report) {
  const files = walk(corpus).filter((rel) => fs.existsSync(path.join(refDir, rel + ".png")));
  fs.mkdirSync(path.join(report, "png"), { recursive: true });
  const scores = {};
  await pool(files, async (b) => ({ raster: await newPage(b), cmp: await newPage(b) }), async (rel, { raster, cmp }) => {
    const refPng = path.join(refDir, rel + ".png");
    const svgPath = path.join(ours, rel + ".svg");
    const ourPng = path.join(report, "png", rel.replace(/\//g, "__") + ".png");
    let s = { score: 0, missing: true };
    if (fs.existsSync(svgPath)) {
      if (await rasterize(raster, fs.readFileSync(svgPath, "utf8"), ourPng)) {
        await cmp.setContent(SHELL);
        s = await score(cmp, dataURL(refPng), dataURL(ourPng));
      }
    } else if (fs.existsSync(path.join(ours, rel + ".err"))) {
      s.err = fs.readFileSync(path.join(ours, rel + ".err"), "utf8").slice(0, 300);
    }
    scores[rel] = s;
  });
  fs.writeFileSync(path.join(report, "scores.json"), JSON.stringify(scores, null, 1));
  const byType = {};
  for (const [rel, s] of Object.entries(scores)) {
    const t = rel.split("/")[0];
    (byType[t] ??= []).push(s.score);
  }
  const rows = Object.entries(byType).map(([t, v]) => [t, v.length, v.reduce((a, b) => a + b, 0) / v.length, v.filter((x) => x >= 0.9).length]);
  rows.sort((a, b) => a[0].localeCompare(b[0]));
  let all = 0, cnt = 0, good = 0;
  for (const r of rows) { all += r[2] * r[1]; cnt += r[1]; good += r[3]; console.log(r[0].padEnd(28), String(r[1]).padStart(4), r[2].toFixed(3), "≥0.9:", r[3]); }
  console.log("ALL".padEnd(28), String(cnt).padStart(4), (all / Math.max(cnt, 1)).toFixed(3), "≥0.9:", good);
  const rel2 = (p) => path.relative(report, p).replace(/\\/g, "/");
  const html = ["<meta charset=utf-8><style>body{font-family:sans-serif}td{vertical-align:top;border-top:1px solid #ccc;padding:6px}img{max-width:700px;border:1px solid #eee}</style><table>"];
  for (const [rel, s] of Object.entries(scores).sort((a, b) => a[1].score - b[1].score)) {
    const flat = rel.replace(/\//g, "__");
    html.push(`<tr><td><b>${rel}</b><br>${s.score.toFixed(3)}<br>${s.err ? s.err.replace(/</g, "&lt;") : ""}</td><td><img src="${rel2(path.join(refDir, rel + ".png"))}"></td><td>${s.missing ? "—" : `<img src="png/${flat}.png">`}</td></tr>`);
  }
  html.push("</table>");
  fs.writeFileSync(path.join(report, "index.html"), html.join("\n"));
}

// Text measured as mermaid measures it: an SVG <text> getBBox, and an HTML label's box.
async function measure(inFile, outFile) {
  const items = JSON.parse(fs.readFileSync(inFile, "utf8"));
  const b = await browser();
  const page = await newPage(b);
  await page.setContent(SHELL);
  await fontsReady(page);
  const res = await page.evaluate((items) => {
    const ns = "http://www.w3.org/2000/svg";
    const svg = document.createElementNS(ns, "svg");
    document.body.appendChild(svg);
    return items.map((it) => {
      const t = document.createElementNS(ns, "text");
      t.setAttribute("style", `font-family:${it.family};font-size:${it.size}px;font-weight:${it.weight ?? 400};white-space:pre`);
      t.textContent = it.text;
      svg.appendChild(t);
      const bb = t.getBBox();
      const len = t.getComputedTextLength();
      t.remove();
      const d = document.createElement("span");
      d.setAttribute("style", `font-family:${it.family};font-size:${it.size}px;font-weight:${it.weight ?? 400};white-space:nowrap;line-height:1.5;display:inline-block`);
      d.textContent = it.text;
      document.body.appendChild(d);
      const r = d.getBoundingClientRect();
      d.remove();
      return { ...it, svgW: bb.width, svgH: bb.height, svgY: bb.y, len, htmlW: r.width, htmlH: r.height };
    });
  }, items);
  await b.close();
  fs.writeFileSync(outFile, JSON.stringify(res, null, 1));
}

const [mode, ...args] = process.argv.slice(2);
if (mode === "ref") await ref(...args);
else if (mode === "cmp") await cmp(...args);
else if (mode === "measure") await measure(...args);
else console.log("usage: ref <corpus> <out> | cmp <corpus> <ref> <ours> <report> | measure <in> <out>");

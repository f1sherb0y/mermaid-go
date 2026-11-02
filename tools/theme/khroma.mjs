// Runs khroma, as bundled into mermaid.min.js, over many inputs and writes what it returns,
// for the color package's tests.
//
//   node tools/theme/khroma.mjs [out.json.gz]   default color/testdata/khroma.json.gz
//
// khroma is not exposed by the bundle, so its sources are taken from the bundle's source map
// (the exact code mermaid ships) and loaded as ES modules from a temporary directory.
//
// Env: MERMAID_MAP (default ../../data/ref/pkg/mermaid-12.2.0/package/dist/mermaid.js.map).
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import zlib from "node:zlib";

const here = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"));
const MAP = process.env.MERMAID_MAP ?? path.join(here, "../../../../data/ref/pkg/mermaid-12.2.0/package/dist/mermaid.js.map");
const OUT = process.argv[2] ?? path.join(here, "../../color/testdata/khroma.json.gz");

const map = JSON.parse(fs.readFileSync(MAP, "utf8"));
const dir = fs.mkdtempSync(path.join(os.tmpdir(), "khroma-"));
fs.writeFileSync(path.join(dir, "package.json"), '{"type":"module"}');
map.sources.forEach((s, i) => {
  const m = s.match(/node_modules\/khroma\/(.*)$/);
  if (!m) return;
  fs.mkdirSync(path.dirname(path.join(dir, m[1])), { recursive: true });
  fs.writeFileSync(path.join(dir, m[1]), map.sourcesContent[i]);
});
const load = async (name) => (await import(pathToFileURL(path.join(dir, "dist/methods", name + ".js")).href)).default;
const k = {};
for (const n of ["adjust", "channel", "darken", "lighten", "invert", "is_dark", "is_light", "luminance", "mix", "rgba", "transparentize"]) {
  k[n] = await load(n);
}

const colors = [
  // hex
  "#fff", "#FFF", "#000", "#abc", "#abcd", "#ABCDEF", "#12345678", "#123456", "#ffffff00", "#fff8", "#ECECFF",
  "#fff4dd", "#ffffde", "#cde498", "#1f2020", "#eee", "#333", "#9370DB", "#f4f4f4", "#ff0000", "#00ff00",
  "#0000ff", "#7f7f7f", "#808080", "#010203", "#fefefe", "#80808080",
  "#ff", "#fffff", "#ggg", "fff", "#", "#1234567", "# fff", "#fff ",
  // rgb
  "rgb(10, 200, 30)", "rgb(10,200,30)", "rgba(10,200,30,0.5)", "rgba(10, 200, 30, .5)", "RGB(1,2,3)",
  "rgb(100% 0% 50%)", "rgb(10 20 30 / 50%)", "rgb(10 20 30 / 0.25)", "rgba(300, -20, 128, 2)",
  "rgb(12.5, 0.5, 254.5)", "rgb(1e2, 2e1, 3)", "rgba(232,232,232, 0.8)", "rgba(255, 255, 255, 0.25)",
  "rgb(250,250,250)", "rgb( 1 , 2 , 3 )", "rgb(1,2)", "rgb(1,2,3,4,5)", "rgb(a,b,c)", "rgba(0,0,0,0)",
  "rgba(1,2,3,+0.5)", "rgb(50%,25%,75%)", "rgba(10,20,30,50%)", "rgb(-1,-2,-3)", "rgb(.5,.5,.5)",
  // hsl
  "hsl(200, 50%, 40%)", "hsla(200, 50%, 40%, 0.5)", "hsl(200 50% 40%)", "hsl(200 50% 40% / 30%)",
  "hsl(120deg, 100%, 50%)", "hsl(100grad, 100%, 50%)", "hsl(1rad, 100%, 50%)", "hsl(0.5turn, 100%, 50%)",
  "hsl(-120, 100%, 50%)", "hsl(480, 100%, 50%)", "hsl(720, 100%, 50%)", "HSL(10, 20%, 30%)",
  "hsl(-79.0909090909, 100%, 93.3333333333%)", "hsl(0, 0%, 50%)", "hsl(0, 0%, 0%)", "hsl(0, 0%, 100%)",
  "hsl(240, 100%, 96.2745098039%)", "hsl(10, 150%, -5%)", "hsl(1e2, 10%, 20%)", "hsl(10, 1e1%, 20%)",
  "hsl(1GRAD, 50%, 50%)", "hsl(10, 50, 50)", "hsl(10, 50%)", "hsla(10, 20%, 30%, 1.5)", "hsl(.5, .5%, .5%)",
  "hsl(52.4, 86.7%, 63.1%)", "hsl(180, 25%, 75%)", "hsl(300,100%,25%)", "hsl(60, 100%, 50%)",
  // keywords
  "red", "Red", "RED", "white", "black", "transparent", "teal", "RebeccaPurple", "lightgrey", "grey", "navy",
  "green", "cyanaqua", "cyan", "notacolor", "", " red", "Khaki",
];

const amounts = [0, 1, 5, 10, 25, 40, 75, 100, -10, 2.5, 33.3, undefined];
const deltas = [
  {}, { h: 0 }, { h: -120 }, { h: 180, l: 5 }, { s: -40, l: -10 }, { s: -40, l: 10 }, { h: 30 }, { h: 210, l: 150 },
  { h: 64 }, { l: -10 }, { h: 60, l: -20 }, { r: 5, g: 5, b: 5 }, { r: -15, g: -15, b: -15 }, { a: -0.5 },
  { h: 180, s: -15, l: -8 }, { h: 180, s: -15, l: -32 }, { s: 10 }, { r: 300 }, { h: 360 }, { h: -360 }, { h: 10, r: 5 },
  { l: -100 }, { a: 0.5 }, { h: 0.1 },
];

const cases = [], seen = new Set();
const run = (fn, args) => {
  const key = JSON.stringify([fn, args]);
  if (seen.has(key)) return;
  seen.add(key);
  let res;
  try {
    const v = k[fn](...args);
    res = typeof v === "string" ? { out: v } : { num: String(v) }; // numbers and booleans as JavaScript prints them
  } catch (e) {
    res = { err: String(e.message ?? e) };
  }
  cases.push({ fn, args: args.map((a) => (a === undefined ? null : a)), ...res });
  return res.out;
};

const derived = new Set();
for (const c of colors) {
  for (const a of amounts) {
    for (const o of [run("darken", [c, a]), run("lighten", [c, a])]) if (o) derived.add(o);
  }
  for (const d of deltas) {
    const o = run("adjust", [c, d]);
    if (o) derived.add(o);
  }
  for (const o of [run("invert", [c])]) if (o) derived.add(o);
  for (const w of [0, 25, 50, 100]) run("invert", [c, w]);
  for (const ch of ["r", "g", "b", "h", "s", "l", "a"]) run("channel", [c, ch]);
  run("luminance", [c]);
  run("is_dark", [c]);
  run("is_light", [c]);
  for (const a of [0, 0.25, 0.5, 1, 1.5, -1]) run("rgba", [c, a]);
  for (const a of [0, 0.1, 0.5, 1]) run("transparentize", [c, a]);
}
// outputs fed back in (hsl() and rgb() strings with decimals, as themes chain them)
for (const c of derived) {
  for (const a of [10, 25]) run("darken", [c, a]), run("lighten", [c, a]);
  run("adjust", [c, { h: 30 }]), run("adjust", [c, { s: -40, l: -10 }]), run("adjust", [c, { r: 5, g: 5, b: 5 }]);
  run("invert", [c]);
  run("is_dark", [c]);
  for (const ch of ["r", "g", "b", "h", "s", "l"]) run("channel", [c, ch]);
}
const pairs = [["#fff", "#000"], ["red", "blue"], ["rgba(10,20,30,0.5)", "#abc"], ["hsl(10, 20%, 30%)", "#12345678"], ["transparent", "white"]];
for (const [a, b] of pairs) for (const w of [0, 10, 33, 50, 75, 100]) run("mix", [a, b, w]);
for (const t of [[0, 0, 0, 1], [255, 255, 255, 50], [102, 102, 255, 0.49], [12.5, 300, -4, 0.3], [1, 2, 3, 0], [10.25, 20, 30, 1], [0.1, 0.2, 0.3, 0.7]]) run("rgba", t);

fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, zlib.gzipSync(JSON.stringify(cases), { level: 9 }));
fs.rmSync(dir, { recursive: true, force: true });
console.log(cases.length, "cases ->", OUT);

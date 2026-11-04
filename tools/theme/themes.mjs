// Runs mermaid.js's themes in headless Chrome and writes the variables they produce, for the
// theme package's tests.
//
//   node tools/theme/themes.mjs [outdir]          default theme/testdata
//
// Writes <outdir>/themes.json.gz: for every theme and every override set, what
// getThemeVariables(overrides) returns (the theme registry is not exported, so the bundle is
// patched to expose it), and whether mermaid.initialize({theme, themeVariables}) followed by
// getConfig().themeVariables differs from it (initialize runs the theme a second time over
// its own output); and <outdir>/appearance.json: each diagram's default theme and look from
// mermaid's defaultConfig.
//
// Env: MERMAID_JS (default $MERMAID_DATA/ref/pkg/mermaid-12.2.0/package/dist/mermaid.min.js),
// CHROME (default the Windows install).
import { chromium } from "playwright-core";
import fs from "node:fs";
import path from "node:path";
import zlib from "node:zlib";

const here = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"));
// The reference data (mermaid.js, fonts, real diagrams): $MERMAID_DATA, else
// the data folder of a MeetBuddy checkout beside this repository.
const DATA = process.env.MERMAID_DATA ?? path.join(here, "../../../shitou/data");
const MERMAID_JS = process.env.MERMAID_JS ?? path.join(DATA, "ref/pkg/mermaid-12.2.0/package/dist/mermaid.min.js");
const CHROME = process.env.CHROME ?? "C:/Program Files/Google/Chrome/Application/chrome.exe";
const OUT = process.argv[2] ?? path.join(here, "../../theme/testdata");

// The registry is `<name>={base:{getThemeVariables:...},...}` in the minified bundle.
let js = fs.readFileSync(MERMAID_JS, "utf8");
const re = /([\w$]+)=\{base:\{getThemeVariables:/;
if (!re.test(js)) throw new Error("theme registry not found in " + MERMAID_JS);
js = js.replace(re, "$1=globalThis.__mermaidThemes={base:{getThemeVariables:");

// Override sets: none, base colors in every notation, dark mode, fonts, derived values,
// nested objects, nulls and a bad color.
const overrides = [
  undefined,
  {},
  { primaryColor: "#ff0000" },
  { primaryColor: "#abc" },
  { primaryColor: "#12345680" },
  { primaryColor: "rgb(10, 200, 30)" },
  { primaryColor: "rgba(10,200,30,0.5)" },
  { primaryColor: "hsl(200, 50%, 40%)" },
  { primaryColor: "hsla(20, 80%, 60%, 0.7)" },
  { primaryColor: "teal" },
  { primaryColor: "RebeccaPurple" },
  { primaryColor: "transparent" },
  { primaryColor: "#1f2020", darkMode: true },
  { darkMode: true },
  { darkMode: "false" },
  { darkMode: false, background: "#222" },
  { background: "black", darkMode: true, primaryColor: "#333" },
  { fontFamily: "Arial, sans-serif", fontSize: "20px" },
  { background: "#fafafa" },
  { primaryColor: "#ff6", secondaryColor: "#6f6", tertiaryColor: "#66f" },
  { lineColor: "red", textColor: "#123", primaryTextColor: "#fff" },
  { mainBkg: "#eef", nodeBorder: "#f00" },
  { nodeBorder: "#f00", useGradient: true },
  { noteBkgColor: "#ffe", noteTextColor: "blue", labelTextColor: "navy" },
  { cScale0: "#123456", cScale5: "rgb(1,2,3)", git0: "#abcdef", pie3: "orange" },
  { xyChart: { titleColor: "red", plotColorPalette: "#000,#fff" }, radar: { axisColor: "green" }, cynefin: { textColor: "#777" } },
  { primaryBorderColor: null, lineColor: null },
  { venn1: null, fillType0: "" },
  { radius: 10, strokeWidth: 2, useGradient: false, dropShadow: "none" },
  { primaryColor: "#9370DB", secondaryColor: "hsl(52.4, 86.7%, 63.1%)", background: "white", darkMode: true },
  { primaryColor: "notacolor" },
];

const browser = await chromium.launch({ executablePath: CHROME, headless: true });
const page = await browser.newPage();
await page.setContent("<html><body></body></html>");
await page.addScriptTag({ content: js });
const result = await page.evaluate((overrides) => {
  // JSON drops undefined (an unset variable, as the Go side has it) and turns NaN into null,
  // so non-finite numbers are tagged.
  const plain = (v) => JSON.parse(JSON.stringify(v, (_, x) => (typeof x === "number" && !Number.isFinite(x) ? { $number: String(x) } : x)));
  const run = (f) => {
    try {
      return { vars: plain(f()) };
    } catch (e) {
      return { error: String(e.message ?? e) };
    }
  };
  const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
  const sorted = (o) => o && Object.fromEntries(Object.keys(o).sort().map((k) => [k, o[k]]));
  const themes = globalThis.__mermaidThemes;
  const out = [];
  for (const name of Object.keys(themes)) {
    for (const ov of overrides) {
      const single = run(() => themes[name].getThemeVariables(ov === undefined ? undefined : structuredClone(ov)));
      const viaConfig = run(() => {
        mermaid.initialize({ theme: name, themeVariables: ov === undefined ? undefined : structuredClone(ov) });
        return mermaid.mermaidAPI.getConfig().themeVariables;
      });
      const c = { theme: name, overrides: ov ?? null, ...single };
      if (!same(sorted(single.vars), sorted(viaConfig.vars)) || single.error !== viaConfig.error) c.viaConfig = viaConfig;
      out.push(c);
    }
  }
  const cfg = mermaid.mermaidAPI.defaultConfig;
  const appearance = { "": { theme: cfg.theme, look: cfg.look } };
  for (const [k, v] of Object.entries(cfg)) {
    if (v && typeof v === "object" && ("theme" in v || "look" in v)) appearance[k] = { theme: v.theme, look: v.look };
  }
  return { out, appearance };
}, overrides);
await browser.close();

fs.mkdirSync(OUT, { recursive: true });
const lines = "[\n" + result.out.map((c) => JSON.stringify(c)).join(",\n") + "\n]\n";
fs.writeFileSync(path.join(OUT, "themes.json.gz"), zlib.gzipSync(lines, { level: 9 }));
fs.writeFileSync(path.join(OUT, "appearance.json"), JSON.stringify(result.appearance, null, 1) + "\n");
const differ = result.out.filter((c) => c.viaConfig);
console.log(result.out.length, "cases,", result.out.filter((c) => c.error).length, "errors,", differ.length, "differ via initialize ->", OUT);
for (const c of differ) console.log("  differs via initialize:", c.theme, JSON.stringify(c.overrides));

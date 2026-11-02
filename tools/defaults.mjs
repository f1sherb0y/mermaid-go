// Dumps mermaid's default config (mermaid.mermaidAPI.defaultConfig) to config/defaults.json.
import { chromium } from "playwright-core";
import fs from "node:fs";
import path from "node:path";
const here = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"));
const js = process.env.MERMAID_JS ?? path.join(here, "../../../data/ref/pkg/mermaid-12.2.0/package/dist/mermaid.min.js");
const b = await chromium.launch({ executablePath: process.env.CHROME ?? "C:/Program Files/Google/Chrome/Application/chrome.exe" });
const page = await b.newPage();
await page.setContent("<html></html>");
await page.addScriptTag({ path: js });
const cfg = await page.evaluate(() => {
  const c = JSON.parse(JSON.stringify(mermaid.mermaidAPI.defaultConfig)); // drops the font getter functions
  delete c.themeVariables; // the theme package computes these
  return c;
});
await b.close();
fs.writeFileSync(path.join(here, "../config/defaults.json"), JSON.stringify(cfg, null, 1) + "\n");

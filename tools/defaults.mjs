// Dumps mermaid's default config (mermaid.mermaidAPI.defaultConfig) to config/defaults.json.
import { chromium } from "playwright-core";
import fs from "node:fs";
import path from "node:path";
const here = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"));
// The reference data (mermaid.js, fonts, real diagrams): $MERMAID_DATA, else
// the data folder of a MeetBuddy checkout beside this repository.
const DATA = process.env.MERMAID_DATA ?? path.join(here, "../../shitou/data");
const js = process.env.MERMAID_JS ?? path.join(DATA, "ref/pkg/mermaid-12.2.0/package/dist/mermaid.min.js");
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

import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const cssPath = join(dirname(fileURLToPath(import.meta.url)), "consol-shell.css");
const css = readFileSync(cssPath, "utf8");

describe("consol-shell adaptivity contracts", () => {
  it("keeps the connections page on the default main max-width", () => {
    expect(css).toMatch(/\.nc-main\s*\{[^}]*max-width:\s*1200px/s);
    expect(css).not.toMatch(/\.nc-main:has\(\.nc-conn-page\)\s*\{[^}]*max-width:\s*1600px/s);
  });

  it("uses a 3-column telemetry grid on the connections page", () => {
    expect(css).toMatch(/\.nc-conn-telemetry\s*\{[^}]*grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/s);
  });

  it("stacks connection telemetry under 720px", () => {
    expect(css).toMatch(/@media\s*\(max-width:\s*720px\)\s*\{[^}]*\.nc-conn-telemetry[^}]*grid-template-columns:\s*1fr/s);
  });

  it("keeps the connections table from forcing horizontal page scroll", () => {
    expect(css).toMatch(/\.nc-conn-page\s+\.nc-conn-table-wrap\s*\{[^}]*overflow-x:\s*hidden/s);
  });

  it("disables shell motion for prefers-reduced-motion", () => {
    expect(css).toMatch(/@media\s*\(prefers-reduced-motion:\s*reduce\)/);
    expect(css).toMatch(/\.nc-shell\s+\*[^}]*animation:\s*none\s*!important/s);
  });

  it("keeps icon buttons paddingless for even topbar gaps", () => {
    expect(css).toMatch(/\.nc-icon-btn\s*\{[^}]*padding:\s*0;/s);
  });
});

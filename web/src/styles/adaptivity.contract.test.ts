import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const cssPath = join(dirname(fileURLToPath(import.meta.url)), "consol-shell.css");
const css = readFileSync(cssPath, "utf8");

describe("consol-shell adaptivity contracts", () => {
  it("stacks connection panels under 900px", () => {
    expect(css).toMatch(/@media\s*\(max-width:\s*900px\)\s*\{[^}]*\.nc-conn-panel[^}]*grid-template-columns:\s*1fr/s);
  });

  it("disables shell motion for prefers-reduced-motion", () => {
    expect(css).toMatch(/@media\s*\(prefers-reduced-motion:\s*reduce\)/);
    expect(css).toMatch(/\.nc-shell\s+\*[^}]*animation:\s*none\s*!important/s);
  });

  it("keeps icon buttons paddingless for even topbar gaps", () => {
    expect(css).toMatch(/\.nc-icon-btn\s*\{[^}]*padding:\s*0;/s);
  });
});

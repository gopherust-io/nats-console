/**
 * Headed probe: login → toggle theme 4×, record long frames during each toggle.
 * Usage: node scripts/theme-anim-probe.mjs
 */
import { chromium } from "playwright";

const BASE = process.env.THEME_PROBE_URL ?? "http://127.0.0.1:8080";
const USER = process.env.THEME_PROBE_USER ?? "admin";
const PASS = process.env.THEME_PROBE_PASS ?? "admin";

async function main() {
  const browser = await chromium.launch({
    headless: false,
    slowMo: 0,
    args: ["--disable-background-timer-throttling"],
  });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });

  page.on("console", (msg) => {
    if (msg.type() === "error") console.log("CONSOLE_ERROR:", msg.text());
  });
  page.on("pageerror", (err) => console.log("PAGE_ERROR:", err.message));

  await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });

  // Login form
  const userInput = page.locator('input[name="username"], input[type="text"], input[autocomplete="username"]').first();
  const passInput = page.locator('input[name="password"], input[type="password"]').first();
  await userInput.waitFor({ timeout: 15_000 });
  await userInput.fill(USER);
  await passInput.fill(PASS);
  await page.locator('button[type="submit"]').first().click();
  await page.waitForURL((url) => !url.pathname.includes("/login"), { timeout: 20_000 });
  await page.waitForSelector(".theme-toggle", { timeout: 20_000 });

  // Let eager warmup finish (preload + paint warm + VT warm).
  await page.waitForTimeout(900);

  const results = [];

  for (let i = 1; i <= 4; i++) {
    await page.evaluate(() => {
      window.__themeProbe = { longFrames: [], start: 0 };
      const probe = window.__themeProbe;
      probe.start = performance.now();
      probe.ro = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          // longtask or measure
          if (entry.duration >= 16) {
            probe.longFrames.push({
              name: entry.name || entry.entryType,
              start: entry.startTime,
              duration: Math.round(entry.duration * 10) / 10,
            });
          }
        }
      });
      try {
        probe.ro.observe({ type: "longtask", buffered: true });
      } catch {
        /* ignore */
      }
      // Also sample rAF gaps
      probe.rafGaps = [];
      let last = performance.now();
      let frames = 0;
      const tick = (now) => {
        const gap = now - last;
        last = now;
        if (gap > 24) probe.rafGaps.push({ at: Math.round(now - probe.start), gap: Math.round(gap) });
        frames += 1;
        if (performance.now() - probe.start < 1400) requestAnimationFrame(tick);
        else probe.frames = frames;
      };
      requestAnimationFrame(tick);
    });

    const beforeTheme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
    await page.locator(".theme-toggle").click();

    // Wait for theme attribute change + animation window
    await page.waitForFunction(
      (prev) => document.documentElement.getAttribute("data-theme") !== prev,
      beforeTheme,
      { timeout: 5000 },
    ).catch(() => {});
    await page.waitForTimeout(1100);

    const afterTheme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
    const metrics = await page.evaluate(() => {
      const p = window.__themeProbe || {};
      try {
        p.ro?.disconnect();
      } catch {
        /* ignore */
      }
      const gaps = (p.rafGaps || []).sort((a, b) => b.gap - a.gap);
      return {
        frames: p.frames ?? 0,
        worstGaps: gaps.slice(0, 5),
        longTasks: (p.longFrames || []).slice(0, 8),
        midGaps: gaps.filter((g) => g.at >= 200 && g.at <= 900),
        earlyGaps: gaps.filter((g) => g.at < 200),
      };
    });

    const shot = `/tmp/theme-toggle-${i}.png`;
    await page.screenshot({ path: shot, fullPage: false });

    results.push({
      n: i,
      from: beforeTheme,
      to: afterTheme,
      earlyGaps: metrics.earlyGaps,
      midGaps: metrics.midGaps,
      worstGaps: metrics.worstGaps,
      longTasks: metrics.longTasks,
      shot,
    });

    console.log(
      `\n#${i} ${beforeTheme} → ${afterTheme}` +
        `\n  early gaps (<200ms): ${JSON.stringify(metrics.earlyGaps)}` +
        `\n  mid gaps (200–900ms): ${JSON.stringify(metrics.midGaps)}` +
        `\n  worst gaps: ${JSON.stringify(metrics.worstGaps)}` +
        `\n  longtasks: ${JSON.stringify(metrics.longTasks)}` +
        `\n  screenshot: ${shot}`,
    );

    await page.waitForTimeout(400);
  }

  // Summary
  const hitchCount = results.filter((r) => r.midGaps.length > 0 || r.worstGaps.some((g) => g.gap >= 50)).length;
  console.log("\n=== SUMMARY ===");
  console.log(JSON.stringify({ url: BASE, hitchCount, results }, null, 2));

  await page.waitForTimeout(800);
  await browser.close();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});

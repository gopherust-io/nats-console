const FLY_MS = 360;
const REVEAL_MS = 560;
const FLYER_SIZE = 40;

/** Smooth ease-in-out — no hard ease-out stall at the end. */
const EASE_FLY = "cubic-bezier(0.33, 0.0, 0.2, 1)";
const EASE_REVEAL = "cubic-bezier(0.45, 0.0, 0.55, 1)";

let themeTransitionBusy = false;
let viewTransitionWarmed = false;

function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

function supportsViewTransition(): boolean {
  return (
    typeof document !== "undefined" &&
    typeof document.startViewTransition === "function"
  );
}

function wait(animation: Animation): Promise<void> {
  return animation.finished.then(
    () => undefined,
    () => undefined,
  );
}

function nextFrame(): Promise<void> {
  return new Promise((resolve) => requestAnimationFrame(() => resolve()));
}

/** Prime the View Transitions pipeline once so the first real toggle is cheaper. */
export async function warmViewTransitionPipeline(): Promise<void> {
  if (viewTransitionWarmed || prefersReducedMotion() || !supportsViewTransition()) return;
  viewTransitionWarmed = true;
  try {
    const transition = document.startViewTransition(() => {
      // no-op: builds snapshot/compositor path without a visual theme change
    });
    // Skip animation — warmup only. `finished` rejects when skipped.
    transition.skipTransition();
    await transition.finished.catch(() => undefined);
  } catch {
    viewTransitionWarmed = false;
  }
}

function createFlyer(sourceEl: HTMLElement): HTMLElement {
  const flyer = document.createElement("div");
  flyer.className = "theme-flyer";
  flyer.setAttribute("aria-hidden", "true");

  const svg = sourceEl.querySelector("svg")?.cloneNode(true);
  if (svg instanceof SVGElement) {
    svg.setAttribute("width", "22");
    svg.setAttribute("height", "22");
    flyer.appendChild(svg);
  }

  flyer.style.color = getComputedStyle(sourceEl).color;
  document.body.appendChild(flyer);
  return flyer;
}

function flyerTransform(x: number, y: number, scale: number): string {
  return `translate3d(${x}px, ${y}px, 0) translate3d(-50%, -50%, 0) scale(${scale})`;
}

function runReveal(root: HTMLElement, endRadius: number, startRadius: number) {
  return root.animate(
    {
      clipPath: [
        `circle(${startRadius}px at 50% 50%)`,
        `circle(${endRadius * 0.42}px at 50% 50%)`,
        `circle(${endRadius}px at 50% 50%)`,
      ],
      offset: [0, 0.48, 1],
    },
    {
      duration: REVEAL_MS,
      easing: EASE_REVEAL,
      fill: "both",
      pseudoElement: "::view-transition-new(root)",
    },
  );
}

function startFly(
  flyer: HTMLElement,
  startX: number,
  startY: number,
  cx: number,
  cy: number,
): Animation {
  flyer.style.transform = flyerTransform(startX, startY, 1);
  flyer.style.opacity = "0";
  return flyer.animate(
    [
      {
        transform: flyerTransform(startX, startY, 0.92),
        opacity: 0,
        offset: 0,
      },
      {
        transform: flyerTransform(startX, startY, 1),
        opacity: 1,
        offset: 0.12,
      },
      {
        transform: flyerTransform(
          startX + (cx - startX) * 0.55,
          startY + (cy - startY) * 0.55 - 18,
          1.45,
        ),
        opacity: 1,
        offset: 0.55,
      },
      {
        transform: flyerTransform(cx, cy, 2.05),
        opacity: 1,
        offset: 1,
      },
    ],
    {
      duration: FLY_MS,
      easing: EASE_FLY,
      fill: "forwards",
    },
  );
}

/**
 * Original choreography:
 * 1) icon flies to center (fully visible)
 * 2) then circular wipe expands from center with the new theme
 *
 * Warmup + deferred React state keep the mid-handoff from hitching.
 */
export async function runThemeViewTransition(
  update: () => void,
  sourceEl?: HTMLElement | null,
): Promise<boolean> {
  if (themeTransitionBusy) return false;
  if (prefersReducedMotion() || !supportsViewTransition()) {
    update();
    return true;
  }

  themeTransitionBusy = true;
  const root = document.documentElement;
  const endRadius =
    Math.hypot(window.innerWidth / 2, window.innerHeight / 2) * 1.12;

  let flyer: HTMLElement | null = null;
  const source = sourceEl ?? (document.querySelector(".theme-toggle") as HTMLElement | null);

  try {
    if (source) {
      const rect = source.getBoundingClientRect();
      const startX = rect.left + rect.width / 2;
      const startY = rect.top + rect.height / 2;
      const cx = window.innerWidth / 2;
      const cy = window.innerHeight / 2;

      flyer = createFlyer(source);
      source.classList.add("theme-toggle--launching");
      await nextFrame();

      // Phase 1 — fly alone (no VT overlay covering the icon).
      await wait(startFly(flyer, startX, startY, cx, cy));

      // Drop flyer before VT so it cannot appear in snapshots / fight the wipe.
      flyer.remove();
      flyer = null;

      // Phase 2 — circular wipe from center (only place live theme may change).
      root.dataset.themeTransition = "circle";
      const transition = document.startViewTransition(update);
      await transition.ready;
      runReveal(root, endRadius, FLYER_SIZE * 0.95);
      await transition.finished;
    } else {
      root.dataset.themeTransition = "circle";
      const transition = document.startViewTransition(update);
      await transition.ready;
      runReveal(root, endRadius, 0);
      await transition.finished;
    }
  } catch {
    // Transition aborted — theme already applied by update().
  } finally {
    flyer?.remove();
    source?.classList.remove("theme-toggle--launching");
    delete root.dataset.themeTransition;
    themeTransitionBusy = false;
  }
  return true;
}

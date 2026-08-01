import { useCallback, useEffect, useRef, useState } from "react";
import { useReducedMotion } from "motion/react";
import { useTranslation } from "react-i18next";

export type LiveArchScene = "deploy" | "layers";

type NodeDef = {
  id: string;
  label: string;
  sub: string;
  x: number;
  y: number;
  r: number;
};

type EdgeDef = { from: string; to: string; key: string };

type NodeStatus = "ok" | "load" | "down";

type Particle = {
  id: number;
  a: NodeDef;
  c: { x: number; y: number };
  b: NodeDef;
  t: number;
  speed: number;
  hot: boolean;
};

type Ripple = {
  id: number;
  x: number;
  y: number;
  r: number;
  max: number;
  life: number;
  born: number;
};

/** Hub-centered ring — same tree shape as the login promo thumbnail. */
const R_HUB = 52;
const R_NODE = 44;
const R_SIDE = 40;
const VIEW_W = 1600;
const VIEW_H = 900;
const HUB = { x: 800, y: 455 };
/** Distance from hub center to satellite centers (login hex, scaled up). */
const RING = 300;

/** Angle 0° = top, clockwise — matches login peer order. */
function ringAt(angleDeg: number, r = RING) {
  const rad = ((angleDeg - 90) * Math.PI) / 180;
  return {
    x: Math.round(HUB.x + r * Math.cos(rad)),
    y: Math.round(HUB.y + r * Math.sin(rad)),
  };
}

const SCENES: Record<
  LiveArchScene,
  { labelKey: string; nodes: NodeDef[]; edges: EdgeDef[] }
> = {
  deploy: {
    labelKey: "liveArch.sceneDeploy",
    nodes: [
      { id: "consol", label: "Consol API", sub: "api · fasthttp", x: HUB.x, y: HUB.y, r: R_HUB },
      { id: "gemini", label: "Assistant", sub: "llm · Gemini", ...ringAt(0), r: R_SIDE },
      { id: "pg", label: "PostgreSQL", sub: "db · store", ...ringAt(72), r: R_NODE },
      { id: "nats", label: "NATS", sub: "bus · JetStream", ...ringAt(144), r: R_NODE },
      { id: "mon", label: "Monitoring", sub: "ops · :8222", ...ringAt(216), r: R_NODE },
      { id: "browser", label: "Browser", sub: "web · React", ...ringAt(288), r: R_NODE },
    ],
    edges: [
      { from: "consol", to: "gemini", key: "ai" },
      { from: "consol", to: "pg", key: "pg" },
      { from: "consol", to: "nats", key: "nats" },
      { from: "consol", to: "mon", key: "mon" },
      { from: "browser", to: "consol", key: "ui" },
    ],
  },
  /** DDD packages around Application — outer edges only between neighbors (no chords). */
  layers: {
    labelKey: "liveArch.sceneLayers",
    nodes: [
      { id: "app", label: "Application", sub: "internal/app", x: HUB.x, y: HUB.y, r: R_HUB },
      { id: "boot", label: "Bootstrap", sub: "internal/bootstrap", ...ringAt(0), r: R_SIDE },
      { id: "adapters", label: "Adapters", sub: "internal/adapter", ...ringAt(72), r: R_NODE },
      { id: "ports", label: "Ports", sub: "internal/port", ...ringAt(144), r: R_NODE },
      { id: "domain", label: "Domain", sub: "internal/domain", ...ringAt(216), r: R_NODE },
      { id: "http", label: "API / Live", sub: "internal/api", ...ringAt(288), r: R_NODE },
    ],
    edges: [
      { from: "boot", to: "app", key: "wire" },
      { from: "app", to: "domain", key: "domain" },
      { from: "app", to: "ports", key: "ports" },
      { from: "ports", to: "adapters", key: "impl" },
      { from: "boot", to: "adapters", key: "wire2" },
      { from: "http", to: "app", key: "drive" },
    ],
  },
};

const SCENARIO_KEYS = [
  "liveArch.scenarioHealthy",
  "liveArch.scenarioLoad",
  "liveArch.scenarioFailure",
  "liveArch.scenarioRecovery",
] as const;

const MAX_PARTICLES = 48;

function curvePath(a: NodeDef, b: NodeDef) {
  const mx = (a.x + b.x) / 2;
  const my = (a.y + b.y) / 2;
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const len = Math.hypot(dx, dy) || 1;
  const bend = Math.min(80, len * 0.18);
  return {
    d: `M ${a.x} ${a.y} Q ${mx - (dy / len) * bend} ${my + (dx / len) * bend} ${b.x} ${b.y}`,
    c: { x: mx - (dy / len) * bend, y: my + (dx / len) * bend },
  };
}

function pointOnQuad(
  a: NodeDef,
  c: { x: number; y: number },
  b: NodeDef,
  t: number,
) {
  const u = 1 - t;
  return {
    x: u * u * a.x + 2 * u * t * c.x + t * t * b.x,
    y: u * u * a.y + 2 * u * t * c.y + t * t * b.y,
  };
}

type LiveArchitecturePaintingProps = {
  scene: LiveArchScene;
};

export default function LiveArchitecturePainting({ scene }: LiveArchitecturePaintingProps) {
  const { t } = useTranslation();
  const reduceMotion = Boolean(useReducedMotion());
  const rootRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const nodeRefs = useRef<Map<string, SVGGElement>>(new Map());
  const [paused, setPaused] = useState(false);
  const [scenarioIndex, setScenarioIndex] = useState(0);
  const [nodeStatus, setNodeStatus] = useState<Record<string, NodeStatus>>({});
  const [edgeLoad, setEdgeLoad] = useState<Record<string, number>>({});

  const sim = useRef({
    scenarioElapsed: 0,
    spawnAcc: 0,
    breath: 0,
    lastTs: 0,
    particleId: 0,
    rippleId: 0,
    nodeState: {} as Record<string, { status: NodeStatus; load: number }>,
    edgeLoad: {} as Record<string, number>,
    particles: [] as Particle[],
    ripples: [] as Ripple[],
    paused: false,
    reduceMotion: false,
    hidden: false,
    scene: scene as LiveArchScene,
    scenarioIndex: 0,
  });

  sim.current.paused = paused;
  sim.current.reduceMotion = reduceMotion;
  sim.current.scene = scene;

  const resetHealthy = useCallback((s: LiveArchScene) => {
    const def = SCENES[s];
    const ns: Record<string, { status: NodeStatus; load: number }> = {};
    const el: Record<string, number> = {};
    def.nodes.forEach((n) => {
      ns[n.id] = { status: "ok", load: 1 };
    });
    def.edges.forEach((e) => {
      el[e.key] = 1;
    });
    sim.current.nodeState = ns;
    sim.current.edgeLoad = el;
  }, []);

  const spawnRipple = useCallback((nodeId: string, s: LiveArchScene) => {
    const n = SCENES[s].nodes.find((x) => x.id === nodeId);
    if (!n) return;
    const now = performance.now();
    sim.current.ripples.push({
      id: ++sim.current.rippleId,
      x: n.x,
      y: n.y,
      r: n.r,
      max: n.r + 220,
      life: 1,
      born: now,
    });
    SCENES[s].edges.forEach((e) => {
      if (e.from !== nodeId && e.to !== nodeId) return;
      const otherId = e.from === nodeId ? e.to : e.from;
      const other = SCENES[s].nodes.find((x) => x.id === otherId);
      if (!other) return;
      window.setTimeout(() => {
        if (sim.current.scene !== s) return;
        sim.current.ripples.push({
          id: ++sim.current.rippleId,
          x: other.x,
          y: other.y,
          r: other.r * 0.8,
          max: other.r + 120,
          life: 0.65,
          born: performance.now(),
        });
      }, 280);
    });
  }, []);

  const applyScenario = useCallback(
    (s: LiveArchScene, index: number) => {
      resetHealthy(s);
      if (index === 1) {
        const hot = s === "deploy" ? ["nats", "consol", "browser"] : ["http", "app", "adapters"];
        hot.forEach((id) => {
          if (sim.current.nodeState[id]) {
            sim.current.nodeState[id] = { status: "load", load: 1.8 };
          }
        });
        Object.keys(sim.current.edgeLoad).forEach((k) => {
          sim.current.edgeLoad[k] =
            k === "nats" || k === "ui" || k === "drive" || k === "impl" ? 2.4 : 1.4;
        });
      } else if (index === 2) {
        const downId = s === "deploy" ? "pg" : "adapters";
        sim.current.nodeState[downId] = { status: "down", load: 0 };
        Object.keys(sim.current.edgeLoad).forEach((k) => {
          sim.current.edgeLoad[k] =
            k === "pg" || k === "impl" || k === "wire2" ? 0.15 : 0.55;
        });
        spawnRipple(downId, s);
      } else if (index === 3) {
        Object.keys(sim.current.edgeLoad).forEach((k) => {
          sim.current.edgeLoad[k] = 0.7;
        });
      }
      setNodeStatus(
        Object.fromEntries(
          Object.entries(sim.current.nodeState).map(([id, v]) => [id, v.status]),
        ),
      );
      setEdgeLoad({ ...sim.current.edgeLoad });
    },
    [resetHealthy, spawnRipple],
  );

  useEffect(() => {
    sim.current.scenarioIndex = 0;
    sim.current.scenarioElapsed = 0;
    sim.current.particles = [];
    sim.current.ripples = [];
    setScenarioIndex(0);
    applyScenario(scene, 0);
  }, [scene, applyScenario]);

  useEffect(() => {
    let raf = 0;
    const canvas = canvasRef.current;
    const ctx2d = canvas?.getContext("2d");
    const paintColors = {
      pulseRgb: "45, 212, 191",
      pulseHotRgb: "251, 146, 60",
      rippleRgb: "94, 234, 212",
    };

    const refreshPaintColors = () => {
      if (!rootRef.current) return;
      const css = getComputedStyle(rootRef.current);
      paintColors.pulseRgb = css.getPropertyValue("--la-pulse-rgb").trim() || paintColors.pulseRgb;
      paintColors.pulseHotRgb =
        css.getPropertyValue("--la-pulse-hot-rgb").trim() || paintColors.pulseHotRgb;
      paintColors.rippleRgb = css.getPropertyValue("--la-ripple-rgb").trim() || paintColors.rippleRgb;
    };

    const syncCanvasSize = () => {
      if (!canvas || !rootRef.current) return;
      const rect = rootRef.current.getBoundingClientRect();
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      const w = Math.max(1, Math.floor(rect.width * dpr));
      const h = Math.max(1, Math.floor(rect.height * dpr));
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w;
        canvas.height = h;
      }
      canvas.style.width = `${rect.width}px`;
      canvas.style.height = `${rect.height}px`;
    };

    const paintOverlay = (ts: number) => {
      if (!ctx2d || !canvas || !rootRef.current) return;
      const rect = rootRef.current.getBoundingClientRect();
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      const scale = Math.min(rect.width / VIEW_W, rect.height / VIEW_H);
      const ox = (rect.width - VIEW_W * scale) / 2;
      const oy = (rect.height - VIEW_H * scale) / 2;
      ctx2d.setTransform(1, 0, 0, 1, 0, 0);
      ctx2d.clearRect(0, 0, canvas.width, canvas.height);
      ctx2d.setTransform(dpr * scale, 0, 0, dpr * scale, dpr * ox, dpr * oy);

      const st = sim.current;
      for (const r of st.ripples) {
        const progress = Math.min(1, (ts - r.born) / 1400);
        const radius = r.r + (r.max - r.r) * progress;
        ctx2d.beginPath();
        ctx2d.arc(r.x, r.y, radius, 0, Math.PI * 2);
        ctx2d.strokeStyle = `rgba(${paintColors.rippleRgb}, ${r.life * (1 - progress) * 0.55})`;
        ctx2d.lineWidth = 2;
        ctx2d.stroke();
      }

      for (const p of st.particles) {
        const pt = pointOnQuad(p.a, p.c, p.b, p.t);
        const alpha = 0.35 + 0.65 * Math.sin(p.t * Math.PI);
        ctx2d.beginPath();
        ctx2d.arc(pt.x, pt.y, p.hot ? 3.2 : 2.4, 0, Math.PI * 2);
        ctx2d.fillStyle = p.hot
          ? `rgba(${paintColors.pulseHotRgb}, ${alpha})`
          : `rgba(${paintColors.pulseRgb}, ${alpha})`;
        ctx2d.fill();
      }
    };

    const tick = (ts: number) => {
      const st = sim.current;
      if (!st.lastTs) st.lastTs = ts;
      const dt = Math.min(48, ts - st.lastTs);
      st.lastTs = ts;

      const def = SCENES[st.scene];
      const durations = [9000, 8000, 10000, 7000];

      if (!st.paused) {
        st.scenarioElapsed += dt;
        if (st.scenarioElapsed >= durations[st.scenarioIndex]) {
          st.scenarioElapsed = 0;
          st.scenarioIndex = (st.scenarioIndex + 1) % durations.length;
          setScenarioIndex(st.scenarioIndex);
          applyScenario(st.scene, st.scenarioIndex);
        }

        st.breath += dt * 0.0012;
        st.spawnAcc += dt;
        const spawnEvery = st.reduceMotion ? 1e9 : st.scenarioIndex === 1 ? 90 : 140;
        while (st.spawnAcc >= spawnEvery && st.particles.length < MAX_PARTICLES) {
          st.spawnAcc -= spawnEvery;
          const weighted: EdgeDef[] = [];
          def.edges.forEach((e) => {
            const w = st.edgeLoad[e.key] || 0;
            if (w < 0.2) return;
            const copies = Math.max(1, Math.round(w * 3));
            for (let i = 0; i < copies; i++) weighted.push(e);
          });
          if (!weighted.length) continue;
          const e = weighted[(Math.random() * weighted.length) | 0];
          const a = def.nodes.find((n) => n.id === e.from)!;
          const b = def.nodes.find((n) => n.id === e.to)!;
          const { c } = curvePath(a, b);
          st.particles.push({
            id: ++st.particleId,
            a,
            c,
            b,
            t: 0,
            speed: (0.22 + Math.random() * 0.18) * (st.edgeLoad[e.key] || 1),
            hot: (st.edgeLoad[e.key] || 0) > 1.5,
          });
        }
      }

      if (!st.reduceMotion) {
        def.nodes.forEach((n, i) => {
          const ns = st.nodeState[n.id] || { status: "ok" as const, load: 1 };
          const amp = ns.status === "down" ? 0.02 : 0.035 + (ns.load - 1) * 0.02;
          const scale = 1 + Math.sin(st.breath * 2.2 + i * 0.9) * amp;
          const el = nodeRefs.current.get(n.id);
          if (el) {
            el.setAttribute("transform", `translate(${n.x},${n.y}) scale(${scale})`);
          }
        });
      } else {
        def.nodes.forEach((n) => {
          const el = nodeRefs.current.get(n.id);
          if (el) {
            el.setAttribute("transform", `translate(${n.x},${n.y}) scale(1)`);
          }
        });
      }

      for (let i = st.particles.length - 1; i >= 0; i--) {
        const p = st.particles[i];
        if (!st.paused) p.t += (dt / 1000) * p.speed;
        if (p.t >= 1) st.particles.splice(i, 1);
      }

      for (let i = st.ripples.length - 1; i >= 0; i--) {
        if ((ts - st.ripples[i].born) / 1400 >= 1) st.ripples.splice(i, 1);
      }

      paintOverlay(ts);
      raf = requestAnimationFrame(tick);
    };

    const startLoop = () => {
      if (raf) return;
      sim.current.lastTs = 0;
      raf = requestAnimationFrame(tick);
    };

    const stopLoop = () => {
      if (!raf) return;
      cancelAnimationFrame(raf);
      raf = 0;
      sim.current.lastTs = 0;
    };

    syncCanvasSize();
    refreshPaintColors();
    const onResize = () => syncCanvasSize();
    const onVisibility = () => {
      const hidden = document.visibilityState === "hidden";
      sim.current.hidden = hidden;
      if (hidden) stopLoop();
      else startLoop();
    };
    const themeObserver = new MutationObserver(refreshPaintColors);
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    sim.current.hidden = typeof document !== "undefined" && document.visibilityState === "hidden";
    window.addEventListener("resize", onResize);
    document.addEventListener("visibilitychange", onVisibility);
    if (!sim.current.hidden) startLoop();
    return () => {
      stopLoop();
      themeObserver.disconnect();
      window.removeEventListener("resize", onResize);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [applyScenario]);

  useEffect(() => {
    const onKey = (ev: KeyboardEvent) => {
      if (ev.code === "Space" && (ev.target as HTMLElement)?.tagName !== "INPUT") {
        ev.preventDefault();
        setPaused((p) => !p);
      } else if (ev.key === "f" || ev.key === "F") {
        const el = rootRef.current;
        if (!el) return;
        if (!document.fullscreenElement) el.requestFullscreen?.().catch(() => {});
        else document.exitFullscreen?.();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const def = SCENES[scene];
  const scenarioLabel = t(SCENARIO_KEYS[scenarioIndex]);
  const scenarioText = paused ? t("liveArch.paused", { name: scenarioLabel }) : scenarioLabel;
  const useGlow = !reduceMotion;

  return (
    <div ref={rootRef} className="live-arch" role="img" aria-label={t("liveArch.aria")}>
      <div className="live-arch__chrome live-arch__top">
        <div className="live-arch__brand">
          {t("common.brand")}
          <span>{t("liveArch.brandSubtitle")}</span>
        </div>
        <div className="live-arch__meta">
          <div>
            {t("liveArch.scene")}: <strong>{t(def.labelKey)}</strong>
          </div>
          <div>
            {t("liveArch.scenario")}: <strong>{scenarioText}</strong>
          </div>
        </div>
      </div>

      <div className="live-arch__viewport">
        <svg className="live-arch__stage" viewBox={`0 0 ${VIEW_W} ${VIEW_H}`} preserveAspectRatio="xMidYMid meet">
          <defs>
            <radialGradient id="liveArchCoreGrad" cx="40%" cy="35%" r="65%">
              <stop offset="0%" stopColor="var(--la-core-0)" />
              <stop offset="45%" stopColor="var(--la-core-1)" />
              <stop offset="100%" stopColor="var(--la-core-2)" />
            </radialGradient>
            {useGlow ? (
              <filter id="liveArchSoftGlow" x="-50%" y="-50%" width="200%" height="200%">
                <feGaussianBlur stdDeviation="6" result="b" />
                <feMerge>
                  <feMergeNode in="b" />
                  <feMergeNode in="SourceGraphic" />
                </feMerge>
              </filter>
            ) : null}
          </defs>

          {def.edges.map((e) => {
            const a = def.nodes.find((n) => n.id === e.from)!;
            const b = def.nodes.find((n) => n.id === e.to)!;
            const dim = (edgeLoad[e.key] ?? 1) < 0.35;
            return (
              <path
                key={e.key}
                d={curvePath(a, b).d}
                className={`live-arch__edge${dim ? " live-arch__edge--dim" : ""}`}
              />
            );
          })}

          {def.nodes.map((n) => {
            const status = nodeStatus[n.id] ?? "ok";
            return (
              <g
                key={n.id}
                ref={(el) => {
                  if (el) nodeRefs.current.set(n.id, el);
                  else nodeRefs.current.delete(n.id);
                }}
                className={`live-arch__organism live-arch__organism--${status}`}
                transform={`translate(${n.x},${n.y}) scale(1)`}
              >
                <circle className="live-arch__glow" r={n.r * 1.6} />
                <circle className="live-arch__halo" r={n.r * 1.18} />
                <circle className="live-arch__membrane" r={n.r} />
                <circle
                  className="live-arch__core"
                  r={n.r * 0.58}
                  filter={useGlow ? "url(#liveArchSoftGlow)" : undefined}
                />
                <text className="live-arch__label" y={n.r + 24}>
                  {n.label}
                </text>
                <text className="live-arch__sub" y={n.r + 40}>
                  {n.sub}
                </text>
              </g>
            );
          })}
        </svg>
        <canvas ref={canvasRef} className="live-arch__fx" aria-hidden="true" />
      </div>

      <div className="live-arch__chrome live-arch__bottom">
        <div className="live-arch__legend" aria-hidden>
          <span>
            <i className="ok" />
            {t("liveArch.legendHealthy")}
          </span>
          <span>
            <i className="load" />
            {t("liveArch.legendLoad")}
          </span>
          <span>
            <i className="down" />
            {t("liveArch.legendFailure")}
          </span>
        </div>
        <div className="live-arch__hints">
          <kbd>1</kbd>/<kbd>2</kbd> {t("liveArch.hintScene")} · <kbd>Space</kbd>{" "}
          {t("liveArch.hintPause")} · <kbd>F</kbd> {t("liveArch.hintFullscreen")}
        </div>
      </div>
    </div>
  );
}

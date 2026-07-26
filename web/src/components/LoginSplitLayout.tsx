import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

type Props = {
  children: ReactNode;
};

type Pt = { x: number; y: number };

const HUB: Pt = { x: 160, y: 148 };

/** Six cluster peers in a hexagon around the hub (nudged down for copy clearance). */
const PEERS: Pt[] = [
  { x: 160, y: 60 },
  { x: 236, y: 104 },
  { x: 236, y: 192 },
  { x: 160, y: 236 },
  { x: 84, y: 192 },
  { x: 84, y: 104 },
];

/** Outer diamonds: labeled clients plus two unlabeled edge nodes. */
const LEAVES: Array<Pt & { label?: string }> = [
  { x: 36, y: 48, label: "cli" },
  { x: 284, y: 44, label: "svc" },
  { x: 300, y: 148 },
  { x: 284, y: 252, label: "leaf" },
  { x: 36, y: 252 },
  { x: 20, y: 148, label: "app" },
];

/** Subject whispers — clear of the hub halo. */
const SUBJECTS: Array<{ x: number; y: number; text: string }> = [
  { x: 112, y: 92, text: "orders.*" },
  { x: 220, y: 92, text: "metrics.>" },
  { x: 160, y: 212, text: "js.API" },
];

function quad(a: Pt, b: Pt, bend = 0.22): string {
  const mx = (a.x + b.x) / 2;
  const my = (a.y + b.y) / 2;
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const cx = mx - dy * bend;
  const cy = my + dx * bend;
  return `M ${a.x.toFixed(1)} ${a.y.toFixed(1)} Q ${cx.toFixed(1)} ${cy.toFixed(1)} ${b.x.toFixed(1)} ${b.y.toFixed(1)}`;
}

const PEER_RING = PEERS.map((p, i) => {
  const next = PEERS[(i + 1) % PEERS.length];
  return quad(p, next, 0.12);
});

const HUB_TO_PEER = PEERS.map((p) => quad(HUB, p, 0.08));

const LEAF_ROUTES = LEAVES.map((leaf, i) => {
  const peer = PEERS[i % PEERS.length];
  return {
    path: quad(leaf, peer, 0.28),
    leaf,
    peer,
  };
});

/** Five calm packet lanes: 2 leaf, 1 spoke, 2 ring. */
const PACKETS: Array<{ path: string; dur: number; delay: number }> = [
  { path: LEAF_ROUTES[0].path, dur: 6.8, delay: 0 },
  { path: LEAF_ROUTES[5].path, dur: 7.2, delay: 2.4 },
  { path: HUB_TO_PEER[2], dur: 5.6, delay: 1.2 },
  { path: PEER_RING[0], dur: 8.8, delay: 0.8 },
  { path: PEER_RING[3], dur: 8.2, delay: 3.6 },
];

function EdgeLayer({
  paths,
  kind,
  stroke,
}: {
  paths: string[];
  kind: "ring" | "spoke" | "leaf";
  stroke?: string;
}) {
  return (
    <>
      {paths.map((d, i) => (
        <path
          key={`${kind}-glow-${i}`}
          className={`login-promo__edge-glow login-promo__edge-glow--${kind}`}
          d={d}
        />
      ))}
      {paths.map((d, i) => (
        <path
          key={`${kind}-${i}`}
          className={`login-promo__edge login-promo__edge--${kind}`}
          d={d}
          stroke={stroke}
          style={{ animationDelay: `${(i % 3) * 0.9}s` }}
        />
      ))}
    </>
  );
}

/** Quiet fabric, one bright moment — hexagon NATS promo mesh. */
function NatsFabric() {
  return (
    <div className="login-promo__stage">
      <svg className="login-promo__mesh" viewBox="0 0 320 290" aria-hidden>
        <defs>
          <radialGradient id="lp-hub-glow" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor="rgba(232, 255, 245, 0.82)" />
            <stop offset="38%" stopColor="rgba(125, 255, 179, 0.36)" />
            <stop offset="100%" stopColor="rgba(125, 255, 179, 0)" />
          </radialGradient>
          <radialGradient id="lp-node-glow" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor="rgba(125, 255, 179, 0.48)" />
            <stop offset="100%" stopColor="rgba(125, 255, 179, 0)" />
          </radialGradient>
          <linearGradient id="lp-route" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="rgba(232,255,245,0.12)" />
            <stop offset="50%" stopColor="rgba(232,255,245,0.78)" />
            <stop offset="100%" stopColor="rgba(232,255,245,0.12)" />
          </linearGradient>
          <linearGradient id="lp-leaf-route" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="rgba(125,255,179,0.12)" />
            <stop offset="50%" stopColor="rgba(125,255,179,0.72)" />
            <stop offset="100%" stopColor="rgba(125,255,179,0.12)" />
          </linearGradient>
          <filter id="lp-soft" x="-80%" y="-80%" width="260%" height="260%">
            <feGaussianBlur stdDeviation="1.8" result="b" />
            <feMerge>
              <feMergeNode in="b" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
          <filter id="lp-trail" x="-120%" y="-120%" width="340%" height="340%">
            <feGaussianBlur stdDeviation="2.8" result="b" />
            <feMerge>
              <feMergeNode in="b" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        <EdgeLayer paths={PEER_RING} kind="ring" stroke="url(#lp-route)" />
        <EdgeLayer paths={HUB_TO_PEER} kind="spoke" />
        <EdgeLayer paths={LEAF_ROUTES.map((r) => r.path)} kind="leaf" stroke="url(#lp-leaf-route)" />

        {SUBJECTS.map((s) => (
          <text key={s.text} className="login-promo__subject" x={s.x} y={s.y} textAnchor="middle">
            {s.text}
          </text>
        ))}

        {PACKETS.map((p, i) => (
          <g key={`pkt-${i}`} className="login-promo__packet-group">
            <circle className="login-promo__packet login-promo__packet--trail" r="7" opacity="0.22" filter="url(#lp-trail)">
              <animateMotion dur={`${p.dur}s`} begin={`${p.delay}s`} repeatCount="indefinite" path={p.path} />
              <animate
                attributeName="opacity"
                values="0;0.28;0.16;0"
                keyTimes="0;0.12;0.78;1"
                dur={`${p.dur}s`}
                begin={`${p.delay}s`}
                repeatCount="indefinite"
              />
            </circle>
            <circle className="login-promo__packet" r="2.2" filter="url(#lp-soft)">
              <animateMotion dur={`${p.dur}s`} begin={`${p.delay}s`} repeatCount="indefinite" path={p.path} />
              <animate
                attributeName="opacity"
                values="0;1;1;0"
                keyTimes="0;0.06;0.9;1"
                dur={`${p.dur}s`}
                begin={`${p.delay}s`}
                repeatCount="indefinite"
              />
            </circle>
          </g>
        ))}

        {LEAVES.map((n, i) => {
          const labeled = Boolean(n.label);
          const half = labeled ? 5 : 3.6;
          const halo = labeled ? 12 : 9;
          return (
            <g key={`leaf-${i}`} className="login-promo__node login-promo__node--leaf">
              <circle cx={n.x} cy={n.y} r={halo} fill="url(#lp-node-glow)" className="login-promo__node-halo" />
              <rect
                x={n.x - half}
                y={n.y - half}
                width={half * 2}
                height={half * 2}
                rx={labeled ? 2.2 : 1.6}
                className="login-promo__node-core login-promo__node-core--leaf"
                transform={`rotate(45 ${n.x} ${n.y})`}
              />
              {n.label && (
                <text className="login-promo__node-label" x={n.x} y={n.y + 20} textAnchor="middle">
                  {n.label}
                </text>
              )}
            </g>
          );
        })}

        {PEERS.map((n, i) => (
          <g
            key={`peer-${i}`}
            className="login-promo__node login-promo__node--peer login-promo__node--on"
            style={{ ["--lp-delay" as string]: `${i * 0.4}s` }}
          >
            <circle cx={n.x} cy={n.y} r="14" fill="url(#lp-node-glow)" className="login-promo__node-halo" />
            <circle cx={n.x} cy={n.y} r="9.5" className="login-promo__peer-ring" />
            <circle cx={n.x} cy={n.y} r="5" className="login-promo__node-core" />
          </g>
        ))}

        <g className="login-promo__hub">
          <circle cx={HUB.x} cy={HUB.y} r="54" fill="url(#lp-hub-glow)" className="login-promo__hub-glow" />
          <circle cx={HUB.x} cy={HUB.y} r="32" className="login-promo__hub-ring" />
          <circle cx={HUB.x} cy={HUB.y} r="32" className="login-promo__hub-ring login-promo__hub-ring--late" />
          <circle cx={HUB.x} cy={HUB.y} r="19" className="login-promo__hub-core" />
          <text
            className="login-promo__hub-label"
            x={HUB.x}
            y={HUB.y}
            textAnchor="middle"
            dominantBaseline="central"
          >
            NATS
          </text>
        </g>
      </svg>
    </div>
  );
}

export default function LoginSplitLayout({ children }: Props) {
  const { t } = useTranslation();

  return (
    <div className="login-page login-page--split">
      <header className="login-topbar">
        <Link to="/login" className="login-topbar__brand brand">
          <span className="brand__icon">
            <span className="brand__mark">NC</span>
          </span>
          <span className="brand__name">{t("common.brand")}</span>
        </Link>
      </header>

      <div className="login-split-body">
        <main className="login-pane">{children}</main>

        <aside className="login-promo">
          <svg className="login-promo__hive" aria-hidden>
            <defs>
              <pattern id="lp-hive" width="48" height="84" patternUnits="userSpaceOnUse">
                <path
                  d="M24 2 L46 14.5 L46 39.5 M24 2 L2 14.5 L2 39.5 M2 39.5 L24 52 L46 39.5 M24 52 L24 77"
                  fill="none"
                  stroke="rgba(232,255,245,0.12)"
                  strokeWidth="1"
                  strokeLinejoin="round"
                  strokeLinecap="round"
                />
              </pattern>
              <radialGradient id="lp-hive-fade" cx="48%" cy="58%" r="55%">
                <stop offset="0%" stopColor="white" stopOpacity="0.15" />
                <stop offset="45%" stopColor="white" stopOpacity="0.55" />
                <stop offset="100%" stopColor="white" stopOpacity="1" />
              </radialGradient>
              <mask id="lp-hive-mask">
                <rect width="100%" height="100%" fill="url(#lp-hive-fade)" />
              </mask>
            </defs>
            <rect width="100%" height="100%" fill="url(#lp-hive)" mask="url(#lp-hive-mask)" />
          </svg>
          <div className="login-promo__aurora" aria-hidden />
          <div className="login-promo__inner">
            <h2 className="login-promo__title">{t("auth.promo.headline")}</h2>
            <p className="login-promo__body">{t("auth.promo.body")}</p>
            <NatsFabric />
          </div>
        </aside>
      </div>
    </div>
  );
}

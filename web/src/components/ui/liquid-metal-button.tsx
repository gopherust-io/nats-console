import {
  emptyPixel,
  liquidMetalFragmentShader,
  LiquidMetalShapes,
  ShaderFitOptions,
  ShaderMount,
} from "@paper-design/shaders";
import { Sparkles } from "lucide-react";
import type { MouseEvent, ReactNode } from "react";
import { useEffect, useRef, useState } from "react";

interface LiquidMetalButtonProps {
  label?: string;
  onClick?: () => void;
  viewMode?: "text" | "icon";
  type?: "button" | "submit";
  disabled?: boolean;
  className?: string;
  /** Full-width form control (default) or fixed circular FAB. */
  variant?: "block" | "fab";
  title?: string;
  ariaExpanded?: boolean;
  ariaLabel?: string;
  /** Replaces the default Sparkles glyph when viewMode is icon. */
  icon?: ReactNode;
}

type Ripple = { x: number; y: number; id: number };

const STYLE_ID = "liquid-metal-btn-styles-v2";

function ensureStyles() {
  if (typeof document === "undefined" || document.getElementById(STYLE_ID)) return;
  const style = document.createElement("style");
  style.id = STYLE_ID;
  style.textContent = `
    .liquid-metal-btn {
      position: relative;
      display: block;
      width: 100%;
    }
    .liquid-metal-btn--fab {
      display: inline-block;
      width: fit-content;
      /* Do not set position here — .assistant-fab owns fixed placement. */
    }
    .liquid-metal-btn__shader {
      /* Paper defaults canvas to z-index:-1; with overflow:hidden that can
         clip the WebGL chrome rim away (esp. after hard reload / Safari). */
      overflow: hidden;
    }
    .liquid-metal-btn__shader canvas {
      width: 100% !important;
      height: 100% !important;
      display: block !important;
      position: absolute !important;
      inset: 0 !important;
      z-index: 0 !important;
      border-radius: inherit !important;
    }
    @keyframes liquid-metal-ripple {
      0% {
        transform: translate(-50%, -50%) scale(0);
        opacity: 0.6;
      }
      100% {
        transform: translate(-50%, -50%) scale(4);
        opacity: 0;
      }
    }
  `;
  document.head.appendChild(style);
}

export function LiquidMetalButton({
  label = "Get Started",
  onClick,
  viewMode = "text",
  type = "button",
  disabled = false,
  className = "",
  variant = "block",
  title,
  ariaExpanded,
  ariaLabel,
  icon,
}: LiquidMetalButtonProps) {
  const isFab = variant === "fab";
  const isIcon = viewMode === "icon" || isFab;
  const fabSize = 46; // prompt icon-mode size
  const [isHovered, setIsHovered] = useState(false);
  const [isPressed, setIsPressed] = useState(false);
  const [ripples, setRipples] = useState<Ripple[]>([]);
  const [width, setWidth] = useState(isFab || viewMode === "icon" ? fabSize : 142);

  const rootRef = useRef<HTMLDivElement>(null);
  const shaderRef = useRef<HTMLDivElement>(null);
  const shaderMount = useRef<ShaderMount | null>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const rippleId = useRef(0);
  const hoveredRef = useRef(false);

  const height = isFab || isIcon ? fabSize : 46;
  // Prompt icon mode: 46 outer, 2px margin → 42 inner (chrome rim).
  const inset = isFab || isIcon ? 2 : 2;
  const innerWidth = Math.max(1, width - inset * 2);
  const innerHeight = height - inset * 2;
  // FAB must not dodge; Sign In may keep a light press squash.
  const pressMotion =
    !isFab && isPressed ? "translateY(1px) scale(0.98)" : "translateY(0) scale(1)";

  useEffect(() => {
    ensureStyles();
  }, []);

  useEffect(() => {
    if (isFab || viewMode === "icon") {
      setWidth(fabSize);
      return;
    }

    const root = rootRef.current;
    if (!root || typeof ResizeObserver === "undefined") return;

    const update = () => {
      const next = Math.max(142, Math.floor(root.clientWidth) || 142);
      setWidth(next);
    };

    update();
    const ro = new ResizeObserver(update);
    ro.observe(root);
    return () => ro.disconnect();
  }, [viewMode, isFab]);

  useEffect(() => {
    const el = shaderRef.current;
    if (!el) return;

    const reducedMotion =
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const baseSpeed = reducedMotion ? 0 : 0.6;
    const iconLike = isFab || viewMode === "icon";
    let cancelled = false;
    let raf = 0;

    const mount = () => {
      if (cancelled || !shaderRef.current) return;
      // Wait until the host has a real box — hard reload can race layout.
      if (shaderRef.current.clientWidth < 2 || shaderRef.current.clientHeight < 2) {
        raf = window.requestAnimationFrame(mount);
        return;
      }

      try {
        shaderMount.current?.dispose();
        const image = new Image();
        image.src = emptyPixel;
        shaderMount.current = new ShaderMount(
          shaderRef.current,
          liquidMetalFragmentShader,
          {
            u_image: image,
            u_isImage: false,
            // Prompt: u_shape: 1 (circle) for icon button.
            u_shape: iconLike ? LiquidMetalShapes.circle : LiquidMetalShapes.none,
            u_colorBack: [0.08, 0.08, 0.08, 1],
            u_colorTint: iconLike ? [0.96, 0.96, 0.98, 1] : [0.92, 0.92, 0.94, 1],
            u_repetition: 4,
            u_softness: 0.5,
            u_shiftRed: 0.3,
            u_shiftBlue: 0.3,
            u_distortion: 0,
            u_contour: 0,
            u_angle: 45,
            u_fit: ShaderFitOptions.cover,
            // Prompt used u_scale: 8; current API tops out ~4.
            u_scale: iconLike ? 3.2 : 1.15,
            u_rotation: 0,
            u_originX: 0.5,
            u_originY: 0.5,
            u_offsetX: iconLike ? 0.1 : 0,
            u_offsetY: iconLike ? -0.1 : 0,
            u_worldWidth: 0,
            u_worldHeight: 0,
          },
          undefined,
          baseSpeed,
        );
      } catch {
        shaderMount.current = null;
      }
    };

    raf = window.requestAnimationFrame(mount);

    return () => {
      cancelled = true;
      window.cancelAnimationFrame(raf);
      shaderMount.current?.dispose();
      shaderMount.current = null;
    };
  }, [width, height, isFab, viewMode]);

  const handleMouseEnter = () => {
    hoveredRef.current = true;
    setIsHovered(true);
    shaderMount.current?.setSpeed(1);
  };

  const handleMouseLeave = () => {
    hoveredRef.current = false;
    setIsHovered(false);
    setIsPressed(false);
    shaderMount.current?.setSpeed(0.6);
  };

  const handleClick = (e: MouseEvent<HTMLButtonElement>) => {
    if (shaderMount.current) {
      shaderMount.current.setSpeed(2.4);
      window.setTimeout(() => {
        shaderMount.current?.setSpeed(hoveredRef.current ? 1 : 0.6);
      }, 300);
    }

    if (buttonRef.current) {
      const rect = buttonRef.current.getBoundingClientRect();
      const ripple = {
        x: e.clientX - rect.left,
        y: e.clientY - rect.top,
        id: rippleId.current++,
      };
      setRipples((prev) => [...prev, ripple]);
      window.setTimeout(() => {
        setRipples((prev) => prev.filter((r) => r.id !== ripple.id));
      }, 600);
    }

    onClick?.();
  };

  const labelColor = isFab || isIcon ? "#c4c4c4" : "#d4d4d4";
  // FAB: include a CSS chrome hairline so the rim still reads if WebGL is late/missing after reload.
  const shadowIdle = isFab || isIcon
    ? "0px 0px 0px 1px rgba(210, 210, 220, 0.55), inset 0px 0px 0px 1px rgba(255, 255, 255, 0.22), 0px 8px 6px 0px rgba(0, 0, 0, 0.12), 0px 2px 5px 0px rgba(0, 0, 0, 0.2)"
    : "0px 0px 0px 1px rgba(255, 255, 255, 0.12), 0px 36px 14px 0px rgba(0, 0, 0, 0.02), 0px 20px 12px 0px rgba(0, 0, 0, 0.08), 0px 9px 9px 0px rgba(0, 0, 0, 0.12), 0px 2px 5px 0px rgba(0, 0, 0, 0.15)";
  const shadowHover = isFab || isIcon
    ? "0px 0px 0px 1px rgba(230, 230, 240, 0.7), inset 0px 0px 0px 1px rgba(255, 255, 255, 0.3), 0px 8px 5px 0px rgba(0, 0, 0, 0.15), 0px 1px 2px 0px rgba(0, 0, 0, 0.2)"
    : "0px 0px 0px 1px rgba(255, 255, 255, 0.2), 0px 12px 6px 0px rgba(0, 0, 0, 0.05), 0px 8px 5px 0px rgba(0, 0, 0, 0.1), 0px 4px 4px 0px rgba(0, 0, 0, 0.15), 0px 1px 2px 0px rgba(0, 0, 0, 0.2)";
  const shadowPressed = isFab || isIcon
    ? "0px 0px 0px 1px rgba(180, 180, 190, 0.45), inset 0px 0px 0px 1px rgba(255, 255, 255, 0.15), 0px 1px 2px 0px rgba(0, 0, 0, 0.3)"
    : "0px 0px 0px 1px rgba(255, 255, 255, 0.1), 0px 1px 2px 0px rgba(0, 0, 0, 0.3)";

  const showIcon = viewMode === "icon" || isFab;

  return (
    <div
      ref={rootRef}
      className={[
        "liquid-metal-btn",
        isFab ? "liquid-metal-btn--fab" : "",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <div
        style={{
          perspective: "1000px",
          perspectiveOrigin: "50% 50%",
        }}
      >
        <div
          style={{
            position: "relative",
            width: `${width}px`,
            height: `${height}px`,
            transformStyle: "preserve-3d",
            transition: "width 0.4s ease, height 0.4s ease",
            transform: "none",
          }}
        >
          <div
            style={{
              position: "absolute",
              top: 0,
              left: 0,
              width: `${width}px`,
              height: `${height}px`,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              gap: "6px",
              transformStyle: "preserve-3d",
              transform: isFab ? undefined : "translateZ(20px)",
              zIndex: 30,
              pointerEvents: "none",
            }}
          >
            {showIcon &&
              (icon ?? (
                <Sparkles
                  size={16}
                  style={{
                    color: labelColor,
                    filter: "drop-shadow(0px 1px 2px rgba(0, 0, 0, 0.5))",
                    transform: "scale(1)",
                  }}
                />
              ))}
            {!showIcon && viewMode === "text" && (
              <span
                style={{
                  fontSize: "14px",
                  color: labelColor,
                  fontWeight: 400,
                  textShadow: "0px 1px 2px rgba(0, 0, 0, 0.5)",
                  whiteSpace: "nowrap",
                }}
              >
                {label}
              </span>
            )}
          </div>

          <div
            style={{
              position: "absolute",
              top: 0,
              left: 0,
              width: `${width}px`,
              height: `${height}px`,
              transformStyle: "preserve-3d",
              transform: isFab ? undefined : `translateZ(10px) ${pressMotion}`,
              zIndex: 20,
              transition: isFab
                ? "box-shadow 0.15s cubic-bezier(0.4, 0, 0.2, 1)"
                : "transform 0.15s cubic-bezier(0.4, 0, 0.2, 1)",
            }}
          >
            <div
              style={{
                width: `${innerWidth}px`,
                height: `${innerHeight}px`,
                margin: `${inset}px`,
                borderRadius: "100px",
                background: "linear-gradient(180deg, #202020 0%, #000000 100%)",
                boxShadow: isPressed
                  ? "inset 0px 2px 4px rgba(0, 0, 0, 0.4), inset 0px 1px 2px rgba(0, 0, 0, 0.3)"
                  : "none",
                transition: "box-shadow 0.15s cubic-bezier(0.4, 0, 0.2, 1)",
              }}
            />
          </div>

          <div
            style={{
              position: "absolute",
              top: 0,
              left: 0,
              width: `${width}px`,
              height: `${height}px`,
              transformStyle: "preserve-3d",
              transform: isFab ? undefined : `translateZ(0px) ${pressMotion}`,
              zIndex: 10,
              transition: isFab
                ? "box-shadow 0.15s cubic-bezier(0.4, 0, 0.2, 1)"
                : "transform 0.15s cubic-bezier(0.4, 0, 0.2, 1)",
            }}
          >
            <div
              style={{
                height: `${height}px`,
                width: `${width}px`,
                borderRadius: "100px",
                boxShadow: isPressed ? shadowPressed : isHovered ? shadowHover : shadowIdle,
                transition: "box-shadow 0.15s cubic-bezier(0.4, 0, 0.2, 1)",
                background: "rgb(0 0 0 / 0)",
              }}
            >
              <div
                ref={shaderRef}
                className="liquid-metal-btn__shader"
                style={{
                  borderRadius: "100px",
                  overflow: "hidden",
                  position: "relative",
                  width: `${width}px`,
                  maxWidth: `${width}px`,
                  height: `${height}px`,
                }}
              />
            </div>
          </div>

          <button
            ref={buttonRef}
            type={type}
            disabled={disabled}
            title={title}
            aria-expanded={ariaExpanded}
            onClick={handleClick}
            onMouseEnter={handleMouseEnter}
            onMouseLeave={handleMouseLeave}
            onMouseDown={() => setIsPressed(true)}
            onMouseUp={() => setIsPressed(false)}
            style={{
              position: "absolute",
              top: 0,
              left: 0,
              width: `${width}px`,
              height: `${height}px`,
              background: "transparent",
              border: "none",
              cursor: disabled ? "not-allowed" : "pointer",
              outline: "none",
              zIndex: 40,
              transformStyle: "preserve-3d",
              transform: isFab ? undefined : "translateZ(25px)",
              overflow: "hidden",
              borderRadius: "100px",
              opacity: disabled ? 0.55 : 1,
            }}
            aria-label={ariaLabel ?? label}
          >
            {ripples.map((ripple) => (
              <span
                key={ripple.id}
                style={{
                  position: "absolute",
                  left: `${ripple.x}px`,
                  top: `${ripple.y}px`,
                  width: "20px",
                  height: "20px",
                  borderRadius: "50%",
                  background:
                    "radial-gradient(circle, rgba(255, 255, 255, 0.4) 0%, rgba(255, 255, 255, 0) 70%)",
                  pointerEvents: "none",
                  animation: "liquid-metal-ripple 0.6s ease-out",
                }}
              />
            ))}
          </button>
        </div>
      </div>
    </div>
  );
}

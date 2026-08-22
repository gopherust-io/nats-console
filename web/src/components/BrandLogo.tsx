import type { HTMLAttributes } from "react";

type BrandLogoProps = {
  className?: string;
  /** Icon-only (no wordmark). */
  markOnly?: boolean;
} & Omit<HTMLAttributes<HTMLElement>, "children">;

/** Cache-bust when assets are replaced during local iteration. */
const LOGO_REV = "20260822b";

/** NC speech-bubble mark cropped from the brand artwork. */
export function BrandMark({ className = "", size = 30 }: { className?: string; size?: number }) {
  return (
    <img
      className={`brand__mark-img ${className}`.trim()}
      src={`/brand-mark.png?v=${LOGO_REV}`}
      width={size}
      height={size}
      alt=""
      draggable={false}
    />
  );
}

/** Full logo cropped from the brand artwork (dark theme wordmark). */
export default function BrandLogo({ className = "", markOnly = false, ...rest }: BrandLogoProps) {
  if (markOnly) {
    return (
      <span className={`brand-logo brand-logo--mark ${className}`.trim()} {...rest}>
        <BrandMark />
      </span>
    );
  }

  return (
    <span className={`brand-logo ${className}`.trim()} {...rest}>
      <img
        className="brand-logo__img"
        src={`/brand-logo.png?v=${LOGO_REV}`}
        alt="NATS-CONSOLE"
        draggable={false}
      />
    </span>
  );
}

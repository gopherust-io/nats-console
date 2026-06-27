import type { ThemePreview } from "../lib/theme";

type Props = {
  preview: ThemePreview;
  size?: number;
};

/** Minimal swatch: rounded square + accent bar */
export default function ThemeIcon({ preview, size = 14 }: Props) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 14 14"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
      className="theme-icon"
    >
      <rect width="14" height="14" rx="3" fill={preview.bg} stroke="currentColor" strokeOpacity="0.12" />
      <rect x="3" y="9" width="8" height="2" rx="1" fill={preview.accent} />
      <circle cx="4.5" cy="4.5" r="1.5" fill={preview.accent} opacity="0.85" />
    </svg>
  );
}

import { useEffect, useState } from "react";
import { useMediaQuery } from "../../hooks/useMediaQuery";

type ClockNumberProps = {
  value: number;
  className?: string;
  /** Defaults to locale string with grouping separators. */
  format?: (n: number) => string;
};

function RollingDigit({ digit }: { digit: number }) {
  return (
    <span className="nc-clock-digit" aria-hidden="true">
      <span className="nc-clock-digit__strip" style={{ transform: `translateY(-${digit * 10}%)` }}>
        {Array.from({ length: 10 }, (_, i) => (
          <span key={i} className="nc-clock-digit__num">
            {i}
          </span>
        ))}
      </span>
    </span>
  );
}

/**
 * Renders a number with per-digit clock/odometer rolls when the value changes.
 */
export default function ClockNumber({
  value,
  className,
  format = (n) => n.toLocaleString(),
}: ClockNumberProps) {
  const reduceMotion = useMediaQuery("(prefers-reduced-motion: reduce)");
  const [ready, setReady] = useState(false);
  const text = format(value);
  const chars = Array.from(text);

  useEffect(() => {
    const id = requestAnimationFrame(() => setReady(true));
    return () => cancelAnimationFrame(id);
  }, []);

  if (reduceMotion) {
    return <span className={["nc-clock", className].filter(Boolean).join(" ")}>{text}</span>;
  }

  return (
    <span
      className={["nc-clock", ready ? "nc-clock--ready" : "", className].filter(Boolean).join(" ")}
      aria-label={text}
    >
      {chars.map((ch, i) => {
        const fromRight = chars.length - 1 - i;
        if (ch >= "0" && ch <= "9") {
          return <RollingDigit key={`d-${fromRight}`} digit={Number(ch)} />;
        }
        return (
          <span key={`s-${fromRight}-${ch}`} className="nc-clock-sep" aria-hidden="true">
            {ch}
          </span>
        );
      })}
    </span>
  );
}

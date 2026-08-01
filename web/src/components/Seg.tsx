import {
  ButtonHTMLAttributes,
  HTMLAttributes,
  ReactNode,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

type HintState = {
  id: string;
  text: string;
  left: number;
  top: number;
};

type ActiveHint = HintState | null;

let activeHint: ActiveHint = null;
const listeners = new Set<(hint: ActiveHint) => void>();

function publishHint(next: ActiveHint) {
  activeHint = next;
  listeners.forEach((listener) => listener(next));
}

function useActiveHint() {
  const [hint, setHint] = useState<ActiveHint>(activeHint);
  useEffect(() => {
    listeners.add(setHint);
    setHint(activeHint);
    return () => {
      listeners.delete(setHint);
    };
  }, []);
  return hint;
}

/** Mount once in the app shell. Renders a single floating hint. */
export function SegHintHost() {
  const hint = useActiveHint();
  if (!hint || typeof document === "undefined") return null;
  return createPortal(
    <div
      className="seg-hint"
      role="tooltip"
      style={{ left: hint.left, top: hint.top }}
    >
      {hint.text}
    </div>,
    document.body,
  );
}

export function Seg({
  children,
  className,
  ...props
}: HTMLAttributes<HTMLDivElement> & { children: ReactNode }) {
  return (
    <div className={className ? `seg ${className}` : "seg"} {...props}>
      {children}
    </div>
  );
}

type SegBtnProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  hint?: string;
};

export function SegBtn({
  hint,
  className,
  onClick,
  onMouseEnter,
  onMouseLeave,
  onFocus,
  onBlur,
  children,
  ...props
}: SegBtnProps) {
  const id = useId();
  const btnRef = useRef<HTMLButtonElement>(null);

  const hide = useCallback(() => {
    if (activeHint?.id === id) {
      publishHint(null);
    }
  }, [id]);

  const show = useCallback(() => {
    if (!hint || !btnRef.current) return;
    const rect = btnRef.current.getBoundingClientRect();
    publishHint({
      id,
      text: hint,
      left: rect.left + rect.width / 2,
      top: rect.top - 8,
    });
  }, [hint, id]);

  useEffect(() => () => hide(), [hide]);

  useEffect(() => {
    if (!hint) return;
    const onScroll = () => hide();
    window.addEventListener("scroll", onScroll, true);
    return () => window.removeEventListener("scroll", onScroll, true);
  }, [hint, hide]);

  return (
    <button
      {...props}
      ref={btnRef}
      type={props.type ?? "button"}
      className={className ? `seg__btn ${className}` : "seg__btn"}
      onMouseEnter={(event) => {
        show();
        onMouseEnter?.(event);
      }}
      onMouseLeave={(event) => {
        hide();
        onMouseLeave?.(event);
      }}
      onFocus={(event) => {
        // Keyboard only — avoid sticky mouse-focus hints after click.
        if (event.currentTarget.matches(":focus-visible")) {
          show();
        }
        onFocus?.(event);
      }}
      onBlur={(event) => {
        hide();
        onBlur?.(event);
      }}
      onClick={(event) => {
        onClick?.(event);
        hide();
        event.currentTarget.blur();
      }}
    >
      {children}
    </button>
  );
}

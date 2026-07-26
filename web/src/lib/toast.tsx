import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

export type ToastKind = "error" | "success" | "info";

type ToastItem = {
  id: string;
  kind: ToastKind;
  message: string;
};

type ToastApi = {
  push: (kind: ToastKind, message: string) => void;
  error: (message: string) => void;
  success: (message: string) => void;
  info: (message: string) => void;
  dismiss: (id: string) => void;
};

const ToastContext = createContext<ToastApi | null>(null);

const AUTO_DISMISS_MS = 6_000;

function newId() {
  return typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `toast-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const timers = useRef(new Map<string, number>());

  const dismiss = useCallback((id: string) => {
    const timer = timers.current.get(id);
    if (timer) {
      window.clearTimeout(timer);
      timers.current.delete(id);
    }
    setToasts((prev) => prev.filter((toast) => toast.id !== id));
  }, []);

  const push = useCallback(
    (kind: ToastKind, message: string) => {
      const text = message.trim();
      if (!text) return;

      setToasts((prev) => {
        const existing = prev.find((toast) => toast.kind === kind && toast.message === text);
        if (existing) {
          const timer = timers.current.get(existing.id);
          if (timer) window.clearTimeout(timer);
          const nextTimer = window.setTimeout(() => dismiss(existing.id), AUTO_DISMISS_MS);
          timers.current.set(existing.id, nextTimer);
          return prev;
        }

        const id = newId();
        const nextTimer = window.setTimeout(() => dismiss(id), AUTO_DISMISS_MS);
        timers.current.set(id, nextTimer);
        return [...prev, { id, kind, message: text }];
      });
    },
    [dismiss],
  );

  useEffect(() => {
    const activeTimers = timers.current;
    return () => {
      for (const timer of activeTimers.values()) {
        window.clearTimeout(timer);
      }
      activeTimers.clear();
    };
  }, []);

  const api = useMemo<ToastApi>(
    () => ({
      push,
      error: (message) => push("error", message),
      success: (message) => push("success", message),
      info: (message) => push("info", message),
      dismiss,
    }),
    [push, dismiss],
  );

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div className="toast-stack" aria-live="polite" aria-relevant="additions text">
        {toasts.map((toast) => (
          <div key={toast.id} className={`toast toast--${toast.kind}`} role={toast.kind === "error" ? "alert" : "status"}>
            <p className="toast__message">{toast.message}</p>
            <button
              type="button"
              className="toast__dismiss"
              aria-label="Dismiss"
              onClick={() => dismiss(toast.id)}
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    // Safe no-op outside provider (tests / isolated render).
    return {
      push: () => undefined,
      error: () => undefined,
      success: () => undefined,
      info: () => undefined,
      dismiss: () => undefined,
    };
  }
  return ctx;
}

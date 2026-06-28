import {
  FormEvent,
  KeyboardEvent as ReactKeyboardEvent,
  PointerEvent as ReactPointerEvent,
  useEffect,
  useRef,
  useState,
} from "react";
import { useLocation } from "react-router-dom";
import { useAuth } from "../lib/auth";
import { useCluster } from "../lib/cluster";
import {
  AssistantRequestError,
  fetchAssistantConfig,
  pageContextFromLocation,
  sendAssistantMessage,
  type AssistantMessage,
} from "../lib/assistant";
import AssistantErrorBanner from "./AssistantErrorBanner";
import {
  clampAssistantPanelLayout,
  defaultAssistantPanelLayout,
  fullscreenAssistantPanelLayout,
  loadAssistantPanelLayout,
  saveAssistantPanelLayout,
  type AssistantPanelLayout,
} from "../lib/assistantPanelLayout";
import AssistantMessageBody from "./AssistantMessageBody";

const STARTERS = [
  "Summarize this cluster's JetStream usage",
  "Which streams have the most messages?",
  "Explain consumer lag on the current stream",
  "What retention policy should I use here?",
];

type DragMode = "move" | "resize";

export default function AssistantPanel() {
  const { user } = useAuth();
  const { clusterId } = useCluster();
  const location = useLocation();
  const [open, setOpen] = useState(false);
  const [configured, setConfigured] = useState(false);
  const [messages, setMessages] = useState<AssistantMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<AssistantRequestError | null>(null);
  const [layout, setLayout] = useState<AssistantPanelLayout>(() => defaultAssistantPanelLayout());
  const [fullscreen, setFullscreen] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);
  const fullscreenRef = useRef(false);
  const layoutBeforeFullscreenRef = useRef<AssistantPanelLayout | null>(null);
  const lastRequestRef = useRef<{ text: string; history: AssistantMessage[] } | null>(null);
  const dragRef = useRef<{
    mode: DragMode;
    startX: number;
    startY: number;
    origin: AssistantPanelLayout;
  } | null>(null);

  useEffect(() => {
    if (!user) {
      setConfigured(false);
      return;
    }
    fetchAssistantConfig().then((cfg) => {
      setConfigured(cfg.aiEnabled);
    });
  }, [user]);

  useEffect(() => {
    fullscreenRef.current = fullscreen;
  }, [fullscreen]);

  useEffect(() => {
    if (!open) {
      setFullscreen(false);
      return;
    }
    setLayout(loadAssistantPanelLayout());
  }, [open]);

  useEffect(() => {
    if (!fullscreen) return;
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        const restored = clampAssistantPanelLayout(
          layoutBeforeFullscreenRef.current ?? loadAssistantPanelLayout(),
        );
        layoutBeforeFullscreenRef.current = null;
        setLayout(restored);
        setFullscreen(false);
        saveAssistantPanelLayout(restored);
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [fullscreen]);

  useEffect(() => {
    if (!open) return;
    bottomRef.current?.scrollIntoView({ behavior: "auto" });
  }, [messages, loading, open]);

  useEffect(() => {
    if (!open) return;

    function onPointerMove(event: PointerEvent) {
      const drag = dragRef.current;
      if (!drag) return;

      const dx = event.clientX - drag.startX;
      const dy = event.clientY - drag.startY;

      if (drag.mode === "move") {
        setLayout(
          clampAssistantPanelLayout({
            ...drag.origin,
            x: drag.origin.x + dx,
            y: drag.origin.y + dy,
          }),
        );
        return;
      }

      setLayout(
        clampAssistantPanelLayout({
          ...drag.origin,
          width: drag.origin.width + dx,
          height: drag.origin.height + dy,
        }),
      );
    }

    function onPointerUp() {
      if (!dragRef.current) return;
      dragRef.current = null;
      if (fullscreenRef.current) return;
      setLayout((current) => {
        const next = clampAssistantPanelLayout(current);
        saveAssistantPanelLayout(next);
        return next;
      });
    }

    function onWindowResize() {
      if (fullscreenRef.current) {
        setLayout(fullscreenAssistantPanelLayout());
        return;
      }
      setLayout((current) => {
        const next = clampAssistantPanelLayout(current);
        saveAssistantPanelLayout(next);
        return next;
      });
    }

    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", onPointerUp);
    window.addEventListener("resize", onWindowResize);
    return () => {
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", onPointerUp);
      window.removeEventListener("resize", onWindowResize);
    };
  }, [open]);

  if (!user) {
    return null;
  }

  function exitFullscreen() {
    const restored = clampAssistantPanelLayout(
      layoutBeforeFullscreenRef.current ?? loadAssistantPanelLayout(),
    );
    layoutBeforeFullscreenRef.current = null;
    setLayout(restored);
    setFullscreen(false);
    saveAssistantPanelLayout(restored);
  }

  function enterFullscreen() {
    layoutBeforeFullscreenRef.current = layout;
    setLayout(fullscreenAssistantPanelLayout());
    setFullscreen(true);
  }

  function toggleFullscreen() {
    if (fullscreen) {
      exitFullscreen();
      return;
    }
    enterFullscreen();
  }

  function beginDrag(mode: DragMode, event: ReactPointerEvent) {
    if (fullscreen) return;
    if (event.button !== 0) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = {
      mode,
      startX: event.clientX,
      startY: event.clientY,
      origin: layout,
    };
  }

  async function sendMessage(text: string, options?: { history?: AssistantMessage[]; appendUser?: boolean }) {
    if (!configured) return;
    const trimmed = text.trim();
    if (!trimmed || !clusterId || loading) return;

    const priorHistory = options?.history ?? messages;
    const appendUser = options?.appendUser ?? true;
    setInput("");
    setError(null);
    const userMessage: AssistantMessage = { role: "user", content: trimmed };
    const history = appendUser ? [...priorHistory, userMessage] : priorHistory;
    lastRequestRef.current = { text: trimmed, history: priorHistory };
    if (appendUser) {
      setMessages(history);
    }
    setLoading(true);

    try {
      const reply = await sendAssistantMessage(
        clusterId,
        trimmed,
        priorHistory,
        pageContextFromLocation(location.pathname),
      );
      setMessages([...history, { role: "assistant", content: reply }]);
      lastRequestRef.current = null;
    } catch (err) {
      const assistantError =
        err instanceof AssistantRequestError
          ? err
          : new AssistantRequestError(err instanceof Error ? err.message : "Assistant failed", {
              code: "provider",
              retryable: true,
            });
      setError(assistantError);
      if (appendUser) {
        setMessages(history);
      }
    } finally {
      setLoading(false);
    }
  }

  function retryLastMessage() {
    const last = lastRequestRef.current;
    if (!last) return;
    void sendMessage(last.text, { history: last.history, appendUser: false });
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    await sendMessage(input);
  }

  function onInputKeyDown(event: ReactKeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== "Enter" || event.shiftKey) return;
    event.preventDefault();
    void sendMessage(input);
  }

  function askStarter(text: string) {
    void sendMessage(text);
  }

  return (
    <>
      <button
        type="button"
        className={`assistant-fab${configured ? "" : " assistant-fab--setup"}`}
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-label="Open NATS JetStream assistant"
        title="JetStream AI assistant"
      >
        AI
      </button>

      {open && (
        <div
          className={`assistant-panel${fullscreen ? " assistant-panel--fullscreen" : ""}`}
          role="dialog"
          aria-label="NATS JetStream assistant"
          style={{
            left: `${layout.x}px`,
            top: `${layout.y}px`,
            width: `${layout.width}px`,
            height: `${layout.height}px`,
          }}
        >
          <div
            className="assistant-panel__header"
            onPointerDown={(event) => beginDrag("move", event)}
          >
            <div>
              <div className="assistant-panel__title">JetStream Assistant</div>
              <div className="assistant-panel__subtitle">
                {configured ? "Gemini · NATS context only" : "Not configured yet"}
              </div>
            </div>
            <div className="assistant-panel__actions">
              <button
                type="button"
                className="assistant-panel__icon-btn"
                onPointerDown={(event) => event.stopPropagation()}
                onClick={toggleFullscreen}
                aria-label={fullscreen ? "Exit fullscreen" : "Enter fullscreen"}
                title={fullscreen ? "Exit fullscreen (Esc)" : "Fullscreen"}
              >
                {fullscreen ? "⤡" : "⤢"}
              </button>
              <button
                type="button"
                className="assistant-panel__close"
                onPointerDown={(event) => event.stopPropagation()}
                onClick={() => setOpen(false)}
                aria-label="Close assistant"
              >
                ×
              </button>
            </div>
          </div>

          {!configured ? (
            <div className="assistant-panel__setup">
              <p>Добавьте Gemini API key в файл <code>.env</code> в корне проекта.</p>
              <pre className="assistant-panel__code">{`AI_ENABLED=true
AI_API_KEY=your-gemini-key
AI_MODEL=gemini-2.5-flash`}</pre>
              <p className="muted">Then restart: <code>docker compose up -d console</code></p>
              <p className="muted">The assistant only uses NATS JetStream context — no secrets or DB data.</p>
            </div>
          ) : (
            <>
              <div className="assistant-panel__messages">
                {messages.length === 0 && (
                  <div className="assistant-panel__empty">
                    <p>Ask about streams, consumers, lag, retention, or this cluster's health.</p>
                    <div className="assistant-starters">
                      {STARTERS.map((text) => (
                        <button
                          key={text}
                          type="button"
                          className="assistant-starter"
                          disabled={loading || !clusterId}
                          onClick={() => askStarter(text)}
                        >
                          {text}
                        </button>
                      ))}
                    </div>
                  </div>
                )}
                {messages.map((msg, index) => (
                  <div key={index} className={`assistant-msg assistant-msg--${msg.role}`}>
                    {msg.role === "assistant" ? (
                      <AssistantMessageBody content={msg.content} />
                    ) : (
                      msg.content
                    )}
                  </div>
                ))}
                {loading && <div className="assistant-msg assistant-msg--assistant assistant-msg--typing">Thinking…</div>}
                {error && (
                  <AssistantErrorBanner
                    key={`${error.code}:${error.message}:${error.retryAfterSeconds ?? 0}`}
                    error={error}
                    onDismiss={() => setError(null)}
                    onRetry={retryLastMessage}
                  />
                )}
                <div ref={bottomRef} />
              </div>

              <form className="assistant-panel__form" onSubmit={onSubmit}>
                <textarea
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={onInputKeyDown}
                  placeholder="Ask about your JetStream cluster…"
                  rows={2}
                  disabled={loading || !clusterId}
                />
                <button className="btn" type="submit" disabled={loading || !input.trim() || !clusterId}>
                  Send
                </button>
              </form>
            </>
          )}

          {!fullscreen && (
            <div
              className="assistant-panel__resize"
              aria-hidden="true"
              onPointerDown={(event) => beginDrag("resize", event)}
            />
          )}
        </div>
      )}
    </>
  );
}

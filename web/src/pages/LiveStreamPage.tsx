import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Link, useParams } from "react-router-dom";
import { decodeBase64, getAuthHeader, getWebSocketURL, tryParseJSON } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { LIVE_STREAM_MAX_MESSAGES, LIVE_SUBJECT_FILTER_DEBOUNCE_MS } from "../lib/constants";

type LiveMessage = {
  type: string;
  seq?: number;
  subject?: string;
  time?: string;
  data?: string;
  error?: string;
};

const MAX_MESSAGES = LIVE_STREAM_MAX_MESSAGES;
const WS_BATCH_MS = 100;
const ESTIMATED_ROW_HEIGHT = 120;

const LiveMessageRow = memo(function LiveMessageRow({
  msg,
  rawMode,
}: {
  msg: LiveMessage;
  rawMode: boolean;
}) {
  const display = useMemo(() => {
    if (!msg.data) return "";
    const payload = decodeBase64(msg.data);
    if (rawMode) return payload;
    const parsed = tryParseJSON(payload);
    return parsed.isJSON ? JSON.stringify(parsed.parsed, null, 2) : payload;
  }, [msg.data, rawMode]);

  return (
    <div className="live-entry">
      <span className="live-meta">
        #{msg.seq} · {msg.subject} · {msg.time}
      </span>
      <pre className="mono">{display}</pre>
    </div>
  );
});

export default function LiveStreamPage() {
  const { name = "" } = useParams();
  const { clusterId } = useCluster();
  const [messages, setMessages] = useState<LiveMessage[]>([]);
  const [status, setStatus] = useState("disconnected");
  const [subjectInput, setSubjectInput] = useState("");
  const [subjectFilter, setSubjectFilter] = useState("");
  const [paused, setPaused] = useState(false);
  const [rawMode, setRawMode] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const pausedRef = useRef(false);
  const pendingRef = useRef<LiveMessage[]>([]);
  const flushTimerRef = useRef<number | null>(null);
  const logRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    pausedRef.current = paused;
  }, [paused]);

  const flushPending = useCallback(() => {
    const batch = pendingRef.current;
    if (batch.length === 0) return;
    pendingRef.current = [];
    setMessages((prev) => {
      const combined = prev.concat(batch);
      if (combined.length <= MAX_MESSAGES) {
        return combined;
      }
      return combined.slice(combined.length - MAX_MESSAGES);
    });
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => setSubjectFilter(subjectInput.trim()), LIVE_SUBJECT_FILTER_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [subjectInput]);

  useEffect(() => {
    if (!clusterId || !name) return;

    const url = getWebSocketURL(clusterId, name, subjectFilter || undefined);
    const ws = new WebSocket(url);
    wsRef.current = ws;
    setStatus("connecting");
    pendingRef.current = [];
    if (flushTimerRef.current !== null) {
      window.clearTimeout(flushTimerRef.current);
      flushTimerRef.current = null;
    }

    ws.onopen = () => setStatus("connected");
    ws.onclose = () => setStatus("disconnected");
    ws.onerror = () => setStatus("error");
    ws.onmessage = (event) => {
      let frame: LiveMessage;
      try {
        frame = JSON.parse(event.data) as LiveMessage;
      } catch {
        setStatus("parse error");
        return;
      }
      if (frame.type === "message") {
        if (pausedRef.current) return;
        pendingRef.current.push(frame);
        if (flushTimerRef.current === null) {
          flushTimerRef.current = window.setTimeout(() => {
            flushTimerRef.current = null;
            flushPending();
          }, WS_BATCH_MS);
        }
      } else if (frame.type === "error") {
        setStatus(frame.error ?? "error");
      }
    };

    return () => {
      if (flushTimerRef.current !== null) {
        window.clearTimeout(flushTimerRef.current);
        flushTimerRef.current = null;
      }
      ws.close();
    };
  }, [clusterId, name, subjectFilter, flushPending]);

  const virtualizer = useVirtualizer({
    count: messages.length,
    getScrollElement: () => logRef.current,
    estimateSize: () => ESTIMATED_ROW_HEIGHT,
    overscan: 8,
  });

  useEffect(() => {
    if (messages.length === 0) return;
    virtualizer.scrollToIndex(messages.length - 1, { align: "end" });
  }, [messages.length, virtualizer]);

  function sendAction(action: string) {
    wsRef.current?.send(JSON.stringify({ action }));
    if (action === "pause") setPaused(true);
    if (action === "resume") setPaused(false);
    if (action === "clear") {
      pendingRef.current = [];
      if (flushTimerRef.current !== null) {
        window.clearTimeout(flushTimerRef.current);
        flushTimerRef.current = null;
      }
      setMessages([]);
    }
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <Link to={`/streams/${name}`} className="link-back">
            ← Back to {name}
          </Link>
          <h1>Live: {name}</h1>
        </div>
        <span className={`status-badge status-${status}`}>{status}</span>
      </div>

      <div className="live-controls">
        <label>
          Subject filter
          <input
            value={subjectInput}
            onChange={(e) => setSubjectInput(e.target.value)}
            placeholder="events.>"
          />
        </label>
        <button className="btn secondary" onClick={() => sendAction(paused ? "resume" : "pause")}>
          {paused ? "Resume" : "Pause"}
        </button>
        <button className="btn secondary" onClick={() => sendAction("clear")}>
          Clear
        </button>
        <button className="btn secondary" onClick={() => setRawMode((v) => !v)}>
          {rawMode ? "JSON" : "Raw"}
        </button>
      </div>

      <div className="live-log" ref={logRef}>
        {messages.length === 0 && <div className="text-muted">Waiting for messages...</div>}
        {messages.length > 0 && (
          <div style={{ height: virtualizer.getTotalSize(), position: "relative", width: "100%" }}>
            {virtualizer.getVirtualItems().map((item) => {
              const msg = messages[item.index];
              return (
                <div
                  key={msg.seq ?? `${msg.time}-${msg.subject}-${item.index}`}
                  ref={virtualizer.measureElement}
                  data-index={item.index}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    transform: `translateY(${item.start}px)`,
                  }}
                >
                  <LiveMessageRow msg={msg} rawMode={rawMode} />
                </div>
              );
            })}
          </div>
        )}
      </div>

      {!getAuthHeader() && (
        <div className="error">Authentication required for WebSocket connection in production.</div>
      )}
    </div>
  );
}

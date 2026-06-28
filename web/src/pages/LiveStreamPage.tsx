import { memo, useEffect, useMemo, useRef, useState } from "react";
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

  useEffect(() => {
    pausedRef.current = paused;
  }, [paused]);

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
        setMessages((prev) => {
          const next = prev.length >= MAX_MESSAGES ? prev.slice(1) : prev.slice();
          next.push(frame);
          return next;
        });
      } else if (frame.type === "error") {
        setStatus(frame.error ?? "error");
      }
    };

    return () => ws.close();
  }, [clusterId, name, subjectFilter]);

  function sendAction(action: string) {
    wsRef.current?.send(JSON.stringify({ action }));
    if (action === "pause") setPaused(true);
    if (action === "resume") setPaused(false);
    if (action === "clear") setMessages([]);
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

      <div className="live-log">
        {messages.length === 0 && <div className="text-muted">Waiting for messages...</div>}
        {messages.map((msg) => (
          <LiveMessageRow key={msg.seq ?? `${msg.time}-${msg.subject}`} msg={msg} rawMode={rawMode} />
        ))}
      </div>

      {!getAuthHeader() && (
        <div className="error">Authentication required for WebSocket connection in production.</div>
      )}
    </div>
  );
}

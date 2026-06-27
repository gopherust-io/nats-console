import { useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { decodeBase64, getAuthHeader, getWebSocketURL, tryParseJSON } from "../lib/api";
import { useCluster } from "../lib/cluster";

type LiveMessage = {
  type: string;
  seq?: number;
  subject?: string;
  time?: string;
  data?: string;
  error?: string;
};

export default function LiveStreamPage() {
  const { name = "" } = useParams();
  const { clusterId } = useCluster();
  const [messages, setMessages] = useState<LiveMessage[]>([]);
  const [status, setStatus] = useState("disconnected");
  const [subjectFilter, setSubjectFilter] = useState("");
  const [paused, setPaused] = useState(false);
  const [rawMode, setRawMode] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);

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
      const frame = JSON.parse(event.data) as LiveMessage;
      if (frame.type === "message") {
        setMessages((prev) => [...prev.slice(-499), frame]);
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
          <input value={subjectFilter} onChange={(e) => setSubjectFilter(e.target.value)} placeholder="events.>" />
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
        {messages.map((msg, idx) => {
          const payload = msg.data ? decodeBase64(msg.data) : "";
          const parsed = tryParseJSON(payload);
          return (
            <div key={`${msg.seq}-${idx}`} className="live-entry">
              <span className="live-meta">
                #{msg.seq} · {msg.subject} · {msg.time}
              </span>
              <pre className="mono">{rawMode || !parsed.isJSON ? payload : JSON.stringify(parsed.parsed, null, 2)}</pre>
            </div>
          );
        })}
      </div>

      {!getAuthHeader() && (
        <div className="error">Authentication required for WebSocket connection in production.</div>
      )}
    </div>
  );
}

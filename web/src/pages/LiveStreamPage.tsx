import { memo, useCallback, useEffect, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import MessageDownloadMenu from "../components/MessageDownloadMenu";
import MessagePayloadViewer from "../components/MessagePayloadViewer";
import Alert from "../components/ui/Alert";
import PageHeader from "../components/ui/PageHeader";
import { getWebSocketURL, jetStreamUIBase } from "../lib/api";
import { useCluster } from "../lib/cluster";
import { formatDateTime } from "../lib/datetime";
import { LIVE_STREAM_MAX_MESSAGES, LIVE_SUBJECT_FILTER_DEBOUNCE_MS } from "../lib/constants";
import { rowFromMessage } from "../lib/messageDownload";

type LiveMessage = {
  type: string;
  seq?: number;
  subject?: string;
  time?: string;
  data?: string;
  headers?: Record<string, string>;
  error?: string;
};

type ConnStatus = "disconnected" | "connecting" | "connected" | "error";

const MAX_MESSAGES = LIVE_STREAM_MAX_MESSAGES;
const WS_BATCH_MS = 100;
const ESTIMATED_ROW_HEIGHT = 140;
const FOLLOW_BOTTOM_PX = 80;

const LiveMessageRow = memo(function LiveMessageRow({ msg }: { msg: LiveMessage }) {
  const timeLabel = formatDateTime(msg.time, "—");

  return (
    <div className="live-entry">
      <div className="live-meta">
        <span className="mono">#{msg.seq ?? "—"}</span>
        <span className="mono" title={msg.subject}>
          {msg.subject ?? "—"}
        </span>
        {msg.time ? (
          <time dateTime={msg.time} title={msg.time}>
            {timeLabel}
          </time>
        ) : (
          <span>{timeLabel}</span>
        )}
      </div>
      {msg.data && (
        <MessagePayloadViewer
          data={msg.data}
          headers={msg.headers}
          compact
          showHeaders={false}
          cacheHost={msg}
        />
      )}
    </div>
  );
});

export default function LiveStreamPage() {
  const { t } = useTranslation();
  const { name = "", clusterId: routeCluster, accountName } = useParams();
  const { clusterId } = useCluster();
  const id = routeCluster ?? clusterId;
  const streamHref = id
    ? `${jetStreamUIBase(id, accountName)}/streams/${encodeURIComponent(name)}`
    : "/systems";
  const [messages, setMessages] = useState<LiveMessage[]>([]);
  const [status, setStatus] = useState<ConnStatus>("disconnected");
  const [statusDetail, setStatusDetail] = useState("");
  const [subjectInput, setSubjectInput] = useState("");
  const [subjectFilter, setSubjectFilter] = useState("");
  const [fromSeqInput, setFromSeqInput] = useState("");
  const [fromSeq, setFromSeq] = useState<number | undefined>(undefined);
  const [paused, setPaused] = useState(false);
  const [follow, setFollow] = useState(true);
  const [showJump, setShowJump] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const pausedRef = useRef(false);
  const followRef = useRef(true);
  const pendingRef = useRef<LiveMessage[]>([]);
  const flushTimerRef = useRef<number | null>(null);
  const logRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    pausedRef.current = paused;
  }, [paused]);

  useEffect(() => {
    followRef.current = follow;
  }, [follow]);

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
    const timer = window.setTimeout(() => {
      const trimmed = fromSeqInput.trim();
      if (!trimmed) {
        setFromSeq(undefined);
        return;
      }
      const seq = Number(trimmed);
      setFromSeq(Number.isFinite(seq) && seq > 0 ? seq : undefined);
    }, LIVE_SUBJECT_FILTER_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [fromSeqInput]);

  useEffect(() => {
    if (!id || !name) return;

    const url = getWebSocketURL(id, name, subjectFilter || undefined, fromSeq);
    const ws = new WebSocket(url);
    wsRef.current = ws;
    setStatus("connecting");
    setStatusDetail("");
    pendingRef.current = [];
    setMessages([]);
    if (flushTimerRef.current !== null) {
      window.clearTimeout(flushTimerRef.current);
      flushTimerRef.current = null;
    }

    const isActive = () => wsRef.current === ws;

    ws.onopen = () => {
      if (!isActive()) return;
      setStatus("connected");
    };
    ws.onclose = () => {
      if (!isActive()) return;
      setStatus("disconnected");
    };
    ws.onerror = () => {
      if (!isActive()) return;
      setStatus("error");
    };
    ws.onmessage = (event) => {
      if (!isActive()) return;
      let frame: LiveMessage;
      try {
        frame = JSON.parse(event.data) as LiveMessage;
      } catch {
        setStatus("error");
        setStatusDetail("parse error");
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
        setStatus("error");
        setStatusDetail(frame.error ?? "error");
      }
    };

    return () => {
      if (flushTimerRef.current !== null) {
        window.clearTimeout(flushTimerRef.current);
        flushTimerRef.current = null;
      }
      if (wsRef.current === ws) {
        wsRef.current = null;
      }
      ws.close();
    };
  }, [id, name, subjectFilter, fromSeq, flushPending]);

  const virtualizer = useVirtualizer({
    count: messages.length,
    getScrollElement: () => logRef.current,
    estimateSize: () => ESTIMATED_ROW_HEIGHT,
    overscan: 8,
  });

  useEffect(() => {
    if (messages.length === 0) return;
    if (!followRef.current) {
      setShowJump(true);
      return;
    }
    virtualizer.scrollToIndex(messages.length - 1, { align: "end" });
    setShowJump(false);
  }, [messages.length, virtualizer]);

  function onLogScroll() {
    const el = logRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight <= FOLLOW_BOTTOM_PX;
    if (nearBottom) {
      if (!followRef.current) {
        setFollow(true);
      }
      setShowJump(false);
    } else if (followRef.current) {
      setFollow(false);
      setShowJump(true);
    }
  }

  function jumpToLatest() {
    setFollow(true);
    setShowJump(false);
    if (messages.length > 0) {
      virtualizer.scrollToIndex(messages.length - 1, { align: "end" });
    }
  }

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
      setShowJump(false);
    }
  }

  const messagesRef = useRef(messages);
  messagesRef.current = messages;

  const getDownloadRows = useCallback(() => {
    return messagesRef.current
      .filter((msg) => msg.type === "message" || msg.data)
      .map((msg) =>
        rowFromMessage({
          seq: msg.seq ?? 0,
          subject: msg.subject ?? "",
          time: msg.time ?? "",
          data: msg.data,
          headers: msg.headers,
        }),
      );
  }, []);

  const statusLabel =
    status === "connected"
      ? t("liveStream.connected")
      : status === "connecting"
        ? t("liveStream.connecting")
        : status === "error"
          ? t("liveStream.error")
          : t("liveStream.disconnected");

  return (
    <div className="page">
      <PageHeader
        eyebrow={t("liveStream.eyebrow")}
        title={t("liveStream.title", { name })}
        badge={
          <span className={`status-badge status-${status}`} aria-live="polite">
            {statusLabel}
            {statusDetail ? `: ${statusDetail}` : ""}
          </span>
        }
      />

      <p className="mb-12">
        <Link to={streamHref} className="link-back">
          {t("liveStream.backToStream", { name })}
        </Link>
      </p>

      <p className="text-muted mb-12">{t("liveStream.clientHint")}</p>

      {status === "error" && statusDetail && <Alert variant="error">{statusDetail}</Alert>}

      <div className="live-controls">
        <label>
          {t("liveStream.subjectFilter")}
          <input
            value={subjectInput}
            onChange={(e) => setSubjectInput(e.target.value)}
            placeholder="events.>"
          />
        </label>
        <label>
          {t("liveStream.fromSequence")}
          <input
            type="number"
            min={1}
            value={fromSeqInput}
            onChange={(e) => setFromSeqInput(e.target.value)}
            placeholder={t("liveStream.fromSeqPlaceholder")}
          />
        </label>
        <button
          type="button"
          className="btn secondary"
          onClick={() => sendAction(paused ? "resume" : "pause")}
        >
          {paused ? t("liveStream.resume") : t("liveStream.pause")}
        </button>
        <button type="button" className="btn secondary" onClick={() => sendAction("clear")}>
          {t("liveStream.clear")}
        </button>
        <MessageDownloadMenu
          mode="live"
          stream={name}
          getRows={getDownloadRows}
          disabled={messages.length === 0}
        />
        <button
          type="button"
          className="btn secondary"
          aria-pressed={follow}
          onClick={() => {
            const next = !follow;
            setFollow(next);
            if (next) jumpToLatest();
            else setShowJump(true);
          }}
        >
          {follow ? t("liveStream.unfollow") : t("liveStream.follow")}
        </button>
      </div>

      <div className="live-toolbar-meta">
        <span className="text-muted">
          {t("liveStream.bufferCount", { count: messages.length, max: MAX_MESSAGES })}
        </span>
      </div>

      <div
        className="live-log"
        ref={logRef}
        role="log"
        aria-label={t("liveStream.eyebrow")}
        onScroll={onLogScroll}
      >
        {messages.length === 0 && <div className="text-muted">{t("liveStream.waiting")}</div>}
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
                  <LiveMessageRow msg={msg} />
                </div>
              );
            })}
          </div>
        )}
        {showJump && messages.length > 0 && (
          <div className="live-jump">
            <button type="button" className="btn" onClick={jumpToLatest}>
              {t("liveStream.jumpToLatest")}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

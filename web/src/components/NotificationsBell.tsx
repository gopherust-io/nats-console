import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { fetchAlertOpenSummary } from "../lib/alerts";
import { visibilityAwareInterval } from "../lib/query";

function formatWhen(iso: string) {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

export default function NotificationsBell() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [closing, setClosing] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const openRef = useRef(open);
  const closingRef = useRef(closing);
  openRef.current = open;
  closingRef.current = closing;

  const summaryQuery = useQuery({
    queryKey: ["alerts", "open-summary"],
    queryFn: fetchAlertOpenSummary,
    refetchInterval: visibilityAwareInterval(20_000),
  });

  const clearCloseTimer = useCallback(() => {
    if (closeTimer.current) {
      clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
  }, []);

  const requestClose = useCallback(() => {
    if (!openRef.current || closingRef.current) return;
    setClosing(true);
    clearCloseTimer();
    closeTimer.current = setTimeout(() => {
      setOpen(false);
      setClosing(false);
      closeTimer.current = null;
    }, 150);
  }, [clearCloseTimer]);

  const requestOpen = useCallback(() => {
    clearCloseTimer();
    setClosing(false);
    setOpen(true);
    void summaryQuery.refetch();
  }, [clearCloseTimer, summaryQuery]);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (!ref.current?.contains(e.target as Node)) requestClose();
    }
    document.addEventListener("mousedown", onDoc);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      clearCloseTimer();
    };
  }, [requestClose, clearCloseTimer]);

  const visible = open || closing;
  const count = summaryQuery.data?.count ?? 0;
  const alerts = summaryQuery.data?.alerts ?? [];

  return (
    <div className="nc-menu nc-alerts-menu" ref={ref}>
      <button
        type="button"
        className="nc-icon-btn nc-icon-btn--badge"
        aria-label={t("nav.notifications")}
        aria-expanded={open && !closing}
        onClick={() => (visible && !closing ? requestClose() : requestOpen())}
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
          <path d="M6 9a6 6 0 0 1 12 0c0 7 3 7 3 9H3c0-2 3-2 3-9" strokeLinecap="round" />
          <path d="M10 20a2 2 0 0 0 4 0" strokeLinecap="round" />
        </svg>
        {count > 0 && (
          <span className="nc-badge" aria-hidden>
            {count > 99 ? "99+" : count}
          </span>
        )}
      </button>
      {visible && (
        <div className="nc-menu__panel nc-alerts-panel" role="menu" data-state={closing ? "closed" : "open"}>
          <div className="nc-alerts-panel__head">
            <strong>{t("alerts.bellTitle")}</strong>
            <Link className="nc-alerts-panel__link" to="/admin/alerts" onClick={requestClose}>
              {t("alerts.viewAll")}
            </Link>
          </div>
          {alerts.length === 0 && (
            <div className="nc-menu__item" style={{ cursor: "default", opacity: 0.75 }}>
              {t("alerts.bellEmpty")}
            </div>
          )}
          {alerts.map((alert) => (
            <Link
              key={alert.id}
              className="nc-menu__item nc-alerts-panel__item"
              to="/admin/alerts"
              onClick={requestClose}
              role="menuitem"
            >
              <span className={`nc-severity nc-severity--${alert.severity}`}>{alert.severity}</span>
              <span className="nc-alerts-panel__msg">{alert.message || alert.ruleName}</span>
              <span className="nc-alerts-panel__meta">{formatWhen(alert.lastSeenAt)}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

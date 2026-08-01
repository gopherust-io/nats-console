import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ApiError, userFacingError } from "../../lib/api";
import { ASSISTANT_RETRY_COUNTDOWN_INTERVAL_MS } from "../../lib/constants";
import Alert from "./Alert";
import EmptyState from "./EmptyState";

type QueryErrorStateProps = {
  error: unknown;
  onRetry?: () => void;
  title?: string;
  /** When true, render as EmptyState instead of a compact Alert. */
  empty?: boolean;
};

export default function QueryErrorState({ error, onRetry, title, empty }: QueryErrorStateProps) {
  const { t } = useTranslation();
  const message = userFacingError(error, t);
  const heading = title ?? t("errors.loadFailed");
  const apiErr = error instanceof ApiError ? error : null;
  const showRetry = Boolean(onRetry) || Boolean(apiErr?.retryable && onRetry);
  const [retryIn, setRetryIn] = useState(apiErr?.retryAfterSeconds ?? 0);

  useEffect(() => {
    setRetryIn(apiErr?.retryAfterSeconds ?? 0);
  }, [apiErr?.retryAfterSeconds, apiErr?.message, apiErr?.code]);

  useEffect(() => {
    if (retryIn <= 0) return;
    const timer = window.setInterval(() => {
      setRetryIn((value) => Math.max(0, value - 1));
    }, ASSISTANT_RETRY_COUNTDOWN_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [retryIn]);

  const retryDisabled = !onRetry || retryIn > 0;
  const retryLabel = retryIn > 0 ? t("errors.retryIn", { seconds: retryIn }) : t("common.retry");

  if (empty) {
    return (
      <EmptyState
        title={heading}
        description={message}
        action={
          showRetry && onRetry ? (
            <button type="button" className="btn" onClick={onRetry} disabled={retryDisabled}>
              {retryLabel}
            </button>
          ) : undefined
        }
      />
    );
  }

  return (
    <div className="nc-query-error">
      <Alert variant="error">
        <div className="nc-query-error__row">
          <span>
            <strong>{heading}</strong>
            {message ? `: ${message}` : null}
          </span>
          {showRetry && onRetry && (
            <button type="button" className="btn btn--small" onClick={onRetry} disabled={retryDisabled}>
              {retryLabel}
            </button>
          )}
        </div>
      </Alert>
    </div>
  );
}

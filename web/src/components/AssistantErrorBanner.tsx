import { useEffect, useState } from "react";
import {
  AssistantRequestError,
  assistantErrorTitle,
  type AssistantErrorCode,
} from "../lib/assistant";
import { ASSISTANT_RETRY_COUNTDOWN_INTERVAL_MS } from "../lib/constants";

type AssistantErrorBannerProps = {
  error: AssistantRequestError;
  onDismiss: () => void;
  onRetry?: () => void;
};

function hintForCode(code: AssistantErrorCode): string | null {
  switch (code) {
    case "auth":
      return "Update AI_API_KEY in .env and restart the console.";
    case "quota":
      return "Try AI_MODEL=gemini-2.5-flash or enable billing in Google AI Studio.";
    case "not_enabled":
      return "Set AI_ENABLED=true and configure AI_API_KEY in .env.";
    case "timeout":
      return "Increase AI_REQUEST_TIMEOUT if responses are often slow.";
    default:
      return null;
  }
}

export default function AssistantErrorBanner({ error, onDismiss, onRetry }: AssistantErrorBannerProps) {
  const hint = hintForCode(error.code);
  const [retryIn, setRetryIn] = useState(error.retryAfterSeconds ?? 0);

  useEffect(() => {
    if (retryIn <= 0) return;
    const timer = window.setInterval(() => {
      setRetryIn((value) => Math.max(0, value - 1));
    }, ASSISTANT_RETRY_COUNTDOWN_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [retryIn]);

  const retryDisabled = retryIn > 0;
  const retryLabel = retryIn > 0 ? `Retry in ${retryIn}s` : "Retry";

  return (
    <div className={`assistant-error assistant-error--${error.code}`} role="alert">
      <div className="assistant-error__content">
        <div className="assistant-error__title">{assistantErrorTitle(error.code)}</div>
        <div className="assistant-error__message">{error.message}</div>
        {hint && <div className="assistant-error__hint">{hint}</div>}
      </div>
      <div className="assistant-error__actions">
        {error.retryable && onRetry && (
          <button
            type="button"
            className="assistant-error__retry"
            onClick={onRetry}
            disabled={retryDisabled}
          >
            {retryLabel}
          </button>
        )}
        <button type="button" className="assistant-error__dismiss" onClick={onDismiss} aria-label="Dismiss error">
          ×
        </button>
      </div>
    </div>
  );
}

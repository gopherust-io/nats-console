import { api, ApiError, clusterPath, UnauthorizedError } from "./api";
import { safeDecodeURIComponent } from "./safeDecode";

export type AssistantMessage = {
  role: "user" | "assistant";
  content: string;
};

export type AssistantPageContext = {
  route?: string;
  stream?: string;
  consumer?: string;
  bucket?: string;
  key?: string;
};

export type AssistantConfig = {
  aiEnabled: boolean;
  aiProvider?: string;
  aiModel?: string;
};

export type AssistantErrorCode =
  | "not_enabled"
  | "validation"
  | "blocked"
  | "context"
  | "rate_limit"
  | "quota"
  | "auth"
  | "timeout"
  | "provider"
  | "unavailable"
  | "network";

export class AssistantRequestError extends Error {
  code: AssistantErrorCode;
  retryable: boolean;
  retryAfterSeconds?: number;

  constructor(
    message: string,
    options: {
      code?: AssistantErrorCode;
      retryable?: boolean;
      retryAfterSeconds?: number;
    } = {},
  ) {
    super(message);
    this.name = "AssistantRequestError";
    this.code = options.code ?? "provider";
    this.retryable = options.retryable ?? false;
    this.retryAfterSeconds = options.retryAfterSeconds;
  }
}

function inferErrorCode(status: number): AssistantErrorCode {
  if (status === 401 || status === 403) return "auth";
  if (status === 404) return "not_enabled";
  if (status === 408 || status === 504) return "timeout";
  if (status === 429) return "rate_limit";
  if (status === 503) return "unavailable";
  if (status >= 500) return "provider";
  return "validation";
}

function toAssistantError(err: unknown): AssistantRequestError {
  if (err instanceof AssistantRequestError) return err;
  if (err instanceof UnauthorizedError) {
    return new AssistantRequestError(err.message || "Unauthorized", { code: "auth", retryable: false });
  }
  if (err instanceof ApiError) {
    return new AssistantRequestError(err.message, {
      code: inferErrorCode(err.status),
      retryable: err.retryable,
      retryAfterSeconds: err.retryAfterSeconds,
    });
  }
  if (err instanceof Error) {
    return new AssistantRequestError(err.message, { code: "provider", retryable: true });
  }
  return new AssistantRequestError("Assistant request failed", { code: "provider", retryable: true });
}

export function assistantErrorTitle(code: AssistantErrorCode): string {
  switch (code) {
    case "rate_limit":
    case "quota":
      return "Rate limit";
    case "auth":
      return "API key issue";
    case "timeout":
      return "Timed out";
    case "blocked":
      return "Request blocked";
    case "validation":
      return "Invalid request";
    case "context":
    case "unavailable":
    case "network":
      return "Connection issue";
    case "not_enabled":
      return "Not configured";
    default:
      return "Assistant error";
  }
}

export async function fetchAssistantConfig(): Promise<AssistantConfig> {
  try {
    const res = await api<AssistantConfig>("/api/v1/assistant/config");
    return res.data ?? { aiEnabled: false };
  } catch (err) {
    // 404 / not configured → disabled. Network/5xx must not look like "AI off".
    if (err instanceof ApiError && (err.status === 404 || err.code === "not_found")) {
      return { aiEnabled: false };
    }
    throw toAssistantError(err);
  }
}

export async function sendAssistantMessage(
  clusterId: string,
  message: string,
  history: AssistantMessage[],
  page: AssistantPageContext,
): Promise<string> {
  try {
    const res = await api<{ reply?: string }>(clusterPath(clusterId, "/assistant/chat"), {
      method: "POST",
      body: JSON.stringify({ message, history, page }),
    });
    const reply = res.data?.reply;
    if (!reply) {
      throw new AssistantRequestError("Assistant returned an empty response.", {
        code: "provider",
        retryable: true,
      });
    }
    return reply;
  } catch (err) {
    throw toAssistantError(err);
  }
}

export function pageContextFromLocation(pathname: string): AssistantPageContext {
  const parts = pathname.split("/").filter(Boolean);
  const page: AssistantPageContext = { route: pathname };

  // Current shape: /systems/:clusterId/accounts/:account/jetstream/(streams|kv|objects)/...
  if (parts[0] === "systems" && parts[2] === "accounts" && parts[4] === "jetstream") {
    const section = parts[5];
    if (section === "streams" && parts[6]) {
      page.stream = safeDecodeURIComponent(parts[6]);
      if (parts[7] === "consumers" && parts[8]) {
        page.consumer = safeDecodeURIComponent(parts[8]);
      }
    }
    if (section === "kv" && parts[6]) {
      page.bucket = safeDecodeURIComponent(parts[6]);
      if (parts[7]) page.key = safeDecodeURIComponent(parts[7]);
    }
    if (section === "objects" && parts[6]) {
      page.bucket = safeDecodeURIComponent(parts[6]);
    }
    return page;
  }

  // Legacy shape kept for old bookmarks/redirects: /streams/:name, /kv/:bucket, /objects/:bucket
  if (parts[0] === "streams" && parts[1]) {
    page.stream = safeDecodeURIComponent(parts[1]);
    if (parts[2] === "consumers" && parts[3]) {
      page.consumer = safeDecodeURIComponent(parts[3]);
    }
  }
  if (parts[0] === "kv" && parts[1]) {
    page.bucket = safeDecodeURIComponent(parts[1]);
    if (parts[2]) page.key = safeDecodeURIComponent(parts[2]);
  }
  if (parts[0] === "objects" && parts[1]) {
    page.bucket = safeDecodeURIComponent(parts[1]);
  }
  return page;
}

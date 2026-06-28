import { clusterPath } from "./api";

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

type AssistantErrorResponse = {
  error?: string;
  code?: AssistantErrorCode;
  retryable?: boolean;
  retryAfterSeconds?: number;
};

function inferErrorCode(status: number): AssistantErrorCode {
  if (status === 401 || status === 403) return "auth";
  if (status === 404) return "not_enabled";
  if (status === 408 || status === 504) return "timeout";
  if (status === 429) return "rate_limit";
  if (status === 503) return "unavailable";
  if (status >= 500) return "provider";
  return "validation";
}

function parseAssistantError(response: Response, body: AssistantErrorResponse): AssistantRequestError {
  const message = body.error ?? `Assistant request failed (${response.status})`;
  return new AssistantRequestError(message, {
    code: body.code ?? inferErrorCode(response.status),
    retryable: body.retryable ?? (response.status === 429 || response.status >= 500),
    retryAfterSeconds: body.retryAfterSeconds,
  });
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
    const response = await fetch("/api/v1/assistant/config", { credentials: "include" });
    if (!response.ok) {
      return { aiEnabled: false };
    }
    return response.json() as Promise<AssistantConfig>;
  } catch {
    return { aiEnabled: false };
  }
}

export async function sendAssistantMessage(
  clusterId: string,
  message: string,
  history: AssistantMessage[],
  page: AssistantPageContext,
): Promise<string> {
  let response: Response;
  try {
    response = await fetch(clusterPath(clusterId, "/assistant/chat"), {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ message, history, page }),
    });
  } catch {
    throw new AssistantRequestError("Network error. Check your connection and try again.", {
      code: "network",
      retryable: true,
    });
  }

  const body = (await response.json().catch(() => ({}))) as AssistantErrorResponse & { reply?: string };
  if (!response.ok) {
    throw parseAssistantError(response, body);
  }
  if (!body.reply) {
    throw new AssistantRequestError("Assistant returned an empty response.", {
      code: "provider",
      retryable: true,
    });
  }
  return body.reply;
}

export function pageContextFromLocation(pathname: string): AssistantPageContext {
  const parts = pathname.split("/").filter(Boolean);
  const page: AssistantPageContext = { route: pathname };

  if (parts[0] === "streams" && parts[1]) {
    page.stream = decodeURIComponent(parts[1]);
    if (parts[2] === "consumers" && parts[3]) {
      page.consumer = decodeURIComponent(parts[3]);
    }
  }
  if (parts[0] === "kv" && parts[1]) {
    page.bucket = decodeURIComponent(parts[1]);
    if (parts[2]) page.key = decodeURIComponent(parts[2]);
  }
  if (parts[0] === "objects" && parts[1]) {
    page.bucket = decodeURIComponent(parts[1]);
  }
  return page;
}

import { afterEach, describe, expect, it, vi } from "vitest";
import { api, ApiError, codeFromStatus, UnauthorizedError, userFacingError } from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("codeFromStatus", () => {
  it("maps common HTTP statuses", () => {
    expect(codeFromStatus(404)).toBe("not_found");
    expect(codeFromStatus(403)).toBe("forbidden");
    expect(codeFromStatus(401)).toBe("unauthorized");
    expect(codeFromStatus(409)).toBe("conflict");
    expect(codeFromStatus(410)).toBe("gone");
    expect(codeFromStatus(504)).toBe("timeout");
    expect(codeFromStatus(429)).toBe("rate_limit");
    expect(codeFromStatus(502)).toBe("unavailable");
    expect(codeFromStatus(400)).toBe("validation");
    expect(codeFromStatus(500)).toBe("internal");
  });
});

describe("userFacingError", () => {
  const t = (key: string) =>
    ({
      "errors.forbidden": "You do not have permission to do that.",
      "errors.csrf_invalid": "Reload the page.",
      "errors.gone": "Invite gone.",
      "errors.rate_limit": "Slow down.",
      "errors.network": "Network down.",
      "errors.requestFailed": "Request failed",
    })[key] ?? key;

  it("maps known ApiError codes to i18n", () => {
    expect(userFacingError(new ApiError("x", { code: "forbidden" }), t)).toBe(
      "You do not have permission to do that.",
    );
    expect(userFacingError(new ApiError("x", { code: "csrf_invalid" }), t)).toBe("Reload the page.");
    expect(userFacingError(new ApiError("x", { code: "gone" }), t)).toBe("Invite gone.");
  });

  it("falls back to message for unknown codes", () => {
    expect(userFacingError(new ApiError("custom detail", { code: "unknown" }), t)).toBe("custom detail");
  });
});

describe("api", () => {
  it("throws ApiError with nested body code and message", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ error: { message: "stream missing", code: "not_found" } }), {
          status: 404,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(api("/api/v1/x")).rejects.toMatchObject({
      name: "ApiError",
      message: "stream missing",
      status: 404,
      code: "not_found",
    });
  });

  it("parses retryable and retryAfterSeconds", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(
          JSON.stringify({
            error: {
              message: "NATS is unavailable",
              code: "unavailable",
              retryable: true,
              retryAfterSeconds: 5,
            },
          }),
          { status: 502, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );

    const err = await api("/api/v1/x").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("unavailable");
    expect(err.retryable).toBe(true);
    expect(err.retryAfterSeconds).toBe(5);
  });

  it("recognizes csrf_invalid and gone codes", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ error: { message: "csrf token missing or invalid", code: "csrf_invalid" } }), {
          status: 403,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );
    const csrf = await api("/api/v1/x", { method: "POST", body: "{}" }).catch((e) => e);
    expect(csrf).toBeInstanceOf(ApiError);
    expect(csrf.code).toBe("csrf_invalid");

    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ error: { message: "invite expired or already used", code: "gone" } }), {
          status: 410,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );
    const gone = await api("/api/v1/x").catch((e) => e);
    expect(gone).toBeInstanceOf(ApiError);
    expect(gone.code).toBe("gone");
  });

  it("infers code from status when body omits code", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ error: { message: "nope" } }), {
          status: 403,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    const err = await api("/api/v1/x").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("forbidden");
    expect(err.message).toBe("nope");
  });

  it("unwraps data and meta from success envelope", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ data: [{ id: "1" }], meta: { total: 1, offset: 0, limit: 50 } }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(api<{ id: string }[]>("/api/v1/x")).resolves.toEqual({
      data: [{ id: "1" }],
      meta: { total: 1, offset: 0, limit: 50 },
    });
  });

  it("returns undefined data on 204", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(null, { status: 204 })),
    );

    await expect(api<void>("/api/v1/x", { method: "DELETE" })).resolves.toEqual({
      data: undefined,
    });
  });

  it("throws ApiError network on fetch failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("Failed to fetch");
      }),
    );

    const err = await api("/api/v1/x").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("network");
    expect(err.status).toBe(0);
    expect(err.message).toMatch(/network/i);
  });

  it("throws UnauthorizedError on 401", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ error: { message: "unauthorized", code: "unauthorized" } }), {
          status: 401,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(api("/api/v1/x")).rejects.toBeInstanceOf(UnauthorizedError);
  });
});

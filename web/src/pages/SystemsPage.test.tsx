import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import SystemsPage from "./SystemsPage";
import { ApiError } from "../lib/api";
import { TestProviders } from "../test/mocks";

const api = vi.fn();

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    api: (...args: unknown[]) => api(...args),
  };
});

vi.mock("../lib/cluster", () => ({
  useCluster: () => ({
    clusters: [{ id: "c1", name: "local", isDefault: true }],
    clusterId: "c1",
    loading: false,
    error: "",
  }),
}));

describe("SystemsPage QueryErrorState", () => {
  it("renders QueryErrorState when connections status fails", async () => {
    api.mockRejectedValue(
      new ApiError("NATS is unavailable", {
        code: "unavailable",
        status: 502,
        retryable: true,
        retryAfterSeconds: 5,
      }),
    );

    render(
      <TestProviders initialEntries={["/systems"]}>
        <SystemsPage />
      </TestProviders>,
    );

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/could not load connection status/i);
      expect(screen.getByRole("alert")).toHaveTextContent(/temporarily unavailable/i);
    });
    expect(screen.getByRole("button", { name: /retry in 5s/i })).toBeDisabled();
  });
});

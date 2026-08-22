import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import SystemsPage from "./SystemsPage";
import { ApiError } from "../lib/api";
import { TestProviders } from "../test/mocks";

const { api } = vi.hoisted(() => ({ api: vi.fn() }));

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    api,
  };
});

vi.mock("../lib/cluster", () => ({
  useCluster: () => ({
    clusters: [{ id: "c1", name: "local", isDefault: true }],
    cluster: { id: "c1", name: "local", isDefault: true },
    clusterId: "c1",
    loading: false,
    error: "",
  }),
}));

vi.mock("../lib/account", () => ({
  useAccount: () => ({
    accounts: [{ id: "default", clusterId: "c1", name: "Default" }],
    accountName: "Default",
    setAccountName: () => undefined,
    loading: false,
    reload: async () => undefined,
  }),
}));

vi.mock("../hooks/useConnectionEvents", () => ({
  useConnectionEvents: () => undefined,
}));

const connectedStatus = {
  clusterId: "c1",
  connected: true,
  jetstreamOk: true,
  serverName: "nats-1",
  reconnects: 0,
  lastCheckedAt: "2026-08-02T12:00:00Z",
};

describe("SystemsPage", () => {
  it("renders QueryErrorState when connection status fails", async () => {
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

  it("embeds cluster status inside system cards", async () => {
    api.mockImplementation(async (path: string) => {
      if (typeof path === "string" && path === "/api/v1/clusters/connections") {
        return { data: [connectedStatus] };
      }
      if (typeof path === "string" && path.endsWith("/connection")) {
        return { data: connectedStatus };
      }
      return { data: null };
    });

    render(
      <TestProviders initialEntries={["/systems"]}>
        <SystemsPage />
      </TestProviders>,
    );

    await waitFor(() => {
      expect(screen.getByRole("link", { name: /local/i })).toBeInTheDocument();
      expect(screen.getByText("nats-1")).toBeInTheDocument();
    });

    expect(screen.getByRole("heading", { name: /^clusters$/i })).toBeInTheDocument();
    expect(screen.getByText("Live")).toBeInTheDocument();
    expect(screen.getByLabelText("Available")).toBeInTheDocument();
    expect(screen.getByText("JetStream")).toBeInTheDocument();
    expect(screen.getByText("Server")).toBeInTheDocument();
    expect(screen.getByText("Last checked")).toBeInTheDocument();
    expect(screen.getByText(/1 account/i)).toBeInTheDocument();
    expect(screen.getByText("Default cluster")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /cluster status/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/view registered clusters/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /^clusters$/i })).not.toBeInTheDocument();
  });
});

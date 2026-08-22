import { render, screen, waitFor } from "@testing-library/react";
import { Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import SystemAccountsPage from "./SystemAccountsPage";
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
    clusters: [{ id: "c1", name: "default", isDefault: true }],
    cluster: { id: "c1", name: "default", isDefault: true },
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

vi.mock("../hooks/useAccountOverviewEvents", () => ({
  useAccountOverviewEvents: () => undefined,
}));

describe("SystemAccountsPage", () => {
  it("renders a cluster-style account card with JetStream footer stats", async () => {
    api.mockImplementation(async (path: string) => {
      if (typeof path === "string" && path.endsWith("/monitoring/varz")) {
        return { data: { connections: 32, in_msgs: 1000, in_bytes: 2048 } };
      }
      if (typeof path === "string" && path.endsWith("/account")) {
        return {
          data: {
            memory: 1024,
            storage: 4096,
            streams: 3,
            consumers: 7,
            limits: { maxMemory: -1, maxStorage: -1, maxStreams: -1, maxConsumers: -1 },
          },
        };
      }
      return { data: null };
    });

    render(
      <TestProviders initialEntries={["/systems/c1"]}>
        <Routes>
          <Route path="/systems/:clusterId" element={<SystemAccountsPage />} />
        </Routes>
      </TestProviders>,
    );

    await waitFor(() => {
      expect(screen.getByRole("link", { name: /default/i })).toBeInTheDocument();
      expect(screen.getByText("3")).toBeInTheDocument();
      expect(screen.getByText("7")).toBeInTheDocument();
    });

    expect(screen.getByText("Live")).toBeInTheDocument();
    expect(screen.getByText("Default account")).toBeInTheDocument();
    expect(screen.getByText("32 conns")).toBeInTheDocument();
    expect(screen.getByText("1,000 msgs")).toBeInTheDocument();
    expect(screen.getByText("2.0 KB")).toBeInTheDocument();
    expect(screen.getByText("Streams")).toBeInTheDocument();
    expect(screen.getByText("Consumers")).toBeInTheDocument();
    expect(screen.getByText("Storage")).toBeInTheDocument();
    expect(screen.getByText("4.0 KB")).toBeInTheDocument();
  });
});

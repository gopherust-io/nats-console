import { render, screen, waitFor } from "@testing-library/react";
import { Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import AccountOverviewPage from "./AccountOverviewPage";
import { TestProviders } from "../test/mocks";

const { api, overviewLive } = vi.hoisted(() => ({
  api: vi.fn(),
  overviewLive: { current: true },
}));

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

vi.mock("../hooks/useClusterMetricsHistory", () => ({
  useClusterMetricsHistory: () => ({
    data: {
      series: [
        {
          metric: "jetstream.streams",
          points: [{ t: "2026-08-07T12:00:00Z", v: 99 }],
        },
      ],
    },
  }),
}));

vi.mock("../hooks/useAccountOverviewEvents", () => ({
  useAccountOverviewEvents: () => ({ live: overviewLive.current }),
}));

describe("AccountOverviewPage", () => {
  it("renders overview metrics without cluster status", async () => {
    overviewLive.current = true;
    api.mockImplementation(async (path: string) => {
      if (typeof path === "string" && path.includes("/account")) {
        return { data: { streams: 1, consumers: 1, storage: 0, memory: 0 } };
      }
      if (typeof path === "string" && path.includes("/request-reply")) {
        return { data: { requesters: 2, responders: 1, medianRttMs: 12 } };
      }
      return { data: null };
    });

    render(
      <TestProviders initialEntries={["/systems/c1/accounts/Default"]}>
        <Routes>
          <Route path="/systems/:clusterId/accounts/:accountName" element={<AccountOverviewPage />} />
        </Routes>
      </TestProviders>,
    );

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /overview/i })).toBeInTheDocument();
      expect(screen.getByRole("heading", { name: /request \/ reply/i })).toBeInTheDocument();
    });
    expect(screen.queryByRole("heading", { name: /cluster status/i })).not.toBeInTheDocument();
    expect(screen.getAllByText("Streams").length).toBeGreaterThan(0);
    expect(screen.queryByText(/live updates interrupted/i)).not.toBeInTheDocument();
  });

  it("prefers live account tile values over metrics history last point", async () => {
    overviewLive.current = true;
    api.mockImplementation(async (path: string) => {
      if (typeof path === "string" && path.includes("/account")) {
        return { data: { streams: 25, consumers: 3, storage: 0, memory: 0 } };
      }
      if (typeof path === "string" && path.includes("/request-reply")) {
        return { data: { requesters: 0, responders: 0, medianRttMs: null } };
      }
      return { data: null };
    });

    render(
      <TestProviders initialEntries={["/systems/c1/accounts/Default"]}>
        <Routes>
          <Route path="/systems/:clusterId/accounts/:accountName" element={<AccountOverviewPage />} />
        </Routes>
      </TestProviders>,
    );

    await waitFor(() => {
      expect(screen.getAllByLabelText("25").length).toBeGreaterThan(0);
    });
    expect(screen.queryByLabelText("99")).not.toBeInTheDocument();
  });

  it("shows stale snapshot alert when SSE is not live", async () => {
    overviewLive.current = false;
    api.mockImplementation(async (path: string) => {
      if (typeof path === "string" && path.includes("/account")) {
        return { data: { streams: 1, consumers: 1, storage: 0, memory: 0 } };
      }
      if (typeof path === "string" && path.includes("/request-reply")) {
        return { data: { requesters: 0, responders: 0, medianRttMs: null } };
      }
      return { data: null };
    });

    render(
      <TestProviders initialEntries={["/systems/c1/accounts/Default"]}>
        <Routes>
          <Route path="/systems/:clusterId/accounts/:accountName" element={<AccountOverviewPage />} />
        </Routes>
      </TestProviders>,
    );

    await waitFor(() => {
      expect(screen.getByText(/live updates interrupted/i)).toBeInTheDocument();
    });
  });
});

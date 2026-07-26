import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import ConsolShell from "./ConsolShell";
import { ShellRoutes, TestProviders } from "../test/mocks";

vi.mock("../lib/themeStyles", () => ({
  loadThemeStyles: vi.fn(async () => undefined),
}));

vi.mock("../lib/auth", () => ({
  useAuth: () => ({
    user: { username: "admin", roles: ["admin"], isRoot: true },
    loading: false,
    logout: vi.fn(async () => undefined),
    canManageUsers: true,
    canViewAudit: true,
    canManageAlertRules: true,
    isAdmin: true,
  }),
}));

vi.mock("../lib/alerts", () => ({
  fetchAlertOpenSummary: vi.fn(async () => ({ count: 0, alerts: [] })),
}));

vi.mock("../lib/cluster", () => ({
  useCluster: () => ({
    clusters: [{ id: "c1", name: "local", isDefault: true }],
    clusterId: "c1",
    cluster: { id: "c1", name: "local", isDefault: true },
    setClusterId: vi.fn(),
    reload: vi.fn(async () => undefined),
    loading: false,
    error: "",
  }),
}));

vi.mock("../lib/account", () => ({
  useAccount: () => ({
    accounts: [{ id: "default", clusterId: "c1", name: "Default", hasJwt: false, createdAt: "", updatedAt: "" }],
    accountName: "Default",
    setAccountName: vi.fn(),
    loading: false,
    reload: vi.fn(async () => undefined),
  }),
}));

vi.mock("./AssistantPanel", () => ({
  default: () => null,
}));

describe("ConsolShell", () => {
  it("renders topbar brand and action icons", () => {
    render(
      <TestProviders initialEntries={["/systems"]}>
        <ShellRoutes>
          <ConsolShell />
        </ShellRoutes>
      </TestProviders>,
    );

    expect(document.querySelector(".nc-topbar__brand")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Switch to Console Light" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Notifications" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open user menu" })).toBeInTheDocument();
  });

  it("opens user menu with data-state open and closes after outside click", async () => {
    const user = userEvent.setup();
    render(
      <TestProviders initialEntries={["/systems"]}>
        <ShellRoutes>
          <div>
            <button type="button">outside</button>
            <ConsolShell />
          </div>
        </ShellRoutes>
      </TestProviders>,
    );

    await user.click(screen.getByRole("button", { name: "Open user menu" }));
    const menu = screen.getByRole("menu");
    expect(menu).toHaveAttribute("data-state", "open");
    expect(screen.getByText("admin")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "outside" }));
    await waitFor(() => {
      expect(screen.queryByRole("menu")).toHaveAttribute("data-state", "closed");
    });
    await waitFor(() => {
      expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });
  });

  it("shows admin tabs on admin routes", () => {
    render(
      <TestProviders initialEntries={["/admin/users"]}>
        <ShellRoutes>
          <ConsolShell />
        </ShellRoutes>
      </TestProviders>,
    );

    expect(screen.getByRole("navigation", { name: "Admin" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Console Users" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Audit" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Clusters" })).not.toBeInTheDocument();
  });

  it("shows Clusters and Settings as team tabs next to Systems", () => {
    render(
      <TestProviders initialEntries={["/systems"]}>
        <ShellRoutes>
          <ConsolShell />
        </ShellRoutes>
      </TestProviders>,
    );

    expect(screen.getByRole("navigation", { name: "Team" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Systems" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Clusters" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Settings" })).toBeInTheDocument();
  });

  it("keeps account tabs on Topology when a system/account is selected", () => {
    render(
      <TestProviders initialEntries={["/admin/topology"]}>
        <ShellRoutes>
          <ConsolShell />
        </ShellRoutes>
      </TestProviders>,
    );

    expect(screen.getByRole("navigation", { name: "Account" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Topology" })).toHaveClass("active");
    expect(screen.getByRole("link", { name: "JetStream" })).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Admin" })).not.toBeInTheDocument();
  });
});

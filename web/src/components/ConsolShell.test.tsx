import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import ConsolShell from "./ConsolShell";
import { ShellRoutes, TestProviders } from "../test/mocks";

vi.mock("../lib/themeStyles", () => ({
  loadThemeStyles: vi.fn(async () => undefined),
  preloadThemeStyles: vi.fn(async () => undefined),
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
  default: () => <div data-testid="assistant-panel">AI</div>,
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
    expect(screen.getByRole("button", { name: /Switch to Console Light/ })).toBeInTheDocument();
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

  it("hides the team tab bar on Systems", () => {
    render(
      <TestProviders initialEntries={["/systems"]}>
        <ShellRoutes>
          <ConsolShell />
        </ShellRoutes>
      </TestProviders>,
    );

    expect(screen.queryByRole("navigation", { name: "Team" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Clusters" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Settings" })).not.toBeInTheDocument();
  });

  it("renders Docs and Architecture in the user menu", async () => {
    const user = userEvent.setup();
    render(
      <TestProviders initialEntries={["/systems"]}>
        <ShellRoutes>
          <ConsolShell />
        </ShellRoutes>
      </TestProviders>,
    );

    expect(screen.queryByRole("menuitem", { name: "Docs and Architecture" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Open user menu" }));
    const docsLink = screen.getByRole("menuitem", { name: "Docs and Architecture" });
    expect(docsLink).toBeInTheDocument();
    expect(docsLink).toHaveAttribute("href", "/docs");
  });

  it("hides the team tab bar on All streams", () => {
    render(
      <TestProviders initialEntries={["/systems/streams"]}>
        <ShellRoutes>
          <ConsolShell />
        </ShellRoutes>
      </TestProviders>,
    );

    expect(screen.queryByRole("navigation", { name: "Team" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "All streams" })).not.toBeInTheDocument();
  });

  it("shows the AI assistant only inside Docs", async () => {
    const { unmount } = render(
      <TestProviders initialEntries={["/systems"]}>
        <ShellRoutes>
          <ConsolShell />
        </ShellRoutes>
      </TestProviders>,
    );
    expect(screen.queryByTestId("assistant-panel")).not.toBeInTheDocument();
    unmount();

    render(
      <TestProviders initialEntries={["/docs"]}>
        <ShellRoutes>
          <ConsolShell />
        </ShellRoutes>
      </TestProviders>,
    );
    expect(await screen.findByTestId("assistant-panel")).toBeInTheDocument();
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

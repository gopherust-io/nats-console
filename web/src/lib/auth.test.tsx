import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthProvider, useAuth } from "./auth";
import { ApiError, UnauthorizedError } from "./api";

const api = vi.fn();

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    api: (...args: unknown[]) => api(...args),
    clearAuth: vi.fn(),
  };
});

function Probe() {
  const { user, sessionError, loading } = useAuth();
  if (loading) return <div>loading</div>;
  return (
    <div>
      <span data-testid="user">{user?.username ?? "none"}</span>
      <span data-testid="session-error">{sessionError ?? ""}</span>
    </div>
  );
}

describe("AuthProvider session errors", () => {
  beforeEach(() => {
    api.mockReset();
  });

  it("keeps session recoverable on network failure for /auth/me", async () => {
    api.mockRejectedValueOnce(new ApiError("Network request failed", { code: "network", status: 0 }));
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("user")).toHaveTextContent("none");
      expect(screen.getByRole("alert")).toHaveTextContent(/network/i);
    });
  });

  it("clears user on unauthorized without session banner", async () => {
    api.mockRejectedValueOnce(new UnauthorizedError());
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId("user")).toHaveTextContent("none");
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });
  });

  it("retries /auth/me from the session banner", async () => {
    const user = userEvent.setup();
    api
      .mockRejectedValueOnce(new ApiError("Network request failed", { code: "network", status: 0 }))
      .mockResolvedValueOnce({ data: { username: "admin", roles: ["admin"] } });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );

    await waitFor(() => expect(screen.getByRole("alert")).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: /retry/i }));
    await waitFor(() => {
      expect(screen.getByTestId("user")).toHaveTextContent("admin");
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });
  });
});

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";
import InviteAcceptPage from "./InviteAcceptPage";
import { ApiError } from "../lib/api";

const api = vi.fn();
const reload = vi.fn();

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return {
    ...actual,
    api: (...args: unknown[]) => api(...args),
  };
});

vi.mock("../lib/auth", () => ({
  useAuth: () => ({
    reload,
    user: null,
    loading: false,
  }),
}));

describe("InviteAcceptPage", () => {
  beforeEach(() => {
    api.mockReset();
    reload.mockReset();
  });

  it("shows gone invite without retry", async () => {
    api.mockRejectedValueOnce(new ApiError("invite expired or already used", { code: "gone", status: 410 }));
    render(
      <MemoryRouter initialEntries={["/invite/tok"]}>
        <Routes>
          <Route path="/invite/:token" element={<InviteAcceptPage />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/expired or was already used/i);
    });
    expect(screen.queryByRole("button", { name: /^retry$/i })).not.toBeInTheDocument();
  });

  it("retries invite load on transient failure", async () => {
    const user = userEvent.setup();
    api
      .mockRejectedValueOnce(new ApiError("NATS is unavailable", { code: "unavailable", status: 502, retryable: true }))
      .mockResolvedValueOnce({
        data: { username: "alice", email: "a@example.com", expiresAt: "2099-01-01T00:00:00Z" },
      });

    render(
      <MemoryRouter initialEntries={["/invite/tok"]}>
        <Routes>
          <Route path="/invite/:token" element={<InviteAcceptPage />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
    });
    await user.click(screen.getByRole("button", { name: /retry/i }));
    await waitFor(() => {
      expect(screen.getByText(/set a password for/i)).toBeInTheDocument();
    });
  });
});

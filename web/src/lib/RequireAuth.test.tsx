import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api";
import App from "../App";

const api = vi.fn();

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    api: (...args: unknown[]) => api(...args),
    clearAuth: vi.fn(),
  };
});

describe("RequireAuth session recovery", () => {
  beforeEach(() => {
    api.mockReset();
  });

  it("redirects to login on network /auth/me failure instead of spinning forever", async () => {
    api.mockRejectedValue(new ApiError("Network request failed", { code: "network", status: 0 }));

    render(
      <MemoryRouter initialEntries={["/systems"]}>
        <App />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/network/i);
    });
    // Login page should be reachable (not infinite PageLoader).
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /sign in/i })).toBeInTheDocument();
    });
  });
});

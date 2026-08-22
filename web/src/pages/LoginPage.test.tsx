import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import LoginPage from "./LoginPage";
import { ApiError } from "../lib/api";

const login = vi.fn();
const navigate = vi.fn();

vi.mock("../lib/auth", () => ({
  useAuth: () => ({
    login,
    user: null,
    loading: false,
  }),
}));

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    useNavigate: () => navigate,
    useSearchParams: () => [new URLSearchParams(), vi.fn()],
  };
});

describe("LoginPage", () => {
  beforeEach(() => {
    login.mockReset();
    navigate.mockReset();
  });

  it("maps ApiError codes to user-facing copy", async () => {
    const user = userEvent.setup();
    login.mockRejectedValueOnce(new ApiError("raw", { code: "rate_limit", status: 429, retryable: true }));
    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>,
    );

    await user.type(screen.getByLabelText(/username/i), "admin");
    await user.type(screen.getByLabelText(/password/i), "bad");
    await user.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/too many attempts/i);
    });
  });

  it("shows reload CTA on csrf_invalid", async () => {
    const user = userEvent.setup();
    login.mockRejectedValueOnce(new ApiError("csrf", { code: "csrf_invalid", status: 403 }));
    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>,
    );

    await user.type(screen.getByLabelText(/username/i), "admin");
    await user.type(screen.getByLabelText(/password/i), "admin");
    await user.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /reload the page/i })).toBeInTheDocument();
    });
  });

  it("toggles help copy without removing the sign-in form", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>,
    );

    const help = screen.getByRole("button", { name: /having trouble signing in/i });
    await user.click(help);
    expect(screen.getByText(/contact your console administrator/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    await user.click(help);
    expect(screen.queryByText(/contact your console administrator/i)).not.toBeInTheDocument();
  });

  it("shows validation hint when submitting empty fields and does not call login", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole("button", { name: /sign in/i }));

    expect(screen.getByRole("alert")).toHaveTextContent(/please fill out this field/i);
    expect(login).not.toHaveBeenCalled();
  });

  it("clears the validation hint when the empty field is typed into", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole("button", { name: /sign in/i }));
    expect(screen.getByRole("alert")).toHaveTextContent(/please fill out this field/i);

    await user.type(screen.getByLabelText(/username/i), "admin");
    expect(screen.queryByText(/please fill out this field/i)).not.toBeInTheDocument();
  });
});

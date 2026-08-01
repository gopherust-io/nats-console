import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "../../lib/api";
import QueryErrorState from "./QueryErrorState";

describe("QueryErrorState", () => {
  it("renders error message and retries", async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    render(<QueryErrorState error={new Error("NATS is unavailable")} onRetry={onRetry} />);

    expect(screen.getByRole("alert")).toHaveTextContent("NATS is unavailable");
    await user.click(screen.getByRole("button", { name: /retry/i }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("maps ApiError codes to i18n copy", () => {
    render(
      <QueryErrorState
        error={new ApiError("raw", { code: "forbidden", status: 403 })}
        onRetry={() => undefined}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/do not have permission/i);
  });

  it("shows retry countdown when retryAfterSeconds is set", () => {
    render(
      <QueryErrorState
        error={
          new ApiError("NATS is unavailable", {
            code: "unavailable",
            status: 502,
            retryable: true,
            retryAfterSeconds: 5,
          })
        }
        onRetry={() => undefined}
      />,
    );
    expect(screen.getByRole("button", { name: /retry in 5s/i })).toBeDisabled();
  });
});

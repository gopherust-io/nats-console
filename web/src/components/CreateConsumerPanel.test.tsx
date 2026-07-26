import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import CreateConsumerPanel from "./CreateConsumerPanel";
import type { StreamConfig } from "../lib/api";
import { TestProviders } from "../test/mocks";

vi.mock("../lib/themeStyles", () => ({
  loadThemeStyles: vi.fn(async () => undefined),
}));

const stream: StreamConfig = {
  name: "ORDERS",
  retention: "limits",
  storage: "file",
  subjects: ["orders.>"],
};

describe("CreateConsumerPanel", () => {
  it("Apply recommended setup fills durable name and filters from stream", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    const onClose = vi.fn();

    render(
      <TestProviders>
        <CreateConsumerPanel mode="create" open stream={stream} onClose={onClose} onSubmit={onSubmit} />
      </TestProviders>,
    );

    expect(screen.getByRole("heading", { name: "Create Consumer" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Apply recommended setup" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Apply recommended setup" }));

    expect(screen.getByLabelText("Durable name")).toHaveValue("ORDERS-worker");
    expect(screen.getByText("orders.>")).toBeInTheDocument();
    expect(screen.getByLabelText("Deliver policy")).toHaveValue("all");
    expect(screen.getByLabelText("Ack policy")).toHaveValue("explicit");
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Save after apply submits pull worker payload", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn(async () => undefined);

    render(
      <TestProviders>
        <CreateConsumerPanel mode="create" open stream={stream} onClose={vi.fn()} onSubmit={onSubmit} />
      </TestProviders>,
    );

    await user.click(screen.getByRole("button", { name: "Apply recommended setup" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      durableName: "ORDERS-worker",
      deliverPolicy: "all",
      ackPolicy: "explicit",
      filterSubject: "orders.>",
      ackWaitNs: 30_000_000_000,
      maxDeliver: 5,
    });
    expect(onSubmit.mock.calls[0][0].deliverSubject).toBeUndefined();
  });

  it("does not show Apply recommended setup without stream", () => {
    render(
      <TestProviders>
        <CreateConsumerPanel mode="create" open onClose={vi.fn()} onSubmit={vi.fn()} />
      </TestProviders>,
    );

    expect(screen.queryByRole("button", { name: "Apply recommended setup" })).not.toBeInTheDocument();
    expect(screen.getByText(/Prefer a durable pull consumer/i)).toBeInTheDocument();
  });
});

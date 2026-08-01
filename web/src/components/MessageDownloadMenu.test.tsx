import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import MessageDownloadMenu from "./MessageDownloadMenu";
import { TestProviders } from "../test/mocks";

vi.mock("../lib/messageDownload", async () => {
  const actual = await vi.importActual<typeof import("../lib/messageDownload")>("../lib/messageDownload");
  return {
    ...actual,
    downloadMessages: vi.fn(async () => undefined),
  };
});

describe("MessageDownloadMenu", () => {
  it("shows download formats when a message row is available", async () => {
    const user = userEvent.setup();
    render(
      <TestProviders>
        <MessageDownloadMenu
          mode="single"
          stream="ORDERS"
          rows={[
            {
              seq: 1,
              subject: "orders.created",
              time: "2026-07-27T12:00:00Z",
              data: btoa("{}"),
            },
          ]}
        />
      </TestProviders>,
    );

    await user.click(screen.getByRole("button", { name: "Download" }));
    expect(screen.getByRole("menuitem", { name: "JSON" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "CSV" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Excel (.xlsx)" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "PDF" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Text" })).toBeInTheDocument();
  });

  it("disables download when there are no rows", () => {
    render(
      <TestProviders>
        <MessageDownloadMenu mode="live" stream="ORDERS" rows={[]} />
      </TestProviders>,
    );
    expect(screen.getByRole("button", { name: "Download" })).toBeDisabled();
  });

  it("builds rows lazily via getRows on download", async () => {
    const user = userEvent.setup();
    const getRows = vi.fn(() => [
      {
        seq: 9,
        subject: "orders.created",
        time: "2026-07-27T12:00:00Z",
        data: btoa("{}"),
      },
    ]);
    const { downloadMessages } = await import("../lib/messageDownload");

    render(
      <TestProviders>
        <MessageDownloadMenu mode="live" stream="ORDERS" getRows={getRows} />
      </TestProviders>,
    );

    expect(getRows).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Download" }));
    // Opening the menu may resolve rows once for format gating; download must resolve again.
    const callsAfterOpen = getRows.mock.calls.length;
    expect(callsAfterOpen).toBeLessThanOrEqual(1);
    await user.click(screen.getByRole("menuitem", { name: "JSON" }));
    expect(getRows.mock.calls.length).toBeGreaterThan(callsAfterOpen);
    expect(downloadMessages).toHaveBeenCalled();
  });
});

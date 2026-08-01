import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import BlastRadiusPanel from "./BlastRadiusPanel";

describe("BlastRadiusPanel", () => {
  it("renders impact as a table with counts and names", () => {
    render(
      <BlastRadiusPanel
        data={{
          stream: "ORDERS",
          services: 2,
          streams: 1,
          consumers: 3,
          critical: ["payment-service", "billing-service"],
          serviceNames: ["billing-service", "payment-service"],
          relatedStreams: ["ORDERS_MIRROR"],
          consumerNames: ["bill-1", "pay-1", "pay-2"],
        }}
      />,
    );

    expect(screen.getByText("Blast Radius Analysis")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Type" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Count" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Names" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "2" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "1" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "3" })).toBeInTheDocument();
    expect(screen.getByText("billing-service, payment-service")).toBeInTheDocument();
    expect(screen.getByText("ORDERS_MIRROR")).toBeInTheDocument();
    expect(screen.getByText("bill-1, pay-1, pay-2")).toBeInTheDocument();
    expect(screen.getAllByText("payment-service").length).toBeGreaterThan(0);
  });

  it("shows loading state without data", () => {
    render(<BlastRadiusPanel loading />);
    expect(screen.getByText(/Analyzing impact/i)).toBeInTheDocument();
  });

  it("shows soft error when impact fails", () => {
    render(<BlastRadiusPanel error="not found" />);
    expect(screen.getByText(/Could not load impact analysis/i)).toBeInTheDocument();
    expect(screen.getByText(/\(not found\)/i)).toBeInTheDocument();
  });
});

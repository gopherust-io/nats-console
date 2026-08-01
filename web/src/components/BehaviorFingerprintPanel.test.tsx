import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import BehaviorFingerprintPanel, {
  formatMsgPerMin,
  formatProcessingMs,
} from "./BehaviorFingerprintPanel";

describe("BehaviorFingerprintPanel", () => {
  it("renders billing-worker normal vs current and anomaly", () => {
    render(
      <BehaviorFingerprintPanel
        durable="billing-worker"
        data={{
          available: true,
          stream: "ORDERS",
          durable: "billing-worker",
          anomaly: true,
          normal: { msgPerMin: 1000, processingMs: 200 },
          current: { msgPerMin: 1000, processingMs: 2400 },
          sustainedForMs: 30000,
        }}
      />,
    );

    expect(screen.getByText("billing-worker")).toBeInTheDocument();
    expect(screen.getByText("Normal")).toBeInTheDocument();
    expect(screen.getByText("Current")).toBeInTheDocument();
    expect(screen.getAllByText(/1,000 msg\/min/).length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText(/200ms/)).toBeInTheDocument();
    expect(screen.getByText(/2\.4s/)).toBeInTheDocument();
    expect(screen.getByText("Anomaly detected")).toBeInTheDocument();
  });

  it("shows idle state when no snapshot", () => {
    render(<BehaviorFingerprintPanel data={{ available: false }} />);
    expect(screen.getByText(/No fingerprint reported yet/i)).toBeInTheDocument();
  });

  it("shows loading state without data", () => {
    render(<BehaviorFingerprintPanel loading />);
    expect(screen.getByText(/Loading fingerprint/i)).toBeInTheDocument();
  });
});

describe("fingerprint formatters", () => {
  it("formats rates and processing", () => {
    expect(formatMsgPerMin(1000)).toBe("1,000");
    expect(formatProcessingMs(200)).toBe("200ms");
    expect(formatProcessingMs(2400)).toBe("2.4s");
  });
});

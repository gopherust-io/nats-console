import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { formatCompactCount, formatDurationMs } from "../lib/formatImpact";
import ReplayDryRunPanel from "./ReplayDryRunPanel";

describe("formatCompactCount", () => {
  it("formats large counts", () => {
    expect(formatCompactCount(10_000_000)).toBe("10M");
    expect(formatCompactCount(1_500)).toBe("1.5K");
    expect(formatCompactCount(42)).toBe("42");
  });
});

describe("formatDurationMs", () => {
  it("humanizes durations", () => {
    expect(formatDurationMs(0)).toBe("0s");
    expect(formatDurationMs(12_000)).toBe("12s");
    expect(formatDurationMs(45 * 60 * 1000)).toBe("45m");
    expect(formatDurationMs(3 * 60 * 60 * 1000)).toBe("3h");
  });
});

describe("ReplayDryRunPanel", () => {
  it("renders expected impact and potential duplicates", () => {
    render(
      <ReplayDryRunPanel
        data={{
          messages: 10_000_000,
          estimatedDurationMs: 3 * 60 * 60 * 1000,
          consumersAffected: 1,
          potentialDuplicates: ["payment-service", "email-service"],
        }}
      />,
    );

    expect(screen.getByText("Replay Dry Run")).toBeInTheDocument();
    expect(screen.getByText("Before replaying:")).toBeInTheDocument();
    expect(screen.getByText("Expected impact")).toBeInTheDocument();
    expect(screen.getByText("10M")).toBeInTheDocument();
    expect(screen.getByText("3h")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("payment-service")).toBeInTheDocument();
    expect(screen.getByText("email-service")).toBeInTheDocument();
  });

  it("shows loading state without data", () => {
    render(<ReplayDryRunPanel loading />);
    expect(screen.getByText(/Estimating impact/i)).toBeInTheDocument();
  });

  it("shows soft error when dry-run fails", () => {
    render(<ReplayDryRunPanel error="not found" />);
    expect(screen.getByText(/Could not estimate replay impact/i)).toBeInTheDocument();
    expect(screen.getByText(/\(not found\)/i)).toBeInTheDocument();
  });
});

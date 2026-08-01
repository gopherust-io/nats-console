import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import IncidentReconstructionPanel from "./IncidentReconstructionPanel";

describe("IncidentReconstructionPanel", () => {
  it("renders the five-event mockup timeline", () => {
    render(
      <IncidentReconstructionPanel
        data={{
          clusterId: "c1",
          stream: "ORDERS",
          consumer: "billing-worker",
          from: "2026-07-29T14:00:00Z",
          to: "2026-07-29T15:00:00Z",
          eventCount: 5,
          usedDeployAnnotations: true,
          events: [
            { at: "2026-07-29T14:02:00Z", category: "deploy", label: "Deploy", source: "annotation" },
            {
              at: "2026-07-29T14:04:00Z",
              category: "lag_growth",
              label: "Consumer lag grows",
              source: "consumer_sample",
              evidence: "lag 10 → 200",
            },
            {
              at: "2026-07-29T14:06:00Z",
              category: "redelivery_spike",
              label: "Redeliveries spike",
              source: "consumer_sample",
            },
            {
              at: "2026-07-29T14:08:00Z",
              category: "node_disconnect",
              label: "Node B disconnects",
              source: "routez",
            },
            {
              at: "2026-07-29T14:09:00Z",
              category: "processing_stopped",
              label: "Processing stops",
              source: "consumer_sample",
            },
          ],
        }}
      />,
    );

    expect(screen.getByText("Incident reconstruction")).toBeInTheDocument();
    expect(screen.getByText("Deploy")).toBeInTheDocument();
    expect(screen.getByText("Consumer lag grows")).toBeInTheDocument();
    expect(screen.getByText("Redeliveries spike")).toBeInTheDocument();
    expect(screen.getByText("Node B disconnects")).toBeInTheDocument();
    expect(screen.getByText("Processing stops")).toBeInTheDocument();
    expect(screen.getByText("lag 10 → 200")).toBeInTheDocument();
  });

  it("shows empty state", () => {
    render(
      <IncidentReconstructionPanel
        data={{
          clusterId: "c1",
          stream: "ORDERS",
          consumer: "billing-worker",
          from: "2026-07-29T14:00:00Z",
          to: "2026-07-29T15:00:00Z",
          events: [],
          eventCount: 0,
        }}
      />,
    );
    expect(screen.getByText(/No incident signals/i)).toBeInTheDocument();
  });

  it("shows loading and error states", () => {
    const { rerender } = render(<IncidentReconstructionPanel loading />);
    expect(screen.getByText(/Reconstructing incident timeline/i)).toBeInTheDocument();

    rerender(<IncidentReconstructionPanel error="timeout" />);
    expect(screen.getByText(/Could not load incident reconstruction/i)).toBeInTheDocument();
    expect(screen.getByText(/timeout/i)).toBeInTheDocument();
  });

  it("notes audit fallback", () => {
    render(
      <IncidentReconstructionPanel
        data={{
          clusterId: "c1",
          stream: "ORDERS",
          consumer: "billing-worker",
          from: "2026-07-29T14:00:00Z",
          to: "2026-07-29T15:00:00Z",
          eventCount: 1,
          usedAuditFallback: true,
          events: [
            {
              at: "2026-07-29T14:02:00Z",
              category: "change",
              label: "update consumer billing-worker",
              source: "audit",
            },
          ],
        }}
      />,
    );
    expect(screen.getByText(/audit changes as fallback/i)).toBeInTheDocument();
  });
});

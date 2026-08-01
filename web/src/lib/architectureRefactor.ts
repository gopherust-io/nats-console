import { api, clusterPath } from "./api";

export type ArchitectureRefactorNode = {
  id: string;
  label: string;
  kind: string;
};

export type ArchitectureRefactorEdge = {
  from: string;
  to: string;
  label?: string;
};

export type ArchitectureRefactorGraph = {
  nodes: ArchitectureRefactorNode[];
  edges: ArchitectureRefactorEdge[];
  label?: string;
};

export type ArchitectureRefactorStep = {
  order: number;
  title: string;
  detail: string;
};

export type ArchitectureRefactorSeed = {
  kind?: string;
  stream?: string;
  subject?: string;
};

export type ArchitectureRefactorPlan = {
  capturedAt?: string;
  clusterName?: string;
  question: string;
  verdict: string;
  rationale: string;
  eventSubject: string;
  before: ArchitectureRefactorGraph;
  after: ArchitectureRefactorGraph;
  steps: ArchitectureRefactorStep[];
  seed?: ArchitectureRefactorSeed;
  demo?: boolean;
  narrative?: string;
};

export type ArchitectureRefactorAskResult = {
  reply: string;
  plan: ArchitectureRefactorPlan;
};

export type FetchRefactorOptions = ArchitectureRefactorSeed & {
  fresh?: boolean;
  demo?: boolean;
};

function normalizePlan(data?: Partial<ArchitectureRefactorPlan> | null): ArchitectureRefactorPlan {
  return {
    capturedAt: data?.capturedAt,
    clusterName: data?.clusterName,
    question: data?.question ?? "Reduce coupling.",
    verdict: data?.verdict ?? "",
    rationale: data?.rationale ?? "",
    eventSubject: data?.eventSubject ?? "",
    before: {
      nodes: Array.isArray(data?.before?.nodes) ? data!.before!.nodes : [],
      edges: Array.isArray(data?.before?.edges) ? data!.before!.edges : [],
      label: data?.before?.label,
    },
    after: {
      nodes: Array.isArray(data?.after?.nodes) ? data!.after!.nodes : [],
      edges: Array.isArray(data?.after?.edges) ? data!.after!.edges : [],
      label: data?.after?.label,
    },
    steps: Array.isArray(data?.steps) ? data!.steps : [],
    seed: data?.seed,
    demo: Boolean(data?.demo),
    narrative: data?.narrative,
  };
}

function qs(options?: FetchRefactorOptions): string {
  const p = new URLSearchParams();
  if (options?.fresh) p.set("fresh", "1");
  if (options?.demo) p.set("demo", "1");
  if (options?.kind) p.set("kind", options.kind);
  if (options?.stream) p.set("stream", options.stream);
  if (options?.subject) p.set("subject", options.subject);
  const s = p.toString();
  return s ? `?${s}` : "";
}

export async function fetchArchitectureRefactor(
  clusterId: string | null | undefined,
  options?: FetchRefactorOptions,
): Promise<ArchitectureRefactorPlan> {
  if (!clusterId || options?.demo) {
    if (clusterId && options?.demo) {
      const snap = await api<ArchitectureRefactorPlan>(
        clusterPath(clusterId, `/architecture-refactor${qs({ ...options, demo: true })}`),
      );
      return normalizePlan(snap.data);
    }
    const snap = await api<ArchitectureRefactorPlan>("/api/v1/architecture-refactor/demo");
    return normalizePlan(snap.data);
  }
  const snap = await api<ArchitectureRefactorPlan>(
    clusterPath(clusterId, `/architecture-refactor${qs(options)}`),
  );
  return normalizePlan(snap.data);
}

export async function askArchitectureRefactor(
  clusterId: string,
  message?: string,
  options?: FetchRefactorOptions,
): Promise<ArchitectureRefactorAskResult> {
  const snap = await api<ArchitectureRefactorAskResult>(
    clusterPath(clusterId, `/architecture-refactor/ask${qs(options)}`),
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ message: message || "Reduce coupling." }),
    },
  );
  return {
    reply: snap.data?.reply ?? "",
    plan: normalizePlan(snap.data?.plan),
  };
}

export function demoArchitectureRefactor(): ArchitectureRefactorPlan {
  return {
    question: "Reduce coupling.",
    verdict: "Decouple A → B → C via event orders.changed",
    rationale:
      "Observed synchronous-style coupling across A, B, C. Introduce a JetStream subject so producers publish once and consumers subscribe independently.",
    eventSubject: "orders.changed",
    demo: true,
    before: {
      label: "Before: A → B → C",
      nodes: [
        { id: "nA", label: "A", kind: "stream" },
        { id: "nB", label: "B", kind: "stream" },
        { id: "nC", label: "C", kind: "stream" },
      ],
      edges: [
        { from: "nA", to: "nB", label: "sync / direct" },
        { from: "nB", to: "nC", label: "sync / direct" },
      ],
    },
    after: {
      label: "After: A → Event → B,C",
      nodes: [
        { id: "nA", label: "A", kind: "stream" },
        { id: "nEvent", label: "orders.changed", kind: "event" },
        { id: "nB", label: "B", kind: "stream" },
        { id: "nC", label: "C", kind: "stream" },
      ],
      edges: [
        { from: "nA", to: "nEvent", label: "publish" },
        { from: "nEvent", to: "nB", label: "consume" },
        { from: "nEvent", to: "nC", label: "consume" },
      ],
    },
    steps: [
      {
        order: 1,
        title: "Introduce event subject",
        detail: "Create or extend a JetStream stream that owns subject `orders.changed`.",
      },
      {
        order: 2,
        title: "Dual-publish from producer",
        detail: "Update `A` to publish `orders.changed` alongside the existing direct path.",
      },
      {
        order: 3,
        title: "Add consumers on the event",
        detail: "Point `B, C` at durable consumers filtered on `orders.changed`.",
      },
      {
        order: 4,
        title: "Shadow / verify",
        detail: "Run both paths in parallel until parity is proven.",
      },
      {
        order: 5,
        title: "Remove direct coupling",
        detail: "Delete sync edges A → B → C once consumers are healthy.",
      },
      {
        order: 6,
        title: "Cut over and monitor",
        detail: "Disable dual-publish leftovers and watch Architecture Review.",
      },
    ],
  };
}

export const ARCHITECTURE_REFACTOR_HREF = "/docs/architecture-refactor";

export function architectureRefactorHref(seed?: ArchitectureRefactorSeed): string {
  const p = new URLSearchParams();
  if (seed?.kind) p.set("kind", seed.kind);
  if (seed?.stream) p.set("stream", seed.stream);
  if (seed?.subject) p.set("subject", seed.subject);
  const q = p.toString();
  return q ? `${ARCHITECTURE_REFACTOR_HREF}?${q}` : ARCHITECTURE_REFACTOR_HREF;
}

export function isCouplingFindingKind(kind: string): boolean {
  return (
    kind === "tight_coupling" ||
    kind === "circular_dependency" ||
    kind === "too_many_consumers"
  );
}

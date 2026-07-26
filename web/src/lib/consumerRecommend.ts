import type { StreamConfig } from "./api";

const ACK_WAIT_NS = 30_000_000_000; // 30s
const MAX_DELIVER = 5;

export type RecommendedConsumerConfig = {
  durableName: string;
  filterSubjects: string[];
  deliverPolicy: string;
  ackPolicy: string;
  replayPolicy: string;
  /** Empty = pull */
  deliverSubject: string;
  ackWaitNs: number;
  maxDeliver: number;
  maxAckPending?: number;
  inactiveThresholdNs?: number;
};

/** NATS-friendly durable suffix: keep alnum, dash, underscore. */
export function recommendedDurableName(streamName: string): string {
  const base = streamName.trim().replace(/[^A-Za-z0-9_-]+/g, "-").replace(/^-+|-+$/g, "") || "stream";
  return `${base}-worker`;
}

/**
 * Use concrete stream subjects as filters. Skip when empty (mirror) or only `>`
 * (consumer already sees the whole stream without a filter).
 */
export function recommendedFilterSubjects(stream: StreamConfig): string[] {
  const subjects = (stream.subjects ?? []).map((s) => s.trim()).filter(Boolean);
  if (subjects.length === 0) return [];
  if (subjects.length === 1 && subjects[0] === ">") return [];
  return subjects;
}

export function recommendedConsumerFromStream(
  stream: StreamConfig,
  streamName = stream.name,
): RecommendedConsumerConfig {
  const limits = stream.consumerLimits;
  const cfg: RecommendedConsumerConfig = {
    durableName: recommendedDurableName(streamName || stream.name),
    filterSubjects: recommendedFilterSubjects(stream),
    deliverPolicy: "all",
    ackPolicy: stream.noAck ? "none" : "explicit",
    replayPolicy: "instant",
    deliverSubject: "",
    ackWaitNs: ACK_WAIT_NS,
    maxDeliver: MAX_DELIVER,
  };
  if (limits?.maxAckPending && limits.maxAckPending > 0) {
    cfg.maxAckPending = limits.maxAckPending;
  }
  if (limits?.inactiveThreshold && limits.inactiveThreshold > 0) {
    cfg.inactiveThresholdNs = limits.inactiveThreshold;
  }
  return cfg;
}

export function recommendFiltersLabel(filters: string[]): string {
  return filters.length > 0 ? filters.join(", ") : "";
}

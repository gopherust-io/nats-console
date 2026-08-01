/** Messages behind the consumer cursor relative to stream tip. */
export function consumerLag(streamLastSeq: number, deliveredStreamSeq?: number): number {
  const delivered = deliveredStreamSeq ?? 0;
  const lag = streamLastSeq - delivered;
  return lag > 0 ? lag : 0;
}

/** Format JetStream ack-wait nanoseconds for display (config, not measured latency). */
export function formatAckWaitNs(ns?: number): string {
  if (ns == null || ns <= 0) return "—";
  const hour = 3_600_000_000_000;
  const minute = 60_000_000_000;
  const second = 1_000_000_000;
  if (ns % hour === 0) return `${ns / hour}h`;
  if (ns % minute === 0) return `${ns / minute}m`;
  if (ns % second === 0) return `${ns / second}s`;
  if (ns % 1_000_000 === 0) return `${ns / 1_000_000}ms`;
  return `${ns}ns`;
}

/** Compact stream/consumer sequence pair for tables and cards. */
export function formatSeqPair(streamSeq?: number, consumerSeq?: number): string {
  if (streamSeq == null && consumerSeq == null) return "—";
  return `${streamSeq ?? 0} / ${consumerSeq ?? 0}`;
}

/** Defaults match server SLOW_CONSUMER_* env / nats WatchSlowConsumer. */
export const DEFAULT_SLOW_CONSUMER_THRESHOLDS = {
  pendingThreshold: 1000,
  lagThreshold: 1000,
  ackPendingRatio: 0.9,
} as const;

export type SlowConsumerThresholds = {
  pendingThreshold?: number;
  lagThreshold?: number;
  ackPendingRatio?: number;
};

export type SlowConsumerInput = {
  pending: number;
  lag: number;
  ackPending: number;
  maxAckPending?: number;
  thresholds?: SlowConsumerThresholds;
};

export type SlowConsumerResult = {
  slow: boolean;
  reasons: Array<"pending" | "lag" | "ack_pending">;
};

/** Evaluate JetStream slow-consumer thresholds (same semantics as server/domain). */
export function evaluateSlowConsumer(input: SlowConsumerInput): SlowConsumerResult {
  const pendingThreshold =
    input.thresholds?.pendingThreshold ?? DEFAULT_SLOW_CONSUMER_THRESHOLDS.pendingThreshold;
  const lagThreshold = input.thresholds?.lagThreshold ?? DEFAULT_SLOW_CONSUMER_THRESHOLDS.lagThreshold;
  const ackPendingRatio =
    input.thresholds?.ackPendingRatio ?? DEFAULT_SLOW_CONSUMER_THRESHOLDS.ackPendingRatio;

  const reasons: SlowConsumerResult["reasons"] = [];
  if (input.pending >= pendingThreshold) reasons.push("pending");
  if (input.lag >= lagThreshold) reasons.push("lag");
  const maxAck = input.maxAckPending ?? 0;
  if (maxAck > 0) {
    const limit = Math.max(1, Math.floor(maxAck * ackPendingRatio));
    if (input.ackPending >= limit) reasons.push("ack_pending");
  }
  return { slow: reasons.length > 0, reasons };
}

export function isSlowConsumer(input: SlowConsumerInput): boolean {
  return evaluateSlowConsumer(input).slow;
}

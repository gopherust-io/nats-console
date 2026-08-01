/** Per-stream history metric names — must match Go domain.StreamMetric. */
export const StreamMetricKind = {
  LastSeq: "last_seq",
  Bytes: "bytes",
  DeliveredSeq: "delivered_seq",
  AckFloorSeq: "ack_floor_seq",
} as const;

export type StreamMetricKindName = (typeof StreamMetricKind)[keyof typeof StreamMetricKind];

export function streamMetric(streamName: string, kind: StreamMetricKindName): string {
  return `stream:${streamName}:${kind}`;
}

export function streamRateMetricsCSV(streamName: string): string {
  return [
    streamMetric(streamName, StreamMetricKind.LastSeq),
    streamMetric(streamName, StreamMetricKind.DeliveredSeq),
    streamMetric(streamName, StreamMetricKind.AckFloorSeq),
    streamMetric(streamName, StreamMetricKind.Bytes),
  ].join(",");
}

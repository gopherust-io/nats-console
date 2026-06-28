export type MetricsHistoryPoint = {
  t: string;
  v: number;
};

export type MetricsHistorySeries = {
  metric: string;
  points: MetricsHistoryPoint[];
};

export type MetricsHistoryResponse = {
  clusterId: string;
  from: string;
  to: string;
  series: MetricsHistorySeries[];
};

export type MetricsRangePreset = "1h" | "6h" | "24h" | "7d";

export function rangeToQuery(preset: MetricsRangePreset): { from: string; to: string; step: string } {
  const to = new Date();
  const from = new Date(to);
  let step = "5m";
  switch (preset) {
    case "1h":
      from.setHours(from.getHours() - 1);
      step = "1m";
      break;
    case "6h":
      from.setHours(from.getHours() - 6);
      step = "5m";
      break;
    case "24h":
      from.setHours(from.getHours() - 24);
      step = "5m";
      break;
    case "7d":
      from.setDate(from.getDate() - 7);
      step = "1h";
      break;
  }
  return { from: from.toISOString(), to: to.toISOString(), step };
}

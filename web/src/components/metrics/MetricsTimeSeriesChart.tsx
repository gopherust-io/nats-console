import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { MetricsHistoryPoint } from "../../lib/metricsHistory";

export type ChartSeries = {
  key: string;
  label: string;
  color: string;
  points: MetricsHistoryPoint[];
  formatValue?: (value: number) => string;
};

type Props = {
  title: string;
  series: ChartSeries[];
  variant?: "line" | "area";
  emptyMessage?: string;
};

function mergeSeries(series: ChartSeries[]) {
  const map = new Map<number, Record<string, number | string>>();
  for (const item of series) {
    for (const point of item.points) {
      const ts = new Date(point.t).getTime();
      const row = map.get(ts) ?? { t: ts };
      row[item.key] = point.v;
      map.set(ts, row);
    }
  }
  return Array.from(map.values()).sort((a, b) => Number(a.t) - Number(b.t));
}

function formatAxisTime(ts: number) {
  const date = new Date(ts);
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export default function MetricsTimeSeriesChart({
  title,
  series,
  variant = "line",
  emptyMessage = "Collecting historical data…",
}: Props) {
  const data = mergeSeries(series);
  const hasData = data.length > 0;

  return (
    <div className="panel metrics-chart">
      <div className="panel__header">
        <h3 className="panel__title">{title}</h3>
      </div>
      {!hasData ? (
        <p className="text-muted metrics-chart__empty">{emptyMessage}</p>
      ) : (
        <div className="metrics-chart__canvas">
          <ResponsiveContainer width="100%" height={240}>
            {variant === "area" ? (
              <AreaChart data={data}>
                <CartesianGrid stroke="var(--border-subtle)" strokeDasharray="3 3" />
                <XAxis
                  dataKey="t"
                  tickFormatter={formatAxisTime}
                  stroke="var(--text-muted)"
                  fontSize={12}
                  tickLine={false}
                />
                <YAxis stroke="var(--text-muted)" fontSize={12} tickLine={false} width={56} />
                <Tooltip
                  labelFormatter={(value) => new Date(Number(value)).toLocaleString()}
                  formatter={(value: number, name: string) => {
                    const item = series.find((s) => s.key === name);
                    return [item?.formatValue ? item.formatValue(value) : value, item?.label ?? name];
                  }}
                  contentStyle={{
                    background: "var(--bg-card)",
                    border: "1px solid var(--border)",
                    borderRadius: "8px",
                  }}
                />
                <Legend />
                {series.map((item) => (
                  <Area
                    key={item.key}
                    type="monotone"
                    dataKey={item.key}
                    name={item.label}
                    stroke={item.color}
                    fill={item.color}
                    fillOpacity={0.12}
                    strokeWidth={2}
                    dot={false}
                  />
                ))}
              </AreaChart>
            ) : (
              <LineChart data={data}>
                <CartesianGrid stroke="var(--border-subtle)" strokeDasharray="3 3" />
                <XAxis
                  dataKey="t"
                  tickFormatter={formatAxisTime}
                  stroke="var(--text-muted)"
                  fontSize={12}
                  tickLine={false}
                />
                <YAxis stroke="var(--text-muted)" fontSize={12} tickLine={false} width={56} />
                <Tooltip
                  labelFormatter={(value) => new Date(Number(value)).toLocaleString()}
                  formatter={(value: number, name: string) => {
                    const item = series.find((s) => s.key === name);
                    return [item?.formatValue ? item.formatValue(value) : value, item?.label ?? name];
                  }}
                  contentStyle={{
                    background: "var(--bg-card)",
                    border: "1px solid var(--border)",
                    borderRadius: "8px",
                  }}
                />
                <Legend />
                {series.map((item) => (
                  <Line
                    key={item.key}
                    type="monotone"
                    dataKey={item.key}
                    name={item.label}
                    stroke={item.color}
                    strokeWidth={2}
                    dot={false}
                  />
                ))}
              </LineChart>
            )}
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

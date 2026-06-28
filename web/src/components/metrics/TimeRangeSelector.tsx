import { MetricsRangePreset } from "../../lib/metricsHistory";

const PRESETS: { id: MetricsRangePreset; label: string }[] = [
  { id: "1h", label: "1h" },
  { id: "6h", label: "6h" },
  { id: "24h", label: "24h" },
  { id: "7d", label: "7d" },
];

type Props = {
  value: MetricsRangePreset;
  onChange: (value: MetricsRangePreset) => void;
};

export default function TimeRangeSelector({ value, onChange }: Props) {
  return (
    <div className="metrics-range" role="group" aria-label="Time range">
      {PRESETS.map((preset) => (
        <button
          key={preset.id}
          type="button"
          className={`metrics-range__btn${value === preset.id ? " metrics-range__btn--active" : ""}`}
          onClick={() => onChange(preset.id)}
        >
          {preset.label}
        </button>
      ))}
    </div>
  );
}

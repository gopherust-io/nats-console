import type { ReactNode } from "react";

type StatCardProps = {
  label: string;
  value: ReactNode;
  hint?: string;
  icon?: ReactNode;
  accent?: "sky" | "violet" | "emerald" | "amber";
};

export default function StatCard({ label, value, hint, icon, accent = "sky" }: StatCardProps) {
  return (
    <article className={`stat-card stat-card--${accent}`}>
      <div className="stat-card__top">
        <span className="stat-card__label">{label}</span>
        {icon && <span className="stat-card__icon">{icon}</span>}
      </div>
      <div className="stat-card__value">{value}</div>
      {hint && <div className="stat-card__hint">{hint}</div>}
    </article>
  );
}

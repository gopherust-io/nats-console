import type { ReactNode } from "react";

type PageHeaderProps = {
  title: string;
  subtitle?: string;
  eyebrow?: string;
  badge?: ReactNode;
  actions?: ReactNode;
};

export default function PageHeader({ title, subtitle, eyebrow, badge, actions }: PageHeaderProps) {
  return (
    <header className="page-header">
      <div className="page-header__main">
        {eyebrow && <div className="page-header__eyebrow">{eyebrow}</div>}
        <div className="page-header__title-row">
          <h1>{title}</h1>
          {badge}
        </div>
        {subtitle && <p className="page-header__subtitle">{subtitle}</p>}
      </div>
      {actions && <div className="page-header__actions">{actions}</div>}
    </header>
  );
}

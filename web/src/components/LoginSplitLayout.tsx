import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import BrandLogo from "./BrandLogo";
import { ParticleWave } from "./ui/particle-wave";

type Props = {
  children: ReactNode;
};

export default function LoginSplitLayout({ children }: Props) {
  const { t } = useTranslation();

  return (
    <div className="login-page login-page--split">
      <div className="login-page__wave" aria-hidden>
        <ParticleWave />
      </div>

      <header className="login-topbar">
        <span className="login-topbar__brand brand" aria-label={t("common.brandTitle")}>
          <BrandLogo />
        </span>
      </header>

      <div className="login-split-body">
        <main className="login-pane">{children}</main>

        <aside className="login-promo">
          <div className="login-promo__inner">
            <h2 className="login-promo__title">{t("auth.promo.headline")}</h2>
            <p className="login-promo__body">{t("auth.promo.body")}</p>
          </div>
        </aside>
      </div>
    </div>
  );
}

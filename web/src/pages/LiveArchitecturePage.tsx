import { useCallback, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import LiveArchitecturePainting, {
  type LiveArchScene,
} from "../components/LiveArchitecturePainting";
import PageHeader from "../components/ui/PageHeader";
import "../styles/live-architecture.css";

function parseScene(value: string | null): LiveArchScene {
  return value === "layers" ? "layers" : "deploy";
}

export default function LiveArchitecturePage() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const scene = parseScene(searchParams.get("scene"));

  const setScene = useCallback(
    (next: LiveArchScene) => {
      setSearchParams(
        (prev) => {
          const p = new URLSearchParams(prev);
          if (next === "layers") p.set("scene", "layers");
          else p.delete("scene");
          p.delete("kiosk");
          return p;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  useEffect(() => {
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key === "1") setScene("deploy");
      else if (ev.key === "2") setScene("layers");
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [setScene]);

  return (
    <div className="arch-page">
      <PageHeader title={t("liveArch.title")} subtitle={t("liveArch.subtitle")} />
      <div className="live-arch-page__toolbar" role="toolbar" aria-label={t("liveArch.toolbarAria")}>
        <button type="button" aria-pressed={scene === "deploy"} onClick={() => setScene("deploy")}>
          {t("liveArch.sceneDeploy")}
        </button>
        <button type="button" aria-pressed={scene === "layers"} onClick={() => setScene("layers")}>
          {t("liveArch.sceneLayers")}
        </button>
      </div>
      <LiveArchitecturePainting scene={scene} />
    </div>
  );
}
